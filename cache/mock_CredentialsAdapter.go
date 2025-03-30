// Code generated by mockery v2.53.3. DO NOT EDIT.

package cache

import mock "github.com/stretchr/testify/mock"

// MockCredentialsAdapter is an autogenerated mock type for the CredentialsAdapter type
type MockCredentialsAdapter struct {
	mock.Mock
}

type MockCredentialsAdapter_Expecter struct {
	mock *mock.Mock
}

func (_m *MockCredentialsAdapter) EXPECT() *MockCredentialsAdapter_Expecter {
	return &MockCredentialsAdapter_Expecter{mock: &_m.Mock}
}

// GetCredentials provides a mock function with no fields
func (_m *MockCredentialsAdapter) GetCredentials() map[string]string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetCredentials")
	}

	var r0 map[string]string
	if rf, ok := ret.Get(0).(func() map[string]string); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]string)
		}
	}

	return r0
}

// MockCredentialsAdapter_GetCredentials_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetCredentials'
type MockCredentialsAdapter_GetCredentials_Call struct {
	*mock.Call
}

// GetCredentials is a helper method to define mock.On call
func (_e *MockCredentialsAdapter_Expecter) GetCredentials() *MockCredentialsAdapter_GetCredentials_Call {
	return &MockCredentialsAdapter_GetCredentials_Call{Call: _e.mock.On("GetCredentials")}
}

func (_c *MockCredentialsAdapter_GetCredentials_Call) Run(run func()) *MockCredentialsAdapter_GetCredentials_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockCredentialsAdapter_GetCredentials_Call) Return(_a0 map[string]string) *MockCredentialsAdapter_GetCredentials_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockCredentialsAdapter_GetCredentials_Call) RunAndReturn(run func() map[string]string) *MockCredentialsAdapter_GetCredentials_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockCredentialsAdapter creates a new instance of MockCredentialsAdapter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockCredentialsAdapter(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockCredentialsAdapter {
	mock := &MockCredentialsAdapter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
