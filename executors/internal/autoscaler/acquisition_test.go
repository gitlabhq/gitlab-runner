//go:build !integration

package autoscaler

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/gitlab-org/fleeting/fleeting/connector"
	fleetingmocks "gitlab.com/gitlab-org/fleeting/fleeting/connector/mocks"
	nestingmocks "gitlab.com/gitlab-org/fleeting/nesting/api/mocks"
	"gitlab.com/gitlab-org/fleeting/nesting/hypervisor"
	"gitlab.com/gitlab-org/fleeting/taskscaler"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestAcquisitionRefPrepare(t *testing.T) {

}

func TestAcquisitionRefClose(t *testing.T) {

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
			ref := &acquisitionRef{
				mapJobImageToVMImage: tc.mapJobImageToVMImage,
			}
			if tc.slot != nil {
				ref.acq = &taskscaler.Acquisition{}
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
		},
	}
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

func ncDelete(vmID string, err error) expectation {
	return func(nc *nestingmocks.Client) {
		nc.EXPECT().Delete(mock.Anything, vmID).Return(err)
	}
}

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
