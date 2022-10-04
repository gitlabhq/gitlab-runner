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

//go:generate mockery --name=Inspect --inpackage
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
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	streams := exec.IOStreams{
		Stdin:  strings.NewReader(command),
		Stdout: limitwriter.New(stdout, 1024),
		Stderr: limitwriter.New(stderr, 1024),
	}

	err := i.exec.Exec(ctx, containerID, streams)
	if err != nil {
		return 0, fmt.Errorf("executing %q on container %q: %w", command, containerID, err)
	}

	stdoutContent := strings.TrimSpace(stdout.String())
	stderrContent := strings.TrimSpace(stderr.String())
	if len(stdoutContent) < 1 {
		return 0, fmt.Errorf("%w (stderr: %s)", errIDNoOutput, stderrContent)
	}

	id, err := strconv.Atoi(stdoutContent)
	if err != nil {
		return 0, fmt.Errorf("parsing %q output: %w (stderr: %s)", command, err, stderrContent)
	}

	return id, nil
}
