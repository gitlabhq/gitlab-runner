package observability

import (
	"context"
	"errors"

	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

var _ tracesdk.SpanExporter = (*MultiSpanExporter)(nil)

type MultiSpanExporter struct {
	Exporters []tracesdk.SpanExporter
}

func (e *MultiSpanExporter) ExportSpans(ctx context.Context, spans []tracesdk.ReadOnlySpan) error {
	var errs []error
	for _, exporter := range e.Exporters {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		err := exporter.ExportSpans(ctx, spans)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (e *MultiSpanExporter) Shutdown(ctx context.Context) error {
	var errs []error
	for _, exporter := range e.Exporters {
		if err := ctx.Err(); err != nil {
			errs = append(errs, err)
			break
		}
		err := exporter.Shutdown(ctx)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
