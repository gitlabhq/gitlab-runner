//go:build !integration

package common

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// predefinedServerJobVariables are variables that _only_ come from the CI
// server.
//
// This list was extracted from:
// https://docs.gitlab.com/ee/ci/variables/predefined_variables.html#predefined-environment-variables-reference
//
// handy console js:
// console.log(Object.values($("tr td:first-child code").map((_, val) => val.innerText)).join("\n"))
//
// commented out variables are non-server ci variables, they are handy to keep
// here for reference/future update updating.
var predefinedServerJobVariables = []string{
	"CHAT_CHANNEL",
	"CHAT_INPUT",
	"CI",
	"CI_API_V4_URL",
	// "CI_BUILDS_DIR",
	"CI_COMMIT_BEFORE_SHA",
	"CI_COMMIT_DESCRIPTION",
	"CI_COMMIT_MESSAGE",
	"CI_COMMIT_REF_NAME",
	"CI_COMMIT_REF_PROTECTED",
	"CI_COMMIT_REF_SLUG",
	"CI_COMMIT_SHA",
	"CI_COMMIT_SHORT_SHA",
	"CI_COMMIT_BRANCH",
	"CI_COMMIT_TAG",
	"CI_COMMIT_TITLE",
	"CI_COMMIT_TIMESTAMP",
	// "CI_CONCURRENT_ID",
	// "CI_CONCURRENT_PROJECT_ID",
	"CI_CONFIG_PATH",
	"CI_DEBUG_TRACE",
	"CI_DEFAULT_BRANCH",
	"CI_DEPLOY_FREEZE",
	"CI_DEPLOY_PASSWORD",
	"CI_DEPLOY_USER",
	// "CI_DISPOSABLE_ENVIRONMENT",
	"CI_ENVIRONMENT_NAME",
	"CI_ENVIRONMENT_SLUG",
	"CI_ENVIRONMENT_URL",
	"CI_EXTERNAL_PULL_REQUEST_IID",
	"CI_EXTERNAL_PULL_REQUEST_SOURCE_REPOSITORY",
	"CI_EXTERNAL_PULL_REQUEST_TARGET_REPOSITORY",
	"CI_EXTERNAL_PULL_REQUEST_SOURCE_BRANCH_NAME",
	"CI_EXTERNAL_PULL_REQUEST_SOURCE_BRANCH_SHA",
	"CI_EXTERNAL_PULL_REQUEST_TARGET_BRANCH_NAME",
	"CI_EXTERNAL_PULL_REQUEST_TARGET_BRANCH_SHA",
	"CI_HAS_OPEN_REQUIREMENTS",
	"CI_JOB_ID",
	"CI_JOB_IMAGE",
	"CI_JOB_MANUAL",
	"CI_JOB_NAME",
	"CI_JOB_STAGE",
	"CI_JOB_TOKEN",
	"CI_JOB_JWT",
	"CI_JOB_URL",
	"CI_KUBERNETES_ACTIVE",
	"CI_MERGE_REQUEST_ASSIGNEES",
	"CI_MERGE_REQUEST_ID",
	"CI_MERGE_REQUEST_IID",
	"CI_MERGE_REQUEST_LABELS",
	"CI_MERGE_REQUEST_MILESTONE",
	"CI_MERGE_REQUEST_PROJECT_ID",
	"CI_MERGE_REQUEST_PROJECT_PATH",
	"CI_MERGE_REQUEST_PROJECT_URL",
	"CI_MERGE_REQUEST_REF_PATH",
	"CI_MERGE_REQUEST_SOURCE_BRANCH_NAME",
	"CI_MERGE_REQUEST_SOURCE_BRANCH_SHA",
	"CI_MERGE_REQUEST_SOURCE_PROJECT_ID",
	"CI_MERGE_REQUEST_SOURCE_PROJECT_PATH",
	"CI_MERGE_REQUEST_SOURCE_PROJECT_URL",
	"CI_MERGE_REQUEST_TARGET_BRANCH_NAME",
	"CI_MERGE_REQUEST_TARGET_BRANCH_SHA",
	"CI_MERGE_REQUEST_TITLE",
	"CI_MERGE_REQUEST_EVENT_TYPE",
	"CI_NODE_INDEX",
	"CI_NODE_TOTAL",
	"CI_PAGES_DOMAIN",
	"CI_PAGES_URL",
	"CI_PIPELINE_ID",
	"CI_PIPELINE_IID",
	"CI_PIPELINE_SOURCE",
	"CI_PIPELINE_TRIGGERED",
	"CI_PIPELINE_URL",
	// "CI_PROJECT_DIR",
	"CI_PROJECT_ID",
	"CI_PROJECT_NAME",
	"CI_PROJECT_NAMESPACE",
	"CI_PROJECT_ROOT_NAMESPACE",
	"CI_PROJECT_PATH",
	"CI_PROJECT_PATH_SLUG",
	"CI_PROJECT_REPOSITORY_LANGUAGES",
	"CI_PROJECT_TITLE",
	"CI_PROJECT_URL",
	"CI_PROJECT_VISIBILITY",
	"CI_REGISTRY",
	"CI_REGISTRY_IMAGE",
	"CI_REGISTRY_PASSWORD",
	"CI_REGISTRY_USER",
	"CI_REPOSITORY_URL",
	"CI_RUNNER_DESCRIPTION",
	// "CI_RUNNER_EXECUTABLE_ARCH",
	"CI_RUNNER_ID",
	// "CI_RUNNER_REVISION",
	"CI_RUNNER_SHORT_TOKEN",
	"CI_RUNNER_TAGS",
	// "CI_RUNNER_VERSION",
	// "CI_SERVER",
	"CI_SERVER_URL",
	"CI_SERVER_HOST",
	"CI_SERVER_PORT",
	"CI_SERVER_PROTOCOL",
	"CI_SERVER_NAME",
	"CI_SERVER_REVISION",
	"CI_SERVER_VERSION",
	"CI_SERVER_VERSION_MAJOR",
	"CI_SERVER_VERSION_MINOR",
	"CI_SERVER_VERSION_PATCH",
	"CI_SHARED_ENVIRONMENT",
	"GITLAB_CI",
	"GITLAB_FEATURES",
	"GITLAB_USER_EMAIL",
	"GITLAB_USER_ID",
	"GITLAB_USER_LOGIN",
	"GITLAB_USER_NAME",
}

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

