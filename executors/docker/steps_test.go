//go:build !integration

package docker

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/internal/readywriter"
)

// stepsProxyFrame is one frame of the stream docker attaches to an exec
// started without a TTY, such as `steps proxy`.
type stepsProxyFrame struct {
	stream stdcopy.StdType
	data   string
}

// stepsProxyStream multiplexes frames the way docker does, so that
// stdcopy.StdCopy can split them again.
func stepsProxyStream(frames ...stepsProxyFrame) *bytes.Buffer {
	var buf bytes.Buffer
	for _, f := range frames {
		var header [8]byte
		header[0] = byte(f.stream)
		binary.BigEndian.PutUint32(header[4:], uint32(len(f.data)))
		buf.Write(header[:])
		buf.WriteString(f.data)
	}

	return &buf
}

func TestCopyStepsProxyOutput(t *testing.T) {
	errReset := errors.New("connection reset")

	tests := map[string]struct {
		src       io.Reader
		wantOut   string
		wantErr   string
		wantErrIs error
	}{
		"stdout only": {
			src: stepsProxyStream(
				stepsProxyFrame{stdcopy.Stdout, "first|"},
				stepsProxyFrame{stdcopy.Stdout, "second|"},
			),
			wantOut: "first|second|",
		},
		"source fails": {
			src: io.MultiReader(
				stepsProxyStream(stepsProxyFrame{stdcopy.Stdout, "first|"}),
				iotest.ErrReader(errReset),
			),
			wantOut:   "first|",
			wantErr:   "connection reset",
			wantErrIs: errReset,
		},
		"stderr between stdout frames": {
			src: stepsProxyStream(
				stepsProxyFrame{stdcopy.Stdout, "first|"},
				stepsProxyFrame{stdcopy.Stderr, "a warning\n"},
				stepsProxyFrame{stdcopy.Stdout, "second|"},
			),
			wantOut: "first|second|",
			wantErr: "steps proxy: a warning\n",
		},
		"stderr then end of stream": {
			src: stepsProxyStream(
				stepsProxyFrame{stdcopy.Stderr, "dialing: connect: no such file or directory\n"},
			),
			wantErr: "steps proxy: dialing: connect: no such file or directory\n",
		},
		"source fails after stderr": {
			src: io.MultiReader(
				stepsProxyStream(stepsProxyFrame{stdcopy.Stderr, "a warning\n"}),
				iotest.ErrReader(errReset),
			),
			wantErr:   "connection reset: steps proxy: a warning\n",
			wantErrIs: errReset,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r, w := io.Pipe()
			go copyStepsProxyOutput(w, tc.src)

			out, err := io.ReadAll(r)
			assert.Equal(t, tc.wantOut, string(out))
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}

			assert.EqualError(t, err, tc.wantErr)
			if tc.wantErrIs != nil {
				assert.ErrorIs(t, err, tc.wantErrIs)
			}
		})
	}
}

// In "job script as entrypoint" mode the job's image writes to the build
// container output before step-runner starts, so it can print the ready line.
// A malformed one must be a script failure, not left unclassified:
// wrapStepStageErr reports an unclassified Connect() failure as a runner
// system failure, which a job could otherwise force.
func TestReadySocketPath(t *testing.T) {
	tests := map[string]struct {
		line         string
		expectedPath string
		expectedErr  string
	}{
		"socket path": {
			line:         "step-runner is listening on socket /run/step-runner.sock\n",
			expectedPath: "/run/step-runner.sock",
		},
		"empty socket path": {
			line:        "step-runner is listening on socket \n",
			expectedErr: "missing socket path",
		},
		"overlong socket path": {
			line:        "step-runner is listening on socket /" + strings.Repeat("a", 4*1024) + "\n",
			expectedErr: "ready channel closed",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			rw, readyCh := readywriter.New(t.Context(), io.Discard)
			_, err := rw.Write([]byte(tc.line))
			require.NoError(t, err)

			path, ok := <-readyCh
			socketPath, err := readySocketPath(path, ok)

			if tc.expectedErr == "" {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedPath, socketPath)
				return
			}

			var buildErr *common.BuildError
			require.ErrorAs(t, err, &buildErr)
			assert.Equal(t, common.ScriptFailure, buildErr.FailureReason)
			assert.Contains(t, err.Error(), tc.expectedErr)
		})
	}
}
