package extractor

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strconv"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/ci-cd/runner-tools/gitlab-changelog/git"
)

const (
	mrLinePattern = "^\\s*See merge request ([a-zA-Z0-9-_/]+)?!([0-9]+)$"
)

var (
	mrRegex = regexp.MustCompile(mrLinePattern)
)

type Extractor interface {
	ExtractMRIIDs(startingPoint string, matcher string) ([]int, error)
}

type defaultExtractor struct {
	gitReader git.Git
}

func New(gitReader git.Git) Extractor {
	return &defaultExtractor{
		gitReader: gitReader,
	}
}

func (e *defaultExtractor) ExtractMRIIDs(startingPoint string, matcher string) ([]int, error) {
	sp, err := e.getStartingPoint(startingPoint, matcher)
	if err != nil {
		return nil, fmt.Errorf("error while establihing starting point: %w", err)
	}

	logOutput, err := e.getLogOutput(sp)
	if err != nil {
		return nil, fmt.Errorf("error while reading git log output: %w", err)
	}

	mrIIDs := make([]int, 0)

	reader := bytes.NewReader(logOutput)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if !mrRegex.MatchString(line) {
			continue
		}

		find := mrRegex.FindStringSubmatch(line)

		id, err := strconv.Atoi(find[2])
		if err != nil {
			return nil, fmt.Errorf("couldn't parse ID %q: %w", find[2], err)
		}

		mrIIDs = append(mrIIDs, id)
	}

	return mrIIDs, nil
}

func (e *defaultExtractor) getStartingPoint(startingPoint string, matcher string) (string, error) {
	if startingPoint != "" {
		logrus.WithField("starting-point", startingPoint).
			Info("Using provided starting point")

		return startingPoint, nil
	}

	logrus.WithField("starting-point-matcher", matcher).
		Debug("Autodiscovering starting point")

	startingPoint, err := e.gitReader.Describe(&git.DescribeOpts{
		Abbrev: 0,
		Match:  matcher,
		Tags:   true,
	})

	if err != nil {
		return "", err
	}

	logrus.WithField("starting-point", startingPoint).
		Info("Starting point autodiscovered")

	return startingPoint, nil
}

func (e *defaultExtractor) getLogOutput(sp string) ([]byte, error) {
	logOutput, err := e.gitReader.Log(fmt.Sprintf("%s..", sp), &git.LogOpts{
		FirstParent: true,
	})

	if err != nil {
		return nil, err
	}

	return logOutput, nil
}
