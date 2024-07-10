//go:build !integration

package autoscaler

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/fleeting/fleeting/connector"
	fleetingmocks "gitlab.com/gitlab-org/fleeting/fleeting/connector/mocks"
	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	nestingapi "gitlab.com/gitlab-org/fleeting/nesting/api"
	nestingmocks "gitlab.com/gitlab-org/fleeting/nesting/api/mocks"
	"gitlab.com/gitlab-org/fleeting/taskscaler/mocks"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

func TestAcquisitionRef_Prepare(t *testing.T) {
	type assertClientFunc func(t *testing.T, c executors.Client)

	assertClient := func(fn func(t *testing.T, c executors.Client)) assertClientFunc {
		return func(t *testing.T, c executors.Client) {
			require.NotNil(t, c)
			fn(t, c)
		}
	}

	testNestingHost := "nesting-host"
	testBuildImageName := "build-image-name"
	testNestingCfgImageName := "nesting-cfg-image-name"
	testSlot := 8765
	testVM := &dummyVM{id: "id", name: "name", addr: "addr"}
	testTunnelClient := fleetingmocks.NewClient(t)
	testVariableValue := "test-variable-value"

	testCases := map[string]struct {
		doNotSetAcq        bool
		vmIsolationEnabled bool
		useExternalAddr    bool

		jobImage        string
		nestingCfgImage string

		dialAcquisitionInstanceCallExpected bool
		connectNestingCallExpected          bool
		dialTunnelCallExpected              bool

		mockDialerClose         bool
		mockNestingClientCreate bool
		mockNestingClientDelete bool

		instanceConnectInfoErr     error
		dialAcquisitionInstanceErr error
		connectNestingErr          error
		nestingCreateErr           error
		tunnelDialErr              error

		assertClient assertClientFunc

		expectedNestingConnCloseCall bool
		expectedNestingImage         string
		expectedError                error
	}{
		"ref.acq is not set": {
			doNotSetAcq:   true,
			expectedError: errRefAcqNotSet,
		},
		"Error when getting InstanceConnectInfo": {
			instanceConnectInfoErr: assert.AnError,
			expectedError:          assert.AnError,
		},
		"Error when dialing preparing instance dialer": {
			dialAcquisitionInstanceCallExpected: true,
			dialAcquisitionInstanceErr:          assert.AnError,
			expectedError:                       assert.AnError,
		},
		"No error and VM isolation disabled": {
			dialAcquisitionInstanceCallExpected: true,
			mockDialerClose:                     true,
			assertClient: assertClient(func(t *testing.T, c executors.Client) {
				cl, ok := c.(*client)
				require.True(t, ok, "expected to be %T, got %T", &client{}, c)

				assert.IsType(t, &fleetingmocks.Client{}, cl.client)
				assert.Nil(t, cl.cleanup)
			}),
		},
		"Error connecting to nesting": {
			vmIsolationEnabled:                  true,
			dialAcquisitionInstanceCallExpected: true,
			connectNestingErr:                   assert.AnError,
			expectedError:                       assert.AnError,
		},
		"Error when no image is specified": {
			vmIsolationEnabled:                  true,
			dialAcquisitionInstanceCallExpected: true,
			mockDialerClose:                     true,
			expectedNestingConnCloseCall:        true,
			expectedError:                       errNoNestingImageSpecified,
		},
		"Error when creating nesting VM": {
			vmIsolationEnabled:                  true,
			nestingCfgImage:                     testNestingCfgImageName,
			dialAcquisitionInstanceCallExpected: true,
			mockDialerClose:                     true,
			mockNestingClientCreate:             true,
			nestingCreateErr:                    assert.AnError,
			expectedNestingConnCloseCall:        true,
			expectedNestingImage:                testNestingCfgImageName,
			expectedError:                       assert.AnError,
		},
		"Error when dialing tunnel": {
			vmIsolationEnabled:                  true,
			jobImage:                            testBuildImageName,
			dialAcquisitionInstanceCallExpected: true,
			dialTunnelCallExpected:              true,
			mockDialerClose:                     true,
			mockNestingClientCreate:             true,
			tunnelDialErr:                       assert.AnError,
			expectedNestingConnCloseCall:        true,
			expectedNestingImage:                testBuildImageName,
			expectedError:                       assert.AnError,
		},
		"preparation completed": {
			vmIsolationEnabled:                  true,
			jobImage:                            testBuildImageName,
			nestingCfgImage:                     testNestingCfgImageName,
			dialAcquisitionInstanceCallExpected: true,
			mockDialerClose:                     true,
			mockNestingClientCreate:             true,
			mockNestingClientDelete:             true,
			assertClient: assertClient(func(t *testing.T, c executors.Client) {
				cl, ok := c.(*client)
				require.True(t, ok, "expected to be %T, got %T", &client{}, c)

				assert.Equal(t, testTunnelClient, cl.client)
				assert.NotNil(t, cl.cleanup)
			}),
			expectedNestingImage: testBuildImageName,
		},
		"variables expansion works for image": {
			vmIsolationEnabled:                  true,
			jobImage:                            "${TEST_VARIABLE}",
			dialAcquisitionInstanceCallExpected: true,
			mockDialerClose:                     true,
			mockNestingClientCreate:             true,
			mockNestingClientDelete:             true,
			assertClient: assertClient(func(t *testing.T, c executors.Client) {
				cl, ok := c.(*client)
				require.True(t, ok, "expected to be %T, got %T", &client{}, c)

				assert.Equal(t, testTunnelClient, cl.client)
				assert.NotNil(t, cl.cleanup)
			}),
			expectedNestingImage: testVariableValue,
		},
	}

	for testName, tc := range testCases {
		t.Run(testName, func(t *testing.T) {
			setAcq := !tc.doNotSetAcq

			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
			defer cancel()

			acq := mocks.NewAcquisition(t)

			fleetingDialer := fleetingmocks.NewClient(t)

			mockAcqInstD := &mockConnectorDialFn{
				expectCall: tc.dialAcquisitionInstanceCallExpected,
				dialer:     fleetingDialer,
				err:        tc.dialAcquisitionInstanceErr,
			}
			defer mockAcqInstD.verify(t, func(t *testing.T, _ fleetingprovider.ConnectInfo, options connector.DialOptions) {
				assert.Equal(t, tc.useExternalAddr, options.UseExternalAddr)
			})

			mockTunnelD := &mockConnectorDialFn{
				expectCall: tc.dialTunnelCallExpected,
				dialer:     testTunnelClient,
				err:        tc.tunnelDialErr,
			}

			nestingClient := nestingmocks.NewClient(t)
			nestingConn := &mockCloser{
				name:       "nestingConn",
				expectCall: tc.dialAcquisitionInstanceCallExpected && tc.expectedNestingConnCloseCall,
			}
			defer nestingConn.assertExpectations(t)

			//nolint:nestif
			if tc.vmIsolationEnabled {
				if tc.expectedError == nil {
					testTunnelClient.EXPECT().Close().Return(nil).Once()
				}

				if tc.mockDialerClose {
					fleetingDialer.EXPECT().Close().Return(nil).Once()
					nestingClient.EXPECT().Close().Return(nil).Once()
				}

				if tc.connectNestingErr != nil {
					fleetingDialer.EXPECT().Close().Return(nil).Once()
				}

				if tc.mockNestingClientCreate {
					nestingClient.EXPECT().Create(mock.Anything, tc.expectedNestingImage, int32Ref(int32(testSlot))).Return(testVM, stringRef("stomped"), tc.nestingCreateErr).Once()
				}

				if tc.tunnelDialErr != nil || tc.mockNestingClientDelete {
					nestingClient.EXPECT().Delete(mock.Anything, testVM.id).Return(nil).Once()
				}
			} else if tc.mockDialerClose {
				fleetingDialer.EXPECT().Close().Return(nil).Once()
			}

			logger, _ := test.NewNullLogger()
			bl := buildlogger.New(nil, logrus.NewEntry(logger), buildlogger.Options{})

			options := executorPrepareOptions(tc.jobImage, tc.nestingCfgImage, testNestingHost, testVariableValue)
			options.Config.Autoscaler.VMIsolation.Enabled = tc.vmIsolationEnabled
			options.Config.Autoscaler.ConnectorConfig.UseExternalAddr = tc.useExternalAddr

			ref := newAcquisitionRef("test-key", true)
			ref.dialAcquisitionInstance = mockAcqInstD.fn()
			ref.dialTunnel = mockTunnelD.fn()
			ref.connectNestingFn = func(host string, _ buildlogger.Logger, _ connector.Client) (nestingapi.Client, io.Closer, error) {
				assert.Equal(t, testNestingHost, host)
				return nestingClient, nestingConn, tc.connectNestingErr
			}

			if setAcq {
				acq.EXPECT().WithContext(ctx).Return(ctx, cancel)
				acq.EXPECT().InstanceConnectInfo(mock.Anything).Return(fleetingprovider.ConnectInfo{}, tc.instanceConnectInfoErr).Once()
				if tc.vmIsolationEnabled && tc.mockNestingClientCreate {
					acq.EXPECT().Slot().Return(testSlot).Once()
				}

				ref.acq = acq
			}

			c, err := ref.Prepare(ctx, bl, options)

			if tc.expectedError != nil {
				assert.Nil(t, c)
				assert.ErrorIs(t, err, tc.expectedError)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, tc.assertClient, "missing assertClient definition in the test case")
			tc.assertClient(t, c)

			require.NoError(t, c.Close())
		})
	}
}

