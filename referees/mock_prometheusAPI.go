// Code generated by mockery v2.43.0. DO NOT EDIT.

package referees

import (
	context "context"

	model "github.com/prometheus/common/model"
	mock "github.com/stretchr/testify/mock"

	time "time"

	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
)

// mockPrometheusAPI is an autogenerated mock type for the prometheusAPI type
type mockPrometheusAPI struct {
	mock.Mock
}

// AlertManagers provides a mock function with given fields: ctx
func (_m *mockPrometheusAPI) AlertManagers(ctx context.Context) (v1.AlertManagersResult, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for AlertManagers")
	}

	var r0 v1.AlertManagersResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (v1.AlertManagersResult, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) v1.AlertManagersResult); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(v1.AlertManagersResult)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Alerts provides a mock function with given fields: ctx
func (_m *mockPrometheusAPI) Alerts(ctx context.Context) (v1.AlertsResult, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Alerts")
	}

	var r0 v1.AlertsResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (v1.AlertsResult, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) v1.AlertsResult); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(v1.AlertsResult)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Buildinfo provides a mock function with given fields: ctx
func (_m *mockPrometheusAPI) Buildinfo(ctx context.Context) (v1.BuildinfoResult, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Buildinfo")
	}

	var r0 v1.BuildinfoResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (v1.BuildinfoResult, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) v1.BuildinfoResult); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(v1.BuildinfoResult)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CleanTombstones provides a mock function with given fields: ctx
func (_m *mockPrometheusAPI) CleanTombstones(ctx context.Context) error {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for CleanTombstones")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Config provides a mock function with given fields: ctx
func (_m *mockPrometheusAPI) Config(ctx context.Context) (v1.ConfigResult, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Config")
	}

	var r0 v1.ConfigResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (v1.ConfigResult, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) v1.ConfigResult); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(v1.ConfigResult)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteSeries provides a mock function with given fields: ctx, matches, startTime, endTime
func (_m *mockPrometheusAPI) DeleteSeries(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) error {
	ret := _m.Called(ctx, matches, startTime, endTime)

	if len(ret) == 0 {
		panic("no return value specified for DeleteSeries")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, []string, time.Time, time.Time) error); ok {
		r0 = rf(ctx, matches, startTime, endTime)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Flags provides a mock function with given fields: ctx
func (_m *mockPrometheusAPI) Flags(ctx context.Context) (v1.FlagsResult, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Flags")
	}

	var r0 v1.FlagsResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (v1.FlagsResult, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) v1.FlagsResult); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(v1.FlagsResult)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// LabelNames provides a mock function with given fields: ctx, matches, startTime, endTime
func (_m *mockPrometheusAPI) LabelNames(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) ([]string, v1.Warnings, error) {
	ret := _m.Called(ctx, matches, startTime, endTime)

	if len(ret) == 0 {
		panic("no return value specified for LabelNames")
	}

	var r0 []string
	var r1 v1.Warnings
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, []string, time.Time, time.Time) ([]string, v1.Warnings, error)); ok {
		return rf(ctx, matches, startTime, endTime)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []string, time.Time, time.Time) []string); ok {
		r0 = rf(ctx, matches, startTime, endTime)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []string, time.Time, time.Time) v1.Warnings); ok {
		r1 = rf(ctx, matches, startTime, endTime)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(v1.Warnings)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, []string, time.Time, time.Time) error); ok {
		r2 = rf(ctx, matches, startTime, endTime)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// LabelValues provides a mock function with given fields: ctx, label, matches, startTime, endTime
func (_m *mockPrometheusAPI) LabelValues(ctx context.Context, label string, matches []string, startTime time.Time, endTime time.Time) (model.LabelValues, v1.Warnings, error) {
	ret := _m.Called(ctx, label, matches, startTime, endTime)

	if len(ret) == 0 {
		panic("no return value specified for LabelValues")
	}

	var r0 model.LabelValues
	var r1 v1.Warnings
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, string, []string, time.Time, time.Time) (model.LabelValues, v1.Warnings, error)); ok {
		return rf(ctx, label, matches, startTime, endTime)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, []string, time.Time, time.Time) model.LabelValues); ok {
		r0 = rf(ctx, label, matches, startTime, endTime)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(model.LabelValues)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, []string, time.Time, time.Time) v1.Warnings); ok {
		r1 = rf(ctx, label, matches, startTime, endTime)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(v1.Warnings)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, string, []string, time.Time, time.Time) error); ok {
		r2 = rf(ctx, label, matches, startTime, endTime)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Metadata provides a mock function with given fields: ctx, metric, limit
func (_m *mockPrometheusAPI) Metadata(ctx context.Context, metric string, limit string) (map[string][]v1.Metadata, error) {
	ret := _m.Called(ctx, metric, limit)

	if len(ret) == 0 {
		panic("no return value specified for Metadata")
	}

	var r0 map[string][]v1.Metadata
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (map[string][]v1.Metadata, error)); ok {
		return rf(ctx, metric, limit)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) map[string][]v1.Metadata); ok {
		r0 = rf(ctx, metric, limit)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string][]v1.Metadata)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, metric, limit)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Query provides a mock function with given fields: ctx, query, ts, opts
