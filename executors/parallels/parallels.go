package parallels

import (
	"errors"
	"fmt"
	"runtime"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
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

func (s *executor) isAppleSilicon() bool {
	result := runtime.GOARCH == "arm64"
	if result {
		s.BuildLogger.Debugln("Apple Silicon detected")
	}

	return result
}

func (s *executor) waitForIPAddress(vmName string, seconds int) (string, error) {
	var lastError error

	if s.ipAddress != "" {
		return s.ipAddress, nil
	}

	s.BuildLogger.Debugln("Requesting IP address...")
	for i := 0; i < seconds; i++ {
		var ipAddr string
		var err error
		if s.isAppleSilicon() {
			ipAddr, err = prl.IPAddress(vmName)
		} else {
			mac, macError := prl.Mac(vmName)
			if macError != nil {
				return "", err
			}
			ipAddr, err = prl.IPAddressFromMac(mac)
		}
		if err == nil {
			s.BuildLogger.Debugln("IP address found", ipAddr, "...")
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
		SshConfig:      *s.Config.SSH,
		Stdout:         s.BuildLogger.Stdout(),
		Stderr:         s.BuildLogger.Stderr(),
		ConnectRetries: 30,
	}
	sshCommand.Host = ipAddr

	s.BuildLogger.Debugln("Connecting to SSH...")
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
	s.BuildLogger.Debugln("Requesting default snapshot for VM...")
	snapshot, err := prl.GetDefaultSnapshot(s.vmName)
	if err != nil {
		return err
	}

	s.BuildLogger.Debugln("Reverting VM to snapshot", snapshot, "...")
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
		s.BuildLogger.Debugln("Creating template from VM", baseImage, "...")
		err := s.createClone(baseImage, templateName)
		if err != nil {
			return err
		}
	}

	s.BuildLogger.Debugln("Creating runner from VM template...")
	err := prl.CreateOsVM(s.vmName, templateName)
	if err != nil {
		return err
	}

	s.BuildLogger.Debugln("Bootstrapping VM...")
	err = prl.Start(s.vmName)
	if err != nil {
		return err
	}

	// TODO: integration tests do fail on this due
	// Unable to open new session in this virtual machine.
	// Make sure the latest version of Parallels Tools is installed in this virtual machine and it has finished booting
	s.BuildLogger.Debugln("Waiting for VM to start...")
	err = prl.TryExec(s.vmName, 120, "exit", "0")
	if err != nil {
		return err
	}

	s.BuildLogger.Debugln("Waiting for VM to become responsive...")
	err = s.verifyMachine(s.vmName)
	if err != nil {
		return err
	}
	return nil
}

func (s *executor) updateGuestTime() error {
	s.BuildLogger.Debugln("Updating VM date...")
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

	//nolint:lll
	s.BuildLogger.Warningln("Neither sntp nor ntpdate are available in a guest VM. Proceeding without time synchronization ...")

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
	s.tryDeleteVM()

	s.tryRestoreFromSnapshot()

	if !prl.Exist(s.vmName) {
		s.BuildLogger.Println("Creating new VM...")
		err = s.createVM(baseName)
		if err != nil {
			return err
		}

		canCreateSnapshot := !s.Config.Parallels.DisableSnapshots && !s.isAppleSilicon()
		if canCreateSnapshot {
			s.BuildLogger.Println("Creating default snapshot...")
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

func (s *executor) tryDeleteVM() {
	shouldDelete := s.Config.Parallels.DisableSnapshots || s.isAppleSilicon()
	if shouldDelete && prl.Exist(s.vmName) {
		s.BuildLogger.Debugln("Deleting old VM...")
		killAndUnregisterVM(s.vmName)
	}
}

func (s *executor) printVersion() error {
	version, err := prl.Version()
	if err != nil {
		return err
	}

	s.BuildLogger.Println("Using Parallels", version, "executor...")
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
	// Apple Silicon does not support snapshots
	if s.isAppleSilicon() {
		return
	}

	if !prl.Exist(s.vmName) {
		return
	}

	s.BuildLogger.Println("Restoring VM from snapshot...")
	err := s.restoreFromSnapshot()
	if err != nil {
		s.BuildLogger.Println("Previous VM failed. Deleting, because", err)
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
	s.BuildLogger.Debugln("Checking VM status...")
	status, err := prl.Status(s.vmName)
	if err != nil {
		return err
	}

	// Start VM if stopped
	if status == prl.Stopped || status == prl.Suspended {
		s.BuildLogger.Println("Starting VM...")
		err = prl.Start(s.vmName)
		if err != nil {
			return err
		}
	}

	if status != prl.Running {
		s.BuildLogger.Debugln("Waiting for VM to run...")
		err = prl.WaitForStatus(s.vmName, prl.Running, 60)
		if err != nil {
			return err
		}
	}

	s.BuildLogger.Println("Waiting for VM to become responsive...")
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
		s.BuildLogger.Println("Could not sync with timeserver!")
		return err
	}

	return nil
}

func (s *executor) sshConnect() error {
	ipAddr, err := s.waitForIPAddress(s.vmName, 120)
	if err != nil {
		return err
	}

	s.BuildLogger.Debugln("Starting SSH command...")

	logger := s.BuildLogger.StreamID(buildlogger.StreamWorkLevel)
	s.sshCommand = ssh.Client{
		SshConfig: *s.Config.SSH,
		Stdout:    logger.Stdout(),
		Stderr:    logger.Stderr(),
	}
	s.sshCommand.Host = ipAddr

	s.BuildLogger.Debugln("Connecting to SSH server...")
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

func (s *executor) createClone(baseImage string, templateName string) error {
	if s.isAppleSilicon() {
		err := prl.CreateCloneTemplate(baseImage, templateName)
		if err != nil {
			return fmt.Errorf("%w (image: %q)", err, baseImage)
		}
	} else {
		err := prl.CreateLinkedCloneTemplate(baseImage, templateName)
		if err != nil {
			return fmt.Errorf("%w (image: %q)", err, baseImage)
		}
	}

	return nil
}

func init() {
	options := executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: false,
		DefaultSafeDirectoryCheckout:  true,
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
