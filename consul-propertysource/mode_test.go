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

func TestResolveAuthSettings_Defaults(t *testing.T) {
	t.Setenv("M2M_AUTH_MODE", "")
	initEnvConfigloader()

	cfg, err := resolveAuthSettings(AuthSettings{})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeLegacy, cfg.Mode)
	assert.Equal(t, "applications-k8s-m2m", cfg.AuthMethod)
	assert.Equal(t, "netcracker", cfg.Audience)
	assert.Equal(t, 5*time.Hour, cfg.FallbackRecheckInterval)
}

func TestResolveAuthSettings_PropertyOverridesDefault(t *testing.T) {
	t.Setenv("CONSUL_AUTH_MODE", "k8s")
	t.Setenv("CONSUL_AUTH_METHOD", "property-method")
	t.Setenv("CONSUL_AUTH_AUDIENCE", "property-audience")
	t.Setenv("CONSUL_AUTH_FALLBACK_RECHECK_INTERVAL", "30m")
	initEnvConfigloader()

	cfg, err := resolveAuthSettings(AuthSettings{})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeK8s, cfg.Mode)
	assert.Equal(t, "property-method", cfg.AuthMethod)
	assert.Equal(t, "property-audience", cfg.Audience)
	assert.Equal(t, 30*time.Minute, cfg.FallbackRecheckInterval)
}

func TestResolveAuthSettings_FieldOverridesProperty(t *testing.T) {
	t.Setenv("CONSUL_AUTH_MODE", "k8s")
	t.Setenv("CONSUL_AUTH_METHOD", "property-method")
	t.Setenv("CONSUL_AUTH_AUDIENCE", "property-audience")
	t.Setenv("CONSUL_AUTH_FALLBACK_RECHECK_INTERVAL", "30m")
	initEnvConfigloader()

	cfg, err := resolveAuthSettings(AuthSettings{
		Mode:                    AuthModeLegacy,
		AuthMethod:              "field-method",
		Audience:                "field-audience",
		FallbackRecheckInterval: time.Minute,
	})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeLegacy, cfg.Mode)
	assert.Equal(t, "field-method", cfg.AuthMethod)
	assert.Equal(t, "field-audience", cfg.Audience)
	assert.Equal(t, time.Minute, cfg.FallbackRecheckInterval)
}

func TestResolveAuthSettings_ModeTakenFromM2MAuthMode(t *testing.T) {
	for m2mMode, expected := range map[string]AuthMode{
		"legacy": AuthModeLegacy,
		"hybrid": AuthModeHybrid,
		"k8s":    AuthModeK8s,
	} {
		t.Run(m2mMode, func(t *testing.T) {
			t.Setenv("M2M_AUTH_MODE", m2mMode)
			initEnvConfigloader()

			cfg, err := resolveAuthSettings(AuthSettings{})

			assert.NoError(t, err)
			assert.Equal(t, expected, cfg.Mode)
		})
	}
}

func TestResolveAuthSettings_EmptyPropertyTakesM2MAuthMode(t *testing.T) {
	t.Setenv("CONSUL_AUTH_MODE", "")
	t.Setenv("M2M_AUTH_MODE", "hybrid")
	initEnvConfigloader()

	cfg, err := resolveAuthSettings(AuthSettings{})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeHybrid, cfg.Mode)
}

func TestResolveAuthSettings_PropertyOverridesM2MAuthMode(t *testing.T) {
	t.Setenv("CONSUL_AUTH_MODE", "legacy")
	t.Setenv("M2M_AUTH_MODE", "k8s")
	initEnvConfigloader()

	cfg, err := resolveAuthSettings(AuthSettings{})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeLegacy, cfg.Mode)
}

func TestResolveAuthSettings_FieldOverridesM2MAuthMode(t *testing.T) {
	t.Setenv("M2M_AUTH_MODE", "k8s")
	initEnvConfigloader()

	cfg, err := resolveAuthSettings(AuthSettings{Mode: AuthModeHybrid})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeHybrid, cfg.Mode)
}

