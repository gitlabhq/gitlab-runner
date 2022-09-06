// Code generated by mockery v2.14.0. DO NOT EDIT.

package helpers

import mock "github.com/stretchr/testify/mock"

// mockLogStreamProvider is an autogenerated mock type for the logStreamProvider type
type mockLogStreamProvider struct {
	mock.Mock
}

// Open provides a mock function with given fields:
func (_m *mockLogStreamProvider) Open() (readSeekCloser, error) {
	ret := _m.Called()

	var r0 readSeekCloser
	if rf, ok := ret.Get(0).(func() readSeekCloser); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(readSeekCloser)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

type mockConstructorTestingTnewMockLogStreamProvider interface {
	mock.TestingT
	Cleanup(func())
}

// newMockLogStreamProvider creates a new instance of mockLogStreamProvider. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func newMockLogStreamProvider(t mockConstructorTestingTnewMockLogStreamProvider) *mockLogStreamProvider {
	mock := &mockLogStreamProvider{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
