package podsecrets

import "github.com/netcracker/qubership-core-lib-go/v3/configloader"

func NewPropertySource() *configloader.PropertySource {
	return &configloader.PropertySource{
		Provider: &provider{dir: resolveDir()},
		Parser:   nil,
	}
}

func AddPodSecretsPropertySource(sources []*configloader.PropertySource) []*configloader.PropertySource {
	return append(sources, NewPropertySource())
}
