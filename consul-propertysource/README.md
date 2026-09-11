# Consul-propertysource

The package provides Consul property source which is intended for [Configloader](https://github.com/Netcracker/qubership-core-lib-go/blob/main/configloader/README.md).
This property source allows downloading properties from a Consul service.

- [How to get](#how-to-get)
- [Usage](#usage)
- [Authentication](#authentication)
- [Plain Consul Client](#plain-consul-client)
  
## How to get
To get `consul-propertysource` use
```go
 go get github.com/netcracker/qubership-core-lib-go-rest-utils/@<latest released version>
```

List of all released versions may be found [here](https://github.com/netcracker/qubership-core-lib-go-rest-utils/tags)

## Usage

Create consul property source and add it to config loader([see for additional info](https://github.com/netcracker/qubership-core-lib-go/blob/main/configloader/README.md#Usage)) 
  ```go
  consulPS := consul.NewPropertySource(
    consul.ProviderConfig{
        Address:          "<consul-url>",
        Namespace:        "<namespace>",
        MicroserviceName: "<microservice-name>",
        Paths:            "<consul-kv-paths>",
        Ctx:              "<context>",
        Failsafe:         <failsafe>,
        Token:            "<token>",
      }
    )
  ```

*  **consul-url** - Consul URL (default: value from **consul.url**)
*  **namespace** - microservice namespace (default: value from **microservice.namespace**)
*  **microservice-name** - microservice name (default: value from **microservice.name**)
*  **consul-kv-paths** - a list of path roots for config properties (default: **"config/\<namespace\>/application", "config/\<namespace\>/\<microservice-name\>"**)
*  **context** - custom context (default: **context.Background()**)
*  **failsafe** - if true, then all problems with connection to Consul will be ignored and will not fail the application (default: false)
*  **token** - setting a value in the Token field disables the mechanism for obtaining a Consul token via anonymous request and instead uses the specified token (nil by default is switches property source to use anonymous request authentication mechanism)

All properties are optional.

Also you can watch for config changes
```go
consul.WatchForProperties(consulPS, func(event interface{}, err error) {
    // your code here if any action on event is required
})
```
WatchForProperties automatically refreshes the properties in application by property sources.

### Consul for logging levels update

If you want to use Consul for automatic logging level updates, then you can create special Consul PropertySource and init your configloader with it:
```go
consulPS := consul.NewLoggingPropertySource()
configloader.InitWithSourcesArray(append(configloader.BasePropertySources(), consulPS))
consul.StartWatchingForPropertiesWithRetry(context.Background(), consulPS, func(event interface{}, err error) {
    // your code here if any action on event is required
})
```

Then logging configuration will be automatically gathered from 'logging/<namespace>/<microserviceName>' root.

## Authentication

The property source logs in to Consul and uses the returned ACL token for all further requests. There are two ways to get that token: an exchange of a Kubernetes projected volume token through a Consul auth method of type `jwt`, and an exchange of an m2m token. The way is selected by **consul.auth.mode**. The mode names below say where the presented token comes from, not which type the Consul auth method has.

| Mode | Behavior |
|---|---|
| **kubernetes-with-m2m-fallback** | Logs in with the Kubernetes projected volume token. On failure falls back to the m2m token and probes the Kubernetes way again every **consul.auth.fallback.recheck.interval**. Once a probe succeeds, the fallback is disabled until the microservice restarts. |
| **kubernetes** | Logs in with the Kubernetes projected volume token only. A login failure is returned to the caller. |
| **m2m** | Logs in with the m2m token only, using the microservice namespace as the auth method name. This is the way used before the projected volume token exchange was introduced. |

A probe of the kubernetes way rides on a scheduled relogin, so **consul.auth.fallback.recheck.interval** is a lower bound and not a period. A client that holds a token without an expiration time never relogins, and keeps the way it picked at startup until the microservice restarts.

*  **consul.auth.mode** - way to get the ACL token: **kubernetes-with-m2m-fallback**, **kubernetes** or **m2m** (default: **kubernetes-with-m2m-fallback**)
*  **consul.auth.method** - name of the Consul auth method of type jwt (default: **applications-k8s-m2m**)
*  **consul.auth.audience** - audience of the Kubernetes projected volume token (default: **netcracker**)
*  **consul.auth.fallback.recheck.interval** - how long the m2m fallback lasts before the kubernetes way is probed again (default: **5h**)

Each property has a matching field in `ProviderConfig` and `ClientConfig`: `Mode`, `AuthMethod`, `Audience`, and `FallbackRecheckInterval`. A non-empty field wins over the property.

The **kubernetes** and **kubernetes-with-m2m-fallback** modes need two things in place:

1. The pod mounts a projected volume with a token of the audience from **consul.auth.audience** under `/var/run/secrets/tokens`.
2. Consul has an auth method of type `jwt` named as in **consul.auth.method**, whose `BoundAudiences` cover the audience from **consul.auth.audience** and whose binding rules grant the microservice its policies.

Roll out in this order: mount the projected volume, create the auth method with its binding rules, then start the microservices with the default mode. Until both are in place, the default mode keeps working through the m2m fallback.

Every successful login writes an INFO record with the name of the auth method that issued the token, so the way in use is visible in the log:

```text
Logged in to Consul with auth method 'applications-k8s-m2m'
```

`ClientConfig.Namespace` is used as the auth method name by the **m2m** mode only.

## Plain Consul Client

To access to Consul KV storage you can use plain KV client

```go
c := consul.NewClient(consul.ClientConfig{
    Address:   "<consul-url>",
    Namespace: "<namespace>",
    Ctx:       "<context>",
})
c.Login()
l, _, _ := c.KV().List("/", &api.QueryOptions{
    Token: c.SecretId(),
})
```

You **must** call c.Login() before first usage.

* **consul-url** - Consul URL
* **namespace** - microservice namespace
* **context** - custom context
