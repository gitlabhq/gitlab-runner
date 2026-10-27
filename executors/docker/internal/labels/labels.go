package labels

import (
	"fmt"
	"maps"
	"strconv"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const dockerLabelPrefix = "com.gitlab.gitlab-runner"

// AcquisitionKeyLabel scopes release to one environment's containers,
// volumes and network. It shares the prefix of the other environment labels,
// so the prefixed container_labels config cannot produce it.
const AcquisitionKeyLabel = common.RuntimeEnvironmentLabelPrefix + "acquisition-key"

// Labeler is responsible for handling labelling logic for docker entities - networks, containers.
type Labeler interface {
	Labels(otherLabels map[string]string) map[string]string
	LabelKey(name string) string
	// EnvLabels returns a copy of the environment labels, which take
	// precedence over every other label.
	EnvLabels() map[string]string
}

// NewLabeler returns a new instance of a Labeler bound to this build.
// envLabels are stamped onto every entity and take precedence over otherLabels.
func NewLabeler(b *common.Build, envLabels map[string]string) Labeler {
	return &labeler{
		build:     b,
		envLabels: envLabels,
	}
}

type labeler struct {
	build     *common.Build
	envLabels map[string]string
}

// Labels returns a map of label to value to be applied to docker entities.
// Includes a set of defaults. Add additional ones or overwrites in the provided map.
func (l *labeler) Labels(otherLabels map[string]string) map[string]string {
	pipelineID := l.build.GetAllVariables().Value("CI_PIPELINE_ID")
	if l.build.JobInfo.PipelineID > 0 {
		pipelineID = strconv.FormatInt(l.build.JobInfo.PipelineID, 10)
	}

	labels := map[string]string{
		dockerLabelPrefix + ".job.id":            strconv.FormatInt(l.build.ID, 10),
		dockerLabelPrefix + ".job.url":           l.build.JobURL(),
		dockerLabelPrefix + ".job.sha":           l.build.GitInfo.Sha,
		dockerLabelPrefix + ".job.before_sha":    l.build.GitInfo.BeforeSha,
		dockerLabelPrefix + ".job.ref":           l.build.GitInfo.Ref,
		dockerLabelPrefix + ".job.timeout":       l.build.GetBuildTimeout().String(),
		dockerLabelPrefix + ".project.id":        strconv.FormatInt(l.build.JobInfo.ProjectID, 10),
		dockerLabelPrefix + ".project.runner_id": strconv.Itoa(l.build.ProjectRunnerID),
		dockerLabelPrefix + ".pipeline.id":       pipelineID,
		dockerLabelPrefix + ".runner.id":         l.build.Runner.ShortDescription(),
		dockerLabelPrefix + ".runner.local_id":   strconv.Itoa(l.build.RunnerID),
		dockerLabelPrefix + ".runner.system_id":  l.build.Runner.SystemID,
		dockerLabelPrefix + ".managed":           "true",
	}

	for k, v := range otherLabels {
		labels[l.LabelKey(k)] = v
	}

	maps.Copy(labels, l.envLabels)

	return labels
}

func (l *labeler) LabelKey(name string) string {
	return fmt.Sprintf("%s.%s", dockerLabelPrefix, name)
}

func (l *labeler) EnvLabels() map[string]string {
	return maps.Clone(l.envLabels)
}
