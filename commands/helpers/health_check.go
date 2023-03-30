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
}

func (c *HealthCheckCommand) Execute(_ *cli.Context) {
	var ports []string
	var addr string

	if c.ctx == nil {
		c.ctx = context.Background()
	}

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

	if addr == "" || len(ports) == 0 {
		logrus.Fatalln("No HOST or PORT found")
	}

	fmt.Printf("waiting for TCP connection to %s on %v...\n", addr, ports)
	wg := sync.WaitGroup{}
	wg.Add(len(ports))
	ctx, cancel := context.WithCancel(c.ctx)
	defer cancel()

	for _, port := range ports {
		go checkPort(ctx, addr, port, cancel, wg.Done)
	}

	wg.Wait()
}

// checkPort will attempt to Dial the specified addr:port until successful. This function is intended to be run as a
// go-routine and has the following exit criteria:
//  1. A call to net.Dial is successful (i.e. does not return an error). A successful dial will also result in the
//     the passed context being cancelled.
//  2. The passed context is cancelled.
func checkPort(parentCtx context.Context, addr, port string, cancel func(), done func()) {
	defer done()
	defer cancel()

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
	common.RegisterCommand2("health-check", "check health for a specific address", &HealthCheckCommand{})
}
