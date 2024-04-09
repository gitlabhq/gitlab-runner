package virtualbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/vm"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
	vbox "gitlab.com/gitlab-org/gitlab-runner/helpers/virtualbox"
)

const virtualboxCleanupTimeout = 5 * time.Minute

type executor struct {
	vm.Executor
	vmName          string
	sshCommand      ssh.Client
	sshPort         string
	provisioned     bool
	machineVerified bool
}

func (s *executor) verifyMachine(sshPort string) error {
	if s.machineVerified {
		return nil
	}

	// Create SSH command
	sshCommand := ssh.Client{
		SshConfig:      *s.Config.SSH,
		ConnectRetries: 30,
	}
	sshCommand.Port = sshPort
	sshCommand.Host = "localhost"

	s.BuildLogger.Debugln("Connecting to SSH...")
	err := sshCommand.Connect()
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
	s.BuildLogger.Debugln("Reverting VM to current snapshot...")
	err := vbox.RevertToSnapshot(s.Context, s.vmName)
	if err != nil {
		return err
	}

	return nil
}

func (s *executor) determineBaseSnapshot(baseImage string) string {
	var err error
	baseSnapshot := s.Config.VirtualBox.BaseSnapshot
	if baseSnapshot == "" {
		baseSnapshot, err = vbox.GetCurrentSnapshot(s.Context, baseImage)
		if err != nil {
			if s.Config.VirtualBox.DisableSnapshots {
				s.BuildLogger.Debugln("No snapshots found for base VM", baseImage)
				return ""
			}

			baseSnapshot = "Base State"
		}
	}

	if baseSnapshot != "" && !vbox.HasSnapshot(s.Context, baseImage, baseSnapshot) {
		if s.Config.VirtualBox.DisableSnapshots {
			s.BuildLogger.Warningln("Snapshot", baseSnapshot, "not found in base VM", baseImage)
			return ""
		}

		s.BuildLogger.Debugln("Creating snapshot", baseSnapshot, "from current base VM", baseImage, "state...")
		err = vbox.CreateSnapshot(s.Context, baseImage, baseSnapshot)
		if err != nil {
			s.BuildLogger.Warningln("Failed to create snapshot", baseSnapshot, "from base VM", baseImage)
			return ""
		}
	}

	return baseSnapshot
}

