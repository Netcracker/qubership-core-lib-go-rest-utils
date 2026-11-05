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

// ReadAuthMode returns the Consul auth mode that a [Client] without [ClientConfig.Mode] uses: the property
// consul.auth.mode, or, when it is empty, M2M_AUTH_MODE as [security.ReadM2MAuthMode] reads it, so an empty
// M2M_AUTH_MODE means [AuthModeLegacy]. It returns an error when the mode is not legacy, hybrid, or k8s.
//
// A service that also talks to Consul without a [Client], and has to follow the same mode, reads it here.
func ReadAuthMode() (AuthMode, error) {
	mode := AuthMode(configloader.GetOrDefaultString(propAuthMode, ""))
	if mode != "" {
		return mode, validateAuthMode(mode)
	}
	// One variable sets the M2M calls and the Consul login together; consul.auth.mode overrides it for the
	// Consul login alone.
	m2mMode, err := security.ReadM2MAuthMode()
	if err != nil {
		return "", fmt.Errorf("property '%s' is not set, so the consul auth mode is taken from %s: %w",
			propAuthMode, security.M2MAuthModeEnv, err)
	}
	return AuthMode(m2mMode), nil
}

func validateAuthMode(mode AuthMode) error {
	switch mode {
	case AuthModeLegacy, AuthModeHybrid, AuthModeK8s:
		return nil
	default:
		return fmt.Errorf("unknown value '%s' of property '%s': expected one of '%s', '%s', '%s'",
			mode, propAuthMode, AuthModeLegacy, AuthModeHybrid, AuthModeK8s)
	}
}

func resolveAuthConfig(cfg ClientConfig) (authConfig, error) {
	mode := cfg.Mode
	if mode == "" {
		var err error
		if mode, err = ReadAuthMode(); err != nil {
			return authConfig{}, err
		}
	} else if err := validateAuthMode(mode); err != nil {
		return authConfig{}, err
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
