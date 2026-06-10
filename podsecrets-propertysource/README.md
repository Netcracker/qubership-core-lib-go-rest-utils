# Podsecrets-propertysource

The package provides a pod-secrets property source which is intended for [Configloader](https://github.com/Netcracker/qubership-core-lib-go/blob/main/configloader/README.md).
This property source reads Kubernetes pod-secrets from a mounted directory and exposes them as koanf properties overriding values from environment variables.

- [How to get](#how-to-get)
- [Usage](#usage)
- [Combining with other property sources](#combining-with-other-property-sources)
- [Directory location](#directory-location)
- [Key naming](#key-naming)
- [Refreshing on secret rotation](#refreshing-on-secret-rotation)
- [Logging](#logging)

## How to get

To get `podsecrets-propertysource` use
```go
 go get github.com/netcracker/qubership-core-lib-go-rest-utils/v2@<latest released version>
```

List of all released versions may be found [here](https://github.com/netcracker/qubership-core-lib-go-rest-utils/tags)

## Usage

Add the pod-secrets source **last** in the property source list passed to `configloader.Init` / `configloader.InitWithSourcesArray` - koanf merges sources in order, with later sources overriding earlier ones, so secrets take precedence over both environment variables and yaml source.

```go
import (
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	podsecrets "github.com/netcracker/qubership-core-lib-go-rest-utils/v2/podsecrets-propertysource"
)

func init() {
	configloader.InitWithSourcesArray(
		podsecrets.AddPodSecretsPropertySource(configloader.BasePropertySources()),
	)
}
```

`AddPodSecretsPropertySource` is a thin convenience wrapper:

```go
podsecrets.AddPodSecretsPropertySource(sources)
// is equivalent to
append(sources, podsecrets.NewPropertySource())
```

You can also use `podsecrets.NewPropertySource()` directly if you are building the source list manually.

## Combining with other property sources

When config-server (or consul) is also used, add pod-secrets **last**, after all other sources, so that locally mounted secrets always take precedence over values coming from a remote source:

```go
import (
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	configserver "github.com/netcracker/qubership-core-lib-go-rest-utils/v2/configserver-propertysource"
	podsecrets "github.com/netcracker/qubership-core-lib-go-rest-utils/v2/podsecrets-propertysource"
)

func init() {
	sources := configloader.BasePropertySources()
	sources = configserver.AddConfigServerPropertySource(sources)
	sources = podsecrets.AddPodSecretsPropertySource(sources)

	configloader.InitWithSourcesArray(sources)
}
```

This results in `yaml -> env -> config-server -> pod-secrets`, so a value coming from config-server can never override a value mounted from a Kubernetes Secret.

## Directory location

The secrets directory is resolved as follows (first match wins):

1. The `POD_SECRETS_DIR` environment variable.
2. `/etc/secrets/pod-secrets` (`podsecrets.DefaultSecretsDir`).

If the directory does not exist (e.g. no pod-secrets are mounted), the property source silently contributes no properties - the application continues to resolve configuration from environment variables / yaml as before.

## Key naming

Each file in the secrets directory becomes one property:

| Secret file  | Property key  |
|--------------|---------------|
| `db_password`| `db.password` |
| `API_TOKEN`  | `api.token`   |

The file name is lower-cased and underscores are converted to dots, matching the
key format produced by `configloader.EnvPropertySource()`.

File contents are read as UTF-8 with a single trailing `\n` (or `\r\n`) stripped,
if present - multi-line secrets (e.g. PEM certificates/keys) are preserved as-is.

## Refreshing on secret rotation

`configloader.Refresh()` re-reads all configured property sources, including
pod-secrets. To pick up rotated secrets automatically, start a watcher:

```go
watcher, err := podsecrets.StartWatcher() // resolves the directory the same way as NewPropertySource
if err != nil {
    // not fatal: the property source itself tolerates a missing directory
}
defer watcher.Stop()
```

The watcher debounces filesystem events (Kubernetes secret rotation typically
touches multiple files via symlink swaps) and calls `configloader.Refresh()` once
per burst of changes.

## Logging

Secret **values are never logged**.

- At `INFO` level, only the number of loaded keys and the fact of (re)loading are
  logged, e.g. `Pod-secrets loaded 2 key(s) from /etc/secrets/pod-secrets`.
- At `DEBUG` level, the list of key **names** (not values) and the secrets
  directory path are additionally logged.

The logger name is `podsecrets`, configurable via the standard
`logging.level.podsecrets` / `LOGGING_LEVEL_PODSECRETS` settings.
