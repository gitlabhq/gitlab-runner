//go:build !integration

package spec

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVariablesJSON(t *testing.T) {
	var x Variable
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
	v := Variable{Key: "key", Value: "value"}
	assert.Equal(t, "key=value", v.String())
}

func TestPublicAndInternalVariables(t *testing.T) {
	v1 := Variable{Key: "key", Value: "value"}
	v2 := Variable{Key: "public", Value: "value", Public: true}
	v3 := Variable{Key: "private", Value: "value", Internal: true}
	all := Variables{v1, v2, v3}
	public := all.PublicOrInternal()
	assert.NotContains(t, public, v1)
	assert.Contains(t, public, v2)
	assert.Contains(t, public, v3)
}

func TestMaskedVariables(t *testing.T) {
	v1 := Variable{Key: "key", Value: "key_value"}
	v2 := Variable{Key: "masked", Value: "masked_value", Masked: true}
	all := Variables{v1, v2}
	masked := all.Masked()
	assert.NotContains(t, masked, v1.Value)
	assert.Contains(t, masked, v2.Value)
}

func TestListVariables(t *testing.T) {
	v := Variables{
		{Key: "key", Value: "value"},
		{Key: "fileKey", Value: "fileValue", File: true},
		{Key: "RUNNER_TEMP_PROJECT_DIR", Value: "/foo/bar", Public: true, Internal: true},
	}

	stringList := v.StringList()

	assert.Len(t, stringList, 3)
	assert.Equal(t, "key=value", stringList[0])
	assert.Equal(t, "fileKey=/foo/bar/fileKey", stringList[1])
	assert.Equal(t, "RUNNER_TEMP_PROJECT_DIR=/foo/bar", stringList[2])
}

func TestGetVariable(t *testing.T) {
	v1 := Variable{Key: "key", Value: "key_value"}
	v2 := Variable{Key: "public", Value: "public_value", Public: true}
	v3 := Variable{Key: "private", Value: "private_value"}
	all := Variables{v1, v2, v3}

	assert.Equal(t, "public_value", all.Get("public"))
	assert.Empty(t, all.Get("other"))
}

