package server

import (
	"context"
	"errors"
	"net"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/runner_wrapper/api"
	pb "gitlab.com/gitlab-org/gitlab-runner/helpers/runner_wrapper/api/proto"
)

//go:generate mockery --name=wrapper --inpackage --with-expecter
type wrapper interface {
	Status() api.Status
	FailureReason() string
	InitiateGracefulShutdown(req api.InitGracefulShutdownRequest) error
	InitiateForcefulShutdown() error
}

type Server struct {
	pb.UnimplementedProcessWrapperServer

	log        logrus.FieldLogger
	wrapper    wrapper
	grpcServer *grpc.Server
}

func New(log logrus.FieldLogger, wrapper wrapper) *Server {
	return &Server{
		log:        log,
		wrapper:    wrapper,
		grpcServer: grpc.NewServer(),
	}
}

func (s *Server) Listen(listener net.Listener) {
	s.log.Info("Starting wrapper GRPC Server")

	pb.RegisterProcessWrapperServer(s.grpcServer, s)

	err := s.grpcServer.Serve(listener)
	if err != nil {
		s.log.WithError(err).Error("Failure while running wrapper GRPC Server")
	}
}

func (s *Server) Stop() {
	s.log.Info("Shutting down wrapper GRPC Server")

	s.grpcServer.Stop()
}

func (s *Server) CheckStatus(_ context.Context, _ *pb.CheckStatusRequest) (*pb.CheckStatusResponse, error) {
	s.log.Debug("Received CheckStatus request")

	resp := &pb.CheckStatusResponse{
		Status:        api.Statuses.Map(s.wrapper.Status()),
		FailureReason: s.wrapper.FailureReason(),
	}

	return resp, nil
}

func (s *Server) InitGracefulShutdown(
	_ context.Context,
	req *pb.InitGracefulShutdownRequest,
) (*pb.InitGracefulShutdownResponse, error) {
	s.log.Debug("Received InitGracefulShutdown request")

	sc := api.NewShutdownCallbackDef(
		req.GetShutdownCallback().GetUrl(),
		req.GetShutdownCallback().GetMethod(),
		req.GetShutdownCallback().GetHeaders(),
	)

	r := api.NewInitGracefulShutdownRequest(sc)

	err := s.wrapper.InitiateGracefulShutdown(r)
	if err != nil {
		if errors.Is(err, api.ErrProcessNotInitialized) {
			err = nil
		}
	}

	resp := &pb.InitGracefulShutdownResponse{
		Status:        api.Statuses.Map(s.wrapper.Status()),
		FailureReason: s.wrapper.FailureReason(),
	}

	return resp, err
}

func (s *Server) InitForcefulShutdown(_ context.Context, _ *pb.InitForcefulShutdownRequest) (*pb.InitForcefulShutdownResponse, error) {
	s.log.Debug("Received InitForcefulShutdown request")

	err := s.wrapper.InitiateForcefulShutdown()
	if err != nil {
		if errors.Is(err, api.ErrProcessNotInitialized) {
			err = nil
		}
	}

	resp := &pb.InitForcefulShutdownResponse{
		Status:        api.Statuses.Map(s.wrapper.Status()),
		FailureReason: s.wrapper.FailureReason(),
	}

	return resp, err
}
