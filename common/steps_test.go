//go:build !integration

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

// The predicate is platform-independent, so this test runs on all platforms.
func TestBuild_nativeStepsBlockedWithoutConcrete(t *testing.T) {
	newBuild := func(concreteOnly, run bool, flags map[string]bool) *Build {
		job := spec.Job{}
		if run {
			job.Run = spec.Run{{}}
		}
		return &Build{
			Runner: &RunnerConfig{
				RunnerSettings: RunnerSettings{FeatureFlags: flags},
			},
			Job: job,
			ExecutorFeatures: FeaturesInfo{
				NativeStepsIntegration:     true,
				NativeStepsViaConcreteOnly: concreteOnly,
			},
		}
	}

	tests := map[string]struct {
		build   *Build
		blocked bool
	}{
		"concrete-only (shell/k8s), run keyword, no FF_CONCRETE -> blocked": {
			build:   newBuild(true, true, nil),
			blocked: true,
		},
		"concrete-only, run keyword, FF_CONCRETE on -> allowed (handled by concrete)": {
			build:   newBuild(true, true, map[string]bool{featureflags.UseConcrete: true}),
			blocked: false,
		},
		"hybrid (docker), run keyword, no FF_CONCRETE -> allowed": {
			build:   newBuild(false, true, nil),
			blocked: false,
		},
		"concrete-only, script-to-step migration, no FF_CONCRETE -> blocked": {
			build:   newBuild(true, false, map[string]bool{featureflags.UseScriptToStepMigration: true}),
			blocked: true,
		},
		"concrete-only, no native steps requested -> allowed": {
			build:   newBuild(true, false, nil),
			blocked: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.blocked, tc.build.nativeStepsBlockedWithoutConcrete())
		})
	}
}
