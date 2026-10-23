package consul

import (
	"fmt"
	"time"

	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
)

type AuthMode string

const (
	AuthModeLegacy AuthMode = "legacy"
	AuthModeHybrid AuthMode = "hybrid"
	AuthModeK8s    AuthMode = "k8s"
)

const (
	propAuthMode                = "consul.auth.mode"
	propAuthMethod              = "consul.auth.method"
	propAuthAudience            = "consul.auth.audience"
	propFallbackRecheckInterval = "consul.auth.fallback.recheck.interval"

	defaultAuthMethod              = "applications-k8s-m2m"
	defaultAudience                = "netcracker"
	defaultFallbackRecheckInterval = "5h"
)

type authConfig struct {
	mode            AuthMode
	authMethod      string
	audience        string
	recheckInterval time.Duration
}

func resolveAuthConfig(cfg ClientConfig) (authConfig, error) {
	mode := cfg.Mode
	if mode == "" {
		mode = AuthMode(configloader.GetOrDefaultString(propAuthMode, ""))
	}
	if mode == "" {
		// One variable sets the M2M calls and the Consul login together; consul.auth.mode overrides it for the
		// Consul login alone. An unset M2M_AUTH_MODE is legacy.
		m2mMode, err := security.ReadM2MAuthMode()
		if err != nil {
			return authConfig{}, fmt.Errorf("property '%s' is not set, so the consul auth mode is taken from %s: %w",
				propAuthMode, security.M2MAuthModeEnv, err)
		}
		mode = AuthMode(m2mMode)
	}
	switch mode {
	case AuthModeLegacy, AuthModeHybrid, AuthModeK8s:
	default:
		return authConfig{}, fmt.Errorf("unknown value '%s' of property '%s': expected one of '%s', '%s', '%s'",
			mode, propAuthMode, AuthModeLegacy, AuthModeHybrid, AuthModeK8s)
	}

	authMethod := cfg.AuthMethod
	if authMethod == "" {
		authMethod = configloader.GetOrDefaultString(propAuthMethod, defaultAuthMethod)
	}

	audience := cfg.Audience
	if audience == "" {
		audience = configloader.GetOrDefaultString(propAuthAudience, defaultAudience)
	}

	recheckInterval := cfg.FallbackRecheckInterval
	if recheckInterval == 0 {
		rawInterval := configloader.GetOrDefaultString(propFallbackRecheckInterval, defaultFallbackRecheckInterval)
		parsedInterval, err := time.ParseDuration(rawInterval)
		if err != nil {
			return authConfig{}, fmt.Errorf("failed to parse value '%s' of property '%s': %w",
				rawInterval, propFallbackRecheckInterval, err)
		}
		recheckInterval = parsedInterval
	}

	return authConfig{
		mode:            mode,
		authMethod:      authMethod,
		audience:        audience,
		recheckInterval: recheckInterval,
	}, nil
}
