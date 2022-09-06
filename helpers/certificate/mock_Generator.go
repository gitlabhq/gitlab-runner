// Code generated by mockery v2.14.0. DO NOT EDIT.

package certificate

import (
	tls "crypto/tls"

	mock "github.com/stretchr/testify/mock"
)

// MockGenerator is an autogenerated mock type for the Generator type
type MockGenerator struct {
	mock.Mock
}

// Generate provides a mock function with given fields: host
func (_m *MockGenerator) Generate(host string) (tls.Certificate, []byte, error) {
	ret := _m.Called(host)

	var r0 tls.Certificate
	if rf, ok := ret.Get(0).(func(string) tls.Certificate); ok {
		r0 = rf(host)
	} else {
		r0 = ret.Get(0).(tls.Certificate)
	}

	var r1 []byte
	if rf, ok := ret.Get(1).(func(string) []byte); ok {
		r1 = rf(host)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).([]byte)
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(string) error); ok {
		r2 = rf(host)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

type mockConstructorTestingTNewMockGenerator interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockGenerator creates a new instance of MockGenerator. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockGenerator(t mockConstructorTestingTNewMockGenerator) *MockGenerator {
	mock := &MockGenerator{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
