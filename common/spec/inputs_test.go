//go:build !integration

package spec

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/moa/value"
)

var (
	_ value.Mapper = (*Inputs)(nil)
)

// TODO: to be replaced, but used here for quick testing
// From: https://gitlab.com/gitlab-org/gitlab/-/issues/543972
// NOTE: all non-string and sensitive inputs have been removed.
const complexExampleInputs = `
[
    {
      "key": "username",
      "value": {
        "type": "string",
        "content": "fred",
        "sensitive": false
      }
    },
    {
      "key": "fullname",
      "value": {
        "type": "string",
        "content": "fred tester",
        "sensitive": false
      }
    },
    {
      "key": "password",
      "value": {
        "type": "string",
        "content": "123456",
        "sensitive": true
      }
    },
	{
      "key": "age",
      "value": {
        "type": "number",
        "content": 1,
        "sensitive": false
      }
    },
    {
      "key": "likes_spaghetti",
      "value": {
        "type": "boolean",
        "content": false,
        "sensitive": false
      }
    },
    {
      "key": "friends",
      "value": {
        "type": "array",
        "content": [
          "bob",
          "sally"
        ],
        "sensitive": false
      }
    },
    {
      "key": "address",
      "value": {
        "type": "struct",
        "content": {
          "line1": "42 Wallaby Way",
          "line2": "Sydney"
        },
        "sensitive": false
      }
    }
]
`

func TestJobInputs_Unmarshalling(t *testing.T) {
	t.Parallel()

	inputData := []byte(complexExampleInputs)
	inputs := Inputs{}

	err := json.Unmarshal(inputData, &inputs)

	require.NoError(t, err)
	assert.Equal(t, 7, inputs.Len())
	keys := make([]string, 0, inputs.Len())
	for i := range inputs.Keys() {
		keys = append(keys, i.String())
	}
	assert.ElementsMatch(t, []string{"username", "fullname", "password", "age", "likes_spaghetti", "friends", "address"}, keys)
}

func TestJobInputs_Unmarshalling_Sensitive(t *testing.T) {
	t.Parallel()

	inputData := []byte(`
		[
			{
				"key": "anykey-implicit-no-sensitive",
				"value": {
					"type": "string",
					"content": "any"
				}
			},
			{
				"key": "anykey-explicit-no-sensitive",
				"value": {
					"type": "string",
					"content": "any",
					"sensitive": false
				}
			},
			{
				"key": "anykey-explicit-sensitive",
				"value": {
					"type": "string",
					"content": "any",
					"sensitive": true
				}
			}
		]
	`)

	inputs := Inputs{}

	err := json.Unmarshal(inputData, &inputs)
	require.NoError(t, err)

	assert.Equal(t, 3, inputs.Len())
	assert.False(t, inputs.inputs[0].Sensitive())
	assert.False(t, inputs.inputs[1].Sensitive())
	assert.True(t, inputs.inputs[2].Sensitive())
}

