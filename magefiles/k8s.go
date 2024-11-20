//go:build mage

package main

import (
	"github.com/magefile/mage/mg"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/kubernetes"
)

type K8s mg.Namespace

func (K8s) GeneratePermissionsDocs() error {
	return kubernetes.GeneratePermissionsDocs()
}

func (K8s) ProvisionIntegrationKubernetes(id string) error {
	return kubernetes.ProvisionIntegrationKubernetes(id)
}

func (K8s) DestroyIntegrationKubernetes(id string) error {
	return kubernetes.DestroyIntegrationKubernetes(id)
}
