package common

import "github.com/stretchr/testify/mock"

type MockExecutor struct {
	mock.Mock
}

func (m *MockExecutor) Shell() *ShellScriptInfo {
	ret := m.Called()

	var r0 *ShellScriptInfo
	if ret.Get(0) != nil {
		r0 = ret.Get(0).(*ShellScriptInfo)
	}

	return r0
}
func (m *MockExecutor) Prepare(options ExecutorPrepareOptions) error {
	ret := m.Called(options)

	r0 := ret.Error(0)

	return r0
}
func (m *MockExecutor) Run(cmd ExecutorCommand) error {
	ret := m.Called(cmd)

	r0 := ret.Error(0)

	return r0
}
func (m *MockExecutor) Finish(err error) {
	m.Called(err)
}
func (m *MockExecutor) Cleanup() {
	m.Called()
}
func (m *MockExecutor) GetCurrentStage() ExecutorStage {
	ret := m.Called()

	r0 := ret.Get(0).(ExecutorStage)

	return r0
}
func (m *MockExecutor) SetCurrentStage(stage ExecutorStage) {
	m.Called(stage)
}
