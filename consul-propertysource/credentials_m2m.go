package consul

import (
	"context"

	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
)

type m2mCredentials struct {
	authMethod string
	provider   func(ctx context.Context) (string, error)
}

func (c m2mCredentials) AuthMethod() string {
	return c.authMethod
}

func (c m2mCredentials) BearerToken(ctx context.Context) (string, error) {
	if c.provider != nil {
		return c.provider(ctx)
	}
	return serviceloader.MustLoad[security.TokenProvider]().GetToken(ctx)
}
