// Code generated by mockery v2.53.3. DO NOT EDIT.

package process

import mock "github.com/stretchr/testify/mock"

// MockKillWaiter is an autogenerated mock type for the KillWaiter type
type MockKillWaiter struct {
	mock.Mock
}

type MockKillWaiter_Expecter struct {
	mock *mock.Mock
}

func (_m *MockKillWaiter) EXPECT() *MockKillWaiter_Expecter {
	return &MockKillWaiter_Expecter{mock: &_m.Mock}
}

// KillAndWait provides a mock function with given fields: command, waitCh
func (_m *MockKillWaiter) KillAndWait(command Commander, waitCh chan error) error {
	ret := _m.Called(command, waitCh)

	if len(ret) == 0 {
		panic("no return value specified for KillAndWait")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(Commander, chan error) error); ok {
		r0 = rf(command, waitCh)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockKillWaiter_KillAndWait_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'KillAndWait'
type MockKillWaiter_KillAndWait_Call struct {
	*mock.Call
}

// KillAndWait is a helper method to define mock.On call
//   - command Commander
//   - waitCh chan error
func (_e *MockKillWaiter_Expecter) KillAndWait(command interface{}, waitCh interface{}) *MockKillWaiter_KillAndWait_Call {
	return &MockKillWaiter_KillAndWait_Call{Call: _e.mock.On("KillAndWait", command, waitCh)}
}

func (_c *MockKillWaiter_KillAndWait_Call) Run(run func(command Commander, waitCh chan error)) *MockKillWaiter_KillAndWait_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(Commander), args[1].(chan error))
	})
	return _c
}

func (_c *MockKillWaiter_KillAndWait_Call) Return(_a0 error) *MockKillWaiter_KillAndWait_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockKillWaiter_KillAndWait_Call) RunAndReturn(run func(Commander, chan error) error) *MockKillWaiter_KillAndWait_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockKillWaiter creates a new instance of MockKillWaiter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockKillWaiter(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockKillWaiter {
	mock := &MockKillWaiter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
