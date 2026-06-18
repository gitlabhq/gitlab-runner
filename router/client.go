package router

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/circuitbreaker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/network"
	"gitlab.com/gitlab-org/gitlab-runner/router/rpc"
)

var (
	_ common.Network       = (*Client)(nil)
	_ prometheus.Collector = (*Client)(nil)
)

const (
	requestIDMetadataKey = "x-request-id" // lowercase version of X-Request-ID
	discoveryTTL         = time.Hour

	// routerBreakerFailureThreshold is the number of consecutive router failures
	// that trips the breaker; fewer are treated as transient.
	routerBreakerFailureThreshold = 3
	// routerBreakerOpenTimeout is how long the breaker stays open before allowing
	// a trial request to test recovery.
	routerBreakerOpenTimeout = 30 * time.Second
	// routerBreakerFailureGrace requires failures to persist for at least this
	// long before tripping, so a burst of simultaneous failures (e.g. a shared
	// connection dropping during a router rollout) does not trip the breaker -
	// only a sustained outage does. Initial estimate, tunable via gitlab-runner#39407.
	routerBreakerFailureGrace = 5 * time.Second
)

// isRouterFailure reports whether a GetJob gRPC code means the router is
// unreachable and should count toward tripping the breaker. It covers only
// availability/transport failures, which the non-router path also treats as
// healthy. Server-side and request-level errors (Internal, InvalidArgument, etc.)
// fall through to the default handling (mark the runner unhealthy); Unimplemented
// is the deliberate-disable path, handled separately.
func isRouterFailure(code codes.Code) bool {
	switch code {
	case codes.Unavailable, codes.DeadlineExceeded, codes.Canceled:
		return true
	default:
		return false
	}
}

type Delegate interface {
	common.Network
	PrepareJobRequest(config common.RunnerConfig, sessionInfo *common.SessionInfo) common.JobRequest
	GetRouterDiscovery(ctx context.Context, config common.RunnerConfig) *common.RouterDiscovery
}

type Client struct {
	common.Network // delegate all the methods except RequestJob()
	delegate       Delegate
	factory        *ClientConnFactory
	breaker        *circuitbreaker.Breaker
	mu             sync.Mutex
	disco          *common.RouterDiscovery
	discoExpiresAt time.Time
}

func NewClient(delegate Delegate, certDirectory, userAgent string) *Client {
	return &Client{
		Network:  delegate,
		delegate: delegate,
		factory:  NewClientConnFactory(certDirectory, userAgent),
		breaker: circuitbreaker.New(routerBreakerFailureThreshold, routerBreakerOpenTimeout,
			circuitbreaker.WithFailureGrace(routerBreakerFailureGrace),
			circuitbreaker.WithMetrics("gitlab_runner", "job_router_circuit_breaker")),
	}
}

func (c *Client) Describe(ch chan<- *prometheus.Desc) {
	c.breaker.Describe(ch)
}

func (c *Client) Collect(ch chan<- prometheus.Metric) {
	c.breaker.Collect(ch)
}

func (c *Client) Shutdown() {
	c.factory.Shutdown()
}

