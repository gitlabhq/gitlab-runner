package common

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

const (
	// DefaultRuntimeEnvironmentTTL applies when runtime_environment_ttl is
	// unset or zero. A zero TTL is never used as-is: it would make every
	// suspended environment immediately eligible for cleanup.
	DefaultRuntimeEnvironmentTTL = 7 * 24 * time.Hour

	// MaxRuntimeEnvironmentTTL bounds how long a suspended environment can
	// hold paid resources. The runner refuses to load a larger value.
	MaxRuntimeEnvironmentTTL = 30 * 24 * time.Hour

	// RuntimeEnvironmentSweepGrace is how long past its expiry the cleanup
	// loop waits before removing an environment, so a resume that passed its
	// expiry check has time to take hold of the environment before it is
	// removed.
	RuntimeEnvironmentSweepGrace = 10 * time.Minute

	// RuntimeEnvironmentLabelPrefix namespaces the labels the runner puts on
	// resources that make up a suspended environment.
	RuntimeEnvironmentLabelPrefix = "environment.runner.gitlab.com/"

	// RuntimeEnvironmentRunnerIDLabel carries the ID of the runner that
	// created the environment. Every manager of a registration shares it, so
	// any of them can clean the environment up.
	RuntimeEnvironmentRunnerIDLabel = RuntimeEnvironmentLabelPrefix + "runner-id"

	// RuntimeEnvironmentExpiresAtLabel carries the environment's expiry, as
	// formatted by FormatRuntimeEnvironmentExpiry. It is stamped once, when
	// the environment's first job creates it, so every manager checks and
	// sweeps against the same expiry whatever TTL it is configured with.
	RuntimeEnvironmentExpiresAtLabel = RuntimeEnvironmentLabelPrefix + "expires-at"
)

var (
	// ErrRuntimeEnvironmentUnavailable marks a resume that cannot proceed
	// because the environment is gone: expired, released or lost.
	ErrRuntimeEnvironmentUnavailable = errors.New("runtime environment unavailable")

	// ErrRuntimeEnvironmentExpired marks an environment past its expiry.
	ErrRuntimeEnvironmentExpired = errors.New("runtime environment expired")
)

// GetRuntimeEnvironmentTTL returns the configured TTL for suspended
// environments, or DefaultRuntimeEnvironmentTTL when unset or not positive.
// It is only used to stamp the expiry of new environments; see
// RuntimeEnvironmentExpiresAt.
func (r *RunnerSettings) GetRuntimeEnvironmentTTL() time.Duration {
	if r.RuntimeEnvironmentTTL == nil || *r.RuntimeEnvironmentTTL <= 0 {
		return DefaultRuntimeEnvironmentTTL
	}
	return *r.RuntimeEnvironmentTTL
}

func (c *RunnerConfig) validateRuntimeEnvironmentTTL() error {
	if c.RuntimeEnvironmentTTL == nil {
		return nil
	}

	ttl := *c.RuntimeEnvironmentTTL
	if ttl < 0 {
		return fmt.Errorf("runtime_environment_ttl %s must not be negative", ttl)
	}
	if ttl > MaxRuntimeEnvironmentTTL {
		return fmt.Errorf("runtime_environment_ttl %s exceeds the maximum of %s", ttl, MaxRuntimeEnvironmentTTL)
	}
	return nil
}

// RuntimeEnvironmentExpiresAt returns the expiry to stamp on an environment
// created at createdAt. The expiry is whole seconds, the precision of the
// stamp.
func (r *RunnerSettings) RuntimeEnvironmentExpiresAt(createdAt time.Time) time.Time {
	return createdAt.Add(r.GetRuntimeEnvironmentTTL()).Truncate(time.Second).UTC()
}

// FormatRuntimeEnvironmentExpiry encodes t as decimal Unix seconds, a valid
// Kubernetes and Docker label value.
func FormatRuntimeEnvironmentExpiry(t time.Time) string {
	return strconv.FormatInt(t.Unix(), 10)
}

// ParseRuntimeEnvironmentExpiry decodes a stamp produced by
// FormatRuntimeEnvironmentExpiry.
func ParseRuntimeEnvironmentExpiry(s string) (time.Time, error) {
	sec, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse environment expiry stamp %q: %w", s, err)
	}
	if sec <= 0 {
		return time.Time{}, fmt.Errorf("parse environment expiry stamp %q: must be positive", s)
	}
	return time.Unix(sec, 0).UTC(), nil
}

// CheckRuntimeEnvironmentExpiry refuses a resume onto an environment past its
// expiry. The returned error fails the build as RuntimeEnvironmentUnavailable.
func CheckRuntimeEnvironmentExpiry(expiresAt, now time.Time) error {
	if !now.After(expiresAt) {
		return nil
	}
	return NewRuntimeEnvironmentUnavailableError(fmt.Errorf(
		"%w: expired %s ago", ErrRuntimeEnvironmentExpired, now.Sub(expiresAt).Round(time.Second)))
}

// RuntimeEnvironmentSweepable reports whether the cleanup loop may remove a
// suspended environment that expires at expiresAt: its expiry plus
// RuntimeEnvironmentSweepGrace has passed.
func RuntimeEnvironmentSweepable(expiresAt, now time.Time) bool {
	return now.After(expiresAt.Add(RuntimeEnvironmentSweepGrace))
}

// NewRuntimeEnvironmentUnavailableError wraps err so the build fails with the
// RuntimeEnvironmentUnavailable reason. The BuildError also stops prepare
// from being retried: a missing or expired environment does not come back.
func NewRuntimeEnvironmentUnavailableError(err error) error {
	return &BuildError{
		Inner:         fmt.Errorf("%w: %w", ErrRuntimeEnvironmentUnavailable, err),
		FailureReason: RuntimeEnvironmentUnavailable,
	}
}
