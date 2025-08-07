//go:build mage

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/magefile/mage/mg"

	"gitlab.com/gitlab-org/gitlab-runner/magefiles/hosted_runners"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/mageutils"
)

type HostedRunners mg.Namespace

// Bridge is a function that feeds Hosted Runners maintainers with information
// about the recently released GitLab Runner pre/stable artifacts
func (HostedRunners) Bridge(ctx context.Context) error {
	logLevel := slog.LevelInfo
	if mageutils.Env("DEBUG") != "" {
		logLevel = slog.LevelDebug
	}

	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))

	gitlabToken := mageutils.Env("GITLAB_TOKEN")
	gitlabURL := mageutils.EnvOrDefault("GITLAB_URL", "https://gitlab.com")
	projectID := mageutils.EnvOrDefault("GITLAB_PROJECT_ID", "250833") // https://gitlab.com/gitlab-org/gitlab-runner/
	wikiPageSlug := mageutils.EnvOrDefault("GITLAB_WIKI_PAGE_SLUG", "Released-runner-versions")

	client, err := hosted_runners.NewGitLabWikiClient(log, gitlabURL, projectID, wikiPageSlug, gitlabToken)
	if err != nil {
		return fmt.Errorf("creating gitlab wiki client: %w", err)
	}

	return hosted_runners.Bridge(ctx, log, client)
}
