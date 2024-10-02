package common

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

type urlHelperConfig struct {
	CloneURL               string
	CredentialsURL         string
	RepoURL                string
	GitSubmoduleForceHTTPS bool

	Token string

	CiProjectPath        string
	CiServerShellSshPort string
	CiServerShellSshHost string
	CiServerHost         string
}

// baseUrlHelper holds shared functionality and configuration for other url helpers, which can be made use of by
// embedding this baseUrlHelper.
type baseUrlHelper struct {
	config *urlHelperConfig
}

func (uh *baseUrlHelper) getURLInsteadOf(replacement, original string) []string {
	return []string{"-c", fmt.Sprintf("url.%s.insteadOf=%s", replacement, original)}
}

func (uh *baseUrlHelper) getBaseURL() string {
	u, _ := url.Parse(uh.config.CloneURL)

	if u == nil || u.Scheme == "" {
		return uh.config.CredentialsURL
	}

	return u.String()
}

// urlsWithoutToken is a urlHelper which does not add any auth data / tokens to different URLs, but rather relies on a
// git cred helper to be set up.
type urlsWithoutToken struct {
	*baseUrlHelper
}

// GetRemoteURL checks if the default clone URL is overwritten by the runner
// configuration option: 'CloneURL'. If it is, we use that to create the clone
// URL.
// Additionally, we remove auth data, except for SSH URLs and instead rely on the fact that the auth data is made
// available elsewhere (e.g. git cred helper).
func (uh *urlsWithoutToken) GetRemoteURL() string {
	u, _ := url.Parse(uh.config.CloneURL)

	if u == nil || u.Scheme == "" {
		return uh.cleanAuthData(uh.config.RepoURL)
	}

	projectPath := uh.config.CiProjectPath + ".git"
	u.Path = path.Join(u.Path, projectPath)

	return uh.cleanAuthData(u.String())
}

// GetURLInsteadOfArgs rewrites the most commonly used SSH/Git protocol URLs (including custom SSH ports) into an
// http(s) URL, and returns an array of strings to pass as options to git commands.
func (uh *urlsWithoutToken) GetURLInsteadOfArgs() []string {
	baseURL := strings.TrimRight(uh.getBaseURL(), "/")
	if !strings.HasPrefix(baseURL, "http") {
		return []string{}
	}

	args := []string{}

	if uh.config.GitSubmoduleForceHTTPS {
		ciServerPort := uh.config.CiServerShellSshPort
		ciServerHost := uh.config.CiServerShellSshHost
		if ciServerHost == "" {
			ciServerHost = uh.config.CiServerHost
		}

		if ciServerPort == "" || ciServerPort == "22" {
			// git@example.com: 		-> https://example.com/
			baseGitURL := fmt.Sprintf("git@%s:", ciServerHost)
			args = append(args, uh.getURLInsteadOf(baseURL+"/", baseGitURL)...)

			// ssh://git@example.com/ 	-> https://example.com/
			baseSSHGitURL := fmt.Sprintf("ssh://git@%s", ciServerHost)
			args = append(args, uh.getURLInsteadOf(baseURL, baseSSHGitURL)...)
		} else {
			// ssh://git@example.com:8022/ 	-> https://example.com/
			baseSSHGitURLWithPort := fmt.Sprintf("ssh://git@%s:%s", ciServerHost, ciServerPort)
			args = append(args, uh.getURLInsteadOf(baseURL, baseSSHGitURLWithPort)...)
		}
	}

	return args
}

// cleanAuthData returns a new URL with auth data set up so that:
// - on ssh URLs we ensure UserInfo is defaulted correctly
// - on other URLs we ensure UserInfo is not set at all, so that we don't leak creds
func (uh *urlsWithoutToken) cleanAuthData(repoURL string) string {
	u, _ := url.Parse(repoURL)

	if u.Scheme == "ssh" {
		if u.User == nil {
			u.User = url.User("git")
		}
	} else {
		u.User = nil
	}

	return u.String()
}

// urlsWithToken is a urlHelper which adds auth data / tokens to URLs where necessary, thus not relying on any other
// credential helper set up.
type urlsWithToken struct {
	*baseUrlHelper
}

// GetRemoteURL checks if the default clone URL is overwritten by the runner
// configuration option: 'CloneURL'. If it is, we use that to create the clone
// URL.
func (uh *urlsWithToken) GetRemoteURL() string {
	u, _ := url.Parse(uh.config.CloneURL)

	if u == nil || u.Scheme == "" {
		return uh.config.RepoURL
	}

	projectPath := uh.config.CiProjectPath + ".git"
	u.Path = path.Join(u.Path, projectPath)

	return uh.getURLWithAuth(u.String())
}

// GetURLInsteadOfArgs rewrites a plain HTTPS base URL and the most commonly used SSH/Git
// protocol URLs (including custom SSH ports) into an HTTPS URL with injected job token
// auth, and returns an array of strings to pass as options to git commands.
func (uh *urlsWithToken) GetURLInsteadOfArgs() []string {
	baseURL := strings.TrimRight(uh.getBaseURL(), "/")
	if !strings.HasPrefix(baseURL, "http") {
		return []string{}
	}

	baseURLWithAuth := uh.getURLWithAuth(baseURL)

	// https://example.com/ 		-> https://gitlab-ci-token:abc123@example.com/
	args := uh.getURLInsteadOf(baseURLWithAuth, baseURL)

	if uh.config.GitSubmoduleForceHTTPS {
		ciServerPort := uh.config.CiServerShellSshPort
		ciServerHost := uh.config.CiServerShellSshHost
		if ciServerHost == "" {
			ciServerHost = uh.config.CiServerHost
		}

		if ciServerPort == "" || ciServerPort == "22" {
			// git@example.com: 		-> https://gitlab-ci-token:abc123@example.com/
			baseGitURL := fmt.Sprintf("git@%s:", ciServerHost)

			args = append(args, uh.getURLInsteadOf(baseURLWithAuth+"/", baseGitURL)...)
			// ssh://git@example.com/ 	-> https://gitlab-ci-token:abc123@example.com/
			baseSSHGitURL := fmt.Sprintf("ssh://git@%s", ciServerHost)
			args = append(args, uh.getURLInsteadOf(baseURLWithAuth, baseSSHGitURL)...)
		} else {
			// ssh://git@example.com:8022/ 	-> https://gitlab-ci-token:abc123@example.com/
			baseSSHGitURLWithPort := fmt.Sprintf("ssh://git@%s:%s", ciServerHost, ciServerPort)
			args = append(args, uh.getURLInsteadOf(baseURLWithAuth, baseSSHGitURLWithPort)...)
		}
	}
	return args
}

// getURLWithAuth ensures the URL has appropriate auth data set.
func (uh *urlsWithToken) getURLWithAuth(repoURL string) string {
	u, _ := url.Parse(repoURL)

	if u.Scheme == "ssh" {
		if u.User == nil {
			u.User = url.User("git")
		}
	} else {
		u.User = url.UserPassword("gitlab-ci-token", uh.config.Token)
	}

	return u.String()
}
