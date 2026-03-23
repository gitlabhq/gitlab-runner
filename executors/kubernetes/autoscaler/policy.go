package autoscaler

import (
	"time"

	"gitlab.com/gitlab-org/fleeting/taskscaler/cron"
)

// Policy defines a scaling policy for idle capacity.
type Policy struct {
	// Periods are cron expressions defining when this policy is active.
	// If empty, defaults to "* * * * *" (always active).
	Periods  []string
	Timezone string

	// IdleCount is the target number of idle pods to maintain.
	IdleCount int

	// IdleTime is how long a pod can be idle before being removed.
	IdleTime time.Duration

	// ScaleFactor scales idle capacity based on active jobs.
	// Idle capacity = max(IdleCount, activeJobs * ScaleFactor)
	ScaleFactor float64

	// ScaleFactorLimit caps the ScaleFactor calculation.
	ScaleFactorLimit int

	// parsed schedules for each period
	schedules []cron.Schedule
}

// Parse parses the cron periods for this policy.
func (p *Policy) Parse() error {
	periods := p.Periods
	if len(periods) == 0 {
		periods = []string{"* * * * *"}
	}

	p.schedules = make([]cron.Schedule, 0, len(periods))
	for _, period := range periods {
		sched, err := cron.Parse(period, p.Timezone)
		if err != nil {
			return err
		}
		p.schedules = append(p.schedules, sched)
	}
	return nil
}

// IsActive returns true if this policy is active at the given time.
func (p *Policy) IsActive(t time.Time) bool {
	for _, sched := range p.schedules {
		if sched.Contains(t) {
			return true
		}
	}
	return false
}

// DefaultPolicy is the policy used when no policies are configured or none are active.
var DefaultPolicy = Policy{
	IdleCount: 0,
	IdleTime:  5 * time.Minute,
}

// PolicyList is a list of policies. The last matching policy wins.
type PolicyList []Policy

// ParseAll parses all policies in the list.
func (pl PolicyList) ParseAll() error {
	for i := range pl {
		if err := pl[i].Parse(); err != nil {
			return err
		}
	}
	return nil
}

// Active returns the currently active policy based on the given time.
// If no policy is active, returns the DefaultPolicy.
// If multiple policies are active, the last one wins.
func (pl PolicyList) Active(t time.Time) Policy {
	for i := len(pl) - 1; i >= 0; i-- {
		if pl[i].IsActive(t) {
			return pl[i]
		}
	}
	return DefaultPolicy
}
