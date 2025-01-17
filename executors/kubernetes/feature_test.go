//go:build !integration

package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	authzv1 "k8s.io/api/authorization/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8sversion "k8s.io/apimachinery/pkg/version"
	testclient "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestKubeClientFeatureChecker(t *testing.T) {
	kubeClientErr := errors.New("clientErr")

	version, _ := testVersionAndCodec()
	tests := map[string]struct {
		version   k8sversion.Info
		clientErr error
		fn        func(*testing.T, featureChecker)
	}{
		"host aliases supported version 1.7": {
			version: k8sversion.Info{
				Major: "1",
				Minor: "7",
			},
			clientErr: nil,
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.NoError(t, err)
				assert.True(t, supported)
			},
		},
		"host aliases supported version 1.11": {
			version: k8sversion.Info{
				Major: "1",
				Minor: "11",
			},
			clientErr: nil,
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.NoError(t, err)
				assert.True(t, supported)
			},
		},
		"host aliases not supported version 1.6": {
			version: k8sversion.Info{
				Major: "1",
				Minor: "6",
			},
			clientErr: nil,
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.NoError(t, err)
				assert.False(t, supported)
			},
		},
		"host aliases cleanup version 1.6 not supported": {
			version: k8sversion.Info{
				Major: "1+535111",
				Minor: "6.^&5151111",
			},
			clientErr: nil,
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.NoError(t, err)
				assert.False(t, supported)
			},
		},
		"host aliases cleanup version 1.14 supported": {
			version: k8sversion.Info{
				Major: "1*)(535111",
				Minor: "14^^%&5151111",
			},
			clientErr: nil,
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.NoError(t, err)
				assert.True(t, supported)
			},
		},
		"host aliases cleanup invalid version with leading characters not supported": {
			version: k8sversion.Info{
				Major: "+1",
				Minor: "-14",
			},
			clientErr: nil,
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.Error(t, err)
				assert.False(t, supported)
				assert.ErrorIs(t, err, &badVersionError{})
				assert.Contains(t, err.Error(), "parsing Kubernetes version +1.-14")
			},
		},
		"host aliases invalid version": {
			version: k8sversion.Info{
				Major: "aaa",
				Minor: "bbb",
			},
			clientErr: nil,
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.Error(t, err)
				assert.False(t, supported)
				assert.ErrorIs(t, err, &badVersionError{})
			},
		},
		"host aliases empty version": {
			version: k8sversion.Info{
				Major: "",
				Minor: "",
			},
			clientErr: nil,
			fn: func(t *testing.T, fc featureChecker) {
				supported, err := fc.IsHostAliasSupported()
				require.Error(t, err)
				assert.False(t, supported)
				assert.ErrorIs(t, err, &badVersionError{})
			},
		},
		"host aliases kube client error": {
			version: k8sversion.Info{
				Major: "",
				Minor: "",
			},
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
			rt := func(request *http.Request) (response *http.Response, err error) {
				if tt.clientErr != nil {
					return nil, tt.clientErr
				}

				ver, _ := json.Marshal(tt.version)
				resp := &http.Response{
					StatusCode: http.StatusOK,
					Body: FakeReadCloser{
						Reader: bytes.NewReader(ver),
					},
				}
				resp.Header = make(http.Header)
				resp.Header.Add(common.ContentType, "application/json")

				return resp, nil
			}
			fc := kubeClientFeatureChecker{
				kubeClient: testKubernetesClient(version, fake.CreateHTTPClient(rt)),
			}

			tt.fn(t, &fc)
		})
	}
}

