//go:build !integration

package common

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
		validateState func(t *testing.T, s *SystemIDState)
	}{
		"parse system_id": {
			contents: `
			s_c2d22f638c25
			`,
			validateState: func(t *testing.T, s *SystemIDState) {
				assert.Equal(t, "s_c2d22f638c25", s.GetSystemID())
			},
		},
		"parse empty system_id": {
			contents: "",
			validateState: func(t *testing.T, s *SystemIDState) {
				assert.Empty(t, s.GetSystemID())
			},
		},
		"parse invalid system_id": {
			contents: "foooooooor_000000000000barrrrr",
			validateState: func(t *testing.T, s *SystemIDState) {
				assert.Empty(t, s.GetSystemID())
			},
		},
		"parse valid system_id with garbage in the file header": {
			contents: `
			garbage
			r_c2d22f638c25`,
			validateState: func(t *testing.T, s *SystemIDState) {
				assert.Empty(t, s.GetSystemID())
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

			state := NewSystemIDState()
			err = state.LoadFromFile(stateFile.Name())
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

	state := NewSystemIDState()
	err = state.LoadFromFile(stateFileName)
	assert.NoError(t, err)
	assert.Empty(t, state.GetSystemID())
}

func TestEnsureSystemID(t *testing.T) {
	tests := map[string]struct {
		contents string
		assertFn func(t *testing.T, config *SystemIDState)
	}{
		"preserves system_id": {
			contents: `
			s_c2d22f638c25
			`,
			assertFn: func(t *testing.T, config *SystemIDState) {
				assert.Equal(t, "s_c2d22f638c25", config.GetSystemID())
			},
		},
		"generates missing system_id": {
			contents: "",
			assertFn: func(t *testing.T, config *SystemIDState) {
				assert.Regexp(t, regexp.MustCompile("[rs]_[0-9a-zA-Z]{12}"), config.GetSystemID())
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

			state := NewSystemIDState()
			err = state.LoadFromFile(stateFile.Name())
			assert.NoError(t, err)

			err = state.EnsureSystemID()
			assert.NoError(t, err)
			if tt.assertFn != nil {
				tt.assertFn(t, state)
			}
		})
	}
}

func TestSaveSystemIDState(t *testing.T) {
	stateFile, err := os.CreateTemp("", ".runner_system_id")
	require.NoError(t, err)
	stateFileName := stateFile.Name()
	_ = stateFile.Close()

	defer func() { _ = os.Remove(stateFileName) }()

	state := NewSystemIDState()
	err = state.SaveConfig(stateFile.Name())
	assert.NoError(t, err)

	buf, err := os.ReadFile(stateFileName)
	require.NoError(t, err)
	assert.Equal(t, state.GetSystemID(), string(buf))
}

func TestSaveSystemIDStateToNonFile(t *testing.T) {
	stateFileName := os.TempDir() + "/."

	state := NewSystemIDState()
	err := state.SaveConfig(stateFileName)
	assert.Error(t, err)
}
