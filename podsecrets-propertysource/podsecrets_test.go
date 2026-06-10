package podsecrets

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testdataSecretsDir = "./testdata/secrets"

func TestNormaliseKey(t *testing.T) {
	assert.Equal(t, "db.password", normaliseKey("db_password"))
	assert.Equal(t, "api.token", normaliseKey("API_TOKEN"))
	assert.Equal(t, "single", normaliseKey("single"))
}

func TestProvider_Read_LoadsAndNormalisesKeys(t *testing.T) {
	p := &provider{dir: testdataSecretsDir}
	values, err := p.Read(nil)
	require.NoError(t, err)

	assert.Equal(t, "secret-password", values["db.password"])
	assert.Equal(t, "secret-token", values["api.token"])
	assert.Len(t, values, 2)
}

func TestProvider_Read_MissingDirectory_ReturnsEmptyMapNoError(t *testing.T) {
	p := &provider{dir: filepath.Join(t.TempDir(), "does-not-exist")}
	values, err := p.Read(nil)
	require.NoError(t, err)
	assert.Empty(t, values)
}

func TestProvider_ReadBytes_NotSupported(t *testing.T) {
	p := &provider{dir: testdataSecretsDir}
	_, err := p.ReadBytes(nil)
	assert.Error(t, err)
}

func TestReadSecretFile_StripsTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	require.NoError(t, os.WriteFile(path, []byte("plain-value\n"), 0o600))

	value, err := readSecretFile(path)
	require.NoError(t, err)
	assert.Equal(t, "plain-value", value)
}

func TestPropertySource_OverridesEnv(t *testing.T) {
	os.Setenv("DB_PASSWORD", "env-password")
	defer os.Unsetenv("DB_PASSWORD")
	os.Setenv("DB_HOST", "env-host")
	defer os.Unsetenv("DB_HOST")

	t.Setenv(EnvSecretsDir, testdataSecretsDir)

	sources := AddPodSecretsPropertySource(
		[]*configloader.PropertySource{configloader.EnvPropertySource()},
	)
	configloader.InitWithSourcesArray(sources)

	assert.Equal(t, "secret-password", configloader.GetOrDefaultString("db.password", ""))
	assert.Equal(t, "secret-token", configloader.GetOrDefaultString("api.token", ""))
	assert.Equal(t, "env-host", configloader.GetOrDefaultString("db.host", ""))
}

func TestPropertySource_RefreshPicksUpRotatedSecret(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "db_password"), []byte("initial-password"), 0o600))
	t.Setenv(EnvSecretsDir, dir)

	configloader.InitWithSourcesArray([]*configloader.PropertySource{NewPropertySource()})
	assert.Equal(t, "initial-password", configloader.GetOrDefaultString("db.password", ""))

	require.NoError(t, os.WriteFile(filepath.Join(dir, "db_password"), []byte("rotated-password"), 0o600))
	require.NoError(t, configloader.Refresh())

	assert.Equal(t, "rotated-password", configloader.GetOrDefaultString("db.password", ""))
}

func TestStartWatcher_RefreshesOnSecretChange(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "db_password"), []byte("initial-password"), 0o600))
	t.Setenv(EnvSecretsDir, dir)

	configloader.InitWithSourcesArray([]*configloader.PropertySource{NewPropertySource()})
	require.Equal(t, "initial-password", configloader.GetOrDefaultString("db.password", ""))

	watcher, err := StartWatcher()
	require.NoError(t, err)
	defer watcher.Stop()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "db_password"), []byte("rotated-password"), 0o600))

	require.Eventually(t, func() bool {
		return configloader.GetOrDefaultString("db.password", "") == "rotated-password"
	}, 5*time.Second, 50*time.Millisecond)
}