// virtualbox doesn't support templates
func (s *executor) createVM(baseImage string) (err error) {
	_, err = vbox.Status(s.Context, s.vmName)
	if err != nil {
		_ = vbox.Unregister(s.Context, s.vmName)
	}

	if !vbox.Exist(s.Context, s.vmName) {
		baseSnapshot := s.determineBaseSnapshot(baseImage)
		if baseSnapshot == "" {
			s.BuildLogger.Debugln("Creating testing VM from VM", baseImage, "...")
		} else {
			s.BuildLogger.Debugln("Creating testing VM from VM", baseImage, "snapshot", baseSnapshot, "...")
		}

		err = vbox.CreateOsVM(s.Context, baseImage, s.vmName, baseSnapshot, s.Config.VirtualBox.BaseFolder)
		if err != nil {
			return err
		}
	}

	s.BuildLogger.Debugln("Identify SSH Port...")
	s.sshPort, err = vbox.FindSSHPort(s.Context, s.vmName)
	if err != nil {
		s.BuildLogger.Debugln("Creating localhost ssh forwarding...")
		vmSSHPort := s.Config.SSH.Port
		if vmSSHPort == "" {
			vmSSHPort = "22"
		}
		s.sshPort, err = vbox.ConfigureSSH(s.Context, s.vmName, vmSSHPort)
		if err != nil {
			return err
		}
	}
	s.BuildLogger.Debugln("Using local", s.sshPort, "SSH port to connect to VM...")

	s.BuildLogger.Debugln("Bootstraping VM...")
	err = s.startVM()
	if err != nil {
		return err
	}

	s.BuildLogger.Debugln("Waiting for VM to become responsive...")
	time.Sleep(10 * time.Second)
	err = s.verifyMachine(s.sshPort)
	if err != nil {
		return err
	}

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
	baseName, err = s.Executor.GetBaseName(s.Config.VirtualBox.BaseName)
	if err != nil {
		return err
	}

	s.vmName = s.getVMName(baseName)

	if s.Config.VirtualBox.DisableSnapshots && vbox.Exist(s.Context, s.vmName) {
		s.BuildLogger.Debugln("Deleting old VM...")
		killAndUnregisterVM(s.Context, s.vmName)
	}

	s.tryRestoreFromSnapshot()

	if !vbox.Exist(s.Context, s.vmName) {
		s.BuildLogger.Println("Creating new VM...")
		err = s.createVM(baseName)
		if err != nil {
			return err
		}

		if !s.Config.VirtualBox.DisableSnapshots {
			s.BuildLogger.Println("Creating default snapshot...")
			err = vbox.CreateSnapshot(s.Context, s.vmName, "Started")
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
	version, err := vbox.Version(s.Context)
	if err != nil {
		return err
	}

	s.BuildLogger.Println("Using VirtualBox version", version, "executor...")
	return nil
}

func (s *executor) validateConfig() error {
	if s.Config.VirtualBox.BaseName == "" {
		return errors.New("missing BaseName setting from VirtualBox configuration")
	}

	if s.BuildShell.PassFile {
		return errors.New("virtualbox doesn't support shells that require script file")
	}

	if s.Config.SSH == nil {
		return errors.New("missing SSH config")
	}

	if s.Config.VirtualBox == nil {
		return errors.New("missing VirtualBox configuration")
	}

	return s.ValidateAllowedImages(s.Config.VirtualBox.AllowedImages)
}

func (s *executor) getVMName(baseName string) string {
	if s.Config.VirtualBox.DisableSnapshots {
		return s.Config.VirtualBox.BaseName + "-" + s.Build.ProjectUniqueName()
	}

	return fmt.Sprintf(
		"%s-runner-%s-concurrent-%d",
		baseName,
		s.Build.Runner.ShortDescription(),
		s.Build.RunnerID,
	)
}

func (s *executor) startVM() error {
	s.BuildLogger.Debugln("Starting VM...")
	startType := s.Config.VirtualBox.StartType
	if startType == "" {
		startType = "headless"
	}
	err := vbox.Start(s.Context, s.vmName, startType)
	if err != nil {
		return err
	}
	return nil
}

func (s *executor) tryRestoreFromSnapshot() {
	if !vbox.Exist(s.Context, s.vmName) {
		return
	}

	s.BuildLogger.Println("Restoring VM from snapshot...")
	err := s.restoreFromSnapshot()
	if err != nil {
		s.BuildLogger.Println("Previous VM failed. Deleting, because", err)
		killAndUnregisterVM(s.Context, s.vmName)
	}
}

func killAndUnregisterVM(ctx context.Context, vmName string) {
	_ = vbox.Kill(ctx, vmName)
	_ = vbox.Delete(ctx, vmName)
	_ = vbox.Unregister(ctx, vmName)
}

func (s *executor) ensureVMStarted() error {
	s.BuildLogger.Debugln("Checking VM status...")
	status, err := vbox.Status(s.Context, s.vmName)
	if err != nil {
		return err
	}

	if !vbox.IsStatusOnlineOrTransient(status) {
		err = s.startVM()
		if err != nil {
			return err
		}
	}

	if status != vbox.Running {
		s.BuildLogger.Debugln("Waiting for VM to run...")
		err = vbox.WaitForStatus(s.Context, s.vmName, vbox.Running, 60)
		if err != nil {
			return err
		}
	}

	s.BuildLogger.Debugln("Identify SSH Port...")
	sshPort, err := vbox.FindSSHPort(s.Context, s.vmName)
	s.sshPort = sshPort
	if err != nil {
		return err
	}

	s.BuildLogger.Println("Waiting for VM to become responsive...")
	err = s.verifyMachine(s.sshPort)
	if err != nil {
		return err
	}

	s.provisioned = true
	return nil
}

func (s *executor) sshConnect() error {
	s.BuildLogger.Println("Starting SSH command...")

	s.sshCommand = ssh.Client{
		SshConfig: *s.Config.SSH,
	}
	s.sshCommand.Port = s.sshPort
	s.sshCommand.Host = "localhost"

	s.BuildLogger.Debugln("Connecting to SSH server...")
	return s.sshCommand.Connect()
}

func (s *executor) Run(cmd common.ExecutorCommand) error {
	logger := s.BuildLogger.StreamID(buildlogger.StreamWorkLevel)

	err := s.sshCommand.Run(cmd.Context, ssh.Command{
		Command: s.BuildShell.CmdLine,
		Stdin:   cmd.Script,
		Stdout:  logger.Stdout(),
		Stderr:  logger.Stderr(),
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
		ctx, cancel := context.WithTimeout(context.Background(), virtualboxCleanupTimeout)
		defer cancel()

		_ = vbox.Kill(ctx, s.vmName)

		if s.Config.VirtualBox.DisableSnapshots || !s.provisioned {
			_ = vbox.Delete(ctx, s.vmName)
		}
	}
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

	common.RegisterExecutorProvider("virtualbox", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
