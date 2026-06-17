# Podsecrets-propertysource

The package provides a pod-secrets property source which is intended for [Configloader](https://github.com/Netcracker/qubership-core-lib-go/blob/main/configloader/README.md).
This property source reads Kubernetes pod-secrets from a mounted directory and exposes them as configuration properties, overriding values from environment variables.

- [How to get](#how-to-get)
- [Usage](#usage)
- [Configuration](#configuration)
- [Watching for secret rotation](#watching-for-secret-rotation)

## How to get

To get `podsecrets-propertysource` use
```go
 go get github.com/netcracker/qubership-core-lib-go-rest-utils/v2@<latest released version>
```

List of all released versions may be found [here](https://github.com/netcracker/qubership-core-lib-go-rest-utils/tags)

## Usage

Add the pod-secrets source to your property source list. It should go **after environment variables but before config-server/Consul**, so that mounted secrets override env vars while Consul/config-server can still take precedence:

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

If you also use config-server or Consul, add pod-secrets **before** them so Consul takes the highest priority (Consul > pod-secrets > env vars):

```go
sources := configloader.BasePropertySources()
sources = podsecrets.AddPodSecretsPropertySource(sources)
sources = configserver.AddConfigServerPropertySource(sources)

configloader.InitWithSourcesArray(sources)
```

## Configuration

By default, secrets are read from `/etc/secrets/pod-secrets`. The directory can be changed with the `POD_SECRETS_DIR` environment variable.

Each file in the secrets directory becomes one property. File names are lowercased and underscores are replaced with dots:

| Secret file   | Property key  |
|---------------|---------------|
| `db_password` | `db.password` |
| `API_TOKEN`   | `api.token`   |

If the directory does not exist (e.g. no secrets are mounted for the pod), the property source simply contributes no properties - the application continues to resolve configuration from environment variables / yaml as usual.

## Watching for secret rotation

To pick up rotated secrets automatically, without restarting the application, start a watcher:

```go
watcher, err := podsecrets.StartWatcher()
if err != nil {
	// not fatal - the property source itself tolerates a missing directory
}
defer watcher.Stop()
```

When a secret file changes, the watcher refreshes the configuration automatically.
