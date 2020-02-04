package kubernetes

import (
	"fmt"

	"github.com/hashicorp/go-version"
	"k8s.io/client-go/kubernetes"
)

type featureChecker interface {
	IsHostAliasSupported() (bool, error)
}

type kubeClientFeatureChecker struct {
	kubeClient *kubernetes.Clientset
}

// https://kubernetes.io/docs/concepts/services-networking/add-entries-to-pod-etc-hosts-with-host-aliases/
var minimumHostAliasesVersionRequired, _ = version.NewVersion("1.7")

func (c *kubeClientFeatureChecker) IsHostAliasSupported() (bool, error) {
	verInfo, err := c.kubeClient.ServerVersion()
	if err != nil {
		return false, err
	}

	ver, err := version.NewVersion(fmt.Sprintf("%s.%s", verInfo.Major, verInfo.Minor))
	if err != nil {
		return false, err
	}

	supportsHostAliases := ver.GreaterThan(minimumHostAliasesVersionRequired) ||
		ver.Equal(minimumHostAliasesVersionRequired)

	return supportsHostAliases, nil
}
