//go:build !integration

package autoscaler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gitlab.com/gitlab-org/fleeting/fleeting/connector"
	fleetingmocks "gitlab.com/gitlab-org/fleeting/fleeting/connector/mocks"
	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	nestingapi "gitlab.com/gitlab-org/fleeting/nesting/api"
	nestingmocks "gitlab.com/gitlab-org/fleeting/nesting/api/mocks"
	"gitlab.com/gitlab-org/fleeting/nesting/hypervisor"
	"gitlab.com/gitlab-org/fleeting/taskscaler"
	"gitlab.com/gitlab-org/fleeting/taskscaler/mocks"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAcquisitionRef_Prepare(t *testing.T) {
	testCases := map[string]struct {
		vmIsolationEnabled     bool
		useExternalAddr        bool
		expectDialerCall       bool
		instanceConnectInfoErr error
		dialerErr              error
		createVMTunnelErr      error
		expectedPrepareErr     error
	}{
		"Error when getting InstanceConnectInfo": {
			instanceConnectInfoErr: assert.AnError,
			expectedPrepareErr:     assert.AnError,
		},
		"Error when dialing VM": {
			expectDialerCall:   true,
			dialerErr:          assert.AnError,
			expectedPrepareErr: assert.AnError,
		},
		"Error when creating VM tunnel": {
			vmIsolationEnabled: true,
			expectDialerCall:   true,
			createVMTunnelErr:  assert.AnError,
			expectedPrepareErr: fmt.Errorf("creating vm tunnel: %w", assert.AnError),
		},
		"No error and VM isolation disabled": {
			expectDialerCall:   true,
			vmIsolationEnabled: false,
		},
		"No error and VM isolation enabled": {
			expectDialerCall:   true,
			vmIsolationEnabled: true,
		},
	}

	for testName, tc := range testCases {
		t.Run(testName, func(t *testing.T) {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Minute))
			defer cancel()
			logger, _ := test.NewNullLogger()
			bl := common.NewBuildLogger(nil, logrus.NewEntry(logger))
			options := executorPrepareOptions("build-image-name", "image")

			acq := mocks.NewAcquisition(t)
			acq.EXPECT().InstanceConnectInfo(mock.Anything).Return(fleetingprovider.ConnectInfo{}, tc.instanceConnectInfoErr)

			dialer := fleetingmocks.NewClient(t)
			// dialer is closed if createVMTunnel errors
			if tc.expectDialerCall {
				dialer.EXPECT().Close().Return(nil)
			}

			mockD := &mockDialer{
				expectCall:            tc.expectDialerCall,
				expectUseExternalAddr: tc.useExternalAddr,
				dialer:                dialer,
				err:                   tc.dialerErr,
			}

			var nestingClient nestingapi.Client
			ref := &acquisitionRef{
				key:                     "test-key",
				acq:                     acq,
				mapJobImageToVMImage:    true,
				dialAcquisitionInstance: mockD.fn(),
				createVMTunnelFunc: func(
					_ context.Context,
					_ common.BuildLogger,
					nc nestingapi.Client,
					_ connector.Client,
					_ common.ExecutorPrepareOptions,
				) (executors.Client, error) {
					nestingClient = nc
					return &client{client: dialer, cleanup: nil}, tc.createVMTunnelErr
				},
			}

			options.Config.Autoscaler = &common.AutoscalerConfig{
				VMIsolation: common.VMIsolation{
					Enabled: tc.vmIsolationEnabled,
				},
				ConnectorConfig: common.ConnectorConfig{
					UseExternalAddr: tc.useExternalAddr,
				},
			}

			c, err := ref.Prepare(ctx, bl, options)

			if tc.expectedPrepareErr != nil {
				assert.Nil(t, c)
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if c != nil {
				// Make sure the tunneled client calls delete on close (cleanup)
				assert.NoError(t, c.Close())
			}
			if tc.expectDialerCall {
				// If there is an error, we won't have a client, but will still need to close the dialer
				assert.NoError(t, dialer.Close())
			}
			if tc.vmIsolationEnabled && tc.createVMTunnelErr == nil {
				assert.NoError(t, nestingClient.Close())
			}
			mockD.verify(t)
			acq.AssertExpectations(t)
			dialer.AssertExpectations(t)
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
			mc := &fleetingmocks.Client{}
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

func TestAcquisitionRefCreateVMTunnel(t *testing.T) {
	cases := []struct {
		name                     string
		nestingCfgImage          string
		buildImageName           string
		mapJobImageToVMImage     bool
		slot                     *int32
		expect                   []expectation
		expectTunneledDialerCall bool
		tunneledDialerErr        error
		expectErr                bool
	}{
		{
			name:            "success",
			nestingCfgImage: "image",
			expect: []expectation{
				ncCreate("image", nil, hypervisor.VirtualMachineInfo{Id: "id"}, nil, nil),
				ncDelete("id", nil),
				ncClose(nil),
			},
			expectTunneledDialerCall: true,
		},
		{
			name:                 "build image override",
			nestingCfgImage:      "image",
			buildImageName:       "override-image",
			mapJobImageToVMImage: true,
			expect: []expectation{
				ncCreate("override-image", nil, hypervisor.VirtualMachineInfo{Id: "id"}, nil, nil),
				ncDelete("id", nil),
				ncClose(nil),
			},
			expectTunneledDialerCall: true,
		},
		{
			name:            "error create nested vm",
			nestingCfgImage: "image",
			expect: []expectation{
				ncCreate("image", nil, nil, nil, fmt.Errorf("no can do")),
			},
			expectTunneledDialerCall: false,
			expectErr:                true,
		},
		{
			name:            "error creating tunneled dialer",
			nestingCfgImage: "image",
			expect: []expectation{
				ncCreate("image", nil, hypervisor.VirtualMachineInfo{Id: "id"}, nil, nil),
				ncDelete("id", nil),
			},
			expectTunneledDialerCall: true,
			tunneledDialerErr:        fmt.Errorf("no can do"),
			expectErr:                true,
		},
		{
			name:            "success with slot",
			nestingCfgImage: "image",
			slot:            int32Ref(0),
			expect: []expectation{
				ncCreate("image", int32Ref(0), hypervisor.VirtualMachineInfo{Id: "id"}, nil, nil),
				ncDelete("id", nil),
				ncClose(nil),
			},
			expectTunneledDialerCall: true,
		},
		{
			name:            "success with stomped slot",
			nestingCfgImage: "image",
			slot:            int32Ref(0),
			expect: []expectation{
				ncCreate("image", int32Ref(0), hypervisor.VirtualMachineInfo{Id: "id"}, stringRef("stomped-id"), nil),
				ncDelete("id", nil),
				ncClose(nil),
			},
			expectTunneledDialerCall: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dialer := fleetingmocks.NewClient(t)
			implCTD := createTunneledDialer
			defer func() {
				createTunneledDialer = implCTD
			}()
			mockCTD := &mockCreateTunneledDialer{
				expectCall: tc.expectTunneledDialerCall,
				err:        tc.tunneledDialerErr,
			}
			if tc.tunneledDialerErr == nil {
				mockCTD.dialer = dialer
			}
			createTunneledDialer = mockCTD.fn()
			nc := nestingmocks.NewClient(t)
			for _, e := range tc.expect {
				e(nc)
			}
			options := executorPrepareOptions(tc.buildImageName, tc.nestingCfgImage)
			ref := newAcquisitionRef("key", tc.mapJobImageToVMImage)
			if tc.slot != nil {
				ref.acq = taskscaler.NewAcquisition()
				// Cannot set slot (unexported field) but defaults to 0
			}

			client, err := ref.createVMTunnel(context.TODO(), common.BuildLogger{}, nc, dialer, options)

			if tc.expectErr {
				assert.Nil(t, client)
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			if client != nil {
				// Make sure the tunneled client calls delete on close (cleanup)
				dialer.EXPECT().Close().Return(nil)
				client.Close()
			}
			mockCTD.verify(t)
		})
	}
}

func executorPrepareOptions(buildImageName, nestingCfgImage string) common.ExecutorPrepareOptions {
	return common.ExecutorPrepareOptions{
		Config: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Autoscaler: &common.AutoscalerConfig{
					VMIsolation: common.VMIsolation{
						Image: nestingCfgImage,
					},
				},
			},
		},
		Build: &common.Build{
			JobResponse: common.JobResponse{
				Image: common.Image{
					Name: buildImageName,
				},
			},
			Runner: &common.RunnerConfig{},
		},
	}
}

type mockDialer struct {
	expectCall            bool
	wasCalled             bool
	expectUseExternalAddr bool
	useExternalAddrTrue   bool
	dialer                connector.Client
	err                   error
}

func (m *mockDialer) fn() connector.DialFn {
	return func(
		ctx context.Context,
		info fleetingprovider.ConnectInfo,
		options connector.DialOptions,
	) (connector.Client, error) {
		m.wasCalled = true
		m.useExternalAddrTrue = options.UseExternalAddr
		return m.dialer, m.err
	}
}

func (m *mockDialer) verify(t *testing.T) {
	assert.Equal(t, m.expectCall, m.wasCalled)
	assert.Equal(t, m.expectUseExternalAddr, m.useExternalAddrTrue)

}

type mockCreateTunneledDialer struct {
	expectCall bool
	wasCalled  bool
	dialer     connector.Client
	err        error
}

func (m *mockCreateTunneledDialer) fn() func(
	context.Context,
	connector.Client,
	common.VMIsolation,
	hypervisor.VirtualMachine,
) (connector.Client, error) {
	return func(
		_ context.Context,
		_ connector.Client,
		_ common.VMIsolation,
		_ hypervisor.VirtualMachine,
	) (connector.Client, error) {
		m.wasCalled = true
		return m.dialer, m.err
	}
}

func (m *mockCreateTunneledDialer) verify(t *testing.T) {
	if m.expectCall && !m.wasCalled {
		t.Errorf("wanted call. got none")
	}
	if !m.expectCall && m.wasCalled {
		t.Errorf("wanted no call. got one")
	}
}

type expectation func(*nestingmocks.Client)

func ncCreate(image string, slot *int32, vm hypervisor.VirtualMachine, stompedVMID *string, err error) expectation {
	return func(nc *nestingmocks.Client) {
		nc.EXPECT().Create(context.TODO(), image, slot).Return(vm, stompedVMID, err)
	}
}

//nolint:unparam
func ncDelete(vmID string, err error) expectation {
	return func(nc *nestingmocks.Client) {
		nc.EXPECT().Delete(mock.Anything, vmID).Return(err)
	}
}

//nolint:unparam
func ncClose(err error) expectation {
	return func(nc *nestingmocks.Client) {
		nc.EXPECT().Close().Return(err)
	}
}

func int32Ref(i int32) *int32 {
	return &i
}

func stringRef(s string) *string {
	return &s
}
