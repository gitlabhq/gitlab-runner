// Helper functions that are shared between unit tests and integration tests

package commands

import (
	"bufio"
	"bytes"
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
