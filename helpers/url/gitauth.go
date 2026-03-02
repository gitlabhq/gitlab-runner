package url_helpers

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// GitAuthServerConfig holds CI server connection details used for SSH-to-HTTPS rewrites.
type GitAuthServerConfig struct {
	Host    string
	SSHHost string
	SSHPort string
}

// EffectiveSSHHost returns SSHHost if set, otherwise falls back to Host.
func (s GitAuthServerConfig) EffectiveSSHHost() string {
	if s.SSHHost != "" {
		return s.SSHHost
	}
	return s.Host
}

// GitAuthConfig holds the URL and credential settings needed to construct authenticated or
// unauthenticated git remote URLs and insteadOf rewrites.
type GitAuthConfig struct {
	CloneURL               string
	CredentialsURL         string
	RepoURL                string
	GitSubmoduleForceHTTPS bool

	Token string

	ProjectPath string
	Server      GitAuthServerConfig
}

// GitAuthHelper manages clone URLs and git insteadOf rewrites. When authenticated, it injects job
// token credentials into URLs. Otherwise it produces credential-free URLs, relying on an external
// credential helper for auth.
type GitAuthHelper struct {
	config        GitAuthConfig
	authenticated bool
}

// NewGitAuthHelper creates a GitAuthHelper. When authenticated is true, the token from config is
// injected into URLs; when false, URLs are produced without credentials.
func NewGitAuthHelper(config GitAuthConfig, authenticated bool) *GitAuthHelper {
	return &GitAuthHelper{config: config, authenticated: authenticated}
}

// GetRemoteURL returns the clone URL for the project. If CloneURL is configured on the runner it
// takes precedence over the API-provided RepoURL.
func (h *GitAuthHelper) GetRemoteURL() (*url.URL, error) {
	u, _ := url.Parse(h.config.CloneURL)
	if u == nil || u.Scheme == "" {
		u, err := url.Parse(h.config.RepoURL)
		if err != nil {
			return nil, err
		}
		// When authenticated, return the RepoURL as-is (it already contains credentials).
		// When unauthenticated, strip any existing credentials via applyAuth.
		if !h.authenticated {
			return h.applyAuth(u)
		}
		return u, nil
	}

	u.Path = path.Join(u.Path, h.config.ProjectPath+".git")

	return h.applyAuth(u)
}

// GetInsteadOfs returns git insteadOf replacements. In authenticated mode it rewrites plain HTTPS
// base URLs and common SSH/Git protocol URLs into HTTPS URLs with injected job token auth. In
// unauthenticated mode it only rewrites SSH/Git URLs to plain HTTPS (without credentials).
func (h *GitAuthHelper) GetInsteadOfs() ([][2]string, error) {
	baseURL, err := h.getBaseURL()
	if err != nil {
		return nil, fmt.Errorf("invalid URL")
	}

	if !isHTTP(baseURL) {
		return nil, nil
	}

	if !h.authenticated {
		return h.sshInsteadOfs(trimmed(baseURL)), nil
	}

	authedBase, err := h.applyAuth(baseURL)
	if err != nil {
		return nil, err
	}

	// https://example.com/ -> https://gitlab-ci-token:abc123@example.com/
	insteadOfs := [][2]string{
		{trimmed(authedBase), trimmed(baseURL)},
	}
	insteadOfs = append(insteadOfs, h.sshInsteadOfs(trimmed(authedBase))...)

	entry, err := h.repoBaseInsteadOf()
	if err != nil {
		return nil, err
	}
	if entry != nil {
		insteadOfs = append(insteadOfs, *entry)
	}

	return insteadOfs, nil
}

// repoBaseInsteadOf returns an insteadOf entry for the RepoURL base (without the project path) so
// that submodules referencing other projects on the same host can be rewritten with credentials.
// The RepoURL may differ from CloneURL since it comes from the API rather than runner config.
// See: https://gitlab.com/gitlab-org/gitlab-runner/-/issues/39170
func (h *GitAuthHelper) repoBaseInsteadOf() (*[2]string, error) {
	repoURL, err := url.Parse(h.config.RepoURL)
	if err != nil || !isHTTP(repoURL) {
		return nil, err
	}

	base := *repoURL
	base.Path = ""

	authed, err := h.applyAuth(&base)
	if err != nil {
		return nil, err
	}

	base.User = nil

	return &[2]string{trimmed(authed), trimmed(&base)}, nil
}

// applyAuth sets userinfo appropriate for the current mode: job token credentials when
// authenticated, nil when unauthenticated. SSH URLs always default to the "git" user.
func (h *GitAuthHelper) applyAuth(u *url.URL) (*url.URL, error) {
	if u == nil {
		return nil, fmt.Errorf("invalid URL")
	}

	c := *u

	switch {
	case c.Scheme == "ssh":
		if c.User == nil {
			c.User = url.User("git")
		}
	case h.authenticated:
		c.User = url.UserPassword("gitlab-ci-token", h.config.Token)
	default:
		c.User = nil
	}

	return &c, nil
}

// sshInsteadOfs returns insteadOf entries that rewrite SSH/Git protocol URLs to the given HTTPS
// base URL. Returns nil if GitSubmoduleForceHTTPS is not set.
func (h *GitAuthHelper) sshInsteadOfs(baseURL string) [][2]string {
	if !h.config.GitSubmoduleForceHTTPS {
		return nil
	}

	host := h.config.Server.EffectiveSSHHost()
	port := h.config.Server.SSHPort

	if port == "" || port == "22" {
		return [][2]string{
			{baseURL + "/", fmt.Sprintf("git@%s:", host)},
			{baseURL, fmt.Sprintf("ssh://git@%s", host)},
		}
	}

	return [][2]string{
		{baseURL, fmt.Sprintf("ssh://git@%s:%s", host, port)},
	}
}

func (h *GitAuthHelper) getBaseURL() (*url.URL, error) {
	if u, err := url.Parse(h.config.CloneURL); err == nil && u.Scheme != "" {
		return u, nil
	}

	return url.Parse(h.config.CredentialsURL)
}

func isHTTP(u *url.URL) bool {
	return u != nil && (strings.EqualFold("https", u.Scheme) || strings.EqualFold("http", u.Scheme))
}

func trimmed(u *url.URL) string {
	return strings.TrimRight(u.String(), "/")
}
