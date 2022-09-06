//go:build !integration

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type allowedImageTestCase struct {
	image           string
	allowedImages   []string
	internalImages  []string
	expectedAllowed bool
}

//nolint:lll
var allowedImageTestCases = []allowedImageTestCase{
	{image: "alpine", allowedImages: []string{"alpine"}, internalImages: []string{}, expectedAllowed: true},
	{image: "alpine", allowedImages: []string{"ubuntu"}, internalImages: []string{}, expectedAllowed: false},
	{image: "library/ruby", allowedImages: []string{"*"}, internalImages: []string{}, expectedAllowed: false},
	{image: "library/ruby", allowedImages: []string{"**/*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "library/ruby", allowedImages: []string{"**/*:*"}, internalImages: []string{}, expectedAllowed: false},
	{image: "library/ruby", allowedImages: []string{"*/*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "library/ruby", allowedImages: []string{"*/*:*"}, internalImages: []string{}, expectedAllowed: false},
	{image: "library/ruby:2.1", allowedImages: []string{"*"}, internalImages: []string{}, expectedAllowed: false},
	{image: "library/ruby:2.1", allowedImages: []string{"**/*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "library/ruby:2.1", allowedImages: []string{"**/*:*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "library/ruby:2.1", allowedImages: []string{"*/*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "library/ruby:2.1", allowedImages: []string{"*/*:*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "my.registry.tld/group/subgroup/ruby", allowedImages: []string{"*"}, internalImages: []string{}, expectedAllowed: false},
	{image: "my.registry.tld/group/subgroup/ruby", allowedImages: []string{"my.registry.tld/**/*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "my.registry.tld/group/subgroup/ruby", allowedImages: []string{"my.registry.tld/*/*"}, internalImages: []string{}, expectedAllowed: false},
	{image: "my.registry.tld/group/subgroup/ruby", allowedImages: []string{"my.registry.tld/*/*/*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "my.registry.tld/group/subgroup/ruby:2.1", allowedImages: []string{"*"}, internalImages: []string{}, expectedAllowed: false},
	{image: "my.registry.tld/group/subgroup/ruby:2.1", allowedImages: []string{"my.registry.tld/**/*:*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "my.registry.tld/group/subgroup/ruby:2.1", allowedImages: []string{"my.registry.tld/*/*/*:*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "my.registry.tld/group/subgroup/ruby:2.1", allowedImages: []string{"my.registry.tld/*/*:*"}, internalImages: []string{}, expectedAllowed: false},
	{image: "my.registry.tld/library/ruby", allowedImages: []string{"*"}, internalImages: []string{}, expectedAllowed: false},
	{image: "my.registry.tld/library/ruby", allowedImages: []string{"my.registry.tld/**/*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "my.registry.tld/library/ruby", allowedImages: []string{"my.registry.tld/*/*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "my.registry.tld/library/ruby:2.1", allowedImages: []string{"*"}, internalImages: []string{}, expectedAllowed: false},
	{image: "my.registry.tld/library/ruby:2.1", allowedImages: []string{"my.registry.tld/**/*:*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "my.registry.tld/library/ruby:2.1", allowedImages: []string{"my.registry.tld/*/*:*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "my.registry.tld/ruby", allowedImages: []string{"*"}, internalImages: []string{}, expectedAllowed: false},
	{image: "my.registry.tld/ruby:2.1", allowedImages: []string{"*"}, internalImages: []string{}, expectedAllowed: false},
	{image: "ruby", allowedImages: []string{"*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "ruby", allowedImages: []string{"**/*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "ruby:2.1", allowedImages: []string{"*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "ruby:2.1", allowedImages: []string{"**/*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "ruby:latest", allowedImages: []string{"*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "ruby:latest", allowedImages: []string{"**/*"}, internalImages: []string{}, expectedAllowed: true},
	{image: "gitlab/gitlab-runner-helper", allowedImages: []string{"alpine"}, internalImages: []string{"gitlab/gitlab-runner-helper"}, expectedAllowed: true},
	{image: "alpine", allowedImages: []string{}, internalImages: []string{}, expectedAllowed: true},
}

func TestVerifyAllowedImage(t *testing.T) {
	logger := BuildLogger{}

	for _, test := range allowedImageTestCases {
		options := VerifyAllowedImageOptions{
			Image:          test.image,
			OptionName:     "",
			AllowedImages:  test.allowedImages,
			InternalImages: test.internalImages,
		}
		err := VerifyAllowedImage(options, logger)

		if test.expectedAllowed {
			assert.NoError(t, err, "%q must be allowed by %q", test.image, test.allowedImages)
		} else {
			assert.Error(t, err, "%q must not be allowed by %q", test.image, test.allowedImages)
		}
	}
}
