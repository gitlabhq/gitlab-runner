package kubernetes

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/hashicorp/go-version"
	"k8s.io/client-go/kubernetes"
)

//go:generate mockery --name=featureChecker --inpackage
type featureChecker interface {
	IsHostAliasSupported() (bool, error)
}

type kubeClientFeatureChecker struct {
	kubeClient *kubernetes.Clientset
}

// https://kubernetes.io/docs/concepts/services-networking/add-entries-to-pod-etc-hosts-with-host-aliases/
var minimumHostAliasesVersionRequired, _ = version.NewVersion("1.7")

type badVersionError struct {
	major string
	minor string
	inner error
}

func (s *badVersionError) Error() string {
	return fmt.Sprintf("parsing Kubernetes version %s.%s - %s", s.major, s.minor, s.inner)
}

func (s *badVersionError) Is(err error) bool {
	_, ok := err.(*badVersionError)
	return ok
}

func (c *kubeClientFeatureChecker) IsHostAliasSupported() (bool, error) {
	verInfo, err := c.kubeClient.ServerVersion()
	if err != nil {
		return false, err
	}

	major := cleanVersion(verInfo.Major)
	minor := cleanVersion(verInfo.Minor)
	ver, err := version.NewVersion(fmt.Sprintf("%s.%s", major, minor))
	if err != nil {
		// Use the original major and minor parts of the version so we can better see in the logs
		// what came straight from kubernetes. The inner error from version.NewVersion will tell us
		// what version we actually tried to parse
		return false, &badVersionError{
			major: verInfo.Major,
			minor: verInfo.Minor,
			inner: err,
		}
	}

	supportsHostAliases := ver.GreaterThan(minimumHostAliasesVersionRequired) ||
		ver.Equal(minimumHostAliasesVersionRequired)

	return supportsHostAliases, nil
}

// Sometimes kubernetes returns a version which aren't valid semver versions
// or invalid enough that the version package can't parse them e.g. GCP returns 1.14+
func cleanVersion(version string) string {
	// Try to find the index of the first symbol that isn't a digit
	// use all the digits before that symbol as the version
	nonDigitIndex := strings.IndexFunc(version, func(r rune) bool {
		return !unicode.IsDigit(r)
	})

	if nonDigitIndex == -1 {
		return version
	}

	return version[:nonDigitIndex]
}
