//go:build !integration

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetStepRunnerImage(t *testing.T) {
	stepRunnerVersion := getModuleDependencyVersion(stepRunnerModule, defaultStepRunnerVersion)
	defaultImage := defaultStepRunnerImage + ":" + stepRunnerVersion

	tests := map[string]struct {
		image    string
		expected string
	}{
		"default values when no variables set": {
			expected: defaultImage,
		},
		"custom image": {
			image:    "custom/runner:1.2.3",
			expected: "custom/runner:1.2.3",
		},
		"empty custom image": {
			image:    "           ",
			expected: defaultImage,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			r := RunnerSettings{
				StepRunnerImage: tt.image,
			}

			assert.Equal(t, tt.expected, r.GetStepRunnerImage())
		})
	}
}
