// Code generated by mockery v2.53.3. DO NOT EDIT.

package common

import (
	mock "github.com/stretchr/testify/mock"
	cli "github.com/urfave/cli"
)

// MockCommander is an autogenerated mock type for the Commander type
type MockCommander struct {
	mock.Mock
}

type MockCommander_Expecter struct {
	mock *mock.Mock
}

func (_m *MockCommander) EXPECT() *MockCommander_Expecter {
	return &MockCommander_Expecter{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: c
func (_m *MockCommander) Execute(c *cli.Context) {
	_m.Called(c)
}

// MockCommander_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type MockCommander_Execute_Call struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - c *cli.Context
func (_e *MockCommander_Expecter) Execute(c interface{}) *MockCommander_Execute_Call {
	return &MockCommander_Execute_Call{Call: _e.mock.On("Execute", c)}
}

func (_c *MockCommander_Execute_Call) Run(run func(c *cli.Context)) *MockCommander_Execute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*cli.Context))
	})
	return _c
}

func (_c *MockCommander_Execute_Call) Return() *MockCommander_Execute_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockCommander_Execute_Call) RunAndReturn(run func(*cli.Context)) *MockCommander_Execute_Call {
	_c.Run(run)
	return _c
}

// NewMockCommander creates a new instance of MockCommander. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockCommander(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockCommander {
	mock := &MockCommander{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
