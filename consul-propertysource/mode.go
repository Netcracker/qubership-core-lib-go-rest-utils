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

// AuthSettings are the login settings a [Client] reads from the configuration when [ClientConfig] leaves them empty.
type AuthSettings struct {
	Mode       AuthMode
	AuthMethod string
	Audience   string
}

// ReadAuthSettings returns the settings a [Client] uses when [ClientConfig] leaves them empty. The mode is
// consul.auth.mode, else M2M_AUTH_MODE, else [AuthModeLegacy]; it returns an error for a mode other than legacy,
// hybrid, or k8s.
func ReadAuthSettings() (AuthSettings, error) {
	mode, err := readAuthMode()
	if err != nil {
		return AuthSettings{}, err
	}
	return AuthSettings{
		Mode:       mode,
		AuthMethod: configloader.GetOrDefaultString(propAuthMethod, defaultAuthMethod),
		Audience:   configloader.GetOrDefaultString(propAuthAudience, defaultAudience),
	}, nil
}

func readAuthMode() (AuthMode, error) {
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
		if mode, err = readAuthMode(); err != nil {
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
