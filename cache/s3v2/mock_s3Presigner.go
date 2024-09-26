// Code generated by mockery v2.43.0. DO NOT EDIT.

package s3v2

import (
	context "context"

	cache "gitlab.com/gitlab-org/gitlab-runner/cache"

	mock "github.com/stretchr/testify/mock"

	time "time"
)

// mockS3Presigner is an autogenerated mock type for the s3Presigner type
type mockS3Presigner struct {
	mock.Mock
}

// FetchCredentialsForRole provides a mock function with given fields: ctx, roleARN, bucketName, objectName
func (_m *mockS3Presigner) FetchCredentialsForRole(ctx context.Context, roleARN string, bucketName string, objectName string) (map[string]string, error) {
	ret := _m.Called(ctx, roleARN, bucketName, objectName)

	if len(ret) == 0 {
		panic("no return value specified for FetchCredentialsForRole")
	}

	var r0 map[string]string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) (map[string]string, error)); ok {
		return rf(ctx, roleARN, bucketName, objectName)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string) map[string]string); ok {
		r0 = rf(ctx, roleARN, bucketName, objectName)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]string)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, string) error); ok {
		r1 = rf(ctx, roleARN, bucketName, objectName)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// PresignURL provides a mock function with given fields: ctx, method, bucketName, objectName, expires
func (_m *mockS3Presigner) PresignURL(ctx context.Context, method string, bucketName string, objectName string, expires time.Duration) (cache.PresignedURL, error) {
	ret := _m.Called(ctx, method, bucketName, objectName, expires)

	if len(ret) == 0 {
		panic("no return value specified for PresignURL")
	}

	var r0 cache.PresignedURL
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, time.Duration) (cache.PresignedURL, error)); ok {
		return rf(ctx, method, bucketName, objectName, expires)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, time.Duration) cache.PresignedURL); ok {
		r0 = rf(ctx, method, bucketName, objectName, expires)
	} else {
		r0 = ret.Get(0).(cache.PresignedURL)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, string, time.Duration) error); ok {
		r1 = rf(ctx, method, bucketName, objectName, expires)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// newMockS3Presigner creates a new instance of mockS3Presigner. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockS3Presigner(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockS3Presigner {
	mock := &mockS3Presigner{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
