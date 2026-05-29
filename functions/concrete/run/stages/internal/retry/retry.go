package retry

import (
	"time"

	"github.com/jpillora/backoff"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
)

// NewBackoff returns a 5s→5min jittered exponential-backoff schedule (factor 1.5).
func NewBackoff() *backoff.Backoff {
	return &backoff.Backoff{
		Min:    5 * time.Second,
		Max:    5 * time.Minute,
		Jitter: true,
		Factor: 1.5,
	}
}

func SleepWithNotice(e *env.Env, d time.Duration) {
	e.Noticef("Retrying in %v", d)
	time.Sleep(d)
}
