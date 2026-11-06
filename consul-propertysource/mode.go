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

// AuthSettings are the settings a [Client] logs in to Consul with.
type AuthSettings struct {
	Mode                    AuthMode
	AuthMethod              string
	Audience                string
	FallbackRecheckInterval time.Duration
}

// ReadAuthSettings reads the settings a [Client] logs in with when its [ClientConfig] sets none of them. The mode
// comes from consul.auth.mode, else M2M_AUTH_MODE, else [AuthModeLegacy]. The auth method, the audience, and the
// fallback recheck interval come from consul.auth.method, consul.auth.audience, and
// consul.auth.fallback.recheck.interval, by default applications-k8s-m2m, netcracker, and 5h. It returns an error
// for a mode other than legacy, hybrid, or k8s, and for an interval that does not parse.
func ReadAuthSettings() (AuthSettings, error) {
	return resolveAuthSettings(AuthSettings{})
}

// resolveAuthSettings takes each setting from explicit where it is set and from the configuration otherwise.
func resolveAuthSettings(explicit AuthSettings) (AuthSettings, error) {
	settings := explicit

	if settings.Mode == "" {
		settings.Mode = AuthMode(configloader.GetOrDefaultString(propAuthMode, ""))
	}
	if settings.Mode == "" {
		// One variable sets the M2M calls and the Consul login together; consul.auth.mode overrides it for the
		// Consul login alone.
		m2mMode, err := security.ReadM2MAuthMode()
		if err != nil {
			return AuthSettings{}, fmt.Errorf("property '%s' is not set, so the consul auth mode is taken from %s: %w",
				propAuthMode, security.M2MAuthModeEnv, err)
		}
		settings.Mode = AuthMode(m2mMode)
	}
	switch settings.Mode {
	case AuthModeLegacy, AuthModeHybrid, AuthModeK8s:
	default:
		return AuthSettings{}, fmt.Errorf("unknown value '%s' of property '%s': expected one of '%s', '%s', '%s'",
			settings.Mode, propAuthMode, AuthModeLegacy, AuthModeHybrid, AuthModeK8s)
	}

	if settings.AuthMethod == "" {
		settings.AuthMethod = configloader.GetOrDefaultString(propAuthMethod, defaultAuthMethod)
	}
	if settings.Audience == "" {
		settings.Audience = configloader.GetOrDefaultString(propAuthAudience, defaultAudience)
	}
	if settings.FallbackRecheckInterval == 0 {
		rawInterval := configloader.GetOrDefaultString(propFallbackRecheckInterval, defaultFallbackRecheckInterval)
		interval, err := time.ParseDuration(rawInterval)
		if err != nil {
			return AuthSettings{}, fmt.Errorf("failed to parse value '%s' of property '%s': %w",
				rawInterval, propFallbackRecheckInterval, err)
		}
		settings.FallbackRecheckInterval = interval
	}
	return settings, nil
}
