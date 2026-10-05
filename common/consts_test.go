//go:build !integration

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTestImagesVersion(t *testing.T) {
	assert.Equal(t, "development", testImagesVersion(""))
	assert.Equal(t, "0.0.53", testImagesVersion("0.0.53"))
	assert.Contains(t, TestLivenessWindowsImage, "/liveness:")
	assert.Contains(t, TestLivenessWindowsImage, "-nanoserver-%s")
}
