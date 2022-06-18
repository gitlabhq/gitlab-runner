package parallels

import (
	"errors"
	"fmt"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/vm"
	prl "gitlab.com/gitlab-org/gitlab-runner/helpers/parallels"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
)

type executor struct {
	vm.Executor
	vmName          string
	sshCommand      ssh.Client
	provisioned     bool
	ipAddress       string
	machineVerified bool
}

func (s *executor) Name() string {
	return "parallels"
}

func (s *executor) waitForIPAddress(vmName string, seconds int) (string, error) {
	var lastError error

	if s.ipAddress != "" {
		return s.ipAddress, nil
	}

	s.Debugln("Looking for MAC address...")
	macAddr, err := prl.Mac(vmName)
	if err != nil {
		return "", err
	}

	s.Debugln("Requesting IP address...")
	for i := 0; i < seconds; i++ {
		ipAddr, err := prl.IPAddress(macAddr)
		if err == nil {
			s.Debugln("IP address found", ipAddr, "...")
			s.ipAddress = ipAddr
			return ipAddr, nil
		}
		lastError = err
		time.Sleep(time.Second)
	}
	return "", lastError
}

func (s *executor) verifyMachine(vmName string) error {
	if s.machineVerified {
		return nil
	}

	ipAddr, err := s.waitForIPAddress(vmName, 120)
	if err != nil {
		return err
	}

	// Create SSH command
	sshCommand := ssh.Client{
		Config:         *s.Config.SSH,
		Stdout:         s.Trace,
		Stderr:         s.Trace,
		ConnectRetries: 30,
	}
	sshCommand.Host = ipAddr

	s.Debugln("Connecting to SSH...")
	err = sshCommand.Connect()
	if err != nil {
		return err
	}
	defer sshCommand.Cleanup()
	err = sshCommand.Run(s.Context, ssh.Command{Command: "exit"})
	if err != nil {
		return err
	}
	s.machineVerified = true
	return nil
}

func (s *executor) restoreFromSnapshot() error {
	s.Debugln("Requesting default snapshot for VM...")
	snapshot, err := prl.GetDefaultSnapshot(s.vmName)
	if err != nil {
		return err
	}

	s.Debugln("Reverting VM to snapshot", snapshot, "...")
	err = prl.RevertToSnapshot(s.vmName, snapshot)
	if err != nil {
		return err
	}

	return nil
}

func (s *executor) createVM(baseImage string) error {
	templateName := s.Config.Parallels.TemplateName
	if templateName == "" {
		templateName = baseImage + "-template"
	}

	// remove invalid template (removed?)
	templateStatus, _ := prl.Status(templateName)
	if templateStatus == prl.Invalid {
		_ = prl.Unregister(templateName)
	}

	if !prl.Exist(templateName) {
		s.Debugln("Creating template from VM", baseImage, "...")
		err := prl.CreateTemplate(baseImage, templateName)
		if err != nil {
			return fmt.Errorf("%w (image: %q)", err, baseImage)
		}
	}

	s.Debugln("Creating runner from VM template...")
	err := prl.CreateOsVM(s.vmName, templateName)
	if err != nil {
		return err
	}

	s.Debugln("Bootstrapping VM...")
	err = prl.Start(s.vmName)
	if err != nil {
		return err
	}

	// TODO: integration tests do fail on this due
	// Unable to open new session in this virtual machine.
	// Make sure the latest version of Parallels Tools is installed in this virtual machine and it has finished booting
	s.Debugln("Waiting for VM to start...")
	err = prl.TryExec(s.vmName, 120, "exit", "0")
	if err != nil {
		return err
	}

	s.Debugln("Waiting for VM to become responsive...")
	err = s.verifyMachine(s.vmName)
	if err != nil {
		return err
	}
	return nil
}

func (s *executor) updateGuestTime() error {
	s.Debugln("Updating VM date...")
	timeServer := s.Config.Parallels.TimeServer
	if timeServer == "" {
		timeServer = "time.apple.com"
	}

	// Try ntpdate first, this command is available in macOS versions prior to Mojave.
	// This is not guaranteed, but there is high probability that ntpdate may be available on other UNIX-like systems.
	_, err := prl.Exec(s.vmName, "which", "ntpdate")
	if err == nil {
		return prl.TryExec(s.vmName, 20, "sudo", "ntpdate", "-u", timeServer)
	}

	// Starting from Mojave, ntpdate is no longer available on macOS, sntp is supposed to be used instead.
	_, err = prl.Exec(s.vmName, "which", "sntp")
	if err == nil {
		return prl.TryExec(s.vmName, 20, "sudo", "sntp", "-sS", timeServer)
	}

	// Neither sntp nor ntpdate is available, very likely guest OS is not macOS.
	// Report a warning to a user and gracefully return.

	s.Warningln("Neither sntp nor ntpdate are available in a guest VM. Proceeding without time synchronization ...")

	return nil
}

