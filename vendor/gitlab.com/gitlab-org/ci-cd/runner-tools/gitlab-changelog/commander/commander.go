package commander

import (
	"context"
	"os/exec"
)

type Factory func(ctx context.Context, command string, args ...string) Commander

type Commander interface {
	Output() ([]byte, error)
	String() string
}

func New(ctx context.Context, command string, args ...string) Commander {
	return exec.CommandContext(ctx, command, args...)
}
