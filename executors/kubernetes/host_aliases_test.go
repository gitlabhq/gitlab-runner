//go:build !integration

package kubernetes

import (
	"testing"

	"github.com/stretchr/testify/assert"
	api "k8s.io/api/core/v1"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestCreateHostAliases(t *testing.T) {
	tests := map[string]struct {
		services            common.Services
		hostAliases         []api.HostAlias
		expectedHostAliases []api.HostAlias
		expectedError       error
	}{
		"supports services as host aliases": {
			services: common.Services{
				{
					Name:  "test-service",
					Alias: "svc-alias svc-alias-alt",
				},
				{
					Name: "docker:dind",
				},
			},
			expectedHostAliases: []api.HostAlias{
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"test-service", "svc-alias", "svc-alias-alt", "docker"},
				},
			},
		},
		"ignores services with ports": {
			services: common.Services{
				{
					Name:  "test-service",
					Alias: "alias",
				},
				{
					Name: "docker:dind",
					Ports: []common.Port{{
						Number:   0,
						Protocol: "",
						Name:     "",
					}},
				},
			},
			hostAliases: []api.HostAlias{
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"redis"},
				},
				{
					IP:        "8.8.8.8",
					Hostnames: []string{"dns1", "dns2"},
				},
			},
			expectedHostAliases: []api.HostAlias{
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"test-service", "alias"},
				},
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"redis"},
				},
				{
					IP:        "8.8.8.8",
					Hostnames: []string{"dns1", "dns2"},
				},
			},
		},
		"no services or aliases defined": {
			services:            common.Services{},
			hostAliases:         []api.HostAlias{},
			expectedHostAliases: nil,
		},
		"no host aliases when services only with ports": {
			services: common.Services{
				{
					Name: "docker:dind",
					Ports: []common.Port{{
						Number:   0,
						Protocol: "",
						Name:     "",
					}},
				},
			},
			expectedHostAliases: nil,
		},
		"hosts aliases with no services": {
			hostAliases: []api.HostAlias{
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"redis"},
				},
				{
					IP:        "8.8.8.8",
					Hostnames: []string{"dns1", "dns2"},
				},
				{
					IP:        "8.8.8.8",
					Hostnames: []string{"dns3", "dns4"},
				},
			},
			expectedHostAliases: []api.HostAlias{
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"redis"},
				},
				{
					IP:        "8.8.8.8",
					Hostnames: []string{"dns1", "dns2"},
				},
				{
					IP:        "8.8.8.8",
					Hostnames: []string{"dns3", "dns4"},
				},
			},
		},
		"host aliases with services": {
			hostAliases: []api.HostAlias{
				{
					IP:        "8.8.8.8",
					Hostnames: []string{"google"},
				},
			},
			services: common.Services{
				{
					Name:  "test-service",
					Alias: "alias",
				},
			},
			expectedHostAliases: []api.HostAlias{
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"test-service", "alias"},
				},
				{
					IP:        "8.8.8.8",
					Hostnames: []string{"google"},
				},
			},
		},
		"host aliases with extra host alias for 120.0.0.1 and host alias for services": {
			hostAliases: []api.HostAlias{
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"redis"},
				},
				{
					IP:        "8.8.8.8",
					Hostnames: []string{"google"},
				},
			},
			services: common.Services{
				{
					Name:  "test-service",
					Alias: "alias",
				},
			},
			expectedHostAliases: []api.HostAlias{
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"test-service", "alias"},
				},
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"redis"},
				},
				{
					IP:        "8.8.8.8",
					Hostnames: []string{"google"},
				},
			},
		},
		"ignores non RFC1123 service aliases": {
			services: common.Services{
				{
					Name:  "test-service",
					Alias: "INVALID_ALIAS",
				},
				{
					Name: "docker:dind",
				},
			},
			expectedError: &invalidHostAliasDNSError{},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			hostAliases, err := createHostAliases(tt.services, tt.hostAliases)
			assert.ErrorIs(t, err, tt.expectedError)
			assert.Equal(t, tt.expectedHostAliases, hostAliases)
		})
	}
}
