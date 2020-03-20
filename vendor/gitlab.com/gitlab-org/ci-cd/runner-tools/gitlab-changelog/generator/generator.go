package generator

import (
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog/config"
	"gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog/extractor"
	"gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog/git"
	"gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog/gitlab"
	"gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog/scope"
	"gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog/writer"
)

const (
	RFC3339Date = "2006-01-02"

	privateTokenEnv = "GITLAB_PRIVATE_TOKEN"
)

type Opts struct {
	ProjectID string
	Release   string

	StartingPoint        string
	StartingPointMatcher string

	ChangelogFile string

	ConfigFile string
}

type Generator interface {
	Generate() error
}

func New(opts Opts) Generator {
	return &defaultGenerator{
		opts: opts,
	}
}

type defaultGenerator struct {
	opts        Opts
	scopeConfig config.Configuration
}

func (g *defaultGenerator) Generate() error {
	logrus.WithField("release", g.opts.Release).
		Info("Generating changelog entries")

	err := g.setupConfiguration()
	if err != nil {
		return fmt.Errorf("error while loading scope configuration file: %w", err)
	}

	mrIIDs, err := g.getMRIIDs()
	if err != nil {
		return fmt.Errorf("error while extracgint MR IIDs from git history: %w", err)
	}

	mergeRequests, err := g.getMergeRequests(g.opts.ProjectID, mrIIDs)
	if err != nil {
		return fmt.Errorf("error while generating merge requests list: %w", err)
	}

	mergeRequestObjects := g.convertMergeRequestsToScopeObjets(mergeRequests)
	entries, err := g.buildEntriesMap(mergeRequestObjects)
	if err != nil {
		return fmt.Errorf("error while building entries map: %w", err)
	}

	w := g.getWriter()
	err = g.writeEntries(w, entries)
	if err != nil {
		return fmt.Errorf("error while writing changelog entries: %w", err)
	}

	logrus.WithField("release", g.opts.Release).
		Info("Changelog entries generated")

	return nil
}

func (g *defaultGenerator) setupConfiguration() error {
	if g.opts.ConfigFile == "" {
		g.scopeConfig = config.DefaultConfig()
		return nil
	}

	cnf, err := config.LoadConfig(g.opts.ConfigFile)
	if err != nil {
		return err
	}

	g.scopeConfig = cnf

	return nil
}

func (g *defaultGenerator) getMRIIDs() ([]int, error) {
	extr := extractor.New(git.New())
	mrIIDs, err := extr.ExtractMRIIDs(g.opts.StartingPoint, g.opts.StartingPointMatcher)
	if err != nil {
		return nil, err
	}

	logrus.WithField("count", len(mrIIDs)).
		Info("Found merge requests commits")

	return mrIIDs, nil
}

func (g *defaultGenerator) getMergeRequests(projectID string, mrIIDs []int) ([]gitlab.MergeRequest, error) {
	if len(mrIIDs) < 1 {
		return []gitlab.MergeRequest{}, nil
	}

	gitlabClient, err := gitlab.NewClient(os.Getenv(privateTokenEnv), projectID)
	if err != nil {
		return nil, fmt.Errorf("couldn't initialize GitLab client: %w", err)
	}

	mergeRequests, err := gitlabClient.ListMergeRequests(mrIIDs, 25)
	if err != nil {
		return nil, err
	}

	logrus.WithField("count", len(mergeRequests)).
		Info("Found merge requests")

	return mergeRequests, nil
}

func (g *defaultGenerator) convertMergeRequestsToScopeObjets(mergeRequests []gitlab.MergeRequest) []scope.Object {
	objects := make([]scope.Object, 0)
	for _, mr := range mergeRequests {
		objects = append(objects, NewMRScopeObjectAdapter(mr))
	}

	return objects
}

func (g *defaultGenerator) buildEntriesMap(objects []scope.Object) (scope.EntriesMap, error) {
	entriesBuilder := scope.NewEntriesMapBuilder(g.scopeConfig)
	entries, err := entriesBuilder.Build(objects)
	if err != nil {
		return nil, err
	}

	return entries, nil
}

func (g *defaultGenerator) getWriter() writer.Writer {
	if g.opts.ChangelogFile == "" {
		return writer.NewProxy(os.Stdout)
	}

	logrus.WithField("file", g.opts.ChangelogFile).
		Info("Writing output to file")

	return writer.NewFilePrepender(g.opts.ChangelogFile)
}

func (g *defaultGenerator) writeEntries(w writer.Writer, entries scope.EntriesMap) error {
	_, err := fmt.Fprintf(w, "## %s (%s)\n\n", g.opts.Release, time.Now().Format(RFC3339Date))
	if err != nil {
		return err
	}

	err = entries.ForEach(func(entry scope.Entries) error {
		_, err = fmt.Fprintf(w, "### %s\n\n", entry.ScopeName)
		if err != nil {
			return err
		}
		for _, scopeEntry := range entry.Entries {
			_, err = fmt.Fprintf(w, "- %s\n", scopeEntry)
			if err != nil {
				return err
			}
		}
		_, err = fmt.Fprintln(w)
		return err
	})

	if err != nil {
		return err
	}

	return w.Flush()
}