func (_m *mockPrometheusAPI) Query(ctx context.Context, query string, ts time.Time, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, query, ts)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Query")
	}

	var r0 model.Value
	var r1 v1.Warnings
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, string, time.Time, ...v1.Option) (model.Value, v1.Warnings, error)); ok {
		return rf(ctx, query, ts, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, time.Time, ...v1.Option) model.Value); ok {
		r0 = rf(ctx, query, ts, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(model.Value)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, time.Time, ...v1.Option) v1.Warnings); ok {
		r1 = rf(ctx, query, ts, opts...)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(v1.Warnings)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, string, time.Time, ...v1.Option) error); ok {
		r2 = rf(ctx, query, ts, opts...)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// QueryExemplars provides a mock function with given fields: ctx, query, startTime, endTime
func (_m *mockPrometheusAPI) QueryExemplars(ctx context.Context, query string, startTime time.Time, endTime time.Time) ([]v1.ExemplarQueryResult, error) {
	ret := _m.Called(ctx, query, startTime, endTime)

	if len(ret) == 0 {
		panic("no return value specified for QueryExemplars")
	}

	var r0 []v1.ExemplarQueryResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, time.Time, time.Time) ([]v1.ExemplarQueryResult, error)); ok {
		return rf(ctx, query, startTime, endTime)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, time.Time, time.Time) []v1.ExemplarQueryResult); ok {
		r0 = rf(ctx, query, startTime, endTime)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]v1.ExemplarQueryResult)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, time.Time, time.Time) error); ok {
		r1 = rf(ctx, query, startTime, endTime)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// QueryRange provides a mock function with given fields: ctx, query, r, opts
func (_m *mockPrometheusAPI) QueryRange(ctx context.Context, query string, r v1.Range, opts ...v1.Option) (model.Value, v1.Warnings, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, query, r)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for QueryRange")
	}

	var r0 model.Value
	var r1 v1.Warnings
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, string, v1.Range, ...v1.Option) (model.Value, v1.Warnings, error)); ok {
		return rf(ctx, query, r, opts...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, v1.Range, ...v1.Option) model.Value); ok {
		r0 = rf(ctx, query, r, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(model.Value)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, v1.Range, ...v1.Option) v1.Warnings); ok {
		r1 = rf(ctx, query, r, opts...)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(v1.Warnings)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, string, v1.Range, ...v1.Option) error); ok {
		r2 = rf(ctx, query, r, opts...)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Rules provides a mock function with given fields: ctx
func (_m *mockPrometheusAPI) Rules(ctx context.Context) (v1.RulesResult, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Rules")
	}

	var r0 v1.RulesResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (v1.RulesResult, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) v1.RulesResult); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(v1.RulesResult)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Runtimeinfo provides a mock function with given fields: ctx
