package podsecrets

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

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
	dir string
}

func (p *provider) ReadBytes(*koanf.Koanf) ([]byte, error) {
	return nil, errors.New("podsecrets provider does not support this method")
}

func (p *provider) Read(*koanf.Koanf) (map[string]any, error) {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			logger.Debug("Pod-secrets directory does not exist: %s", p.dir)
		} else {
			logger.Debug("Cannot list pod-secrets directory %s: %s", p.dir, err.Error())
		}
		return map[string]any{}, nil
	}

	result := make(map[string]any, len(entries))
	keyNames := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		value, err := readSecretFile(filepath.Join(p.dir, entry.Name()))
		if err != nil {
			logger.Debug("Cannot read pod-secret file %s: %s", entry.Name(), err.Error())
			continue
		}

		result[normaliseKey(entry.Name())] = value
		keyNames = append(keyNames, entry.Name())
	}

	logger.Info("Pod-secrets loaded %d key(s) from %s", len(result), p.dir)
	if len(keyNames) > 0 {
		logger.Debug("Pod-secrets key names: %v (dir=%s)", keyNames, p.dir)
	}
	return result, nil
}

func normaliseKey(fileName string) string {
	return strings.ReplaceAll(strings.ToLower(fileName), "_", ".")
}

func readSecretFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	value := strings.TrimSuffix(string(data), "\n")
	value = strings.TrimSuffix(value, "\r")
	return value, nil
}
