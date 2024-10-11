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

// authenticatedURLHelper is a urlHelper which adds auth data / tokens to URLs where necessary, thus not relying on any other
// credential helper set up.
type authenticatedURLHelper struct {
	config *urlHelperConfig
}

// GetRemoteURL checks if the default clone URL is overwritten by the runner
// configuration option: 'CloneURL'. If it is, we use that to create the clone
// URL.
func (uh *authenticatedURLHelper) GetRemoteURL() (string, error) {
	u, _ := url.Parse(uh.config.CloneURL)

	if u == nil || u.Scheme == "" {
		return uh.config.RepoURL, nil
	}

	projectPath := uh.config.CiProjectPath + ".git"
	u.Path = path.Join(u.Path, projectPath)

	return uh.defaultAuthWithToken(u.String())
}

// GetURLInsteadOfArgs rewrites a plain HTTPS base URL and the most commonly used SSH/Git
// protocol URLs (including custom SSH ports) into an HTTPS URL with injected job token
// auth, and returns an array of strings to pass as options to git commands.
func (uh *authenticatedURLHelper) GetURLInsteadOfArgs() ([]string, error) {
	baseURL := uh.getBaseURL()
	if !strings.HasPrefix(baseURL, "http") {
		return []string{}, nil
	}

	baseURLWithAuth, err := uh.defaultAuthWithToken(baseURL)
	if err != nil {
		return []string{}, err
	}

	// https://example.com/ 		-> https://gitlab-ci-token:abc123@example.com/
	args := uh.getURLInsteadOf(baseURLWithAuth, baseURL)

	return append(args, uh.getURLInsteadOfArgs(baseURLWithAuth)...), nil
}

// defaultAuth ensures the URL has appropriate auth data set.
// For ssh URLs it ensures valid userinfo is defaulted. For other URLs it sets the userinfo according to the provided
// argument.
func (uh *authenticatedURLHelper) defaultAuth(repoURL string, userInfo *url.Userinfo) (string, error) {
	u, _ := url.Parse(repoURL)

	if u == nil {
		return "", fmt.Errorf("invalid URL")
	}

	if u.Scheme == "ssh" {
		if u.User == nil {
			u.User = url.User("git")
		}
	} else {
		u.User = userInfo
	}

	return u.String(), nil
}

// defaultAuthWithToken wraps around defaultAuth and ensures the userinfo is set according to the token from the
// urlHelperConfig.
func (uh *authenticatedURLHelper) defaultAuthWithToken(baseURL string) (string, error) {
	return uh.defaultAuth(baseURL, url.UserPassword("gitlab-ci-token", uh.config.Token))
}

// getURLInsteadOfArgs, if submodules are forced to use https, returns a list of git insteadOf stanzaz, which can be
// used as commandline arguments on git calls.
func (uh *authenticatedURLHelper) getURLInsteadOfArgs(baseURL string) []string {
	args := []string{}

	if !uh.config.GitSubmoduleForceHTTPS {
		return args
	}

	ciServerPort := uh.config.CiServerShellSshPort
	ciServerHost := uh.config.CiServerShellSshHost
	if ciServerHost == "" {
		ciServerHost = uh.config.CiServerHost
	}

	if ciServerPort == "" || ciServerPort == "22" {
		// git@example.com: 		-> https://gitlab-ci-token:abc123@example.com/
		baseGitURL := fmt.Sprintf("git@%s:", ciServerHost)

		args = append(args, uh.getURLInsteadOf(baseURL+"/", baseGitURL)...)
		// ssh://git@example.com/ 	-> https://gitlab-ci-token:abc123@example.com/
		baseSSHGitURL := fmt.Sprintf("ssh://git@%s", ciServerHost)
		args = append(args, uh.getURLInsteadOf(baseURL, baseSSHGitURL)...)
	} else {
		// ssh://git@example.com:8022/ 	-> https://gitlab-ci-token:abc123@example.com/
		baseSSHGitURLWithPort := fmt.Sprintf("ssh://git@%s:%s", ciServerHost, ciServerPort)
		args = append(args, uh.getURLInsteadOf(baseURL, baseSSHGitURLWithPort)...)
	}

	return args
}

// getURLInsteadOf returns git commandline arguments, which "redirect" any git operation from the original to a
// replacement URL
func (uh *authenticatedURLHelper) getURLInsteadOf(replacement, original string) []string {
	return []string{"-c", fmt.Sprintf("url.%s.insteadOf=%s", replacement, original)}
}

func (uh *authenticatedURLHelper) getBaseURL() string {
	u, _ := url.Parse(uh.config.CloneURL)

	if u == nil || u.Scheme == "" {
		return strings.TrimRight(uh.config.CredentialsURL, "/")
	}

	return strings.TrimRight(u.String(), "/")
}

// unauthenticatedURLHelper wraps around authenticatedURLs and ensures to remove tokens from various URLs.
// Using unauthenticatedURLHelper reduces the chance of leaking tokens, because those won't make it into e.g. the git config
// anymore, however it also means that auth needs to be done via other means, e.g. a git cred helper.
type unauthenticatedURLHelper struct {
	*authenticatedURLHelper
}

// GetRemoteURL gets the remote URL and explicitly ensures the token is removed.
func (uh *unauthenticatedURLHelper) GetRemoteURL() (string, error) {
	u, err := uh.authenticatedURLHelper.GetRemoteURL()
	if err != nil {
		return "", err
	}

	return uh.defaultAuth(u, nil)
}

// GetURLInsteadOfArgs rewrites the most commonly used SSH/Git protocol URLs (including custom SSH ports) into an
// http(s) URL, and returns an array of strings to pass as options to git commands.
func (uh *unauthenticatedURLHelper) GetURLInsteadOfArgs() ([]string, error) {
	baseURL := uh.getBaseURL()
	if !strings.HasPrefix(baseURL, "http") {
		return []string{}, nil
	}

	return uh.getURLInsteadOfArgs(baseURL), nil
}
