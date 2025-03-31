// Code generated by mockery v2.53.3. DO NOT EDIT.

package network

import mock "github.com/stretchr/testify/mock"

// mockRequestCredentials is an autogenerated mock type for the requestCredentials type
type mockRequestCredentials struct {
	mock.Mock
}

type mockRequestCredentials_Expecter struct {
	mock *mock.Mock
}

func (_m *mockRequestCredentials) EXPECT() *mockRequestCredentials_Expecter {
	return &mockRequestCredentials_Expecter{mock: &_m.Mock}
}

// GetTLSCAFile provides a mock function with no fields
func (_m *mockRequestCredentials) GetTLSCAFile() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetTLSCAFile")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// mockRequestCredentials_GetTLSCAFile_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetTLSCAFile'
type mockRequestCredentials_GetTLSCAFile_Call struct {
	*mock.Call
}

// GetTLSCAFile is a helper method to define mock.On call
func (_e *mockRequestCredentials_Expecter) GetTLSCAFile() *mockRequestCredentials_GetTLSCAFile_Call {
	return &mockRequestCredentials_GetTLSCAFile_Call{Call: _e.mock.On("GetTLSCAFile")}
}

func (_c *mockRequestCredentials_GetTLSCAFile_Call) Run(run func()) *mockRequestCredentials_GetTLSCAFile_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockRequestCredentials_GetTLSCAFile_Call) Return(_a0 string) *mockRequestCredentials_GetTLSCAFile_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockRequestCredentials_GetTLSCAFile_Call) RunAndReturn(run func() string) *mockRequestCredentials_GetTLSCAFile_Call {
	_c.Call.Return(run)
	return _c
}

// GetTLSCertFile provides a mock function with no fields
func (_m *mockRequestCredentials) GetTLSCertFile() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetTLSCertFile")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// mockRequestCredentials_GetTLSCertFile_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetTLSCertFile'
type mockRequestCredentials_GetTLSCertFile_Call struct {
	*mock.Call
}

// GetTLSCertFile is a helper method to define mock.On call
func (_e *mockRequestCredentials_Expecter) GetTLSCertFile() *mockRequestCredentials_GetTLSCertFile_Call {
	return &mockRequestCredentials_GetTLSCertFile_Call{Call: _e.mock.On("GetTLSCertFile")}
}

func (_c *mockRequestCredentials_GetTLSCertFile_Call) Run(run func()) *mockRequestCredentials_GetTLSCertFile_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockRequestCredentials_GetTLSCertFile_Call) Return(_a0 string) *mockRequestCredentials_GetTLSCertFile_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockRequestCredentials_GetTLSCertFile_Call) RunAndReturn(run func() string) *mockRequestCredentials_GetTLSCertFile_Call {
	_c.Call.Return(run)
	return _c
}

// GetTLSKeyFile provides a mock function with no fields
func (_m *mockRequestCredentials) GetTLSKeyFile() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetTLSKeyFile")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// mockRequestCredentials_GetTLSKeyFile_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetTLSKeyFile'
type mockRequestCredentials_GetTLSKeyFile_Call struct {
	*mock.Call
}

// GetTLSKeyFile is a helper method to define mock.On call
func (_e *mockRequestCredentials_Expecter) GetTLSKeyFile() *mockRequestCredentials_GetTLSKeyFile_Call {
	return &mockRequestCredentials_GetTLSKeyFile_Call{Call: _e.mock.On("GetTLSKeyFile")}
}

func (_c *mockRequestCredentials_GetTLSKeyFile_Call) Run(run func()) *mockRequestCredentials_GetTLSKeyFile_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockRequestCredentials_GetTLSKeyFile_Call) Return(_a0 string) *mockRequestCredentials_GetTLSKeyFile_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockRequestCredentials_GetTLSKeyFile_Call) RunAndReturn(run func() string) *mockRequestCredentials_GetTLSKeyFile_Call {
	_c.Call.Return(run)
	return _c
}

// GetToken provides a mock function with no fields
func (_m *mockRequestCredentials) GetToken() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetToken")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// mockRequestCredentials_GetToken_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetToken'
type mockRequestCredentials_GetToken_Call struct {
	*mock.Call
}

// GetToken is a helper method to define mock.On call
func (_e *mockRequestCredentials_Expecter) GetToken() *mockRequestCredentials_GetToken_Call {
	return &mockRequestCredentials_GetToken_Call{Call: _e.mock.On("GetToken")}
}

func (_c *mockRequestCredentials_GetToken_Call) Run(run func()) *mockRequestCredentials_GetToken_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockRequestCredentials_GetToken_Call) Return(_a0 string) *mockRequestCredentials_GetToken_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockRequestCredentials_GetToken_Call) RunAndReturn(run func() string) *mockRequestCredentials_GetToken_Call {
	_c.Call.Return(run)
	return _c
}

// GetURL provides a mock function with no fields
func (_m *mockRequestCredentials) GetURL() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetURL")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// mockRequestCredentials_GetURL_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetURL'
type mockRequestCredentials_GetURL_Call struct {
	*mock.Call
}

// GetURL is a helper method to define mock.On call
func (_e *mockRequestCredentials_Expecter) GetURL() *mockRequestCredentials_GetURL_Call {
	return &mockRequestCredentials_GetURL_Call{Call: _e.mock.On("GetURL")}
}

func (_c *mockRequestCredentials_GetURL_Call) Run(run func()) *mockRequestCredentials_GetURL_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockRequestCredentials_GetURL_Call) Return(_a0 string) *mockRequestCredentials_GetURL_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockRequestCredentials_GetURL_Call) RunAndReturn(run func() string) *mockRequestCredentials_GetURL_Call {
	_c.Call.Return(run)
	return _c
}

// newMockRequestCredentials creates a new instance of mockRequestCredentials. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockRequestCredentials(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockRequestCredentials {
	mock := &mockRequestCredentials{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
