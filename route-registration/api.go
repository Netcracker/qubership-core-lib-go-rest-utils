package routeregistration

import (
	"encoding/json"
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

	topologyPath := "/etc/topology.json"
	if envPath := os.Getenv("TOPOLOGY_CONFIG_PATH"); envPath != "" {
		topologyPath = envPath
	}

	data, err := os.ReadFile(topologyPath)
	if err != nil {
		log.Error("failed to read topology config, defaulting to %s: %w", serviceMeshType, err)
		return &RegistrarConfig{ServiceMeshType: serviceMeshType}
	}

	var topology struct {
		FeatureFlags struct {
			Core struct {
				ServiceMeshType string `json:"serviceMeshType"`
			} `json:"core"`
		} `json:"featureFlags"`
	}

	if err := json.Unmarshal(data, &topology); err != nil {
		log.Error("failed to parse topology config, defaulting to %s: %w", serviceMeshType, err)
		return &RegistrarConfig{ServiceMeshType: serviceMeshType}
	}

	if value := topology.FeatureFlags.Core.ServiceMeshType; value != "" {
		switch ServiceMeshType(strings.ToUpper(value)) {
		case IstioServiceMeshType:
			serviceMeshType = IstioServiceMeshType
		case CoreServiceMeshType:
			serviceMeshType = CoreServiceMeshType
		default:
			log.Error("unknown serviceMeshType %q, defaulting to %s", value, serviceMeshType)
		}
	}

	return &RegistrarConfig{ServiceMeshType: serviceMeshType}
}
