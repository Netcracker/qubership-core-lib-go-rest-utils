package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/netcracker/qubership-core-lib-go/v3/const"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	restclient "github.com/netcracker/qubership-core-lib-go/v3/security/rest"
)

var log logging.Logger

func init() {
	log = logging.GetLogger("routemanagement")
}

type ControlPlaneClient struct {
	ctx context.Context
	cancel context.CancelFunc
	controlPlaneAddr string
	retryManager     *RetryManager
	restClient       restclient.Client
}

func NewControlPlaneClient(controlPlaneAddr string, retryManager *RetryManager) *ControlPlaneClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &ControlPlaneClient{ctx: ctx, cancel: cancel, controlPlaneAddr: formatAddr(controlPlaneAddr), retryManager: retryManager, restClient: restclient.NewM2MRestClient()}
}

func formatAddr(addr string) string {
	for strings.HasSuffix(addr, "/") {
		addr = addr[0 : len(addr)-1]
	}
	log.Debugf("Control plane addr is %v", addr)
	if strings.HasPrefix(addr, "http://") || strings.HasPrefix(addr, "https://") {
		return addr
	}
	return constants.SelectUrl("http://"+addr, "https://"+addr)
}

func (client *ControlPlaneClient) getApiUrl(request RegistrationRequest) (string, error) {
	switch request.ApiVersion() {
	case V3:
		return fmt.Sprintf("%s/api/v3/routes", client.controlPlaneAddr), nil
	default:
		errorMsg := fmt.Sprintf("control plane api version is not supported: %v", request.ApiVersion())
		log.Errorf("%s", errorMsg)
		return "", errors.New(errorMsg)
	}
}

func (client *ControlPlaneClient) SendRequest(request RegistrationRequest) {
	url, err := client.getApiUrl(request)
	if err != nil {
		log.Panicf("Failed to resolve api version: %+v", err)
	}
	payload, err := json.Marshal(request.Payload())
	if err != nil {
		log.Panicf("Failed to marshall route registration request to JSON: %+v", err)
	}

	client.sendRequestWithRetry(client.ctx, url, payload)
}

func (client *ControlPlaneClient) sendRequestWithRetry(ctx context.Context, url string, payload []byte) {
	client.retryManager.DoWithRetry(func() error {
		resp, err := client.restClient.DoRequest(ctx, "POST", url, map[string][]string{
			"Content-Type": []string{"application/json"},
		}, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			return errors.New(fmt.Sprintf("got error status code in route registration response: %d", resp.StatusCode))
		}
		return nil
	})
}

func (client *ControlPlaneClient) Close() {
	client.cancel()
}

type RetryManager struct {
	progressiveTimeout *ProgressiveTimeout
}

func NewRetryManager(progressiveTimeout *ProgressiveTimeout) *RetryManager {
	return &RetryManager{progressiveTimeout: progressiveTimeout}
}

func (rm *RetryManager) DoWithRetry(action func() error) {
	defer func() {
		if r := recover(); r != nil {
			log.Debug("Can't connect to control plane, retry")
			time.Sleep(rm.progressiveTimeout.NextTimeoutValue())
			rm.DoWithRetry(action)
		}
	}()
	if err := action(); err != nil {
		log.Panicf("Action failed with error: %+v", err)
	}
	rm.progressiveTimeout.Reset()
}

type ProgressiveTimeout struct {
	baseTimeout            time.Duration
	startMultiplier        int
	endMultiplier          int
	multiplierStep         int
	currentMultiplierValue int
	maxTimeoutValue        time.Duration

	mutex *sync.Mutex
}

func NewProgressiveTimeout(baseTimeout time.Duration, startMultiplier int, endMultiplier int, multiplierStep int) *ProgressiveTimeout {
	if endMultiplier <= startMultiplier {
		log.Panic("EndMultiplier must be greater than startMultiplier in ProgressiveTimeout")
	}
	return &ProgressiveTimeout{
		baseTimeout:            baseTimeout,
		startMultiplier:        startMultiplier,
		endMultiplier:          endMultiplier,
		multiplierStep:         multiplierStep,
		currentMultiplierValue: startMultiplier,
		maxTimeoutValue:        time.Duration(int64(endMultiplier) * baseTimeout.Nanoseconds()),
		mutex:                  &sync.Mutex{}}
}

func (pt *ProgressiveTimeout) NextTimeoutValue() time.Duration {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	if pt.currentMultiplierValue >= pt.endMultiplier {
		return pt.maxTimeoutValue
	}

	result := time.Duration(int64(pt.currentMultiplierValue) * pt.baseTimeout.Nanoseconds())
	pt.currentMultiplierValue += pt.multiplierStep
	return result
}

func (pt *ProgressiveTimeout) Reset() {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	pt.currentMultiplierValue = pt.startMultiplier
}
