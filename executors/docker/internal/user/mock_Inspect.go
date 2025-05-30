// Code generated by mockery v2.53.3. DO NOT EDIT.

package user

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockInspect is an autogenerated mock type for the Inspect type
type MockInspect struct {
	mock.Mock
}

type MockInspect_Expecter struct {
	mock *mock.Mock
}

func (_m *MockInspect) EXPECT() *MockInspect_Expecter {
	return &MockInspect_Expecter{mock: &_m.Mock}
}

// GID provides a mock function with given fields: ctx, containerID
func (_m *MockInspect) GID(ctx context.Context, containerID string) (int, error) {
	ret := _m.Called(ctx, containerID)

	if len(ret) == 0 {
		panic("no return value specified for GID")
	}

	var r0 int
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (int, error)); ok {
		return rf(ctx, containerID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) int); ok {
		r0 = rf(ctx, containerID)
	} else {
		r0 = ret.Get(0).(int)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, containerID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockInspect_GID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GID'
type MockInspect_GID_Call struct {
	*mock.Call
}

// GID is a helper method to define mock.On call
//   - ctx context.Context
//   - containerID string
func (_e *MockInspect_Expecter) GID(ctx interface{}, containerID interface{}) *MockInspect_GID_Call {
	return &MockInspect_GID_Call{Call: _e.mock.On("GID", ctx, containerID)}
}

func (_c *MockInspect_GID_Call) Run(run func(ctx context.Context, containerID string)) *MockInspect_GID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockInspect_GID_Call) Return(_a0 int, _a1 error) *MockInspect_GID_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockInspect_GID_Call) RunAndReturn(run func(context.Context, string) (int, error)) *MockInspect_GID_Call {
	_c.Call.Return(run)
	return _c
}

// IsRoot provides a mock function with given fields: ctx, imageID
func (_m *MockInspect) IsRoot(ctx context.Context, imageID string) (bool, error) {
	ret := _m.Called(ctx, imageID)

	if len(ret) == 0 {
		panic("no return value specified for IsRoot")
	}

	var r0 bool
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (bool, error)); ok {
		return rf(ctx, imageID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) bool); ok {
		r0 = rf(ctx, imageID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, imageID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockInspect_IsRoot_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'IsRoot'
type MockInspect_IsRoot_Call struct {
	*mock.Call
}

// IsRoot is a helper method to define mock.On call
//   - ctx context.Context
//   - imageID string
func (_e *MockInspect_Expecter) IsRoot(ctx interface{}, imageID interface{}) *MockInspect_IsRoot_Call {
	return &MockInspect_IsRoot_Call{Call: _e.mock.On("IsRoot", ctx, imageID)}
}

func (_c *MockInspect_IsRoot_Call) Run(run func(ctx context.Context, imageID string)) *MockInspect_IsRoot_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockInspect_IsRoot_Call) Return(_a0 bool, _a1 error) *MockInspect_IsRoot_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockInspect_IsRoot_Call) RunAndReturn(run func(context.Context, string) (bool, error)) *MockInspect_IsRoot_Call {
	_c.Call.Return(run)
	return _c
}

// UID provides a mock function with given fields: ctx, containerID
func (_m *MockInspect) UID(ctx context.Context, containerID string) (int, error) {
	ret := _m.Called(ctx, containerID)

	if len(ret) == 0 {
		panic("no return value specified for UID")
	}

	var r0 int
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (int, error)); ok {
		return rf(ctx, containerID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) int); ok {
		r0 = rf(ctx, containerID)
	} else {
		r0 = ret.Get(0).(int)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, containerID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockInspect_UID_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UID'
type MockInspect_UID_Call struct {
	*mock.Call
}

// UID is a helper method to define mock.On call
//   - ctx context.Context
//   - containerID string
func (_e *MockInspect_Expecter) UID(ctx interface{}, containerID interface{}) *MockInspect_UID_Call {
	return &MockInspect_UID_Call{Call: _e.mock.On("UID", ctx, containerID)}
}

func (_c *MockInspect_UID_Call) Run(run func(ctx context.Context, containerID string)) *MockInspect_UID_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string))
	})
	return _c
}

func (_c *MockInspect_UID_Call) Return(_a0 int, _a1 error) *MockInspect_UID_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockInspect_UID_Call) RunAndReturn(run func(context.Context, string) (int, error)) *MockInspect_UID_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockInspect creates a new instance of MockInspect. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockInspect(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockInspect {
	mock := &MockInspect{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
