//go:build !integration

package steps

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"

	"gitlab.com/gitlab-org/step-runner/schema/v1"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

func Test_AddStepsPreamble(t *testing.T) {
	tests := map[string]struct {
		in   string
		want string
	}{
		"simple script": {
			in:   `[{"name":"script", "script": "foo bar baz"}]`,
			want: "{}\n---\nrun:\n    - name: script\n      script: foo bar baz\n",
		},
		"reference local step": {
			in:   `[{"name":"local", "func":"./some/step", "inputs":{"in":"bar"}}]`,
			want: "{}\n---\nrun:\n    - inputs:\n        in: bar\n      name: local\n      func: ./some/step\n",
		},
		"reference remote step": {
			in:   `[{"name":"remote", "func":"https://gitlab.com/components/script@v1", "inputs":{"in":"bar"}}]`,
			want: "{}\n---\nrun:\n    - inputs:\n        in: bar\n      name: remote\n      func: https://gitlab.com/components/script@v1\n",
		},
		"action step": {
			in:   `[{"name":"action", "action":"some-action@v1", "inputs":{"in":"bar"}}]`,
			want: "{}\n---\nrun:\n    - action: some-action@v1\n      inputs:\n        in: bar\n      name: action\n",
		},
		"exec step": {
			in:   `[{"name":"exec", "exec":{"command":["cmd","arg1", "arg2"],"work_dir":"/foo/bar/baz"}, "inputs":{"in":"bar"}}]`,
			want: "{}\n---\nrun:\n    - exec:\n        command:\n            - cmd\n            - arg1\n            - arg2\n        work_dir: /foo/bar/baz\n      inputs:\n        in: bar\n      name: exec\n",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var steps []schema.Step
			assert.NoError(t, json.Unmarshal([]byte(tt.in), &steps))
			got, err := AddStepsPreamble(steps)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_NewRequest_ScriptLinesSurviveStepRunnerDecoding(t *testing.T) {
	tests := map[string][]string{
		"leading newline block":     {"\necho hi\necho there\n"},
		"several leading newlines":  {"\n\n\necho hi"},
		"leading newline only":      {"\n"},
		"leading spaces":            {"  echo hi\n  echo there\n"},
		"leading newline and space": {"\n  echo hi\n"},
		"leading tab":               {"\techo hi\n\techo there\n"},
		"leading tab after newline": {"\n\techo hi\n"},
		"trailing spaces":           {"echo hi  \necho there \n"},
		"blank lines inside":        {"echo hi\n\n\necho there\n"},
		"carriage returns":          {"\r\necho hi\r\n"},
		"comment and colon":         {"echo hi # not a comment\n# real comment\nkey: value\n"},
		"yaml indicators":           {"- item", "? key", "| pipe", "> fold", "%dir", "@at", "`tick`", "*star", "&anchor", "!tag"},
		"yaml literals":             {"null", "true", "123", "~", "---", "..."},
		"quotes and backslashes":    {"echo \"it's\" \\n\n"},
		"unicode":                   {"echo \u00e9\u65e5\u672c\U0001f600\n", "\u00a0nbsp\nline", "line\u2028separator"},
		"control characters":        {"a\x01b\x7fc\x1bd\n"},
		"empty line":                {""},
		"mixed":                     {"plain", "\n  indented\n", "\techo", "", "tail  \n"},
	}

	for name, lines := range tests {
		t.Run(name, func(t *testing.T) {
			env := map[string]string{"MULTILINE": strings.Join(lines, "\n")}
			steps := []schema.Step{{
				Name:   func(s string) *string { return &s }("user_script"),
				Step:   "builtin://script_legacy",
				Env:    env,
				Inputs: schema.StepInputs{"script": lines, "posix_escape": true},
			}}

			req, err := NewRequest(JobInfo{ID: 1}, steps)
			require.NoError(t, err)

			got := decodeLikeStepRunner(t, req.Steps)
			require.Len(t, got.Run, 1)

			assert.Equal(t, lines, requireStringList(t, got.Run[0].Inputs["script"]))
			assert.Equal(t, env, got.Run[0].Env)
			assert.Equal(t, true, got.Run[0].Inputs["posix_escape"])
		})
	}
}

func Test_NewRequest_InputTypesSurviveStepRunnerDecoding(t *testing.T) {
	in := `[{"name":"typed","func":"./some/step","inputs":{
		"count":3,"ratio":1.5,"big":1000000,"negative":-2,
		"on":true,"off":false,"nothing":null,"text":"3",
		"nested":{"n":7,"flag":true,"list":[1,"two",null,false]}}}]`

	var steps []schema.Step
	require.NoError(t, json.Unmarshal([]byte(in), &steps))

	req, err := NewRequest(JobInfo{ID: 1}, steps)
	require.NoError(t, err)

	got := decodeLikeStepRunner(t, req.Steps)
	require.Len(t, got.Run, 1)

	want := schema.StepInputs{
		"count":    3,
		"ratio":    1.5,
		"big":      float64(1000000),
		"negative": -2,
		"on":       true,
		"off":      false,
		"nothing":  nil,
		"text":     "3",
		"nested": schema.StepInputs{
			"n":    7,
			"flag": true,
			"list": []any{1, "two", nil, false},
		},
	}
	assert.Equal(t, want, got.Run[0].Inputs)
}

func Test_NewRequest_RunKeywordStepsSurviveStepRunnerDecoding(t *testing.T) {
	jobRun := `[
		{"name":"greet","func":"./some/step","inputs":{"count":3,"verbose":true,"who":"world"}},
		{"name":"cmd","exec":{"command":["echo","hi"],"work_dir":"/tmp"},"env":{"A":"1"}}
	]`
	payload, err := json.Marshal(jobRun)
	require.NoError(t, err)

	var run spec.Run
	require.NoError(t, json.Unmarshal(payload, &run))

	req, err := NewRequest(JobInfo{ID: 1}, run)
	require.NoError(t, err)

	got := decodeLikeStepRunner(t, req.Steps)
	require.Len(t, got.Run, 2)

	assert.Equal(t, "./some/step", got.Run[0].Func)
	assert.Equal(t, schema.StepInputs{"count": 3, "verbose": true, "who": "world"}, got.Run[0].Inputs)
	require.NotNil(t, got.Run[1].Exec)
	assert.Equal(t, []string{"echo", "hi"}, got.Run[1].Exec.Command)
	require.NotNil(t, got.Run[1].Exec.WorkDir)
	assert.Equal(t, "/tmp", *got.Run[1].Exec.WorkDir)
	assert.Equal(t, map[string]string{"A": "1"}, got.Run[1].Env)
}

func requireStringList(t *testing.T, v any) []string {
	t.Helper()

	list, ok := v.([]any)
	require.True(t, ok, "want a list, got %T", v)

	out := make([]string, 0, len(list))
	for _, item := range list {
		s, ok := item.(string)
		require.True(t, ok, "want a string, got %T", item)
		out = append(out, s)
	}
	return out
}

func decodeLikeStepRunner(t *testing.T, content string) schema.Step {
	t.Helper()

	var spec schema.Spec
	var step schema.Step

	decoder := yaml.NewDecoder(strings.NewReader(content))
	decoder.KnownFields(true)
	require.NoError(t, decoder.Decode(&spec))
	require.NoError(t, decoder.Decode(&step))

	return step
}

func Test_NewRequest_Adds_Timeout(t *testing.T) {
	step := `[{"name":"script", "script": "foo bar baz"}]`
	var steps []schema.Step
	assert.NoError(t, json.Unmarshal([]byte(step), &steps))
	to := time.Second * 42

	ji := JobInfo{Timeout: to}
	got, err := NewRequest(ji, steps)
	assert.NoError(t, err)
	assert.Equal(t, to, *got.Timeout)
}

func Test_addVariables_Omits(t *testing.T) {
	keysToVars := func(keys []string) spec.Variables {
		jobVars := spec.Variables{}

		for _, k := range keys {
			jobVars = append(jobVars, spec.Variable{
				Key:   k,
				Value: k,
			})
		}
		return jobVars
	}
	want := []string{"FOO", "BAR", "BAZ"}
	doNotWant := []string{"STEPS", "FF_USE_NATIVE_STEPS"}
	all := append(keysToVars(want), keysToVars(doNotWant)...)

	got := addVariables(all)

	for _, g := range got {
		assert.Contains(t, want, g.Key)
		assert.NotContains(t, doNotWant, g.Key)
	}
}
