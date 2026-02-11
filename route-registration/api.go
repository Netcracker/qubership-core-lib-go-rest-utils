package routeregistration

import (
	"os"
	"strings"
	"time"
)

const PublicGatewayService = "public-gateway-service"
const PrivateGatewayService = "private-gateway-service"
const InternalGatewayService = "internal-gateway-service"

const AnyHost = "*"

// RouteType stands for the type of route to be registered.
//
// Public type declares that routes should be registered in PUBLIC, PRIVATE and INTERNAL gateways.
//
// Private type declares that routes should be registered in PRIVATE and INTERNAL gateways.
//
// Internal type declares that routes should be registered only in INTERNAL gateway.
//
// Mesh type declares that routes should be registered in MESH gateway with the specified name.
type RouteType string

const (
	Public   RouteType = "public"
	Private  RouteType = "private"
	Internal RouteType = "internal"
	Mesh     RouteType = "mesh"
)

type Route struct {
	From           string
	To             string
	Forbidden      bool
	Timeout        time.Duration
	RouteType      RouteType
	Gateway        string
	VirtualService string
	Hosts          []string
}

type Registrar interface {
	WithRoutes(routes ...Route) Registrar
	Register()
}

func NewRegistrar() Registrar {
	return newRegistrar(defaultRegistrarConfig())
}

func NewRegistrarWithConfig(config *RegistrarConfig) Registrar {
	return newRegistrar(config)
}

func defaultRegistrarConfig() *RegistrarConfig {
	serviceMeshType := CoreServiceMeshType

	if env, ok := os.LookupEnv("SERVICE_MESH_TYPE"); ok {
		switch ServiceMeshType(strings.ToUpper(env)) {
		case IstioServiceMeshType:
			serviceMeshType = IstioServiceMeshType
		}
	}

	return &RegistrarConfig{ServiceMeshType: serviceMeshType}
}
