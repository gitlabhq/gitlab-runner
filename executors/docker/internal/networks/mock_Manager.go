// Code generated by mockery v2.53.3. DO NOT EDIT.

package networks

import (
	context "context"

	container "github.com/docker/docker/api/types/container"

	mock "github.com/stretchr/testify/mock"

	network "github.com/docker/docker/api/types/network"
)

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

// Cleanup provides a mock function with given fields: ctx
func (_m *MockManager) Cleanup(ctx context.Context) error {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Cleanup")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
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
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockManager_Cleanup_Call) Return(_a0 error) *MockManager_Cleanup_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockManager_Cleanup_Call) RunAndReturn(run func(context.Context) error) *MockManager_Cleanup_Call {
	_c.Call.Return(run)
	return _c
}

// Create provides a mock function with given fields: ctx, networkMode, enableIPv6
func (_m *MockManager) Create(ctx context.Context, networkMode string, enableIPv6 bool) (container.NetworkMode, error) {
	ret := _m.Called(ctx, networkMode, enableIPv6)

	if len(ret) == 0 {
		panic("no return value specified for Create")
	}

	var r0 container.NetworkMode
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, bool) (container.NetworkMode, error)); ok {
		return rf(ctx, networkMode, enableIPv6)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, bool) container.NetworkMode); ok {
		r0 = rf(ctx, networkMode, enableIPv6)
	} else {
		r0 = ret.Get(0).(container.NetworkMode)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, bool) error); ok {
		r1 = rf(ctx, networkMode, enableIPv6)
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
		run(args[0].(context.Context), args[1].(string), args[2].(bool))
	})
	return _c
}

func (_c *MockManager_Create_Call) Return(_a0 container.NetworkMode, _a1 error) *MockManager_Create_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockManager_Create_Call) RunAndReturn(run func(context.Context, string, bool) (container.NetworkMode, error)) *MockManager_Create_Call {
	_c.Call.Return(run)
	return _c
}

// Inspect provides a mock function with given fields: ctx
func (_m *MockManager) Inspect(ctx context.Context) (network.Inspect, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Inspect")
	}

	var r0 network.Inspect
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (network.Inspect, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) network.Inspect); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(network.Inspect)
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
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
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockManager_Inspect_Call) Return(_a0 network.Inspect, _a1 error) *MockManager_Inspect_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockManager_Inspect_Call) RunAndReturn(run func(context.Context) (network.Inspect, error)) *MockManager_Inspect_Call {
	_c.Call.Return(run)
	return _c
}

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