func TestResolveAuthSettings_UnsupportedM2MAuthMode(t *testing.T) {
	t.Setenv("M2M_AUTH_MODE", "cloud-foundry")
	initEnvConfigloader()

	_, err := resolveAuthSettings(AuthSettings{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "consul.auth.mode")
	assert.Contains(t, err.Error(), "M2M_AUTH_MODE")
	assert.Contains(t, err.Error(), "cloud-foundry")
}

func TestResolveAuthSettings_UnsupportedM2MAuthModeIgnoredWhenPropertyIsSet(t *testing.T) {
	t.Setenv("CONSUL_AUTH_MODE", "k8s")
	t.Setenv("M2M_AUTH_MODE", "cloud-foundry")
	initEnvConfigloader()

	cfg, err := resolveAuthSettings(AuthSettings{})

	assert.NoError(t, err)
	assert.Equal(t, AuthModeK8s, cfg.Mode)
}

func TestResolveAuthSettings_UnknownMode(t *testing.T) {
	initEnvConfigloader()

	_, err := resolveAuthSettings(AuthSettings{Mode: "unknown-mode"})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "consul.auth.mode")
	assert.Contains(t, err.Error(), "unknown-mode")
}

func TestResolveAuthSettings_UnparsableInterval(t *testing.T) {
	t.Setenv("CONSUL_AUTH_FALLBACK_RECHECK_INTERVAL", "5 hours")
	initEnvConfigloader()

	_, err := resolveAuthSettings(AuthSettings{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "consul.auth.fallback.recheck.interval")
	assert.Contains(t, err.Error(), "5 hours")
}

func TestReadAuthSettings_Mode(t *testing.T) {
	cases := []struct {
		name           string
		consulAuthMode string
		m2mAuthMode    string
		want           AuthMode
	}{
		{"the property sets the mode", "hybrid", "", AuthModeHybrid},
		{"the property overrides M2M_AUTH_MODE", "legacy", "k8s", AuthModeLegacy},
		{"an empty property takes M2M_AUTH_MODE", "", "k8s", AuthModeK8s},
		{"neither set means legacy", "", "", AuthModeLegacy},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CONSUL_AUTH_MODE", tc.consulAuthMode)
			t.Setenv("M2M_AUTH_MODE", tc.m2mAuthMode)
			initEnvConfigloader()

			settings, err := ReadAuthSettings()

			assert.NoError(t, err)
			assert.Equal(t, tc.want, settings.Mode)
		})
	}
}

func TestReadAuthSettings_MethodAndAudience(t *testing.T) {
	cases := []struct {
		name         string
		method       string
		audience     string
		wantMethod   string
		wantAudience string
	}{
		{"defaults", "", "", "applications-k8s-m2m", "netcracker"},
		{"properties", "custom-method", "custom-audience", "custom-method", "custom-audience"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CONSUL_AUTH_METHOD", tc.method)
			t.Setenv("CONSUL_AUTH_AUDIENCE", tc.audience)
			initEnvConfigloader()

			settings, err := ReadAuthSettings()

			assert.NoError(t, err)
			assert.Equal(t, tc.wantMethod, settings.AuthMethod)
			assert.Equal(t, tc.wantAudience, settings.Audience)
		})
	}
}

func TestReadAuthSettings_RejectsAnUnsupportedMode(t *testing.T) {
	cases := []struct {
		name           string
		consulAuthMode string
		m2mAuthMode    string
	}{
		{"unsupported property", "cloud-foundry", "legacy"},
		{"unsupported M2M_AUTH_MODE with the property unset", "", "cloud-foundry"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("CONSUL_AUTH_MODE", tc.consulAuthMode)
			t.Setenv("M2M_AUTH_MODE", tc.m2mAuthMode)
			initEnvConfigloader()

			_, err := ReadAuthSettings()

			assert.ErrorContains(t, err, "cloud-foundry")
		})
	}
}
