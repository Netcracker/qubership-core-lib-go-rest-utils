package consul

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/avast/retry-go/v4"
	consulApi "github.com/hashicorp/consul/api"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-core-lib-go/v3/utils"
)

var logger logging.Logger

func init() {
	logger = logging.GetLogger("consul-property-source")
}

type ClientConfig struct {
	Address string
	// Deprecated: used by the m2m mode only
	Namespace               string
	Ctx                     context.Context
	Token                   *ClientToken
	Mode                    AuthMode
	AuthMethod              string
	Audience                string
	FallbackRecheckInterval time.Duration
	tokenProvider           func() (string, error)
}

type Client struct {
	consul    *consulApi.Client
	cfg       ClientConfig
	token     *ClientToken
	provider  tokenProvider
	updater   *tokenUpdater
	configErr error
	mutex     *sync.Mutex
}

type ClientToken struct {
	val            atomic.Value
	expirationTime time.Time
}

func NewClient(cfg ClientConfig) *Client {
	if cfg.Ctx == nil {
		cfg.Ctx = context.Background()
	}
	return &Client{
		consul: newConsulApiClient(cfg.Address, ""),
		cfg:    cfg,
		token:  cfg.Token,
		mutex:  &sync.Mutex{},
	}
}

func (r *Client) KV() *consulApi.KV {
	tokenValue, _ := r.token.val.Load().(string)
	r.consul = newConsulApiClient(r.cfg.Address, tokenValue)
	return r.consul.KV()
}

func (r *Client) Login() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.token != nil {
		return nil
	}
	if err := r.initTokenProvider(); err != nil {
		return err
	}
	token, err := r.provider.GetToken(r.cfg.Ctx)
	if err != nil {
		return err
	}
	authMethod := token.authMethod
	if authMethod == "" {
		authMethod = "unknown"
	}
	logger.InfoC(r.cfg.Ctx, "Logged in to Consul with auth method '%s'", authMethod)

	r.consul = newConsulApiClient(r.cfg.Address, token.secretID)
	r.token = &ClientToken{}

	if token.secretID != "" {
		r.token.val.Store(token.secretID)
	}
	if token.expirationTime != nil {
		r.token.expirationTime = *token.expirationTime
		r.updater.start(r.cfg.Ctx, token)
	}

	return nil
}

// Deferred to the first Login: at NewClient time configloader is not initialized yet and cfg.Namespace
// may still be empty, both filled in by the property source before it logs in.
func (r *Client) initTokenProvider() error {
	if r.provider != nil {
		return nil
	}
	if r.configErr != nil {
		return r.configErr
	}
	authCfg, err := resolveAuthConfig(r.cfg)
	if err != nil {
		r.configErr = err
		return err
	}
	r.provider = r.newTokenProvider(authCfg)
	r.updater = &tokenUpdater{
		provider: r.provider,
		apply:    r.applyToken,
		now:      time.Now,
	}
	return nil
}

func (r *Client) newTokenProvider(authCfg authConfig) tokenProvider {
	switch authCfg.mode {
	case AuthModeM2M:
		return r.m2mTokenProvider()
	case AuthModeKubernetes:
		return r.kubernetesTokenProvider(authCfg)
	default:
		return &fallbackTokenProvider{
			primary:             r.kubernetesTokenProvider(authCfg),
			secondary:           r.m2mTokenProvider(),
			primaryAuthMethod:   authCfg.authMethod,
			secondaryAuthMethod: r.cfg.Namespace,
			interval:            authCfg.recheckInterval,
			now:                 time.Now,
		}
	}
}

func (r *Client) kubernetesTokenProvider(authCfg authConfig) tokenProvider {
	return &loginTokenProvider{
		credentials: kubernetesCredentials{authMethod: authCfg.authMethod, audience: authCfg.audience},
		acl:         r.acl,
	}
}

func (r *Client) m2mTokenProvider() tokenProvider {
	credentials := m2mCredentials{authMethod: r.cfg.Namespace}
	if r.cfg.tokenProvider != nil {
		getToken := r.cfg.tokenProvider
		credentials.provider = func(ctx context.Context) (string, error) { return getToken() }
	}
	return &loginTokenProvider{credentials: credentials, acl: r.acl}
}

func (r *Client) acl() *consulApi.ACL {
	return newConsulApiClient(r.cfg.Address, "").ACL()
}

func (r *Client) applyToken(token *consulToken) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.token.val.Store(token.secretID)
	if token.expirationTime != nil {
		r.token.expirationTime = *token.expirationTime
	}
}

func (r *Client) SecretId() string {
	tokenValue, _ := r.token.val.Load().(string)
	return tokenValue
}

func newConsulApiClient(addr, token string) *consulApi.Client {
	consulConfig := consulApi.DefaultConfig()
	consulConfig.Address = strings.TrimSuffix(addr, "/")
	consulConfig.Token = token
	consulConfig.TLSConfig = consulApi.TLSConfig{
		CAFile:   utils.GetCaCertFile(),
		CertFile: utils.GetCertFile(),
		KeyFile:  utils.GetKeyFile(),
	}
	client, err := consulApi.NewClient(consulConfig)
	if err != nil {
		logger.Panicf("can not create Consul client: %s", err.Error())
		return nil
	}
	return client
}

func (r *Client) subscribeFor(path string, keyIndex uint64, cb func(event interface{}, err error)) {
	currentIndex := keyIndex
	go func() {
		for {
			err := retry.Do(
				func() error {
					err := r.Login()
					if err != nil {
						logger.Errorf("Error during login to Consul: %+v", err)
						return fmt.Errorf("error during login to Consul: %w", err)
					}

					list, meta, err := r.KV().List(path, (&consulApi.QueryOptions{WaitIndex: currentIndex}).WithContext(r.cfg.Ctx))
					if err != nil {
						logger.ErrorC(r.cfg.Ctx, "Error read from KV: key=%s; err=%s", path, err.Error())
						return fmt.Errorf("error reading from KV: %w", err)
					}
					if list == nil {
						logger.ErrorC(r.cfg.Ctx, "there is no path created for '%s'", path)
						return fmt.Errorf("path for '%s' is not exists", path)
					}

					if list != nil && meta != nil {
						configMap := make(map[string]interface{})
						kvPairsAsMap(cutPrefix(list, path), configMap)
						cb(nil, nil)
						currentIndex = meta.LastIndex
					}
					return nil
				},
				retry.Context(r.cfg.Ctx),
				retry.Delay(5*time.Second),
				retry.MaxDelay(5*time.Minute),
				retry.DelayType(retry.BackOffDelay),
				retry.UntilSucceeded(),
			)
			if err != nil {
				logger.ErrorC(r.cfg.Ctx, "Stopped subscription: %v", err.Error())
				return
			}
		}
	}()
}
