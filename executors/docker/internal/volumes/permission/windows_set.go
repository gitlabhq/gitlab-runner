//go:build windows

package permission

import (
	"context"
)

type dockerWindowsSetter struct {
}

// NewDockerWindowsSetter is a noop permissions for Windows, this will be
// implemented in https://gitlab.com/gitlab-org/gitlab-runner/-/issues/25480.
func NewDockerWindowsSetter() Setter {
	return &dockerWindowsSetter{}
}

// Set noop
func (d dockerWindowsSetter) Set(_ context.Context, _ string, _ map[string]string) error {
	return nil
}
