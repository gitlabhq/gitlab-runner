// Code generated by mockery v2.53.3. DO NOT EDIT.

package common

import mock "github.com/stretchr/testify/mock"

// mockUrlHelper is an autogenerated mock type for the urlHelper type
type mockUrlHelper struct {
	mock.Mock
}

type mockUrlHelper_Expecter struct {
	mock *mock.Mock
}

func (_m *mockUrlHelper) EXPECT() *mockUrlHelper_Expecter {
	return &mockUrlHelper_Expecter{mock: &_m.Mock}
}

// GetRemoteURL provides a mock function with no fields
func (_m *mockUrlHelper) GetRemoteURL() (string, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetRemoteURL")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func() (string, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mockUrlHelper_GetRemoteURL_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetRemoteURL'
type mockUrlHelper_GetRemoteURL_Call struct {
	*mock.Call
}

// GetRemoteURL is a helper method to define mock.On call
func (_e *mockUrlHelper_Expecter) GetRemoteURL() *mockUrlHelper_GetRemoteURL_Call {
	return &mockUrlHelper_GetRemoteURL_Call{Call: _e.mock.On("GetRemoteURL")}
}

func (_c *mockUrlHelper_GetRemoteURL_Call) Run(run func()) *mockUrlHelper_GetRemoteURL_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockUrlHelper_GetRemoteURL_Call) Return(_a0 string, _a1 error) *mockUrlHelper_GetRemoteURL_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *mockUrlHelper_GetRemoteURL_Call) RunAndReturn(run func() (string, error)) *mockUrlHelper_GetRemoteURL_Call {
	_c.Call.Return(run)
	return _c
}

// GetURLInsteadOfArgs provides a mock function with no fields
func (_m *mockUrlHelper) GetURLInsteadOfArgs() ([]string, error) {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetURLInsteadOfArgs")
	}

	var r0 []string
	var r1 error
	if rf, ok := ret.Get(0).(func() ([]string, error)); ok {
		return rf()
	}
	if rf, ok := ret.Get(0).(func() []string); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mockUrlHelper_GetURLInsteadOfArgs_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetURLInsteadOfArgs'
type mockUrlHelper_GetURLInsteadOfArgs_Call struct {
	*mock.Call
}

// GetURLInsteadOfArgs is a helper method to define mock.On call
func (_e *mockUrlHelper_Expecter) GetURLInsteadOfArgs() *mockUrlHelper_GetURLInsteadOfArgs_Call {
	return &mockUrlHelper_GetURLInsteadOfArgs_Call{Call: _e.mock.On("GetURLInsteadOfArgs")}
}

func (_c *mockUrlHelper_GetURLInsteadOfArgs_Call) Run(run func()) *mockUrlHelper_GetURLInsteadOfArgs_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockUrlHelper_GetURLInsteadOfArgs_Call) Return(_a0 []string, _a1 error) *mockUrlHelper_GetURLInsteadOfArgs_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *mockUrlHelper_GetURLInsteadOfArgs_Call) RunAndReturn(run func() ([]string, error)) *mockUrlHelper_GetURLInsteadOfArgs_Call {
	_c.Call.Return(run)
	return _c
}

// newMockUrlHelper creates a new instance of mockUrlHelper. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockUrlHelper(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockUrlHelper {
	mock := &mockUrlHelper{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
