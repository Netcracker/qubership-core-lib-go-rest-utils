package consul

import (
	"context"
	"time"
)

const refreshRatio = 0.8

var minRefreshDelay = 10 * time.Second

func refreshDelay(expiration *time.Time, now time.Time) (time.Duration, bool) {
	if expiration == nil {
		return 0, false
	}
	delay := time.Duration(float64(expiration.Sub(now)) * refreshRatio)
	if delay < minRefreshDelay {
		delay = minRefreshDelay
	}
	return delay, true
}

type tokenUpdater struct {
	provider tokenProvider
	apply    func(*consulToken)
	now      func() time.Time
}

func (u *tokenUpdater) start(ctx context.Context, first *consulToken) {
	delay, scheduled := refreshDelay(first.expirationTime, u.now())
	if !scheduled {
		return
	}
	go func() {
		current := first
		timer := time.NewTimer(delay)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
				token, err := u.provider.GetToken(ctx)
				if err != nil {
					retryDelay, retryScheduled := refreshDelay(current.expirationTime, u.now())
					if !retryScheduled {
						retryDelay = minRefreshDelay
					}
					logger.ErrorC(ctx, "failed to refresh Consul token: %s. Next attempt in %s", err.Error(), retryDelay)
					timer.Reset(retryDelay)
					continue
				}
				current = token
				u.apply(token)
				nextDelay, nextScheduled := refreshDelay(token.expirationTime, u.now())
				if !nextScheduled {
					return
				}
				timer.Reset(nextDelay)
			}
		}
	}()
}