func (s *executor) Prepare(options common.ExecutorPrepareOptions) error {
	err := s.AbstractExecutor.Prepare(options)
	if err != nil {
		return err
	}

	err = s.validateConfig()
	if err != nil {
		return err
	}

	err = s.printVersion()
	if err != nil {
		return err
	}

	var baseName string
	baseName, err = s.Executor.GetBaseName(s.Config.Parallels.BaseName)
	if err != nil {
		return err
	}

	unregisterInvalidVM(s.vmName)

	s.vmName = s.getVMName(baseName)

	if s.Config.Parallels.DisableSnapshots && prl.Exist(s.vmName) {
		s.Debugln("Deleting old VM...")
		killAndUnregisterVM(s.vmName)
	}

	s.tryRestoreFromSnapshot()

	if !prl.Exist(s.vmName) {
		s.Println("Creating new VM...")
		err = s.createVM(baseName)
		if err != nil {
			return err
		}

		if !s.Config.Parallels.DisableSnapshots {
			s.Println("Creating default snapshot...")
			err = prl.CreateSnapshot(s.vmName, "Started")
			if err != nil {
				return err
			}
		}
	}

	err = s.ensureVMStarted()
	if err != nil {
		return err
	}

	return s.sshConnect()
}

func (s *executor) printVersion() error {
	version, err := prl.Version()
	if err != nil {
		return err
	}

	s.Println("Using Parallels", version, "executor...")
	return nil
}

func (s *executor) validateConfig() error {
	if s.Config.Parallels.BaseName == "" {
		return errors.New("missing BaseName setting from Parallels configuration")
	}

	if s.BuildShell.PassFile {
		return errors.New("parallels doesn't support shells that require script file")
	}

	if s.Config.SSH == nil {
		return errors.New("missing SSH configuration")
	}

	if s.Config.Parallels == nil {
		return errors.New("missing Parallels configuration")
	}

	return s.ValidateAllowedImages(s.Config.Parallels.AllowedImages)
}

func (s *executor) tryRestoreFromSnapshot() {
	if !prl.Exist(s.vmName) {
		return
	}

	s.Println("Restoring VM from snapshot...")
	err := s.restoreFromSnapshot()
	if err != nil {
		s.Println("Previous VM failed. Deleting, because", err)
		killAndUnregisterVM(s.vmName)
	}
}

func (s *executor) getVMName(baseName string) string {
	if s.Config.Parallels.DisableSnapshots {
		return baseName + "-" + s.Build.ProjectUniqueName()
	}

	return fmt.Sprintf(
		"%s-runner-%s-concurrent-%d",
		baseName,
		s.Build.Runner.ShortDescription(),
		s.Build.RunnerID,
	)
}

func unregisterInvalidVM(vmName string) {
	// remove invalid VM (removed?)
	vmStatus, _ := prl.Status(vmName)
	if vmStatus == prl.Invalid {
		_ = prl.Unregister(vmName)
	}
}

func killAndUnregisterVM(vmName string) {
	_ = prl.Kill(vmName)
	_ = prl.Delete(vmName)
	_ = prl.Unregister(vmName)
}

func (s *executor) ensureVMStarted() error {
	s.Debugln("Checking VM status...")
	status, err := prl.Status(s.vmName)
	if err != nil {
		return err
	}

	// Start VM if stopped
	if status == prl.Stopped || status == prl.Suspended {
		s.Println("Starting VM...")
		err = prl.Start(s.vmName)
		if err != nil {
			return err
		}
	}

	if status != prl.Running {
		s.Debugln("Waiting for VM to run...")
		err = prl.WaitForStatus(s.vmName, prl.Running, 60)
		if err != nil {
			return err
		}
	}

	s.Println("Waiting for VM to become responsive...")
	err = s.verifyMachine(s.vmName)
	if err != nil {
		return err
	}

	s.provisioned = true

	// TODO: integration tests do fail on this due
	// Unable to open new session in this virtual machine.
	// Make sure the latest version of Parallels Tools is installed in this virtual machine and it has finished booting
	err = s.updateGuestTime()
	if err != nil {
		s.Println("Could not sync with timeserver!")
		return err
	}

	return nil
}

func (s *executor) sshConnect() error {
	ipAddr, err := s.waitForIPAddress(s.vmName, 60)
	if err != nil {
		return err
	}

	s.Debugln("Starting SSH command...")
	s.sshCommand = ssh.Client{
		Config: *s.Config.SSH,
		Stdout: s.Trace,
		Stderr: s.Trace,
	}
	s.sshCommand.Host = ipAddr

	s.Debugln("Connecting to SSH server...")
	return s.sshCommand.Connect()
}

func (s *executor) Run(cmd common.ExecutorCommand) error {
	err := s.sshCommand.Run(cmd.Context, ssh.Command{
		Command: s.BuildShell.CmdLine,
		Stdin:   cmd.Script,
	})
	if exitError, ok := err.(*ssh.ExitError); ok {
		exitCode := exitError.ExitCode()
		err = &common.BuildError{Inner: err, ExitCode: exitCode}
	}
	return err
}

func (s *executor) Cleanup() {
	s.sshCommand.Cleanup()

	if s.vmName != "" {
		_ = prl.Kill(s.vmName)

		if s.Config.Parallels.DisableSnapshots || !s.provisioned {
			_ = prl.Delete(s.vmName)
		}
	}

	s.AbstractExecutor.Cleanup()
}

func init() {
	options := executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: false,
		DefaultBuildsDir:              "builds",
		DefaultCacheDir:               "cache",
		SharedBuildsDir:               false,
		Shell: common.ShellScriptInfo{
			Shell:         "bash",
			Type:          common.LoginShell,
			RunnerCommand: "gitlab-runner",
		},
		ShowHostname: true,
	}

	creator := func() common.Executor {
		return &executor{
			Executor: vm.Executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: options,
				},
			},
		}
	}

	featuresUpdater := func(features *common.FeaturesInfo) {
		features.Variables = true
	}

	common.RegisterExecutorProvider("parallels", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
