package consul

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type stubTokenProvider struct {
	calls int
	token *consulToken
	err   error
}

func (p *stubTokenProvider) GetToken(ctx context.Context) (*consulToken, error) {
	p.calls++
	return p.token, p.err
}

func captureStdout(t *testing.T, action func()) string {
	original := os.Stdout
	reader, writer, err := os.Pipe()
	assert.NoError(t, err)
	os.Stdout = writer
	action()
	os.Stdout = original
	assert.NoError(t, writer.Close())
	out, err := io.ReadAll(reader)
	assert.NoError(t, err)
	assert.NoError(t, reader.Close())
	return string(out)
}

func newFallbackForTest(primary, secondary tokenProvider, now func() time.Time) *fallbackTokenProvider {
	return &fallbackTokenProvider{
		primary:             primary,
		secondary:           secondary,
		primaryAuthMethod:   "k8s-method",
		secondaryAuthMethod: "test-namespace",
		interval:            time.Hour,
		now:                 now,
	}
}

func TestFallbackTokenProvider_KeepsSecondaryUntilIntervalElapses(t *testing.T) {
	primary := &stubTokenProvider{err: errors.New("login failed")}
	secondary := &stubTokenProvider{token: &consulToken{secretID: "m2m-secret"}}
	moment := time.Now()
	provider := newFallbackForTest(primary, secondary, func() time.Time { return moment })

	for i := 0; i < 3; i++ {
		token, err := provider.GetToken(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "m2m-secret", token.secretID)
	}

	assert.Equal(t, 1, primary.calls)
	assert.Equal(t, 3, secondary.calls)
	assert.Equal(t, moment, provider.lastProbe)
}

func TestFallbackTokenProvider_ProbesAgainAfterInterval(t *testing.T) {
	primary := &stubTokenProvider{err: errors.New("login failed")}
	secondary := &stubTokenProvider{token: &consulToken{secretID: "m2m-secret"}}
	moment := time.Now()
	provider := newFallbackForTest(primary, secondary, func() time.Time { return moment })

	_, err := provider.GetToken(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, 1, primary.calls)

	moment = moment.Add(provider.interval)
	_, err = provider.GetToken(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, 2, primary.calls)
	assert.Equal(t, 2, secondary.calls)
	assert.Equal(t, moment, provider.lastProbe)
}

func TestFallbackTokenProvider_SwitchesPermanentlyOnSuccess(t *testing.T) {
	primary := &stubTokenProvider{token: &consulToken{secretID: "k8s-secret", authMethod: "k8s-method"}}
	secondary := &stubTokenProvider{token: &consulToken{secretID: "m2m-secret"}}
	moment := time.Now()
	provider := newFallbackForTest(primary, secondary, func() time.Time { return moment })

	token, err := provider.GetToken(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "k8s-secret", token.secretID)
	assert.True(t, provider.switched)

	for i := 0; i < 2; i++ {
		token, err = provider.GetToken(context.Background())
		assert.NoError(t, err)
		assert.Equal(t, "k8s-secret", token.secretID)
	}

	assert.Equal(t, 3, primary.calls)
	assert.Zero(t, secondary.calls)
}

func TestFallbackTokenProvider_ReturnsSecondaryError(t *testing.T) {
	primary := &stubTokenProvider{err: errors.New("login failed")}
	secondaryErr := errors.New("m2m token is not available")
	secondary := &stubTokenProvider{err: secondaryErr}
	moment := time.Now()
	provider := newFallbackForTest(primary, secondary, func() time.Time { return moment })

	token, err := provider.GetToken(context.Background())

	assert.Nil(t, token)
	assert.Equal(t, secondaryErr, err)
}

func TestFallbackTokenProvider_LogsFallbackOnce(t *testing.T) {
	primary := &stubTokenProvider{err: errors.New("login failed")}
	secondary := &stubTokenProvider{token: &consulToken{secretID: "m2m-secret"}}
	moment := time.Now()
	provider := newFallbackForTest(primary, secondary, func() time.Time { return moment })

	out := captureStdout(t, func() {
		for i := 0; i < 3; i++ {
			moment = moment.Add(provider.interval)
			_, err := provider.GetToken(context.Background())
			assert.NoError(t, err)
		}
	})

	assert.Equal(t, 3, primary.calls)
	assert.Equal(t, 1, strings.Count(out, "Falling back to auth method 'test-namespace'"))
	assert.Contains(t, out, "Consul login with auth method 'k8s-method' failed")
}

func TestFallbackTokenProvider_LogsSwitch(t *testing.T) {
	primary := &stubTokenProvider{err: errors.New("login failed")}
	secondary := &stubTokenProvider{token: &consulToken{secretID: "m2m-secret"}}
	moment := time.Now()
	provider := newFallbackForTest(primary, secondary, func() time.Time { return moment })

	out := captureStdout(t, func() {
		_, err := provider.GetToken(context.Background())
		assert.NoError(t, err)

		primary.err = nil
		primary.token = &consulToken{secretID: "k8s-secret", authMethod: "k8s-method"}
		moment = moment.Add(provider.interval)
		_, err = provider.GetToken(context.Background())
		assert.NoError(t, err)
	})

	assert.Equal(t, 1, strings.Count(out, "Consul login with auth method 'k8s-method' succeeded. Fallback disabled"))
}

func TestFallbackTokenProvider_SilentWhenPrimarySucceedsFirst(t *testing.T) {
	primary := &stubTokenProvider{token: &consulToken{secretID: "k8s-secret", authMethod: "k8s-method"}}
	secondary := &stubTokenProvider{token: &consulToken{secretID: "m2m-secret"}}
	moment := time.Now()
	provider := newFallbackForTest(primary, secondary, func() time.Time { return moment })

	out := captureStdout(t, func() {
		_, err := provider.GetToken(context.Background())
		assert.NoError(t, err)
	})

	assert.True(t, provider.switched)
	assert.NotContains(t, out, "Fallback disabled")
}
