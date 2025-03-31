// Code generated by mockery v2.53.3. DO NOT EDIT.

package service

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockAkeyless is an autogenerated mock type for the Akeyless type
type MockAkeyless struct {
	mock.Mock
}

type MockAkeyless_Expecter struct {
	mock *mock.Mock
}

func (_m *MockAkeyless) EXPECT() *MockAkeyless_Expecter {
	return &MockAkeyless_Expecter{mock: &_m.Mock}
}

// GetSecret provides a mock function with given fields: ctx
func (_m *MockAkeyless) GetSecret(ctx context.Context) (interface{}, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for GetSecret")
	}

	var r0 interface{}
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (interface{}, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) interface{}); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockAkeyless_GetSecret_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetSecret'
type MockAkeyless_GetSecret_Call struct {
	*mock.Call
}

// GetSecret is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockAkeyless_Expecter) GetSecret(ctx interface{}) *MockAkeyless_GetSecret_Call {
	return &MockAkeyless_GetSecret_Call{Call: _e.mock.On("GetSecret", ctx)}
}

func (_c *MockAkeyless_GetSecret_Call) Run(run func(ctx context.Context)) *MockAkeyless_GetSecret_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockAkeyless_GetSecret_Call) Return(_a0 interface{}, _a1 error) *MockAkeyless_GetSecret_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockAkeyless_GetSecret_Call) RunAndReturn(run func(context.Context) (interface{}, error)) *MockAkeyless_GetSecret_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockAkeyless creates a new instance of MockAkeyless. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockAkeyless(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockAkeyless {
	mock := &MockAkeyless{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
