package consul

import (
	"context"
	"fmt"
	"time"

	consulApi "github.com/hashicorp/consul/api"
)

type consulToken struct {
	secretID       string
	expirationTime *time.Time
	authMethod     string
}

type tokenProvider interface {
	GetToken(ctx context.Context) (*consulToken, error)
}

type loginTokenProvider struct {
	credentials loginCredentials
	acl         func() *consulApi.ACL
}

func (p *loginTokenProvider) GetToken(ctx context.Context) (*consulToken, error) {
	authMethod := p.credentials.AuthMethod()
	bearer, err := p.credentials.BearerToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get bearer token for auth method '%s': %w", authMethod, err)
	}
	if bearer == "" {
		return &consulToken{}, nil
	}

	aclToken, _, err := p.acl().Login(&consulApi.ACLLoginParams{
		AuthMethod:  authMethod,
		BearerToken: bearer,
	}, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to log in to Consul with auth method '%s': %w", authMethod, err)
	}

	return &consulToken{
		secretID:       aclToken.SecretID,
		expirationTime: aclToken.ExpirationTime,
		authMethod:     aclToken.AuthMethod,
	}, nil
}
