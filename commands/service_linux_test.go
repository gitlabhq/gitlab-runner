//go:build !integration

package commands

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/kardianos/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"

	service_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/service"
)

// forcedSystem wraps a service.System so it is always detected, making
// service.Platform() and service.New deterministic on hosts that do not
// run systemd (e.g. CI containers).
type forcedSystem struct {
	service.System
}

func (forcedSystem) Detect() bool { return true }

func forceSystemdPlatform(t *testing.T) {
	t.Helper()

	orig := service.AvailableSystems()
	for _, sys := range orig {
		if sys.String() == "linux-systemd" {
			service.ChooseSystem(forcedSystem{sys})
			t.Cleanup(func() { service.ChooseSystem(orig...) })
			return
		}
	}
	t.Fatal("linux-systemd is not among the available service systems")
}

// stubSystemctl puts a no-op systemctl first in $PATH so Install() never
// talks to a real systemd instance.
func stubSystemctl(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	script := "#!/bin/sh\nexit 0\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "systemctl"), []byte(script), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func newServiceCliContext(t *testing.T) *cli.Context {
	t.Helper()

	fs := flag.NewFlagSet("", flag.ContinueOnError)
	fs.String("service", defaultServiceName, "")
	fs.Bool("user-service", false, "")
	require.NoError(t, fs.Parse([]string{"--user-service"}))

	return cli.NewContext(cli.NewApp(), fs, nil)
}

func TestSetupOSServiceConfig(t *testing.T) {
	forceSystemdPlatform(t)

	config := &service.Config{}
	setupOSServiceConfig(newServiceCliContext(t), config)

	assert.Contains(t, config.Dependencies, "After=network.target")
	assert.Equal(t, service.KeyValue{
		"ReloadSignal": "HUP",
		"Restart":      "always",
		"UserService":  true,
	}, config.Option)
}

func TestInstallGeneratedSystemdUnitContainsReload(t *testing.T) {
	forceSystemdPlatform(t)
	stubSystemctl(t)
	// Install as a user service so the unit file lands in
	// $HOME/.config/systemd/user instead of /etc/systemd/system.
	t.Setenv("HOME", t.TempDir())

	s, err := service_helpers.New(&NullService{}, createServiceConfig(newServiceCliContext(t)))
	require.NoError(t, err)

	require.NoError(t, s.Install())

	unitPath := filepath.Join(os.Getenv("HOME"), ".config/systemd/user", defaultServiceName+".service")
	content, err := os.ReadFile(unitPath)
	require.NoError(t, err, "generated unit file should exist")

	assert.Contains(t, string(content), `ExecReload=/bin/kill -HUP "$MAINPID"`)
	assert.Contains(t, string(content), "Restart=always")
}
