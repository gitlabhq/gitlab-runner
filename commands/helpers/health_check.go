package helpers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type HealthCheckCommand struct {
	ctx context.Context

	Ports []string `long:"port" description:"Service port on localhost; every --port must respond"`
	// Separate field: with both forms on one field urfave appends the joined slice.
	ShortPorts      []string      `short:"p" description:"Deprecated alias for --port"`
	Images          []string      `long:"image" description:"NAME=REFERENCE of a service image; any of its EXPOSEd TCP ports must respond on localhost"`
	RegistryAuthDir string        `long:"registry-auth-dir" description:"Directory of mounted image pull secrets used to inspect --image references"`
	ServiceTimeout  time.Duration `long:"service-timeout" description:"How long to wait for an --image service before continuing with a warning; 0 waits until cancelled"`
}

type serviceCheck struct {
	name    string
	ports   []string
	timeout time.Duration
}

func NewHealthCheckCommand() cli.Command {
	return common.NewCommand("health-check", "check health for a specific address", &HealthCheckCommand{})
}

func (c *HealthCheckCommand) Execute(_ *cli.Context) {
	if c.ctx == nil {
		c.ctx = context.Background()
	}

	var addr string
	var services []serviceCheck

	ports := slices.Concat(c.Ports, c.ShortPorts)
	if len(ports) > 0 || len(c.Images) > 0 {
		// Kubernetes: the pod shares one network namespace.
		addr = "localhost"
		for _, port := range ports {
			services = append(services, serviceCheck{ports: []string{port}})
		}
		services = append(services, c.inspectImages()...)
		if len(services) == 0 {
			fmt.Println("no ports to wait for")
			return
		}
	} else {
		var envPorts []string
		addr, envPorts = parseEnvPorts()
		if len(envPorts) > 0 {
			services = []serviceCheck{{ports: envPorts}}
		}
	}

	if addr == "" || len(services) == 0 {
		logrus.Fatalln("No HOST or PORT found")
	}

	fmt.Printf("waiting for TCP connection to %s with %d service(s)...\n", addr, len(services))

	wg := sync.WaitGroup{}
	wg.Add(len(services))
	for i, service := range services {
		go c.checkService(addr, service, i, wg.Done)
	}
	wg.Wait()
}

// Uninspectable or portless images are not waited for, as without the flag.
func (c *HealthCheckCommand) inspectImages() []serviceCheck {
	if len(c.Images) == 0 {
		return nil
	}

	keychain := loadRegistryKeychain(c.RegistryAuthDir)

	ctx, cancel := context.WithTimeout(c.ctx, imageInspectTimeout)
	defer cancel()

	results := make([][]string, len(c.Images))
	wg := sync.WaitGroup{}
	for i, arg := range c.Images {
		name, ref, ok := strings.Cut(arg, "=")
		if !ok {
			logrus.Fatalln("--image must be NAME=REFERENCE, got:", arg)
		}
		wg.Add(1)
		go func(i int, name, ref string) {
			defer wg.Done()
			ports, err := imageExposedPorts(ctx, ref, keychain)
			switch {
			case err != nil:
				logrus.Warningln(fmt.Sprintf("Skipping health check for service %s: inspecting image %s: %v", name, ref, err))
			case len(ports) == 0:
				fmt.Printf("service %s: image %s exposes no TCP ports, skipping health check\n", name, ref)
			default:
				fmt.Printf("service %s: image %s exposes ports %v\n", name, ref, ports)
				results[i] = ports
			}
		}(i, name, ref)
	}
	wg.Wait()

	var services []serviceCheck
	for i, ports := range results {
		if len(ports) > 0 {
			name, _, _ := strings.Cut(c.Images[i], "=")
			services = append(services, serviceCheck{name: name, ports: ports, timeout: c.ServiceTimeout})
		}
	}
	return services
}

func parseEnvPorts() (string, []string) {
	var addr string
	var ports []string

	for _, e := range os.Environ() {
		key, val, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		switch {
		case strings.HasSuffix(key, "_TCP_ADDR"):
			addr = val
		case strings.HasSuffix(key, "_TCP_PORT"):
			ports = append(ports, val)
		}
	}

	return addr, ports
}

func (c *HealthCheckCommand) checkService(addr string, service serviceCheck, idx int, done func()) {
	defer done()

	fmt.Printf("checking service %d on ports %v...\n", idx+1, service.ports)

	var ctx context.Context
	var cancel context.CancelFunc
	if service.timeout > 0 {
		ctx, cancel = context.WithTimeout(c.ctx, service.timeout)
	} else {
		ctx, cancel = context.WithCancel(c.ctx)
	}
	defer cancel()

	var healthy atomic.Bool
	wg := sync.WaitGroup{}
	for _, port := range service.ports {
		wg.Add(1)
		go func(port string) {
			defer wg.Done()
			if checkPort(ctx, addr, port) {
				healthy.Store(true)
				cancel()
			}
		}(port)
	}
	wg.Wait()

	switch {
	case healthy.Load():
		fmt.Printf("service %d on ports %v is healthy\n", idx+1, service.ports)
	case errors.Is(ctx.Err(), context.DeadlineExceeded) && c.ctx.Err() == nil:
		logrus.Warningln(fmt.Sprintf(
			"Service %s did not accept a TCP connection on ports %v within %s, continuing without waiting for it; "+
				"set HEALTHCHECK_TCP_PORT on the service to wait for a specific port",
			service.name, service.ports, service.timeout))
	}
}

func checkPort(ctx context.Context, addr, port string) bool {
	for {
		if tryDial(ctx, addr, port) {
			return true
		}
		if ctx.Err() != nil {
			return false
		}
		time.Sleep(time.Second)
	}
}

func tryDial(ctx context.Context, addr, port string) bool {
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	fmt.Printf("dialing %s:%s...\n", addr, port)
	conn, err := (&net.Dialer{}).DialContext(dialCtx, "tcp", net.JoinHostPort(addr, port))
	if err != nil {
		return false
	}

	_ = conn.Close()
	fmt.Printf("dial succeeded on %s:%s. Exiting...\n", addr, port)
	return true
}
