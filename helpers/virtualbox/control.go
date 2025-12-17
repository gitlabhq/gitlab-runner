package virtualbox

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
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

var hddInfoRe = regexp.MustCompile(`UUID:[[:space:]]*([a-f0-9\-]+)[\s|\S]*?Location:[[:space:]]*([a-zA-Z0-9 -/\\]*)`)

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

func VboxManageOutput(ctx context.Context, exe string, args ...string) (string, error) {
	var stdout, stderr bytes.Buffer
	logrus.Debugf("Executing VBoxManageOutput: %#v", args)
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	stderrString := strings.TrimSpace(stderr.String())

	if _, ok := err.(*exec.ExitError); ok {
		err = fmt.Errorf("VBoxManageOutput error: %s", stderrString)
	}

	return stdout.String(), err
}

func VBoxManage(ctx context.Context, args ...string) (string, error) {
	return VboxManageOutput(ctx, "vboxmanage", args...)
}

func Version(ctx context.Context) (string, error) {
	version, err := VBoxManage(ctx, "--version")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(version), nil
}

func FindSSHPort(ctx context.Context, vmName string) (port string, err error) {
	info, err := VBoxManage(ctx, "showvminfo", vmName)
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

func Exist(ctx context.Context, vmName string) bool {
	_, err := VBoxManage(ctx, "showvminfo", vmName)
	return err == nil
}

func CreateOsVM(
	ctx context.Context,
	vmName string,
	templateName string,
	templateSnapshot string,
	baseFolder string,
) error {
	args := []string{"clonevm", vmName, "--mode", "machine", "--name", templateName, "--register"}
	if templateSnapshot != "" {
		args = append(args, "--snapshot", templateSnapshot, "--options", "link")
	}
	if baseFolder != "" {
		args = append(args, "--basefolder", baseFolder)
	}
	_, err := VBoxManage(ctx, args...)
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

func getUsedVirtualBoxPorts(ctx context.Context) (usedPorts [][]string, err error) {
	output, err := VBoxManage(ctx, "list", "vms", "-l")
	if err != nil {
		return
	}
	allPortsRe := regexp.MustCompile(`host port = (\d+)`)
	usedPorts = allPortsRe.FindAllStringSubmatch(output, -1)
	return
}

func allocatePort(ctx context.Context, handler func(port string) error) (port string, err error) {
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		logrus.Debugln("VirtualBox ConfigureSSH:", err)
		return
	}
	defer func() { _ = ln.Close() }()

	usedPorts, err := getUsedVirtualBoxPorts(ctx)
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

func ConfigureSSH(ctx context.Context, vmName string, vmSSHPort string) (port string, err error) {
	for {
		port, err = allocatePort(
			ctx,
			func(port string) error {
				rule := fmt.Sprintf("guestssh,tcp,127.0.0.1,%s,,%s", port, vmSSHPort)
				_, err = VBoxManage(ctx, "modifyvm", vmName, "--natpf1", rule)
				return err
			},
		)
		if err == nil || err != os.ErrExist {
			return
		}
	}
}

func CreateSnapshot(ctx context.Context, vmName string, snapshotName string) error {
	_, err := VBoxManage(ctx, "snapshot", vmName, "take", snapshotName)
	return err
}

func RevertToSnapshot(ctx context.Context, vmName string) error {
	_, err := VBoxManage(ctx, "snapshot", vmName, "restorecurrent")
	return err
}

func matchSnapshotName(snapshotName string, snapshotList string) bool {
	snapshotRe := regexp.MustCompile(
		fmt.Sprintf(`(?m)^Snapshot(Name|UUID)[^=]*="(%s)"\r?$`, regexp.QuoteMeta(snapshotName)),
	)
	snapshot := snapshotRe.FindStringSubmatch(snapshotList)
	return snapshot != nil
}

func HasSnapshot(ctx context.Context, vmName string, snapshotName string) bool {
	output, err := VBoxManage(ctx, "snapshot", vmName, "list", "--machinereadable")
	if err != nil {
		return false
	}
	return matchSnapshotName(snapshotName, output)
}

func matchCurrentSnapshotName(snapshotList string) []string {
	snapshotRe := regexp.MustCompile(`(?m)^CurrentSnapshotName="([^"]*)"\r?$`)
	return snapshotRe.FindStringSubmatch(snapshotList)
}

func GetCurrentSnapshot(ctx context.Context, vmName string) (string, error) {
	output, err := VBoxManage(ctx, "snapshot", vmName, "list", "--machinereadable")
	if err != nil {
		return "", err
	}
	snapshot := matchCurrentSnapshotName(output)
	if snapshot == nil {
		return "", errors.New("failed to match current snapshot name")
	}
	return snapshot[1], nil
}

func Start(ctx context.Context, vmName string, startType string) error {
	_, err := VBoxManage(ctx, "startvm", vmName, "--type", startType)
	return err
}

func Kill(ctx context.Context, vmName string) error {
	_, err := VBoxManage(ctx, "controlvm", vmName, "poweroff")
	return err
}

func Delete(ctx context.Context, vmName string) error {
	_, err := VBoxManage(ctx, "unregistervm", vmName, "--delete")
	if err == nil {
		return nil
	}
	// VM itself does not exist, but there are some dangling resources which need to be cleaned up
	// This occurs when the VM boot up was prematurely aborted e.g. user cancels the job while VM is booting up.
	// Unregistering a non-existent VM returns an error above.
	hdds, err := ListHDDForVM(ctx, vmName)
	if err != nil {
		return err
	}
	for _, hdd := range hdds {
		if err := DeleteHDD(ctx, hdd); err != nil {
			return err
		}
	}
	// Does not handle default folder change after this VM is created
	folder, err := GetDefaultMachineFolder(ctx)
	if err != nil {
		return fmt.Errorf("failed to get machine folder: %w", err)
	}
	vmFolder := filepath.Join(folder, vmName)
	// Check if the vm folder is a child folder of `folder` to add another check preventing path traversal attacks
	immediate := helpers.IsImmediateChild(folder, vmFolder)
	if !immediate {
		return fmt.Errorf("vm machine folder is not immediate child of the default machine folder")
	}
	return os.RemoveAll(vmFolder)
}

func Status(ctx context.Context, vmName string) (StatusType, error) {
	output, err := VBoxManage(ctx, "showvminfo", vmName, "--machinereadable")
	statusRe := regexp.MustCompile(`VMState="(\w+)"`)
	status := statusRe.FindStringSubmatch(output)
	if err != nil {
		return NotFound, err
	}
	return StatusType(status[1]), nil
}

func WaitForStatus(ctx context.Context, vmName string, vmStatus StatusType, seconds int) error {
	var status StatusType
	var err error
	for i := 0; i < seconds; i++ {
		status, err = Status(ctx, vmName)
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

func Unregister(ctx context.Context, vmName string) error {
	_, err := VBoxManage(ctx, "unregistervm", vmName)
	return err
}

func GetDefaultMachineFolder(ctx context.Context) (string, error) {
	output, err := VBoxManage(ctx, "list", "systemproperties")
	if err != nil {
		return "", err
	}
	_, after, found := strings.Cut(output, "Default machine folder:")
	if !found {
		return "", errors.New("failed to extract default machine folder")
	}
	if after == "" {
		return "", errors.New("empty default machine folder")
	}
	return filepath.Clean(strings.TrimSpace(after)), nil
}

func extractHDDInfo(output string) [][]string {
	return hddInfoRe.FindAllStringSubmatch(output, -1)
}

func ListHDDForVM(ctx context.Context, vmName string) ([]string, error) {
	output, err := VBoxManage(ctx, "list", "hdds")
	if err != nil {
		return nil, err
	}
	hddsResult := extractHDDInfo(output)

	// Check if location contains the VM name.
	// Do not use the machine folder path since it can be overridden and any new value only affects new VMs.
	// VM name is surrounded by path separator to prevent any possible substring matching.
	vmPath := string(filepath.Separator) + vmName + string(filepath.Separator)
	locationRe := regexp.MustCompile(regexp.QuoteMeta(vmPath))

	var hdds []string
	for _, match := range hddsResult {
		if len(match) >= 3 {
			hdd := match[1]
			location := match[2]
			if locationRe.MatchString(location) {
				hdds = append(hdds, hdd)
			}
		} else {
			return nil, errors.New("failed to find hdds for vm")
		}
	}

	return hdds, nil
}

func DeleteHDD(ctx context.Context, identifier string) error {
	_, err := VBoxManage(ctx, "closemedium", identifier, "--delete")
	return err
}
