//go:build !integration

package autoscaler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
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
	"gitlab.com/gitlab-org/fleeting/taskscaler"
	"gitlab.com/gitlab-org/fleeting/taskscaler/mocks"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
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

	// netDialErr is a realistic network dial error: connection refused on a TCP dial.
	// It implements net.Error so the autoscaler treats it as an instance-level failure.
	netDialErr := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}

	// expiredConnectInfoErr simulates fleeting's ErrConnectInfoExpired shape: the
	// connect-info token expired, which is a config/timing error, not an instance failure.
	expiredConnectInfoErr := fmt.Errorf("%w: %w", context.DeadlineExceeded, connector.ErrConnectInfoExpired)

	// sshAuthErr simulates a failed SSH authentication attempt. It is deliberately
	// not a net.Error so it is classified as a config error, not an instance failure.
	sshAuthErr := errors.New("ssh: handshake failed: ssh: unable to authenticate, attempted methods [none], no supported methods remain")

	// keyParseErr simulates ssh.ParsePrivateKey failing on a bad key. Same class.
	keyParseErr := errors.New("ssh: cannot decode encrypted private keys")

	// sshHandshakeReadTimeoutErr simulates a handshake i/o timeout: TCP connected
	// (Op "read") but the SSH negotiation timed out. The host is alive, so this
	// must NOT trigger HealthFailure. This matches the actual error x/crypto/ssh
	// returns when conn.SetDeadline fires during NewClientConn.
	sshHandshakeReadTimeoutErr := fmt.Errorf("ssh: handshake failed: %w",
		&net.OpError{Op: "read", Net: "tcp", Err: errors.New("i/o timeout")})

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

		// expectHealthFailure is true when the dial/connect-info failure should be
		// reported to taskscaler via HealthFailure (i.e. the instance is unreachable).
		expectHealthFailure bool
		// expectInstanceUnavailable is true when the returned error should be wrapped
		// as an instanceUnavailableError, causing the executor to drop the acquisition.
		expectInstanceUnavailable bool
	}{
		"ref.acq is not set": {
			doNotSetAcq:   true,
			expectedError: errRefAcqNotSet,
		},
		"ConnectInfo error": {
			instanceConnectInfoErr:    assert.AnError,
			expectedError:             assert.AnError,
			expectHealthFailure:       true,
			expectInstanceUnavailable: true,
		},
		// net.OpError from a TCP dial is an instance-level failure: the host is unreachable.
		"net.OpError dial error": {
			dialAcquisitionInstanceCallExpected: true,
			dialAcquisitionInstanceErr:          netDialErr,
			expectedError:                       netDialErr,
			expectHealthFailure:                 true,
			expectInstanceUnavailable:           true,
		},
		// net.Dialer.DialContext wraps a connect-phase deadline as *net.OpError{Op:"dial"},
		// so a dial timeout is an instance failure. fleeting further wraps the OpError with
		// a retry-count prefix; errors.AsType still finds the inner *net.OpError.
		"dial-phase context.DeadlineExceeded (connect timeout)": {
			dialAcquisitionInstanceCallExpected: true,
			dialAcquisitionInstanceErr: fmt.Errorf("after retrying 0 times during 5s timeout: %w",
				&net.OpError{Op: "dial", Net: "tcp", Err: context.DeadlineExceeded}),
			expectedError:             context.DeadlineExceeded,
			expectHealthFailure:       true,
			expectInstanceUnavailable: true,
		},
		// SSH auth failure is a config error, not an instance failure: do not call
		// HealthFailure and do not wrap as instanceUnavailableError.
		"SSH auth error": {
			dialAcquisitionInstanceCallExpected: true,
			dialAcquisitionInstanceErr:          sshAuthErr,
			expectedError:                       sshAuthErr,
			expectHealthFailure:                 false,
			expectInstanceUnavailable:           false,
		},
		// SSH handshake i/o timeout: TCP connected (Op "read"), host is alive.
		// fleeting's ssh.go retries "handshake failed" errors until ioTimeout runs
		// out, after which the last attempt fails with a read-phase OpError.
		// This must NOT trigger HealthFailure — the instance accepted TCP.
		"SSH handshake read timeout": {
			dialAcquisitionInstanceCallExpected: true,
			dialAcquisitionInstanceErr:          sshHandshakeReadTimeoutErr,
			expectedError:                       sshHandshakeReadTimeoutErr,
			expectHealthFailure:                 false,
			expectInstanceUnavailable:           false,
		},
		// Key parse failure is a config error, not an instance failure.
		"key parse error": {
			dialAcquisitionInstanceCallExpected: true,
			dialAcquisitionInstanceErr:          keyParseErr,
			expectedError:                       keyParseErr,
			expectHealthFailure:                 false,
			expectInstanceUnavailable:           false,
		},
		// ErrConnectInfoExpired is a config/timing error; the underlying TCP error must
		// not be mistaken for an unreachable host. Check the ErrConnectInfoExpired branch
		// before the net.Error branch so the order in production code is exercised.
		"expired connect info error": {
			dialAcquisitionInstanceCallExpected: true,
			dialAcquisitionInstanceErr:          expiredConnectInfoErr,
			expectedError:                       connector.ErrConnectInfoExpired,
			expectHealthFailure:                 false,
			expectInstanceUnavailable:           false,
		},
		"No error and VM isolation disabled": {
			dialAcquisitionInstanceCallExpected: true,
			mockDialerClose:                     true,
			assertClient: assertClient(func(t *testing.T, c executors.Client) {
				cl, ok := c.(*client)
				require.True(t, ok, "expected to be %T, got %T", &client{}, c)

				assert.IsType(t, fleetingmocks.NewClient(t), cl.client)
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

			ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(time.Minute))
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
					nestingClient.EXPECT().Create(mock.Anything, tc.expectedNestingImage, new(int32(testSlot))).Return(testVM, new("stomped"), tc.nestingCreateErr).Once()
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
				if tc.expectHealthFailure {
					acq.EXPECT().HealthFailure().Return().Once()
				}
				// a successful dial proves the instance is reachable
				if tc.dialAcquisitionInstanceCallExpected && tc.dialAcquisitionInstanceErr == nil {
					acq.EXPECT().HealthSuccess().Return().Once()
				}
				if tc.vmIsolationEnabled && tc.mockNestingClientCreate {
					acq.EXPECT().Slot().Return(testSlot).Once()
				}

				ref.acq = acq
			}

			c, err := ref.Prepare(ctx, bl, options)

			if tc.expectedError != nil {
				assert.Nil(t, c)
				assert.ErrorIs(t, err, tc.expectedError)
				assert.Equal(t, tc.expectInstanceUnavailable, isInstanceUnavailable(err))
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
			Job: spec.Job{
				Image: spec.Image{
					Name: buildImageName,
				},
				Variables: spec.Variables{
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

func TestAcquisitionRef_AcquisitionKey(t *testing.T) {
	assert.Equal(t, "test-key", newAcquisitionRef("test-key", false).AcquisitionKey())
}

func TestAcquisitionRef_AcquisitionSlot(t *testing.T) {
	tests := []struct {
		name         string
		acq          taskscaler.Acquisition
		expectedSlot int
	}{
		{
			name: "returns slot when acquisition set",
			acq: func() taskscaler.Acquisition {
				acq := mocks.NewAcquisition(t)
				acq.EXPECT().Slot().Return(42)
				return acq
			}(),
			expectedSlot: 42,
		},
		{
			name:         "returns -1 when acquisition not set",
			acq:          nil,
			expectedSlot: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref := &acquisitionRef{
				key: "test-key",
				acq: tt.acq,
			}

			result := ref.AcquisitionSlot()

			assert.Equal(t, tt.expectedSlot, result)
		})
	}
}

func TestAcquisitionRef_Prepare_SlotCgroupEnvironmentVariable(t *testing.T) {
	tests := []struct {
		name                  string
		useSlotCgroups        bool
		slotCgroupTemplate    string
		slot                  int
		expectedVariableValue string
		expectVariable        bool
	}{
		{
			name:                  "adds GITLAB_RUNNER_SLOT_CGROUP when use_slot_cgroups is true",
			useSlotCgroups:        true,
			slotCgroupTemplate:    "gitlab-runner/slot-${slot}",
			slot:                  5,
			expectedVariableValue: "gitlab-runner/slot-5",
			expectVariable:        true,
		},
		{
			name:                  "uses default template when slot_cgroup_template is empty",
			useSlotCgroups:        true,
			slotCgroupTemplate:    "",
			slot:                  10,
			expectedVariableValue: "gitlab-runner/slot-10",
			expectVariable:        true,
		},
		{
			name:                  "does not add variable when use_slot_cgroups is false",
			useSlotCgroups:        false,
			slotCgroupTemplate:    "gitlab-runner/slot-${slot}",
			slot:                  5,
			expectedVariableValue: "",
			expectVariable:        false,
		},
		{
			name:                  "does not add variable when slot is negative",
			useSlotCgroups:        true,
			slotCgroupTemplate:    "gitlab-runner/slot-${slot}",
			slot:                  -1,
			expectedVariableValue: "",
			expectVariable:        false,
		},
		{
			name:                  "adds variable with custom template",
			useSlotCgroups:        true,
			slotCgroupTemplate:    "custom/runner-${slot}",
			slot:                  3,
			expectedVariableValue: "custom/runner-3",
			expectVariable:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithDeadline(t.Context(), time.Now().Add(time.Minute))
			defer cancel()

			acq := mocks.NewAcquisition(t)
			acq.EXPECT().WithContext(ctx).Return(ctx, cancel)
			acq.EXPECT().InstanceConnectInfo(mock.Anything).Return(fleetingprovider.ConnectInfo{}, nil).Once()
			acq.EXPECT().HealthSuccess().Return().Once()
			// Slot() is called when UseSlotCgroups is true, regardless of slot value
			if tt.useSlotCgroups {
				acq.EXPECT().Slot().Return(tt.slot).Once()
			}

			fleetingDialer := fleetingmocks.NewClient(t)
			fleetingDialer.EXPECT().Close().Return(nil).Once()

			logger, _ := test.NewNullLogger()
			bl := buildlogger.New(nil, logrus.NewEntry(logger), buildlogger.Options{})

			options := common.ExecutorPrepareOptions{
				Config: &common.RunnerConfig{
					RunnerSettings: common.RunnerSettings{
						UseSlotCgroups:     tt.useSlotCgroups,
						SlotCgroupTemplate: tt.slotCgroupTemplate,
						Autoscaler: &common.AutoscalerConfig{
							VMIsolation: common.VMIsolation{
								Enabled: false,
							},
						},
					},
				},
				Build: &common.Build{
					Job: spec.Job{
						Variables: spec.Variables{},
					},
					Runner: &common.RunnerConfig{},
				},
			}

			ref := newAcquisitionRef("test-key", true)
			ref.acq = acq
			ref.dialAcquisitionInstance = func(_ context.Context, _ fleetingprovider.ConnectInfo, _ connector.DialOptions) (connector.Client, error) {
				return fleetingDialer, nil
			}

			c, err := ref.Prepare(ctx, bl, options)

			require.NoError(t, err)
			require.NotNil(t, c)
			defer c.Close()

			// Check if the environment variable was added
			found := false
			for _, v := range options.Build.Variables {
				if v.Key == "GITLAB_RUNNER_SLOT_CGROUP" {
					found = true
					assert.Equal(t, tt.expectedVariableValue, v.Value, "Environment variable value should match expected")
					break
				}
			}

			if tt.expectVariable {
				assert.True(t, found, "GITLAB_RUNNER_SLOT_CGROUP environment variable should be present")
			} else {
				assert.False(t, found, "GITLAB_RUNNER_SLOT_CGROUP environment variable should not be present")
			}
		})
	}
}

// TestAcquisitionRef_PrepareTaskscalerCancelledDial checks that when taskscaler
// cancels the dial context (because the instance was removed) while the job is
// still alive, Prepare returns an instanceUnavailableError so the retry loop
// can try a different instance. It also verifies the connected client is closed.
func TestAcquisitionRef_PrepareTaskscalerCancelledDial(t *testing.T) {
	dialCtx, cancelDial := context.WithCancelCause(t.Context())

	acq := mocks.NewAcquisition(t)
	acq.EXPECT().WithContext(mock.Anything).Return(dialCtx, func() {}).Once()
	acq.EXPECT().InstanceConnectInfo(mock.Anything).Return(fleetingprovider.ConnectInfo{}, nil).Once()

	fleetingDialer := fleetingmocks.NewClient(t)
	fleetingDialer.EXPECT().Close().Return(nil).Once()

	ref := newAcquisitionRef("test-key", false)
	ref.acq = acq
	ref.dialAcquisitionInstance = func(_ context.Context, _ fleetingprovider.ConnectInfo, _ connector.DialOptions) (connector.Client, error) {
		// taskscaler removes the instance mid-dial; dialCtx is cancelled but
		// the dial still succeeds and returns a connected client
		cancelDial(errors.New("instance externally removed"))
		return fleetingDialer, nil
	}

	logger, _ := test.NewNullLogger()
	bl := buildlogger.New(nil, logrus.NewEntry(logger), buildlogger.Options{})

	// job context is still alive: the cancel came from taskscaler, not the job
	c, err := ref.Prepare(t.Context(), bl, executorPrepareOptions("", "", "", ""))

	assert.Nil(t, c)
	assert.True(t, isInstanceUnavailable(err), "expected instanceUnavailableError, got %v", err)
}

// TestAcquisitionRef_PrepareJobCancelledAfterTaskscalerCancel checks that when
// taskscaler cancels dialCtx (instance removed) AND the job is also cancelled,
// Prepare returns a BuildError attributed to the job cancellation, not the
// taskscaler cause. This distinguishes context.Cause(parentCtx) from
// context.Cause(dialCtx).
func TestAcquisitionRef_PrepareJobCancelledAfterTaskscalerCancel(t *testing.T) {
	jobCtx, cancelJob := context.WithCancelCause(t.Context())
	// dialCtx is independent of jobCtx so we can cancel them in a controlled order
	dialCtx, cancelDial := context.WithCancelCause(t.Context())

	acq := mocks.NewAcquisition(t)
	acq.EXPECT().WithContext(mock.Anything).Return(dialCtx, func() {}).Once()
	acq.EXPECT().InstanceConnectInfo(mock.Anything).Return(fleetingprovider.ConnectInfo{}, nil).Once()

	fleetingDialer := fleetingmocks.NewClient(t)
	fleetingDialer.EXPECT().Close().Return(nil).Once()

	ref := newAcquisitionRef("test-key", false)
	ref.acq = acq
	ref.dialAcquisitionInstance = func(_ context.Context, _ fleetingprovider.ConnectInfo, _ connector.DialOptions) (connector.Client, error) {
		// taskscaler cancels dialCtx first; then the job is also cancelled
		cancelDial(errors.New("instance externally removed"))
		cancelJob(context.Canceled)
		return fleetingDialer, nil
	}

	logger, _ := test.NewNullLogger()
	bl := buildlogger.New(nil, logrus.NewEntry(logger), buildlogger.Options{})

	c, err := ref.Prepare(jobCtx, bl, executorPrepareOptions("", "", "", ""))

	assert.Nil(t, c)
	var buildErr *common.BuildError
	require.ErrorAs(t, err, &buildErr)
	// must reflect the job cancellation, not the taskscaler cause
	assert.Equal(t, common.JobCanceled, buildErr.FailureReason)
}

// TestAcquisitionRef_PrepareTaskscalerCancelledConnectInfo checks that when
// taskscaler cancels dialCtx while InstanceConnectInfo is in flight and the
// job context is still alive, Prepare returns an instanceUnavailableError —
// not a BuildError — and does not call HealthFailure (taskscaler already knows).
func TestAcquisitionRef_PrepareTaskscalerCancelledConnectInfo(t *testing.T) {
	tsCause := errors.New("instance removed by taskscaler")
	dialCtx, cancelDial := context.WithCancelCause(t.Context())

	acq := mocks.NewAcquisition(t)
	acq.EXPECT().WithContext(mock.Anything).Return(dialCtx, func() {}).Once()
	// InstanceConnectInfo cancels dialCtx mid-flight; HealthFailure must NOT be
	// called — taskscaler already knows the instance is gone.
	// Deliberately no HealthFailure EXPECT: an unexpected call fails the test.
	acq.EXPECT().InstanceConnectInfo(mock.Anything).
		RunAndReturn(func(_ context.Context) (fleetingprovider.ConnectInfo, error) {
			cancelDial(tsCause)
			return fleetingprovider.ConnectInfo{}, context.Canceled
		}).Once()

	ref := newAcquisitionRef("test-key", false)
	ref.acq = acq
	// dialAcquisitionInstance must not be reached — the early return in Prepare
	// happens before the dial.
	ref.dialAcquisitionInstance = func(_ context.Context, _ fleetingprovider.ConnectInfo, _ connector.DialOptions) (connector.Client, error) {
		t.Error("dialAcquisitionInstance must not be called when the InstanceConnectInfo path returns early")
		return nil, errors.New("unexpected dial")
	}

	logger, _ := test.NewNullLogger()
	bl := buildlogger.New(nil, logrus.NewEntry(logger), buildlogger.Options{})

	// job context is still alive
	c, err := ref.Prepare(t.Context(), bl, executorPrepareOptions("", "", "", ""))

	assert.Nil(t, c)
	assert.True(t, isInstanceUnavailable(err), "expected instanceUnavailableError, got %v", err)
	assert.ErrorIs(t, err, tsCause, "error must wrap the taskscaler cause")
	_, isBuildErr := errors.AsType[*common.BuildError](err)
	assert.False(t, isBuildErr, "error must not be a BuildError")
}

// TestAcquisitionRef_PrepareJobCancelledDuringConnectInfo checks that when
// taskscaler cancels dialCtx and the job context is also cancelled,
// Prepare returns a BuildError attributed to the job cancellation, not to
// the taskscaler cause.
func TestAcquisitionRef_PrepareJobCancelledDuringConnectInfo(t *testing.T) {
	jobCause := fmt.Errorf("job cancelled: %w", context.Canceled)
	tsCause := errors.New("instance removed by taskscaler")
	jobCtx, cancelJob := context.WithCancelCause(t.Context())
	dialCtx, cancelDial := context.WithCancelCause(t.Context())

	acq := mocks.NewAcquisition(t)
	acq.EXPECT().WithContext(mock.Anything).Return(dialCtx, func() {}).Once()
	// Deliberately no HealthFailure EXPECT: taskscaler-cancel path skips it.
	acq.EXPECT().InstanceConnectInfo(mock.Anything).
		RunAndReturn(func(_ context.Context) (fleetingprovider.ConnectInfo, error) {
			cancelDial(tsCause)
			cancelJob(jobCause)
			return fleetingprovider.ConnectInfo{}, context.Canceled
		}).Once()

	ref := newAcquisitionRef("test-key", false)
	ref.acq = acq
	ref.dialAcquisitionInstance = func(_ context.Context, _ fleetingprovider.ConnectInfo, _ connector.DialOptions) (connector.Client, error) {
		t.Error("dialAcquisitionInstance must not be called when the InstanceConnectInfo path returns early")
		return nil, errors.New("unexpected dial")
	}

	logger, _ := test.NewNullLogger()
	bl := buildlogger.New(nil, logrus.NewEntry(logger), buildlogger.Options{})

	c, err := ref.Prepare(jobCtx, bl, executorPrepareOptions("", "", "", ""))

	assert.Nil(t, c)
	var buildErr *common.BuildError
	require.ErrorAs(t, err, &buildErr)
	assert.Equal(t, common.JobCanceled, buildErr.FailureReason)
	assert.ErrorIs(t, buildErr.Inner, jobCause,
		"BuildError must reflect the job cause, not the taskscaler cause")
	assert.False(t, isInstanceUnavailable(err), "error must not be instanceUnavailableError")
}

func TestBuildErrorFromContextCause(t *testing.T) {
	tests := map[string]struct {
		cause          error
		expectedReason spec.JobFailureReason
	}{
		"context.Canceled maps to JobCanceled": {
			cause:          context.Canceled,
			expectedReason: common.JobCanceled,
		},
		"context.DeadlineExceeded maps to JobExecutionTimeout": {
			cause:          context.DeadlineExceeded,
			expectedReason: common.JobExecutionTimeout,
		},
		"other error maps to RunnerSystemFailure": {
			cause:          fmt.Errorf("autoscaler internal error"),
			expectedReason: common.RunnerSystemFailure,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			berr := buildErrorFromContextCause(tc.cause)
			require.NotNil(t, berr)
			assert.Equal(t, tc.expectedReason, berr.FailureReason)
			assert.Equal(t, tc.cause, berr.Inner)
		})
	}
}
