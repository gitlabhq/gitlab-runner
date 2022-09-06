//go:build !integration

package vm_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/vm"
)

func TestGetBaseName(t *testing.T) {
	tests := map[string]struct {
		image            string
		allowedImages    []string
		buildVariables   common.JobVariables
		expectedBaseName string
		expectedErr      string
	}{
		"empty allowed with no override uses default": {
			image:            "",
			expectedBaseName: "default",
		},
		"empty allowed with identical image uses default": {
			image:            "default",
			expectedBaseName: "default",
		},
		"empty allowed with different image uses default": {
			image:            "image1",
			expectedBaseName: "default",
		},
		"override using valid image and simple pattern": {
			image:            "image1",
			allowedImages:    []string{"image"},
			expectedBaseName: "image1",
		},
		"override using valid image and wildcard pattern": {
			image:            "image1",
			allowedImages:    []string{"^image.*$"},
			expectedBaseName: "image1",
		},
		"override using valid image and numeric pattern": {
			image:            "image1",
			allowedImages:    []string{`^image\d+$`},
			expectedBaseName: "image1",
		},
		"override using valid image and exact match pattern": {
			image:            "image1",
			allowedImages:    []string{"^image1$"},
			expectedBaseName: "image1",
		},
		"override using valid image and multiple patterns": {
			image:            "image1",
			allowedImages:    []string{"^foobar$", "^image1$"},
			expectedBaseName: "image1",
		},
		"override using expanded image and exact match pattern": {
			image:         "${IMAGE}1",
			allowedImages: []string{"^image1$"},
			buildVariables: common.JobVariables{
				{Key: "IMAGE", Value: "image"},
			},
			expectedBaseName: "image1",
		},
		"attempt override using expanded image and disallowed pattern": {
			image:         "${IMAGE}1",
			allowedImages: []string{"^foobar$"},
			buildVariables: common.JobVariables{
				{Key: "IMAGE", Value: "image"},
			},
			expectedErr: "invalid image",
		},
		"attempt override using disallowed pattern": {
			image:         "non_default",
			allowedImages: []string{"^image$"},
			expectedErr:   "invalid image",
		},
		"attempt override using multiple disallowed pattern": {
			image:         "non_default",
			allowedImages: []string{"^image1$", "^image2$"},
			expectedErr:   "invalid image",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			e := new(vm.Executor)
			e.Build = new(common.Build)
			e.Build.Image.Name = tc.image
			e.Build.Variables = append(e.Build.Variables, tc.buildVariables...)

			assert.NoError(t, e.ValidateAllowedImages(tc.allowedImages))

			baseName, err := e.GetBaseName("default")
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expectedBaseName, baseName)
		})
	}
}

func TestValidateAllowedImages(t *testing.T) {
	tests := map[string]struct {
		allowed     []string
		expectedErr string
	}{
		"nil": {
			allowed: nil,
		},
		"empty": {
			allowed: []string{},
		},
		"valid": {
			allowed: []string{"^.*$"},
		},
		"invalid": {
			allowed:     []string{"^.*$", "^[$"},
			expectedErr: "invalid regexp pattern in allowed_images parameter: ^[$",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			executor := vm.Executor{}
			err := executor.ValidateAllowedImages(tc.allowed)
			if tc.expectedErr != "" {
				assert.EqualError(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
