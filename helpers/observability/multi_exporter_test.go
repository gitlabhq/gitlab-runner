//go:build !integration

package observability

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
)

type mockExporter struct {
	exportCalled   int
	shutdownCalled int
	exportErr      error
	shutdownErr    error
}

func (m *mockExporter) ExportSpans(_ context.Context, _ []tracesdk.ReadOnlySpan) error {
	m.exportCalled++
	return m.exportErr
}

func (m *mockExporter) Shutdown(_ context.Context) error {
	m.shutdownCalled++
	return m.shutdownErr
}

func TestMultiSpanExporter_ExportSpans(t *testing.T) {
	t.Run("calls all exporters", func(t *testing.T) {
		e1 := &mockExporter{}
		e2 := &mockExporter{}
		me := &MultiSpanExporter{Exporters: []tracesdk.SpanExporter{e1, e2}}

		err := me.ExportSpans(t.Context(), nil)
		require.NoError(t, err)
		assert.Equal(t, 1, e1.exportCalled)
		assert.Equal(t, 1, e2.exportCalled)
	})

	t.Run("joins errors from all exporters", func(t *testing.T) {
		err1 := errors.New("exporter 1 error")
		err2 := errors.New("exporter 2 error")
		e1 := &mockExporter{exportErr: err1}
		e2 := &mockExporter{exportErr: err2}
		me := &MultiSpanExporter{Exporters: []tracesdk.SpanExporter{e1, e2}}

		err := me.ExportSpans(t.Context(), nil)
		assert.ErrorIs(t, err, err1)
		assert.ErrorIs(t, err, err2)
	})

	t.Run("stops on cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		e1 := &mockExporter{}
		e2 := &mockExporter{}
		me := &MultiSpanExporter{Exporters: []tracesdk.SpanExporter{e1, e2}}

		err := me.ExportSpans(ctx, nil)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, e1.exportCalled)
		assert.Equal(t, 0, e2.exportCalled)
	})
}

func TestMultiSpanExporter_Shutdown(t *testing.T) {
	t.Run("calls all exporters", func(t *testing.T) {
		e1 := &mockExporter{}
		e2 := &mockExporter{}
		me := &MultiSpanExporter{Exporters: []tracesdk.SpanExporter{e1, e2}}

		err := me.Shutdown(t.Context())
		require.NoError(t, err)
		assert.Equal(t, 1, e1.shutdownCalled)
		assert.Equal(t, 1, e2.shutdownCalled)
	})

	t.Run("joins errors from all exporters", func(t *testing.T) {
		err1 := errors.New("exporter 1 error")
		err2 := errors.New("exporter 2 error")
		e1 := &mockExporter{shutdownErr: err1}
		e2 := &mockExporter{shutdownErr: err2}
		me := &MultiSpanExporter{Exporters: []tracesdk.SpanExporter{e1, e2}}

		err := me.Shutdown(t.Context())
		assert.ErrorIs(t, err, err1)
		assert.ErrorIs(t, err, err2)
	})

	t.Run("stops on cancelled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		e1 := &mockExporter{}
		e2 := &mockExporter{}
		me := &MultiSpanExporter{Exporters: []tracesdk.SpanExporter{e1, e2}}

		err := me.Shutdown(ctx)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Equal(t, 0, e1.shutdownCalled)
		assert.Equal(t, 0, e2.shutdownCalled)
	})
}