func TestVariablesExpansion(t *testing.T) {
	all := Variables{
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

func TestFileVariablesExpansion(t *testing.T) {
	all := Variables{
		{Key: "a_file_var", Value: "some top secret stuff", File: true},
		{Key: "ref_file_var", Value: "${a_file_var}.txt"},
		{Key: "regular_var", Value: "bla bla bla"},
		{Key: "ref_regular_var", Value: "bla bla bla"},
		{Key: "RUNNER_TEMP_PROJECT_DIR", Value: "/foo/bar", Public: true, Internal: true},
	}

	validate := func(t *testing.T, variables Variables) {
		assert.Len(t, variables, 5)

		// correct expansion of file variables
		assert.Equal(t, "/foo/bar/a_file_var", variables.Get("a_file_var"))
		assert.Equal(t, "some top secret stuff", variables.Value("a_file_var"))

		// correct expansion of variables that reference file variables
		assert.Equal(t, "/foo/bar/a_file_var.txt", variables.Get("ref_file_var"))
		assert.Equal(t, "/foo/bar/a_file_var.txt", variables.Value("ref_file_var"))
		assert.Equal(t, "/foo/bar/a_file_var.txt.blammo", variables.ExpandValue("${ref_file_var}.blammo"))
		assert.Equal(t, "/foo/bar/a_file_var.blammo", variables.ExpandValue("${a_file_var}.blammo"))

		// correct expansion of regular variables, and variables that reference
		// regular variables
		assert.Equal(t, "bla bla bla", variables.Get("regular_var"))
		assert.Equal(t, "bla bla bla", variables.Get("ref_regular_var"))
		assert.Equal(t, "bla bla bla", variables.Value("regular_var"))
		assert.Equal(t, "bla bla bla", variables.Value("ref_regular_var"))
	}

	expanded := all.Expand()
	validate(t, expanded)
	// calling Expand multiple times is idempotent.
	validate(t, expanded.Expand())
}

func TestSpecialVariablesExpansion(t *testing.T) {
	all := Variables{
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

func TestOverwriteKey(t *testing.T) {
	vars := Variables{
		{Key: "hello", Value: "world"},
		{Key: "foo", Value: ""},
	}

	// Overwrite empty value
	vars.OverwriteKey("foo", Variable{Key: "foo", Value: "bar"})

	assert.Equal(t, "world", vars.Get("hello"))
	assert.Equal(t, "bar", vars.Get("foo"))

	// Overwrite existing value
	vars.OverwriteKey("hello", Variable{Key: "hello", Value: "universe"})

	assert.Equal(t, "universe", vars.Get("hello"))
	assert.Equal(t, "bar", vars.Get("foo"))

	// Overwrite key
	vars.OverwriteKey("hello", Variable{Key: "goodbye", Value: "universe"})

	assert.Equal(t, "universe", vars.Get("goodbye"))
	assert.Equal(t, "", vars.Get("hello"))
	assert.Equal(t, "bar", vars.Get("foo"))

	// Overwrite properties
	fooOverwriteVar := Variable{
		Key:      "foo",
		Value:    "baz",
		Public:   true,
		Internal: true,
		File:     true,
		Masked:   true,
		Raw:      true,
	}
	vars.OverwriteKey("foo", fooOverwriteVar)

	assert.Equal(t, fooOverwriteVar, vars[1])
}

type multipleKeyUsagesTestCase struct {
	variables     Variables
	expectedValue string
}

func TestMultipleUsageOfAKey(t *testing.T) {
	getVariable := func(value string) Variable {
		return Variable{Key: "key", Value: value}
	}

	tests := map[string]multipleKeyUsagesTestCase{
		"defined at job level": {
			variables: Variables{
				getVariable("from-job"),
			},
			expectedValue: "from-job",
		},
		"defined at default and job level": {
			variables: Variables{
				getVariable("from-default"),
				getVariable("from-job"),
			},
			expectedValue: "from-job",
		},
		"defined at config, default and job level": {
			variables: Variables{
				getVariable("from-config"),
				getVariable("from-default"),
				getVariable("from-job"),
			},
			expectedValue: "from-job",
		},
		"defined at config and default level": {
			variables: Variables{
				getVariable("from-config"),
				getVariable("from-default"),
			},
			expectedValue: "from-default",
		},
		"defined at config level": {
			variables: Variables{
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
			variables := Variables{
				{Key: "base", Value: "base_value"},
				{Key: "related", Value: "value_of_${base}", Raw: raw},
			}

			expanded := variables.Expand()
			assert.Equal(t, expectedValue, expanded.Get("related"))
		})
	}
}

func TestBoolVariables(t *testing.T) {
	tests := map[string]bool{
		"true":           true,
		"TRUE":           true,
		"tRuE":           true,
		"false":          false,
		"FALSE":          false,
		"fAlsE":          false,
		"1":              true,
		"-1":             false,
		"0":              false,
		"100":            false,
		"":               false,
		"something else": false,
	}

	for value, expected := range tests {
		t.Run(value, func(t *testing.T) {
			v := Variables{
				{Key: "variable", Value: value},
			}

			result := v.Bool("variable")
			require.Equal(t, expected, result)
		})
	}
}

func Test_JobVariables_Set(t *testing.T) {
	tests := map[string]struct {
		jobVars  Variables
		set      Variables
		expected []string
	}{
		"noop": {},
		"add one": {
			set: Variables{
				{Key: "foo", Value: "don't use that foo"},
				{Key: "foo", Value: "the new foo"},
			},
			expected: []string{"foo=the new foo"},
		},
		"overwrite one": {
			jobVars: Variables{
				{Key: "foo", Value: "this foo gets overridden"},
				{Key: "foo", Value: "this one too"},
			},
			set: Variables{
				{Key: "foo", Value: "new foo"},
			},

			expected: []string{"foo=new foo"},
		},
		"overwrite and add": {
			jobVars: Variables{
				{Key: "foo", Value: "this foo gets overridden"},
				{Key: "org", Value: "the org keeps as is"},
				{Key: "foo", Value: "this one too"},
			},
			set: Variables{
				{Key: "bar", Value: "don't use that bar"},
				{Key: "foo", Value: "new foo"},
				{Key: "bar", Value: "new bar"},
			},
			expected: []string{"foo=new foo", "bar=new bar", "org=the org keeps as is"},
		},
		"duplicates are preserved if not set": {
			jobVars: Variables{
				{Key: "foo", Value: "1st foo"},
				{Key: "blerp", Value: "nope"},
				{Key: "foo", Value: "2nd foo"},
				{Key: "foo", Value: "3rd foo"},
			},
			set: Variables{
				{Key: "blerp", Value: "blerp!"},
			},
			expected: []string{"blerp=blerp!", "foo=1st foo", "foo=2nd foo", "foo=3rd foo"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			jv := test.jobVars
			jv.Set(test.set...)
			actual := jv.StringList()
			assert.ElementsMatch(t, actual, test.expected)
		})
	}
}

func Test_JobVariables_Dedup(t *testing.T) {
	vars := Variables{
		{Key: "foo-key", Value: "foo"},
		{Key: "some-key", Value: "this is the original"},
		{Key: "bar-key", Value: "bar"},
		{Key: "some-key", Value: "this is unused"},
		{Key: "baz-key", Value: "baz"},
		{Key: "some-key", Value: "this is overridden"},
		{Key: "blerp-key", Value: "blerp"},
	}

	tests := []struct {
		name         string
		keepOriginal bool
		expectedVars Variables
	}{
		{
			name: "keep overridden",
			expectedVars: Variables{
				{Key: "bar-key", Value: "bar"},
				{Key: "baz-key", Value: "baz"},
				{Key: "blerp-key", Value: "blerp"},
				{Key: "foo-key", Value: "foo"},
				{Key: "some-key", Value: "this is overridden"},
			},
		},
		{
			name:         "keep original",
			keepOriginal: true,
			expectedVars: Variables{
				{Key: "bar-key", Value: "bar"},
				{Key: "baz-key", Value: "baz"},
				{Key: "blerp-key", Value: "blerp"},
				{Key: "foo-key", Value: "foo"},
				{Key: "some-key", Value: "this is the original"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedVars, vars.Dedup(tc.keepOriginal))
		})
	}
}
