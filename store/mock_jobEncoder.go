// Code generated by mockery v2.53.3. DO NOT EDIT.

package store

import (
	io "io"

	common "gitlab.com/gitlab-org/gitlab-runner/common"

	mock "github.com/stretchr/testify/mock"
)

// mockJobEncoder is an autogenerated mock type for the jobEncoder type
type mockJobEncoder struct {
	mock.Mock
}

type mockJobEncoder_Expecter struct {
	mock *mock.Mock
}

func (_m *mockJobEncoder) EXPECT() *mockJobEncoder_Expecter {
	return &mockJobEncoder_Expecter{mock: &_m.Mock}
}

// Encode provides a mock function with given fields: _a0, _a1
func (_m *mockJobEncoder) Encode(_a0 io.Writer, _a1 *common.Job) error {
	ret := _m.Called(_a0, _a1)

	if len(ret) == 0 {
		panic("no return value specified for Encode")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(io.Writer, *common.Job) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// mockJobEncoder_Encode_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Encode'
type mockJobEncoder_Encode_Call struct {
	*mock.Call
}

// Encode is a helper method to define mock.On call
//   - _a0 io.Writer
//   - _a1 *common.Job
func (_e *mockJobEncoder_Expecter) Encode(_a0 interface{}, _a1 interface{}) *mockJobEncoder_Encode_Call {
	return &mockJobEncoder_Encode_Call{Call: _e.mock.On("Encode", _a0, _a1)}
}

func (_c *mockJobEncoder_Encode_Call) Run(run func(_a0 io.Writer, _a1 *common.Job)) *mockJobEncoder_Encode_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(io.Writer), args[1].(*common.Job))
	})
	return _c
}

func (_c *mockJobEncoder_Encode_Call) Return(_a0 error) *mockJobEncoder_Encode_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockJobEncoder_Encode_Call) RunAndReturn(run func(io.Writer, *common.Job) error) *mockJobEncoder_Encode_Call {
	_c.Call.Return(run)
	return _c
}

// newMockJobEncoder creates a new instance of mockJobEncoder. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockJobEncoder(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockJobEncoder {
	mock := &mockJobEncoder{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
