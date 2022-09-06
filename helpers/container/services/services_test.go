//go:build !integration

package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	docker_helpers_test "gitlab.com/gitlab-org/gitlab-runner/helpers/container/services/test"
)

func TestSplitNameAndVersion(t *testing.T) {
	for _, test := range docker_helpers_test.Services {
		t.Run(test.Description, func(t *testing.T) {
			out := SplitNameAndVersion(test.Description)
			service := out.Service
			version := out.Version
			imageName := out.ImageName
			aliases := out.Aliases

			assert.Equal(t, test.Service, service, "service for "+test.Description)
			assert.Equal(t, test.Version, version, "version for "+test.Description)
			assert.Equal(t, test.Image, imageName, "image for "+test.Description)

			require.True(t, len(aliases) > 0, "aliases len for "+test.Description)
			assert.Equal(t, test.Alias, aliases[0], "alias for "+test.Description)
			if test.Alternative != "" {
				require.Len(t, aliases, 2, "aliases len for "+test.Description)
				assert.Equal(t, test.Alternative, aliases[1], "alternative for "+test.Description)
			} else {
				assert.Len(t, aliases, 1, "aliases len for "+test.Description)
			}
		})
	}
}

func TestSplitNameAndVersionEmpty(t *testing.T) {
	expectedService := Service{
		Version:   imageVersionLatest,
		ImageName: "",
	}
	assert.Equal(t, expectedService, SplitNameAndVersion(""))
}
