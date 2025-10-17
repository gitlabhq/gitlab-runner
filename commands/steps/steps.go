package steps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/urfave/cli"
	"google.golang.org/grpc"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/step-runner/pkg/api"
	"gitlab.com/gitlab-org/step-runner/pkg/api/proxy"
	"gitlab.com/gitlab-org/step-runner/pkg/di"
	"gitlab.com/gitlab-org/step-runner/proto"
)

const (
	SubCommandName = "steps"

	// shutdownTimeout is time we wait for a graceful shutdown of the server before we run the forceful shutdown
	shutdownTimeout = time.Second * 5
)

type IOStreams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// gracefulShutdown is a special error we use to cancel the context when no error occurred. With that, we can
// differentiate between explicit cancels we did, or any cancellation by a parent context.
var gracefulShutdown = fmt.Errorf("shut down gracefully")

func Bootstrap(destination string) error {
	source, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get source path: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
		return err
	}

	src, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source file %q: %w", source, err)
	}
	defer func() { _ = src.Close() }()

	dest, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func() { _ = dest.Close() }()

	_, err = io.Copy(dest, src)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	if err := dest.Close(); err != nil {
		return fmt.Errorf("failed to close file: %w", err)
	}

	return os.Chmod(destination, 0o755)
}

func Serve(ctx context.Context, sockPath string, ioStreams IOStreams, cmdAndArgs ...string) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(gracefulShutdown)

	listener, err := net.ListenUnix("unix", api.SocketAddr(sockPath))
	if err != nil {
		return fmt.Errorf("opening socket: %w", err)
	}
	defer listener.Close()

	service, err := di.NewContainer().StepRunnerService()
	if err != nil {
		return fmt.Errorf("initializing step-runner: %w", err)
	}

	srv := grpc.NewServer()
	proto.RegisterStepRunnerServer(srv, service)

	serverStopped := make(chan struct{}, 2)

	go func() {
		<-ctx.Done()

		defer time.AfterFunc(shutdownTimeout, func() {
			srv.Stop()
			serverStopped <- struct{}{}
		}).Stop()

		srv.GracefulStop()
		serverStopped <- struct{}{}
	}()

	go func() {
		err := srv.Serve(listener)
		if err != nil {
			cancel(fmt.Errorf("server error: %w", err))
		}
		cancel(gracefulShutdown)
	}()

	if len(cmdAndArgs) > 0 {
		go func() {
			cmd := exec.CommandContext(ctx, cmdAndArgs[0], cmdAndArgs[1:]...)
			cmd.Stdin = ioStreams.Stdin
			cmd.Stdout = ioStreams.Stdout
			cmd.Stderr = ioStreams.Stderr

			err := cmd.Run()
			if err != nil {
				cancel(fmt.Errorf("command error: %w", err))
			}
			cancel(gracefulShutdown)
		}()
	}

	<-ctx.Done()
	err = context.Cause(ctx)
	if errors.Is(err, gracefulShutdown) {
		// context.Cancel will always give as at least context.Canceled, and we can't be sure where this came from (a parent
		// context that was canceled by a timeout?). Thus we use a special cancel cause for our known graceful
		// cancellations, and know that this is not an error case.
		err = nil
	}

	<-serverStopped
	return err
}

func Proxy(sockPath string, io IOStreams) error {
	conn, err := net.DialUnix("unix", nil, api.SocketAddr(sockPath))
	if err != nil {
		return fmt.Errorf("dialing: %w", err)
	}
	defer conn.Close()

	return proxy.Proxy(io.Stdin, io.Stdout, conn)
}

func init() {
	const sockFlag = "socket"
	defaultSockPath := api.DefaultSocketPath()

	subcommands := []cli.Command{
		{
			Name:  "bootstrap",
			Usage: "bootstrap the gitlab-runner-helper to the build container",
			Action: func(cliCtx *cli.Context) error {
				destination := cliCtx.Args().First()
				if destination == "" {
					return fmt.Errorf("destination argument must be provided")
				}

				return Bootstrap(destination)
			},
		},
		{
			Name:  "serve",
			Usage: "start the CI Functions server",
			Action: func(cliCtx *cli.Context) error {
				ctx, stopNotify := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
				defer stopNotify()
				io := IOStreams{
					Stdin:  os.Stdin,
					Stdout: os.Stdout,
					Stderr: os.Stderr,
				}
				return Serve(ctx, cliCtx.String(sockFlag), io, cliCtx.Args()...)
			},
			Flags: []cli.Flag{
				cli.StringFlag{Name: sockFlag, Value: defaultSockPath},
			},
		},
		{
			Name:  "proxy",
			Usage: "connect stdin/stdout to the CI Functions server",
			Action: func(cliCtx *cli.Context) error {
				io := IOStreams{
					Stdin:  os.Stdin,
					Stdout: os.Stdout,
				}
				return Proxy(cliCtx.String(sockFlag), io)
			},
			Flags: []cli.Flag{
				cli.StringFlag{Name: sockFlag, Value: defaultSockPath},
			},
		},
	}

	common.RegisterCommandWithSubcommands(
		SubCommandName,
		"manage server that can run CI Functions (internal)",
		common.CommanderFunc(func(ctx *cli.Context) {
			_ = cli.ShowAppHelp(ctx)
		}),
		true,
		subcommands,
	)
}
