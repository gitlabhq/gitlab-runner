package steps

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
)

// RecoveryEnvVar carries argv (base64-encoded JSON) for the helper to
// reconstruct when an image entrypoint drops CMD. Set by the executor on
// the build container for both native steps and proxy_exec; read and unset
// by RecoverArgv. The name predates proxy_exec support and is kept so a
// runner and a pinned helper image of neighbouring versions still agree.
const RecoveryEnvVar = "_GITLAB_RUNNER_HELPER_NATIVE_STEPS_ARGV"

// EncodeRecoveryArgv returns the RecoveryEnvVar payload for argv.
func EncodeRecoveryArgv(argv []string) (string, error) {
	raw, err := json.Marshal(argv)
	if err != nil {
		return "", fmt.Errorf("marshalling argv: %w", err)
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

// RecoverArgv reconstructs os.Args from RecoveryEnvVar when the helper was
// invoked with no subcommand, the fingerprint of an `sh -c` entrypoint
// dropping CMD[1:] as positional params that exec doesn't forward. Call
// from main before any arg processing.
//
// The variable is removed from the environment whenever it is present, not
// only when recovery happens: the helper goes on to exec the job shell (or
// step-runner), and neither should inherit it.
func RecoverArgv() {
	encoded, ok := os.LookupEnv(RecoveryEnvVar)
	if !ok {
		return
	}
	_ = os.Unsetenv(RecoveryEnvVar)

	if len(os.Args) > 1 || encoded == "" {
		return
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return
	}
	var argv []string
	if err := json.Unmarshal(raw, &argv); err != nil {
		return
	}
	if len(argv) == 0 {
		return
	}
	os.Args = append(os.Args, argv...)
}
