// Code generated by mockery v2.53.3. DO NOT EDIT.

package azure

import (
	context "context"

	sas "github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	mock "github.com/stretchr/testify/mock"
)

// mockSasSigner is an autogenerated mock type for the sasSigner type
type mockSasSigner struct {
	mock.Mock
}

type mockSasSigner_Expecter struct {
	mock *mock.Mock
}

func (_m *mockSasSigner) EXPECT() *mockSasSigner_Expecter {
	return &mockSasSigner_Expecter{mock: &_m.Mock}
}

// Prepare provides a mock function with given fields: ctx, o
func (_m *mockSasSigner) Prepare(ctx context.Context, o *signedURLOptions) error {
	ret := _m.Called(ctx, o)

	if len(ret) == 0 {
		panic("no return value specified for Prepare")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *signedURLOptions) error); ok {
		r0 = rf(ctx, o)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// mockSasSigner_Prepare_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Prepare'
type mockSasSigner_Prepare_Call struct {
	*mock.Call
}

// Prepare is a helper method to define mock.On call
//   - ctx context.Context
//   - o *signedURLOptions
func (_e *mockSasSigner_Expecter) Prepare(ctx interface{}, o interface{}) *mockSasSigner_Prepare_Call {
	return &mockSasSigner_Prepare_Call{Call: _e.mock.On("Prepare", ctx, o)}
}

func (_c *mockSasSigner_Prepare_Call) Run(run func(ctx context.Context, o *signedURLOptions)) *mockSasSigner_Prepare_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*signedURLOptions))
	})
	return _c
}

func (_c *mockSasSigner_Prepare_Call) Return(_a0 error) *mockSasSigner_Prepare_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockSasSigner_Prepare_Call) RunAndReturn(run func(context.Context, *signedURLOptions) error) *mockSasSigner_Prepare_Call {
	_c.Call.Return(run)
	return _c
}

// ServiceURL provides a mock function with no fields
func (_m *mockSasSigner) ServiceURL() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for ServiceURL")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// mockSasSigner_ServiceURL_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'ServiceURL'
type mockSasSigner_ServiceURL_Call struct {
	*mock.Call
}

// ServiceURL is a helper method to define mock.On call
func (_e *mockSasSigner_Expecter) ServiceURL() *mockSasSigner_ServiceURL_Call {
	return &mockSasSigner_ServiceURL_Call{Call: _e.mock.On("ServiceURL")}
}

func (_c *mockSasSigner_ServiceURL_Call) Run(run func()) *mockSasSigner_ServiceURL_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *mockSasSigner_ServiceURL_Call) Return(_a0 string) *mockSasSigner_ServiceURL_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *mockSasSigner_ServiceURL_Call) RunAndReturn(run func() string) *mockSasSigner_ServiceURL_Call {
	_c.Call.Return(run)
	return _c
}

// Sign provides a mock function with given fields: values
func (_m *mockSasSigner) Sign(values sas.BlobSignatureValues) (sas.QueryParameters, error) {
	ret := _m.Called(values)

	if len(ret) == 0 {
		panic("no return value specified for Sign")
	}

	var r0 sas.QueryParameters
	var r1 error
	if rf, ok := ret.Get(0).(func(sas.BlobSignatureValues) (sas.QueryParameters, error)); ok {
		return rf(values)
	}
	if rf, ok := ret.Get(0).(func(sas.BlobSignatureValues) sas.QueryParameters); ok {
		r0 = rf(values)
	} else {
		r0 = ret.Get(0).(sas.QueryParameters)
	}

	if rf, ok := ret.Get(1).(func(sas.BlobSignatureValues) error); ok {
		r1 = rf(values)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// mockSasSigner_Sign_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Sign'
type mockSasSigner_Sign_Call struct {
	*mock.Call
}

// Sign is a helper method to define mock.On call
//   - values sas.BlobSignatureValues
func (_e *mockSasSigner_Expecter) Sign(values interface{}) *mockSasSigner_Sign_Call {
	return &mockSasSigner_Sign_Call{Call: _e.mock.On("Sign", values)}
}

func (_c *mockSasSigner_Sign_Call) Run(run func(values sas.BlobSignatureValues)) *mockSasSigner_Sign_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(sas.BlobSignatureValues))
	})
	return _c
}

func (_c *mockSasSigner_Sign_Call) Return(_a0 sas.QueryParameters, _a1 error) *mockSasSigner_Sign_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *mockSasSigner_Sign_Call) RunAndReturn(run func(sas.BlobSignatureValues) (sas.QueryParameters, error)) *mockSasSigner_Sign_Call {
	_c.Call.Return(run)
	return _c
}

// newMockSasSigner creates a new instance of mockSasSigner. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockSasSigner(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockSasSigner {
	mock := &mockSasSigner{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
