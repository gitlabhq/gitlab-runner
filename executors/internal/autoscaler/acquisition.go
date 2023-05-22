package autoscaler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/fleeting/fleeting/connector"
	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	nestingapi "gitlab.com/gitlab-org/fleeting/nesting/api"
	"gitlab.com/gitlab-org/fleeting/nesting/hypervisor"
	"gitlab.com/gitlab-org/fleeting/taskscaler"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

var _ executors.Environment = (*acquisitionRef)(nil)

var (
	errRefAcqNotSet = errors.New("ref.acq is not set")

	errNoNestingImageSpecified = errors.New("no nesting VM image specified to run the job in")
)

type acquisitionRef struct {
	key string
	acq taskscaler.Acquisition

	mapJobImageToVMImage bool

	// test hooks
	dialAcquisitionInstance connector.DialFn
	dialTunnel              connector.DialFn

	connectNestingFn func(
		host string,
		logger common.BuildLogger,
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

func (ref *acquisitionRef) Prepare(
	ctx context.Context,
	logger common.BuildLogger,
	options common.ExecutorPrepareOptions,
) (executors.Client, error) {
	if ref.acq == nil {
		return nil, errRefAcqNotSet
	}

	info, err := ref.acq.InstanceConnectInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting instance connect info: %w", err)
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
	}).Info("Dialing instance")

	fleetingDialOpts := connector.DialOptions{
		UseExternalAddr: useExternalAddr,
	}

	logger.Println(fmt.Sprintf("Dialing instance %s...", info.ID))
	fleetingDialer, err := ref.dialAcquisitionInstance(ctx, info, fleetingDialOpts)
	if err != nil {
		return nil, err
	}
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

func (ref *acquisitionRef) connectNesting(
	host string,
	logger common.BuildLogger,
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
	logger common.BuildLogger,
	nc nestingapi.Client,
	dialer connector.Client,
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
	var slot *int32

	var slot32 = int32(ref.acq.Slot())
	slot = &slot32

	var vm hypervisor.VirtualMachine
	var stompedVMID *string
	var err error
	err = withInit(ctx, options.Config, nc, func() error {
		vm, stompedVMID, err = nc.Create(ctx, image, slot)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("creating nesting vm: %w", err)
	}

	logger.Infoln("Created nesting VM", vm.GetId(), vm.GetAddr())
	if stompedVMID != nil {
		logger.Infoln("Stomped nesting VM: ", *stompedVMID)
	}
	dialer, err = ref.createTunneledDialer(ctx, dialer, nestingCfg, vm)
	if err != nil {
		defer func() { _ = nc.Delete(ctx, vm.GetId()) }()

		return nil, fmt.Errorf("dialing nesting vm: %w", err)
	}

	cl := &client{dialer, func() error {
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

	return ref.dialTunnel(ctx, info, options)
}

type client struct {
	client  connector.Client
	cleanup func() error
}

func (c *client) Dial(n string, addr string) (net.Conn, error) {
	return c.client.Dial(n, addr)
}

func (c *client) Run(ctx context.Context, opts executors.RunOptions) error {
	err := c.client.Run(ctx, connector.RunOptions(opts))

	var exitErr *connector.ExitError
	if errors.As(err, &exitErr) {
		return &common.BuildError{
			Inner:    err,
			ExitCode: exitErr.ExitCode(),
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
