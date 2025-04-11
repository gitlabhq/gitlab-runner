//go:build !integration

package pull_policies

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	v1 "k8s.io/api/core/v1"
)

type (
	dpps = []common.DockerPullPolicy
	kpps = []v1.PullPolicy
	soa  = common.StringOrArray
)

func Test_ComputeEffectivePullPolicies(t *testing.T) {
	tests := map[string]struct {
		pullPolicies        kpps
		allowedPullPolicies kpps
		wantPullPolicies    kpps
		wantErr             bool
	}{
		"identical policies, ok": {
			pullPolicies:        kpps{v1.PullAlways, v1.PullNever, v1.PullIfNotPresent},
			allowedPullPolicies: kpps{v1.PullAlways, v1.PullNever, v1.PullIfNotPresent},
			wantPullPolicies:    kpps{v1.PullAlways, v1.PullNever, v1.PullIfNotPresent},
		},
		"no pull policies, ok": {
			allowedPullPolicies: kpps{v1.PullAlways, v1.PullNever, v1.PullIfNotPresent},
			wantPullPolicies:    kpps{v1.PullAlways, v1.PullNever, v1.PullIfNotPresent},
		},
		"no allowed pull policies, ok": {
			pullPolicies:     kpps{v1.PullAlways, v1.PullNever, v1.PullIfNotPresent},
			wantPullPolicies: kpps{v1.PullAlways, v1.PullNever, v1.PullIfNotPresent},
		},
		"repeated pull policies, ok": {
			pullPolicies:     kpps{v1.PullAlways, v1.PullAlways, v1.PullIfNotPresent, v1.PullIfNotPresent},
			wantPullPolicies: kpps{v1.PullAlways, v1.PullAlways, v1.PullIfNotPresent, v1.PullIfNotPresent},
		},
		"both empty, ok": {},
		"empty intersection, fail": {
			pullPolicies:        kpps{v1.PullAlways},
			allowedPullPolicies: kpps{v1.PullIfNotPresent},
			wantErr:             true,
		},
		"non-empty intersection, ok": {
			pullPolicies:        kpps{v1.PullAlways, v1.PullNever},
			allowedPullPolicies: kpps{v1.PullNever, v1.PullIfNotPresent},
			wantPullPolicies:    kpps{v1.PullNever},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			gotPP, err := ComputeEffectivePullPolicies(
				tt.pullPolicies,
				tt.allowedPullPolicies,
				dpps{}, soa{},
			)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, gotPP)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantPullPolicies, gotPP)
			}
		})
	}
}

func Test_GetPullPolicySource(t *testing.T) {
	tests := map[string]struct {
		ciPullPolicies       dpps
		executorPullPolicies common.StringOrArray
		wantSource           string
	}{
		"both, ci config prevails": {
			ciPullPolicies:       dpps{common.PullPolicyAlways, common.PullPolicyNever, common.PullPolicyIfNotPresent},
			executorPullPolicies: soa{common.PullPolicyAlways, common.PullPolicyNever, common.PullPolicyIfNotPresent},
			wantSource:           pullPolicySourceGitLabCI,
		},
		"ci config only": {
			ciPullPolicies:       dpps{common.PullPolicyAlways, common.PullPolicyNever, common.PullPolicyIfNotPresent},
			executorPullPolicies: common.StringOrArray{common.PullPolicyAlways, common.PullPolicyNever, common.PullPolicyIfNotPresent},
			wantSource:           pullPolicySourceGitLabCI,
		},
		"executor config only": {
			executorPullPolicies: common.StringOrArray{common.PullPolicyAlways, common.PullPolicyNever, common.PullPolicyIfNotPresent},
			wantSource:           pullPolicySourceRunner,
		},
		"neither/default": {
			wantSource: pullPolicySourceDefault,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.wantSource, getPullPolicySource(
				tt.ciPullPolicies,
				tt.executorPullPolicies,
			))
		})
	}
}
