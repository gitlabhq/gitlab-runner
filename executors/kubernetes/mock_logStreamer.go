// Code generated by mockery v2.43.0. DO NOT EDIT.

package kubernetes

import (
	context "context"
	io "io"

	mock "github.com/stretchr/testify/mock"
)

// mockLogStreamer is an autogenerated mock type for the logStreamer type
type mockLogStreamer struct {
	mock.Mock
}

// Stream provides a mock function with given fields: ctx, offset, output
func (_m *mockLogStreamer) Stream(ctx context.Context, offset int64, output io.Writer) error {
	ret := _m.Called(ctx, offset, output)

	if len(ret) == 0 {
		panic("no return value specified for Stream")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, int64, io.Writer) error); ok {
		r0 = rf(ctx, offset, output)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// String provides a mock function with given fields:
func (_m *mockLogStreamer) String() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for String")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// newMockLogStreamer creates a new instance of mockLogStreamer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockLogStreamer(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockLogStreamer {
	mock := &mockLogStreamer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
