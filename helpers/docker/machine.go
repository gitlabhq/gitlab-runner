package docker

import (
	"context"
)

type Machine interface {
	Create(ctx context.Context, driver, name string, opts ...string) error
	Provision(ctx context.Context, name string) error
	Remove(ctx context.Context, name string) error
	ForceRemove(ctx context.Context, name string) error
	Stop(ctx context.Context, name string) error
	List() (machines []string, err error)
	Exist(ctx context.Context, name string) bool

	CanConnect(ctx context.Context, name string, skipCache bool) bool
	Credentials(ctx context.Context, name string) (Credentials, error)
}
