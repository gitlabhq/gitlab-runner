package service_helpers

import (
	"errors"
	"os"
	"os/signal"
	"syscall"

	"github.com/kardianos/service"
)

var (
	// ErrNotSupported is returned when specific feature is not supported.
	ErrNotSupported = errors.New("not supported")
)

//nolint:deadcode
//go:generate mockery --name=stopStarter --inpackage
type stopStarter interface {
	Start(service.Service) error
	Stop(service.Service) error
}

type SimpleService struct {
	i service.Interface
	c *service.Config
}

// Run should be called shortly after the program entry point.
// After Interface.Stop has finished running, Run will stop blocking.
// After Run stops blocking, the program must exit shortly after.
func (s *SimpleService) Run() (err error) {
	err = s.i.Start(s)
	if err != nil {
		return err
	}

	sigChan := make(chan os.Signal, 3)
	signal.Notify(sigChan, syscall.SIGTERM, os.Interrupt)

	<-sigChan

	return s.i.Stop(s)
}

// Start signals to the OS service manager the given service should start.
func (s *SimpleService) Start() error {
	return service.ErrNoServiceSystemDetected
}

// Stop signals to the OS service manager the given service should stop.
func (s *SimpleService) Stop() error {
	return ErrNotSupported
}

// Restart signals to the OS service manager the given service should stop then start.
func (s *SimpleService) Restart() error {
	return ErrNotSupported
}

// Install setups up the given service in the OS service manager. This may require
// greater rights. Will return an error if it is already installed.
func (s *SimpleService) Install() error {
	return ErrNotSupported
}

// Uninstall removes the given service from the OS service manager. This may require
// greater rights. Will return an error if the service is not present.
func (s *SimpleService) Uninstall() error {
	return ErrNotSupported
}

// Status returns nil if the given service is running.
// Will return an error if the service is not running or is not present.
func (s *SimpleService) Status() (service.Status, error) {
	return service.StatusUnknown, ErrNotSupported
}

// Logger opens and returns a system logger. If the user program is running
// interactively rather then as a service, the returned logger will write to
// os.Stderr. If errs is non-nil errors will be sent on errs as well as
// returned from Logger's functions.
func (s *SimpleService) Logger(errs chan<- error) (service.Logger, error) {
	return service.ConsoleLogger, nil
}

// SystemLogger opens and returns a system logger. If errs is non-nil errors
// will be sent on errs as well as returned from Logger's functions.
func (s *SimpleService) SystemLogger(errs chan<- error) (service.Logger, error) {
	return nil, ErrNotSupported
}

// String displays the name of the service. The display name if present,
// otherwise the name.
func (s *SimpleService) String() string {
	return "SimpleService"
}

// Platform displays the name of the system that manages the service.
// In most cases this will be the same as service.Platform().
func (s *SimpleService) Platform() string {
	return service.Platform()
}
