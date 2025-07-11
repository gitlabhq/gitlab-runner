// Code generated by mockery; DO NOT EDIT.
// github.com/vektra/mockery
// template: testify

package registry

import (
	mock "github.com/stretchr/testify/mock"
)

// NewMockRegistry creates a new instance of MockRegistry. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockRegistry(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockRegistry {
	mock := &MockRegistry{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// MockRegistry is an autogenerated mock type for the Registry type
type MockRegistry struct {
	mock.Mock
}

type MockRegistry_Expecter struct {
	mock *mock.Mock
}

func (_m *MockRegistry) EXPECT() *MockRegistry_Expecter {
	return &MockRegistry_Expecter{mock: &_m.Mock}
}

// Get provides a mock function for the type MockRegistry
func (_mock *MockRegistry) Get(factoryName string) (interface{}, error) {
	ret := _mock.Called(factoryName)

	if len(ret) == 0 {
		panic("no return value specified for Get")
	}

	var r0 interface{}
	var r1 error
	if returnFunc, ok := ret.Get(0).(func(string) (interface{}, error)); ok {
		return returnFunc(factoryName)
	}
	if returnFunc, ok := ret.Get(0).(func(string) interface{}); ok {
		r0 = returnFunc(factoryName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}
	if returnFunc, ok := ret.Get(1).(func(string) error); ok {
		r1 = returnFunc(factoryName)
	} else {
		r1 = ret.Error(1)
	}
	return r0, r1
}

// MockRegistry_Get_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Get'
type MockRegistry_Get_Call struct {
	*mock.Call
}

// Get is a helper method to define mock.On call
//   - factoryName string
func (_e *MockRegistry_Expecter) Get(factoryName interface{}) *MockRegistry_Get_Call {
	return &MockRegistry_Get_Call{Call: _e.mock.On("Get", factoryName)}
}

func (_c *MockRegistry_Get_Call) Run(run func(factoryName string)) *MockRegistry_Get_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 string
		if args[0] != nil {
			arg0 = args[0].(string)
		}
		run(
			arg0,
		)
	})
	return _c
}

func (_c *MockRegistry_Get_Call) Return(ifaceVal interface{}, err error) *MockRegistry_Get_Call {
	_c.Call.Return(ifaceVal, err)
	return _c
}

func (_c *MockRegistry_Get_Call) RunAndReturn(run func(factoryName string) (interface{}, error)) *MockRegistry_Get_Call {
	_c.Call.Return(run)
	return _c
}

// Register provides a mock function for the type MockRegistry
func (_mock *MockRegistry) Register(factoryName string, factory interface{}) error {
	ret := _mock.Called(factoryName, factory)

	if len(ret) == 0 {
		panic("no return value specified for Register")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(string, interface{}) error); ok {
		r0 = returnFunc(factoryName, factory)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// MockRegistry_Register_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Register'
type MockRegistry_Register_Call struct {
	*mock.Call
}

// Register is a helper method to define mock.On call
//   - factoryName string
//   - factory interface{}
func (_e *MockRegistry_Expecter) Register(factoryName interface{}, factory interface{}) *MockRegistry_Register_Call {
	return &MockRegistry_Register_Call{Call: _e.mock.On("Register", factoryName, factory)}
}

func (_c *MockRegistry_Register_Call) Run(run func(factoryName string, factory interface{})) *MockRegistry_Register_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 string
		if args[0] != nil {
			arg0 = args[0].(string)
		}
		var arg1 interface{}
		if args[1] != nil {
			arg1 = args[1].(interface{})
		}
		run(
			arg0,
			arg1,
		)
	})
	return _c
}

func (_c *MockRegistry_Register_Call) Return(err error) *MockRegistry_Register_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *MockRegistry_Register_Call) RunAndReturn(run func(factoryName string, factory interface{}) error) *MockRegistry_Register_Call {
	_c.Call.Return(run)
	return _c
}
