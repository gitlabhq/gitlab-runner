// Code generated by mockery v2.28.2. DO NOT EDIT.

package common

import (
	context "context"
	io "io"

	mock "github.com/stretchr/testify/mock"

	time "time"
)

// MockNetwork is an autogenerated mock type for the Network type
type MockNetwork struct {
	mock.Mock
}

// DownloadArtifacts provides a mock function with given fields: config, artifactsFile, directDownload
func (_m *MockNetwork) DownloadArtifacts(config JobCredentials, artifactsFile io.WriteCloser, directDownload *bool) DownloadState {
	ret := _m.Called(config, artifactsFile, directDownload)

	var r0 DownloadState
	if rf, ok := ret.Get(0).(func(JobCredentials, io.WriteCloser, *bool) DownloadState); ok {
		r0 = rf(config, artifactsFile, directDownload)
	} else {
		r0 = ret.Get(0).(DownloadState)
	}

	return r0
}

// PatchTrace provides a mock function with given fields: config, jobCredentials, content, startOffset, debugModeEnabled
func (_m *MockNetwork) PatchTrace(config RunnerConfig, jobCredentials *JobCredentials, content []byte, startOffset int, debugModeEnabled bool) PatchTraceResult {
	ret := _m.Called(config, jobCredentials, content, startOffset, debugModeEnabled)

	var r0 PatchTraceResult
	if rf, ok := ret.Get(0).(func(RunnerConfig, *JobCredentials, []byte, int, bool) PatchTraceResult); ok {
		r0 = rf(config, jobCredentials, content, startOffset, debugModeEnabled)
	} else {
		r0 = ret.Get(0).(PatchTraceResult)
	}

	return r0
}

// ProcessJob provides a mock function with given fields: config, buildCredentials
func (_m *MockNetwork) ProcessJob(config RunnerConfig, buildCredentials *JobCredentials) (JobTrace, error) {
	ret := _m.Called(config, buildCredentials)

	var r0 JobTrace
	var r1 error
	if rf, ok := ret.Get(0).(func(RunnerConfig, *JobCredentials) (JobTrace, error)); ok {
		return rf(config, buildCredentials)
	}
	if rf, ok := ret.Get(0).(func(RunnerConfig, *JobCredentials) JobTrace); ok {
		r0 = rf(config, buildCredentials)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(JobTrace)
		}
	}

	if rf, ok := ret.Get(1).(func(RunnerConfig, *JobCredentials) error); ok {
		r1 = rf(config, buildCredentials)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RegisterRunner provides a mock function with given fields: config, parameters
func (_m *MockNetwork) RegisterRunner(config RunnerCredentials, parameters RegisterRunnerParameters) *RegisterRunnerResponse {
	ret := _m.Called(config, parameters)

	var r0 *RegisterRunnerResponse
	if rf, ok := ret.Get(0).(func(RunnerCredentials, RegisterRunnerParameters) *RegisterRunnerResponse); ok {
		r0 = rf(config, parameters)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*RegisterRunnerResponse)
		}
	}

	return r0
}

// RequestJob provides a mock function with given fields: ctx, config, sessionInfo
func (_m *MockNetwork) RequestJob(ctx context.Context, config RunnerConfig, sessionInfo *SessionInfo) (*JobResponse, bool) {
	ret := _m.Called(ctx, config, sessionInfo)

	var r0 *JobResponse
	var r1 bool
	if rf, ok := ret.Get(0).(func(context.Context, RunnerConfig, *SessionInfo) (*JobResponse, bool)); ok {
		return rf(ctx, config, sessionInfo)
	}
	if rf, ok := ret.Get(0).(func(context.Context, RunnerConfig, *SessionInfo) *JobResponse); ok {
		r0 = rf(ctx, config, sessionInfo)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*JobResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, RunnerConfig, *SessionInfo) bool); ok {
		r1 = rf(ctx, config, sessionInfo)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// ResetToken provides a mock function with given fields: runner, systemID
func (_m *MockNetwork) ResetToken(runner RunnerCredentials, systemID string) *ResetTokenResponse {
	ret := _m.Called(runner, systemID)

	var r0 *ResetTokenResponse
	if rf, ok := ret.Get(0).(func(RunnerCredentials, string) *ResetTokenResponse); ok {
		r0 = rf(runner, systemID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*ResetTokenResponse)
		}
	}

	return r0
}

