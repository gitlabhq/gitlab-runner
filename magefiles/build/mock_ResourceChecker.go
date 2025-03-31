// Code generated by mockery v2.53.3. DO NOT EDIT.

package build

import mock "github.com/stretchr/testify/mock"

// MockResourceChecker is an autogenerated mock type for the ResourceChecker type
type MockResourceChecker struct {
	mock.Mock
}

type MockResourceChecker_Expecter struct {
	mock *mock.Mock
}

func (_m *MockResourceChecker) EXPECT() *MockResourceChecker_Expecter {
	return &MockResourceChecker_Expecter{mock: &_m.Mock}
}

// Exists provides a mock function with no fields
func (_m *MockResourceChecker) Exists() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Exists")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockResourceChecker_Exists_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Exists'
type MockResourceChecker_Exists_Call struct {
	*mock.Call
}

// Exists is a helper method to define mock.On call
func (_e *MockResourceChecker_Expecter) Exists() *MockResourceChecker_Exists_Call {
	return &MockResourceChecker_Exists_Call{Call: _e.mock.On("Exists")}
}

func (_c *MockResourceChecker_Exists_Call) Run(run func()) *MockResourceChecker_Exists_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockResourceChecker_Exists_Call) Return(_a0 error) *MockResourceChecker_Exists_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockResourceChecker_Exists_Call) RunAndReturn(run func() error) *MockResourceChecker_Exists_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockResourceChecker creates a new instance of MockResourceChecker. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockResourceChecker(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockResourceChecker {
	mock := &MockResourceChecker{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
