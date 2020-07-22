package common

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVariablesJSON(t *testing.T) {
	var x JobVariable
	data := []byte(
		`{"key": "FOO", "value": "bar", "public": true, "internal": true, "file": true, "masked": true, "raw": true}`,
	)

	err := json.Unmarshal(data, &x)
	assert.NoError(t, err)
	assert.Equal(t, "FOO", x.Key)
	assert.Equal(t, "bar", x.Value)
	assert.True(t, x.Public)
	assert.False(t, x.Internal) // cannot be set from the network
	assert.True(t, x.File)
	assert.True(t, x.Masked)
	assert.True(t, x.Raw)
}

func TestVariableString(t *testing.T) {
	v := JobVariable{Key: "key", Value: "value"}
	assert.Equal(t, "key=value", v.String())
}

func TestPublicAndInternalVariables(t *testing.T) {
	v1 := JobVariable{Key: "key", Value: "value"}
	v2 := JobVariable{Key: "public", Value: "value", Public: true}
	v3 := JobVariable{Key: "private", Value: "value", Internal: true}
	all := JobVariables{v1, v2, v3}
	public := all.PublicOrInternal()
	assert.NotContains(t, public, v1)
	assert.Contains(t, public, v2)
	assert.Contains(t, public, v3)
}

func TestMaskedVariables(t *testing.T) {
	v1 := JobVariable{Key: "key", Value: "key_value"}
	v2 := JobVariable{Key: "masked", Value: "masked_value", Masked: true}
	all := JobVariables{v1, v2}
	masked := all.Masked()
	assert.NotContains(t, masked, v1.Value)
	assert.Contains(t, masked, v2.Value)
}

func TestListVariables(t *testing.T) {
	v := JobVariables{{Key: "key", Value: "value"}}
	assert.Equal(t, []string{"key=value"}, v.StringList())
}

func TestGetVariable(t *testing.T) {
	v1 := JobVariable{Key: "key", Value: "key_value"}
	v2 := JobVariable{Key: "public", Value: "public_value", Public: true}
	v3 := JobVariable{Key: "private", Value: "private_value"}
	all := JobVariables{v1, v2, v3}

	assert.Equal(t, "public_value", all.Get("public"))
	assert.Empty(t, all.Get("other"))
}

func TestParseVariable(t *testing.T) {
	v, err := ParseVariable("key=value=value2")
	assert.NoError(t, err)
	assert.Equal(t, JobVariable{Key: "key", Value: "value=value2"}, v)
}

func TestInvalidParseVariable(t *testing.T) {
	_, err := ParseVariable("some_other_key")
	assert.Error(t, err)
}

func TestVariablesExpansion(t *testing.T) {
	all := JobVariables{
		{Key: "key", Value: "value_of_$public"},
		{Key: "public", Value: "some_value", Public: true},
		{Key: "private", Value: "value_of_${public}"},
		{Key: "public", Value: "value_of_$undefined", Public: true},
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
		{Key: "key", Value: "$$"},
		{Key: "key2", Value: "$/dsa", Public: true},
		{Key: "key3", Value: "aa$@bb"},
		{Key: "key4", Value: "aa${@}bb"},
	}

	expanded := all.Expand()
	assert.Len(t, expanded, 4)
	assert.Equal(t, "$", expanded.Get("key"))
	assert.Equal(t, "$/dsa", expanded.Get("key2"))
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

func TestRawVariableExpansion(t *testing.T) {
	tests := map[bool]string{
		true:  "value_of_${base}",
		false: "value_of_base_value",
	}

	for raw, expectedValue := range tests {
		t.Run(fmt.Sprintf("raw-%v", raw), func(t *testing.T) {
			variables := JobVariables{
				{Key: "base", Value: "base_value"},
				{Key: "related", Value: "value_of_${base}", Raw: raw},
			}

			expanded := variables.Expand()
			assert.Equal(t, expectedValue, expanded.Get("related"))
		})
	}
}
