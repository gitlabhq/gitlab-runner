// go:build !integration
package steps

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func Test_addStepsPreamble(t *testing.T) {
	tests := map[string]struct {
		in   string
		want string
	}{
		"simple script": {
			in:   `[{"name":"script", "script": "foo bar baz"}]`,
			want: "{}\n---\nsteps:\n- name: script\n  script: foo bar baz\n",
		},
		"reference local step": {
			in:   `[{"name":"local", "step":"./some/step", "inputs":{"in":"bar"}}]`,
			want: "{}\n---\nsteps:\n- inputs:\n    in: bar\n  name: local\n  step: ./some/step\n",
		},
		"reference remote step": {
			in:   `[{"name":"remote", "step":"https://gitlab.com/components/script@v1", "inputs":{"in":"bar"}}]`,
			want: "{}\n---\nsteps:\n- inputs:\n    in: bar\n  name: remote\n  step: https://gitlab.com/components/script@v1\n",
		},
		"action step": {
			in:   `[{"name":"action", "action":"some-action@v1", "inputs":{"in":"bar"}}]`,
			want: "{}\n---\nsteps:\n- action: some-action@v1\n  inputs:\n    in: bar\n  name: action\n",
		},
		"exec step": {
			in:   `[{"name":"exec", "exec":{"command":["cmd","arg1", "arg2"],"work_dir":"/foo/bar/baz"}, "inputs":{"in":"bar"}}]`,
			want: "{}\n---\nsteps:\n- exec:\n    command:\n    - cmd\n    - arg1\n    - arg2\n    work_dir: /foo/bar/baz\n  inputs:\n    in: bar\n  name: exec\n",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := addStepsPreamble(tt.in)
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_addVariables_Omits(t *testing.T) {
	keysToVars := func(keys []string) common.JobVariables {
		jobVars := common.JobVariables{}

		for _, k := range keys {
			jobVars = append(jobVars, common.JobVariable{
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
