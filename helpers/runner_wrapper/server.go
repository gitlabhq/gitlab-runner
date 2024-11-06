package runner_wrapper

import (
	"context"
	"errors"
	"net"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/runner_wrapper/proto"
)

type defaultShutdownCallbackDef struct {
	url     string
	method  string
	headers map[string]string
}

func (d *defaultShutdownCallbackDef) URL() string {
	return d.url
}

func (d *defaultShutdownCallbackDef) Method() string {
	return d.method
}

func (d *defaultShutdownCallbackDef) Headers() map[string]string {
	return d.headers
}

type defaultInitGracefulShutdownRequest struct {
	shutdownCallbackDef shutdownCallbackDef
}

func (d *defaultInitGracefulShutdownRequest) ShutdownCallbackDef() shutdownCallbackDef {
	return d.shutdownCallbackDef
}

//go:generate mockery --name=wrapper --inpackage --with-expecter
type wrapper interface {
	Status() Status
	FailureReason() string
	InitiateGracefulShutdown(req initGracefulShutdownRequest) error
}

var (
	statusMap = map[Status]proto.Status{
		StatusUnknown:    proto.Status_unknown,
		StatusRunning:    proto.Status_running,
		StatusInShutdown: proto.Status_in_shutdown,
		StatusStopped:    proto.Status_stopped,
	}
)

type Server struct {
	proto.UnimplementedProcessWrapperServer

	log        logrus.FieldLogger
	wrapper    wrapper
	grpcServer *grpc.Server
}

func NewServer(log logrus.FieldLogger, wrapper wrapper) *Server {
	return &Server{
		log:        log,
		wrapper:    wrapper,
		grpcServer: grpc.NewServer(),
	}
}

func (s *Server) Listen(listener net.Listener) {
	s.log.Info("Starting wrapper GRPC Server")

	proto.RegisterProcessWrapperServer(s.grpcServer, s)

	err := s.grpcServer.Serve(listener)
	if err != nil {
		s.log.WithError(err).Error("Failure while running wrapper GRPC Server")
	}
}

func (s *Server) Stop() {
	s.log.Info("Shutting down wrapper GRPC Server")

	s.grpcServer.Stop()
}

func (s *Server) CheckStatus(_ context.Context, _ *proto.Empty) (*proto.CheckStatusResponse, error) {
	s.log.Debug("Received CheckStatus request")

	resp := &proto.CheckStatusResponse{
		Status:        s.checkStatus(),
		FailureReason: s.wrapper.FailureReason(),
	}

	return resp, nil
}

func (s *Server) checkStatus() proto.Status {
	status, ok := statusMap[s.wrapper.Status()]
	if !ok {
		return proto.Status_unknown
	}

	return status
}

func (s *Server) InitGracefulShutdown(
	_ context.Context,
	req *proto.InitGracefulShutdownRequest,
) (*proto.InitGracefulShutdownResponse, error) {
	s.log.Debug("Received InitGracefulShutdown request")

	sc := &defaultShutdownCallbackDef{
		url:     req.GetShutdownCallback().GetUrl(),
		method:  req.GetShutdownCallback().GetMethod(),
		headers: req.GetShutdownCallback().GetHeaders(),
	}

	r := &defaultInitGracefulShutdownRequest{
		shutdownCallbackDef: sc,
	}

	err := s.wrapper.InitiateGracefulShutdown(r)

	resp := &proto.InitGracefulShutdownResponse{
		Status:        s.checkStatus(),
		FailureReason: s.wrapper.FailureReason(),
	}

	if errors.Is(err, errProcessNotInitialized) {
		return resp, nil
	}

	return resp, err
}
