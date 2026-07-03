package snowplow

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	labkitsnowplow "gitlab.com/gitlab-org/labkit/v2/events/snowplow"
	"gitlab.com/gitlab-org/labkit/v2/events/snowplow/metrics"
	"gitlab.com/gitlab-org/labkit/v2/events/snowplow/oidc"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/usage_log"
)

const (
	eventType     = "runner_compute_usage"
	unitOfMeasure = "seconds"
)

type Writer struct {
	options options
	emitter *labkitsnowplow.Emitter
	tracker *labkitsnowplow.Tracker
	log     logrus.FieldLogger
}

func New(log logrus.FieldLogger, collectorURI string, o ...Option) (*Writer, error) {
	opts := setupOptions(collectorURI, o...)

	var emitterOpts []labkitsnowplow.EmitterOption

	if opts.callback != nil {
		emitterOpts = append(emitterOpts, labkitsnowplow.WithCallback(opts.callback))
	}

	if opts.enableOIDC {
		oidcOpts := opts.oidcOptions
		if len(oidcOpts) == 0 {
			// Default: skip if not running on a supported cloud provider
			oidcOpts = []oidc.Option{oidc.WithSkipIfUnsupportedCloud(true)}
		}
		ts := oidc.NewSource(oidcOpts...)
		emitterOpts = append(emitterOpts, labkitsnowplow.WithTokenSource(ts))
	}

	metricsOptions := []metrics.CollectorOption{
		metrics.WithCollectorNamespace("gitlab_runner"),
	}

	if len(opts.batchDeliveryDurationBuckets) > 0 {
		metricsOptions = append(metricsOptions, metrics.WithCollectorBatchDeliveryDurationBuckets(opts.batchDeliveryDurationBuckets))
	}

	emitterOpts = append(emitterOpts, labkitsnowplow.WithMetricsCollectorOptions(metricsOptions...))

	emitter, err := labkitsnowplow.NewEmitter(collectorURI, emitterOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating snowplow emitter: %w", err)
	}

	tracker, err := labkitsnowplow.NewTracker(opts.appID, emitter)
	if err != nil {
		return nil, fmt.Errorf("creating snowplow tracker: %w", err)
	}

	if opts.promRegistry != nil {
		err = opts.promRegistry.Register(emitter.Collector())
		if err != nil {
			return nil, fmt.Errorf("registering snowplow emitter collector: %w", err)
		}
	}

	return &Writer{
		options: opts,
		emitter: emitter,
		tracker: tracker,
		log:     log,
	}, nil
}

// Store queues a billing event for async delivery to the Snowplow collector.
// Returns an error if the writer has been closed (emitter stopped).
//
// Records whose Job.FailureReason matches the value configured via
// WithSkipFailureReason are dropped silently. The caller decides which
// reason (if any) to filter on; this is typically used to drop
// runner_system_failure jobs that are not billable.
func (w *Writer) Store(record usage_log.Record) error {
	if w.options.skipFailureReason != "" && record.Job.FailureReason == w.options.skipFailureReason {
		w.log.WithFields(logrus.Fields{
			"job_id":         record.Job.ID,
			"job_url":        record.Job.URL,
			"failure_reason": record.Job.FailureReason,
			"runner_uuid":    record.Runner.UUID,
		}).Debug("Skipping billing event")
		return nil
	}

	requiredInput, optionalInput := w.buildBillingInputs(record)

	if err := w.tracker.TrackBillingEventWithOptionalInput(requiredInput, optionalInput); err != nil {
		return fmt.Errorf("tracking billing event: %w", err)
	}

	return nil
}

func (w *Writer) buildBillingInputs(record usage_log.Record) (labkitsnowplow.BillingEventRequiredInput, labkitsnowplow.BillingEventOptionalInput) {
	required := labkitsnowplow.BillingEventRequiredInput{
		Category:      w.options.category,
		EventType:     eventType,
		Realm:         w.options.realm,
		UnitOfMeasure: unitOfMeasure,
		Quantity:      record.Job.DurationSeconds,
	}

	// Build metadata: config metadata (static), then per-job fields.
	metadata := make(map[string]any, len(w.options.metadata)+8)
	for k, v := range w.options.metadata {
		metadata[k] = v
	}
	metadata["job_id"] = record.Job.ID
	metadata["pipeline_id"] = record.Job.PipelineID
	metadata["job_status"] = record.Job.Status
	metadata["job_failure_reason"] = record.Job.FailureReason
	metadata["ref"] = record.Job.Ref
	metadata["started_at"] = record.Job.StartedAt.Format(time.RFC3339Nano)
	metadata["finished_at"] = record.Job.FinishedAt.Format(time.RFC3339Nano)
	metadata["executor"] = record.Runner.Executor

	subject := "0"
	if record.Runner.UUID != "" {
		subject = record.Runner.UUID
	}

	optional := labkitsnowplow.BillingEventOptionalInput{
		Subject:        subject,
		DeploymentType: w.options.deploymentType,
		Metadata:       metadata,
	}

	if record.Job.Instance.ID != "" {
		optional.InstanceID = record.Job.Instance.ID
	}
	if record.Job.Instance.UniqueID != "" {
		optional.UniqueInstanceID = record.Job.Instance.UniqueID
	}
	if record.Job.Project.ID != 0 {
		optional.ProjectID = record.Job.Project.ID
	}
	if record.Job.Namespace.ID != 0 {
		optional.NamespaceID = record.Job.Namespace.ID
	}
	if record.Job.RootNamespace.ID != 0 {
		optional.RootNamespaceID = record.Job.RootNamespace.ID
	}
	if record.Job.Organization.ID != 0 {
		optional.OrganizationID = record.Job.Organization.ID
	}

	return required, optional
}

// Close flushes pending events and stops the emitter.
// Safe to call multiple times (labkit Emitter.Stop is idempotent via sync.Once).
func (w *Writer) Close() error {
	w.log.Debug("Stopping snowplow tracker")
	w.tracker.Stop()

	if w.options.promRegistry != nil {
		w.options.promRegistry.Unregister(w.emitter.Collector())
	}

	return nil
}
