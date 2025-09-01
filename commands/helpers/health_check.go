package helpers

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type HealthCheckCommand struct {
	ctx context.Context

	Ports []string `short:"p" long:"port" description:"Service port"`
}

func (c *HealthCheckCommand) Execute(_ *cli.Context) {
	var ports []string
	var addr string
	var waitAll bool

	if c.ctx == nil {
		c.ctx = context.Background()
	}

	// If command-line ports were given, use those. Otherwise search the environment. The command-line
	// 'port' flag is used by the kubernetes executor, and in kubernetes the networking environment is
	// shared among all containers in the pod. So we use localhost instead of another tcp address.
	if len(c.Ports) > 0 {
		addr = "localhost"

		// The urfave/cli package gives us an unwanted trailing entry, which apparently contains the
		// concatenation of all the --port arguments. Elide it.
		ports = c.Ports[:len(c.Ports)-1]

		// For kubernetes port checks, wait for all services to respond.
		waitAll = true
	} else {
		for _, e := range os.Environ() {
			parts := strings.Split(e, "=")

			switch {
			case len(parts) != 2:
				continue
			case strings.HasSuffix(parts[0], "_TCP_ADDR"):
				addr = parts[1]
			case strings.HasSuffix(parts[0], "_TCP_PORT"):
				ports = append(ports, parts[1])
			}
		}
	}

	if addr == "" || len(ports) == 0 {
		logrus.Fatalln("No HOST or PORT found")
	}

	fmt.Printf("waiting for TCP connection to %s on %v...\n", addr, ports)
	wg := sync.WaitGroup{}
	wg.Add(len(ports))
	ctx, cancel := context.WithCancel(c.ctx)
	defer cancel()

	for _, port := range ports {
		go checkPort(ctx, addr, port, cancel, wg.Done, waitAll)
	}

	wg.Wait()
}

// checkPort will attempt to Dial the specified addr:port until successful. This function is intended to be run as a
// go-routine and has the following exit criteria:
//  1. A call to net.Dial is successful (i.e. does not return an error). A successful dial will also result in the
//     the passed context being cancelled.
//  2. The passed context is cancelled.
func checkPort(parentCtx context.Context, addr, port string, cancel func(), done func(), waitAll bool) {
	defer done()

	// If we're not awaiting all services, arrange to cancel the parent context as soon as
	// a dial succeeds.
	if !waitAll {
		defer cancel()
	}

	for {
		ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
		defer cancel()

		fmt.Printf("dialing %s:%s...\n", addr, port)
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(addr, port))
		if err != nil {
			if parentCtx.Err() != nil {
				return
			}
			time.Sleep(time.Second)
			continue
		}

		_ = conn.Close()
		fmt.Printf("dial succeeded on %s:%s. Exiting...\n", addr, port)
		return
	}
}

func init() {
	common.RegisterCommand("health-check", "check health for a specific address", &HealthCheckCommand{})
}
