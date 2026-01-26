package steps

import (
	"bufio"
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

	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/functions/script_legacy"
	"gitlab.com/gitlab-org/step-runner/pkg/api"
	"gitlab.com/gitlab-org/step-runner/pkg/api/proxy"
	"gitlab.com/gitlab-org/step-runner/pkg/di"
	"gitlab.com/gitlab-org/step-runner/proto"
)

const (
	SubCommandName = "steps"

	readyMessage = "step-runner is ready."
)

type IOStreams struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

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

//nolint:gocognit
func Serve(ctx context.Context, sockPath string, ioStreams IOStreams, cmdAndArgs ...string) error {
	listener, err := net.ListenUnix("unix", api.SocketAddr(sockPath))
	if err != nil {
		return fmt.Errorf("opening socket: %w", err)
	}
	defer listener.Close()

	service, err := di.NewContainer(
		di.WithStepFunc("script_legacy", script_legacy.Spec(), script_legacy.Run),
	).StepRunnerService()

	if err != nil {
		return fmt.Errorf("initializing step-runner: %w", err)
	}

	srv := grpc.NewServer()
	proto.RegisterStepRunnerServer(srv, service)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	wg, ctx := errgroup.WithContext(ctx)

	go func() {
		<-ctx.Done()
		srv.GracefulStop()
	}()

	wg.Go(func() error {
		if err := srv.Serve(listener); err != nil {
			return fmt.Errorf("server error: %w", err)
		}

		return nil
	})

	fmt.Fprintln(os.Stderr, readyMessage)

	if len(cmdAndArgs) > 0 {
		wg.Go(func() error {
			// on script exit, we cancel() so that the step-runner serve also terminates
			defer cancel()

			stdin := bufio.NewReader(ioStreams.Stdin)

			stdinCheck := make(chan error, 1)
			go func() {
				_, err := stdin.Peek(1)
				stdinCheck <- err
			}()

			// block until either:
			// - cancellation
			// - data on stdin
			//
			// this prevents us running a command with no script to execute, and therefore returning
			// an error on cancellation even if there's no work performed.
			select {
			case err := <-stdinCheck:
				if errors.Is(err, io.EOF) {
					return nil
				}
			case <-ctx.Done():
				return nil
			}

			cmd := exec.CommandContext(ctx, cmdAndArgs[0], cmdAndArgs[1:]...)
			cmd.Stdin = stdin
			cmd.Stdout = ioStreams.Stdout
			cmd.Stderr = ioStreams.Stderr

			// error is not wrapped intentionally:
			// os.ExitError needs to be returned unwrapped.
			return cmd.Run()
		})
	}

	return wg.Wait()
}

func Proxy(sockPath string, io IOStreams) error {
	conn, err := net.DialUnix("unix", nil, api.SocketAddr(sockPath))
	if err != nil {
		return fmt.Errorf("dialing: %w", err)
	}
	defer conn.Close()

	return proxy.Proxy(io.Stdin, io.Stdout, conn)
}

func NewCommand() cli.Command {
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

	return common.NewCommandWithSubcommands(
		SubCommandName,
		"manage server that can run CI Functions (internal)",
		common.CommanderFunc(func(ctx *cli.Context) {
			_ = cli.ShowAppHelp(ctx)
		}),
		true,
		subcommands,
	)
}
