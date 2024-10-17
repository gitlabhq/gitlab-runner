package commands

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/runner_wrapper"
)

const (
	defaultWrapperGRPCListen = "tcp://localhost:7777"
)

var (
	errFailedToParseGRPCAddress     = errors.New("failed to parse grpc-listen address")
	errUnsupportedGRPCAddressScheme = errors.New("unsupported grpc-listen address scheme")
)

type logHook struct{}

func (h *logHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *logHook) Fire(e *logrus.Entry) error {
	e.Message = "[WRAPPER] " + e.Message

	return nil
}

type RunnerWrapperCommand struct {
	GRPCListen                string        `long:"grpc-listen"`
	ProcessTerminationTimeout time.Duration `long:"process-termination-timeout"`
}

func (c *RunnerWrapperCommand) Execute(cctx *cli.Context) {
	logrus.AddHook(new(logHook))
	log := logrus.WithField("wrapper", true)
	grpcLog := log.WithField("grpc-listen-addr", c.GRPCListen)

	path, err := os.Executable()
	if err != nil {
		log.WithError(err).Fatal("Failed to get executable path")
	}

	l, err := net.Listen("tcp", c.GRPCListen)
	if err != nil {
		grpcLog.WithError(err).Fatal("Failed to create listener")
	}

	ctx, cancelFn := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancelFn()

	w := runner_wrapper.New(log, path, cctx.Args())
	w.SetTerminationTimeout(c.ProcessTerminationTimeout)

	srv := runner_wrapper.NewServer(grpcLog, w)

	go srv.Listen(l)

	err = w.Run(ctx)
	if err != nil {
		log.WithError(err).Fatal("Failed while executing wrapped command")
	}

	srv.Stop()
	log.Info("All wrapper tasks finished. See you!")
}

func (c *RunnerWrapperCommand) createListener() (net.Listener, error) {
	uri, err := url.ParseRequestURI(c.GRPCListen)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errFailedToParseGRPCAddress, err)
	}

	switch uri.Scheme {
	case "unix":
		return net.Listen("unix", uri.Path)
	case "tcp":
		return net.Listen("tcp", uri.Host)
	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportedGRPCAddressScheme, uri.Scheme)
	}
}

func init() {
	common.RegisterCommand2(
		"wrapper", "start multi runner service wrapped with gRPC manager server",
		&RunnerWrapperCommand{
			GRPCListen:                defaultWrapperGRPCListen,
			ProcessTerminationTimeout: runner_wrapper.DefaultTerminationTimeout,
		},
	)
}
