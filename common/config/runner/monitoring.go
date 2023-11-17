package runner

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/common/config/runner/monitoring"
)

type Monitoring struct {
	JobQueuingDurations monitoring.JobQueuingDurations `toml:"job_queuing_durations,omitempty" long:"job-queuing-duration" json:",omitempty"`
}

func (m *Monitoring) Compile() error {
	var err error

	if m.JobQueuingDurations != nil {
		err = m.JobQueuingDurations.Compile()
		if err != nil {
			return fmt.Errorf("compiling job_queuing_durations: %w", err)
		}
	}

	return nil
}
