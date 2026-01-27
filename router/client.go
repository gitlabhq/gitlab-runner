package router

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/network"
	"gitlab.com/gitlab-org/gitlab-runner/router/rpc"
)

var (
	_ common.Network = (*Client)(nil)
)

const (
	requestIDMetadataKey = "x-request-id" // lowercase version of X-Request-ID
	discoveryTTL         = time.Hour
)

type Delegate interface {
	common.Network
	PrepareJobRequest(config common.RunnerConfig, sessionInfo *common.SessionInfo) common.JobRequest
	GetRouterDiscovery(ctx context.Context, config common.RunnerConfig) *common.RouterDiscovery
}

type Client struct {
	common.Network // delegate all the methods except RequestJob()
	delegate       Delegate
	factory        *ClientConnFactory
	mu             sync.Mutex
	disco          *common.RouterDiscovery
	discoExpiresAt time.Time
}

func NewClient(delegate Delegate, certDirectory, userAgent string) *Client {
	return &Client{
		Network:  delegate,
		delegate: delegate,
		factory:  NewClientConnFactory(certDirectory, userAgent),
	}
}

func (c *Client) Shutdown() {
	c.factory.Shutdown()
}

func (c *Client) RequestJob(ctx context.Context, config common.RunnerConfig, sessionInfo *common.SessionInfo) (*spec.Job, bool) {
	client, disco := c.getClientOrNil(ctx, config)
	if client == nil {
		return c.delegate.RequestJob(ctx, config, sessionInfo)
	}
	defer client.Done()

	jobRequest := c.delegate.PrepareJobRequest(config, sessionInfo)
	jobRequestJSON, err := json.Marshal(jobRequest)
	if err != nil {
		config.Log().WithError(err).Error("json.Marshal()")
		return nil, false
	}

	var responseMD metadata.MD
	requestCorrelationID := network.NewCorrelationID()
	job, err := rpc.NewJobRouterClient(client).GetJob(
		metadata.NewOutgoingContext(ctx, metadata.Pairs(
			requestIDMetadataKey, requestCorrelationID,
		)),
		&rpc.GetJobRequest{
			JobRequest: jobRequestJSON,
		},
		grpc.Header(&responseMD),
	)
	if err != nil {
		config.Log().WithError(err).Error("Error requesting a job")
		switch status.Code(err) {
		case codes.DeadlineExceeded, codes.Canceled, codes.Unavailable:
			return nil, true
		default:
			return nil, false
		}
	}
	if len(job.JobResponse) == 0 {
		return nil, true
	}
	var response spec.Job
	err = json.Unmarshal(job.JobResponse, &response)
	if err != nil {
		config.Log().WithError(err).Error("json.Unmarshal()")
		return nil, false
	}
	response.TLSData = disco.TLSData
	correlationIDs := responseMD[requestIDMetadataKey]
	if len(correlationIDs) > 0 {
		requestCorrelationID = correlationIDs[0]
	}
	response.JobRequestCorrelationID = requestCorrelationID
	return &response, true
}

func (c *Client) getClientOrNil(ctx context.Context, config common.RunnerConfig) (ClientConn, *common.RouterDiscovery) {
	disco := c.getRouterDiscovery(ctx, config)
	if disco == nil {
		return nil, nil
	}
	conn, err := c.factory.Dial(DialTarget{
		URL:         disco.ServerURL,
		Token:       config.Token,
		TLSCAFile:   config.TLSCAFile, // use the same TLS bits as for the main GitLab URL
		TLSCertFile: config.TLSCertFile,
		TLSKeyFile:  config.TLSKeyFile,
	})
	if err != nil {
		config.Log().WithError(err).Error("Dial")
		return nil, disco
	}
	return conn, disco
}

func (c *Client) getRouterDiscovery(ctx context.Context, config common.RunnerConfig) *common.RouterDiscovery {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.discoExpiresAt.After(time.Now()) {
		return c.disco
	}
	c.disco = c.delegate.GetRouterDiscovery(ctx, config)
	c.discoExpiresAt = time.Now().Add(discoveryTTL)
	if c.disco != nil {
		config.Log().Info("Using job router at " + c.disco.ServerURL)
	}
	return c.disco
}
