package consul

import (
	"context"

	"github.com/netcracker/qubership-core-lib-go/v3/security/tokensource"
)

type loginCredentials interface {
	AuthMethod() string
	BearerToken(ctx context.Context) (string, error)
}

type kubernetesCredentials struct {
	authMethod string
	audience   string
}

func (c kubernetesCredentials) AuthMethod() string {
	return c.authMethod
}

func (c kubernetesCredentials) BearerToken(ctx context.Context) (string, error) {
	return tokensource.GetAudienceToken(ctx, tokensource.TokenAudience(c.audience))
}
