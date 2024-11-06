//go:build !integration

package runner_wrapper

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultCommander_Start(t *testing.T) {
	const (
		commandPath = "unknown-binary"
	)

	c := newDefaultCommander(commandPath, []string{})
	assert.Nil(t, c.Process())

	err := c.Start()
	var eerr *exec.Error
	if assert.ErrorAs(t, err, &eerr) {
		assert.Equal(t, commandPath, eerr.Name)
	}
}

func TestDefaultCommander_Wait(t *testing.T) {
	// Adding the `.exe` extension as otherwise the binary will not be
	// executable when tests are executed on Windows
	testBinary := filepath.Join(os.TempDir(), fmt.Sprintf("commander-binary-%d.exe", time.Now().UnixNano()))
	defer func() {
		_ = os.Remove(testBinary)
	}()

	ctx, cancelFn := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFn()

	t.Log("building test binary", testBinary)
	cmd := exec.CommandContext(ctx, "go", "build", "-o", testBinary, "./testdata/commander-binary/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	require.NoError(t, cmd.Run())
	t.Log("test binary built")

	tests := map[string]struct {
		args        []string
		assertError func(t *testing.T, err error)
	}{
		"failed execution": {
			args: []string{"fail"},
			assertError: func(t *testing.T, err error) {
				var eerr *exec.ExitError
				if assert.ErrorAs(t, err, &eerr) {
					assert.Equal(t, 1, eerr.ExitCode())
				}
			},
		},
		"successful execution": {},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			c, ok := newDefaultCommander(testBinary, tc.args).(*defaultCommander)
			require.True(t, ok)

			c.cmd.Stdout = io.Discard
			c.cmd.Stderr = io.Discard

			require.NoError(t, c.Start())
			assert.NotNil(t, c.Process())

			err := c.Wait()
			if tc.assertError != nil {
				tc.assertError(t, err)
				return
			}

			assert.NoError(t, err)
		})
	}
}
