package labels

import (
	"fmt"
	"strconv"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const dockerLabelPrefix = "com.gitlab.gitlab-runner"

// Labeler is responsible for handling labelling logic for docker entities - networks, containers.
type Labeler interface {
	Labels(otherLabels map[string]string) map[string]string
}

// NewLabeler returns a new instance of a Labeler bound to this build.
func NewLabeler(b *common.Build) Labeler {
	return &labeler{
		build: b,
	}
}

type labeler struct {
	build *common.Build
}

// Labels returns a map of label to value to be applied to docker entities.
// Includes a set of defaults. Add additional ones or overwrites in the provided map.
func (l *labeler) Labels(otherLabels map[string]string) map[string]string {
	labels := map[string]string{
		dockerLabelPrefix + ".job.id":          strconv.FormatInt(l.build.ID, 10),
		dockerLabelPrefix + ".job.url":         l.build.JobURL(),
		dockerLabelPrefix + ".job.sha":         l.build.GitInfo.Sha,
		dockerLabelPrefix + ".job.before_sha":  l.build.GitInfo.BeforeSha,
		dockerLabelPrefix + ".job.ref":         l.build.GitInfo.Ref,
		dockerLabelPrefix + ".project.id":      strconv.FormatInt(l.build.JobInfo.ProjectID, 10),
		dockerLabelPrefix + ".pipeline.id":     l.build.GetAllVariables().Value("CI_PIPELINE_ID"),
		dockerLabelPrefix + ".runner.id":       l.build.Runner.ShortDescription(),
		dockerLabelPrefix + ".runner.local_id": strconv.Itoa(l.build.RunnerID),
		dockerLabelPrefix + ".managed":         "true",
	}

	for k, v := range otherLabels {
		labels[fmt.Sprintf("%s.%s", dockerLabelPrefix, k)] = v
	}

	return labels
}