func TestClientClose(t *testing.T) {
	cleanupError := fmt.Errorf("cleanup error")
	clientCloseError := fmt.Errorf("client close error")

	testCases := []struct {
		name     string
		cleanup  func() error
		closeErr error
		wantErr  error
	}{
		{
			name:     "No cleanup and client close without error",
			cleanup:  nil,
			closeErr: nil,
			wantErr:  nil,
		},
		{
			name:     "Cleanup with error and client close without error",
			cleanup:  func() error { return cleanupError },
			closeErr: nil,
			wantErr:  cleanupError,
		},
		{
			name:     "No cleanup and client close with error",
			cleanup:  nil,
			closeErr: clientCloseError,
			wantErr:  clientCloseError,
		},
		{
			name:     "Cleanup with error and client close with error",
			cleanup:  func() error { return cleanupError },
			closeErr: clientCloseError,
			wantErr:  clientCloseError,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mc := fleetingmocks.NewClient(t)
			mc.On("Close").Return(tc.closeErr)
			c := &client{
				client:  mc,
				cleanup: tc.cleanup,
			}

			err := c.Close()
			assert.Equal(t, tc.wantErr, err)
		})
	}
}

func executorPrepareOptions(buildImageName, nestingCfgImage, host, variableValue string) common.ExecutorPrepareOptions {
	return common.ExecutorPrepareOptions{
		Config: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Autoscaler: &common.AutoscalerConfig{
					VMIsolation: common.VMIsolation{
						NestingHost: host,
						Image:       nestingCfgImage,
					},
				},
			},
		},
		Build: &common.Build{
			JobResponse: common.JobResponse{
				Image: common.Image{
					Name: buildImageName,
				},
				Variables: common.JobVariables{
					{
						Key:    "TEST_VARIABLE",
						Value:  variableValue,
						Public: true,
					},
				},
			},
			Runner: &common.RunnerConfig{},
		},
	}
}

