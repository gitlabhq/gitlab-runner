package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jpillora/backoff"
	"gitlab.com/gitlab-org/labkit/fields"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

// Boot-verify runs one synthetic job per [[runners]] section through the
// normal build pipeline and gates the Kubernetes /health/ready endpoint
// until every job succeeds. It works with any executor.

const (
	bootVerifyScript = "echo boot verify"
	// bootVerifyStepMargin keeps the job's step timeout below the canary
	// deadline, so a slow step fails with a clear step-timeout error before the
	// context deadline fires.
	bootVerifyStepMargin = 30 * time.Second
)

func (mr *RunCommand) runBootVerify() error {
	var runners []*common.RunnerConfig
	for _, r := range mr.configfile.Config().Runners {
		if bv := r.GetBootVerify(); bv != nil && bv.Enabled {
			runners = append(runners, r)
		}
	}

	logger := mr.log().WithField("phase", "boot-verify")
	if len(runners) == 0 {
		logger.Debug("No runners with experimental.boot_verify enabled; skipping canary")
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel on a stop signal so the canary doesn't run during shutdown.
	abortCh := make(chan os.Signal, 1)
	signal.Notify(abortCh, syscall.SIGQUIT, syscall.SIGTERM, os.Interrupt)
	defer signal.Stop(abortCh)
	go func() {
		select {
		case <-abortCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	logger.WithField("runners", len(runners)).Info("Running boot-verify canary")
	start := time.Now()

	var wg sync.WaitGroup
	errs := make([]error, len(runners))
	for i, runner := range runners {
		wg.Go(func() {
			if err := mr.bootVerifyOneRunner(ctx, runner); err != nil {
				errs[i] = fmt.Errorf("runner %q boot-verify failed: %w", runner.Name, err)
			}
		})
	}
	wg.Wait()

	if err := errors.Join(errs...); err != nil {
		return err
	}
	logger.WithField(fields.DurationS, time.Since(start).Seconds()).Info("Boot-verify canary complete")
	return nil
}

func (mr *RunCommand) bootVerifyOneRunner(ctx context.Context, runner *common.RunnerConfig) error {
	bootVerify := runner.GetBootVerify()

	ctx, cancel := context.WithTimeout(ctx, bootVerify.GetTimeout())
	defer cancel()

	provider := mr.executorProviders.GetByName(runner.Executor)
	if provider == nil {
		return fmt.Errorf("no executor provider for %q", runner.Executor)
	}

	// Acquire returns "no free machines" until the idle pool warms, so retry.
	executorData, err := acquireWithRetry(ctx, provider, runner,
		bootVerify.GetAcquireMinBackoff(), bootVerify.GetAcquireMaxBackoff())
	if err != nil {
		return fmt.Errorf("Acquire: %w", err)
	}
	defer provider.Release(runner, executorData)

	// nil SystemInterrupt: the canary is cancelled via the context, not a signal.
	build, err := common.NewBuild(bootVerifyJobSpec(bootVerify.GetTimeout()), runner, nil, executorData, provider)
	if err != nil {
		return fmt.Errorf("NewBuild: %w", err)
	}
	build.Synthetic = true

	trace := newBootVerifyTrace()
	defer trace.Finish()

	if err := build.Run(ctx, mr.configfile.Config(), trace); err != nil {
		return fmt.Errorf("build.Run: %w\n--- build output ---\n%s", err, trace.dump())
	}
	return nil
}

func acquireWithRetry(ctx context.Context, provider common.ExecutorProvider, runner *common.RunnerConfig, minBackoff, maxBackoff time.Duration) (common.ExecutorData, error) {
	bo := &backoff.Backoff{Min: minBackoff, Max: maxBackoff, Factor: 2, Jitter: true}
	timer := time.NewTimer(bo.Duration())
	defer timer.Stop()

	var lastErr error
	for {
		data, err := provider.Acquire(runner)
		if err == nil {
			return data, nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("ctx expired: %w", errors.Join(ctx.Err(), lastErr))
		case <-timer.C:
		}
		timer.Reset(bo.Duration())
	}
}

const (
	// Synthetic job IDs sit at the top of the int64 range so they stay positive
	// and never collide with a real, much smaller p_ci_builds.id. The low bits
	// are a per-job counter whose wrap is harmless given how few canaries run.
	//
	// Real job IDs are around 1.5e10 today (about 34 bits). The reserved region
	// starts near 2^63, so real IDs can still grow by roughly 29 bits (about
	// 6e8x, to ~9.2e18) before they reach it. Past that point the ID alone no
	// longer distinguishes a synthetic job from a real one.
	//
	// Synthetic IDs are therefore best effort: they keep canary jobs clear of
	// today's real IDs and easy to spot in logs, but they are not a guarantee.
	// Enforce invariants on the Build.Synthetic flag, not on the ID.
	bootVerifyIDCounterBits = 16
	bootVerifyIDCounterMask = int64(1)<<bootVerifyIDCounterBits - 1
	bootVerifyIDBase        = math.MaxInt64 &^ bootVerifyIDCounterMask
)

var bootVerifyJobSeq atomic.Int64

func nextBootVerifyJobID() int64 {
	return bootVerifyIDBase | (bootVerifyJobSeq.Add(1) & bootVerifyIDCounterMask)
}

// bootVerifyJobSpec builds the synthetic job. It sets no image, so the docker
// and kubernetes executors run it on their configured default image.
func bootVerifyJobSpec(timeout time.Duration) spec.Job {
	stepTimeout := timeout - bootVerifyStepMargin
	if stepTimeout <= 0 {
		stepTimeout = timeout
	}

	return spec.Job{
		ID:    nextBootVerifyJobID(),
		Token: "boot-verify",
		JobInfo: spec.JobInfo{
			Name:  "boot-verify",
			Stage: "boot-verify",
		},
		Variables: spec.Variables{
			{Key: "GIT_STRATEGY", Value: "none", Public: true},
			{Key: "GIT_CHECKOUT", Value: "false", Public: true},
		},
		Steps: spec.Steps{
			{
				Name:    spec.StepNameScript,
				Script:  spec.StepScript{bootVerifyScript},
				Timeout: int(stepTimeout.Seconds()),
				When:    spec.StepWhenOnSuccess,
			},
		},
	}
}

// noopTrace discards trace updates, since the synthetic job has no GitLab API
// counterpart. It buffers build output, which is dumped only when a job fails.
type noopTrace struct {
	mu     sync.Mutex
	buf    bytes.Buffer
	cancel context.CancelFunc
	abort  context.CancelFunc
}

// cancel/abort start nil. build.Run installs the real ones.
func newBootVerifyTrace() *noopTrace {
	return &noopTrace{}
}

func (t *noopTrace) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.buf.Write(p)
}

func (t *noopTrace) dump() string {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.buf.String()
}

func (t *noopTrace) Success() error {
	return nil
}

func (t *noopTrace) Fail(err error, _ common.JobFailureData) error {
	return nil
}

func (t *noopTrace) Finish() {
}

func (t *noopTrace) SetCancelFunc(cancelFunc context.CancelFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.cancel = cancelFunc
}

func (t *noopTrace) Cancel() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.cancel == nil {
		return false
	}

	t.cancel()
	return true
}

func (t *noopTrace) SetAbortFunc(abortFunc context.CancelFunc) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.abort = abortFunc
}

func (t *noopTrace) Abort() bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.abort == nil {
		return false
	}

	t.cancel = nil
	t.abort()
	return true
}

func (t *noopTrace) SetFailuresCollector(common.FailuresCollector) {}

func (t *noopTrace) SetSupportedFailureReasonMapper(common.SupportedFailureReasonMapper) {}

func (t *noopTrace) SetDebugModeEnabled(bool) {}

func (t *noopTrace) SetEnvironmentKey(_ string) {}

func (t *noopTrace) IsStdout() bool {
	return false
}
