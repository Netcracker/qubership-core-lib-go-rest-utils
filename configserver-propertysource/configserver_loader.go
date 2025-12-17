package configserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/knadh/koanf/maps"
	"github.com/knadh/koanf/v2"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	constants "github.com/netcracker/qubership-core-lib-go/v3/const"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-core-lib-go/v3/security"
	"github.com/netcracker/qubership-core-lib-go/v3/serviceloader"
	"github.com/netcracker/qubership-core-lib-go/v3/utils"
)

var logger logging.Logger

type configServerLoader struct {
	propertySourceConfiguration *PropertySourceConfiguration
}

func newConfigServerLoader(params *PropertySourceConfiguration) *configServerLoader {
	return &configServerLoader{params}
}

func (this *configServerLoader) ReadBytes(*koanf.Koanf) ([]byte, error) {
	return nil, errors.New("configserver provider does not support this method")
}

func (this *configServerLoader) Read(*koanf.Koanf) (map[string]interface{}, error) {
	source, err := getConfigServerProperties(this.propertySourceConfiguration)
	if err != nil {
		return nil, err
	}
	flattenMap, _ := maps.Flatten(source, []string{}, ".")
	return flattenMap, nil
}

func getConfigServerProperties(params *PropertySourceConfiguration) (map[string]interface{}, error) {
	microserviceName, configServerUrl := getMicroserviceNameAndURL(params)
	tokenProvider := serviceloader.MustLoad[security.TokenProvider]()
	token, err := tokenProvider.GetToken(context.Background())
	if err != nil {
		err = fmt.Errorf("could not get token to load properties from config-server, err: %w", err)
		return nil, err
	}
	client := utils.GetClient()
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/%s/default", configServerUrl, microserviceName), nil)
	if token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	res, err := client.Do(req)
	if err != nil {
		logger.Error("Failed send request to config-server: %s", err)
		return nil, err
	}
	defer res.Body.Close()
	return parseBody(res.Body)
}

func parseBody(body io.Reader) (map[string]interface{}, error) {
	restBody, err := io.ReadAll(body)
	if err != nil {
		logger.Error("Failed to read response body: %s", err)
		return nil, err
	}

	configserverEnv := configserverEnv{}
	err = json.Unmarshal(restBody, &configserverEnv)
	if err != nil {
		logger.Error("Failed to unmarshal response body: %s", err)
		return nil, err
	}
	if len(configserverEnv.PropertySources) == 0 {
		logger.Warn("PropertySources is empty for '%s'", configserverEnv.Name)
		return make(map[string]interface{}), nil
	}
	return configserverEnv.PropertySources[0].Source, nil
}

type configserverEnv struct {
	Name            string                             `json:"name"`
	Profiles        []string                           `json:"profiles"`
	PropertySources []configserverPropertySourceEntity `json:"propertySources"`
}

type configserverPropertySourceEntity struct {
	Name   string                 `json:"name"`
	Source map[string]interface{} `json:"source"`
}

func getMicroserviceNameAndURL(params *PropertySourceConfiguration) (string, string) {
	defaultUrl := constants.SelectUrl("http://config-server:8080", "https://config-server:8443")
	configserverURL := configloader.GetOrDefaultString("config-server.url", defaultUrl)
	microserviceName := configloader.GetOrDefaultString("microservice.name", "")
	if params != nil {
		if params.MicroserviceName != "" {
			microserviceName = params.MicroserviceName
		}
		if params.ConfigServerUrl != "" {
			configserverURL = params.ConfigServerUrl
		}
	}
	if microserviceName == "" {
		panic(fmt.Sprint("You did not specify the mandatory 'microservice.name' property. " +
			"You should use configloader with BasePropertySources and " +
			"specify the parameter in application.yaml(pass through env variable) or use configserver.PropertySourceConfiguration."))
	}
	return microserviceName, configserverURL
}
