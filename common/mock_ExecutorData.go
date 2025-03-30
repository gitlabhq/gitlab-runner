// Code generated by mockery v2.53.3. DO NOT EDIT.

package common

import mock "github.com/stretchr/testify/mock"

// MockExecutorData is an autogenerated mock type for the ExecutorData type
type MockExecutorData struct {
	mock.Mock
}

type MockExecutorData_Expecter struct {
	mock *mock.Mock
}

func (_m *MockExecutorData) EXPECT() *MockExecutorData_Expecter {
	return &MockExecutorData_Expecter{mock: &_m.Mock}
}

// NewMockExecutorData creates a new instance of MockExecutorData. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockExecutorData(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockExecutorData {
	mock := &MockExecutorData{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
