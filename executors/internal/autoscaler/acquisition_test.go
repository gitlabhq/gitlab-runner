package autoscaler

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/gitlab-org/fleeting/fleeting/connector"
	fleetingmocks "gitlab.com/gitlab-org/fleeting/fleeting/connector/mocks"
	nestingmocks "gitlab.com/gitlab-org/fleeting/nesting/api/mocks"
	"gitlab.com/gitlab-org/fleeting/nesting/hypervisor"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestAcquisitionRefPrepare(t *testing.T) {

}

func TestAcquisitionRefClose(t *testing.T) {

}

func TestAcquisitionRefCreateVMTunner(t *testing.T) {
	cases := []struct {
		name                     string
		ref                      *acquisitionRef
		buildImageName           string
		nestingCfgImage          string
		expect                   []expectation
		expectTunneledDialerCall bool
		tunneledDialerErr        error
		err                      bool
	}{
		{
			name: "success",
			ref: &acquisitionRef{
				key: "key",
			},
			nestingCfgImage: "image",
			expect: []expectation{
				ncCreate("image", nil, hypervisor.VirtualMachineInfo{Name: "name", Id: "id"}, nil, nil),
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
			createTunneledDialer = mockCTD.Fn()
			nc := nestingmocks.NewClient(t)
			for _, e := range tc.expect {
				e(nc)
			}
			options := executorPrepareOptions(tc.buildImageName, tc.nestingCfgImage)

			client, err := tc.ref.createVMTunnel(context.TODO(), common.BuildLogger{}, nc, dialer, options)

			if tc.err {
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

func (m *mockCreateTunneledDialer) Fn() func(
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
		return m.dialer, m.err
	}
}

type expectation func(*nestingmocks.Client)

func ncCreate(image string, slot *int32, vm hypervisor.VirtualMachine, stompedVmId *string, err error) expectation {
	return func(nc *nestingmocks.Client) {
		nc.EXPECT().Create(context.TODO(), image, slot).Return(vm, stompedVmId, err)
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
