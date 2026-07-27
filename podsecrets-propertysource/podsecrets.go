package podsecrets

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"

	"github.com/knadh/koanf/v2"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
)

var logger = logging.GetLogger("podsecrets")

const (
	EnvSecretsDir     = "POD_SECRETS_DIR"
	DefaultSecretsDir = "/etc/secrets/pod-secrets"
)

func resolveDir() string {
	if dir, ok := os.LookupEnv(EnvSecretsDir); ok && dir != "" {
		return dir
	}
	return DefaultSecretsDir
}

type provider struct {
	dir       string
	lastKnown atomic.Value
}

func (p *provider) ReadBytes(*koanf.Koanf) ([]byte, error) {
	return nil, errors.New("podsecrets provider does not support this method")
}

func (p *provider) Read(*koanf.Koanf) (map[string]any, error) {
	snapshot, _ := p.lastKnown.Load().(map[string]any)

	entries, err := os.ReadDir(p.dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			logger.Debug("Pod-secrets directory does not exist: %s", p.dir)
			return map[string]any{}, nil
		}
		logger.Warn("Cannot list pod-secrets directory %s: %s", p.dir, err.Error())
		return snapshot, nil
	}

	result := make(map[string]any, len(entries))
	keyNames := make([]string, 0, len(entries))

	for _, entry := range entries {
		key := normaliseKey(entry.Name())
		path := filepath.Join(p.dir, entry.Name())

		info, err := os.Stat(path)
		if err != nil {
			logger.Warn("Cannot stat pod-secret file %s: %s", entry.Name(), err.Error())
			useStored(snapshot, key, result)
			continue
		}
		if info.IsDir() {
			continue
		}

		value, err := readSecretFile(path)
		if err != nil {
			logger.Warn("Cannot read pod-secret file %s: %s", entry.Name(), err.Error())
			useStored(snapshot, key, result)
			continue
		}

		result[key] = value
		keyNames = append(keyNames, entry.Name())
	}

	logger.Debug("Pod-secrets key names: %v (dir=%s)", keyNames, p.dir)
	p.lastKnown.Store(result)

	return result, nil
}

func useStored(snapshot map[string]any, key string, result map[string]any) {
	if prev, ok := snapshot[key]; ok {
		result[key] = prev
	}
}

func normaliseKey(fileName string) string {
	return strings.ReplaceAll(strings.ToLower(fileName), "_", ".")
}

func readSecretFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
