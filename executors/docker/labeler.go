package docker

import "gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/labels"

func (e *executor) createLabeler() error {
	e.labeler = labels.NewLabeler(e.Build)
	return nil
}
