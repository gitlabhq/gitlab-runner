// Code generated by mockery; DO NOT EDIT.
// github.com/vektra/mockery
// template: testify

package packages

import (
	mock "github.com/stretchr/testify/mock"
	"gitlab.com/gitlab-org/gitlab-runner/magefiles/build"
)

// NewMockBlueprint creates a new instance of MockBlueprint. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockBlueprint(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockBlueprint {
	mock := &MockBlueprint{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// MockBlueprint is an autogenerated mock type for the Blueprint type
type MockBlueprint struct {
	mock.Mock
}

type MockBlueprint_Expecter struct {
	mock *mock.Mock
}

func (_m *MockBlueprint) EXPECT() *MockBlueprint_Expecter {
	return &MockBlueprint_Expecter{mock: &_m.Mock}
}

// Artifacts provides a mock function for the type MockBlueprint
func (_mock *MockBlueprint) Artifacts() []build.Component {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for Artifacts")
	}

	var r0 []build.Component
	if returnFunc, ok := ret.Get(0).(func() []build.Component); ok {
		r0 = returnFunc()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]build.Component)
		}
	}
	return r0
}

// MockBlueprint_Artifacts_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Artifacts'
type MockBlueprint_Artifacts_Call struct {
	*mock.Call
}

// Artifacts is a helper method to define mock.On call
func (_e *MockBlueprint_Expecter) Artifacts() *MockBlueprint_Artifacts_Call {
	return &MockBlueprint_Artifacts_Call{Call: _e.mock.On("Artifacts")}
}

func (_c *MockBlueprint_Artifacts_Call) Run(run func()) *MockBlueprint_Artifacts_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockBlueprint_Artifacts_Call) Return(components []build.Component) *MockBlueprint_Artifacts_Call {
	_c.Call.Return(components)
	return _c
}

func (_c *MockBlueprint_Artifacts_Call) RunAndReturn(run func() []build.Component) *MockBlueprint_Artifacts_Call {
	_c.Call.Return(run)
	return _c
}

// Data provides a mock function for the type MockBlueprint
func (_mock *MockBlueprint) Data() blueprintParams {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for Data")
	}

	var r0 blueprintParams
	if returnFunc, ok := ret.Get(0).(func() blueprintParams); ok {
		r0 = returnFunc()
	} else {
		r0 = ret.Get(0).(blueprintParams)
	}
	return r0
}

// MockBlueprint_Data_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Data'
type MockBlueprint_Data_Call struct {
	*mock.Call
}

// Data is a helper method to define mock.On call
func (_e *MockBlueprint_Expecter) Data() *MockBlueprint_Data_Call {
	return &MockBlueprint_Data_Call{Call: _e.mock.On("Data")}
}

func (_c *MockBlueprint_Data_Call) Run(run func()) *MockBlueprint_Data_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockBlueprint_Data_Call) Return(blueprintParamsMoqParam blueprintParams) *MockBlueprint_Data_Call {
	_c.Call.Return(blueprintParamsMoqParam)
	return _c
}

func (_c *MockBlueprint_Data_Call) RunAndReturn(run func() blueprintParams) *MockBlueprint_Data_Call {
	_c.Call.Return(run)
	return _c
}

// Dependencies provides a mock function for the type MockBlueprint
func (_mock *MockBlueprint) Dependencies() []build.Component {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for Dependencies")
	}

	var r0 []build.Component
	if returnFunc, ok := ret.Get(0).(func() []build.Component); ok {
		r0 = returnFunc()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]build.Component)
		}
	}
	return r0
}

// MockBlueprint_Dependencies_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Dependencies'
type MockBlueprint_Dependencies_Call struct {
	*mock.Call
}

// Dependencies is a helper method to define mock.On call
func (_e *MockBlueprint_Expecter) Dependencies() *MockBlueprint_Dependencies_Call {
	return &MockBlueprint_Dependencies_Call{Call: _e.mock.On("Dependencies")}
}

func (_c *MockBlueprint_Dependencies_Call) Run(run func()) *MockBlueprint_Dependencies_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockBlueprint_Dependencies_Call) Return(components []build.Component) *MockBlueprint_Dependencies_Call {
	_c.Call.Return(components)
	return _c
}

func (_c *MockBlueprint_Dependencies_Call) RunAndReturn(run func() []build.Component) *MockBlueprint_Dependencies_Call {
	_c.Call.Return(run)
	return _c
}

// Env provides a mock function for the type MockBlueprint
func (_mock *MockBlueprint) Env() build.BlueprintEnv {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for Env")
	}

	var r0 build.BlueprintEnv
	if returnFunc, ok := ret.Get(0).(func() build.BlueprintEnv); ok {
		r0 = returnFunc()
	} else {
		r0 = ret.Get(0).(build.BlueprintEnv)
	}
	return r0
}

// MockBlueprint_Env_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Env'
type MockBlueprint_Env_Call struct {
	*mock.Call
}

// Env is a helper method to define mock.On call
func (_e *MockBlueprint_Expecter) Env() *MockBlueprint_Env_Call {
	return &MockBlueprint_Env_Call{Call: _e.mock.On("Env")}
}

func (_c *MockBlueprint_Env_Call) Run(run func()) *MockBlueprint_Env_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockBlueprint_Env_Call) Return(blueprintEnv build.BlueprintEnv) *MockBlueprint_Env_Call {
	_c.Call.Return(blueprintEnv)
	return _c
}

func (_c *MockBlueprint_Env_Call) RunAndReturn(run func() build.BlueprintEnv) *MockBlueprint_Env_Call {
	_c.Call.Return(run)
	return _c
}
