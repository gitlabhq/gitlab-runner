// Code generated by mockery v2.53.3. DO NOT EDIT.

package commands

import mock "github.com/stretchr/testify/mock"

// mockRunAtTask is an autogenerated mock type for the runAtTask type
type mockRunAtTask struct {
	mock.Mock
}

type mockRunAtTask_Expecter struct {
	mock *mock.Mock
}

func (_m *mockRunAtTask) EXPECT() *mockRunAtTask_Expecter {
	return &mockRunAtTask_Expecter{mock: &_m.Mock}
}

// cancel provides a mock function with no fields
func (_m *mockRunAtTask) cancel() {
	_m.Called()
}

// mockRunAtTask_cancel_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'cancel'
type mockRunAtTask_cancel_Call struct {
	*mock.Call
}

// cancel is a helper method to define mock.On call
func (_e *mockRunAtTask_Expecter) cancel() *mockRunAtTask_cancel_Call {
	return &mockRunAtTask_cancel_Call{Call: _e.mock.On("cancel")}
}

func (_c *mockRunAtTask_cancel_Call) Run(run func()) *mockRunAtTask_cancel_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockRunAtTask_cancel_Call) Return() *mockRunAtTask_cancel_Call {
	_c.Call.Return()
	return _c
}

func (_c *mockRunAtTask_cancel_Call) RunAndReturn(run func()) *mockRunAtTask_cancel_Call {
	_c.Run(run)
	return _c
}

// newMockRunAtTask creates a new instance of mockRunAtTask. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockRunAtTask(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockRunAtTask {
	mock := &mockRunAtTask{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
