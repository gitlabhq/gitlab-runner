// Code generated by mockery; DO NOT EDIT.
// github.com/vektra/mockery
// template: testify

package networks

import (
	"context"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	mock "github.com/stretchr/testify/mock"
)

// NewMockManager creates a new instance of MockManager. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockManager(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockManager {
	mock := &MockManager{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// MockManager is an autogenerated mock type for the Manager type
type MockManager struct {
	mock.Mock
}

type MockManager_Expecter struct {
	mock *mock.Mock
}

func (_m *MockManager) EXPECT() *MockManager_Expecter {
	return &MockManager_Expecter{mock: &_m.Mock}
}

// Cleanup provides a mock function for the type MockManager
func (_mock *MockManager) Cleanup(ctx context.Context) error {
	ret := _mock.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Cleanup")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = returnFunc(ctx)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// MockManager_Cleanup_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Cleanup'
type MockManager_Cleanup_Call struct {
	*mock.Call
}

// Cleanup is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockManager_Expecter) Cleanup(ctx interface{}) *MockManager_Cleanup_Call {
	return &MockManager_Cleanup_Call{Call: _e.mock.On("Cleanup", ctx)}
}

func (_c *MockManager_Cleanup_Call) Run(run func(ctx context.Context)) *MockManager_Cleanup_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		run(
			arg0,
		)
	})
	return _c
}

func (_c *MockManager_Cleanup_Call) Return(err error) *MockManager_Cleanup_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *MockManager_Cleanup_Call) RunAndReturn(run func(ctx context.Context) error) *MockManager_Cleanup_Call {
	_c.Call.Return(run)
	return _c
}

// Create provides a mock function for the type MockManager
func (_mock *MockManager) Create(ctx context.Context, networkMode string, enableIPv6 bool) (container.NetworkMode, error) {
	ret := _mock.Called(ctx, networkMode, enableIPv6)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 container.NetworkMode
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(context.Context, string, bool) (container.NetworkMode, error)); ok {
		return returnFunc(ctx, networkMode, enableIPv6)
	}
	if returnFunc, ok := ret.Get(0).(func(context.Context, string, bool) container.NetworkMode); ok {
		r0 = returnFunc(ctx, networkMode, enableIPv6)
	} else {
		r0 = ret.Get(0).(container.NetworkMode)
	}
	if returnFunc, ok := ret.Get(1).(func(context.Context, string, bool) error); ok {
		r1 = returnFunc(ctx, networkMode, enableIPv6)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// MockManager_Create_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Create'
type MockManager_Create_Call struct {
	*mock.Call
}

// Create is a helper method to define mock.On call
//   - ctx context.Context
//   - networkMode string
//   - enableIPv6 bool
func (_e *MockManager_Expecter) Create(ctx interface{}, networkMode interface{}, enableIPv6 interface{}) *MockManager_Create_Call {
	return &MockManager_Create_Call{Call: _e.mock.On("Create", ctx, networkMode, enableIPv6)}
}

func (_c *MockManager_Create_Call) Run(run func(ctx context.Context, networkMode string, enableIPv6 bool)) *MockManager_Create_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		var arg1 string
		if args[1] != nil {
			arg1 = args[1].(string)
		}
		var arg2 bool
		if args[2] != nil {
			arg2 = args[2].(bool)
		}
		run(
			arg0,
			arg1,
			arg2,
		)
	})
	return _c
}

func (_c *MockManager_Create_Call) Return(networkMode1 container.NetworkMode, err error) *MockManager_Create_Call {
	_c.Call.Return(networkMode1, err)
	return _c
}

func (_c *MockManager_Create_Call) RunAndReturn(run func(ctx context.Context, networkMode string, enableIPv6 bool) (container.NetworkMode, error)) *MockManager_Create_Call {
	_c.Call.Return(run)
	return _c
}

// Inspect provides a mock function for the type MockManager
func (_mock *MockManager) Inspect(ctx context.Context) (network.Inspect, error) {
	ret := _mock.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Inspect")
	}

	var r0 network.Inspect
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(context.Context) (network.Inspect, error)); ok {
		return returnFunc(ctx)
	}
	if returnFunc, ok := ret.Get(0).(func(context.Context) network.Inspect); ok {
		r0 = returnFunc(ctx)
	} else {
		r0 = ret.Get(0).(network.Inspect)
	}
	if returnFunc, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = returnFunc(ctx)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// MockManager_Inspect_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Inspect'
type MockManager_Inspect_Call struct {
	*mock.Call
}

// Inspect is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockManager_Expecter) Inspect(ctx interface{}) *MockManager_Inspect_Call {
	return &MockManager_Inspect_Call{Call: _e.mock.On("Inspect", ctx)}
}

func (_c *MockManager_Inspect_Call) Run(run func(ctx context.Context)) *MockManager_Inspect_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 context.Context
		if args[0] != nil {
			arg0 = args[0].(context.Context)
		}
		run(
			arg0,
		)
	})
	return _c
}

func (_c *MockManager_Inspect_Call) Return(inspect network.Inspect, err error) *MockManager_Inspect_Call {
	_c.Call.Return(inspect, err)
	return _c
}

func (_c *MockManager_Inspect_Call) RunAndReturn(run func(ctx context.Context) (network.Inspect, error)) *MockManager_Inspect_Call {
	_c.Call.Return(run)
	return _c
}

// newMockDebugLogger creates a new instance of mockDebugLogger. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockDebugLogger(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockDebugLogger {
	mock := &mockDebugLogger{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// mockDebugLogger is an autogenerated mock type for the debugLogger type
type mockDebugLogger struct {
	mock.Mock
}

type mockDebugLogger_Expecter struct {
	mock *mock.Mock
}

func (_m *mockDebugLogger) EXPECT() *mockDebugLogger_Expecter {
	return &mockDebugLogger_Expecter{mock: &_m.Mock}
}

// Debugln provides a mock function for the type mockDebugLogger
func (_mock *mockDebugLogger) Debugln(args ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, args...)
	_mock.Called(_ca...)
	return
}

// mockDebugLogger_Debugln_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Debugln'
type mockDebugLogger_Debugln_Call struct {
	*mock.Call
}

// Debugln is a helper method to define mock.On call
//   - args ...interface{}
func (_e *mockDebugLogger_Expecter) Debugln(args ...interface{}) *mockDebugLogger_Debugln_Call {
	return &mockDebugLogger_Debugln_Call{Call: _e.mock.On("Debugln",
		append([]interface{}{}, args...)...)}
}

func (_c *mockDebugLogger_Debugln_Call) Run(run func(args ...interface{})) *mockDebugLogger_Debugln_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 []interface{}
		variadicArgs := make([]interface{}, len(args)-0)
		for i, a := range args[0:] {
			if a != nil {
				variadicArgs[i] = a.(interface{})
			}
		}
		arg0 = variadicArgs
		run(
			arg0...,
		)
	})
	return _c
}

func (_c *mockDebugLogger_Debugln_Call) Return() *mockDebugLogger_Debugln_Call {
	_c.Call.Return()
	return _c
}

func (_c *mockDebugLogger_Debugln_Call) RunAndReturn(run func(args ...interface{})) *mockDebugLogger_Debugln_Call {
	_c.Run(run)
	return _c
}
