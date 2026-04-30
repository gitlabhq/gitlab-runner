package commands

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

const (
	tracerName           = "gitlab-ci-runner"
	spanNameJobExecution = "job_execution"

	spanAttrJobID          attribute.Key = "ci.job.id"
	spanAttrProjectID      attribute.Key = "ci.project.id"
	spanAttrPipelineID     attribute.Key = "ci.pipeline.id"
	spanAttrPipelineSource attribute.Key = "ci.pipeline.source"
	spanAttrRunnerID       attribute.Key = "ci.runner.id"
	spanAttrRunnerExecutor attribute.Key = "ci.runner.executor"
	spanAttrJobStatus      attribute.Key = "ci.job.status"
)

func tracerContext(ctx context.Context, log *logrus.Entry, tracingFeature *spec.Tracing) context.Context {
	if tracingFeature == nil {
		return ctx
	}
	traceID, err := oteltrace.TraceIDFromHex(tracingFeature.TraceID)
	if err != nil {
		log.WithError(err).Warn("Failed to parse trace ID")
		return ctx
	}
	spanID, err := oteltrace.SpanIDFromHex(tracingFeature.SpanParentID)
	if err != nil {
		log.WithError(err).Warn("Failed to parse span ID")
		return ctx
	}
	return oteltrace.ContextWithSpanContext(ctx, oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: oteltrace.FlagsSampled, // we got the trace feature set, so presumably the server wants the Runner to trace this job.
		Remote:     true,
	}))
}

func tracer(log *logrus.Entry, tracingFeature *spec.Tracing) (oteltrace.Tracer, func() error) {
	if tracingFeature == nil || len(tracingFeature.OTELEndpoints) == 0 {
		return noop.Tracer{}, nopStop
	}
	tp := traceProviderForURLs(log, tracingFeature.OTELEndpoints)
	if tp == nil {
		return noop.Tracer{}, nopStop
	}
	tpStop := func() error { //nolint:contextcheck
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return tp.Shutdown(ctx)
	}
	return tp.Tracer(tracerName), tpStop
}

func nopStop() error {
	return nil
}

func setJobSpanAttributes(span oteltrace.Span, build *common.Build, runner *common.RunnerConfig) {
	span.SetAttributes(
		spanAttrJobID.Int64(build.ID),
		spanAttrProjectID.Int64(build.JobInfo.ProjectID),
		spanAttrPipelineID.String(build.Variables.Value("CI_PIPELINE_ID")),
		spanAttrPipelineSource.String(build.Variables.Value("CI_PIPELINE_SOURCE")),
		spanAttrRunnerID.Int64(runner.ID),
		spanAttrRunnerExecutor.String(runner.Executor),
	)
}

func traceProviderForURLs(log *logrus.Entry, endpoints []spec.OTELEndpoint) *tracesdk.TracerProvider {
	var exporters []tracesdk.SpanExporter
	for _, e := range endpoints {
		if exp := exporterForEndpoint(log, &e); exp != nil {
			exporters = append(exporters, exp)
		}
	}
	var exporter tracesdk.SpanExporter
	switch len(exporters) {
	case 0:
		return nil
	case 1:
		exporter = exporters[0]
	default:
		exporter = &observability.MultiSpanExporter{
			Exporters: exporters,
		}
	}
	return tracesdk.NewTracerProvider(
		tracesdk.WithResource(constructOTELResource()),
		tracesdk.WithBatcher(exporter),
		tracesdk.WithSampler(tracesdk.AlwaysSample()), // we got the tracing configuration - we must trace!
	)
}

//nolint:gocognit
func exporterForEndpoint(log *logrus.Entry, e *spec.OTELEndpoint) tracesdk.SpanExporter {
	u, err := url.Parse(e.URL)
	if err != nil {
		log.WithError(err).Warn("Error parsing OTEL URL")
		return nil
	}

	var otlpHTTPOptions []otlptracehttp.Option
	var otlpGRPCOptions []otlptracegrpc.Option

	switch u.Scheme {
	case "http":
		otlpHTTPOptions = []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(u.Host),
			otlptracehttp.WithURLPath(u.Path),
			otlptracehttp.WithInsecure(),
		}
	case "https":
		otlpHTTPOptions = []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(u.Host),
			otlptracehttp.WithURLPath(u.Path),
		}
	case "grpc":
		otlpGRPCOptions = []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(u.Host), // gRPC ignores the URL path, don't bother setting it.
			otlptracegrpc.WithInsecure(),
		}
	case "grpcs":
		otlpGRPCOptions = []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(u.Host), // gRPC ignores the URL path, don't bother setting it.
		}
	default:
		log.Warn("Unsupported scheme in URL: ", u.Scheme)
		return nil
	}
	if e.Auth != nil {
		switch e.Auth.Type {
		case "http_bearer_gcp_oidc":
			oidcCfg := e.Auth.HTTPBearerGCPOIDC
			if oidcCfg == nil {
				log.Warn("Missing http_bearer_gcp_oidc field for tracing URL: ", e.URL)
				return nil
			}
			credentials, err := google.FindDefaultCredentials(context.Background())
			if err != nil {
				log.WithError(err).Warn("Error finding default GCP credentials for tracing URL: ", e.URL)
				return nil
			}
			ts, err := idtoken.NewTokenSource(context.Background(), oidcCfg.Audience, option.WithCredentials(credentials))
			if err != nil {
				log.WithError(err).Warn("Error creating token source")
				return nil
			}
			ts = oauth2.ReuseTokenSource(nil, ts)
			switch u.Scheme {
			case "http", "https":
				otlpHTTPOptions = append(otlpHTTPOptions,
					otlptracehttp.WithHTTPClient(&http.Client{
						Transport: &oauth2.Transport{
							Base:   http.DefaultTransport,
							Source: ts,
						},
						CheckRedirect: func(req *http.Request, via []*http.Request) error {
							return http.ErrUseLastResponse
						},
					}),
				)
			default: // gRPC
				otlpGRPCOptions = append(otlpGRPCOptions,
					otlptracegrpc.WithDialOption(grpc.WithPerRPCCredentials(&perRPCCredentialsFromTokenSource{
						src: ts,
					})),
				)
			}
		default:
			log.Warnf("Unsupported authentication type %q for OTLP endpoint: %s", e.Auth.Type, e.URL)
			return nil
		}
	}
	var c otlptrace.Client
	if len(otlpHTTPOptions) > 0 {
		c = otlptracehttp.NewClient(otlpHTTPOptions...)
	} else {
		c = otlptracegrpc.NewClient(otlpGRPCOptions...)
	}
	exporter, err := otlptrace.New(context.Background(), c)
	if err != nil {
		log.WithError(err).Warn("Error constructing OTLP exporter")
		return nil
	}
	return exporter
}

func constructOTELResource() *resource.Resource {
	// Do not use resource.Default() as it doesn't provide anything particularly useful but leads to problems.
	// See https://github.com/open-telemetry/opentelemetry-go/issues/3769 and https://github.com/letsencrypt/boulder/pull/7712.
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName("runner"),
		semconv.ServiceVersion(common.AppVersion.Version),
	)
}

type perRPCCredentialsFromTokenSource struct {
	src oauth2.TokenSource
}

func (p *perRPCCredentialsFromTokenSource) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	t, err := p.src.Token()
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"authorization": t.Type() + " " + t.AccessToken, // metadata keys must be lowercase
	}, nil
}

func (p *perRPCCredentialsFromTokenSource) RequireTransportSecurity() bool {
	return false // it should work for insecure connections.
}
