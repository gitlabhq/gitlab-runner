package autoscaler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/fleeting/fleeting/connector"
	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	nestingapi "gitlab.com/gitlab-org/fleeting/nesting/api"
	"gitlab.com/gitlab-org/fleeting/nesting/hypervisor"
	"gitlab.com/gitlab-org/fleeting/taskscaler"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

var _ executors.Environment = (*acquisitionRef)(nil)

// AcquisitionRef is an exported alias for acquisitionRef
type AcquisitionRef = acquisitionRef

var (
	errRefAcqNotSet = errors.New("ref.acq is not set")

	errNoNestingImageSpecified = errors.New("no nesting VM image specified to run the job in")
)

type acquisitionRef struct {
	key string
	acq taskscaler.Acquisition

	// replace is set when the current acquisition's instance was found
	// unreachable; the next Prepare attempt must call taskscaler.Replace so
	// the runner gets a different instance without giving up its capacity claim.
	replace bool

	mapJobImageToVMImage bool

	// test hooks
	dialAcquisitionInstance connector.DialFn
	dialTunnel              connector.DialFn

	connectNestingFn func(
		host string,
		logger buildlogger.Logger,
		fleetingDialer connector.Client,
	) (nestingapi.Client, io.Closer, error)
}

func newAcquisitionRef(key string, mapJobImageToVMImage bool) *acquisitionRef {
	return &acquisitionRef{
		key:                     key,
		mapJobImageToVMImage:    mapJobImageToVMImage,
		dialAcquisitionInstance: connector.Dial,
		dialTunnel:              connector.Dial,
	}
}

func (ref *acquisitionRef) AcquisitionKey() string {
	return ref.key
}

// AcquisitionSlot returns the slot number for this acquisition
func (ref *acquisitionRef) AcquisitionSlot() int {
	if ref.acq != nil {
		return ref.acq.Slot()
	}
	return -1
}

// addSlotCgroupEnvironmentVariable adds GITLAB_RUNNER_SLOT_CGROUP environment variable if slot cgroups are enabled
func (ref *acquisitionRef) addSlotCgroupEnvironmentVariable(options *common.ExecutorPrepareOptions) {
	if cgroupPath := options.Config.GetSlotCgroupPath(ref); cgroupPath != "" {
		options.Build.Variables = append(options.Build.Variables, spec.Variable{
			Key:   "GITLAB_RUNNER_SLOT_CGROUP",
			Value: cgroupPath,
		})
	}
}

func (ref *acquisitionRef) Prepare(
	ctx context.Context,
	logger buildlogger.Logger,
	options common.ExecutorPrepareOptions,
) (executors.Client, error) {
	if ref.acq == nil {
		return nil, errRefAcqNotSet
	}

	ref.addSlotCgroupEnvironmentVariable(&options)

	dialCtx, cancel := ref.acq.WithContext(ctx)
	defer cancel()

	info, err := ref.acq.InstanceConnectInfo(dialCtx)
	if cause := context.Cause(dialCtx); cause != nil {
		// Benign race: if the job context is cancelled right after taskscaler
		// cancelled dialCtx, ctx.Err() may still be nil here and we return
		// instanceUnavailableError instead of a BuildError. The retry loop
		// sees the cancelled job context on its next attempt, so at most one
		// extra attempt happens.
		if ctx.Err() == nil {
			// dialCtx was cancelled by taskscaler (instance removed), not by the job
			return nil, &instanceUnavailableError{err: cause}
		}
		return nil, buildErrorFromContextCause(context.Cause(ctx))
	}
	if err != nil {
		ref.acq.HealthFailure()
		return nil, &instanceUnavailableError{err: fmt.Errorf("getting instance connect info: %w", err)}
	}

	useExternalAddr := true
	if options.Config != nil && options.Config.Autoscaler != nil {
		useExternalAddr = options.Config.Autoscaler.ConnectorConfig.UseExternalAddr
	}

	options.Build.Log().WithFields(logrus.Fields{
		"internal-address":     info.InternalAddr,
		"external-address":     info.ExternalAddr,
		"use-external-address": useExternalAddr,
		"instance-id":          info.ID,
		"protocol-port":        options.Config.Autoscaler.ConnectorConfig.ProtocolPort,
	}).Info("Dialing instance")

	fleetingDialOpts := connector.DialOptions{
		UseExternalAddr: useExternalAddr,
	}

	ident := []string{info.ID}
	if options.Config.Autoscaler.LogInternalIP && info.InternalAddr != "" {
		ident = append(ident, info.InternalAddr)
	}
	if options.Config.Autoscaler.LogExternalIP && info.ExternalAddr != "" {
		ident = append(ident, info.ExternalAddr)
	}

	logger.Println(fmt.Sprintf("Dialing instance %s...", strings.Join(ident, ", ")))
	fleetingDialer, err := dialWithCancelCheck(dialCtx, ctx, ref.dialAcquisitionInstance, info, fleetingDialOpts)
	if err != nil {
		return nil, ref.classifyDialError(err)
	}
	ref.acq.HealthSuccess()
	logger.Println(fmt.Sprintf("Instance %s connected", info.ID))

	// if nesting is disabled, return a client for the host instance, for example VM Isolation and VM tunnel not needed
	if !options.Config.Autoscaler.VMIsolation.Enabled {
		return &client{client: fleetingDialer, cleanup: nil}, nil
	}

	// Enforce VM Isolation by dialing nesting daemon with gRPC
	logger.Println("Enforcing VM Isolation")
	nc, conn, err := ref.connectNesting(options.Config.Autoscaler.VMIsolation.NestingHost, logger, fleetingDialer)
	if err != nil {
		fleetingDialer.Close()
		return nil, err
	}

	logger.Println("Creating nesting VM tunnel")
	client, err := ref.createVMTunnel(ctx, logger, nc, fleetingDialer, options)
	if err != nil {
		nc.Close()
		conn.Close()
		fleetingDialer.Close()

		return nil, fmt.Errorf("creating vm tunnel: %w", err)
	}

	return client, nil
}

