package consul

import (
	"context"
	"testing"

	"github.com/netcracker/qubership-core-lib-go/v3/security/tokensource"
	"github.com/stretchr/testify/assert"
)

func TestKubernetesCredentials_AuthMethod(t *testing.T) {
	credentials := kubernetesCredentials{authMethod: "k8s-method", audience: "netcracker"}

	assert.Equal(t, "k8s-method", credentials.AuthMethod())
}

func TestKubernetesCredentials_BearerTokenError(t *testing.T) {
	tokensDir := tokensource.DefaultAudienceTokensDir
	tokensource.DefaultAudienceTokensDir = "absent-audience-tokens-dir"
	defer func() { tokensource.DefaultAudienceTokensDir = tokensDir }()

	credentials := kubernetesCredentials{authMethod: "k8s-method", audience: "netcracker"}
	token, err := credentials.BearerToken(context.Background())

	assert.Error(t, err)
	assert.Empty(t, token)
}

func TestM2MCredentials_AuthMethod(t *testing.T) {
	credentials := m2mCredentials{authMethod: "test-namespace"}

	assert.Equal(t, "test-namespace", credentials.AuthMethod())
}

func TestM2MCredentials_BearerTokenEmpty(t *testing.T) {
	credentials := m2mCredentials{
		authMethod: "test-namespace",
		provider:   func(ctx context.Context) (string, error) { return "", nil },
	}

	token, err := credentials.BearerToken(context.Background())

	assert.NoError(t, err)
	assert.Empty(t, token)
}
