//go:build !integration

package docker

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvKeyFields_roundtrip(t *testing.T) {
	tests := []struct {
		name string
		in   envKeyFields
	}{
		{
			name: "build and helper only",
			in:   envKeyFields{buildContainerID: "build-cid", helperContainerID: "helper-cid"},
		},
		{
			name: "single service",
			in: envKeyFields{
				buildContainerID:    "b",
				helperContainerID:   "h",
				serviceContainerIDs: []string{"svc-only"},
			},
		},
		{
			name: "multiple services",
			in: envKeyFields{
				buildContainerID:    "build-cid",
				helperContainerID:   "helper-cid",
				serviceContainerIDs: []string{"svc-a", "svc-b"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := parseEnvKeyFields(tt.in.toValues())
			require.NoError(t, err)
			assert.Equal(t, tt.in, out)
		})
	}
}

func TestEnvKeyFields_parseErrors(t *testing.T) {
	tests := []struct {
		name    string
		input   url.Values
		wantErr string
	}{
		{
			name:    "empty values",
			input:   url.Values{},
			wantErr: "build-container-id is required",
		},
		{
			name:    "missing build container ID",
			input:   url.Values{envKeyHelperIDField: []string{"x"}},
			wantErr: "build-container-id is required",
		},
		{
			name:    "missing helper ID",
			input:   url.Values{envKeyBuildContainerIDField: []string{"x"}},
			wantErr: "helper-id is required",
		},
		{
			name: "empty service ID",
			input: url.Values{
				envKeyBuildContainerIDField: []string{"b"},
				envKeyHelperIDField:         []string{"h"},
				envKeyServiceIDsField:       []string{"svc-a,,svc-b"},
			},
			wantErr: "service-ids contains an empty ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseEnvKeyFields(tt.input)
			require.EqualError(t, err, tt.wantErr)
		})
	}
}
