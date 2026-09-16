package consul

import (
	"context"
	"errors"
	"sync"
	"time"
)

const probeTries = 3

var probePause = time.Second

type fallbackTokenProvider struct {
	primary             tokenProvider
	secondary           tokenProvider
	primaryAuthMethod   string
	secondaryAuthMethod string
	interval            time.Duration
	now                 func() time.Time

	mu             sync.Mutex
	switched       bool
	lastProbe      time.Time
	fallbackLogged bool
}

func (p *fallbackTokenProvider) GetToken(ctx context.Context) (*consulToken, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.switched {
		return p.primary.GetToken(ctx)
	}

	if p.probeDue() {
		token, err := p.probe(ctx)
		if err == nil {
			p.switched = true
			if p.fallbackLogged {
				logger.InfoC(ctx, "Consul login with auth method '%s' succeeded. Fallback disabled", p.primaryAuthMethod)
			}
			return token, nil
		}
		p.lastProbe = p.now()
		if !p.fallbackLogged {
			p.fallbackLogged = true
			logger.InfoC(ctx, "Consul login with auth method '%s' failed: %s. Falling back to auth method '%s'",
				p.primaryAuthMethod, err.Error(), p.secondaryAuthMethod)
		}
	}

	return p.secondary.GetToken(ctx)
}

func (p *fallbackTokenProvider) probe(ctx context.Context) (*consulToken, error) {
	for attempt := 1; ; attempt++ {
		token, err := p.primary.GetToken(ctx)
		if err == nil || attempt == probeTries || !errors.Is(err, errConsulLogin) {
			return token, err
		}
		select {
		case <-ctx.Done():
			return nil, err
		case <-time.After(probePause):
		}
	}
}

func (p *fallbackTokenProvider) probeDue() bool {
	return p.lastProbe.IsZero() || p.now().Sub(p.lastProbe) >= p.interval
}
