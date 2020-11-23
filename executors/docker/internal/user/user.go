package user

import (
	"bytes"
	"context"
	"fmt"
	"strconv"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/exec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

const (
	commandIDU = "id -u"
	commandIDG = "id -g"
)

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
	input := bytes.NewBufferString(command)
	output := new(bytes.Buffer)

	err := i.exec.Exec(ctx, containerID, input, output)
	if err != nil {
		return 0, fmt.Errorf("executing %q on container %q: %w", command, containerID, err)
	}

	id, err := strconv.Atoi(strings.TrimSpace(output.String()))
	if err != nil {
		return 0, fmt.Errorf("parsing %q output: %w", command, err)
	}

	return id, nil
}
