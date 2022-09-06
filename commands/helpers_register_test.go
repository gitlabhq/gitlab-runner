// Helper functions that are shared between unit tests and integration tests

package commands

import (
	"bufio"
	"bytes"
	"os"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// NewRegisterCommandForTest exposes RegisterCommand to integration tests
func NewRegisterCommandForTest(reader *bufio.Reader, network common.Network) *RegisterCommand {
	cmd := newRegisterCommand()
	cmd.reader = reader
	cmd.network = network

	return cmd
}

func GetLogrusOutput(t *testing.T, hook *test.Hook) string {
	buf := &bytes.Buffer{}
	for _, entry := range hook.AllEntries() {
		message, err := entry.String()
		require.NoError(t, err)

		buf.WriteString(message)
	}

	return buf.String()
}

func PrepareConfigurationTemplateFile(t *testing.T, content string) (string, func()) {
	file, err := os.CreateTemp("", "config.template.toml")
	require.NoError(t, err)

	defer func() {
		err = file.Close()
		require.NoError(t, err)
	}()

	_, err = file.WriteString(content)
	require.NoError(t, err)

	cleanup := func() {
		_ = os.Remove(file.Name())
	}

	return file.Name(), cleanup
}
