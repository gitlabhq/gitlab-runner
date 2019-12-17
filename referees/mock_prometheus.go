package referees

import (
	"context"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/mock"
)

type MockPrometheusAPI struct {
	mock.Mock
}

type MockPrometheusValue struct {
	mock.Mock
}

type MockPrometheusMatrix struct {
	mock.Mock
}

func (mpi *MockPrometheusAPI) Alerts(ctx context.Context) (v1.AlertsResult, error) {
	return v1.AlertsResult{}, nil
}

func (mpi *MockPrometheusAPI) AlertManagers(ctx context.Context) (v1.AlertManagersResult, error) {
	return v1.AlertManagersResult{}, nil
}

func (mpi *MockPrometheusAPI) CleanTombstones(ctx context.Context) error {
	return nil
}

func (mpi *MockPrometheusAPI) Config(ctx context.Context) (v1.ConfigResult, error) {
	return v1.ConfigResult{}, nil
}

func (mpi *MockPrometheusAPI) DeleteSeries(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) error {
	return nil
}

func (mpi *MockPrometheusAPI) Flags(ctx context.Context) (v1.FlagsResult, error) {
	return nil, nil
}

func (mpi *MockPrometheusAPI) LabelNames(ctx context.Context) ([]string, api.Warnings, error) {
	return nil, nil, nil
}

func (mpi *MockPrometheusAPI) LabelValues(ctx context.Context, label string) (model.LabelValues, api.Warnings, error) {
	return nil, nil, nil
}

func (mpi *MockPrometheusAPI) Query(ctx context.Context, query string, ts time.Time) (model.Value, api.Warnings, error) {
	return nil, nil, nil
}

func (mpi *MockPrometheusAPI) QueryRange(ctx context.Context, query string, r v1.Range) (model.Value, api.Warnings, error) {
	args := mpi.Called(ctx, query, r)
	return args.Get(0).(model.Value), args.Get(1).(api.Warnings), args.Error(2)
}

func (mpi *MockPrometheusAPI) Rules(ctx context.Context) (v1.RulesResult, error) {
	return v1.RulesResult{}, nil
}

func (mpi *MockPrometheusAPI) Series(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) ([]model.LabelSet, api.Warnings, error) {
	return nil, nil, nil
}

func (mpi *MockPrometheusAPI) Snapshot(ctx context.Context, skipHead bool) (v1.SnapshotResult, error) {
	return v1.SnapshotResult{}, nil
}

func (mpi *MockPrometheusAPI) Targets(ctx context.Context) (v1.TargetsResult, error) {
	return v1.TargetsResult{}, nil
}

func (mpi *MockPrometheusAPI) TargetsMetadata(ctx context.Context, matchTarget string, metric string, limit string) ([]v1.MetricMetadata, error) {
	return nil, nil
}

func (mpv *MockPrometheusValue) String() string {
	return ""
}

func (mpv *MockPrometheusValue) Type() model.ValueType {
	return model.ValueType(1)
}

func (mpm *MockPrometheusMatrix) String() string {
	return ""
}

func (mpm *MockPrometheusMatrix) Type() model.ValueType {
	return model.ValueType(1)
}
