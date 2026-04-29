package routeregistration

import (
	"os"
	"testing"

	"github.com/netcracker/qubership-core-lib-go-rest-utils/v2/route-registration/internal/rest"
	"github.com/netcracker/qubership-core-lib-go-rest-utils/v2/route-registration/internal/utils"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/stretchr/testify/assert"
)

func init () {
	serviceloader.Register(2, &security.DummyToken{})
}

const (
	meshGateway = "test-gateway"
	pathFromV1  = "/v1"
	pathToV1    = "/v1/test"
)

var params = configloader.YamlPropertySourceParams{ConfigFilePath: "./testdata/application.yaml"}

func TestNewRegistrar(t *testing.T) {
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))
	registrar := NewRegistrar()
	assert.NotNil(t, registrar)
}

func TestWithRoutes_PublicRoute(t *testing.T) {
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))
	registrar := NewRegistrar().(*registrar)
	testRoute := createTestRoute(Public, "", pathFromV1, pathToV1)
	registrar.WithRoutes(testRoute)
	routesByGateway := registrar.routesByGatewayMap

	assert.True(t, contains(routesByGateway, PublicGatewayService))
	publicGatewayService := routesByGateway[PublicGatewayService]
	assert.True(t, publicGatewayService[0].Allowed)

	assert.True(t, contains(routesByGateway, PrivateGatewayService))
	privateGatewayService := routesByGateway[PrivateGatewayService]
	assert.True(t, privateGatewayService[0].Allowed)

	assert.True(t, contains(routesByGateway, InternalGatewayService))
	internalGatewayRoutes := routesByGateway[InternalGatewayService]
	assert.True(t, internalGatewayRoutes[0].Allowed)
}

func TestWithRoutes_PrivateRoute(t *testing.T) {
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))
	registrar := NewRegistrar().(*registrar)
	testRoute := createTestRoute(Private, "", pathFromV1, pathToV1)
	registrar.WithRoutes(testRoute)
	routesByGateway := registrar.routesByGatewayMap

	assert.True(t, contains(routesByGateway, PublicGatewayService))
	publicGatewayService := routesByGateway[PublicGatewayService]
	assert.False(t, publicGatewayService[0].Allowed)

	assert.True(t, contains(routesByGateway, PrivateGatewayService))
	privateGatewayService := routesByGateway[PrivateGatewayService]
	assert.True(t, privateGatewayService[0].Allowed)

	assert.True(t, contains(routesByGateway, InternalGatewayService))
	internalGatewayRoutes := routesByGateway[InternalGatewayService]
	assert.True(t, internalGatewayRoutes[0].Allowed)
}

func TestWithRoutes_InternalRoute(t *testing.T) {
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))
	registrar := NewRegistrar().(*registrar)
	testRoute := createTestRoute(Internal, "", pathFromV1, pathToV1)
	registrar.WithRoutes(testRoute)
	routesByGateway := registrar.routesByGatewayMap

	assert.True(t, contains(routesByGateway, PublicGatewayService))
	publicGatewayService := routesByGateway[PublicGatewayService]
	assert.False(t, publicGatewayService[0].Allowed)

	assert.True(t, contains(routesByGateway, PrivateGatewayService))
	privateGatewayService := routesByGateway[PrivateGatewayService]
	assert.False(t, privateGatewayService[0].Allowed)

	assert.True(t, contains(routesByGateway, InternalGatewayService))
	internalGatewayRoutes := routesByGateway[InternalGatewayService]
	assert.True(t, internalGatewayRoutes[0].Allowed)
}

func TestWithRoutes_MeshRoute(t *testing.T) {
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))
	registrar := NewRegistrar().(*registrar)
	testRoute := createTestRoute("", meshGateway, pathFromV1, pathToV1)
	registrar.WithRoutes(testRoute)
	routesByGateway := registrar.routesByGatewayMap
	assert.Equal(t, 1, len(routesByGateway))
	meshGatewayRoutes := routesByGateway[meshGateway]
	assert.True(t, meshGatewayRoutes[0].Allowed)
	assert.Equal(t, meshGateway, meshGatewayRoutes[0].Gateway)
}

func TestWithRoutes_NoRouteTypeButGatewayIsPublic(t *testing.T) {
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))
	registrar := NewRegistrar().(*registrar)
	testRoute := createTestRoute("", PublicGatewayService, pathFromV1, pathToV1)
	registrar.WithRoutes(testRoute)
	routesByGateway := registrar.routesByGatewayMap

	assert.True(t, contains(routesByGateway, PublicGatewayService))
	publicGatewayService := routesByGateway[PublicGatewayService]
	assert.True(t, publicGatewayService[0].Allowed)

	assert.True(t, contains(routesByGateway, PrivateGatewayService))
	privateGatewayService := routesByGateway[PrivateGatewayService]
	assert.True(t, privateGatewayService[0].Allowed)

	assert.True(t, contains(routesByGateway, InternalGatewayService))
	internalGatewayRoutes := routesByGateway[InternalGatewayService]
	assert.True(t, internalGatewayRoutes[0].Allowed)
}

