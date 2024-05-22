// Code generated by mockery v2.43.0. DO NOT EDIT.

package process

import mock "github.com/stretchr/testify/mock"

// MockKillWaiter is an autogenerated mock type for the KillWaiter type
type MockKillWaiter struct {
	mock.Mock
}

// KillAndWait provides a mock function with given fields: command, waitCh
func (_m *MockKillWaiter) KillAndWait(command Commander, waitCh chan error) error {
	ret := _m.Called(command, waitCh)

	if len(ret) == 0 {
		panic("no return value specified for KillAndWait")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(Commander, chan error) error); ok {
		r0 = rf(command, waitCh)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewMockKillWaiter creates a new instance of MockKillWaiter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockKillWaiter(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockKillWaiter {
	mock := &MockKillWaiter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
