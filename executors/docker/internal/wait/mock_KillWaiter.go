// Code generated by mockery v1.1.0. DO NOT EDIT.

package wait

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockKillWaiter is an autogenerated mock type for the KillWaiter type
type MockKillWaiter struct {
	mock.Mock
}

// KillWait provides a mock function with given fields: ctx, containerID
func (_m *MockKillWaiter) KillWait(ctx context.Context, containerID string) error {
	ret := _m.Called(ctx, containerID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, containerID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Wait provides a mock function with given fields: ctx, containerID
func (_m *MockKillWaiter) Wait(ctx context.Context, containerID string) error {
	ret := _m.Called(ctx, containerID)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, containerID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