func TestWithRoutes_WithoutRouteTypeAndWithoutGatewayName(t *testing.T) {
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))
	registrar := NewRegistrar()
	testRoute := createTestRoute("", "", pathFromV1, pathToV1)
	assert.Panics(t, func() { registrar.WithRoutes(testRoute) }, "Expected panic "+
		"because: No type and no target gateway specified for route")
}

func TestAssertPath(t *testing.T) {
	assertPath("/api/v4/tenant-manager/activate/create-os-tenant-alias-routes/rollback/{tenantId}")
	assertPath("/api/*")
	assertPath("/api/v4/tenant-manager/activate/create-os-tenant-alias-routes/{tenantId}/rollback")
	assertPath("/api")
}

func TestAssertPath_Panic(t *testing.T) {
	assert.Panics(t, func() { assertPath("api/") })
	assert.Panics(t, func() { assertPath("/api/:tenantId") })
}

func TestWithRoutes_MeshRouteTypeAndWithoutGatewayName(t *testing.T) {
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))
	registrar := NewRegistrar().(*registrar)
	testRoute := createTestRoute(Mesh, "", pathFromV1, pathToV1)
	registrar.WithRoutes(testRoute)
	routesByGateway := registrar.routesByGatewayMap
	microserviceName := configloader.GetOrDefaultString("microservice.name", "")
	meshGatewayRoutes := routesByGateway[microserviceName]
	assert.True(t, meshGatewayRoutes[0].Allowed)
	assert.Equal(t, utils.Mesh, meshGatewayRoutes[0].RouteType)
}

func TestWithRoutes_ConflictRouteTypeWithGateway(t *testing.T) {
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))
	registrar := NewRegistrar()
	testRoute := createTestRoute("custom", meshGateway, pathFromV1, pathToV1)
	assert.Panics(t, func() { registrar.WithRoutes(testRoute) }, "Conflicting field "+
		"RouteType and Gateway values in route")
}

func TestWithRoutes_IncorrectPairFromTo(t *testing.T) {
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))
	registrar := NewRegistrar()
	testRoute := Route{
		From:           "v1",
		To:             "v1/test",
		Forbidden:      false,
		Timeout:        0,
		RouteType:      Public,
		Gateway:        "",
		VirtualService: "",
		Hosts:          nil,
	}
	assert.Panics(t, func() { registrar.WithRoutes(testRoute) }, "Path doesn't satisfy validation regexp")
}

func TestWithRoutes_BulkRegister(t *testing.T) {
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))
	registrar := NewRegistrar().(*registrar)
	testRoute := createTestRoute(Public, "", pathFromV1, pathToV1)
	testRoute2 := createTestRoute(Private, "", "/v2", "/v2/test")
	testRoute3 := createTestRoute(Internal, "", "/v3", "/v3/test")
	registrar.WithRoutes(testRoute, testRoute2, testRoute3)
	routesByGateway := registrar.routesByGatewayMap

	assert.True(t, contains(routesByGateway, PublicGatewayService))
	publicGatewayService := routesByGateway[PublicGatewayService]
	assert.Equal(t, 3, len(publicGatewayService))
	assert.True(t, publicGatewayService[0].Allowed)
	assert.False(t, publicGatewayService[1].Allowed)
	assert.False(t, publicGatewayService[2].Allowed)

	assert.True(t, contains(routesByGateway, PrivateGatewayService))
	privateGatewayService := routesByGateway[PrivateGatewayService]
	assert.Equal(t, 3, len(privateGatewayService))
	assert.True(t, privateGatewayService[0].Allowed)
	assert.True(t, privateGatewayService[1].Allowed)
	assert.False(t, privateGatewayService[2].Allowed)

	assert.True(t, contains(routesByGateway, InternalGatewayService))
	internalGatewayRoutes := routesByGateway[InternalGatewayService]
	assert.Equal(t, 3, len(internalGatewayRoutes))
	assert.True(t, internalGatewayRoutes[0].Allowed)
	assert.True(t, internalGatewayRoutes[0].Allowed)
	assert.True(t, internalGatewayRoutes[0].Allowed)
}

func createTestRoute(rType RouteType, gateway string, from string, to string) Route {
	return Route{
		From:           from,
		To:             to,
		Forbidden:      false,
		Timeout:        0,
		RouteType:      rType,
		Gateway:        gateway,
		VirtualService: "",
		Hosts:          nil,
	}
}

