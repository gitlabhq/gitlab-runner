// Code generated by mockery v2.53.3. DO NOT EDIT.

package common

import mock "github.com/stretchr/testify/mock"

// mockLogger is an autogenerated mock type for the logger type
type mockLogger struct {
	mock.Mock
}

type mockLogger_Expecter struct {
	mock *mock.Mock
}

func (_m *mockLogger) EXPECT() *mockLogger_Expecter {
	return &mockLogger_Expecter{mock: &_m.Mock}
}

// Println provides a mock function with given fields: args
func (_m *mockLogger) Println(args ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, args...)
	_m.Called(_ca...)
}

// mockLogger_Println_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Println'
type mockLogger_Println_Call struct {
	*mock.Call
}

// Println is a helper method to define mock.On call
//   - args ...interface{}
func (_e *mockLogger_Expecter) Println(args ...interface{}) *mockLogger_Println_Call {
	return &mockLogger_Println_Call{Call: _e.mock.On("Println",
		append([]interface{}{}, args...)...)}
}

func (_c *mockLogger_Println_Call) Run(run func(args ...interface{})) *mockLogger_Println_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]interface{}, len(args)-0)
		for i, a := range args[0:] {
			if a != nil {
				variadicArgs[i] = a.(interface{})
			}
		}
		run(variadicArgs...)
	})
	return _c
}

func (_c *mockLogger_Println_Call) Return() *mockLogger_Println_Call {
	_c.Call.Return()
	return _c
}

func (_c *mockLogger_Println_Call) RunAndReturn(run func(...interface{})) *mockLogger_Println_Call {
	_c.Run(run)
	return _c
}

// Warningln provides a mock function with given fields: args
func (_m *mockLogger) Warningln(args ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, args...)
	_m.Called(_ca...)
}

// mockLogger_Warningln_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Warningln'
type mockLogger_Warningln_Call struct {
	*mock.Call
}

// Warningln is a helper method to define mock.On call
//   - args ...interface{}
func (_e *mockLogger_Expecter) Warningln(args ...interface{}) *mockLogger_Warningln_Call {
	return &mockLogger_Warningln_Call{Call: _e.mock.On("Warningln",
		append([]interface{}{}, args...)...)}
}

func (_c *mockLogger_Warningln_Call) Run(run func(args ...interface{})) *mockLogger_Warningln_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]interface{}, len(args)-0)
		for i, a := range args[0:] {
			if a != nil {
				variadicArgs[i] = a.(interface{})
			}
		}
		run(variadicArgs...)
	})
	return _c
}

func (_c *mockLogger_Warningln_Call) Return() *mockLogger_Warningln_Call {
	_c.Call.Return()
	return _c
}

func (_c *mockLogger_Warningln_Call) RunAndReturn(run func(...interface{})) *mockLogger_Warningln_Call {
	_c.Run(run)
	return _c
}

// newMockLogger creates a new instance of mockLogger. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockLogger(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockLogger {
	mock := &mockLogger{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
