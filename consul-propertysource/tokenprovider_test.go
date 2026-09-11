package consul

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	consulApi "github.com/hashicorp/consul/api"
	"github.com/stretchr/testify/assert"
)

type stubCredentials struct {
	authMethod string
	token      string
	err        error
}

func (c stubCredentials) AuthMethod() string {
	return c.authMethod
}

func (c stubCredentials) BearerToken(ctx context.Context) (string, error) {
	return c.token, c.err
}

func aclOf(address string) func() *consulApi.ACL {
	return func() *consulApi.ACL {
		return newConsulApiClient(address, "").ACL()
	}
}

func TestLoginTokenProvider_GetToken(t *testing.T) {
	expirationTime := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(`{"SecretID": "test-secret", "AuthMethod": "k8s-method", "ExpirationTime": "` +
			expirationTime.Format(time.RFC3339) + `"}`))
	}))
	defer testServer.Close()

	provider := &loginTokenProvider{
		credentials: stubCredentials{authMethod: "k8s-method", token: "bearer"},
		acl:         aclOf(testServer.URL),
	}

	token, err := provider.GetToken(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "test-secret", token.secretID)
	assert.Equal(t, "k8s-method", token.authMethod)
	assert.NotNil(t, token.expirationTime)
	assert.Equal(t, expirationTime, token.expirationTime.UTC())
}

func TestLoginTokenProvider_GetTokenWithoutExpirationTime(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusOK)
		res.Write([]byte(`{"SecretID": "test-secret", "AuthMethod": "k8s-method"}`))
	}))
	defer testServer.Close()

	provider := &loginTokenProvider{
		credentials: stubCredentials{authMethod: "k8s-method", token: "bearer"},
		acl:         aclOf(testServer.URL),
	}

	token, err := provider.GetToken(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "test-secret", token.secretID)
	assert.Nil(t, token.expirationTime)
}

func TestLoginTokenProvider_GetTokenForbidden(t *testing.T) {
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		res.WriteHeader(http.StatusForbidden)
		res.Write([]byte("Permission denied"))
	}))
	defer testServer.Close()

	provider := &loginTokenProvider{
		credentials: stubCredentials{authMethod: "k8s-method", token: "bearer"},
		acl:         aclOf(testServer.URL),
	}

	token, err := provider.GetToken(context.Background())

	assert.Nil(t, token)
	assert.Error(t, err)
	var statusError consulApi.StatusError
	assert.True(t, errors.As(err, &statusError))
	assert.Equal(t, http.StatusForbidden, statusError.Code)
	assert.Contains(t, err.Error(), "k8s-method")
}

func TestLoginTokenProvider_GetTokenCredentialsError(t *testing.T) {
	provider := &loginTokenProvider{
		credentials: stubCredentials{authMethod: "k8s-method", err: errors.New("no token file")},
		acl: func() *consulApi.ACL {
			assert.Fail(t, "must not call Consul when credentials fail")
			return nil
		},
	}

	token, err := provider.GetToken(context.Background())

	assert.Nil(t, token)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "k8s-method")
	assert.Contains(t, err.Error(), "no token file")
}

func TestLoginTokenProvider_GetTokenEmptyBearer(t *testing.T) {
	provider := &loginTokenProvider{
		credentials: stubCredentials{authMethod: "test-namespace", token: ""},
		acl: func() *consulApi.ACL {
			assert.Fail(t, "must not call Consul with an empty bearer token")
			return nil
		},
	}

	token, err := provider.GetToken(context.Background())

	assert.NoError(t, err)
	assert.Empty(t, token.secretID)
	assert.Nil(t, token.expirationTime)
	assert.Empty(t, token.authMethod)
}
