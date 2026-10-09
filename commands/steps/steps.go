package steps

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run"
	"gitlab.com/gitlab-org/gitlab-runner/functions/script_legacy"
	"gitlab.com/gitlab-org/step-runner/pkg/api"
	"gitlab.com/gitlab-org/step-runner/pkg/api/proxy"
	"gitlab.com/gitlab-org/step-runner/pkg/di"
	"gitlab.com/gitlab-org/step-runner/pkg/runner"
	"gitlab.com/gitlab-org/step-runner/pkg/runner/gracefulexitcmd"
	"gitlab.com/gitlab-org/step-runner/pkg/runner/reaper"
	"gitlab.com/gitlab-org/step-runner/proto"
)

// gracefulExitDelay bounds the time the spawned script has between
// SIGTERM (on context cancellation) and SIGKILL of its process group.
// Also bounds how long we wait for stdout/stderr drain when background
// processes have inherited the pipe write-ends.
const gracefulExitDelay = 5 * time.Second

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

// builtinOptions registers the runner's own builtins with step-runner. Each
// wraps an entire job (common/build_step_dispatch.go), so both are
// Unannounced: announced, step-runner would write "Running step
// name=concrete" into the job log right after the runner's own stage header,
// on the rlog stream that no masker covers.
func builtinOptions() []func(*di.Container) {
	return []func(*di.Container){
		di.WithBuiltinFunc("script_legacy", script_legacy.New(), runner.Unannounced()),
		di.WithBuiltinFunc("concrete", concrete.New(), runner.Unannounced()),
	}
}

// EnvSocketPath is the env var the serving process advertises its socket
// path under, so in-process builtins (e.g. concrete) can dial back into
// step-runner to dispatch nested jobs. The reading side lives in
// functions/concrete/run; this alias keeps a single source of truth.
const EnvSocketPath = run.EnvSocketPath

// shutdownTimeout bounds how long a signal waits for running jobs to stop and
// their clients to collect the status, so a step that ignores cancellation
// cannot keep steps serve (PID 1 of the build container) alive until SIGKILL.
// Matches step-runner's own serve command.
const shutdownTimeout = 60 * time.Second

// signalError carries the OS signal that stopped the server. The serve command
// sets it as the context cancellation cause so Serve can fail the running jobs
// with the signal's exit code (128+N) via Shutdown, while an ordinary
// cancellation (command exit, parent cancel) carries no signal.
type signalError struct{ sig os.Signal }

func (s signalError) Error() string     { return s.sig.String() + " received" }
func (s signalError) Signal() os.Signal { return s.sig }

// sigExitCode is the conventional 128+N exit code for death by signal N.
func sigExitCode(sig os.Signal) int {
	if s, ok := sig.(syscall.Signal); ok {
		return 128 + int(s)
	}
	return 1
}

// Serve runs step-runner on sockPath until ctx is done or the optional command
// exits. If ctx is cancelled with a cause carrying an OS signal, Serve fails
// the jobs still running with that signal's exit code before stopping the
// server, so their clients receive a cause rather than a bare transport error.
func Serve(ctx context.Context, sockPath string, ioStreams IOStreams, cmdAndArgs ...string) error {
	// This process is PID 1 of a native-steps build container, so every
	// daemon or escaped process group a script leaves behind is reparented
	// here and stays a zombie unless reaped; zombies keep their pid, so a
	// script waiting for a stopped service to disappear never finishes.
	// step-runner leaves starting its reaper to the embedder.
	reaper.StartIfInit()

	listener, err := net.ListenUnix("unix", api.SocketAddr(sockPath))
	if err != nil {
		return fmt.Errorf("opening socket: %w", err)
	}
	defer listener.Close()

	// Advertise the socket path so in-process builtins can dial back in.
	_ = os.Setenv(EnvSocketPath, sockPath)

	service, err := di.NewContainer(append(
		builtinOptions(),
		di.WithOperatorLogger(slog.New(slog.DiscardHandler)),
	)...).StepRunnerService()
	if err != nil {
		return fmt.Errorf("initializing step-runner: %w", err)
	}

	srv := grpc.NewServer()
	proto.RegisterStepRunnerServer(srv, service)

	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	wg, ctx := errgroup.WithContext(ctx)

	go func() {
		<-ctx.Done()
		// A caller signal cancels ctx with a cause carrying the signal (see the
		// serve command). Fail the running jobs with it before GracefulStop:
		// GracefulStop rejects the Status and Close calls a client makes once
		// the log stream ends, so the client would otherwise see only a
		// transport error and the runner would report exit code 0 with no
		// cause. A command exit or a plain parent cancel carries no signal, so
		// those stop the server without failing jobs.
		if sc, ok := context.Cause(ctx).(interface{ Signal() os.Signal }); ok {
			// Bound the wait: a step that ignores cancellation, or a client
			// that never collects the status, must not keep this process
			// (PID 1) alive indefinitely.
			shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), shutdownTimeout)
			service.Shutdown(shutdownCtx, sc.Signal())
			cancelShutdown()
		}
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
			// on script exit, cancel so the step-runner serve also terminates
			defer cancel(nil)

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

			cmd := gracefulexitcmd.New(ctx, gracefulExitDelay, cmdAndArgs[0], cmdAndArgs[1:]...)
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
				// Turn SIGTERM/SIGINT into a context cancellation whose cause
				// carries the signal, so Serve fails running jobs with the right
				// exit code without threading a separate signal channel.
				ctx, cancel := context.WithCancelCause(context.Background())
				defer cancel(nil)
				sigs := make(chan os.Signal, 1)
				signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT)
				defer signal.Stop(sigs)
				go func() {
					select {
					case sig := <-sigs:
						cancel(signalError{sig})
						// The first signal starts a graceful shutdown that can
						// stall on a step ignoring cancellation. steps serve is
						// PID 1, so the handler stays installed and a second
						// signal would otherwise be swallowed; force-exit on it
						// so the container can always be stopped.
						sig = <-sigs
						os.Exit(sigExitCode(sig))
					case <-ctx.Done():
					}
				}()
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
