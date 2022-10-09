//go:build !integration

package networks

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/labels"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

func newDebugLoggerMock() *mockDebugLogger {
	loggerMock := new(mockDebugLogger)
	loggerMock.On("Debugln", mock.Anything, mock.Anything)

	return loggerMock
}

func TestNewDefaultManager(t *testing.T) {
	logger := newDebugLoggerMock()

	m := NewManager(logger, nil, nil, nil)
	assert.IsType(t, &manager{}, m)
}

func newDefaultManager() *manager {
	b := &common.Build{
		ProjectRunnerID: 0,
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{Token: "test-token"},
		},
		JobResponse: common.JobResponse{
			JobInfo: common.JobInfo{
				ProjectID: 0,
			},
		},
	}
	m := &manager{
		logger:  newDebugLoggerMock(),
		build:   b,
		labeler: labels.NewLabeler(b),
	}
	return m
}

func addClient(manager *manager) *docker.MockClient {
	client := new(docker.MockClient)

	manager.client = client

	return client
}

func TestCreateNetwork(t *testing.T) {
	testCases := map[string]struct {
		networkMode         string
		networkPerBuild     string
		buildNetwork        types.NetworkResource
		enableIPv6          bool
		expectedNetworkMode container.NetworkMode
		expectedErr         error
		clientAssertions    func(*docker.MockClient)
	}{
		"network specified": {
			networkMode:         "default",
			expectedNetworkMode: container.NetworkMode("default"),
		},
		"network create per build with network mode": {
			networkMode:         "default",
			networkPerBuild:     "true",
			expectedNetworkMode: container.NetworkMode("default"),
		},
		"network per-build flag off": {
			networkMode:         "",
			networkPerBuild:     "false",
			expectedNetworkMode: container.NetworkMode(""),
		},
		"network create per-build network": {
			networkMode:         "",
			networkPerBuild:     "true",
			expectedNetworkMode: container.NetworkMode("runner-test-tok-project-0-concurrent-0-job-0-network"),
			clientAssertions: func(mc *docker.MockClient) {
				mc.On(
					"NetworkCreate",
					mock.Anything,
					mock.AnythingOfType("string"),
					mock.AnythingOfType("types.NetworkCreate"),
				).
					Return(types.NetworkCreateResponse{ID: "test-network"}, nil).
					Once()
				mc.On("NetworkInspect", mock.Anything, mock.AnythingOfType("string")).
					Return(types.NetworkResource{
						ID:   "test-network",
						Name: "test-network",
					}, nil).
					Once()
			},
		},
		"network create per-build network failure": {
			networkMode:         "",
			networkPerBuild:     "true",
			expectedNetworkMode: "",
			expectedErr:         errors.New("test-network failed"),
			clientAssertions: func(mc *docker.MockClient) {
				mc.On(
					"NetworkCreate",
					mock.Anything,
					mock.AnythingOfType("string"),
					mock.AnythingOfType("types.NetworkCreate"),
				).
					Return(types.NetworkCreateResponse{ID: "test-network"}, errors.New("test-network failed")).
					Once()
			},
		},
		"network create per-build inspect failure": {
			networkMode:         "",
			networkPerBuild:     "true",
			expectedNetworkMode: "",
			expectedErr:         errors.New("network-inspect-failed"),
			clientAssertions: func(mc *docker.MockClient) {
				mc.On(
					"NetworkCreate",
					mock.Anything,
					mock.AnythingOfType("string"),
					mock.AnythingOfType("types.NetworkCreate"),
				).
					Return(types.NetworkCreateResponse{ID: "test-network"}, nil).
					Once()
				mc.On(
					"NetworkInspect",
					mock.Anything,
					mock.AnythingOfType("string"),
				).
					Return(types.NetworkResource{}, errors.New("network-inspect-failed")).
					Once()
			},
		},
		"networkID already set": {
			networkMode:     "",
			networkPerBuild: "true",
			buildNetwork: types.NetworkResource{
				ID: "some-id",
			},
			expectedNetworkMode: "",
			expectedErr:         errBuildNetworkExists,
		},
		"IPv6 network created": {
			networkMode:         "",
			networkPerBuild:     "true",
			expectedNetworkMode: container.NetworkMode("runner-test-tok-project-0-concurrent-0-job-0-network"),
			enableIPv6:          true,
			clientAssertions: func(mc *docker.MockClient) {
				mc.On(
					"NetworkCreate",
					mock.Anything,
					mock.AnythingOfType("string"),
					mock.AnythingOfType("types.NetworkCreate"),
				).
					Return(types.NetworkCreateResponse{ID: "test-network"}, nil).
					Once()
				mc.On("NetworkInspect", mock.Anything, mock.AnythingOfType("string")).
					Return(types.NetworkResource{
						ID:         "test-network",
						Name:       "test-network",
						EnableIPv6: true,
					}, nil).
					Once()
			},
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			m := newDefaultManager()
			m.build.ID = 0
			m.buildNetwork = testCase.buildNetwork

			client := addClient(m)
			defer client.AssertExpectations(t)

			m.build.Variables = append(m.build.Variables, common.JobVariable{
				Key:   featureflags.NetworkPerBuild,
				Value: testCase.networkPerBuild,
			})

			if testCase.clientAssertions != nil {
				testCase.clientAssertions(client)
			}

			networkMode, err := m.Create(context.Background(), testCase.networkMode, testCase.enableIPv6)

			assert.Equal(t, testCase.expectedNetworkMode, networkMode)
			assert.Equal(t, testCase.expectedErr, err)
		})
	}
}