func (_m *mockPrometheusAPI) Runtimeinfo(ctx context.Context) (v1.RuntimeinfoResult, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Runtimeinfo")
	}

	var r0 v1.RuntimeinfoResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (v1.RuntimeinfoResult, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) v1.RuntimeinfoResult); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(v1.RuntimeinfoResult)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Series provides a mock function with given fields: ctx, matches, startTime, endTime
func (_m *mockPrometheusAPI) Series(ctx context.Context, matches []string, startTime time.Time, endTime time.Time) ([]model.LabelSet, v1.Warnings, error) {
	ret := _m.Called(ctx, matches, startTime, endTime)

	if len(ret) == 0 {
		panic("no return value specified for Series")
	}

	var r0 []model.LabelSet
	var r1 v1.Warnings
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, []string, time.Time, time.Time) ([]model.LabelSet, v1.Warnings, error)); ok {
		return rf(ctx, matches, startTime, endTime)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []string, time.Time, time.Time) []model.LabelSet); ok {
		r0 = rf(ctx, matches, startTime, endTime)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]model.LabelSet)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []string, time.Time, time.Time) v1.Warnings); ok {
		r1 = rf(ctx, matches, startTime, endTime)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(v1.Warnings)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, []string, time.Time, time.Time) error); ok {
		r2 = rf(ctx, matches, startTime, endTime)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// Snapshot provides a mock function with given fields: ctx, skipHead
func (_m *mockPrometheusAPI) Snapshot(ctx context.Context, skipHead bool) (v1.SnapshotResult, error) {
	ret := _m.Called(ctx, skipHead)

	if len(ret) == 0 {
		panic("no return value specified for Snapshot")
	}

	var r0 v1.SnapshotResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, bool) (v1.SnapshotResult, error)); ok {
		return rf(ctx, skipHead)
	}
	if rf, ok := ret.Get(0).(func(context.Context, bool) v1.SnapshotResult); ok {
		r0 = rf(ctx, skipHead)
	} else {
		r0 = ret.Get(0).(v1.SnapshotResult)
	}

	if rf, ok := ret.Get(1).(func(context.Context, bool) error); ok {
		r1 = rf(ctx, skipHead)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// TSDB provides a mock function with given fields: ctx
func (_m *mockPrometheusAPI) TSDB(ctx context.Context) (v1.TSDBResult, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for TSDB")
	}

	var r0 v1.TSDBResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (v1.TSDBResult, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) v1.TSDBResult); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(v1.TSDBResult)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Targets provides a mock function with given fields: ctx
func (_m *mockPrometheusAPI) Targets(ctx context.Context) (v1.TargetsResult, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Targets")
	}

	var r0 v1.TargetsResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (v1.TargetsResult, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) v1.TargetsResult); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(v1.TargetsResult)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// TargetsMetadata provides a mock function with given fields: ctx, matchTarget, metric, limit
func (_m *mockPrometheusAPI) TargetsMetadata(ctx context.Context, matchTarget string, metric string, limit string) ([]v1.MetricMetadata, error) {
	ret := _m.Called(ctx, matchTarget, metric, limit)

	if len(ret) == 0 {
		panic("no return value specified for TargetsMetadata")
	}

	var r0 []v1.MetricMetadata
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) ([]v1.MetricMetadata, error)); ok {
		return rf(ctx, matchTarget, metric, limit)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) []v1.MetricMetadata); ok {
		r0 = rf(ctx, matchTarget, metric, limit)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]v1.MetricMetadata)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, string) error); ok {
		r1 = rf(ctx, matchTarget, metric, limit)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// WalReplay provides a mock function with given fields: ctx
func (_m *mockPrometheusAPI) WalReplay(ctx context.Context) (v1.WalReplayStatus, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for WalReplay")
	}

	var r0 v1.WalReplayStatus
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (v1.WalReplayStatus, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) v1.WalReplayStatus); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(v1.WalReplayStatus)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// newMockPrometheusAPI creates a new instance of mockPrometheusAPI. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockPrometheusAPI(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockPrometheusAPI {
	mock := &mockPrometheusAPI{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