func TestFileVariablesExpansion(t *testing.T) {
	all := JobVariables{
		{Key: "a_file_var", Value: "some top secret stuff", File: true},
		{Key: "ref_file_var", Value: "${a_file_var}.txt"},
		{Key: "regular_var", Value: "bla bla bla"},
		{Key: "ref_regular_var", Value: "bla bla bla"},
		{Key: "RUNNER_TEMP_PROJECT_DIR", Value: "/foo/bar", Public: true, Internal: true},
	}

	validate := func(t *testing.T, variables JobVariables) {
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

func TestOverwriteKey(t *testing.T) {
	vars := JobVariables{
		{Key: "hello", Value: "world"},
		{Key: "foo", Value: ""},
	}

	// Overwrite empty value
	vars.OverwriteKey("foo", JobVariable{Key: "foo", Value: "bar"})

	assert.Equal(t, "world", vars.Get("hello"))
	assert.Equal(t, "bar", vars.Get("foo"))

	// Overwrite existing value
	vars.OverwriteKey("hello", JobVariable{Key: "hello", Value: "universe"})

	assert.Equal(t, "universe", vars.Get("hello"))
	assert.Equal(t, "bar", vars.Get("foo"))

	// Overwrite key
	vars.OverwriteKey("hello", JobVariable{Key: "goodbye", Value: "universe"})

	assert.Equal(t, "universe", vars.Get("goodbye"))
	assert.Equal(t, "", vars.Get("hello"))
	assert.Equal(t, "bar", vars.Get("foo"))

	// Overwrite properties
	fooOverwriteVar := JobVariable{
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

func TestPredefinedServerVariables(t *testing.T) {
	build := &Build{}
	for _, v := range build.GetAllVariables() {
		for _, predefined := range predefinedServerJobVariables {
			assert.NotEqual(
				t,
				predefined,
				v.Key,
				"%s is a predefined server variable and should not be set by runner",
				predefined,
			)
		}
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
			v := JobVariables{
				{Key: "variable", Value: value},
			}

			result := v.Bool("variable")
			require.Equal(t, expected, result)
		})
	}
}
