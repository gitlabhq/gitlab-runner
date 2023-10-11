//go:build integration && !windows

package autoscaler_test

import "gitlab.com/gitlab-org/gitlab-runner/common"

func getImage() string {
	return common.TestAlpineImage
}
