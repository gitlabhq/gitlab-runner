//go:build !integration

package common

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvironmentKey_String(t *testing.T) {
	tests := []struct {
		name string
		key  EnvironmentKey
		want string
	}{
		{
			name: "simple key with single field",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-1",
				Fields:   url.Values{"acquisition-key": []string{"abc-123"}},
			},
			want: "1/sys-1/acquisition-key=abc-123",
		},
		{
			name: "multiple fields are sorted alphabetically by key",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-1",
				Fields: url.Values{
					"zebra": []string{"z-val"},
					"alpha": []string{"a-val"},
					"mid":   []string{"m-val"},
				},
			},
			want: "1/sys-1/alpha=a-val&mid=m-val&zebra=z-val",
		},
		{
			name: "empty fields produce trailing slash",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-1",
				Fields:   url.Values{},
			},
			want: "1/sys-1/",
		},
		{
			name: "slash in SystemID is percent-encoded as %2F",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys/with/slashes",
				Fields:   url.Values{"k": []string{"v"}},
			},
			want: "1/sys%2Fwith%2Fslashes/k=v",
		},
		{
			name: "ampersand in field value is percent-encoded",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-1",
				Fields:   url.Values{"k": []string{"a&b"}},
			},
			want: "1/sys-1/k=a%26b",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.key.String())
		})
	}
}

func TestEnvironmentKey_String_DeterministicAcrossInsertionOrder(t *testing.T) {
	k1 := EnvironmentKey{RunnerID: 1, SystemID: "sys-1", Fields: url.Values{}}
	k1.Fields.Set("zebra", "z-val")
	k1.Fields.Set("alpha", "a-val")
	k1.Fields.Set("mid", "m-val")

	k2 := EnvironmentKey{RunnerID: 1, SystemID: "sys-1", Fields: url.Values{}}
	k2.Fields.Set("alpha", "a-val")
	k2.Fields.Set("mid", "m-val")
	k2.Fields.Set("zebra", "z-val")

	assert.Equal(t, k1.String(), k2.String())
}

func TestEnvironmentKey_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		key  EnvironmentKey
	}{
		{
			name: "simple key with single field",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-1",
				Fields:   url.Values{"acquisition-key": []string{"abc-123"}},
			},
		},
		{
			name: "multiple fields",
			key: EnvironmentKey{
				RunnerID: 42,
				SystemID: "machine-abc",
				Fields: url.Values{
					"acquisition-key": []string{"uuid-1234"},
					"container-id":    []string{"container-5678"},
				},
			},
		},
		{
			name: "empty fields",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-1",
				Fields:   url.Values{},
			},
		},
		{
			name: "multi-value field",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-1",
				Fields:   url.Values{"tags": []string{"alpha", "beta", "gamma"}},
			},
		},
		{
			name: "slash in SystemID",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys/with/slashes",
				Fields:   url.Values{"k": []string{"v"}},
			},
		},
		{
			name: "question mark in SystemID",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys?question",
				Fields:   url.Values{"k": []string{"v"}},
			},
		},
		{
			name: "hash in SystemID",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys#hash",
				Fields:   url.Values{"k": []string{"v"}},
			},
		},
		{
			name: "percent in SystemID",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys%percent",
				Fields:   url.Values{"k": []string{"v"}},
			},
		},
		{
			name: "space in SystemID",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys with space",
				Fields:   url.Values{"k": []string{"v"}},
			},
		},
		{
			name: "unicode in SystemID",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-é-unicode",
				Fields:   url.Values{"k": []string{"v"}},
			},
		},
		{
			name: "quotes in field value",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-1",
				Fields:   url.Values{"key-with-quotes": []string{`value with "quotes"`}},
			},
		},
		{
			name: "backslash in field value",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-1",
				Fields: url.Values{
					"key-with-backslash": []string{`path\to\file`},
					"path":               []string{`C:\Users\`},
				},
			},
		},
		{
			name: "commas in field value",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-1",
				Fields:   url.Values{"key-with-commas": []string{"value,with,commas"}},
			},
		},
		{
			name: "ampersand in field value",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-1",
				Fields:   url.Values{"key-with-amp": []string{"a&b=c"}},
			},
		},
		{
			name: "equals in field value",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-1",
				Fields:   url.Values{"key-with-equals": []string{"k=v=w"}},
			},
		},
		{
			name: "empty field value",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-1",
				Fields:   url.Values{"key": []string{""}},
			},
		},
		{
			name: "unicode in field value",
			key: EnvironmentKey{
				RunnerID: 1,
				SystemID: "sys-1",
				Fields:   url.Values{"greeting": []string{"héllo-世界"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := ParseEnvironmentKey(tt.key.String())
			require.NoError(t, err)
			assert.Equal(t, tt.key.RunnerID, parsed.RunnerID)
			assert.Equal(t, tt.key.SystemID, parsed.SystemID)
			assert.Equal(t, tt.key.Fields, parsed.Fields)
		})
	}
}

func TestParseEnvironmentKey_Errors(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantErrSubstr string
	}{
		{
			name:          "empty string",
			input:         "",
			wantErrSubstr: "expected at least two '/' separators",
		},
		{
			name:          "no slashes at all",
			input:         "no-slash-here",
			wantErrSubstr: "expected at least two '/' separators",
		},
		{
			name:          "only one slash (missing system ID segment)",
			input:         "1/acquisition-key=abc",
			wantErrSubstr: "expected at least two '/' separators",
		},
		{
			name:          "empty runner ID",
			input:         "/sys-1/acquisition-key=abc",
			wantErrSubstr: "invalid runner ID",
		},
		{
			name:          "non-numeric runner ID",
			input:         "abc/sys-1/foo=bar",
			wantErrSubstr: "invalid runner ID",
		},
		{
			name:          "zero runner ID",
			input:         "0/sys-1/acquisition-key=abc",
			wantErrSubstr: "runner ID must be positive",
		},
		{
			name:          "negative runner ID",
			input:         "-1/sys-1/acquisition-key=abc",
			wantErrSubstr: "runner ID must be positive",
		},
		{
			name:          "empty system ID",
			input:         "1//foo=bar",
			wantErrSubstr: "system ID is empty",
		},
		{
			name:          "invalid percent encoding in system ID",
			input:         "1/sys%ZZ/foo=bar",
			wantErrSubstr: "invalid system ID encoding",
		},
		{
			name:          "invalid percent encoding in fields",
			input:         "1/sys-1/foo=%ZZ",
			wantErrSubstr: "invalid fields",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseEnvironmentKey(tt.input)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "environment key:")
			assert.Contains(t, err.Error(), tt.wantErrSubstr)
		})
	}
}
