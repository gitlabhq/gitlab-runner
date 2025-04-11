package pull_policies

import (
	"fmt"
	"slices"
)

// ComputeEffectivePullPolicies returns the intersection of the specified pullPolices and allowedPullPolicies, or
// an error if pullPolicies is not empty but the intersection IS empty. In other words, it limits pullPolicies to what
// is in allowedPullPolicies, and returns an error if none of pullPolicies was in allowedPullPolicies. All of the
// arguments are ultimately ~[]string, but defined in different ways.
func ComputeEffectivePullPolicies[A ~string, B ~string, C ~[]string](pullPolicies, allowedPullPolicies []A, ciPullPolicies []B, executorPullPoilcies C) ([]A, error) {
	if len(pullPolicies) == 0 {
		return allowedPullPolicies, nil
	}
	if len(allowedPullPolicies) == 0 {
		return pullPolicies, nil
	}
	var actuallyAllowed []A
	for _, policy := range pullPolicies {
		if slices.Contains(allowedPullPolicies, policy) {
			actuallyAllowed = append(actuallyAllowed, policy)
		}
	}

	if len(actuallyAllowed) == 0 {
		return nil, &incompatiblePullPolicyError[A]{
			pullPolicies:        pullPolicies,
			allowedPullPolicies: allowedPullPolicies,
			pullPolicySource:    getPullPolicySource(ciPullPolicies, executorPullPoilcies),
		}
	}
	return actuallyAllowed, nil
}

// getPullPolicySource returns the source (i.e. file) of the pull_policy
// configuration used by this runner. This is used to produce a more detailed
// error message. See https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29115
func getPullPolicySource[A ~string, B ~[]string](ciPullPolicies []A, executorPullPolicies B) string {
	switch {
	case len(ciPullPolicies) != 0:
		return pullPolicySourceGitLabCI
	case len(executorPullPolicies) != 0:
		return pullPolicySourceRunner
	default:
		return pullPolicySourceDefault
	}
}

const (
	incompatiblePullPolicy   = "pull_policy (%v) defined in %s is not one of the allowed_pull_policies (%v)"
	pullPolicySourceGitLabCI = "GitLab pipeline config"
	pullPolicySourceRunner   = "Runner config"
	pullPolicySourceDefault  = "Runner config (default)"
)

type incompatiblePullPolicyError[T ~string] struct {
	pullPolicySource    string
	pullPolicies        []T
	allowedPullPolicies []T
}

func (e *incompatiblePullPolicyError[T]) Error() string {
	return fmt.Sprintf(incompatiblePullPolicy, e.pullPolicies, e.pullPolicySource, e.allowedPullPolicies)
}
