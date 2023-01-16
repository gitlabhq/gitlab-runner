package virtualbox

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

type StatusType string

const (
	NotFound               StatusType = "notfound"
	PoweredOff             StatusType = "poweroff"
	Saved                  StatusType = "saved"
	Teleported             StatusType = "teleported"
	Aborted                StatusType = "aborted"
	Running                StatusType = "running"
	Paused                 StatusType = "paused"
	Stuck                  StatusType = "gurumeditation"
	Teleporting            StatusType = "teleporting"
	LiveSnapshotting       StatusType = "livesnapshotting"
	Starting               StatusType = "starting"
	Stopping               StatusType = "stopping"
	Saving                 StatusType = "saving"
	Restoring              StatusType = "restoring"
	TeleportingPausedVM    StatusType = "teleportingpausedvm"
	TeleportingIn          StatusType = "teleportingin"
	FaultTolerantSyncing   StatusType = "faulttolerantsyncing"
	DeletingSnapshotOnline StatusType = "deletingsnapshotlive"
	DeletingSnapshotPaused StatusType = "deletingsnapshotlivepaused"
	OnlineSnapshotting     StatusType = "onlinesnapshotting"
	RestoringSnapshot      StatusType = "restoringsnapshot"
	DeletingSnapshot       StatusType = "deletingsnapshot"
	SettingUp              StatusType = "settingup"
	Snapshotting           StatusType = "snapshotting"
	Unknown                StatusType = "unknown"
	// TODO: update as new VM states are added
)

func IsStatusOnlineOrTransient(vmStatus StatusType) bool {
	switch vmStatus {
	case Running,
		Paused,
		Stuck,
		Teleporting,
		LiveSnapshotting,
		Starting,
		Stopping,
		Saving,
		Restoring,
		TeleportingPausedVM,
		TeleportingIn,
		FaultTolerantSyncing,
		DeletingSnapshotOnline,
		DeletingSnapshotPaused,
		OnlineSnapshotting,
		RestoringSnapshot,
		DeletingSnapshot,
		SettingUp,
		Snapshotting:
		return true
	}

	return false
}

func VboxManageOutput(exe string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	logrus.Debugf("Executing VBoxManageOutput: %#v", args)
	cmd := exec.Command(exe, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	stderrString := strings.TrimSpace(stderr.String())

	if _, ok := err.(*exec.ExitError); ok {
		err = fmt.Errorf("VBoxManageOutput error: %s", stderrString)
	}

	return stdout.String(), err
}

func VBoxManage(args ...string) (string, error) {
	return VboxManageOutput("vboxmanage", args...)
}

func Version() (string, error) {
	version, err := VBoxManage("--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(version), nil
}

func FindSSHPort(vmName string) (port string, err error) {
	info, err := VBoxManage("showvminfo", vmName)
	if err != nil {
		return
	}
	portRe := regexp.MustCompile(`guestssh.*host port = (\d+)`)
	sshPort := portRe.FindStringSubmatch(info)
	if len(sshPort) >= 2 {
		port = sshPort[1]
	} else {
		err = errors.New("failed to find guestssh port")
	}
	return
}

func Exist(vmName string) bool {
	_, err := VBoxManage("showvminfo", vmName)
	return err == nil
}

func CreateOsVM(vmName string, templateName string, templateSnapshot string, baseFolder string) error {
	args := []string{"clonevm", vmName, "--mode", "machine", "--name", templateName, "--register"}
	if templateSnapshot != "" {
		args = append(args, "--snapshot", templateSnapshot, "--options", "link")
	}
	if baseFolder != "" {
		args = append(args, "--basefolder", baseFolder)
	}
	_, err := VBoxManage(args...)
	return err
}

func isPortUnassigned(testPort string, usedPorts [][]string) bool {
	for _, port := range usedPorts {
		if testPort == port[1] {
			return false
		}
	}
	return true
}

func getUsedVirtualBoxPorts() (usedPorts [][]string, err error) {
	output, err := VBoxManage("list", "vms", "-l")
	if err != nil {
		return
	}
	allPortsRe := regexp.MustCompile(`host port = (\d+)`)
	usedPorts = allPortsRe.FindAllStringSubmatch(output, -1)
	return
}

func allocatePort(handler func(port string) error) (port string, err error) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		logrus.Debugln("VirtualBox ConfigureSSH:", err)
		return
	}
	defer func() { _ = ln.Close() }()

	usedPorts, err := getUsedVirtualBoxPorts()
	if err != nil {
		logrus.Debugln("VirtualBox ConfigureSSH:", err)
		return
	}

	addressElements := strings.Split(ln.Addr().String(), ":")
	port = addressElements[len(addressElements)-1]

	if isPortUnassigned(port, usedPorts) {
		err = handler(port)
	} else {
		err = os.ErrExist
	}
	return
}

