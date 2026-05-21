package configserver

import (
	"context"

	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
)

type PropertySourceConfiguration struct {
	Ctx              context.Context
	MicroserviceName string // name of microservice
	ConfigServerUrl  string // URL to config-server, example is <http://config-server:8080>
}

func GetPropertySource(params ...PropertySourceConfiguration) *configloader.PropertySource {
	var configuration PropertySourceConfiguration
	if len(params) > 0 {
		configuration = params[0]
	}
	ctx := configuration.Ctx
	if ctx == nil {
		ctx = context.TODO()
	}
	return &configloader.PropertySource{Provider: newConfigServerLoader(ctx, &configuration)}
}

func AddConfigServerPropertySource(sources []*configloader.PropertySource, params ...PropertySourceConfiguration) []*configloader.PropertySource {
	return append(sources, GetPropertySource(params...))
}
