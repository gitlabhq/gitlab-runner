// Code generated by mockery v2.53.3. DO NOT EDIT.

package ca_chain

import (
	x509 "crypto/x509"

	mock "github.com/stretchr/testify/mock"
)

// mockVerifier is an autogenerated mock type for the verifier type
type mockVerifier struct {
	mock.Mock
}

type mockVerifier_Expecter struct {
	mock *mock.Mock
}

func (_m *mockVerifier) EXPECT() *mockVerifier_Expecter {
	return &mockVerifier_Expecter{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: cert
func (_m *mockVerifier) Execute(cert *x509.Certificate) ([][]*x509.Certificate, error) {
	ret := _m.Called(cert)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 [][]*x509.Certificate
	var r1 error
	if rf, ok := ret.Get(0).(func(*x509.Certificate) ([][]*x509.Certificate, error)); ok {
		return rf(cert)
	}
	if rf, ok := ret.Get(0).(func(*x509.Certificate) [][]*x509.Certificate); ok {
		r0 = rf(cert)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([][]*x509.Certificate)
		}
	}

	if rf, ok := ret.Get(1).(func(*x509.Certificate) error); ok {
		r1 = rf(cert)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mockVerifier_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type mockVerifier_Execute_Call struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - cert *x509.Certificate
func (_e *mockVerifier_Expecter) Execute(cert interface{}) *mockVerifier_Execute_Call {
	return &mockVerifier_Execute_Call{Call: _e.mock.On("Execute", cert)}
}

func (_c *mockVerifier_Execute_Call) Run(run func(cert *x509.Certificate)) *mockVerifier_Execute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(*x509.Certificate))
	})
	return _c
}

func (_c *mockVerifier_Execute_Call) Return(_a0 [][]*x509.Certificate, _a1 error) *mockVerifier_Execute_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *mockVerifier_Execute_Call) RunAndReturn(run func(*x509.Certificate) ([][]*x509.Certificate, error)) *mockVerifier_Execute_Call {
	_c.Call.Return(run)
	return _c
}

// newMockVerifier creates a new instance of mockVerifier. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockVerifier(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockVerifier {
	mock := &mockVerifier{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
