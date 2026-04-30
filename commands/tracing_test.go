//go:build !integration

package commands

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	oteltrace "go.opentelemetry.io/otel/trace"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

func TestTracerContext(t *testing.T) {
	log := logrus.WithFields(nil)

	t.Run("nil tracing feature returns original context", func(t *testing.T) {
		baseCtx := t.Context()
		ctx := tracerContext(baseCtx, log, nil)
		assert.Equal(t, baseCtx, ctx)
	})

	t.Run("invalid trace ID returns original context", func(t *testing.T) {
		baseCtx := t.Context()
		ctx := tracerContext(baseCtx, log, &spec.Tracing{
			TraceID: "not-a-valid-hex",
		})
		assert.Equal(t, baseCtx, ctx)
	})

	t.Run("valid trace ID and span parent ID sets both", func(t *testing.T) {
		baseCtx := t.Context()
		traceID := "0000000000000000000000000000abcd"
		spanID := "000000000000abcd"
		ctx := tracerContext(baseCtx, log, &spec.Tracing{
			TraceID:      traceID,
			SpanParentID: spanID,
		})
		sc := oteltrace.SpanFromContext(ctx).SpanContext()
		assert.Equal(t, traceID, sc.TraceID().String())
		assert.Equal(t, spanID, sc.SpanID().String())
	})
}

func TestTraceProviderForURLs(t *testing.T) {
	log := logrus.WithFields(nil)

	t.Run("no endpoints returns nil", func(t *testing.T) {
		tp := traceProviderForURLs(log, nil)
		assert.Nil(t, tp)
	})

	t.Run("invalid URL returns nil", func(t *testing.T) {
		endpoints := []spec.OTELEndpoint{{URL: "://invalid"}}
		tp := traceProviderForURLs(log, endpoints)
		assert.Nil(t, tp)
	})

	t.Run("unsupported scheme returns nil", func(t *testing.T) {
		endpoints := []spec.OTELEndpoint{{URL: "ftp://localhost:4318"}}
		tp := traceProviderForURLs(log, endpoints)
		assert.Nil(t, tp)
	})

	for _, scheme := range []string{"http", "https", "grpc", "grpcs"} {
		t.Run("scheme "+scheme+" without auth returns non-nil", func(t *testing.T) {
			endpoints := []spec.OTELEndpoint{{URL: scheme + "://localhost:4318/v1/traces"}}
			tp := traceProviderForURLs(log, endpoints)
			require.NotNil(t, tp)
			_ = tp.Shutdown(t.Context())
		})
	}

	t.Run("unsupported auth type returns nil", func(t *testing.T) {
		endpoints := []spec.OTELEndpoint{{
			URL:  "http://localhost:4318",
			Auth: &spec.OTELEndpointAuth{Type: "unsupported_type"},
		}}
		tp := traceProviderForURLs(log, endpoints)
		assert.Nil(t, tp)
	})

	t.Run("http_bearer_gcp_oidc with nil config returns nil", func(t *testing.T) {
		endpoints := []spec.OTELEndpoint{{
			URL: "http://localhost:4318",
			Auth: &spec.OTELEndpointAuth{
				Type:              "http_bearer_gcp_oidc",
				HTTPBearerGCPOIDC: nil,
			},
		}}
		tp := traceProviderForURLs(log, endpoints)
		assert.Nil(t, tp)
	})

	t.Run("multiple endpoints with one skipped returns non-nil", func(t *testing.T) {
		endpoints := []spec.OTELEndpoint{
			{URL: "http://localhost:4318"},
			{URL: "ftp://invalid"},
		}
		tp := traceProviderForURLs(log, endpoints)
		require.NotNil(t, tp)
		_ = tp.Shutdown(t.Context())
	})

	t.Run("multiple valid endpoints returns non-nil", func(t *testing.T) {
		endpoints := []spec.OTELEndpoint{
			{URL: "http://localhost:4318"},
			{URL: "grpc://localhost:4317"},
		}
		tp := traceProviderForURLs(log, endpoints)
		require.NotNil(t, tp)
		_ = tp.Shutdown(t.Context())
	})
}
