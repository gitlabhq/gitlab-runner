//go:build !integration

package docker

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func guardMachineOperationTest(t *testing.T, name string, callback func(t *testing.T)) {
	tempHomeDir := t.TempDir()

	machineDir := path.Join(tempHomeDir, ".docker", "machine")
	err := os.MkdirAll(machineDir, 0755)
	require.NoError(t, err)

	t.Setenv("MACHINE_STORAGE_PATH", machineDir)
	t.Run(name, callback)
}

func TestList(t *testing.T) {
	guardMachineOperationTest(t, "no machines", func(t *testing.T) {
		err := os.MkdirAll(getMachineDir(), 0755)
		require.NoError(t, err)

		mc := NewMachineCommand()
		hostNames, err := mc.List()
		assert.Empty(t, hostNames)
		assert.NoError(t, err)
	})

	guardMachineOperationTest(t, "one machine", func(t *testing.T) {
		err := os.MkdirAll(getMachineDir(), 0755)
		require.NoError(t, err)

		machineDir := path.Join(getMachineDir(), "machine-1")
		err = os.MkdirAll(machineDir, 0755)
		require.NoError(t, err)

		mc := NewMachineCommand()
		hostNames, err := mc.List()
		assert.Contains(t, hostNames, "machine-1")
		assert.Len(t, hostNames, 1)
		assert.NoError(t, err)
	})

	guardMachineOperationTest(t, "machines directory doesn't exist", func(t *testing.T) {
		mc := NewMachineCommand()
		hostNames, err := mc.List()
		assert.Empty(t, hostNames)
		assert.NoError(t, err)
	})

	guardMachineOperationTest(t, "machines directory is invalid", func(t *testing.T) {
		err := os.MkdirAll(getBaseDir(), 0755)
		require.NoError(t, err)

		err = os.WriteFile(getMachineDir(), []byte{}, 0o600)
		require.NoError(t, err)

		mc := NewMachineCommand()
		hostNames, err := mc.List()
		assert.Empty(t, hostNames)
		assert.Error(t, err)
	})
}

func mockDockerMachineExecutable(t *testing.T) func() {
	tempDir := t.TempDir()

	dmExecutable := filepath.Join(tempDir, "docker-machine")
	if runtime.GOOS == "windows" {
		dmExecutable += ".exe"
	}

	err := os.WriteFile(dmExecutable, []byte{}, 0o777)
	require.NoError(t, err)

	currentDockerMachineExecutable := dockerMachineExecutable
	dockerMachineExecutable = dmExecutable

	return func() {
		dockerMachineExecutable = currentDockerMachineExecutable
	}
}

var dockerMachineCommandArgs = []string{"version", "--help"}

func getDockerMachineCommandExpectedArgs(token string) []string {
	if token == "" {
		token = "no-report"
	}

	return []string{dockerMachineExecutable, fmt.Sprintf("--bugsnag-api-token=%s", token), "version", "--help"}
}

var dockerMachineCommandTests = map[string]struct {
	tokenEnvValue string
	expectedArgs  func() []string
}{
	"MACHINE_BUGSNAG_API_TOKEN is defined by the user": {
		tokenEnvValue: "some-other-token",
		expectedArgs:  func() []string { return getDockerMachineCommandExpectedArgs("some-other-token") },
	},
	"MACHINE_BUGSNAG_API_TOKEN is not defined by the user": {
		tokenEnvValue: "",
		expectedArgs:  func() []string { return getDockerMachineCommandExpectedArgs("") },
	},
}

