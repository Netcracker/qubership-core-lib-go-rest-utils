package consul

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
)

func TestClient_subscribeFor(t *testing.T) {
	try := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		try++
		if try == 1 {
			res.WriteHeader(http.StatusInternalServerError) // must retry on error
		}
		res.WriteHeader(http.StatusOK)
		p := make([]*api.KVPair, 1)
		p[0] = &api.KVPair{
			Key:         "test-key",
			CreateIndex: 0,
			Value:       []byte("test-value"),
		}
		resp, err := json.Marshal(p)
		assert.NoError(t, err)
		res.Write(resp)
	}))
	defer func() { testServer.Close() }()

	ctx, done := context.WithCancel(context.Background())
	client := NewClient(ClientConfig{
		Address:   testServer.URL,
		Namespace: "test",
		Ctx:       ctx,
	})

	client.token = &ClientToken{}
	client.token.val.Store("test-token")
	var wg sync.WaitGroup
	ch := make(chan map[string]interface{})
	client.subscribeFor("/", 1, func(event interface{}, err error) {
		ch <- map[string]interface{}{"test-key": "test-value"}
	})
	val := <-ch
	assert.Equal(t, "test-value", val["test-key"])
	done()
	assert.Eventuallyf(t, func() bool { wg.Wait(); return true }, 5*time.Second, 100*time.Millisecond, "must stop on done")
}

func TestClient_subscribeFor_no_path(t *testing.T) {
	callCount := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		callCount++
		res.WriteHeader(http.StatusNotFound)
	}))
	defer func() { testServer.Close() }()

	ctx, done := context.WithCancel(context.Background())
	defer done()
	client := NewClient(ClientConfig{
		Address:   testServer.URL,
		Namespace: "test",
		Ctx:       ctx,
	})

	client.token = &ClientToken{}
	client.token.val.Store("test-token")
	var wg sync.WaitGroup
	wg.Add(1)
	client.subscribeFor("/", 1, func(event interface{}, err error) {})
	go func() {
		time.Sleep(time.Second)
		wg.Done()
	}()
	wg.Wait()
	assert.NotZero(t, callCount)
	assert.LessOrEqual(t, callCount, 1)
}

func TestClient_Login(t *testing.T) {
	testSecretId := "anonymous"
	initEnvConfigloader()
	timeStr := time.Now().Add(5 * time.Minute).Format(time.RFC3339)
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		res.Write([]byte("{\"SecretID\": \"" + testSecretId + "\", \"ExpirationTime\": \"" + timeStr + "\"}"))
	}))
	defer func() { testServer.Close() }()
	client := NewClient(ClientConfig{
		Address:   testServer.URL,
		Namespace: "test",
		Ctx:       context.Background(),
		Mode:      AuthModeM2M,
	})
	assert.Nil(t, client.token)
	err := client.Login()
	assert.NoError(t, err)
	assert.Equal(t, timeStr, client.token.expirationTime.Format(time.RFC3339))
	assert.Equal(t, testSecretId, client.token.val.Load())
}

func TestClient_LoginUsesNamespaceAsM2MAuthMethod(t *testing.T) {
	initEnvConfigloader()
	requestedAuthMethod := ""
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		var params api.ACLLoginParams
		assert.NoError(t, json.NewDecoder(req.Body).Decode(&params))
		requestedAuthMethod = params.AuthMethod
		res.WriteHeader(http.StatusOK)
		res.Write([]byte("{\"SecretID\": \"test-secretId\", \"AuthMethod\": \"" + params.AuthMethod + "\"}"))
	}))
	defer func() { testServer.Close() }()
	client := NewClient(ClientConfig{
		Address:   testServer.URL,
		Namespace: "test-namespace",
		Ctx:       context.Background(),
		Mode:      AuthModeM2M,
	})

	err := client.Login()

	assert.NoError(t, err)
	assert.Equal(t, "test-namespace", requestedAuthMethod)
	assert.Equal(t, "test-secretId", client.SecretId())
}

func TestClient_LoginError(t *testing.T) {
	initEnvConfigloader()
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusForbidden)
		res.Write([]byte("Permission denied"))
	}))
	defer func() { testServer.Close() }()
	client := NewClient(ClientConfig{
		Address:   testServer.URL,
		Namespace: "test",
		Ctx:       context.Background(),
		Mode:      AuthModeM2M,
	})
	assert.Nil(t, client.token)
	err := client.Login()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to log in to Consul with auth method 'test'")
	assert.Contains(t, err.Error(), "Unexpected response code: 403")
	assert.Contains(t, err.Error(), "Permission denied")
}