func ConfigureSSH(vmName string, vmSSHPort string) (port string, err error) {
	for {
		port, err = allocatePort(
			func(port string) error {
				rule := fmt.Sprintf("guestssh,tcp,127.0.0.1,%s,,%s", port, vmSSHPort)
				_, err = VBoxManage("modifyvm", vmName, "--natpf1", rule)
				return err
			},
		)
		if err == nil || err != os.ErrExist {
			return
		}
	}
}

func CreateSnapshot(vmName string, snapshotName string) error {
	_, err := VBoxManage("snapshot", vmName, "take", snapshotName)
	return err
}

func RevertToSnapshot(vmName string) error {
	_, err := VBoxManage("snapshot", vmName, "restorecurrent")
	return err
}

func matchSnapshotName(snapshotName string, snapshotList string) bool {
	snapshotRe := regexp.MustCompile(
		fmt.Sprintf(`(?m)^Snapshot(Name|UUID)[^=]*="(%s)"\r?$`, regexp.QuoteMeta(snapshotName)),
	)
	snapshot := snapshotRe.FindStringSubmatch(snapshotList)
	return snapshot != nil
}

func HasSnapshot(vmName string, snapshotName string) bool {
	output, err := VBoxManage("snapshot", vmName, "list", "--machinereadable")
	if err != nil {
		return false
	}
	return matchSnapshotName(snapshotName, output)
}

func matchCurrentSnapshotName(snapshotList string) []string {
	snapshotRe := regexp.MustCompile(`(?m)^CurrentSnapshotName="([^"]*)"\r?$`)
	return snapshotRe.FindStringSubmatch(snapshotList)
}

func GetCurrentSnapshot(vmName string) (string, error) {
	output, err := VBoxManage("snapshot", vmName, "list", "--machinereadable")
	if err != nil {
		return "", err
	}
	snapshot := matchCurrentSnapshotName(output)
	if snapshot == nil {
		return "", errors.New("failed to match current snapshot name")
	}
	return snapshot[1], nil
}

func Start(vmName string, startType string) error {
	_, err := VBoxManage("startvm", vmName, "--type", startType)
	return err
}

func Kill(vmName string) error {
	_, err := VBoxManage("controlvm", vmName, "poweroff")
	return err
}

func Delete(vmName string) error {
	_, err := VBoxManage("unregistervm", vmName, "--delete")
	return err
}

func Status(vmName string) (StatusType, error) {
	output, err := VBoxManage("showvminfo", vmName, "--machinereadable")
	statusRe := regexp.MustCompile(`VMState="(\w+)"`)
	status := statusRe.FindStringSubmatch(output)
	if err != nil {
		return NotFound, err
	}
	return StatusType(status[1]), nil
}

func WaitForStatus(vmName string, vmStatus StatusType, seconds int) error {
	var status StatusType
	var err error
	for i := 0; i < seconds; i++ {
		status, err = Status(vmName)
		if err != nil {
			return err
		}
		if status == vmStatus {
			return nil
		}
		time.Sleep(time.Second)
	}
	return errors.New("VM " + vmName + " is in " + string(status) + " where it should be in " + string(vmStatus))
}

func Unregister(vmName string) error {
	_, err := VBoxManage("unregistervm", vmName)
	return err
}
