// Code generated by mockery; DO NOT EDIT.
// github.com/vektra/mockery
// template: testify

package runner_wrapper

import (
	"os"

	mock "github.com/stretchr/testify/mock"
)

// newMockProcess creates a new instance of mockProcess. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockProcess(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockProcess {
	mock := &mockProcess{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// mockProcess is an autogenerated mock type for the process type
type mockProcess struct {
	mock.Mock
}

type mockProcess_Expecter struct {
	mock *mock.Mock
}

func (_m *mockProcess) EXPECT() *mockProcess_Expecter {
	return &mockProcess_Expecter{mock: &_m.Mock}
}

// Signal provides a mock function for the type mockProcess
func (_mock *mockProcess) Signal(sig os.Signal) error {
	ret := _mock.Called(sig)

	if len(ret) == 0 {
		panic("no return value specified for Signal")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func(os.Signal) error); ok {
		r0 = returnFunc(sig)
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// mockProcess_Signal_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Signal'
type mockProcess_Signal_Call struct {
	*mock.Call
}

// Signal is a helper method to define mock.On call
//   - sig os.Signal
func (_e *mockProcess_Expecter) Signal(sig interface{}) *mockProcess_Signal_Call {
	return &mockProcess_Signal_Call{Call: _e.mock.On("Signal", sig)}
}

func (_c *mockProcess_Signal_Call) Run(run func(sig os.Signal)) *mockProcess_Signal_Call {
	_c.Call.Run(func(args mock.Arguments) {
		var arg0 os.Signal
		if args[0] != nil {
			arg0 = args[0].(os.Signal)
		}
		run(
			arg0,
		)
	})
	return _c
}

func (_c *mockProcess_Signal_Call) Return(err error) *mockProcess_Signal_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *mockProcess_Signal_Call) RunAndReturn(run func(sig os.Signal) error) *mockProcess_Signal_Call {
	_c.Call.Return(run)
	return _c
}

// newMockCommander creates a new instance of mockCommander. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockCommander(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockCommander {
	mock := &mockCommander{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}

// mockCommander is an autogenerated mock type for the commander type
type mockCommander struct {
	mock.Mock
}

type mockCommander_Expecter struct {
	mock *mock.Mock
}

func (_m *mockCommander) EXPECT() *mockCommander_Expecter {
	return &mockCommander_Expecter{mock: &_m.Mock}
}

// Process provides a mock function for the type mockCommander
func (_mock *mockCommander) Process() process {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for Process")
	}

	var r0 process
	if returnFunc, ok := ret.Get(0).(func() process); ok {
		r0 = returnFunc()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(process)
		}
	}
	return r0
}

// mockCommander_Process_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Process'
type mockCommander_Process_Call struct {
	*mock.Call
}

// Process is a helper method to define mock.On call
func (_e *mockCommander_Expecter) Process() *mockCommander_Process_Call {
	return &mockCommander_Process_Call{Call: _e.mock.On("Process")}
}

func (_c *mockCommander_Process_Call) Run(run func()) *mockCommander_Process_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockCommander_Process_Call) Return(processMoqParam process) *mockCommander_Process_Call {
	_c.Call.Return(processMoqParam)
	return _c
}

func (_c *mockCommander_Process_Call) RunAndReturn(run func() process) *mockCommander_Process_Call {
	_c.Call.Return(run)
	return _c
}

// Start provides a mock function for the type mockCommander
func (_mock *mockCommander) Start() error {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for Start")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func() error); ok {
		r0 = returnFunc()
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// mockCommander_Start_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Start'
type mockCommander_Start_Call struct {
	*mock.Call
}

// Start is a helper method to define mock.On call
func (_e *mockCommander_Expecter) Start() *mockCommander_Start_Call {
	return &mockCommander_Start_Call{Call: _e.mock.On("Start")}
}

func (_c *mockCommander_Start_Call) Run(run func()) *mockCommander_Start_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockCommander_Start_Call) Return(err error) *mockCommander_Start_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *mockCommander_Start_Call) RunAndReturn(run func() error) *mockCommander_Start_Call {
	_c.Call.Return(run)
	return _c
}

// Wait provides a mock function for the type mockCommander
func (_mock *mockCommander) Wait() error {
	ret := _mock.Called()

	if len(ret) == 0 {
		panic("no return value specified for Wait")
	}

	var r0 error
	if returnFunc, ok := ret.Get(0).(func() error); ok {
		r0 = returnFunc()
	} else {
		r0 = ret.Error(0)
	}
	return r0
}

// mockCommander_Wait_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Wait'
type mockCommander_Wait_Call struct {
	*mock.Call
}

// Wait is a helper method to define mock.On call
func (_e *mockCommander_Expecter) Wait() *mockCommander_Wait_Call {
	return &mockCommander_Wait_Call{Call: _e.mock.On("Wait")}
}

func (_c *mockCommander_Wait_Call) Run(run func()) *mockCommander_Wait_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockCommander_Wait_Call) Return(err error) *mockCommander_Wait_Call {
	_c.Call.Return(err)
	return _c
}

func (_c *mockCommander_Wait_Call) RunAndReturn(run func() error) *mockCommander_Wait_Call {
	_c.Call.Return(run)
	return _c
}
