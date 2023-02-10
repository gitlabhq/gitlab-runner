package autoscaler

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/fleeting/fleeting/connector"
	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	"gitlab.com/gitlab-org/fleeting/nesting/api"
	"gitlab.com/gitlab-org/fleeting/nesting/hypervisor"
	"gitlab.com/gitlab-org/fleeting/taskscaler"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

var _ executors.Environment = (*acquisitionRef)(nil)

type acquisitionRef struct {
	key string
	acq *taskscaler.Acquisition

	mapJobImageToVMImage bool
}

func (ref *acquisitionRef) Prepare(
	ctx context.Context,
	logger common.BuildLogger,
	options common.ExecutorPrepareOptions,
) (executors.Client, error) {
	info, err := ref.acq.InstanceConnectInfo(ctx)
	if err != nil {
		return nil, err
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

	logger.Println(fmt.Sprintf("Dialing instance %s...", info.ID))
	dialer, err := connector.Dial(ctx, info, connector.DialOptions{
		UseExternalAddr: useExternalAddr,
	})
	if err != nil {
		return nil, err
	}
	logger.Println(fmt.Sprintf("Instance %s connected", info.ID))

	// if nesting is disabled, return a client for the instance
	if !options.Config.Autoscaler.VMIsolation.Enabled {
		return &client{dialer, nil}, nil
	}

	logger.Println("Enforcing VM Isolation")
	conn, err := api.NewClientConn(
		options.Config.Autoscaler.VMIsolation.NestingHost,
		func(ctx context.Context, network, address string) (net.Conn, error) {
			logger.Println("Dialing nesting daemon")
			return dialer.Dial(network, address)
		},
	)
	if err != nil {
		return nil, fmt.Errorf("dialing nesting daemon: %w", err)
	}

	nc := api.New(conn)

	logger.Println("Creating nesting VM tunnel")
	client, err := ref.createVMTunnel(ctx, logger, nc, dialer, options)
	if err != nil {
		nc.Close()
		conn.Close()
		dialer.Close()

		return nil, fmt.Errorf("creating vm tunnel: %w", err)
	}

	return client, nil
}

type client struct {
	client  connector.Client
	cleanup func() error
}

func (c *client) Dial(n string, addr string) (net.Conn, error) {
	return c.client.Dial(n, addr)
}

func (c *client) Run(ctx context.Context, opts executors.RunOptions) error {
	return c.client.Run(ctx, connector.RunOptions(opts))
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

func (ref *acquisitionRef) createVMTunnel(
	ctx context.Context,
	logger common.BuildLogger,
	nc api.Client,
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

	logger.Println("Creating nesting VM", image)

	// create vm
	var slot *int32
	if ref.acq != nil {
		var slot32 = int32(ref.acq.Slot())
		slot = &slot32
	}
	vm, stompedVMID, err := nc.Create(ctx, image, slot)
	if err != nil {
		return nil, fmt.Errorf("creating nesting vm: %w", err)
	}

	logger.Infoln("Created nesting VM", vm.GetId(), vm.GetAddr())
	if stompedVMID != nil {
		logger.Infoln("Stomped nesting VM: ", *stompedVMID)
	}
	dialer, err = createTunneledDialer(ctx, dialer, nestingCfg, vm)
	if err != nil {
		defer func() { _ = nc.Delete(ctx, vm.GetId()) }()

		return nil, fmt.Errorf("dialing nesting vm: %w", err)
	}

	return &client{dialer, func() error {
		defer nc.Close()

		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()

		return nc.Delete(ctx, vm.GetId())
	}}, nil
}

// Testing hook
var createTunneledDialer func(
	ctx context.Context,
	dialer connector.Client,
	nestingCfg common.VMIsolation,
	vm hypervisor.VirtualMachine,
) (connector.Client, error)

func init() {
	createTunneledDialer = func(
		ctx context.Context,
		dialer connector.Client,
		nestingCfg common.VMIsolation,
		vm hypervisor.VirtualMachine,
	) (connector.Client, error) {
		return connector.Dial(ctx, fleetingprovider.ConnectInfo{
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
		}, connector.DialOptions{
			DialFn: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
		})
	}
}
