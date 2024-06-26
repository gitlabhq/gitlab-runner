// Code generated by mockery v2.43.0. DO NOT EDIT.

package pull

import (
	mock "github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
)

// MockManager is an autogenerated mock type for the Manager type
type MockManager struct {
	mock.Mock
}

// GetPullPolicyFor provides a mock function with given fields: image
func (_m *MockManager) GetPullPolicyFor(image string) (v1.PullPolicy, error) {
	ret := _m.Called(image)

	if len(ret) == 0 {
		panic("no return value specified for GetPullPolicyFor")
	}

	var r0 v1.PullPolicy
	var r1 error
	if rf, ok := ret.Get(0).(func(string) (v1.PullPolicy, error)); ok {
		return rf(image)
	}
	if rf, ok := ret.Get(0).(func(string) v1.PullPolicy); ok {
		r0 = rf(image)
	} else {
		r0 = ret.Get(0).(v1.PullPolicy)
	}

	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(image)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UpdatePolicyForImage provides a mock function with given fields: attempt, imagePullErr
func (_m *MockManager) UpdatePolicyForImage(attempt int, imagePullErr *ImagePullError) bool {
	ret := _m.Called(attempt, imagePullErr)

	if len(ret) == 0 {
		panic("no return value specified for UpdatePolicyForImage")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func(int, *ImagePullError) bool); ok {
		r0 = rf(attempt, imagePullErr)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// NewMockManager creates a new instance of MockManager. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockManager(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockManager {
	mock := &MockManager{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
