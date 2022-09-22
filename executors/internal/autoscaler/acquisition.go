package autoscaler

import (
	"context"
	"net"

	"gitlab.com/gitlab-org/fleeting/fleeting/connector"
	"gitlab.com/gitlab-org/fleeting/taskscaler"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

var _ executors.Environment = (*acqusitionRef)(nil)

type acqusitionRef struct {
	key string
	acq *taskscaler.Aquisition
}

func (ref *acqusitionRef) ID() string {
	return ref.acq.InstanceID()
}

func (ref *acqusitionRef) OS() string {
	return ref.acq.InstanceConnectInfo().OS
}

func (ref *acqusitionRef) Arch() string {
	return ref.acq.InstanceConnectInfo().Arch
}

func (ref *acqusitionRef) Dial(ctx context.Context) (executors.Client, error) {
	info := ref.acq.InstanceConnectInfo()

	dialer, err := connector.Dial(ctx, info, connector.DialOptions{
		// todo: make this configurable
		UseExternalAddr: true,
	})
	if err != nil {
		return nil, err
	}

	return &client{dialer}, nil
}

func (ref *acqusitionRef) set(key string, acq *taskscaler.Aquisition) {
	if ref.acq != nil {
		panic("acqusition ref already set")
	}

	ref.key = key
	ref.acq = acq
}

func (ref *acqusitionRef) get() string {
	return ref.key
}

type client struct {
	client connector.Client
}

func (c *client) Dial(n string, addr string) (net.Conn, error) {
	return c.client.Dial(n, addr)
}

func (c *client) Run(opts executors.RunOptions) error {
	return c.client.Run(connector.RunOptions(opts))
}

func (c *client) Close() error {
	return c.client.Close()
}