type mockConnectorDialFn struct {
	expectCall bool
	wasCalled  bool
	info       fleetingprovider.ConnectInfo
	options    connector.DialOptions
	dialer     connector.Client
	err        error
}

func (m *mockConnectorDialFn) fn() connector.DialFn {
	return func(
		ctx context.Context,
		info fleetingprovider.ConnectInfo,
		options connector.DialOptions,
	) (connector.Client, error) {
		m.wasCalled = true
		m.info = info
		m.options = options

		return m.dialer, m.err
	}
}

func (m *mockConnectorDialFn) verify(t *testing.T, fn func(t *testing.T, info fleetingprovider.ConnectInfo, options connector.DialOptions)) {
	assert.Equal(t, m.expectCall, m.wasCalled)
	fn(t, m.info, m.options)
}

type mockCloser struct {
	name       string
	expectCall bool
	wasCalled  bool
}

func (c *mockCloser) Close() error {
	c.wasCalled = true
	return nil
}

func (c *mockCloser) assertExpectations(t *testing.T) {
	if c.expectCall && !c.wasCalled {
		t.Errorf("expected %s closer to be called", c.name)
	}

	if !c.expectCall && c.wasCalled {
		t.Errorf("expected %s closer not to be called", c.name)
	}
}

type dummyVM struct {
	id   string
	name string
	addr string
}

// nolint:revive
func (vm *dummyVM) GetId() string { return vm.id }

func (vm *dummyVM) GetName() string { return vm.name }
func (vm *dummyVM) GetAddr() string { return vm.addr }

func int32Ref(i int32) *int32 {
	return &i
}

func stringRef(s string) *string {
	return &s
}
