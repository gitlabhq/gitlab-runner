// Code generated by mockery v2.53.3. DO NOT EDIT.

package permission

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// MockSetter is an autogenerated mock type for the Setter type
type MockSetter struct {
	mock.Mock
}

type MockSetter_Expecter struct {
	mock *mock.Mock
}

func (_m *MockSetter) EXPECT() *MockSetter_Expecter {
	return &MockSetter_Expecter{mock: &_m.Mock}
}

// Set provides a mock function with given fields: ctx, volumeName, labels
func (_m *MockSetter) Set(ctx context.Context, volumeName string, labels map[string]string) error {
	ret := _m.Called(ctx, volumeName, labels)

	if len(ret) == 0 {
		panic("no return value specified for Set")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]string) error); ok {
		r0 = rf(ctx, volumeName, labels)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockSetter_Set_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Set'
type MockSetter_Set_Call struct {
	*mock.Call
}

// Set is a helper method to define mock.On call
//   - ctx context.Context
//   - volumeName string
//   - labels map[string]string
func (_e *MockSetter_Expecter) Set(ctx interface{}, volumeName interface{}, labels interface{}) *MockSetter_Set_Call {
	return &MockSetter_Set_Call{Call: _e.mock.On("Set", ctx, volumeName, labels)}
}

func (_c *MockSetter_Set_Call) Run(run func(ctx context.Context, volumeName string, labels map[string]string)) *MockSetter_Set_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(map[string]string))
	})
	return _c
}

func (_c *MockSetter_Set_Call) Return(_a0 error) *MockSetter_Set_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockSetter_Set_Call) RunAndReturn(run func(context.Context, string, map[string]string) error) *MockSetter_Set_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockSetter creates a new instance of MockSetter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockSetter(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockSetter {
	mock := &MockSetter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
