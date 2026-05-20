//go:build !integration

package machine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

func TestTargetLabelsFromInspect(t *testing.T) {
	tests := map[string]struct {
		in   docker.MachineInfo
		want targetLabels
	}{
		"google driver, all fields populated": {
			in: docker.MachineInfo{
				DriverName:  "google",
				Zone:        "us-east1-d",
				Project:     "gitlab-r-saas-l-s-amd64-2",
				MachineType: "n2d-standard-2",
			},
			want: targetLabels{
				zone:        "us-east1-d",
				region:      "us-east1",
				project:     "gitlab-r-saas-l-s-amd64-2",
				machineType: "n2d-standard-2",
			},
		},
		"google driver, regional MIG pre-create state (no zone yet)": {
			// Regional MIG mode: zone is unknown until the MIG places
			// the instance. The state file written by libmachine before
			// driver.Create captures empty Zone. The metric just has an
			// empty target_zone for that emission — better than guessing.
			in: docker.MachineInfo{
				DriverName: "google",
				Project:    "p",
			},
			want: targetLabels{
				project: "p",
			},
		},
		"non-google driver returns empty labels": {
			in: docker.MachineInfo{
				DriverName:  "amazonec2",
				Zone:        "us-east-1a",
				MachineType: "m5.large",
			},
			want: targetLabels{},
		},
		"empty info yields empty labels": {
			in:   docker.MachineInfo{},
			want: targetLabels{},
		},
		"malformed zone (no dash) leaves region empty": {
			in: docker.MachineInfo{
				DriverName:  "google",
				Zone:        "unexpected",
				MachineType: "n2-standard-2",
			},
			want: targetLabels{
				zone:        "unexpected",
				machineType: "n2-standard-2",
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, targetLabelsFromInspect(tc.in))
		})
	}
}

func TestActionLabels(t *testing.T) {
	got := actionLabels("creation-failed", targetLabels{
		zone:        "us-east1-d",
		region:      "us-east1",
		project:     "p",
		machineType: "n2d-standard-2",
	})
	assert.Equal(t, []string{
		"creation-failed",
		"us-east1-d",
		"us-east1",
		"p",
		"n2d-standard-2",
	}, got)
	// Label order must match the CounterVec schema.
	assert.Equal(t, 1+len(targetLabelNames), len(got))
}
