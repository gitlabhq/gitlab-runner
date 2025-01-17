package kubernetes

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/hashicorp/go-version"
	authzv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//go:generate mockery --name=featureChecker --inpackage
type featureChecker interface {
	IsHostAliasSupported() (bool, error)
	AreResourceVerbsAllowed(context.Context, metav1.GroupVersionResource, string, ...string) (bool, string, error)
}

type kubeClientFeatureChecker struct {
	kubeClient kubernetes.Interface
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
	// kubeAPI: ignore
	verInfo, err := c.kubeClient.Discovery().ServerVersion()
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

// AreResourceVerbsAllowed checks if the current user has all requested permissions for the resource in a specific
// namespace.
// If an error occurred when creating the review for an individual verb, we'll return denied, an empty reason and the
// first error. No further verbs will be checked.
// If an evaluation error occurred, we treat this as denied, and return the first evaluation error as the reason. No
// further verbs will be checked.
// If either Denied is true or Allowed is false, we treat that as denied and return with a reason, if one is available.
// No further verbs will be checked.
// Else, if all verbs are allowed, have no evaluation or creation error, we return allowed.
func (c *kubeClientFeatureChecker) AreResourceVerbsAllowed(ctx context.Context, gvr metav1.GroupVersionResource, namespace string, verbs ...string) (bool, string, error) {
	for _, verb := range verbs {
		if allowed, reason, err := c.isResourceVerbAllowed(ctx, gvr, namespace, verb); err != nil || !allowed {
			return allowed, reason, err
		}
	}
	return true, "", nil
}

func (c *kubeClientFeatureChecker) isResourceVerbAllowed(ctx context.Context, gvr metav1.GroupVersionResource, namespace string, verb string) (bool, string, error) {
	review := &authzv1.SelfSubjectAccessReview{
		Spec: authzv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authzv1.ResourceAttributes{
				Group:     gvr.Group,
				Version:   gvr.Version,
				Resource:  gvr.Resource,
				Namespace: namespace,
				Verb:      verb,
			},
		},
	}

	// We don't need any RBAC permissions to get our own access review
	// kubeAPI: ignore
	res, err := c.kubeClient.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, review, metav1.CreateOptions{})
	if err != nil {
		return false, "", fmt.Errorf("SelfSubjectAccessReview creation: %w", err)
	}

	// EvaluationErrors might not mean denied per se, but we treat it like that, because we can't be sure
	if ee := res.Status.EvaluationError; ee != "" {
		return false, "evaluation error: " + ee, nil
	}

	allowed := res.Status.Allowed && !res.Status.Denied

	if allowed {
		return true, "", nil
	}

	reason := fmt.Sprintf("not allowed: %s on %s", verb, gvr.Resource)
	if r := res.Status.Reason; r != "" {
		reason += " (reason: " + r + ")"
	}
	return false, reason, nil
}
