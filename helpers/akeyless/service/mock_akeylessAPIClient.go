// Code generated by mockery v2.43.0. DO NOT EDIT.

package service

import (
	context "context"

	akeyless "github.com/akeylesslabs/akeyless-go/v4"

	mock "github.com/stretchr/testify/mock"
)

// mockAkeylessAPIClient is an autogenerated mock type for the akeylessAPIClient type
type mockAkeylessAPIClient struct {
	mock.Mock
}

// Auth provides a mock function with given fields: ctx, params
func (_m *mockAkeylessAPIClient) Auth(ctx context.Context, params akeyless.Auth) (akeyless.AuthOutput, error) {
	ret := _m.Called(ctx, params)

	if len(ret) == 0 {
		panic("no return value specified for Auth")
	}

	var r0 akeyless.AuthOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, akeyless.Auth) (akeyless.AuthOutput, error)); ok {
		return rf(ctx, params)
	}
	if rf, ok := ret.Get(0).(func(context.Context, akeyless.Auth) akeyless.AuthOutput); ok {
		r0 = rf(ctx, params)
	} else {
		r0 = ret.Get(0).(akeyless.AuthOutput)
	}

	if rf, ok := ret.Get(1).(func(context.Context, akeyless.Auth) error); ok {
		r1 = rf(ctx, params)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DescribeItem provides a mock function with given fields: ctx, params
func (_m *mockAkeylessAPIClient) DescribeItem(ctx context.Context, params akeyless.DescribeItem) (akeyless.Item, error) {
	ret := _m.Called(ctx, params)

	if len(ret) == 0 {
		panic("no return value specified for DescribeItem")
	}

	var r0 akeyless.Item
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, akeyless.DescribeItem) (akeyless.Item, error)); ok {
		return rf(ctx, params)
	}
	if rf, ok := ret.Get(0).(func(context.Context, akeyless.DescribeItem) akeyless.Item); ok {
		r0 = rf(ctx, params)
	} else {
		r0 = ret.Get(0).(akeyless.Item)
	}

	if rf, ok := ret.Get(1).(func(context.Context, akeyless.DescribeItem) error); ok {
		r1 = rf(ctx, params)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetDynamicSecretValue provides a mock function with given fields: ctx, params
func (_m *mockAkeylessAPIClient) GetDynamicSecretValue(ctx context.Context, params akeyless.GetDynamicSecretValue) (map[string]interface{}, error) {
	ret := _m.Called(ctx, params)

	if len(ret) == 0 {
		panic("no return value specified for GetDynamicSecretValue")
	}

	var r0 map[string]interface{}
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, akeyless.GetDynamicSecretValue) (map[string]interface{}, error)); ok {
		return rf(ctx, params)
	}
	if rf, ok := ret.Get(0).(func(context.Context, akeyless.GetDynamicSecretValue) map[string]interface{}); ok {
		r0 = rf(ctx, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]interface{})
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, akeyless.GetDynamicSecretValue) error); ok {
		r1 = rf(ctx, params)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPKICertificate provides a mock function with given fields: ctx, params
func (_m *mockAkeylessAPIClient) GetPKICertificate(ctx context.Context, params akeyless.GetPKICertificate) (akeyless.GetPKICertificateOutput, error) {
	ret := _m.Called(ctx, params)

	if len(ret) == 0 {
		panic("no return value specified for GetPKICertificate")
	}

	var r0 akeyless.GetPKICertificateOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, akeyless.GetPKICertificate) (akeyless.GetPKICertificateOutput, error)); ok {
		return rf(ctx, params)
	}
	if rf, ok := ret.Get(0).(func(context.Context, akeyless.GetPKICertificate) akeyless.GetPKICertificateOutput); ok {
		r0 = rf(ctx, params)
	} else {
		r0 = ret.Get(0).(akeyless.GetPKICertificateOutput)
	}

	if rf, ok := ret.Get(1).(func(context.Context, akeyless.GetPKICertificate) error); ok {
		r1 = rf(ctx, params)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetRotatedSecretValue provides a mock function with given fields: ctx, params
func (_m *mockAkeylessAPIClient) GetRotatedSecretValue(ctx context.Context, params akeyless.GetRotatedSecretValue) (map[string]interface{}, error) {
	ret := _m.Called(ctx, params)

	if len(ret) == 0 {
		panic("no return value specified for GetRotatedSecretValue")
	}

	var r0 map[string]interface{}
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, akeyless.GetRotatedSecretValue) (map[string]interface{}, error)); ok {
		return rf(ctx, params)
	}
	if rf, ok := ret.Get(0).(func(context.Context, akeyless.GetRotatedSecretValue) map[string]interface{}); ok {
		r0 = rf(ctx, params)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]interface{})
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, akeyless.GetRotatedSecretValue) error); ok {
		r1 = rf(ctx, params)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetSSHCertificate provides a mock function with given fields: ctx, params
func (_m *mockAkeylessAPIClient) GetSSHCertificate(ctx context.Context, params akeyless.GetSSHCertificate) (akeyless.GetSSHCertificateOutput, error) {
	ret := _m.Called(ctx, params)

	if len(ret) == 0 {
		panic("no return value specified for GetSSHCertificate")
	}

	var r0 akeyless.GetSSHCertificateOutput
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, akeyless.GetSSHCertificate) (akeyless.GetSSHCertificateOutput, error)); ok {
		return rf(ctx, params)
	}
	if rf, ok := ret.Get(0).(func(context.Context, akeyless.GetSSHCertificate) akeyless.GetSSHCertificateOutput); ok {
		r0 = rf(ctx, params)
	} else {
		r0 = ret.Get(0).(akeyless.GetSSHCertificateOutput)
	}

	if rf, ok := ret.Get(1).(func(context.Context, akeyless.GetSSHCertificate) error); ok {
		r1 = rf(ctx, params)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetSecretValue provides a mock function with given fields: ctx, body
func (_m *mockAkeylessAPIClient) GetSecretValue(ctx context.Context, body akeyless.GetSecretValue) (map[string]interface{}, error) {
	ret := _m.Called(ctx, body)

	if len(ret) == 0 {
		panic("no return value specified for GetSecretValue")
	}

	var r0 map[string]interface{}
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, akeyless.GetSecretValue) (map[string]interface{}, error)); ok {
		return rf(ctx, body)
	}
	if rf, ok := ret.Get(0).(func(context.Context, akeyless.GetSecretValue) map[string]interface{}); ok {
		r0 = rf(ctx, body)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]interface{})
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, akeyless.GetSecretValue) error); ok {
		r1 = rf(ctx, body)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// newMockAkeylessAPIClient creates a new instance of mockAkeylessAPIClient. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockAkeylessAPIClient(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockAkeylessAPIClient {
	mock := &mockAkeylessAPIClient{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
