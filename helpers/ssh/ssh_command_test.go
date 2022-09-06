//go:build !integration

package ssh_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
)

func TestStrictHostCheckingWithKnownHostsFile(t *testing.T) {
	user, pass := "testuser", "testpass"

	boolTrueValue := true
	boolFalseValue := false

	s, _ := ssh.NewStubServer(user, pass)
	defer s.Stop()

	tempDir := t.TempDir()

	knownHostsFile := filepath.Join(tempDir, "known-hosts-file")
	require.NoError(t, os.WriteFile(
		knownHostsFile,
		[]byte(fmt.Sprintf("[127.0.0.1]:%s %s\n", s.Port(), ssh.TestSSHKeyPair.PublicKey)),
		0o644,
	))

	missingEntryKnownHostsFile := filepath.Join(tempDir, "missing-entry-known-hosts-file")
	require.NoError(t, os.WriteFile(
		missingEntryKnownHostsFile,
		[]byte(knownHostsWithGitlabOnly),
		0o644,
	))

	testCases := map[string]struct {
		disableHostChecking    *bool
		knownHostsFileLocation string
		expectErr              bool
	}{
		"strict host checking not initialized with missing known hosts file": {
			expectErr: true,
		},
		"strict host checking with valid known hosts file": {
			disableHostChecking:    &boolFalseValue,
			knownHostsFileLocation: knownHostsFile,
			expectErr:              false,
		},
		"strict host checking with missing known hosts file": {
			disableHostChecking:    &boolFalseValue,
			knownHostsFileLocation: missingEntryKnownHostsFile,
			expectErr:              true,
		},
		"no strict host checking with missing known hosts file": {
			disableHostChecking:    &boolTrueValue,
			knownHostsFileLocation: missingEntryKnownHostsFile,
			expectErr:              false,
		},
		"strict host checking without provided known hosts file": {
			disableHostChecking: &boolFalseValue,
			expectErr:           true,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			c := s.Client()
			c.Config.DisableStrictHostKeyChecking = tc.disableHostChecking
			c.Config.KnownHostsFile = tc.knownHostsFileLocation

			err := c.Connect()
			defer c.Cleanup()

			if tc.expectErr {
				assert.Error(t, err, "should not succeed in connecting")
			} else {
				assert.NoError(t, err, "should succeed in connecting")
			}
		})
	}
}

//nolint:lll
var knownHostsWithGitlabOnly = `gitlab.com ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCsj2bNKTBSpIYDEGk9KxsGh3mySTRgMtXL583qmBpzeQ+jqCMRgBqB98u3z++J1sKlXHWfM9dyhSevkMwSbhoR8XIq/U0tCNyokEi/ueaBMCvbcTHhO7FcwzY92WK4Yt0aGROY5qX2UKSeOvuP4D6TPqKF1onrSzH9bx9XUf2lEdWT/ia1NEKjunUqu1xOB/StKDHMoX4/OKyIzuS0q/T1zOATthvasJFoPrAjkohTyaDUz2LN5JoH839hViyEG82yB+MjcFV5MU3N1l1QL3cVUCh93xSaua1N85qivl+siMkPGbO5xR/En4iEY6K2XPASUEMaieWVNTRCtJ4S8H+9
gitlab.com ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFSMqzJeV9rUzU4kWitGjeR4PWSa29SPqJ1fVkhtj3Hw9xjLVXVYrU9QlYWrOLXBpQ6KWjbjTDTdDkoohFzgbEY=
gitlab.com ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIAfuCHKVTjquxvt6CM6tdG4SLp1Btn/nOeHHE5UOzRdf`
