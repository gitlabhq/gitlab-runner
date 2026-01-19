//go:build !integration

package spec

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_Image_ExecutorOptions_GetUIDGID(t *testing.T) {
	tests := map[string]struct {
		kubernetesOptions func() *ImageKubernetesOptions
		expectedError     bool
		expectedUID       int64
		expectedGID       int64
	}{
		"empty user": {
			kubernetesOptions: func() *ImageKubernetesOptions {
				return &ImageKubernetesOptions{
					User: "",
				}
			},
		},
		"only user": {
			kubernetesOptions: func() *ImageKubernetesOptions {
				return &ImageKubernetesOptions{
					User: "1000",
				}
			},
			expectedUID: int64(1000),
		},
		"uid and gid": {
			kubernetesOptions: func() *ImageKubernetesOptions {
				return &ImageKubernetesOptions{
					User: "1000:1000",
				}
			},
			expectedUID: int64(1000),
			expectedGID: int64(1000),
		},
		"invalid user": {
			kubernetesOptions: func() *ImageKubernetesOptions {
				return &ImageKubernetesOptions{
					User: "gitlab-runner",
				}
			},
			expectedError: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			uid, gid, err := tt.kubernetesOptions().GetUIDGID()
			if tt.expectedError {
				require.Error(t, err)
				require.Equal(t, int64(0), uid)
				require.Equal(t, int64(0), gid)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedUID, uid)
			require.Equal(t, tt.expectedGID, gid)
		})
	}
}
