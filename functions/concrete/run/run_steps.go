package run

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"strconv"

	"go.yaml.in/yaml/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/innerstream"
	"gitlab.com/gitlab-org/step-runner/pkg/api"
	"gitlab.com/gitlab-org/step-runner/pkg/api/client"
	"gitlab.com/gitlab-org/step-runner/pkg/api/client/extended"
	"gitlab.com/gitlab-org/step-runner/schema/v1"
)

// EnvSocketPath is the env var the hosting step-runner advertises its socket
// path under, so in-process builtins (this one) can dial back in to dispatch
// nested jobs. The serving side (commands/steps.Serve) reads this constant
// when setting the env var, so the two sides are kept in sync by the compiler.
const EnvSocketPath = "STEP_RUNNER_SOCKET"

// stepsPreamble is required by step-runner's YAML parser: an empty front
// matter document followed by the actual step list.
const stepsPreamble = "{}\n---\n"

// runUserSteps dispatches the user's `run:` keyword as a nested job through
// the step-runner that is hosting this concrete step. Concrete dials back
// into the same socket as a separate tenant ID derived from the outer job
// ID, so a failure in the user's steps is captured as an error here and
// the outer concrete step's surrounding stages (cache/artifacts/cleanup)
// continue to run.
func (r *Runner) runUserSteps(ctx context.Context, steps []schema.Step) error {
	stepsYAML, err := buildStepsYAML(steps)
	if err != nil {
		return fmt.Errorf("marshalling user steps: %w", err)
	}

	tenantID := strconv.FormatInt(r.config.ID, 10) + "-run"

	// Merge Env and GitLabEnv with the same precedence as env.Command, so
	// the nested job sees $GITLAB_ENV (and any KEY=VALUE lines written by
	// earlier stages) just like a local stage would.
	mergedEnv := maps.Clone(r.env.Env)
	maps.Copy(mergedEnv, r.env.GitLabEnv)

	req := &client.RunRequest{
		Id:       tenantID,
		WorkDir:  r.env.WorkingDir,
		BuildDir: r.env.WorkingDir,
		Env:      mergedEnv,
		Steps:    stepsYAML,
	}
	if r.config.ScriptTimeout > 0 {
		t := r.config.ScriptTimeout
		req.Timeout = &t
	}

	dialer := unixSocketDialer(socketPath())
	cli, err := extended.New(dialer)
	if err != nil {
		return fmt.Errorf("dialing step-runner: %w", err)
	}
	//nolint:errcheck
	defer cli.CloseConn()

	splitter := innerstream.New(r.env.Stdout, r.env.Stderr)
	out := &extended.FollowOutput{Logs: splitter}

	status, err := cli.RunAndFollow(ctx, req, out)
	// Flush any line whose continuation marker we never saw.
	flushErr := splitter.Flush()

	return interpretRunResult(status, err, flushErr)
}

// interpretRunResult collapses the (status, runErr, flushErr) triple from a
// nested step-runner dispatch into the single error contract callers expect.
//
// Precedence: a runErr from RunAndFollow always wins over a flushErr — when
// dispatch already failed we don't want a flush error to clobber the real
// cause. With no runErr, a flushErr is surfaced. With no transport errors at
// all, the Status drives the result:
//
//   - StateSuccess         → nil
//   - StateCancelled       → wraps ErrJobCanceled with %w so callers can
//     errors.Is(err, ErrJobCanceled). Removing the %w would silently break
//     every upstream caller's identity check.
//   - any other state      → returns the server-provided message, or
//     State.String() when the message is empty.
func interpretRunResult(status client.Status, runErr, flushErr error) error {
	if runErr == nil {
		runErr = flushErr
	}
	if runErr != nil {
		return fmt.Errorf("running user steps via step-runner: %w", runErr)
	}

	switch status.State {
	case client.StateSuccess:
		return nil
	case client.StateCancelled:
		return fmt.Errorf("%w: %s", ErrJobCanceled, status.Message)
	default:
		msg := status.Message
		if msg == "" {
			msg = status.State.String()
		}
		return errors.New(msg)
	}
}

func buildStepsYAML(steps []schema.Step) (string, error) {
	wrapper := &schema.Step{Run: steps}
	out, err := yaml.Marshal(wrapper)
	if err != nil {
		return "", err
	}
	return stepsPreamble + string(out), nil
}

func socketPath() string {
	if p, ok := os.LookupEnv(EnvSocketPath); ok && p != "" {
		return p
	}
	return api.DefaultSocketPath()
}

type unixSocketDialer string

func (d unixSocketDialer) Dial() (*grpc.ClientConn, error) {
	return grpc.NewClient(
		"unix:"+string(d),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
}
