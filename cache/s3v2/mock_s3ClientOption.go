// Code generated by mockery v2.53.3. DO NOT EDIT.

package s3v2

import mock "github.com/stretchr/testify/mock"

// mockS3ClientOption is an autogenerated mock type for the s3ClientOption type
type mockS3ClientOption struct {
	mock.Mock
}

type mockS3ClientOption_Expecter struct {
	mock *mock.Mock
}

func (_m *mockS3ClientOption) EXPECT() *mockS3ClientOption_Expecter {
	return &mockS3ClientOption_Expecter{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: _a0
func (_m *mockS3ClientOption) Execute(_a0 *s3Client) {
	_m.Called(_a0)
}

// mockS3ClientOption_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type mockS3ClientOption_Execute_Call struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - _a0 *s3Client
func (_e *mockS3ClientOption_Expecter) Execute(_a0 interface{}) *mockS3ClientOption_Execute_Call {
	return &mockS3ClientOption_Execute_Call{Call: _e.mock.On("Execute", _a0)}
}

func (_c *mockS3ClientOption_Execute_Call) Run(run func(_a0 *s3Client)) *mockS3ClientOption_Execute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*s3Client))
	})
	return _c
}

func (_c *mockS3ClientOption_Execute_Call) Return() *mockS3ClientOption_Execute_Call {
	_c.Call.Return()
	return _c
}

func (_c *mockS3ClientOption_Execute_Call) RunAndReturn(run func(*s3Client)) *mockS3ClientOption_Execute_Call {
	_c.Run(run)
	return _c
}

// newMockS3ClientOption creates a new instance of mockS3ClientOption. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockS3ClientOption(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockS3ClientOption {
	mock := &mockS3ClientOption{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
