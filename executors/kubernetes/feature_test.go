//go:build !integration

package kubernetes

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	k8sversion "k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/rest/fake"
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
				resp.Header.Add("Content-Type", "application/json")

				return resp, nil
			}
			fc := kubeClientFeatureChecker{
				kubeClient: testKubernetesClient(version, fake.CreateHTTPClient(rt)),
			}

			tt.fn(t, &fc)
		})
	}
}
