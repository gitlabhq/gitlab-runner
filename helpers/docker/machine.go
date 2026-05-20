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

	// Inspect returns selected fields from the per-machine state file
	// docker-machine writes during Create. Used to label autoscaling
	// metrics with what was actually provisioned (zone the regional
	// MIG placed in, machine type Flex MIG selected) rather than
	// operator-supplied static values that may drift from reality.
	// Best-effort: missing or malformed state returns empty info + err.
	Inspect(name string) (MachineInfo, error)
}

// MachineInfo carries the subset of docker-machine driver state read
// off disk for metric labels. Driver-specific fields (Zone, MachineType,
// Project) are only populated when DriverName has an explicit schema —
// currently just "google". Other drivers get DriverName only; we'd
// rather emit empty labels than surface uninterpretable values.
type MachineInfo struct {
	DriverName  string
	Zone        string
	MachineType string
	Project     string
}
