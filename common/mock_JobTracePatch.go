package common

import "github.com/stretchr/testify/mock"

type MockJobTracePatch struct {
	mock.Mock
}

func (m *MockJobTracePatch) Patch() []byte {
	ret := m.Called()

	var r0 []byte
	if ret.Get(0) != nil {
		r0 = ret.Get(0).([]byte)
	}

	return r0
}
func (m *MockJobTracePatch) Offset() int {
	ret := m.Called()

	r0 := ret.Get(0).(int)

	return r0
}
func (m *MockJobTracePatch) Limit() int {
	ret := m.Called()

	r0 := ret.Get(0).(int)

	return r0
}
func (m *MockJobTracePatch) SetNewOffset(newOffset int) {
	m.Called(newOffset)
}
func (m *MockJobTracePatch) ValidateRange() bool {
	ret := m.Called()

	r0 := ret.Get(0).(bool)

	return r0
}