func TestInspect(t *testing.T) {
	// writeConfig drops a config.json file in the per-machine state dir
	// like docker-machine would.
	writeConfig := func(t *testing.T, name, content string) {
		t.Helper()
		dir := filepath.Join(getMachineDir(), name)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "config.json"), []byte(content), 0o644))
	}

	guardMachineOperationTest(t, "google driver populates Zone / MachineType / Project", func(t *testing.T) {
		writeConfig(t, "m1", `{
			"DriverName": "google",
			"Driver": {
				"Zone": "us-east1-c",
				"MachineType": "n2d-standard-4",
				"Project": "gitlab-r-saas-l-m-amd64-1",
				"SomeOtherField": "ignored"
			}
		}`)

		mc := NewMachineCommand()
		info, err := mc.Inspect("m1")
		require.NoError(t, err)
		assert.Equal(t, MachineInfo{
			DriverName:  "google",
			Zone:        "us-east1-c",
			MachineType: "n2d-standard-4",
			Project:     "gitlab-r-saas-l-m-amd64-1",
		}, info)
	})

	guardMachineOperationTest(t, "non-google driver returns only DriverName", func(t *testing.T) {
		// Even though Zone / MachineType are physically present in the
		// file, Inspect must NOT surface them — the consumer's GCE-style
		// heuristics would produce nonsense for AWS-style values.
		writeConfig(t, "m2", `{
			"DriverName": "amazonec2",
			"Driver": {
				"Zone": "us-east-1a",
				"MachineType": "m5.large",
				"Project": "should-not-leak"
			}
		}`)

		mc := NewMachineCommand()
		info, err := mc.Inspect("m2")
		require.NoError(t, err)
		assert.Equal(t, MachineInfo{DriverName: "amazonec2"}, info)
	})

	guardMachineOperationTest(t, "missing config file returns error", func(t *testing.T) {
		mc := NewMachineCommand()
		_, err := mc.Inspect("does-not-exist")
		require.Error(t, err)
	})

	guardMachineOperationTest(t, "malformed JSON returns error", func(t *testing.T) {
		writeConfig(t, "m3", `{not json`)

		mc := NewMachineCommand()
		_, err := mc.Inspect("m3")
		require.Error(t, err)
	})

	guardMachineOperationTest(t, "google MIG mode uses Resolved* exclusively, ignores operator-intent fields", func(t *testing.T) {
		// MIG mode: the operator-intent Zone / MachineType are
		// misleading because the MIG (or Flex policy) decides
		// placement at create time. docker-machine!168 writes the
		// observed values to Resolved* during post-create discovery.
		// Inspect must surface only those — never the operator
		// values, even when Resolved* is empty.
		writeConfig(t, "mig-resolved", `{
			"DriverName": "google",
			"Driver": {
				"Zone": "us-central1-a",
				"MachineType": "n1-standard-1",
				"Project": "iwiedler-x",
				"RegionInstanceGroupManager": "my-rmig",
				"Region": "us-east1",
				"ResolvedZone": "us-east1-c",
				"ResolvedMachineType": "n2-standard-2"
			}
		}`)
		mc := NewMachineCommand()
		info, err := mc.Inspect("mig-resolved")
		require.NoError(t, err)
		assert.Equal(t, MachineInfo{
			DriverName:  "google",
			Zone:        "us-east1-c",
			MachineType: "n2-standard-2",
			Project:     "iwiedler-x",
		}, info)
	})

	guardMachineOperationTest(t, "google MIG mode with empty Resolved* surfaces empty, not the operator default", func(t *testing.T) {
		// Edge case: a MIG-mode create that errored before
		// finishPostCreate's syncDriverStateFromInstance ran, OR a VM
		// created before this fix shipped. The operator-intent values
		// are flag defaults ("us-central1-a", "n1-standard-1") and
		// don't match what the MIG actually provisioned. Inspect must
		// NOT fall back — emit empty, let the metric label be empty,
		// let stockout investigators see "we don't know" rather than a
		// silently-wrong answer.
		writeConfig(t, "mig-no-resolved", `{
			"DriverName": "google",
			"Driver": {
				"Zone": "us-central1-a",
				"MachineType": "n1-standard-1",
				"Project": "iwiedler-x",
				"InstanceGroupManager": "my-mig"
			}
		}`)
		mc := NewMachineCommand()
		info, err := mc.Inspect("mig-no-resolved")
		require.NoError(t, err)
		assert.Equal(t, MachineInfo{
			DriverName: "google",
			Project:    "iwiedler-x",
			// Zone and MachineType both empty.
		}, info)
	})

	guardMachineOperationTest(t, "google non-MIG mode uses Zone / MachineType, Resolved* ignored", func(t *testing.T) {
		// Non-MIG: operator-supplied flags == reality (the driver
		// passed them to Instances.Insert and GCP provisioned what
		// was requested). Inspect surfaces them directly.
		writeConfig(t, "non-mig", `{
			"DriverName": "google",
			"Driver": {
				"Zone": "us-east1-b",
				"MachineType": "n2d-standard-4",
				"Project": "p"
			}
		}`)
		mc := NewMachineCommand()
		info, err := mc.Inspect("non-mig")
		require.NoError(t, err)
		assert.Equal(t, MachineInfo{
			DriverName:  "google",
			Zone:        "us-east1-b",
			MachineType: "n2d-standard-4",
			Project:     "p",
		}, info)
	})

	guardMachineOperationTest(t, "google bulkInsert mode uses Resolved* exclusively", func(t *testing.T) {
		// bulkInsert mode (docker-machine!169): explicit BulkInsert
		// opt-in marks GCP-picked placement. The operator-intent
		// Zone / MachineType flags are misleading; Inspect must
		// surface only the post-create Resolved* values populated
		// by AggregatedList discovery.
		writeConfig(t, "bulk-resolved", `{
			"DriverName": "google",
			"Driver": {
				"Zone": "",
				"MachineType": "n1-standard-1",
				"Project": "iwiedler-x",
				"BulkInsert": true,
				"Region": "us-east1",
				"ResolvedZone": "us-east1-d",
				"ResolvedMachineType": "n2-standard-2"
			}
		}`)
		mc := NewMachineCommand()
		info, err := mc.Inspect("bulk-resolved")
		require.NoError(t, err)
		assert.Equal(t, MachineInfo{
			DriverName:  "google",
			Zone:        "us-east1-d",
			MachineType: "n2-standard-2",
			Project:     "iwiedler-x",
		}, info)
	})

	guardMachineOperationTest(t, "google bulkInsert mode with empty Resolved* surfaces empty, not the operator default", func(t *testing.T) {
		// Edge case parallel to the MIG-mode one: a bulkInsert
		// create that errored before finishPostCreate's
		// syncDriverStateFromInstance ran. Operator-intent values
		// are the flag default ("n1-standard-1"); Zone is empty
		// because bulkInsert mode rejects --google-zone at config
		// time. Inspect must NOT fall back to either — emit empty,
		// let the metric label be empty.
		writeConfig(t, "bulk-no-resolved", `{
			"DriverName": "google",
			"Driver": {
				"Zone": "",
				"MachineType": "n1-standard-1",
				"Project": "iwiedler-x",
				"BulkInsert": true,
				"Region": "us-east1"
			}
		}`)
		mc := NewMachineCommand()
		info, err := mc.Inspect("bulk-no-resolved")
		require.NoError(t, err)
		assert.Equal(t, MachineInfo{
			DriverName: "google",
			Project:    "iwiedler-x",
			// Zone and MachineType both empty.
		}, info)
	})

	guardMachineOperationTest(t, "google Region without BulkInsert opt-in stays direct mode", func(t *testing.T) {
		// Region alone is not a mode signal: the explicit
		// BulkInsert opt-in is. A driver state with Region set
		// but BulkInsert false means the operator configured
		// region for some other reason and the create still went
		// through Instances.Insert with --google-zone. Zone /
		// MachineType remain authoritative.
		writeConfig(t, "region-no-bulk", `{
			"DriverName": "google",
			"Driver": {
				"Zone": "us-east1-c",
				"MachineType": "n2d-standard-4",
				"Project": "p",
				"Region": "us-east1"
			}
		}`)
		mc := NewMachineCommand()
		info, err := mc.Inspect("region-no-bulk")
		require.NoError(t, err)
		assert.Equal(t, MachineInfo{
			DriverName:  "google",
			Zone:        "us-east1-c",
			MachineType: "n2d-standard-4",
			Project:     "p",
		}, info)
	})

	guardMachineOperationTest(t, "missing Driver subobject leaves driver fields empty", func(t *testing.T) {
		// libmachine writes the file early in the create flow; a very
		// stale or pre-PreCreate state might lack the Driver block.
		// Must not error — just no driver-specific data to surface.
		writeConfig(t, "m4", `{"DriverName": "google"}`)

		mc := NewMachineCommand()
		info, err := mc.Inspect("m4")
		require.NoError(t, err)
		assert.Equal(t, MachineInfo{DriverName: "google"}, info)
	})
}

func TestNewDockerMachineCommand(t *testing.T) {
	for tn, tc := range dockerMachineCommandTests {
		t.Run(tn, func(t *testing.T) {
			err := os.Setenv("MACHINE_BUGSNAG_API_TOKEN", tc.tokenEnvValue)
			require.NoError(t, err)

			ctx, ctxCancelFn := context.WithTimeout(t.Context(), 1*time.Hour)
			defer ctxCancelFn()

			cmd := newDockerMachineCommand(ctx, dockerMachineCommandArgs...)

			assert.Equal(t, tc.expectedArgs(), cmd.Args)
			assert.NotEmpty(t, cmd.Env)
		})
	}
}