func contains(routesMap utils.RoutesByGateway, key string) bool {
	_, isFound := routesMap[key]
	return isFound
}

func Test_registrar_Register(t *testing.T) {
	defer os.Clearenv()
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))

	reg := NewRegistrar().(*registrar)
	reg.requestSender = &TestRouteConsumerClient{}
	reg.WithRoutes(createTestRoute(Public, "", pathFromV1, pathToV1))
	reg.Register()
	assert.True(t, reg.requestSender.(*TestRouteConsumerClient).SendRequestCalled)

	os.Setenv("SERVICE_MESH_TYPE", "CORE")
	reg = NewRegistrar().(*registrar)
	reg.requestSender = &TestRouteConsumerClient{}
	reg.WithRoutes(createTestRoute(Public, "", pathFromV1, pathToV1))
	reg.Register()
	assert.True(t, reg.requestSender.(*TestRouteConsumerClient).SendRequestCalled)

	os.Setenv("SERVICE_MESH_TYPE", "ISTIO")
	reg = NewRegistrar().(*registrar)
	reg.requestSender = &TestRouteConsumerClient{}
	reg.WithRoutes(createTestRoute(Public, "", pathFromV1, pathToV1))
	reg.Register()
	assert.False(t, reg.requestSender.(*TestRouteConsumerClient).SendRequestCalled)
}

func Test_registrar_Register_WithConfig(t *testing.T) {
	defer os.Clearenv()
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))

	reg := NewRegistrarWithConfig(defaultRegistrarConfig()).(*registrar)
	reg.requestSender = &TestRouteConsumerClient{}
	reg.WithRoutes(createTestRoute(Public, "", pathFromV1, pathToV1))
	reg.Register()
	assert.True(t, reg.requestSender.(*TestRouteConsumerClient).SendRequestCalled)

	reg = NewRegistrarWithConfig(&RegistrarConfig{ServiceMeshType: CoreServiceMeshType}).(*registrar)
	reg.requestSender = &TestRouteConsumerClient{}
	reg.WithRoutes(createTestRoute(Public, "", pathFromV1, pathToV1))
	reg.Register()
	assert.True(t, reg.requestSender.(*TestRouteConsumerClient).SendRequestCalled)

	reg = NewRegistrarWithConfig(&RegistrarConfig{ServiceMeshType: IstioServiceMeshType}).(*registrar)
	reg.requestSender = &TestRouteConsumerClient{}
	reg.WithRoutes(createTestRoute(Public, "", pathFromV1, pathToV1))
	reg.Register()
	assert.False(t, reg.requestSender.(*TestRouteConsumerClient).SendRequestCalled)
}

func Test_defaultRegistrarConfig_NoEnv_DefaultsToCore(t *testing.T) {
	defer os.Clearenv()

	config := defaultRegistrarConfig()

	assert.Equal(t, CoreServiceMeshType, config.ServiceMeshType)
}

func Test_defaultRegistrarConfig_IstioEnv(t *testing.T) {
	defer os.Clearenv()
	os.Setenv("SERVICE_MESH_TYPE", "ISTIO")

	config := defaultRegistrarConfig()

	assert.Equal(t, IstioServiceMeshType, config.ServiceMeshType)
}

func Test_defaultRegistrarConfig_CoreEnv(t *testing.T) {
	defer os.Clearenv()
	os.Setenv("SERVICE_MESH_TYPE", "CORE")

	config := defaultRegistrarConfig()

	assert.Equal(t, CoreServiceMeshType, config.ServiceMeshType)
}

func Test_defaultRegistrarConfig_UnknownEnv_DefaultsToCore(t *testing.T) {
	defer os.Clearenv()
	os.Setenv("SERVICE_MESH_TYPE", "SOMETHING_UNKNOWN")

	config := defaultRegistrarConfig()

	assert.Equal(t, CoreServiceMeshType, config.ServiceMeshType)
}

func Test_defaultRegistrarConfig_LowercaseIstio_TreatedAsIstio(t *testing.T) {
	defer os.Clearenv()
	os.Setenv("SERVICE_MESH_TYPE", "istio")

	config := defaultRegistrarConfig()

	assert.Equal(t, IstioServiceMeshType, config.ServiceMeshType)
}

func Test_defaultRegistrarConfig_MixedCaseCore_TreatedAsCore(t *testing.T) {
	defer os.Clearenv()
	os.Setenv("SERVICE_MESH_TYPE", "Core")

	config := defaultRegistrarConfig()

	assert.Equal(t, CoreServiceMeshType, config.ServiceMeshType)
}