// ResetTokenWithPAT provides a mock function with given fields: runner, systemID, pat
func (_m *MockNetwork) ResetTokenWithPAT(runner RunnerCredentials, systemID string, pat string) *ResetTokenResponse {
	ret := _m.Called(runner, systemID, pat)

	var r0 *ResetTokenResponse
	if rf, ok := ret.Get(0).(func(RunnerCredentials, string, string) *ResetTokenResponse); ok {
		r0 = rf(runner, systemID, pat)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*ResetTokenResponse)
		}
	}

	return r0
}

// SetConnectionMaxAge provides a mock function with given fields: _a0
func (_m *MockNetwork) SetConnectionMaxAge(_a0 time.Duration) {
	_m.Called(_a0)
}

// UnregisterRunner provides a mock function with given fields: config
func (_m *MockNetwork) UnregisterRunner(config RunnerCredentials) bool {
	ret := _m.Called(config)

	var r0 bool
	if rf, ok := ret.Get(0).(func(RunnerCredentials) bool); ok {
		r0 = rf(config)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// UnregisterRunnerManager provides a mock function with given fields: config, systemID
func (_m *MockNetwork) UnregisterRunnerManager(config RunnerCredentials, systemID string) bool {
	ret := _m.Called(config, systemID)

	var r0 bool
	if rf, ok := ret.Get(0).(func(RunnerCredentials, string) bool); ok {
		r0 = rf(config, systemID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// UpdateJob provides a mock function with given fields: config, jobCredentials, jobInfo
func (_m *MockNetwork) UpdateJob(config RunnerConfig, jobCredentials *JobCredentials, jobInfo UpdateJobInfo) UpdateJobResult {
	ret := _m.Called(config, jobCredentials, jobInfo)

	var r0 UpdateJobResult
	if rf, ok := ret.Get(0).(func(RunnerConfig, *JobCredentials, UpdateJobInfo) UpdateJobResult); ok {
		r0 = rf(config, jobCredentials, jobInfo)
	} else {
		r0 = ret.Get(0).(UpdateJobResult)
	}

	return r0
}

// UploadRawArtifacts provides a mock function with given fields: config, reader, options
func (_m *MockNetwork) UploadRawArtifacts(config JobCredentials, reader io.ReadCloser, options ArtifactsOptions) (UploadState, string) {
	ret := _m.Called(config, reader, options)

	var r0 UploadState
	var r1 string
	if rf, ok := ret.Get(0).(func(JobCredentials, io.ReadCloser, ArtifactsOptions) (UploadState, string)); ok {
		return rf(config, reader, options)
	}
	if rf, ok := ret.Get(0).(func(JobCredentials, io.ReadCloser, ArtifactsOptions) UploadState); ok {
		r0 = rf(config, reader, options)
	} else {
		r0 = ret.Get(0).(UploadState)
	}

	if rf, ok := ret.Get(1).(func(JobCredentials, io.ReadCloser, ArtifactsOptions) string); ok {
		r1 = rf(config, reader, options)
	} else {
		r1 = ret.Get(1).(string)
	}

	return r0, r1
}

// VerifyRunner provides a mock function with given fields: config, systemID
func (_m *MockNetwork) VerifyRunner(config RunnerCredentials, systemID string) *VerifyRunnerResponse {
	ret := _m.Called(config, systemID)

	var r0 *VerifyRunnerResponse
	if rf, ok := ret.Get(0).(func(RunnerCredentials, string) *VerifyRunnerResponse); ok {
		r0 = rf(config, systemID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*VerifyRunnerResponse)
		}
	}

	return r0
}

type mockConstructorTestingTNewMockNetwork interface {
	mock.TestingT
	Cleanup(func())
}

// NewMockNetwork creates a new instance of MockNetwork. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewMockNetwork(t mockConstructorTestingTNewMockNetwork) *MockNetwork {
	mock := &MockNetwork{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