func (c *Client) RequestJob(ctx context.Context, config common.RunnerConfig, sessionInfo *common.SessionInfo) (*spec.Job, bool) {
	if !config.IsFeatureFlagOn(featureflags.UseJobRouter) {
		return c.delegate.RequestJob(ctx, config, sessionInfo)
	}

	// Resolve discovery before the breaker gate so a "no router" result can't
	// strand a half-open trial. Every path past Allow() must then resolve the
	// trial: record an outcome once the router is reached, or Abort if we bail.
	disco := c.getRouterDiscovery(ctx, config)
	if disco == nil {
		return c.delegate.RequestJob(ctx, config, sessionInfo)
	}

	if !c.breaker.Allow() {
		return c.delegate.RequestJob(ctx, config, sessionInfo)
	}

	jobRequest := c.delegate.PrepareJobRequest(config, sessionInfo)
	jobRequestJSON, err := json.Marshal(jobRequest)
	if err != nil {
		// The router was never contacted, so there's no success/failure to record;
		// Abort releases the half-open trial Allow() may have granted.
		c.breaker.Abort()
		config.Log().WithError(err).Error("json.Marshal()")
		return nil, false
	}

	client, err := c.factory.Dial(DialTarget{
		URL:         disco.ServerURL,
		Token:       config.Token,
		TLSCAFile:   config.TLSCAFile, // use the same TLS bits as for the main GitLab URL
		TLSCertFile: config.TLSCertFile,
		TLSKeyFile:  config.TLSKeyFile,
	})
	if err != nil {
		if c.breaker.RecordFailure() {
			config.Log().WithError(err).WithField("job_router_url", disco.ServerURL).
				Warn("Job router circuit breaker tripped on dial failure, falling back to direct job requests")
		} else {
			config.Log().WithError(err).WithField("job_router_url", disco.ServerURL).
				Warn("Job router dial failed, falling back to direct job requests")
		}
		return c.delegate.RequestJob(ctx, config, sessionInfo)
	}
	defer client.Done()

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
		healthy, fallback := c.handleRouterError(err, disco, config)
		if fallback {
			return c.delegate.RequestJob(ctx, config, sessionInfo)
		}
		return nil, healthy
	}

	if c.breaker.RecordSuccess() {
		config.Log().WithField("job_router_url", disco.ServerURL).
			Info("Job router recovered, resuming routing")
	}

	return parseJobResponse(job, responseMD, disco, requestCorrelationID, config)
}

// handleRouterError records a breaker outcome for a failed GetJob and reports how
// the caller should respond: fallback=true means serve the request from Rails,
// otherwise return (nil, healthy).
func (c *Client) handleRouterError(err error, disco *common.RouterDiscovery, config common.RunnerConfig) (healthy, fallback bool) {
	switch code := status.Code(err); {
	case code == codes.Unimplemented:
		// Job router deliberately disabled. The router answered, so the breaker is
		// fine; drop the cached router to force re-discovery, then fall back to Rails.
		c.breaker.RecordSuccess()
		c.invalidateRouterDiscovery()
		config.Log().Info("Job router is disabled, falling back to direct job requests")
		return false, true
	case isRouterFailure(code):
		// The router is unreachable: count it toward the breaker. On trip, fall back;
		// below the threshold, treat it as transient and keep polling the router.
		if c.breaker.RecordFailure() {
			config.Log().WithError(err).WithField("job_router_url", disco.ServerURL).
				Warn("Job router circuit breaker tripped, falling back to direct job requests")
			return false, true
		}
		config.Log().WithError(err).Error("Error requesting a job")
		return true, false
	default:
		// The router responded (it is reachable), so the breaker is satisfied even
		// though the request itself failed.
		c.breaker.RecordSuccess()
		config.Log().WithError(err).Error("Error requesting a job")
		return false, false
	}
}

// parseJobResponse decodes a successful GetJob response into a job. A response
// with no payload means no job was available (healthy, keep polling).
func parseJobResponse(job *rpc.GetJobResponse, responseMD metadata.MD, disco *common.RouterDiscovery, requestCorrelationID string, config common.RunnerConfig) (*spec.Job, bool) {
	if len(job.JobResponse) == 0 {
		return nil, true
	}
	var response spec.Job
	if err := json.Unmarshal(job.JobResponse, &response); err != nil {
		config.Log().WithError(err).Error("json.Unmarshal()")
		return nil, false
	}
	response.TLSData = disco.TLSData
	if correlationIDs := responseMD[requestIDMetadataKey]; len(correlationIDs) > 0 {
		requestCorrelationID = correlationIDs[0]
	}
	response.JobRequestCorrelationID = requestCorrelationID
	return &response, true
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

func (c *Client) invalidateRouterDiscovery() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disco = nil
	c.discoExpiresAt = time.Time{}
}
