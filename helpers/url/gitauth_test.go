//go:build !integration

package url_helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func defaultConfig() GitAuthConfig {
	return GitAuthConfig{
		CloneURL:    "https://gitlab.example.com/",
		RepoURL:     "https://gitlab.example.com/group/project.git",
		ProjectPath: "group/project",
		Token:       "abc123",
		Server: GitAuthServerConfig{
			Host: "gitlab.example.com",
		},
	}
}

func TestGetRemoteURL(t *testing.T) {
	tests := []struct {
		name          string
		config        GitAuthConfig
		authenticated bool
		expected      string
	}{
		{
			name:          "authenticated with CloneURL",
			config:        defaultConfig(),
			authenticated: true,
			expected:      "https://gitlab-ci-token:abc123@gitlab.example.com/group/project.git",
		},
		{
			name:          "unauthenticated with CloneURL",
			config:        defaultConfig(),
			authenticated: false,
			expected:      "https://gitlab.example.com/group/project.git",
		},
		{
			name: "authenticated with HTTP CloneURL",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = "http://gitlab.example.com/"
				return c
			}(),
			authenticated: true,
			expected:      "http://gitlab-ci-token:abc123@gitlab.example.com/group/project.git",
		},
		{
			name: "falls back to RepoURL when CloneURL is empty authenticated preserves credentials",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = ""
				return c
			}(),
			authenticated: true,
			expected:      "https://gitlab.example.com/group/project.git",
		},
		{
			name: "falls back to RepoURL when CloneURL is empty unauthenticated strips credentials",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = ""
				c.RepoURL = "https://foo:bar@gitlab.example.com/group/project.git"
				return c
			}(),
			authenticated: false,
			expected:      "https://gitlab.example.com/group/project.git",
		},
		{
			name: "falls back to RepoURL when CloneURL has no scheme",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = "not-a-url"
				return c
			}(),
			authenticated: true,
			expected:      "https://gitlab.example.com/group/project.git",
		},
		{
			name: "CloneURL with path",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = "https://gitlab.example.com/base"
				return c
			}(),
			authenticated: true,
			expected:      "https://gitlab-ci-token:abc123@gitlab.example.com/base/group/project.git",
		},
		{
			name: "CloneURL with path and trailing slash",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = "https://gitlab.example.com/base/"
				return c
			}(),
			authenticated: true,
			expected:      "https://gitlab-ci-token:abc123@gitlab.example.com/base/group/project.git",
		},
		{
			name: "SSH CloneURL defaults to git user",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = "ssh://gitlab.example.com/"
				return c
			}(),
			authenticated: true,
			expected:      "ssh://git@gitlab.example.com/group/project.git",
		},
		{
			name: "SSH CloneURL preserves existing user",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = "ssh://deploy@gitlab.example.com/"
				return c
			}(),
			authenticated: true,
			expected:      "ssh://deploy@gitlab.example.com/group/project.git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewGitAuthHelper(tt.config, tt.authenticated)
			u, err := h.GetRemoteURL()
			require.NoError(t, err)
			assert.Equal(t, tt.expected, u.String())
		})
	}
}