func TestNewRegistrarWithConfig_ConfigIsApplied(t *testing.T) {
	defer os.Clearenv()
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))

	// Even if env says CORE, explicit ISTIO config must win
	os.Setenv("SERVICE_MESH_TYPE", "CORE")
	reg := NewRegistrarWithConfig(&RegistrarConfig{ServiceMeshType: IstioServiceMeshType}).(*registrar)

	assert.Equal(t, IstioServiceMeshType, reg.config.ServiceMeshType)
}

func TestNewRegistrarWithConfig_IstioConfig_SkipsRegistration(t *testing.T) {
	defer os.Clearenv()
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))

	// Even if env says CORE, an explicit ISTIO config must suppress sending
	os.Setenv("SERVICE_MESH_TYPE", "CORE")
	reg := NewRegistrarWithConfig(&RegistrarConfig{ServiceMeshType: IstioServiceMeshType}).(*registrar)
	reg.requestSender = &TestRouteConsumerClient{}
	reg.WithRoutes(createTestRoute(Public, "", pathFromV1, pathToV1))
	reg.Register()

	assert.False(t, reg.requestSender.(*TestRouteConsumerClient).SendRequestCalled)
}

func TestWithRoutes_ForbiddenRoute_AllGatewaysDisallowed(t *testing.T) {
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))
	registrar := NewRegistrar().(*registrar)
	testRoute := Route{
		From:      pathFromV1,
		To:        pathToV1,
		Forbidden: true,
		RouteType: Public,
	}
	registrar.WithRoutes(testRoute)
	routesByGateway := registrar.routesByGatewayMap

	assert.False(t, routesByGateway[PublicGatewayService][0].Allowed)
	assert.False(t, routesByGateway[PrivateGatewayService][0].Allowed)
	assert.False(t, routesByGateway[InternalGatewayService][0].Allowed)
}

func TestWithRoutes_ForbiddenMeshRoute_IsDisallowed(t *testing.T) {
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))
	reg := NewRegistrar().(*registrar)
	testRoute := Route{
		From:      pathFromV1,
		To:        pathToV1,
		Forbidden: true,
		Gateway:   meshGateway,
	}
	reg.WithRoutes(testRoute)

	meshRoutes := reg.routesByGatewayMap[meshGateway]
	assert.False(t, meshRoutes[0].Allowed)
}

func TestWithRoutes_AccumulatesRoutesAcrossCalls(t *testing.T) {
	configloader.InitWithSourcesArray(configloader.BasePropertySources(params))
	reg := NewRegistrar().(*registrar)

	reg.WithRoutes(createTestRoute(Public, "", pathFromV1, pathToV1))
	reg.WithRoutes(createTestRoute(Public, "", "/v2", "/v2/test"))

	assert.Equal(t, 2, len(reg.routesByGatewayMap[PublicGatewayService]))
}

func TestServiceMeshType_Constants(t *testing.T) {
	assert.Equal(t, ServiceMeshType("CORE"), CoreServiceMeshType)
	assert.Equal(t, ServiceMeshType("ISTIO"), IstioServiceMeshType)
}

func TestServiceMeshType_IsString(t *testing.T) {
	var meshType ServiceMeshType = "CORE"
	assert.Equal(t, "CORE", string(meshType))
}

func TestRegistrarConfig_CoreServiceMeshType(t *testing.T) {
	config := &RegistrarConfig{ServiceMeshType: CoreServiceMeshType}
	assert.Equal(t, CoreServiceMeshType, config.ServiceMeshType)
}

func TestRegistrarConfig_IstioServiceMeshType(t *testing.T) {
	config := &RegistrarConfig{ServiceMeshType: IstioServiceMeshType}
	assert.Equal(t, IstioServiceMeshType, config.ServiceMeshType)
}

func TestRegistrarConfig_CustomServiceMeshType(t *testing.T) {
	customType := ServiceMeshType("CUSTOM")
	config := &RegistrarConfig{ServiceMeshType: customType}
	assert.Equal(t, ServiceMeshType("CUSTOM"), config.ServiceMeshType)
}

func TestRegistrarConfig_ZeroValue(t *testing.T) {
	config := &RegistrarConfig{}
	assert.Equal(t, ServiceMeshType(""), config.ServiceMeshType)
	assert.NotEqual(t, CoreServiceMeshType, config.ServiceMeshType)
	assert.NotEqual(t, IstioServiceMeshType, config.ServiceMeshType)
}

type TestRouteConsumerClient struct {
	SendRequestCalled bool
}

func (t *TestRouteConsumerClient) SendRequest(_ rest.RegistrationRequest) {
	t.SendRequestCalled = true
}
