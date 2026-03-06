package steps

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete"
	"gitlab.com/gitlab-org/gitlab-runner/functions/script_legacy"
	"gitlab.com/gitlab-org/step-runner/pkg/api"
	"gitlab.com/gitlab-org/step-runner/pkg/api/proxy"
	"gitlab.com/gitlab-org/step-runner/pkg/di"
	"gitlab.com/gitlab-org/step-runner/proto"
)

const (
	SubCommandName = "steps"
)

func readyMessage(sockPath string) string {
	return fmt.Sprintf("step-runner is listening on socket %s", sockPath)
}

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

	if err := copyFile(source, destination, 0o755); err != nil {
		return fmt.Errorf("failed to copy binary: %w", err)
	}

	sslSource := "/ca-certs.pem"
	if _, err := os.Stat(sslSource); err == nil {
		sslDest := filepath.Join(filepath.Dir(destination), "ca-certs.pem")
		if err := copyFile(sslSource, sslDest, 0o644); err != nil {
			return fmt.Errorf("failed to copy ssl certs: %w", err)
		}
	}

	gitSource := "/git"
	if _, err := os.Stat(gitSource); err == nil {
		gitDest := filepath.Join(filepath.Dir(destination), "git")
		if err := copyDir(gitSource, gitDest); err != nil {
			return fmt.Errorf("failed to copy git directory: %w", err)
		}
	}

	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		info, err := d.Info()
		if err != nil {
			return err
		}

		if info.Mode()&os.ModeSymlink != 0 {
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(link, target)
		}

		if d.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}

		return copyFile(path, target, info.Mode())
	})
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	if err := out.Close(); err != nil {
		return err
	}

	return os.Chmod(dst, mode)
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
		di.WithStepFunc("concrete", concrete.Spec(), concrete.Run),
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

	fmt.Fprintln(os.Stderr, readyMessage(sockPath))

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
