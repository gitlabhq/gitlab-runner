package snowplow

import (
	"github.com/prometheus/client_golang/prometheus"
	labkitsnowplow "gitlab.com/gitlab-org/labkit/v2/events/snowplow"
	"gitlab.com/gitlab-org/labkit/v2/events/snowplow/oidc"
)

type options struct {
	collectorURI string
	appID        string

	// category is the Snowplow structured event category for billing events.
	// Required by the labkit billing API.
	category string

	// realm identifies the deployment type: "SaaS", "Dedicated", or "SM"
	realm string

	// deploymentType is the deployment_type field in the billing schema.
	// Valid values: "dedicated", ".com", "self-managed"
	deploymentType string

	// metadata is merged into every event's metadata map.
	// Useful for static fields like runner_type, job_environment_*, etc.
	metadata map[string]string

	oidcOptions []oidc.Option
	enableOIDC  bool

	// skipFailureReason, when non-empty, causes records whose
	// Job.FailureReason matches this value to be dropped before being sent
	// to the collector. The caller decides which reason to filter on so
	// the writer stays decoupled from runner-specific failure constants.
	skipFailureReason string

	// callback is called after each send loop with aggregated results.
	callback labkitsnowplow.SendCallback

	promRegistry *prometheus.Registry

	batchDeliveryDurationBuckets []float64
}

type Option func(*options)

func setupOptions(collectorURI string, o ...Option) options {
	opts := options{
		collectorURI:                 collectorURI,
		appID:                        "gitlab-runner",
		category:                     "runner_compute_usage",
		realm:                        "SM",
		deploymentType:               "self-managed",
		batchDeliveryDurationBuckets: []float64{0.1, 0.5, 1, 2.5, 5, 10, 30, 60},
	}

	for _, opt := range o {
		opt(&opts)
	}

	return opts
}

func WithAppID(appID string) Option {
	return func(o *options) {
		o.appID = appID
	}
}

func WithCategory(category string) Option {
	return func(o *options) {
		o.category = category
	}
}

func WithRealm(realm string) Option {
	return func(o *options) {
		o.realm = realm
	}
}

func WithDeploymentType(deploymentType string) Option {
	return func(o *options) {
		o.deploymentType = deploymentType
	}
}

func WithMetadata(metadata map[string]string) Option {
	return func(o *options) {
		o.metadata = metadata
	}
}

// WithOIDC enables OIDC-based authentication for requests to the Snowplow collector.
// The OIDC token source auto-detects the cloud provider (AWS or GCP) and generates
// ID tokens accordingly.
func WithOIDC(oidcOpts ...oidc.Option) Option {
	return func(o *options) {
		o.enableOIDC = true
		o.oidcOptions = oidcOpts
	}
}

func WithCallback(cb labkitsnowplow.SendCallback) Option {
	return func(o *options) {
		o.callback = cb
	}
}

// WithSkipFailureReason causes records whose Job.FailureReason equals the
// given value to be dropped before they are sent to the collector. Useful
// for filtering out non-billable jobs (e.g. runner_system_failure). An
// empty value disables the filter.
func WithSkipFailureReason(reason string) Option {
	return func(o *options) {
		o.skipFailureReason = reason
	}
}

func WithPrometheusRegistry(r *prometheus.Registry) Option {
	return func(o *options) {
		o.promRegistry = r
	}
}

func WithBatchDeliveryDurationBuckets(buckets []float64) Option {
	return func(o *options) {
		o.batchDeliveryDurationBuckets = buckets
	}
}
