// Code generated by mockery v2.53.3. DO NOT EDIT.

package executors

import (
	context "context"
	net "net"

	mock "github.com/stretchr/testify/mock"
)

// MockClient is an autogenerated mock type for the Client type
type MockClient struct {
	mock.Mock
}

type MockClient_Expecter struct {
	mock *mock.Mock
}

func (_m *MockClient) EXPECT() *MockClient_Expecter {
	return &MockClient_Expecter{mock: &_m.Mock}
}

// Close provides a mock function with no fields
func (_m *MockClient) Close() error {
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

// MockClient_Close_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Close'
type MockClient_Close_Call struct {
	*mock.Call
}

// Close is a helper method to define mock.On call
func (_e *MockClient_Expecter) Close() *MockClient_Close_Call {
	return &MockClient_Close_Call{Call: _e.mock.On("Close")}
}

func (_c *MockClient_Close_Call) Run(run func()) *MockClient_Close_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockClient_Close_Call) Return(_a0 error) *MockClient_Close_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClient_Close_Call) RunAndReturn(run func() error) *MockClient_Close_Call {
	_c.Call.Return(run)
	return _c
}

// Dial provides a mock function with given fields: n, addr
func (_m *MockClient) Dial(n string, addr string) (net.Conn, error) {
	ret := _m.Called(n, addr)

	if len(ret) == 0 {
		panic("no return value specified for Dial")
	}

	var r0 net.Conn
	var r1 error
	if rf, ok := ret.Get(0).(func(string, string) (net.Conn, error)); ok {
		return rf(n, addr)
	}
	if rf, ok := ret.Get(0).(func(string, string) net.Conn); ok {
		r0 = rf(n, addr)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(net.Conn)
		}
	}

	if rf, ok := ret.Get(1).(func(string, string) error); ok {
		r1 = rf(n, addr)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockClient_Dial_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Dial'
type MockClient_Dial_Call struct {
	*mock.Call
}

// Dial is a helper method to define mock.On call
//   - n string
//   - addr string
func (_e *MockClient_Expecter) Dial(n interface{}, addr interface{}) *MockClient_Dial_Call {
	return &MockClient_Dial_Call{Call: _e.mock.On("Dial", n, addr)}
}

func (_c *MockClient_Dial_Call) Run(run func(n string, addr string)) *MockClient_Dial_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string), args[1].(string))
	})
	return _c
}

func (_c *MockClient_Dial_Call) Return(_a0 net.Conn, _a1 error) *MockClient_Dial_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockClient_Dial_Call) RunAndReturn(run func(string, string) (net.Conn, error)) *MockClient_Dial_Call {
	_c.Call.Return(run)
	return _c
}

// DialRun provides a mock function with given fields: _a0, _a1
func (_m *MockClient) DialRun(_a0 context.Context, _a1 string) (net.Conn, error) {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for DialRun")
	}

	var r0 net.Conn
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (net.Conn, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) net.Conn); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(net.Conn)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockClient_DialRun_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DialRun'
type MockClient_DialRun_Call struct {
	*mock.Call
}

// DialRun is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 string
func (_e *MockClient_Expecter) DialRun(_a0 interface{}, _a1 interface{}) *MockClient_DialRun_Call {
	return &MockClient_DialRun_Call{Call: _e.mock.On("DialRun", _a0, _a1)}
}

func (_c *MockClient_DialRun_Call) Run(run func(_a0 context.Context, _a1 string)) *MockClient_DialRun_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockClient_DialRun_Call) Return(_a0 net.Conn, _a1 error) *MockClient_DialRun_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockClient_DialRun_Call) RunAndReturn(run func(context.Context, string) (net.Conn, error)) *MockClient_DialRun_Call {
	_c.Call.Return(run)
	return _c
}

// Run provides a mock function with given fields: _a0, _a1
func (_m *MockClient) Run(_a0 context.Context, _a1 RunOptions) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for Run")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, RunOptions) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockClient_Run_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Run'
type MockClient_Run_Call struct {
	*mock.Call
}

// Run is a helper method to define mock.On call
//   - _a0 context.Context
//   - _a1 RunOptions
func (_e *MockClient_Expecter) Run(_a0 interface{}, _a1 interface{}) *MockClient_Run_Call {
	return &MockClient_Run_Call{Call: _e.mock.On("Run", _a0, _a1)}
}

func (_c *MockClient_Run_Call) Run(run func(_a0 context.Context, _a1 RunOptions)) *MockClient_Run_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(RunOptions))
	})
	return _c
}

func (_c *MockClient_Run_Call) Return(_a0 error) *MockClient_Run_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockClient_Run_Call) RunAndReturn(run func(context.Context, RunOptions) error) *MockClient_Run_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockClient creates a new instance of MockClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockClient {
	mock := &MockClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
