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
func (uh *authenticatedURLHelper) GetRemoteURL() (*url.URL, error) {
	u, _ := url.Parse(uh.config.CloneURL)

	if u == nil || u.Scheme == "" {
		return url.Parse(uh.config.RepoURL)
	}

	projectPath := uh.config.CiProjectPath + ".git"
	u.Path = path.Join(u.Path, projectPath)

	return uh.defaultAuthWithToken(u)
}

// GetInsteadOfs rewrites a plain HTTPS base URL and the most commonly used SSH/Git
// protocol URLs (including custom SSH ports) into an HTTPS URL with injected job token
// auth, and returns an array of all replacements.
func (uh *authenticatedURLHelper) GetInsteadOfs() ([][2]string, error) {
	baseURL, err := uh.getBaseURL()
	if err != nil {
		return nil, fmt.Errorf("invalid URL")
	}

	if !isHTTP(baseURL) {
		return nil, nil
	}

	baseURLWithAuth, err := uh.defaultAuthWithToken(baseURL)
	if err != nil {
		return nil, err
	}

	// https://example.com/ 		-> https://gitlab-ci-token:abc123@example.com/
	insteadOfs := [][2]string{
		{renderTrimmed(baseURLWithAuth), renderTrimmed(baseURL)},
	}

	insteadOfs = append(insteadOfs, uh.getInsteadOfs(renderTrimmed(baseURLWithAuth))...)

	// Always include the RepoURL base (without project path) in insteadOf config to support Git submodules.
	// The RepoURL comes from the GitInfo returned by the API and may differ from the CloneURL configured
	// in the runner. For submodules to clone properly, we need to ensure that the API-provided URL can be
	// rewritten with credentials. We strip the project path from RepoURL to get just the base URL, since
	// submodules may reference different projects on the same host.
	// See: https://gitlab.com/gitlab-org/gitlab-runner/-/issues/39170
	repoURL, err := url.Parse(uh.config.RepoURL)
	if err != nil {
		return nil, err
	}

	if isHTTP(repoURL) {
		// Create a base URL without the project path for submodule support
		repoBaseURL := *repoURL
		repoBaseURL.Path = ""
		repoBaseURLWithAuth, err := uh.defaultAuthWithToken(&repoBaseURL)
		if err != nil {
			return nil, err
		}

		repoBaseURLWithoutAuth := repoBaseURL
		repoBaseURLWithoutAuth.User = nil
		insteadOfs = append(insteadOfs, [2]string{
			renderTrimmed(repoBaseURLWithAuth),
			renderTrimmed(&repoBaseURLWithoutAuth),
		})
	}

	return insteadOfs, nil
}

// defaultAuth ensures the URL has appropriate auth data set.
// For ssh URLs it ensures valid userinfo is defaulted. For other URLs it sets the userinfo according to the provided
// argument.
func (uh *authenticatedURLHelper) defaultAuth(repoURL *url.URL, userInfo *url.Userinfo) (*url.URL, error) {
	if repoURL == nil {
		return nil, fmt.Errorf("invalid URL")
	}

	c := *repoURL

	if c.Scheme == "ssh" {
		if c.User == nil {
			c.User = url.User("git")
		}
	} else {
		c.User = userInfo
	}

	return &c, nil
}

// defaultAuthWithToken wraps around defaultAuth and ensures the userinfo is set according to the token from the
// urlHelperConfig.
func (uh *authenticatedURLHelper) defaultAuthWithToken(baseURL *url.URL) (*url.URL, error) {
	return uh.defaultAuth(baseURL, url.UserPassword("gitlab-ci-token", uh.config.Token))
}

// getInsteadOfs, if submodules are forced to use https, returns a list of git insteadOf replacements.
// It returns a list of tuples of replacementURLs : originalURLs:
//
//	[][2]string{
//		{"http://theReplacementURL.tld", "http://theOriginalURL.tld"},
//		{"http://theReplacementURL.tld", "http://anotherOriginalURL.tld"},
//	}
func (uh *authenticatedURLHelper) getInsteadOfs(baseURL string) [][2]string {
	insteadOfs := [][2]string{}

	if !uh.config.GitSubmoduleForceHTTPS {
		return insteadOfs
	}

	ciServerPort := uh.config.CiServerShellSshPort
	ciServerHost := uh.config.CiServerShellSshHost
	if ciServerHost == "" {
		ciServerHost = uh.config.CiServerHost
	}

	if ciServerPort == "" || ciServerPort == "22" {
		// git@example.com: 		-> https://gitlab-ci-token:abc123@example.com/
		baseGitURL := fmt.Sprintf("git@%s:", ciServerHost)

		insteadOfs = append(insteadOfs, [2]string{baseURL + "/", baseGitURL})
		// ssh://git@example.com/ 	-> https://gitlab-ci-token:abc123@example.com/
		baseSSHGitURL := fmt.Sprintf("ssh://git@%s", ciServerHost)
		insteadOfs = append(insteadOfs, [2]string{baseURL, baseSSHGitURL})
	} else {
		// ssh://git@example.com:8022/ 	-> https://gitlab-ci-token:abc123@example.com/
		baseSSHGitURLWithPort := fmt.Sprintf("ssh://git@%s:%s", ciServerHost, ciServerPort)
		insteadOfs = append(insteadOfs, [2]string{baseURL, baseSSHGitURLWithPort})
	}

	return insteadOfs
}

func (uh *authenticatedURLHelper) getBaseURL() (*url.URL, error) {
	var err error
	u, _ := url.Parse(uh.config.CloneURL)

	if u == nil || u.Scheme == "" {
		u, err = url.Parse(uh.config.CredentialsURL)
	}

	return u, err
}

// unauthenticatedURLHelper wraps around authenticatedURLHelper and ensures to remove tokens from various URLs.
// Using unauthenticatedURLHelper reduces the chance of leaking tokens, because those won't make it into e.g. the git config
// anymore, however it also means that auth needs to be done via other means, e.g. a git cred helper.
type unauthenticatedURLHelper struct {
	*authenticatedURLHelper
}

// GetRemoteURL gets the remote URL and explicitly ensures the token is removed.
func (uh *unauthenticatedURLHelper) GetRemoteURL() (*url.URL, error) {
	u, err := uh.authenticatedURLHelper.GetRemoteURL()
	if err != nil {
		return nil, err
	}

	return uh.defaultAuth(u, nil)
}

// GetInsteadOfs rewrites the most commonly used SSH/Git protocol URLs (including custom SSH ports) into an
// http(s) URL, and returns an array of all replacements.
func (uh *unauthenticatedURLHelper) GetInsteadOfs() ([][2]string, error) {
	baseURL, err := uh.getBaseURL()
	if err != nil {
		return nil, fmt.Errorf("invalid URL")
	}

	if !isHTTP(baseURL) {
		return nil, nil
	}

	return uh.getInsteadOfs(renderTrimmed(baseURL)), nil
}

// isHTTP checks if an URL's scheme is http or https
func isHTTP(u *url.URL) bool {
	return strings.EqualFold("https", u.Scheme) || strings.EqualFold("http", u.Scheme)
}

// renderTrimmed renders an URL into a string and ensures it has no trailing '/'
func renderTrimmed(u *url.URL) string {
	return strings.TrimRight(u.String(), "/")
}