func (ref *acquisitionRef) WithContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ref.acq == nil {
		return context.WithCancel(ctx)
	}

	return ref.acq.WithContext(ctx)
}

func (ref *acquisitionRef) connectNesting(
	host string,
	logger buildlogger.Logger,
	fleetingDialer connector.Client,
) (nestingapi.Client, io.Closer, error) {
	if ref.connectNestingFn != nil {
		return ref.connectNestingFn(host, logger, fleetingDialer)
	}

	conn, err := nestingapi.NewClientConn(
		host,
		func(ctx context.Context, network, address string) (net.Conn, error) {
			logger.Println("Dialing nesting daemon")
			return fleetingDialer.Dial(network, address)
		},
	)
	if err != nil {
		// Could not dial nesting daemon
		return nil, nil, fmt.Errorf("dialing nesting daemon: %w", err)
	}

	return nestingapi.New(conn), conn, nil
}

func (ref *acquisitionRef) createVMTunnel(
	ctx context.Context,
	logger buildlogger.Logger,
	nc nestingapi.Client,
	fleetingDialer connector.Client,
	options common.ExecutorPrepareOptions,
) (executors.Client, error) {
	nestingCfg := options.Config.Autoscaler.VMIsolation

	// use nesting config defined image, unless the executor allows for the
	// job image to override.
	image := nestingCfg.Image
	if options.Build.Image.Name != "" && ref.mapJobImageToVMImage {
		image = options.Build.Image.Name
	}

	image = options.Build.GetAllVariables().ExpandValue(image)
	if image == "" {
		return nil, errNoNestingImageSpecified
	}

	logger.Println("Creating nesting VM", image)

	// create vm
	var vm hypervisor.VirtualMachine
	var stompedVMID *string
	var err error
	err = withInit(ctx, options.Config, nc, func() error {
		slot := int32(ref.AcquisitionSlot())
		vm, stompedVMID, err = nc.Create(ctx, image, &slot)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("creating nesting vm: %w", err)
	}

	logger.Infoln("Created nesting VM", vm.GetId(), vm.GetAddr())
	if stompedVMID != nil {
		logger.Infoln("Stomped nesting VM: ", *stompedVMID)
	}
	dialer, err := ref.createTunneledDialer(ctx, fleetingDialer, nestingCfg, vm)
	if err != nil {
		defer func() { _ = nc.Delete(ctx, vm.GetId()) }()

		return nil, fmt.Errorf("dialing nesting vm: %w", err)
	}

	cl := &client{dialer, func() error {
		defer fleetingDialer.Close()
		defer nc.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		return nc.Delete(ctx, vm.GetId())
	}}

	return cl, nil
}

func (ref *acquisitionRef) createTunneledDialer(
	ctx context.Context,
	dialer connector.Client,
	nestingCfg common.VMIsolation,
	vm hypervisor.VirtualMachine,
) (connector.Client, error) {
	info := fleetingprovider.ConnectInfo{
		ConnectorConfig: fleetingprovider.ConnectorConfig{
			OS:                   nestingCfg.ConnectorConfig.OS,
			Arch:                 nestingCfg.ConnectorConfig.Arch,
			Protocol:             fleetingprovider.Protocol(nestingCfg.ConnectorConfig.Protocol),
			ProtocolPort:         nestingCfg.ConnectorConfig.ProtocolPort,
			Username:             nestingCfg.ConnectorConfig.Username,
			Password:             nestingCfg.ConnectorConfig.Password,
			UseStaticCredentials: nestingCfg.ConnectorConfig.UseStaticCredentials,
			Keepalive:            nestingCfg.ConnectorConfig.Keepalive,
			Timeout:              nestingCfg.ConnectorConfig.Timeout,
		},
		InternalAddr: vm.GetAddr(),
	}

	options := connector.DialOptions{
		DialFn: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.Dial(network, addr)
		},
	}

	dialCtx, cancel := ref.acq.WithContext(ctx)
	defer cancel()

	return dialWithCancelCheck(dialCtx, ctx, ref.dialTunnel, info, options)
}

