// Code generated by mockery v2.53.3. DO NOT EDIT.

package referees

import mock "github.com/stretchr/testify/mock"

// MockMetricsExecutor is an autogenerated mock type for the MetricsExecutor type
type MockMetricsExecutor struct {
	mock.Mock
}

type MockMetricsExecutor_Expecter struct {
	mock *mock.Mock
}

func (_m *MockMetricsExecutor) EXPECT() *MockMetricsExecutor_Expecter {
	return &MockMetricsExecutor_Expecter{mock: &_m.Mock}
}

// GetMetricsSelector provides a mock function with no fields
func (_m *MockMetricsExecutor) GetMetricsSelector() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetMetricsSelector")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockMetricsExecutor_GetMetricsSelector_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetMetricsSelector'
type MockMetricsExecutor_GetMetricsSelector_Call struct {
	*mock.Call
}

// GetMetricsSelector is a helper method to define mock.On call
func (_e *MockMetricsExecutor_Expecter) GetMetricsSelector() *MockMetricsExecutor_GetMetricsSelector_Call {
	return &MockMetricsExecutor_GetMetricsSelector_Call{Call: _e.mock.On("GetMetricsSelector")}
}

func (_c *MockMetricsExecutor_GetMetricsSelector_Call) Run(run func()) *MockMetricsExecutor_GetMetricsSelector_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockMetricsExecutor_GetMetricsSelector_Call) Return(_a0 string) *MockMetricsExecutor_GetMetricsSelector_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockMetricsExecutor_GetMetricsSelector_Call) RunAndReturn(run func() string) *MockMetricsExecutor_GetMetricsSelector_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockMetricsExecutor creates a new instance of MockMetricsExecutor. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockMetricsExecutor(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockMetricsExecutor {
	mock := &MockMetricsExecutor{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
