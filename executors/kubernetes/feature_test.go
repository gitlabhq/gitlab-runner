//go:build !integration

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authzv1 "k8s.io/api/authorization/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestKubeClientFeatureChecker_IsHostAliasSupported(t *testing.T) {
	t.Parallel()

	kubeClientErr := errors.New("clientErr")

	tests := map[string]struct {
		version   *version.Info
		clientErr error
		fn        func(*testing.T, featureChecker)
	}{
		"host aliases supported version 1.7": {
			version: &version.Info{
				Major: "1",
				Minor: "7",
			},
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.NoError(t, err)
				assert.True(t, supported)
			},
		},
		"host aliases supported version 1.11": {
			version: &version.Info{
				Major: "1",
				Minor: "11",
			},
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.NoError(t, err)
				assert.True(t, supported)
			},
		},
		"host aliases not supported version 1.6": {
			version: &version.Info{
				Major: "1",
				Minor: "6",
			},
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.NoError(t, err)
				assert.False(t, supported)
			},
		},
		"host aliases cleanup version 1.6 not supported": {
			version: &version.Info{
				Major: "1+535111",
				Minor: "6.^&5151111",
			},
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.NoError(t, err)
				assert.False(t, supported)
			},
		},
		"host aliases cleanup version 1.14 supported": {
			version: &version.Info{
				Major: "1*)(535111",
				Minor: "14^^%&5151111",
			},
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.NoError(t, err)
				assert.True(t, supported)
			},
		},
		"host aliases cleanup invalid version with leading characters not supported": {
			version: &version.Info{
				Major: "+1",
				Minor: "-14",
			},
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.Error(t, err)
				assert.False(t, supported)
				assert.ErrorIs(t, err, &badVersionError{})
				assert.Contains(t, err.Error(), "parsing Kubernetes version +1.-14")
			},
		},
		"host aliases invalid version": {
			version: &version.Info{
				Major: "aaa",
				Minor: "bbb",
			},
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.Error(t, err)
				assert.False(t, supported)
				assert.ErrorIs(t, err, &badVersionError{})
			},
		},
		"host aliases empty version": {
			version: &version.Info{
				Major: "",
				Minor: "",
			},
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.Error(t, err)
				assert.False(t, supported)
				assert.ErrorIs(t, err, &badVersionError{})
			},
		},
		"host aliases kube client error": {
			clientErr: kubeClientErr,
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.Error(t, err)
				assert.ErrorIs(t, err, kubeClientErr)
				assert.False(t, supported)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			t.Parallel()

			fakeClient := &FakeClient{
				Interface: nil, // explicitly setting the inner client to nil, to show we only call Discovery() and nothing else
				FakeDiscovery: &FakeDiscovery{
					FakeVersion:    tt.version,
					FakeVersionErr: tt.clientErr,
				},
			}

			featureChecker := &kubeClientFeatureChecker{kubeClient: fakeClient}

			tt.fn(t, featureChecker)
		})
	}
}

func TestKubeClientFeatureChecker_ResouceVerbAllowed(t *testing.T) {
	t.Parallel()

	namespace := "some-namespace"
	gvr := v1.GroupVersionResource{Group: "blipp.blapp.io", Version: "v1delta5", Resource: "thingamajigs"}
	verb := "blarg"

	tests := map[string]struct {
		apiResult *authzv1.SelfSubjectAccessReview
		apiError  error

		expectedErrorMsg string
		expectedReason   string
		expectedAllowed  bool
	}{
		"allowed": {
			apiResult:       &authzv1.SelfSubjectAccessReview{Status: authzv1.SubjectAccessReviewStatus{Allowed: true}},
			expectedAllowed: true,
		},
		"not allowed": {
			apiResult:      &authzv1.SelfSubjectAccessReview{Status: authzv1.SubjectAccessReviewStatus{Allowed: false}},
			expectedReason: "not allowed: blarg on thingamajigs",
		},
		"denied": {
			apiResult:      &authzv1.SelfSubjectAccessReview{Status: authzv1.SubjectAccessReviewStatus{Denied: false}},
			expectedReason: "not allowed: blarg on thingamajigs",
		},
		"errors": {
			apiResult:        &authzv1.SelfSubjectAccessReview{},
			apiError:         fmt.Errorf("some api error"),
			expectedErrorMsg: "SelfSubjectAccessReview creation: some api error",
		},
		"evaluation error": {
			apiResult:      &authzv1.SelfSubjectAccessReview{Status: authzv1.SubjectAccessReviewStatus{EvaluationError: "some evaluation error"}},
			expectedReason: "SelfSubjectAccessReview evaluation error: some evaluation error",
		},
		"with reason": {
			apiResult:      &authzv1.SelfSubjectAccessReview{Status: authzv1.SubjectAccessReviewStatus{Allowed: false, Reason: "some reason"}},
			expectedReason: "not allowed: blarg on thingamajigs (reason: some reason)",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			fakeClient := fake.NewSimpleClientset()
			ctx := context.TODO()

			fakeClient.PrependReactor("create", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
				creatAction := action.(k8stesting.CreateAction)
				review := creatAction.GetObject().(*authzv1.SelfSubjectAccessReview)

				assert.Equal(t, namespace, review.Spec.ResourceAttributes.Namespace, "create request for wrong namespace")
				assert.Equal(t, gvr.Group, review.Spec.ResourceAttributes.Group, "create request for wrong apiGroup")
				assert.Equal(t, gvr.Version, review.Spec.ResourceAttributes.Version, "create request for wrong apiVersion")
				assert.Equal(t, gvr.Resource, review.Spec.ResourceAttributes.Resource, "create request for wrong resource name")
				assert.Equal(t, verb, review.Spec.ResourceAttributes.Verb, "create request for wrong verb")

				return true, test.apiResult, test.apiError
			})

			featureChecker := &kubeClientFeatureChecker{fakeClient}
			allowed, reason, err := featureChecker.IsResourceVerbAllowed(ctx, gvr, namespace, verb)

			if test.expectedErrorMsg == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.expectedErrorMsg)
			}
			assert.Equal(t, test.expectedAllowed, allowed, "allowed")
			assert.Equal(t, test.expectedReason, reason, "reason")
		})
	}
}
