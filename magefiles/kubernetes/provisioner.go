package kubernetes

import "gitlab.com/gitlab-org/gitlab-runner/magefiles/kubernetes/provision"

func ProvisionIntegrationKubernetes(id string) error {
	return provision.ProvisionIntegrationKubernetes(id)
}

func DestroyIntegrationKubernetes(id string) error {
	return provision.DestroyIntegrationKubernetes(id)
}