func TestGetInsteadOfs(t *testing.T) {
	tests := []struct {
		name          string
		config        GitAuthConfig
		authenticated bool
		expected      [][2]string
		expectErr     bool
	}{
		// Authenticated mode
		{
			name:          "authenticated basic HTTPS rewrite",
			config:        defaultConfig(),
			authenticated: true,
			expected: [][2]string{
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
			},
		},
		{
			name: "authenticated HTTP rewrite",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = "http://gitlab.example.com/"
				c.RepoURL = "http://gitlab.example.com/group/project.git"
				return c
			}(),
			authenticated: true,
			expected: [][2]string{
				{"http://gitlab-ci-token:abc123@gitlab.example.com", "http://gitlab.example.com"},
				{"http://gitlab-ci-token:abc123@gitlab.example.com", "http://gitlab.example.com"},
			},
		},
		{
			name: "authenticated with directory URL",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = "https://gitlab.example.com/gitlab"
				c.GitSubmoduleForceHTTPS = true
				return c
			}(),
			authenticated: true,
			expected: [][2]string{
				{"https://gitlab-ci-token:abc123@gitlab.example.com/gitlab", "https://gitlab.example.com/gitlab"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com/gitlab/", "git@gitlab.example.com:"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com/gitlab", "ssh://git@gitlab.example.com"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
			},
		},
		{
			name: "authenticated with directory URL trailing slash stripped",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = "https://gitlab.example.com/gitlab/"
				c.GitSubmoduleForceHTTPS = true
				return c
			}(),
			authenticated: true,
			expected: [][2]string{
				{"https://gitlab-ci-token:abc123@gitlab.example.com/gitlab", "https://gitlab.example.com/gitlab"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com/gitlab/", "git@gitlab.example.com:"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com/gitlab", "ssh://git@gitlab.example.com"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
			},
		},
		{
			name: "authenticated with submodule force HTTPS and default SSH port",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.GitSubmoduleForceHTTPS = true
				return c
			}(),
			authenticated: true,
			expected: [][2]string{
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com/", "git@gitlab.example.com:"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "ssh://git@gitlab.example.com"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
			},
		},
		{
			name: "authenticated with submodule force HTTPS and custom SSH port",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.GitSubmoduleForceHTTPS = true
				c.Server.SSHPort = "8022"
				return c
			}(),
			authenticated: true,
			expected: [][2]string{
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "ssh://git@gitlab.example.com:8022"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
			},
		},
		{
			name: "authenticated with submodule force HTTPS and explicit port 22",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.GitSubmoduleForceHTTPS = true
				c.Server.SSHPort = "22"
				return c
			}(),
			authenticated: true,
			expected: [][2]string{
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com/", "git@gitlab.example.com:"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "ssh://git@gitlab.example.com"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
			},
		},
		{
			name: "authenticated with custom SSH host",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.GitSubmoduleForceHTTPS = true
				c.Server.SSHHost = "ssh.gitlab.example.com"
				return c
			}(),
			authenticated: true,
			expected: [][2]string{
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com/", "git@ssh.gitlab.example.com:"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "ssh://git@ssh.gitlab.example.com"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
			},
		},
		{
			name: "authenticated RepoURL differs from CloneURL",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = "https://runner-mirror.example.com/"
				return c
			}(),
			authenticated: true,
			expected: [][2]string{
				{"https://gitlab-ci-token:abc123@runner-mirror.example.com", "https://runner-mirror.example.com"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
			},
		},
		{
			name: "authenticated falls back to CredentialsURL when CloneURL is empty",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = ""
				c.CredentialsURL = "https://credentials.example.com/"
				return c
			}(),
			authenticated: true,
			expected: [][2]string{
				{"https://gitlab-ci-token:abc123@credentials.example.com", "https://credentials.example.com"},
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
			},
		},
		{
			name: "authenticated SSH RepoURL skips repoBase entry",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.RepoURL = "ssh://git@gitlab.example.com/group/project.git"
				return c
			}(),
			authenticated: true,
			expected: [][2]string{
				{"https://gitlab-ci-token:abc123@gitlab.example.com", "https://gitlab.example.com"},
			},
		},
		{
			name: "authenticated SSH CloneURL returns nil",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = "ssh://gitlab.example.com/"
				return c
			}(),
			authenticated: true,
			expected:      nil,
		},
		{
			name: "authenticated invalid CredentialsURL fallback returns error",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = ""
				c.CredentialsURL = "://bad"
				return c
			}(),
			authenticated: true,
			expectErr:     true,
		},

		// Unauthenticated mode
		{
			name:          "unauthenticated no rewrites without submodule force HTTPS",
			config:        defaultConfig(),
			authenticated: false,
			expected:      nil,
		},
		{
			name: "unauthenticated SSH rewrites without credentials",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.GitSubmoduleForceHTTPS = true
				return c
			}(),
			authenticated: false,
			expected: [][2]string{
				{"https://gitlab.example.com/", "git@gitlab.example.com:"},
				{"https://gitlab.example.com", "ssh://git@gitlab.example.com"},
			},
		},
		{
			name: "unauthenticated SSH rewrites with custom port",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.GitSubmoduleForceHTTPS = true
				c.Server.SSHPort = "8022"
				return c
			}(),
			authenticated: false,
			expected: [][2]string{
				{"https://gitlab.example.com", "ssh://git@gitlab.example.com:8022"},
			},
		},
		{
			name: "unauthenticated with directory URL and force HTTPS",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = "https://gitlab.example.com/gitlab"
				c.GitSubmoduleForceHTTPS = true
				return c
			}(),
			authenticated: false,
			expected: [][2]string{
				{"https://gitlab.example.com/gitlab/", "git@gitlab.example.com:"},
				{"https://gitlab.example.com/gitlab", "ssh://git@gitlab.example.com"},
			},
		},
		{
			name: "unauthenticated with trailing slash and force HTTPS",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = "https://gitlab.example.com/"
				c.GitSubmoduleForceHTTPS = true
				return c
			}(),
			authenticated: false,
			expected: [][2]string{
				{"https://gitlab.example.com/", "git@gitlab.example.com:"},
				{"https://gitlab.example.com", "ssh://git@gitlab.example.com"},
			},
		},
		{
			name: "unauthenticated SSH CloneURL returns nil",
			config: func() GitAuthConfig {
				c := defaultConfig()
				c.CloneURL = "ssh://git@gitlab.example.com"
				c.GitSubmoduleForceHTTPS = true
				return c
			}(),
			authenticated: false,
			expected:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := NewGitAuthHelper(tt.config, tt.authenticated)
			result, err := h.GetInsteadOfs()
			if tt.expectErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEffectiveSSHHost(t *testing.T) {
	tests := []struct {
		name     string
		server   GitAuthServerConfig
		expected string
	}{
		{
			name:     "uses SSHHost when set",
			server:   GitAuthServerConfig{Host: "gitlab.example.com", SSHHost: "ssh.example.com"},
			expected: "ssh.example.com",
		},
		{
			name:     "falls back to Host",
			server:   GitAuthServerConfig{Host: "gitlab.example.com"},
			expected: "gitlab.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.server.EffectiveSSHHost())
		})
	}
}
