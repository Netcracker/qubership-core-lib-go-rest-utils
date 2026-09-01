package consul

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type tokenResult struct {
	token *consulToken
	err   error
}

type scriptedTokenProvider struct {
	mu      sync.Mutex
	results []tokenResult
	calls   int
}

func (p *scriptedTokenProvider) GetToken(ctx context.Context) (*consulToken, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := p.results[len(p.results)-1]
	if p.calls < len(p.results) {
		result = p.results[p.calls]
	}
	p.calls++
	return result.token, result.err
}

func (p *scriptedTokenProvider) callCount() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.calls
}

func withShortMinRefreshDelay(t *testing.T) {
	original := minRefreshDelay
	minRefreshDelay = 10 * time.Millisecond
	t.Cleanup(func() { minRefreshDelay = original })
}

func expirationAt(moment time.Time, ttl time.Duration) *time.Time {
	expiration := moment.Add(ttl)
	return &expiration
}

func TestRefreshDelay_Formula(t *testing.T) {
	moment := time.Now()

	for _, ttl := range []time.Duration{time.Minute, 15 * time.Minute, time.Hour, 24 * time.Hour} {
		delay, scheduled := refreshDelay(expirationAt(moment, ttl), moment)

		assert.True(t, scheduled)
		assert.Equal(t, time.Duration(float64(ttl)*0.8), delay)
	}
}

func TestRefreshDelay_WithoutExpiration(t *testing.T) {
	delay, scheduled := refreshDelay(nil, time.Now())

	assert.False(t, scheduled)
	assert.Zero(t, delay)
}

func TestRefreshDelay_LowerBound(t *testing.T) {
	moment := time.Now()

	shortDelay, scheduled := refreshDelay(expirationAt(moment, 5*time.Second), moment)
	assert.True(t, scheduled)
	assert.Equal(t, minRefreshDelay, shortDelay)

	expiredDelay, scheduled := refreshDelay(expirationAt(moment, -time.Hour), moment)
	assert.True(t, scheduled)
	assert.Equal(t, minRefreshDelay, expiredDelay)
}

func TestTokenUpdater_SchedulesBySecondTokenExpiration(t *testing.T) {
	withShortMinRefreshDelay(t)
	moment := time.Now()
	second := &consulToken{secretID: "second", expirationTime: expirationAt(moment, time.Hour)}
	provider := &scriptedTokenProvider{results: []tokenResult{{token: second}}}
	applied := make(chan *consulToken, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updater := &tokenUpdater{
		provider: provider,
		apply:    func(token *consulToken) { applied <- token },
		now:      func() time.Time { return moment },
	}
	updater.start(ctx, &consulToken{secretID: "first", expirationTime: expirationAt(moment, time.Millisecond)})

	assert.Equal(t, second, <-applied)
	assert.Never(t, func() bool { return provider.callCount() > 1 }, 300*time.Millisecond, 20*time.Millisecond)
}

func TestTokenUpdater_StopsWhenTokenHasNoExpiration(t *testing.T) {
	withShortMinRefreshDelay(t)
	moment := time.Now()
	provider := &scriptedTokenProvider{results: []tokenResult{{token: &consulToken{secretID: "endless"}}}}
	applied := make(chan *consulToken, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updater := &tokenUpdater{
		provider: provider,
		apply:    func(token *consulToken) { applied <- token },
		now:      func() time.Time { return moment },
	}
	updater.start(ctx, &consulToken{secretID: "first", expirationTime: expirationAt(moment, time.Millisecond)})

	assert.Equal(t, "endless", (<-applied).secretID)
	assert.Never(t, func() bool { return provider.callCount() > 1 }, 300*time.Millisecond, 20*time.Millisecond)
}

func TestTokenUpdater_NotStartedWithoutExpiration(t *testing.T) {
	withShortMinRefreshDelay(t)
	moment := time.Now()
	provider := &scriptedTokenProvider{results: []tokenResult{{token: &consulToken{secretID: "second"}}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updater := &tokenUpdater{
		provider: provider,
		apply:    func(token *consulToken) {},
		now:      func() time.Time { return moment },
	}
	updater.start(ctx, &consulToken{secretID: "first"})

	assert.Never(t, func() bool { return provider.callCount() > 0 }, 300*time.Millisecond, 20*time.Millisecond)
}

func TestTokenUpdater_KeepsScheduleAfterError(t *testing.T) {
	withShortMinRefreshDelay(t)
	moment := time.Now()
	second := &consulToken{secretID: "second", expirationTime: expirationAt(moment, time.Hour)}
	provider := &scriptedTokenProvider{results: []tokenResult{
		{err: errors.New("login failed")},
		{token: second},
	}}
	applied := make(chan *consulToken, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	updater := &tokenUpdater{
		provider: provider,
		apply:    func(token *consulToken) { applied <- token },
		now:      func() time.Time { return moment },
	}
	updater.start(ctx, &consulToken{secretID: "first", expirationTime: expirationAt(moment, time.Millisecond)})

	assert.Equal(t, second, <-applied)
	assert.Equal(t, 2, provider.callCount())
}

func TestTokenUpdater_StopsOnContextDone(t *testing.T) {
	withShortMinRefreshDelay(t)
	moment := time.Now()
	provider := &scriptedTokenProvider{results: []tokenResult{
		{token: &consulToken{secretID: "second", expirationTime: expirationAt(moment, time.Millisecond)}},
	}}
	applied := make(chan *consulToken, 16)
	ctx, cancel := context.WithCancel(context.Background())

	updater := &tokenUpdater{
		provider: provider,
		apply:    func(token *consulToken) { applied <- token },
		now:      func() time.Time { return moment },
	}
	updater.start(ctx, &consulToken{secretID: "first", expirationTime: expirationAt(moment, time.Millisecond)})

	<-applied
	cancel()
	stopped := provider.callCount()

	assert.Eventually(t, func() bool { return provider.callCount() == stopped }, time.Second, 50*time.Millisecond)
}