func TestKubeClientFeatureChecker_ResouceVerbsAllowed(t *testing.T) {
	tests := map[string]struct {
		apiResults []*authzv1.SelfSubjectAccessReview
		apiErrors  []error

		expectedAccessReviews int
		expectedErrorMsg      string
		expectedReason        string
		expectedAllowed       bool
	}{
		"all allowed": {
			apiResults: []*authzv1.SelfSubjectAccessReview{
				{Status: authzv1.SubjectAccessReviewStatus{Allowed: true}},
				{Status: authzv1.SubjectAccessReviewStatus{Allowed: true}},
			},
			apiErrors:             []error{nil, nil},
			expectedAccessReviews: 2,
			expectedAllowed:       true,
		},
		"1st allowed, 2nd not": {
			apiResults: []*authzv1.SelfSubjectAccessReview{
				{Status: authzv1.SubjectAccessReviewStatus{Allowed: true}},
				{Status: authzv1.SubjectAccessReviewStatus{Allowed: false}},
			},
			apiErrors:             []error{nil, nil},
			expectedAccessReviews: 2,
			expectedReason:        "not allowed: bar on thingamajigs",
		},
		"1st allowed, 2nd denied": {
			apiResults: []*authzv1.SelfSubjectAccessReview{
				{Status: authzv1.SubjectAccessReviewStatus{Allowed: true}},
				{Status: authzv1.SubjectAccessReviewStatus{Denied: true}},
			},
			apiErrors:             []error{nil, nil},
			expectedAccessReviews: 2,
			expectedReason:        "not allowed: bar on thingamajigs",
		},
		"1st errors": {
			apiResults: []*authzv1.SelfSubjectAccessReview{
				{Status: authzv1.SubjectAccessReviewStatus{Allowed: true}},
				{Status: authzv1.SubjectAccessReviewStatus{Allowed: true}},
			},
			apiErrors:             []error{fmt.Errorf("some api error"), nil},
			expectedAccessReviews: 1,
			expectedErrorMsg:      "SelfSubjectAccessReview creation: some api error",
		},
		"2nd has evaluation error": {
			apiResults: []*authzv1.SelfSubjectAccessReview{
				{Status: authzv1.SubjectAccessReviewStatus{Allowed: true}},
				{Status: authzv1.SubjectAccessReviewStatus{EvaluationError: "some evaluation error"}},
			},
			apiErrors:             []error{nil, nil},
			expectedAccessReviews: 2,
			expectedReason:        "evaluation error: some evaluation error",
		},
		"1st with reason": {
			apiResults: []*authzv1.SelfSubjectAccessReview{
				{Status: authzv1.SubjectAccessReviewStatus{Allowed: false, Reason: "some reason"}},
			},
			apiErrors:             []error{nil},
			expectedAccessReviews: 1,
			expectedReason:        "not allowed: foo on thingamajigs (reason: some reason)",
		},
	}

	namespace := "some-namespace"
	gvr := v1.GroupVersionResource{Group: "blipp.blapp.io", Version: "v1delta5", Resource: "thingamajigs"}
	verbs := []string{"foo", "bar"}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			fakeClient := testclient.NewSimpleClientset()
			ctx := context.TODO()

			callCount := 0
			fakeClient.PrependReactor("create", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
				if !isSelfSubjectAccessReview(action.GetResource()) {
					return false, nil, nil
				}

				creatAction := action.(k8stesting.CreateAction)
				review := creatAction.GetObject().(*authzv1.SelfSubjectAccessReview)

				assert.Equal(t, namespace, review.Spec.ResourceAttributes.Namespace, "create request for wrong namespace")
				assert.Equal(t, gvr.Group, review.Spec.ResourceAttributes.Group, "create request for wrong apiGroup")
				assert.Equal(t, gvr.Version, review.Spec.ResourceAttributes.Version, "create request for wrong apiVersion")
				assert.Equal(t, gvr.Resource, review.Spec.ResourceAttributes.Resource, "create request for wrong resource name")
				assert.Equal(t, verbs[callCount], review.Spec.ResourceAttributes.Verb, "create request for wrong verb")

				defer func() { callCount += 1 }()
				return true, test.apiResults[callCount], test.apiErrors[callCount]
			})

			featureChecker := &kubeClientFeatureChecker{fakeClient}
			allowed, reason, err := featureChecker.AreResourceVerbsAllowed(ctx, gvr, namespace, verbs...)

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

func isSelfSubjectAccessReview(gvr schema.GroupVersionResource) bool {
	return gvr.Group == "authorization.k8s.io" && gvr.Version == "v1" && gvr.Resource == "selfsubjectaccessreviews"
}