// classifyDialError determines whether a dial error is an instance failure that
// should trigger HealthFailure and instanceUnavailableError wrapping, or a plain
// error to return as-is. It must only be called when err != nil.
func (ref *acquisitionRef) classifyDialError(err error) error {
	if _, ok := errors.AsType[*common.BuildError](err); ok {
		return err
	}
	// taskscaler cancelled the dial (instance removed); already wrapped as
	// instanceUnavailableError — no HealthFailure needed, taskscaler already knows.
	if isInstanceUnavailable(err) {
		return err
	}
	// ErrConnectInfoExpired means the connect-info token expired before the dial
	// completed — a config/timing error, not an instance failure.
	if errors.Is(err, connector.ErrConnectInfoExpired) {
		return err
	}
	// Only a TCP-level connection failure counts as an instance failure:
	// *net.OpError{Op:"dial"} covers connection refused, no route to host, and
	// connect-phase timeouts (net.Dialer.DialContext wraps context.DeadlineExceeded
	// as Op:"dial", so no separate DeadlineExceeded case is needed).
	// Errors with Op "read" or "write" mean TCP connected successfully — the host
	// is alive; the failure is in the SSH handshake (auth, key mismatch, or a
	// read-phase i/o timeout after TCP accept) and must not penalise health state.
	// Note: a wrong protocol_port (connection refused) is indistinguishable from a
	// dead host at the network level; a later HealthSuccess from a good connection
	// resets the failure count.
	if opErr, ok := errors.AsType[*net.OpError](err); ok && opErr.Op == "dial" {
		ref.acq.HealthFailure()
		return &instanceUnavailableError{err: err}
	}
	// SSH auth failures, host-key mismatches, key-parse errors, handshake
	// read/write timeouts, and other non-dial errors are not instance failures.
	return err
}

// dialWithCancelCheck maps context cancellation to either an instanceUnavailableError
// (taskscaler removed the instance) or a BuildError (job was cancelled or timed out).
// When the dial itself succeeds concurrently with cancellation the client is closed.
func dialWithCancelCheck(
	dialCtx context.Context,
	parentCtx context.Context,
	dial connector.DialFn,
	info fleetingprovider.ConnectInfo,
	options connector.DialOptions,
) (connector.Client, error) {
	client, err := dial(dialCtx, info, options)
	if cause := context.Cause(dialCtx); cause != nil {
		if client != nil {
			client.Close()
		}
		// Benign race: if the job context is cancelled right after taskscaler
		// cancelled dialCtx, parentCtx.Err() may still be nil here and we
		// return instanceUnavailableError instead of a BuildError. The retry
		// loop sees the cancelled job context on its next attempt, so at most
		// one extra attempt happens.
		if parentCtx.Err() == nil {
			// dialCtx was cancelled by taskscaler (instance removed), not by the job
			return nil, &instanceUnavailableError{err: cause}
		}
		return nil, buildErrorFromContextCause(context.Cause(parentCtx))
	}

	return client, err
}

// instanceUnavailableError marks a Prepare failure caused by the acquired
// instance itself (its connect info could not be fetched or it could not be
// dialed). Retrying the same acquisition cannot succeed, so the autoscaler
// executor drops it and acquires another instance on the next attempt.
type instanceUnavailableError struct {
	err error
}

func (e *instanceUnavailableError) Error() string {
	return e.err.Error()
}

func (e *instanceUnavailableError) Unwrap() error {
	return e.err
}

func isInstanceUnavailable(err error) bool {
	var e *instanceUnavailableError
	return errors.As(err, &e)
}

// buildErrorFromContextCause maps a context cancellation cause to the
// appropriate BuildError failure reason. User-initiated cancels and job
// timeouts are distinguished from autoscaler-internal failures so that
// Rails receives an accurate failure attribution.
func buildErrorFromContextCause(cause error) *common.BuildError {
	switch {
	case errors.Is(cause, context.Canceled):
		return &common.BuildError{Inner: cause, FailureReason: common.JobCanceled}
	case errors.Is(cause, context.DeadlineExceeded):
		return &common.BuildError{Inner: cause, FailureReason: common.JobExecutionTimeout}
	default:
		return &common.BuildError{Inner: cause, FailureReason: common.RunnerSystemFailure}
	}
}

type client struct {
	client  connector.Client
	cleanup func() error
}

func (c *client) Dial(n string, addr string) (net.Conn, error) {
	return c.client.Dial(n, addr)
}

func (c *client) DialRun(ctx context.Context, command string) (net.Conn, error) {
	return c.client.DialRun(ctx, command)
}

func (c *client) Run(ctx context.Context, opts executors.RunOptions) error {
	err := c.client.Run(ctx, connector.RunOptions(opts))

	var exitErr *connector.ExitError
	if errors.As(err, &exitErr) {
		return &common.BuildError{
			Inner:    err,
			ExitCode: common.NormalizeExitCode(exitErr.ExitCode()),
		}
	}

	return err
}

func (c *client) Close() error {
	var err error
	if c.cleanup != nil {
		err = c.cleanup()
	}

	if cerr := c.client.Close(); cerr != nil {
		return cerr
	}
	return err
}
