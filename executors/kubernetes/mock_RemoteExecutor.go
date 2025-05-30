// Code generated by mockery v2.53.3. DO NOT EDIT.

package kubernetes

import (
	context "context"
	io "io"

	mock "github.com/stretchr/testify/mock"

	rest "k8s.io/client-go/rest"

	url "net/url"
)

// MockRemoteExecutor is an autogenerated mock type for the RemoteExecutor type
type MockRemoteExecutor struct {
	mock.Mock
}

type MockRemoteExecutor_Expecter struct {
	mock *mock.Mock
}

func (_m *MockRemoteExecutor) EXPECT() *MockRemoteExecutor_Expecter {
	return &MockRemoteExecutor_Expecter{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: ctx, method, _a2, config, stdin, stdout, stderr, tty
func (_m *MockRemoteExecutor) Execute(ctx context.Context, method string, _a2 *url.URL, config *rest.Config, stdin io.Reader, stdout io.Writer, stderr io.Writer, tty bool) error {
	ret := _m.Called(ctx, method, _a2, config, stdin, stdout, stderr, tty)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *url.URL, *rest.Config, io.Reader, io.Writer, io.Writer, bool) error); ok {
		r0 = rf(ctx, method, _a2, config, stdin, stdout, stderr, tty)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockRemoteExecutor_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type MockRemoteExecutor_Execute_Call struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - ctx context.Context
//   - method string
//   - _a2 *url.URL
//   - config *rest.Config
//   - stdin io.Reader
//   - stdout io.Writer
//   - stderr io.Writer
//   - tty bool
func (_e *MockRemoteExecutor_Expecter) Execute(ctx interface{}, method interface{}, _a2 interface{}, config interface{}, stdin interface{}, stdout interface{}, stderr interface{}, tty interface{}) *MockRemoteExecutor_Execute_Call {
	return &MockRemoteExecutor_Execute_Call{Call: _e.mock.On("Execute", ctx, method, _a2, config, stdin, stdout, stderr, tty)}
}

func (_c *MockRemoteExecutor_Execute_Call) Run(run func(ctx context.Context, method string, _a2 *url.URL, config *rest.Config, stdin io.Reader, stdout io.Writer, stderr io.Writer, tty bool)) *MockRemoteExecutor_Execute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(string), args[2].(*url.URL), args[3].(*rest.Config), args[4].(io.Reader), args[5].(io.Writer), args[6].(io.Writer), args[7].(bool))
	})
	return _c
}

func (_c *MockRemoteExecutor_Execute_Call) Return(_a0 error) *MockRemoteExecutor_Execute_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockRemoteExecutor_Execute_Call) RunAndReturn(run func(context.Context, string, *url.URL, *rest.Config, io.Reader, io.Writer, io.Writer, bool) error) *MockRemoteExecutor_Execute_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockRemoteExecutor creates a new instance of MockRemoteExecutor. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockRemoteExecutor(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockRemoteExecutor {
	mock := &MockRemoteExecutor{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
