package common

import "github.com/stretchr/testify/mock"

import "context"

type MockJobTrace struct {
	mock.Mock
}

func (m *MockJobTrace) Success() {
	m.Called()
}
func (m *MockJobTrace) Fail(err error) {
	m.Called(err)
}
func (m *MockJobTrace) SetCancelFunc(cancelFunc context.CancelFunc) {
	m.Called(cancelFunc)
}
func (m *MockJobTrace) IsStdout() bool {
	ret := m.Called()

	r0 := ret.Get(0).(bool)

	return r0
}
