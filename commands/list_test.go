//go:build !integration

package commands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListCommand_DoesNotCreateSystemIDFile(t *testing.T) {
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.toml")
	cfg := "[[runners]]\n  name = \"runner\"\n  token = \"glrt-token\"\n  url = \"https://gitlab.example.com/\"\n"
	require.NoError(t, os.WriteFile(cfgFile, []byte(cfg), 0o600))

	(&ListCommand{ConfigFile: cfgFile}).Execute(nil)

	assert.NoFileExists(t, filepath.Join(dir, ".runner_system_id"))
}
