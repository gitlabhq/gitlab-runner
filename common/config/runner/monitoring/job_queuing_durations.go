package monitoring

import (
	"fmt"
	"regexp"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/timeperiod"
)

type JobQueuingDurations []*JobQueuingDuration

func (d JobQueuingDurations) Compile() error {
	var err error

	for id, q := range d {
		err = q.Compile()
		if err != nil {
			return fmt.Errorf("entry %d: %w", id, err)
		}
	}

	return nil
}

func (d JobQueuingDurations) GetActiveConfiguration() *JobQueuingDuration {
	for i := len(d) - 1; i >= 0; i-- {
		if d[i].InPeriod() {
			return d[i]
		}
	}

	return nil
}

type JobQueuingDuration struct {
	Periods  []string `toml:"periods" long:"periods"`
	Timezone string   `toml:"timezone" long:"timezone" json:",omitempty"`

	Threshold             time.Duration `toml:"threshold" long:"threshold"`
	JobsRunningForProject string        `toml:"jobs_running_for_project,omitempty" long:"jobs-running-for-project" json:",omitempty"`

	jobsRunningForProjectRx *regexp.Regexp
	timePeriod              *timeperiod.TimePeriod

	timer func() time.Time
}

func (d *JobQueuingDuration) Compile() error {
	var err error

	if d.timer == nil {
		d.timer = time.Now
	}

	d.timePeriod, err = timeperiod.TimePeriodsWithTimer(d.Periods, d.Timezone, d.timer)
	if err != nil {
		return fmt.Errorf("periods: %w", err)
	}

	d.jobsRunningForProjectRx, err = regexp.Compile(d.JobsRunningForProject)
	if err != nil {
		return fmt.Errorf("jobs_running_for_project: %w", err)
	}

	return nil
}

func (d *JobQueuingDuration) InPeriod() bool {
	return d.timePeriod.InPeriod()
}

func (d *JobQueuingDuration) JobsRunningForProjectMatched(s string) bool {
	// If regexp was invalid or not configured at all, we ignore this part of the check
	if d.jobsRunningForProjectRx == nil {
		return true
	}

	return d.jobsRunningForProjectRx.MatchString(s)
}
