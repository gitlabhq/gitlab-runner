package pulp

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"slices"
	"time"

	"github.com/samber/lo"
	"go.yaml.in/yaml/v3"
)

const (
	pulpReposURL = "https://gitlab.com/api/v4/projects/75880111/repository/files/repos%2Frunner.yaml/raw"
	rpm          = "rpm"
	deb          = "deb"
)

type (
	pulpRepository struct {
		Path string `yaml:"path"`
		EOL  bool   `yaml:"eol"`
	}

	pulpRepositories struct {
		Deb []pulpRepository `yaml:"deb"`
		Rpm []pulpRepository `yaml:"rpm"`
	}

	pulpRelease struct {
		Repositories pulpRepositories `yaml:"repositories"`
	}

	pulpRepos struct {
		Stable   pulpRelease `yaml:"gitlab-runner"`
		Unstable pulpRelease `yaml:"unstable"`
	}

	pulpConfig struct {
		Runner pulpRepos `yaml:"runner"`
	}
)

var (
	dists    = []string{rpm, deb}
	branches = []string{"stable", "unstable"}
)

var tokenHeaders = map[string]string{
	"CI_JOB_TOKEN":  "JOB-TOKEN",
	"PRIVATE_TOKEN": "PRIVATE-TOKEN",
}

func Releases(dist, branch string) ([]string, error) {
	if err := validateInputs(dist, branch); err != nil {
		return nil, err
	}
	return releases(dist, branch)
}

func releases(dist, branch string) ([]string, error) {
	tokenType, tokenValue, err := getToken()
	if err != nil {
		return nil, err
	}

	config, err := getPulpRunnerConfig(tokenType, tokenValue)
	if err != nil {
		return nil, err
	}

	return releasesForDistBranch(dist, branch, config), nil
}

func validateInputs(pkgType, branch string) error {
	if !slices.Contains(dists, pkgType) {
		return fmt.Errorf("unsupported package type %q", pkgType)
	}

	if !slices.Contains(branches, branch) {
		return fmt.Errorf("unsupported branch %q", branch)
	}
	return nil
}

func firstEnv(envs ...string) (string, string, bool) {
	for _, env := range envs {
		if val, ok := os.LookupEnv(env); ok {
			return env, val, true
		}
	}
	return "", "", false
}

func getToken() (string, string, error) {
	tokenType, tokenValue, ok := firstEnv("CI_JOB_TOKEN", "PRIVATE_TOKEN")
	if !ok {
		return "", "", errors.New("required 'CI_JOB_TOKEN' or 'PRIVATE_TOKEN' variable missing")
	}

	if tokenValue == "" {
		return "", "", fmt.Errorf("%s cannot be empty", tokenType)
	}

	// Translate token type to required headers
	tokenType = tokenHeaders[tokenType]

	return tokenType, tokenValue, nil
}

func releasesForDistBranch(dist, branch string, config *pulpConfig) []string {
	var release pulpRelease
	switch branch {
	case "stable":
		release = config.Runner.Stable
	case "unstable":
		release = config.Runner.Unstable
	}

	var repos []pulpRepository
	switch dist {
	case deb:
		repos = release.Repositories.Deb
	case rpm:
		repos = release.Repositories.Rpm
	}

	// exclude releases that have reached EOL.
	repos = lo.Filter(repos, func(repo pulpRepository, _ int) bool { return !repo.EOL })

	return lo.Map(repos, func(repo pulpRepository, _ int) string {
		return repo.Path
	})
}

// The full Pulp runner repo config file can be enjoyed at
// https://gitlab.com/gitlab-org/build/pulp-repository-automation/-/blob/main/repos/runner.yaml?ref_type=heads
func getPulpRunnerConfig(tokenType, tokenValue string) (*pulpConfig, error) {
	req, err := http.NewRequest("GET", pulpReposURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for url %q: %w", pulpReposURL, err)
	}
	req.Header.Add(tokenType, tokenValue)

	client := http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get url %q: %w", pulpReposURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got unexpected response status code: %s", resp.Status)
	}

	result := pulpConfig{}
	if err := yaml.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response body: %w", err)
	}

	return &result, nil
}
