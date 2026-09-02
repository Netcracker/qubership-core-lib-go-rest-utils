package consul

import (
	"context"
	"fmt"

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
	token, err := tokensource.GetAudienceToken(ctx, tokensource.TokenAudience(c.audience))
	if err != nil {
		return "", err
	}
	// An empty bearer token means anonymous access, which the kubernetes auth method never grants.
	if token == "" {
		return "", fmt.Errorf("projected volume token for audience '%s' is empty", c.audience)
	}
	return token, nil
}