func TestInspectNetwork(t *testing.T) {
	networkName := "test-network"
	testError := errors.New("failure")

	testCases := map[string]struct {
		perBuild         bool
		clientAssertions func(client *docker.MockClient)
		expectedResponse types.NetworkResource
		expectedErr      error
	}{
		"network per build": {
			perBuild:         false,
			expectedResponse: types.NetworkResource{},
			expectedErr:      nil,
		},
		"no network per build": {
			perBuild: true,
			clientAssertions: func(m *docker.MockClient) {
				m.On("NetworkInspect", mock.Anything, mock.AnythingOfType("string")).
					Return(types.NetworkResource{
						ID:   networkName,
						Name: networkName,
					}, nil).
					Once()
			},
			expectedResponse: types.NetworkResource{
				ID:   networkName,
				Name: networkName,
			},
			expectedErr: nil,
		},
		"network inspect failed": {
			perBuild: true,
			clientAssertions: func(m *docker.MockClient) {
				m.On("NetworkInspect", mock.Anything, mock.AnythingOfType("string")).
					Return(types.NetworkResource{}, testError)
			},
			expectedErr: testError,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			m := newDefaultManager()
			m.perBuild = testCase.perBuild
			client := addClient(m)
			defer client.AssertExpectations(t)

			if testCase.clientAssertions != nil {
				testCase.clientAssertions(client)
			}

			inspectResponse, err := m.Inspect(context.Background())

			assert.Equal(t, testCase.expectedResponse, inspectResponse)
			assert.ErrorIs(t, err, testCase.expectedErr)
		})
	}
}

func TestCleanupNetwork(t *testing.T) {
	testErr := errors.New("test-error")

	testCases := map[string]struct {
		networkMode      string
		networkPerBuild  string
		clientAssertions func(*docker.MockClient)
		expectErr        error
	}{
		"network per-build flag off": {
			networkPerBuild: "false",
		},
		"network per-build flag on with defined network": {
			networkPerBuild: "true",
			networkMode:     "default",
		},
		"cleanup per-build network": {
			networkPerBuild: "true",
			clientAssertions: func(mc *docker.MockClient) {
				mc.On("NetworkRemove", mock.Anything, mock.AnythingOfType("string")).
					Return(nil).
					Once()
			},
		},
		"cleanup per-build error": {
			networkPerBuild: "true",
			clientAssertions: func(mc *docker.MockClient) {
				mc.On("NetworkRemove", mock.Anything, mock.AnythingOfType("string")).
					Return(testErr).
					Once()
			},
			expectErr: testErr,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			m := newDefaultManager()
			m.build.ID = 0

			client := addClient(m)
			defer client.AssertExpectations(t)

			m.build.Variables = append(m.build.Variables, common.JobVariable{
				Key:   featureflags.NetworkPerBuild,
				Value: testCase.networkPerBuild,
			})

			if testCase.networkPerBuild == "true" {
				if testCase.networkMode == "" {
					m.perBuild = true
				}
			}

			if testCase.clientAssertions != nil {
				testCase.clientAssertions(client)
			}

			err := m.Cleanup(context.Background())
			assert.ErrorIs(t, err, testCase.expectErr)
		})
	}
}
