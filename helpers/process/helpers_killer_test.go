// Helper functions that are shared between unit tests and integration tests

package process

// Killer is used to the killer interface to the integration tests package
type Killer interface {
	killer
}

// NewKillerForTest is used to expose a new killer to the integration tests package
func NewKillerForTest(logger Logger, cmd Commander) Killer {
	return newKiller(logger, cmd)
}
