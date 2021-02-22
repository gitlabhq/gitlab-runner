package user

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/exec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/limitwriter"
)

const (
	commandIDU = "id -u"
	commandIDG = "id -g"
)

var errIDNoOutput = errors.New("id command returned no output on stdout")

type Inspect interface {
	IsRoot(ctx context.Context, imageID string) (bool, error)
	UID(ctx context.Context, containerID string) (int, error)
	GID(ctx context.Context, containerID string) (int, error)
}

func NewInspect(c docker.Client, exec exec.Docker) Inspect {
	return &defaultInspect{
		c:    c,
		exec: exec,
	}
}

type defaultInspect struct {
	c    docker.Client
	exec exec.Docker
}

func (i *defaultInspect) IsRoot(ctx context.Context, imageID string) (bool, error) {
	img, _, err := i.c.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		return true, fmt.Errorf("inspecting container %q image: %w", imageID, err)
	}

	if img.Config == nil || img.Config.User == "" || img.Config.User == "root" {
		return true, nil
	}

	return false, nil
}

func (i *defaultInspect) UID(ctx context.Context, containerID string) (int, error) {
	return i.executeCommand(ctx, containerID, commandIDU)
}

func (i *defaultInspect) GID(ctx context.Context, containerID string) (int, error) {
	return i.executeCommand(ctx, containerID, commandIDG)
}

func (i *defaultInspect) executeCommand(ctx context.Context, containerID string, command string) (int, error) {
	stdOut := new(bytes.Buffer)
	stdErr := new(bytes.Buffer)
	streams := exec.IOStreams{
		Input: strings.NewReader(command),
		// limit how much data we read from the container log to
		// avoid memory exhaustion
		Out: limitwriter.New(stdOut, 1024),
		Err: limitwriter.New(stdErr, 1024),
	}

	err := i.exec.Exec(ctx, containerID, streams)
	if err != nil {
		return 0, fmt.Errorf("executing %q on container %q: %w", command, containerID, err)
	}

	out := strings.TrimSpace(stdOut.String())
	errOut := strings.TrimSpace(stdErr.String())
	if len(out) < 1 {
		return 0, fmt.Errorf("%w (stderr: %s)", errIDNoOutput, errOut)
	}

	id, err := strconv.Atoi(out)
	if err != nil {
		return 0, fmt.Errorf("parsing %q output: %w (stderr: %s)", command, err, errOut)
	}

	return id, nil
}
