//go:build !integration

package steps

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/step-runner/schema/v1"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

func Test_addStepsPreamble(t *testing.T) {
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
			got, err := addStepsPreamble(steps)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
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
