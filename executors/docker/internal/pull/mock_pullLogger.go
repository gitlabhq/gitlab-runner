// Code generated by mockery v2.53.3. DO NOT EDIT.

package pull

import mock "github.com/stretchr/testify/mock"

// mockPullLogger is an autogenerated mock type for the pullLogger type
type mockPullLogger struct {
	mock.Mock
}

type mockPullLogger_Expecter struct {
	mock *mock.Mock
}

func (_m *mockPullLogger) EXPECT() *mockPullLogger_Expecter {
	return &mockPullLogger_Expecter{mock: &_m.Mock}
}

// Debugln provides a mock function with given fields: args
func (_m *mockPullLogger) Debugln(args ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, args...)
	_m.Called(_ca...)
}

// mockPullLogger_Debugln_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Debugln'
type mockPullLogger_Debugln_Call struct {
	*mock.Call
}

// Debugln is a helper method to define mock.On call
//   - args ...interface{}
func (_e *mockPullLogger_Expecter) Debugln(args ...interface{}) *mockPullLogger_Debugln_Call {
	return &mockPullLogger_Debugln_Call{Call: _e.mock.On("Debugln",
		append([]interface{}{}, args...)...)}
}

func (_c *mockPullLogger_Debugln_Call) Run(run func(args ...interface{})) *mockPullLogger_Debugln_Call {
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

func (_c *mockPullLogger_Debugln_Call) Return() *mockPullLogger_Debugln_Call {
	_c.Call.Return()
	return _c
}

func (_c *mockPullLogger_Debugln_Call) RunAndReturn(run func(...interface{})) *mockPullLogger_Debugln_Call {
	_c.Run(run)
	return _c
}

// Infoln provides a mock function with given fields: args
func (_m *mockPullLogger) Infoln(args ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, args...)
	_m.Called(_ca...)
}

// mockPullLogger_Infoln_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Infoln'
type mockPullLogger_Infoln_Call struct {
	*mock.Call
}

// Infoln is a helper method to define mock.On call
//   - args ...interface{}
func (_e *mockPullLogger_Expecter) Infoln(args ...interface{}) *mockPullLogger_Infoln_Call {
	return &mockPullLogger_Infoln_Call{Call: _e.mock.On("Infoln",
		append([]interface{}{}, args...)...)}
}

func (_c *mockPullLogger_Infoln_Call) Run(run func(args ...interface{})) *mockPullLogger_Infoln_Call {
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

func (_c *mockPullLogger_Infoln_Call) Return() *mockPullLogger_Infoln_Call {
	_c.Call.Return()
	return _c
}

func (_c *mockPullLogger_Infoln_Call) RunAndReturn(run func(...interface{})) *mockPullLogger_Infoln_Call {
	_c.Run(run)
	return _c
}

// Println provides a mock function with given fields: args
func (_m *mockPullLogger) Println(args ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, args...)
	_m.Called(_ca...)
}

// mockPullLogger_Println_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Println'
type mockPullLogger_Println_Call struct {
	*mock.Call
}

// Println is a helper method to define mock.On call
//   - args ...interface{}
func (_e *mockPullLogger_Expecter) Println(args ...interface{}) *mockPullLogger_Println_Call {
	return &mockPullLogger_Println_Call{Call: _e.mock.On("Println",
		append([]interface{}{}, args...)...)}
}

func (_c *mockPullLogger_Println_Call) Run(run func(args ...interface{})) *mockPullLogger_Println_Call {
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

func (_c *mockPullLogger_Println_Call) Return() *mockPullLogger_Println_Call {
	_c.Call.Return()
	return _c
}

func (_c *mockPullLogger_Println_Call) RunAndReturn(run func(...interface{})) *mockPullLogger_Println_Call {
	_c.Run(run)
	return _c
}

// Warningln provides a mock function with given fields: args
func (_m *mockPullLogger) Warningln(args ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, args...)
	_m.Called(_ca...)
}

// mockPullLogger_Warningln_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Warningln'
type mockPullLogger_Warningln_Call struct {
	*mock.Call
}

// Warningln is a helper method to define mock.On call
//   - args ...interface{}
func (_e *mockPullLogger_Expecter) Warningln(args ...interface{}) *mockPullLogger_Warningln_Call {
	return &mockPullLogger_Warningln_Call{Call: _e.mock.On("Warningln",
		append([]interface{}{}, args...)...)}
}

func (_c *mockPullLogger_Warningln_Call) Run(run func(args ...interface{})) *mockPullLogger_Warningln_Call {
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

func (_c *mockPullLogger_Warningln_Call) Return() *mockPullLogger_Warningln_Call {
	_c.Call.Return()
	return _c
}

func (_c *mockPullLogger_Warningln_Call) RunAndReturn(run func(...interface{})) *mockPullLogger_Warningln_Call {
	_c.Run(run)
	return _c
}

// newMockPullLogger creates a new instance of mockPullLogger. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockPullLogger(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockPullLogger {
	mock := &mockPullLogger{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
