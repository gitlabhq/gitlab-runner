// Code generated by mockery v2.53.3. DO NOT EDIT.

package retry

import mock "github.com/stretchr/testify/mock"

// MockProvider is an autogenerated mock type for the Provider type
type MockProvider struct {
	mock.Mock
}

type MockProvider_Expecter struct {
	mock *mock.Mock
}

func (_m *MockProvider) EXPECT() *MockProvider_Expecter {
	return &MockProvider_Expecter{mock: &_m.Mock}
}

// NewRetry provides a mock function with no fields
func (_m *MockProvider) NewRetry() *Retry {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for NewRetry")
	}

	var r0 *Retry
	if rf, ok := ret.Get(0).(func() *Retry); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*Retry)
		}
	}

	return r0
}

// MockProvider_NewRetry_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'NewRetry'
type MockProvider_NewRetry_Call struct {
	*mock.Call
}

// NewRetry is a helper method to define mock.On call
func (_e *MockProvider_Expecter) NewRetry() *MockProvider_NewRetry_Call {
	return &MockProvider_NewRetry_Call{Call: _e.mock.On("NewRetry")}
}

func (_c *MockProvider_NewRetry_Call) Run(run func()) *MockProvider_NewRetry_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockProvider_NewRetry_Call) Return(_a0 *Retry) *MockProvider_NewRetry_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockProvider_NewRetry_Call) RunAndReturn(run func() *Retry) *MockProvider_NewRetry_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockProvider creates a new instance of MockProvider. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockProvider(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockProvider {
	mock := &MockProvider{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