func TestClient_LoginConfigError(t *testing.T) {
	initEnvConfigloader()
	client := NewClient(ClientConfig{
		Address:   "test:8500",
		Namespace: "test",
		Ctx:       context.Background(),
		Mode:      "unknown-mode",
	})

	err := client.Login()

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "consul.auth.mode")
	assert.Nil(t, client.token)
	assert.Equal(t, err, client.Login())
}

func TestClient_LoginEmptyToken(t *testing.T) {
	initEnvConfigloader()

	client := NewClient(ClientConfig{
		Address:   "test:8500",
		Namespace: "test",
		Ctx:       context.Background(),
		Mode:      AuthModeM2M,
		tokenProvider: func() (string, error) {
			return "", nil
		},
	})
	assert.Nil(t, client.token)
	err := client.Login()
	assert.NoError(t, err)
	assert.Nil(t, client.token.val.Load())
}

func assertKubernetesCredentials(t *testing.T, provider tokenProvider, authMethod, audience string) {
	login, isLogin := provider.(*loginTokenProvider)
	if !assert.True(t, isLogin) {
		return
	}
	credentials, isKubernetes := login.credentials.(kubernetesCredentials)
	if !assert.True(t, isKubernetes) {
		return
	}
	assert.Equal(t, authMethod, credentials.authMethod)
	assert.Equal(t, audience, credentials.audience)
}

func assertM2MCredentials(t *testing.T, provider tokenProvider, authMethod string) {
	login, isLogin := provider.(*loginTokenProvider)
	if !assert.True(t, isLogin) {
		return
	}
	credentials, isM2M := login.credentials.(m2mCredentials)
	if !assert.True(t, isM2M) {
		return
	}
	assert.Equal(t, authMethod, credentials.authMethod)
}

func TestClient_TokenProviderPerMode(t *testing.T) {
	initEnvConfigloader()

	fallbackClient := NewClient(ClientConfig{Namespace: "test-namespace", Ctx: context.Background()})
	assert.NoError(t, fallbackClient.initTokenProvider())
	fallback, isFallback := fallbackClient.provider.(*fallbackTokenProvider)
	if assert.True(t, isFallback) {
		assert.Equal(t, "applications-k8s-m2m", fallback.primaryAuthMethod)
		assert.Equal(t, "test-namespace", fallback.secondaryAuthMethod)
		assert.Equal(t, 5*time.Hour, fallback.interval)
		assertKubernetesCredentials(t, fallback.primary, "applications-k8s-m2m", "netcracker")
		assertM2MCredentials(t, fallback.secondary, "test-namespace")
	}

	kubernetesClient := NewClient(ClientConfig{
		Namespace:  "test-namespace",
		Ctx:        context.Background(),
		Mode:       AuthModeKubernetes,
		AuthMethod: "k8s-method",
		Audience:   "k8s-audience",
	})
	assert.NoError(t, kubernetesClient.initTokenProvider())
	assertKubernetesCredentials(t, kubernetesClient.provider, "k8s-method", "k8s-audience")

	m2mClient := NewClient(ClientConfig{
		Namespace: "test-namespace",
		Ctx:       context.Background(),
		Mode:      AuthModeM2M,
	})
	assert.NoError(t, m2mClient.initTokenProvider())
	assertM2MCredentials(t, m2mClient.provider, "test-namespace")
}

func TestClient_DefaultsContext(t *testing.T) {
	assert.NotNil(t, NewClient(ClientConfig{Address: "test:8500"}).cfg.Ctx)
}

func TestClient_RefreshedTokenReachesSecretId(t *testing.T) {
	withShortMinRefreshDelay(t)
	initEnvConfigloader()
	logins := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		logins++
		expirationTime := time.Now().Add(time.Millisecond).Format(time.RFC3339)
		if logins > 1 {
			expirationTime = time.Now().Add(time.Hour).Format(time.RFC3339)
		}
		res.WriteHeader(http.StatusOK)
		res.Write([]byte("{\"SecretID\": \"secret-" + strconv.Itoa(logins) + "\", \"ExpirationTime\": \"" + expirationTime + "\"}"))
	}))
	defer func() { testServer.Close() }()
	ctx, done := context.WithCancel(context.Background())
	defer done()
	client := NewClient(ClientConfig{
		Address:   testServer.URL,
		Namespace: "test",
		Ctx:       ctx,
		Mode:      AuthModeM2M,
	})

	assert.NoError(t, client.Login())
	assert.Equal(t, "secret-1", client.SecretId())

	assert.Eventually(t, func() bool { return client.SecretId() == "secret-2" }, 5*time.Second, 20*time.Millisecond)
}
