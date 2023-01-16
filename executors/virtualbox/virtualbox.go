package virtualbox

import (
	"errors"
	"fmt"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/vm"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
	vbox "gitlab.com/gitlab-org/gitlab-runner/helpers/virtualbox"
)

type executor struct {
	vm.Executor
	vmName          string
	sshCommand      ssh.Client
	sshPort         string
	provisioned     bool
	machineVerified bool
}

func (s *executor) Name() string {
	return "virtualbox"
}

func (s *executor) verifyMachine(sshPort string) error {
	if s.machineVerified {
		return nil
	}

	// Create SSH command
	sshCommand := ssh.Client{
		Config:         *s.Config.SSH,
		Stdout:         s.Trace,
		Stderr:         s.Trace,
		ConnectRetries: 30,
	}
	sshCommand.Port = sshPort
	sshCommand.Host = "localhost"

	s.Debugln("Connecting to SSH...")
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
	s.Debugln("Reverting VM to current snapshot...")
	err := vbox.RevertToSnapshot(s.vmName)
	if err != nil {
		return err
	}

	return nil
}

func (s *executor) determineBaseSnapshot(baseImage string) string {
	var err error
	baseSnapshot := s.Config.VirtualBox.BaseSnapshot
	if baseSnapshot == "" {
		baseSnapshot, err = vbox.GetCurrentSnapshot(baseImage)
		if err != nil {
			if s.Config.VirtualBox.DisableSnapshots {
				s.Debugln("No snapshots found for base VM", baseImage)
				return ""
			}

			baseSnapshot = "Base State"
		}
	}

	if baseSnapshot != "" && !vbox.HasSnapshot(baseImage, baseSnapshot) {
		if s.Config.VirtualBox.DisableSnapshots {
			s.Warningln("Snapshot", baseSnapshot, "not found in base VM", baseImage)
			return ""
		}

		s.Debugln("Creating snapshot", baseSnapshot, "from current base VM", baseImage, "state...")
		err = vbox.CreateSnapshot(baseImage, baseSnapshot)
		if err != nil {
			s.Warningln("Failed to create snapshot", baseSnapshot, "from base VM", baseImage)
			return ""
		}
	}

	return baseSnapshot
}

// virtualbox doesn't support templates
func (s *executor) createVM(baseImage string) (err error) {
	_, err = vbox.Status(s.vmName)
	if err != nil {
		_ = vbox.Unregister(s.vmName)
	}

	if !vbox.Exist(s.vmName) {
		baseSnapshot := s.determineBaseSnapshot(baseImage)
		if baseSnapshot == "" {
			s.Debugln("Creating testing VM from VM", baseImage, "...")
		} else {
			s.Debugln("Creating testing VM from VM", baseImage, "snapshot", baseSnapshot, "...")
		}

		err = vbox.CreateOsVM(baseImage, s.vmName, baseSnapshot, s.Config.VirtualBox.BaseFolder)
		if err != nil {
			return err
		}
	}

	s.Debugln("Identify SSH Port...")
	s.sshPort, err = vbox.FindSSHPort(s.vmName)
	if err != nil {
		s.Debugln("Creating localhost ssh forwarding...")
		vmSSHPort := s.Config.SSH.Port
		if vmSSHPort == "" {
			vmSSHPort = "22"
		}
		s.sshPort, err = vbox.ConfigureSSH(s.vmName, vmSSHPort)
		if err != nil {
			return err
		}
	}
	s.Debugln("Using local", s.sshPort, "SSH port to connect to VM...")

	s.Debugln("Bootstraping VM...")
	err = s.startVM()
	if err != nil {
		return err
	}

	s.Debugln("Waiting for VM to become responsive...")
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

	if s.Config.VirtualBox.DisableSnapshots && vbox.Exist(s.vmName) {
		s.Debugln("Deleting old VM...")
		killAndUnregisterVM(s.vmName)
	}

	s.tryRestoreFromSnapshot()

	if !vbox.Exist(s.vmName) {
		s.Println("Creating new VM...")
		err = s.createVM(baseName)
		if err != nil {
			return err
		}

		if !s.Config.VirtualBox.DisableSnapshots {
			s.Println("Creating default snapshot...")
			err = vbox.CreateSnapshot(s.vmName, "Started")
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
	version, err := vbox.Version()
	if err != nil {
		return err
	}

	s.Println("Using VirtualBox version", version, "executor...")
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
	s.Debugln("Starting VM...")
	startType := s.Config.VirtualBox.StartType
	if startType == "" {
		startType = "headless"
	}
	err := vbox.Start(s.vmName, startType)
	if err != nil {
		return err
	}
	return nil
}

func (s *executor) tryRestoreFromSnapshot() {
	if !vbox.Exist(s.vmName) {
		return
	}

	s.Println("Restoring VM from snapshot...")
	err := s.restoreFromSnapshot()
	if err != nil {
		s.Println("Previous VM failed. Deleting, because", err)
		killAndUnregisterVM(s.vmName)
	}
}

func killAndUnregisterVM(vmName string) {
	_ = vbox.Kill(vmName)
	_ = vbox.Delete(vmName)
	_ = vbox.Unregister(vmName)
}

func (s *executor) ensureVMStarted() error {
	s.Debugln("Checking VM status...")
	status, err := vbox.Status(s.vmName)
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
		s.Debugln("Waiting for VM to run...")
		err = vbox.WaitForStatus(s.vmName, vbox.Running, 60)
		if err != nil {
			return err
		}
	}

	s.Debugln("Identify SSH Port...")
	sshPort, err := vbox.FindSSHPort(s.vmName)
	s.sshPort = sshPort
	if err != nil {
		return err
	}

	s.Println("Waiting for VM to become responsive...")
	err = s.verifyMachine(s.sshPort)
	if err != nil {
		return err
	}

	s.provisioned = true
	return nil
}

func (s *executor) sshConnect() error {
	s.Println("Starting SSH command...")
	s.sshCommand = ssh.Client{
		Config: *s.Config.SSH,
		Stdout: s.Trace,
		Stderr: s.Trace,
	}
	s.sshCommand.Port = s.sshPort
	s.sshCommand.Host = "localhost"

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
		_ = vbox.Kill(s.vmName)

		if s.Config.VirtualBox.DisableSnapshots || !s.provisioned {
			_ = vbox.Delete(s.vmName)
		}
	}
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

	common.RegisterExecutorProvider("virtualbox", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
