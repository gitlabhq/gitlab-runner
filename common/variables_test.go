package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVariablesJSON(t *testing.T) {
	var x JobVariable
	data := []byte(`{"key": "FOO", "value": "bar", "public": true, "internal": true, "file": true}`)

	err := json.Unmarshal(data, &x)
	assert.NoError(t, err)
	assert.Equal(t, "FOO", x.Key)
	assert.Equal(t, "bar", x.Value)
	assert.Equal(t, true, x.Public)
	assert.Equal(t, false, x.Internal) // cannot be set from the network
	assert.Equal(t, true, x.File)
}

func TestVariableString(t *testing.T) {
	v := JobVariable{"key", "value", false, false, false}
	assert.Equal(t, "key=value", v.String())
}

func TestPublicAndInternalVariables(t *testing.T) {
	v1 := JobVariable{"key", "value", false, false, false}
	v2 := JobVariable{"public", "value", true, false, false}
	v3 := JobVariable{"private", "value", false, true, false}
	all := JobVariables{v1, v2, v3}
	public := all.PublicOrInternal()
	assert.NotContains(t, public, v1)
	assert.Contains(t, public, v2)
	assert.Contains(t, public, v3)
}

func TestListVariables(t *testing.T) {
	v := JobVariables{{"key", "value", false, false, false}}
	assert.Equal(t, []string{"key=value"}, v.StringList())
}

func TestGetVariable(t *testing.T) {
	v1 := JobVariable{"key", "key_value", false, false, false}
	v2 := JobVariable{"public", "public_value", true, false, false}
	v3 := JobVariable{"private", "private_value", false, false, false}
	all := JobVariables{v1, v2, v3}

	assert.Equal(t, "public_value", all.Get("public"))
	assert.Empty(t, all.Get("other"))
}

func TestParseVariable(t *testing.T) {
	v, err := ParseVariable("key=value=value2")
	assert.NoError(t, err)
	assert.Equal(t, JobVariable{"key", "value=value2", false, false, false}, v)
}

func TestInvalidParseVariable(t *testing.T) {
	_, err := ParseVariable("some_other_key")
	assert.Error(t, err)
}

func TestVariablesExpansion(t *testing.T) {
	all := JobVariables{
		{"key", "value_of_$public", false, false, false},
		{"public", "some_value", true, false, false},
		{"private", "value_of_${public}", false, false, false},
		{"public", "value_of_$undefined", true, false, false},
	}

	expanded := all.Expand()
	assert.Len(t, expanded, 4)
	assert.Equal(t, "value_of_value_of_$undefined", expanded.Get("key"))
	assert.Equal(t, "value_of_", expanded.Get("public"))
	assert.Equal(t, "value_of_value_of_$undefined", expanded.Get("private"))
	assert.Equal(t, "value_of_ value_of_value_of_$undefined", expanded.ExpandValue("${public} ${private}"))
}

func TestSpecialVariablesExpansion(t *testing.T) {
	all := JobVariables{
		{"key", "$$", false, false, false},
		{"key2", "$/dsa", true, false, false},
		{"key3", "aa$@bb", false, false, false},
		{"key4", "aa${@}bb", false, false, false},
	}

	expanded := all.Expand()
	assert.Len(t, expanded, 4)
	assert.Equal(t, "$", expanded.Get("key"))
	assert.Equal(t, "/dsa", expanded.Get("key2"))
	assert.Equal(t, "aabb", expanded.Get("key3"))
	assert.Equal(t, "aabb", expanded.Get("key4"))
}

type multipleKeyUsagesTestCase struct {
	variables     JobVariables
	expectedValue string
}

func TestMultipleUsageOfAKey(t *testing.T) {
	getVariable := func(value string) JobVariable {
		return JobVariable{Key: "key", Value: value}
	}

	tests := map[string]multipleKeyUsagesTestCase{
		"defined at job level": {
			variables: JobVariables{
				getVariable("from-job"),
			},
			expectedValue: "from-job",
		},
		"defined at default and job level": {
			variables: JobVariables{
				getVariable("from-default"),
				getVariable("from-job"),
			},
			expectedValue: "from-job",
		},
		"defined at config, default and job level": {
			variables: JobVariables{
				getVariable("from-config"),
				getVariable("from-default"),
				getVariable("from-job"),
			},
			expectedValue: "from-job",
		},
		"defined at config and default level": {
			variables: JobVariables{
				getVariable("from-config"),
				getVariable("from-default"),
			},
			expectedValue: "from-default",
		},
		"defined at config level": {
			variables: JobVariables{
				getVariable("from-config"),
			},
			expectedValue: "from-config",
		},
	}

	for name, testCase := range tests {
		t.Run(name, func(t *testing.T) {
			for i := 0; i < 100; i++ {
				require.Equal(t, testCase.expectedValue, testCase.variables.Get("key"))
			}
		})
	}

}
