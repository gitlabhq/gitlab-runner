// Code generated by mockery v2.43.0. DO NOT EDIT.

package terminal

import (
	http "net/http"

	mock "github.com/stretchr/testify/mock"
)

// MockConn is an autogenerated mock type for the Conn type
type MockConn struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *MockConn) Close() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Close")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Start provides a mock function with given fields: w, r, timeoutCh, disconnectCh
func (_m *MockConn) Start(w http.ResponseWriter, r *http.Request, timeoutCh chan error, disconnectCh chan error) {
	_m.Called(w, r, timeoutCh, disconnectCh)
}

// NewMockConn creates a new instance of MockConn. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockConn(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockConn {
	mock := &MockConn{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
