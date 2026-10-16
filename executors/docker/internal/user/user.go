package user

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
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

func NewInspect(c docker.Client) Inspect {
	return &defaultInspect{c: c}
}

type defaultInspect struct {
	c docker.Client
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

func retrieveLastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")

	return lines[len(lines)-1]
}

func (i *defaultInspect) executeCommand(ctx context.Context, containerID string, command string) (int, error) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	execResponse, err := i.c.ContainerExecCreate(ctx, containerID, client.ExecCreateOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          strings.Fields(command),
	})
	if err != nil {
		return 0, fmt.Errorf("creating exec for %q on container %q: %w", command, containerID, err)
	}

	response, err := i.c.ContainerExecAttach(ctx, execResponse.ID, client.ExecAttachOptions{})
	if err != nil {
		return 0, fmt.Errorf("attaching exec for %q on container %q: %w", command, containerID, err)
	}
	defer response.Close()

	_, err = stdcopy.StdCopy(
		limitwriter.New(stdout, 1024),
		limitwriter.New(stderr, 1024),
		response.Reader,
	)
	if err != nil {
		return 0, fmt.Errorf("reading output of %q on container %q: %w", command, containerID, err)
	}

	execInspect, err := i.c.ContainerExecInspect(ctx, execResponse.ID)
	if err != nil {
		return 0, fmt.Errorf("inspecting exec for %q on container %q: %w", command, containerID, err)
	}
	if execInspect.Running {
		return 0, fmt.Errorf("exec for %q on container %q is still running", command, containerID)
	}

	stderrContent := strings.TrimSpace(stderr.String())
	if execInspect.ExitCode != 0 {
		return 0, fmt.Errorf("command %q on container %q exited with status %d (stderr: %s)", command, containerID, execInspect.ExitCode, stderrContent)
	}

	stdoutContent := retrieveLastLine(stdout.String())
	if len(stdoutContent) < 1 {
		return 0, fmt.Errorf("%w (stderr: %s)", errIDNoOutput, stderrContent)
	}

	id, err := strconv.Atoi(stdoutContent)
	if err != nil {
		return 0, fmt.Errorf("parsing %q output: %w (stderr: %s)", command, err, stderrContent)
	}

	return id, nil
}
