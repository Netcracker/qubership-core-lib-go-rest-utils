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

// AuthSettings are the settings of the Consul login.
type AuthSettings struct {
	Mode                    AuthMode
	AuthMethod              string
	Audience                string
	FallbackRecheckInterval time.Duration
}

// ReadAuthSettings reads the Consul login settings from the consul.auth.* properties and M2M_AUTH_MODE, as the
// property source does. It returns an error for an unsupported mode or an interval that does not parse.
func ReadAuthSettings() (AuthSettings, error) {
	return resolveAuthConfig(ClientConfig{})
}

func resolveAuthConfig(cfg ClientConfig) (AuthSettings, error) {
	mode := cfg.Mode
	if mode == "" {
		mode = AuthMode(configloader.GetOrDefaultString(propAuthMode, ""))
	}
	if mode == "" {
		// One variable sets the M2M calls and the Consul login together; consul.auth.mode overrides it for the
		// Consul login alone. An unset M2M_AUTH_MODE is legacy.
		m2mMode, err := security.ReadM2MAuthMode()
		if err != nil {
			return AuthSettings{}, fmt.Errorf("property '%s' is not set, so the consul auth mode is taken from %s: %w",
				propAuthMode, security.M2MAuthModeEnv, err)
		}
		mode = AuthMode(m2mMode)
	}
	switch mode {
	case AuthModeLegacy, AuthModeHybrid, AuthModeK8s:
	default:
		return AuthSettings{}, fmt.Errorf("unknown value '%s' of property '%s': expected one of '%s', '%s', '%s'",
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
			return AuthSettings{}, fmt.Errorf("failed to parse value '%s' of property '%s': %w",
				rawInterval, propFallbackRecheckInterval, err)
		}
		recheckInterval = parsedInterval
	}

	return AuthSettings{
		Mode:                    mode,
		AuthMethod:              authMethod,
		Audience:                audience,
		FallbackRecheckInterval: recheckInterval,
	}, nil
}
