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
