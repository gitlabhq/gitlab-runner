package kubernetes

import "gitlab.com/gitlab-org/gitlab-runner/magefiles/kubernetes/docs"

func GeneratePermissionsDocs() error {
	permissions, err := docs.ParsePermissions()
	if err != nil {
		return err
	}

	return docs.GeneratePermissionsDocs("gitlab-runner", "default", permissions)
}
