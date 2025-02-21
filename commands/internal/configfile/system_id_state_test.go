//go:build !integration

package configfile

import (
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSystemIDStateLoadFromFile(t *testing.T) {
	tests := map[string]struct {
		contents      string
		validateState func(t *testing.T, s *systemIDState)
	}{
		"parse system_id": {
			contents: `
			s_c2d22f638c25
			`,
			validateState: func(t *testing.T, s *systemIDState) {
				assert.Equal(t, "s_c2d22f638c25", s.GetSystemID())
			},
		},
		"parse empty system_id generates new": {
			contents: "",
			validateState: func(t *testing.T, s *systemIDState) {
				assert.Regexp(t, regexp.MustCompile("[rs]_[0-9a-zA-Z]{12}"), s.GetSystemID())
			},
		},
		"parse invalid system_id generates new": {
			contents: "foooooooor_000000000000barrrrr",
			validateState: func(t *testing.T, s *systemIDState) {
				assert.Regexp(t, regexp.MustCompile("[rs]_[0-9a-zA-Z]{12}"), s.GetSystemID())
			},
		},
		"parse valid system_id with garbage in the file header generates new": {
			contents: `
			garbage
			r_c2d22f638c25`,
			validateState: func(t *testing.T, s *systemIDState) {
				assert.Regexp(t, regexp.MustCompile("[rs]_[0-9a-zA-Z]{12}"), s.GetSystemID())
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			stateFile, err := os.CreateTemp("", ".runner_system_id")
			require.NoError(t, err)
			_, err = stateFile.WriteString(tt.contents)
			require.NoError(t, err)
			_ = stateFile.Close()

			defer func() { _ = os.Remove(stateFile.Name()) }()

			state, err := newSystemIDState(stateFile.Name())
			assert.NoError(t, err)
			if tt.validateState != nil {
				tt.validateState(t, state)
			}
		})
	}
}

func TestSystemIDStateLoadFromMissingFile(t *testing.T) {
	stateFile, err := os.CreateTemp("", ".runner_system_id")
	require.NoError(t, err)
	stateFileName := stateFile.Name()
	_ = os.Remove(stateFileName)

	state, err := newSystemIDState(stateFileName)
	assert.NoError(t, err)
	assert.Regexp(t, regexp.MustCompile("[rs]_[0-9a-zA-Z]{12}"), state.GetSystemID())
}

func TestSaveSystemIDState(t *testing.T) {
	stateFile, err := os.CreateTemp("", ".runner_system_id")
	require.NoError(t, err)
	stateFileName := stateFile.Name()
	_ = stateFile.Close()

	defer func() { _ = os.Remove(stateFileName) }()

	state, err := newSystemIDState(stateFile.Name())
	assert.NoError(t, err)

	buf, err := os.ReadFile(stateFileName)
	require.NoError(t, err)
	assert.Equal(t, state.GetSystemID(), string(buf))
}

func TestSaveSystemIDStateToNonFile(t *testing.T) {
	stateFileName := os.TempDir() + "/."

	_, err := newSystemIDState(stateFileName)
	assert.Error(t, err)
}