func TestJobInputs_Unmarshalling_Error(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		inputData     []byte
		expectedError string
	}{
		{
			name: "empty input",
			inputData: []byte(`
				[
					{}
				]
			`),
			expectedError: `input without key`,
		},
		{
			name: "input without value",
			inputData: []byte(`
				[
					{
						"key": "anykey"
					}
				]
			`),
			expectedError: `input "anykey" is null, must have valid value`,
		},
		{
			name: "input with empty value",
			inputData: []byte(`
				[
					{
						"key": "anykey",
						"value": {}
					}
				]
			`),
			expectedError: `input "anykey" is null, must have valid value`,
		},
		{
			name: "input without type",
			inputData: []byte(`
				[
					{
						"key": "anykey",
						"value": {
							"content": "any"
						}
					}
				]
			`),
			expectedError: `invalid type "" for input "anykey": type is unknown`,
		},
		{
			name: "input without content",
			inputData: []byte(`
				[
					{
						"key": "anykey",
						"value": {
							"type": "string"
						}
					}
				]
			`),
			expectedError: `input "anykey" is null, must have valid value`,
		},
		{
			name: "input with invalid type",
			inputData: []byte(`
				[
					{
						"key": "anykey",
						"value": {
							"type": "unexisting-type",
							"content": "any"
						}
					}
				]
			`),
			expectedError: `invalid type "unexisting-type" for input "anykey": type is unknown`,
		},
		{
			name: "input with mismatching type",
			inputData: []byte(`
				[
					{
						"key": "anykey",
						"value": {
							"type": "number",
							"content": "any"
						}
					}
				]
			`),
			expectedError: `mismatching type of input "anykey". Announced "number", but got "string"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inputs := Inputs{}
			err := json.Unmarshal(tt.inputData, &inputs)

			assert.EqualError(t, err, tt.expectedError)
		})
	}
}

func TestJobInputs_Expand_string(t *testing.T) {
	t.Parallel()

	inputData := []byte(complexExampleInputs)
	inputs := Inputs{}
	err := json.Unmarshal(inputData, &inputs)
	require.NoError(t, err)

	expanded, err := inputs.Expand("Hello ${{ job.inputs.username }}, your fullname is ${{ job.inputs.fullname }}")
	require.NoError(t, err)

	assert.Equal(t, "Hello fred, your fullname is fred tester", expanded)
}

func TestJobInputs_Expand_sensitive_string_reject(t *testing.T) {
	t.Parallel()

	inputData := []byte(complexExampleInputs)
	inputs := Inputs{}
	err := json.Unmarshal(inputData, &inputs)
	require.NoError(t, err)

	_, err = inputs.Expand("Hello ${{ job.inputs.username }}, your password is ${{ job.inputs.password }}")

	require.ErrorIs(t, err, ErrSensitiveUnsupported)
}

func TestJobInputs_Expand_nonstring(t *testing.T) {
	t.Parallel()

	inputData := []byte(complexExampleInputs)
	inputs := Inputs{}
	err := json.Unmarshal(inputData, &inputs)
	require.NoError(t, err)

	expanded, err := inputs.Expand("Hello ${{ job.inputs.username }}, your age is ${{ str(job.inputs.age) }}")
	require.NoError(t, err)

	assert.Equal(t, "Hello fred, your age is 1", expanded)
}

func TestJobInputs_Expand_ArrayElement(t *testing.T) {
	t.Parallel()

	inputData := []byte(`
		[
			{
				"key": "field",
				"value": {
					"type": "array",
					"content": ["one", "two", "three"]
				}
			}
		]
	`)

	inputs := Inputs{}
	err := json.Unmarshal(inputData, &inputs)
	require.NoError(t, err)

	expanded, err := inputs.Expand("Field is ${{ job.inputs.field[1] }}")

	require.NoError(t, err)
	assert.Equal(t, "Field is two", expanded)
}

func TestJobInputs_Expand_StructField(t *testing.T) {
	t.Parallel()

	inputData := []byte(`
		[
			{
				"key": "field",
				"value": {
					"type": "struct",
					"content": {
						"line1": "Streetname 1",
						"line2": "1234 ..."
					},
					"sensitive": false
				}
			}
		]
	`)

	inputs := Inputs{}
	err := json.Unmarshal(inputData, &inputs)
	require.NoError(t, err)

	expanded, err := inputs.Expand("Field is ${{ job.inputs.field.line1 }}")

	require.NoError(t, err)
	assert.Equal(t, "Field is Streetname 1", expanded)
}

type customInputExpander string

func (c *customInputExpander) Expand(inputs *Inputs) error {
	*c = "REDACTED"
	return nil
}

func TestInputsTag(t *testing.T) {
	type MyString string

	type JobResponse struct {
		StringToExpand          string `inputs:"expand"`
		StringNotToExpand       string
		CustomStringToExpand    MyString `inputs:"expand"`
		CustomStringNotToExpand MyString
		StructToExpand          struct {
			StringToExpand    string `inputs:"expand"`
			StringNotToExpand string
		} `inputs:"expand"`
		StructNotToExpand struct {
			StringToExpand    string `inputs:"expand"`
			StringNotToExpand string
		}
		SliceToExpand                  []string `inputs:"expand"`
		SliceNotToExpand               []string
		CustomInputExpanderToExpand    customInputExpander `inputs:"expand"`
		CustomInputExpanderNotToExpand customInputExpander
	}

	jobResponse := JobResponse{
		StringToExpand:          "${{ job.inputs.any }}",
		StringNotToExpand:       "${{ job.inputs.any }}",
		CustomStringToExpand:    "${{ job.inputs.any }}",
		CustomStringNotToExpand: "${{ job.inputs.any }}",
		StructToExpand: struct {
			StringToExpand    string "inputs:\"expand\""
			StringNotToExpand string
		}{
			StringToExpand:    "${{ job.inputs.any }}",
			StringNotToExpand: "${{ job.inputs.any }}",
		},
		StructNotToExpand: struct {
			StringToExpand    string "inputs:\"expand\""
			StringNotToExpand string
		}{
			StringToExpand:    "${{ job.inputs.any }}",
			StringNotToExpand: "${{ job.inputs.any }}",
		},
		SliceToExpand:                  []string{"${{ job.inputs.any }}", "${{ job.inputs.any }}"},
		SliceNotToExpand:               []string{"${{ job.inputs.any }}", "${{ job.inputs.any }}"},
		CustomInputExpanderToExpand:    "${{ job.inputs.any }}",
		CustomInputExpanderNotToExpand: "${{ job.inputs.any }}",
	}

	inputs, err := NewJobInputs([]JobInput{
		{
			Key: "any",
			Value: JobInputValue{
				Type:      JobInputContentTypeNameString,
				Content:   value.String("value"),
				Sensitive: false,
			},
		},
	})
	require.NoError(t, err)

	err = ExpandInputs(&inputs, &jobResponse)
	require.NoError(t, err)

	assert.Equal(t, "value", jobResponse.StringToExpand)
	assert.Equal(t, "${{ job.inputs.any }}", jobResponse.StringNotToExpand)
	assert.Equal(t, MyString("value"), jobResponse.CustomStringToExpand)
	assert.Equal(t, MyString("${{ job.inputs.any }}"), jobResponse.CustomStringNotToExpand)
	assert.Equal(t, "value", jobResponse.StructToExpand.StringToExpand)
	assert.Equal(t, "${{ job.inputs.any }}", jobResponse.StructToExpand.StringNotToExpand)
	assert.Equal(t, "${{ job.inputs.any }}", jobResponse.StructNotToExpand.StringToExpand)
	assert.Equal(t, "${{ job.inputs.any }}", jobResponse.StructNotToExpand.StringNotToExpand)
	assert.Equal(t, []string{"value", "value"}, jobResponse.SliceToExpand)
	assert.Equal(t, []string{"${{ job.inputs.any }}", "${{ job.inputs.any }}"}, jobResponse.SliceNotToExpand)
	assert.Equal(t, customInputExpander("REDACTED"), jobResponse.CustomInputExpanderToExpand)
	assert.Equal(t, customInputExpander("${{ job.inputs.any }}"), jobResponse.CustomInputExpanderNotToExpand)
}

func TestJobInputs_Expand_NoInputsDefined(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "no job input access, invalid moa syntax",
			text:     "Hello $",
			expected: "Hello $",
		},
		{
			name:     "no job input access",
			text:     "Hello ${{ 1 + 2 }}",
			expected: "Hello ${{ 1 + 2 }}",
		},
		{
			name:     "with job input access",
			text:     "Hello ${{ job.inputs.username }}",
			expected: "Hello ${{ job.inputs.username }}",
		},
		{
			name:     "plain text",
			text:     "Hello world",
			expected: "Hello world",
		},
		{
			name:     "other selector",
			text:     "${{ foo.bar.baz }}",
			expected: "${{ foo.bar.baz }}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			inputs := Inputs{}
			inputs.SetMetricsCollector(NewJobInputsMetricsCollector())

			expanded, err := inputs.Expand(tt.text)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, expanded)
		})
	}
}
