//go:build !integration

package autoscaler

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEnvKeyFields_HappyPath(t *testing.T) {
	tests := []struct {
		name               string
		input              url.Values
		wantAcqKey         string
		wantExecutorFields url.Values
	}{
		{
			name: "simple split",
			input: url.Values{
				"acquisition-key": []string{"abc"},
				"x":               []string{"xyz"},
			},
			wantAcqKey:         "abc",
			wantExecutorFields: url.Values{"x": []string{"xyz"}},
		},
		{
			name: "multi-value executor field preserved",
			input: url.Values{
				"acquisition-key": []string{"k"},
				"x":               []string{"a", "b"},
			},
			wantAcqKey:         "k",
			wantExecutorFields: url.Values{"x": []string{"a", "b"}},
		},
		{
			name: "acq-key only no executor fields",
			input: url.Values{
				"acquisition-key": []string{"only"},
			},
			wantAcqKey:         "only",
			wantExecutorFields: url.Values{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, executorFields, err := parseEnvKeyFields(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.wantAcqKey, data.acqKey)
			assert.Equal(t, tt.wantExecutorFields, executorFields)
		})
	}
}

func TestParseEnvKeyFields_Errors(t *testing.T) {
	tests := []struct {
		name          string
		input         url.Values
		wantErrSubstr string
	}{
		{
			name:          "acquisition-key absent",
			input:         url.Values{},
			wantErrSubstr: "acquisition-key is required",
		},
		{
			name:          "acquisition-key empty",
			input:         url.Values{"acquisition-key": []string{""}},
			wantErrSubstr: "acquisition-key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := parseEnvKeyFields(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErrSubstr)
		})
	}
}

func TestParseEnvKeyFields_ReturnedFieldsAreFreshMap(t *testing.T) {
	input := url.Values{
		"acquisition-key": []string{"abc"},
		"x":               []string{"xyz"},
	}

	_, executorFields, err := parseEnvKeyFields(input)
	require.NoError(t, err)

	// Mutating the returned bag must not affect the input.
	executorFields.Set("x", "mutated")
	executorFields.Set("new-key", "new-value")

	assert.Equal(t, "xyz", input.Get("x"), "input x should be unchanged")
	assert.Empty(t, input.Get("new-key"), "input must not gain new-key from mutation of returned bag")
}
