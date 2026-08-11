package build

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDockerInspectArgs(t *testing.T) {
	tests := map[string]struct {
		image string

		ciRegistryUser     string
		ciRegistryPassword string
		dockerHubUser      string
		dockerHubPassword  string

		expectArgs []string
	}{
		"gitlab.com image uses gitlab.com creds": {
			image: "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:bleeding",

			ciRegistryUser:     "gitlab-user",
			ciRegistryPassword: "gitlab-pass",
			dockerHubUser:      "dockerhub-user",
			dockerHubPassword:  "dockerhub-pass",

			expectArgs: []string{
				"inspect", "--raw", "--no-tags",
				"--username", "gitlab-user", "--password", "gitlab-pass",
			},
		},
		"gitlab.com image is matched case-insensitively": {
			image: "Registry.GitLab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:bleeding",

			ciRegistryUser:     "gitlab-user",
			ciRegistryPassword: "gitlab-pass",

			expectArgs: []string{
				"inspect", "--raw", "--no-tags",
				"--username", "gitlab-user", "--password", "gitlab-pass",
			},
		},
		"docker hub image uses docker hub creds, never gitlab.com creds": {
			image: "registry.hub.docker.com/gitlab/gitlab-runner-helper:bleeding",

			ciRegistryUser:     "gitlab-user",
			ciRegistryPassword: "gitlab-pass",
			dockerHubUser:      "dockerhub-user",
			dockerHubPassword:  "dockerhub-pass",

			expectArgs: []string{
				"inspect", "--raw", "--no-tags",
				"--username", "dockerhub-user", "--password", "dockerhub-pass",
			},
		},
		"docker hub image is matched case-insensitively": {
			image: "Registry.Hub.Docker.com/gitlab/gitlab-runner-helper:bleeding",

			dockerHubUser:     "dockerhub-user",
			dockerHubPassword: "dockerhub-pass",

			expectArgs: []string{
				"inspect", "--raw", "--no-tags",
				"--username", "dockerhub-user", "--password", "dockerhub-pass",
			},
		},
		"third party registry never gets any creds": {
			image: "quay.io/skopeo/stable:v1.12.0",

			ciRegistryUser:     "gitlab-user",
			ciRegistryPassword: "gitlab-pass",
			dockerHubUser:      "dockerhub-user",
			dockerHubPassword:  "dockerhub-pass",

			expectArgs: []string{"inspect", "--raw", "--no-tags"},
		},
		"gitlab.com image with only docker hub creds set gets no creds": {
			image: "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:bleeding",

			dockerHubUser:     "dockerhub-user",
			dockerHubPassword: "dockerhub-pass",

			expectArgs: []string{"inspect", "--raw", "--no-tags"},
		},
		"docker hub image with only gitlab.com creds set gets no creds": {
			image: "registry.hub.docker.com/gitlab/gitlab-runner-helper:bleeding",

			ciRegistryUser:     "gitlab-user",
			ciRegistryPassword: "gitlab-pass",

			expectArgs: []string{"inspect", "--raw", "--no-tags"},
		},
		"no creds set at all falls through unauthenticated": {
			image: "registry.hub.docker.com/gitlab/gitlab-runner-helper:bleeding",

			expectArgs: []string{"inspect", "--raw", "--no-tags"},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv("CI_REGISTRY_USER", tt.ciRegistryUser)
			t.Setenv("CI_REGISTRY_PASSWORD", tt.ciRegistryPassword)
			t.Setenv("DOCKER_HUB_USER", tt.dockerHubUser)
			t.Setenv("DOCKER_HUB_PASSWORD", tt.dockerHubPassword)

			args := dockerInspectArgs(tt.image)

			require.Equal(t, tt.expectArgs, args)
		})
	}
}
