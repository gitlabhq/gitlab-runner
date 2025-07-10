//go:build !integration

package networks

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/labels"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

func TestNewDefaultManager(t *testing.T) {
	logger := newMockDebugLogger(t)

	m := NewManager(logger, nil, nil, nil)
	assert.IsType(t, &manager{}, m)
}

func newDefaultManager(t *testing.T) *manager {
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

	loggerMock := newMockDebugLogger(t)
	loggerMock.On("Debugln", mock.Anything, mock.Anything).Maybe()

	m := &manager{
		logger:  loggerMock,
		build:   b,
		labeler: labels.NewLabeler(b),
	}
	return m
}

func addClient(t *testing.T, manager *manager) *docker.MockClient {
	client := docker.NewMockClient(t)
	manager.client = client

	return client
}

func TestCreateNetwork(t *testing.T) {
	testCases := map[string]struct {
		networkMode         string
		networkPerBuild     string
		buildNetwork        network.Inspect
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
			expectedNetworkMode: container.NetworkMode("runner-test-toke-0-0-0"),
			clientAssertions: func(mc *docker.MockClient) {
				mc.On(
					"NetworkCreate",
					mock.Anything,
					mock.AnythingOfType("string"),
					mock.AnythingOfType("network.CreateOptions"),
				).
					Return(network.CreateResponse{ID: "test-network"}, nil).
					Once()
				mc.On("NetworkInspect", mock.Anything, mock.AnythingOfType("string")).
					Return(network.Inspect{
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
					mock.AnythingOfType("network.CreateOptions"),
				).
					Return(network.CreateResponse{ID: "test-network"}, errors.New("test-network failed")).
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
					mock.AnythingOfType("network.CreateOptions"),
				).
					Return(network.CreateResponse{ID: "test-network"}, nil).
					Once()
				mc.On(
					"NetworkInspect",
					mock.Anything,
					mock.AnythingOfType("string"),
				).
					Return(network.Inspect{}, errors.New("network-inspect-failed")).
					Once()
			},
		},
		"networkID already set": {
			networkMode:     "",
			networkPerBuild: "true",
			buildNetwork: network.Inspect{
				ID: "some-id",
			},
			expectedNetworkMode: "",
			expectedErr:         errBuildNetworkExists,
		},
		"IPv6 network created": {
			networkMode:         "",
			networkPerBuild:     "true",
			expectedNetworkMode: container.NetworkMode("runner-test-toke-0-0-0"),
			enableIPv6:          true,
			clientAssertions: func(mc *docker.MockClient) {
				mc.On(
					"NetworkCreate",
					mock.Anything,
					mock.AnythingOfType("string"),
					mock.AnythingOfType("network.CreateOptions"),
				).
					Return(network.CreateResponse{ID: "test-network"}, nil).
					Once()
				mc.On("NetworkInspect", mock.Anything, mock.AnythingOfType("string")).
					Return(network.Inspect{
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
			m := newDefaultManager(t)
			m.build.ID = 0
			m.buildNetwork = testCase.buildNetwork

			client := addClient(t, m)

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

func TestCreateNetworkWithCustomMTU(t *testing.T) {
	testCases := map[string]struct {
		networkPerBuild bool
		mtu             int
		expectedMTU     int
	}{
		"feature-flag is enabled, with mtu": {
			networkPerBuild: true,
			mtu:             1402,
			expectedMTU:     1402,
		},
		"feature-flag is enabled, no mtu": {
			networkPerBuild: true,
		},
		"feature-flag disabled": {
			mtu: 1234,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			m := newDefaultManager(t)
			m.build.ID = 0

			client := addClient(t, m)

			m.build.Runner.Docker = &common.DockerConfig{NetworkMTU: testCase.mtu}

			var receivedMTU int

			if testCase.networkPerBuild {
				m.build.Variables = append(m.build.Variables, common.JobVariable{
					Key:   featureflags.NetworkPerBuild,
					Value: "true",
				})

				client.On("NetworkCreate", mock.Anything, mock.AnythingOfType("string"), mock.AnythingOfType("network.CreateOptions")).
					Run(func(args mock.Arguments) {
						arg, ok := args.Get(2).(network.CreateOptions)
						require.True(t, ok)

						if testCase.mtu != 0 {
							mtu, ok := arg.Options["com.docker.network.driver.mtu"]
							require.True(t, ok)
							var err error
							receivedMTU, err = strconv.Atoi(mtu)
							assert.NoError(t, err)
						} else {
							_, ok := arg.Options["com.docker.network.driver.mtu"]
							require.False(t, ok)
						}
					}).
					Return(network.CreateResponse{ID: "test-network"}, nil).
					Once()

				client.On("NetworkInspect", mock.Anything, mock.AnythingOfType("string")).
					Return(network.Inspect{
						ID:   "test-network",
						Name: "test-network",
					}, nil).
					Once()
			}

			_, err := m.Create(context.Background(), "", false)

			assert.Equal(t, testCase.expectedMTU, receivedMTU)
			assert.NoError(t, err)
		})
	}
}

func TestInspectNetwork(t *testing.T) {
	networkName := "test-network"
	testError := errors.New("failure")

	testCases := map[string]struct {
		perBuild         bool
		clientAssertions func(client *docker.MockClient)
		expectedResponse network.Inspect
		expectedErr      error
	}{
		"network per build": {
			perBuild:         false,
			expectedResponse: network.Inspect{},
			expectedErr:      nil,
		},
		"no network per build": {
			perBuild: true,
			clientAssertions: func(m *docker.MockClient) {
				m.On("NetworkInspect", mock.Anything, mock.AnythingOfType("string")).
					Return(network.Inspect{
						ID:   networkName,
						Name: networkName,
					}, nil).
					Once()
			},
			expectedResponse: network.Inspect{
				ID:   networkName,
				Name: networkName,
			},
			expectedErr: nil,
		},
		"network inspect failed": {
			perBuild: true,
			clientAssertions: func(m *docker.MockClient) {
				m.On("NetworkInspect", mock.Anything, mock.AnythingOfType("string")).
					Return(network.Inspect{}, testError)
			},
			expectedErr: testError,
		},
	}

	for testName, testCase := range testCases {
		t.Run(testName, func(t *testing.T) {
			m := newDefaultManager(t)
			m.perBuild = testCase.perBuild
			client := addClient(t, m)

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
			m := newDefaultManager(t)
			m.build.ID = 0

			client := addClient(t, m)

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
