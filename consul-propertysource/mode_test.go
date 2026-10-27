package consul

import (
	"os"
	"testing"
	"time"

	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/stretchr/testify/assert"
)

// TestMain removes M2M_AUTH_MODE, which the auth mode falls back to, so that the machine running the tests does not
// pick the mode of every test that leaves the mode unset. A test that needs the variable sets it with t.Setenv.
func TestMain(m *testing.M) {
	_ = os.Unsetenv("M2M_AUTH_MODE")
	os.Exit(m.Run())
}

func initEnvConfigloader() {
	configloader.InitWithSourcesArray([]*configloader.PropertySource{configloader.EnvPropertySource()})
}

func TestResolveAuthConfig_Defaults(t *testing.T) {
	t.Setenv("M2M_AUTH_MODE", "")
	initEnvConfigloader()

	cfg, err := resolveAuthConfig(ClientConfig{})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeLegacy, cfg.mode)
	assert.Equal(t, "applications-k8s-m2m", cfg.authMethod)
	assert.Equal(t, "netcracker", cfg.audience)
	assert.Equal(t, 5*time.Hour, cfg.recheckInterval)
}

func TestResolveAuthConfig_PropertyOverridesDefault(t *testing.T) {
	t.Setenv("CONSUL_AUTH_MODE", "k8s")
	t.Setenv("CONSUL_AUTH_METHOD", "property-method")
	t.Setenv("CONSUL_AUTH_AUDIENCE", "property-audience")
	t.Setenv("CONSUL_AUTH_FALLBACK_RECHECK_INTERVAL", "30m")
	initEnvConfigloader()

	cfg, err := resolveAuthConfig(ClientConfig{})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeK8s, cfg.mode)
	assert.Equal(t, "property-method", cfg.authMethod)
	assert.Equal(t, "property-audience", cfg.audience)
	assert.Equal(t, 30*time.Minute, cfg.recheckInterval)
}

func TestResolveAuthConfig_FieldOverridesProperty(t *testing.T) {
	t.Setenv("CONSUL_AUTH_MODE", "k8s")
	t.Setenv("CONSUL_AUTH_METHOD", "property-method")
	t.Setenv("CONSUL_AUTH_AUDIENCE", "property-audience")
	t.Setenv("CONSUL_AUTH_FALLBACK_RECHECK_INTERVAL", "30m")
	initEnvConfigloader()

	cfg, err := resolveAuthConfig(ClientConfig{
		Mode:                    AuthModeLegacy,
		AuthMethod:              "field-method",
		Audience:                "field-audience",
		FallbackRecheckInterval: time.Minute,
	})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeLegacy, cfg.mode)
	assert.Equal(t, "field-method", cfg.authMethod)
	assert.Equal(t, "field-audience", cfg.audience)
	assert.Equal(t, time.Minute, cfg.recheckInterval)
}

func TestResolveAuthConfig_ModeTakenFromM2MAuthMode(t *testing.T) {
	for m2mMode, expected := range map[string]AuthMode{
		"legacy": AuthModeLegacy,
		"hybrid": AuthModeHybrid,
		"k8s":    AuthModeK8s,
	} {
		t.Run(m2mMode, func(t *testing.T) {
			t.Setenv("M2M_AUTH_MODE", m2mMode)
			initEnvConfigloader()

			cfg, err := resolveAuthConfig(ClientConfig{})

			assert.NoError(t, err)
			assert.Equal(t, expected, cfg.mode)
		})
	}
}

func TestResolveAuthConfig_EmptyPropertyTakesM2MAuthMode(t *testing.T) {
	t.Setenv("CONSUL_AUTH_MODE", "")
	t.Setenv("M2M_AUTH_MODE", "hybrid")
	initEnvConfigloader()

	cfg, err := resolveAuthConfig(ClientConfig{})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeHybrid, cfg.mode)
}

func TestResolveAuthConfig_PropertyOverridesM2MAuthMode(t *testing.T) {
	t.Setenv("CONSUL_AUTH_MODE", "legacy")
	t.Setenv("M2M_AUTH_MODE", "k8s")
	initEnvConfigloader()

	cfg, err := resolveAuthConfig(ClientConfig{})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeLegacy, cfg.mode)
}

func TestResolveAuthConfig_FieldOverridesM2MAuthMode(t *testing.T) {
	t.Setenv("M2M_AUTH_MODE", "k8s")
	initEnvConfigloader()

	cfg, err := resolveAuthConfig(ClientConfig{Mode: AuthModeHybrid})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeHybrid, cfg.mode)
}

func TestResolveAuthConfig_UnsupportedM2MAuthMode(t *testing.T) {
	t.Setenv("M2M_AUTH_MODE", "cloud-foundry")
	initEnvConfigloader()

	_, err := resolveAuthConfig(ClientConfig{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "consul.auth.mode")
	assert.Contains(t, err.Error(), "M2M_AUTH_MODE")
	assert.Contains(t, err.Error(), "cloud-foundry")
}

func TestResolveAuthConfig_UnsupportedM2MAuthModeIgnoredWhenPropertyIsSet(t *testing.T) {
	t.Setenv("CONSUL_AUTH_MODE", "k8s")
	t.Setenv("M2M_AUTH_MODE", "cloud-foundry")
	initEnvConfigloader()

	cfg, err := resolveAuthConfig(ClientConfig{})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeK8s, cfg.mode)
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
