package helpers

import (
	"fmt"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type HealthCheckCommand struct{}

func (c *HealthCheckCommand) Execute(ctx *cli.Context) {
	var ports []int
	var addr, port string

	for _, e := range os.Environ() {
		parts := strings.Split(e, "=")

		switch {
		case len(parts) != 2:
			continue
		case strings.HasSuffix(parts[0], "_TCP_ADDR"):
			addr = parts[1]
		case strings.HasSuffix(parts[0], "_TCP_PORT"):
			portNumber, err := strconv.Atoi(parts[1])
			if err != nil {
				continue
			}
			ports = append(ports, portNumber)
		}
	}

	sort.Ints(ports)
	if len(ports) > 0 {
		port = strconv.Itoa(ports[0])
	}

	if addr == "" || port == "" {
		logrus.Fatalln("No HOST or PORT found")
	}

	_, _ = fmt.Fprintf(os.Stdout, "waiting for TCP connection to %s:%s...", addr, port)

	for {
		conn, err := net.Dial("tcp", net.JoinHostPort(addr, port))
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		_ = conn.Close()
		return
	}
}

func init() {
	common.RegisterCommand2("health-check", "check health for a specific address", &HealthCheckCommand{})
}
