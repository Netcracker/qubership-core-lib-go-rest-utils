package consul

import (
	"testing"
	"time"

	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/stretchr/testify/assert"
)

func initEnvConfigloader() {
	configloader.InitWithSourcesArray([]*configloader.PropertySource{configloader.EnvPropertySource()})
}

func TestResolveAuthConfig_Defaults(t *testing.T) {
	initEnvConfigloader()

	cfg, err := resolveAuthConfig(ClientConfig{})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeKubernetesWithM2MFallback, cfg.mode)
	assert.Equal(t, "applications-k8s-m2m", cfg.authMethod)
	assert.Equal(t, "netcracker", cfg.audience)
	assert.Equal(t, 5*time.Hour, cfg.recheckInterval)
}

func TestResolveAuthConfig_PropertyOverridesDefault(t *testing.T) {
	t.Setenv("CONSUL_AUTH_MODE", "kubernetes")
	t.Setenv("CONSUL_AUTH_METHOD", "property-method")
	t.Setenv("CONSUL_AUTH_AUDIENCE", "property-audience")
	t.Setenv("CONSUL_AUTH_FALLBACK_RECHECK_INTERVAL", "30m")
	initEnvConfigloader()

	cfg, err := resolveAuthConfig(ClientConfig{})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeKubernetes, cfg.mode)
	assert.Equal(t, "property-method", cfg.authMethod)
	assert.Equal(t, "property-audience", cfg.audience)
	assert.Equal(t, 30*time.Minute, cfg.recheckInterval)
}

func TestResolveAuthConfig_FieldOverridesProperty(t *testing.T) {
	t.Setenv("CONSUL_AUTH_MODE", "kubernetes")
	t.Setenv("CONSUL_AUTH_METHOD", "property-method")
	t.Setenv("CONSUL_AUTH_AUDIENCE", "property-audience")
	t.Setenv("CONSUL_AUTH_FALLBACK_RECHECK_INTERVAL", "30m")
	initEnvConfigloader()

	cfg, err := resolveAuthConfig(ClientConfig{
		Mode:                    AuthModeM2M,
		AuthMethod:              "field-method",
		Audience:                "field-audience",
		FallbackRecheckInterval: time.Minute,
	})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeM2M, cfg.mode)
	assert.Equal(t, "field-method", cfg.authMethod)
	assert.Equal(t, "field-audience", cfg.audience)
	assert.Equal(t, time.Minute, cfg.recheckInterval)
}

func TestResolveAuthConfig_UnknownMode(t *testing.T) {
	initEnvConfigloader()

	_, err := resolveAuthConfig(ClientConfig{Mode: "unknown-mode"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "consul.auth.mode")
	assert.Contains(t, err.Error(), "unknown-mode")
}

func TestResolveAuthConfig_UnparsableInterval(t *testing.T) {
	t.Setenv("CONSUL_AUTH_FALLBACK_RECHECK_INTERVAL", "5 hours")
	initEnvConfigloader()

	_, err := resolveAuthConfig(ClientConfig{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "consul.auth.fallback.recheck.interval")
	assert.Contains(t, err.Error(), "5 hours")
}
