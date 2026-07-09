---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Using libvirt with the Custom executor
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Using [libvirt](https://libvirt.org/), the Custom executor driver will
create a new disk and VM for every job it executes, after which the disk
and VM will be deleted.

This document does not try to explain how to set up libvirt, since it's
out of scope. However, this driver was tested using
[GCP Nested Virtualization](https://docs.cloud.google.com/compute/docs/instances/nested-virtualization/overview),
which also has
[details on how to set up libvirt](https://docs.cloud.google.com/compute/docs/instances/nested-virtualization/overview#starting_a_private_bridge_between_the_host_and_nested_vms)
with bridge networking. This example will use the `default` network that
comes with when installing libvirt so make sure it's running.

This driver requires bridge networking since each VM needs to have
it's own dedicated IP address so GitLab Runner can SSH inside of it to
run commands. An SSH key can be generated
[using the following commands](https://docs.gitlab.com/user/ssh/#generate-an-ssh-key-pair).

## Build the base image

A base disk VM image is created so that dependencies are not downloaded
every build. Build it for the guest operating system family you run.

### Debian and Ubuntu (`virt-builder`)

[`virt-builder`](https://libguestfs.org/virt-builder.1.html) creates the base
image directly from a template:

```shell
virt-builder debian-12 \
    --size 8G \
    --output /var/lib/libvirt/images/gitlab-runner-base.qcow2 \
    --format qcow2 \
    --hostname gitlab-runner-bookworm \
    --network \
    --install curl \
    --run-command 'curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" | bash' \
    --run-command 'curl -s "https://packagecloud.io/install/repositories/github/git-lfs/script.deb.sh" | bash' \
    --run-command 'useradd -m -p "" gitlab-runner -s /bin/bash' \
    --install gitlab-runner,git,git-lfs,openssh-server \
    --run-command "git lfs install --skip-repo" \
    --ssh-inject gitlab-runner:file:/root/.ssh/id_rsa.pub \
    --run-command "echo 'gitlab-runner ALL=(ALL) NOPASSWD: ALL' >> /etc/sudoers" \
    --run-command "sed -E 's/GRUB_CMDLINE_LINUX=\"\"/GRUB_CMDLINE_LINUX=\"net.ifnames=0 biosdevname=0\"/' -i /etc/default/grub" \
    --run-command "grub-mkconfig -o /boot/grub/grub.cfg" \
    --run-command "echo 'auto eth0' >> /etc/network/interfaces" \
    --run-command "echo 'allow-hotplug eth0' >> /etc/network/interfaces" \
    --run-command "echo 'iface eth0 inet dhcp' >> /etc/network/interfaces"
```

The previous command installs all the
[prerequisites](../custom.md#prerequisite-software-for-running-a-job) specified
earlier.

`virt-builder` sets a root password automatically and prints it at the end.
To set your own, pass
[`--root-password password:$SOME_PASSWORD`](https://libguestfs.org/virt-builder.1.html#setting-the-root-password).

### RHEL, CentOS, and AlmaLinux (`virt-customize`)

`virt-builder` ships no licensed RHEL guest template. Download the
distribution's GenericCloud `qcow2` and customize it offline with
[`virt-customize`](https://libguestfs.org/virt-customize.1.html). This example
uses the AlmaLinux 9 `x86_64` image; substitute the RHEL or CentOS Stream 9
image, or a different architecture, as needed.

```shell
IMAGES=/var/lib/libvirt/images
BASE="$IMAGES/gitlab-runner-base.qcow2"

curl -fL "https://repo.almalinux.org/almalinux/9/cloud/x86_64/images/AlmaLinux-9-GenericCloud-latest.x86_64.qcow2" -o "$BASE"
qemu-img resize "$BASE" 12G

virt-customize -a "$BASE" \
    --run-command 'curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.rpm.sh" | bash' \
    --run-command 'curl -L "https://packagecloud.io/install/repositories/github/git-lfs/script.rpm.sh" | bash' \
    --install gitlab-runner,git,git-lfs,openssh-server \
    --run-command 'git lfs install --skip-repo' \
    --run-command 'id gitlab-runner >/dev/null 2>&1 || useradd -m -s /bin/bash gitlab-runner' \
    --ssh-inject gitlab-runner:file:/root/.ssh/id_rsa.pub \
    --run-command 'echo "gitlab-runner ALL=(ALL) NOPASSWD: ALL" > /etc/sudoers.d/gitlab-runner' \
    --run-command 'systemctl enable sshd' \
    --selinux-relabel
```

RHEL-family specifics:

- Use the `.rpm.sh` package repository scripts and `dnf`. The tools that provide
  `virt-customize` and `virt-install` are in the `guestfs-tools` package.
- Install whatever runtime your jobs need into the base image. This example
  installs `gitlab-runner`, `git`, `git-lfs`, and `openssh-server`. Add a
  container engine such as `podman` if jobs build images inside the VM.
- Pass `--selinux-relabel` so the guest boots clean under enforcing SELinux, and
  keep images under `/var/lib/libvirt/images/` so they carry the `virt_image_t`
  SELinux label.
- Unlike the Debian recipe, the GenericCloud image doesn't need `net.ifnames` or
  `/etc/network/interfaces`. It boots with `cloud-init` and `NetworkManager`.
  If you do change the kernel command line, regenerate GRUB with `grub2-mkconfig`.
- Start a libvirt daemon and confirm nested virtualization with
  `virt-host-validate`. libvirt 9 and later ship the modular daemons
  (`virtqemud` and companions). The monolithic `libvirtd` compatibility unit also
  works and might already be socket-activated. Enable whichever your
  installation provides and confirm it is active.
- The Custom executor scripts must talk to the system libvirt instance, where
  these VMs live. The [base](#base) script sets
  `export LIBVIRT_DEFAULT_URI="qemu:///system"` for this connection.
- In the [prepare](#prepare) script, set `--os-variant` to an ID your `osinfo-db`
  recognizes. This example uses `rhel9.0`. `almalinux9` or `centos-stream9` also
  work if `osinfo-db` includes them. List available IDs with
  `osinfo-query os`.

## Configuration

The following is an example of a GitLab Runner configuration for libvirt:

```toml
concurrent = 1
check_interval = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = "libvirt-driver"
  url = "https://gitlab.com/"
  token = "xxxxx"
  executor = "custom"
  builds_dir = "/home/gitlab-runner/builds"
  cache_dir = "/home/gitlab-runner/cache"
  [runners.custom_build_dir]
  [runners.cache]
    [runners.cache.s3]
    [runners.cache.gcs]
  [runners.custom]
    prepare_exec = "/opt/libvirt-driver/prepare.sh" # Path to a bash script to create VM.
    run_exec = "/opt/libvirt-driver/run.sh" # Path to a bash script to run script inside of VM over ssh.
    cleanup_exec = "/opt/libvirt-driver/cleanup.sh" # Path to a bash script to delete VM and disks.
```

## Base

Each stage ([prepare](#prepare), [run](#run), and [cleanup](#cleanup))
will use the base script below to generate variables that are used
throughout other scripts.

It's important that this script is located in the same directory as the
other scripts, in this case `/opt/libvirt-driver/`.

```shell
#!/usr/bin/env bash

# /opt/libvirt-driver/base.sh

VM_IMAGES_PATH="/var/lib/libvirt/images"
BASE_VM_IMAGE="$VM_IMAGES_PATH/gitlab-runner-base.qcow2"
VM_ID="runner-$CUSTOM_ENV_CI_RUNNER_ID-project-$CUSTOM_ENV_CI_PROJECT_ID-concurrent-$CUSTOM_ENV_CI_CONCURRENT_PROJECT_ID-job-$CUSTOM_ENV_CI_JOB_ID"
VM_IMAGE="$VM_IMAGES_PATH/$VM_ID.qcow2"

# Talk to the system libvirt instance, where these VMs live, rather than the
# per-user session instance.
export LIBVIRT_DEFAULT_URI="qemu:///system"

_get_vm_ip() {
    virsh -q domifaddr "$VM_ID" | awk '{print $4}' | sed -E 's|/([0-9]+)?$||'
}
```

## Prepare

The prepare script:

- Copies the disk to a new path.
- Installs a new VM from the copied disk.
- Waits for the VM to get an IP.
- Waits for SSH to respond on the VM.

```shell
#!/usr/bin/env bash

# /opt/libvirt-driver/prepare.sh

currentDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${currentDir}/base.sh # Get variables from base script.

set -eo pipefail

# trap any error, and mark it as a system failure.
trap "exit $SYSTEM_FAILURE_EXIT_CODE" ERR

# Copy base disk to use for Job.
qemu-img create -f qcow2 -b "$BASE_VM_IMAGE" "$VM_IMAGE" -F qcow2

# Install the VM
# To boot VM in UEFI mode, add: --boot uefi
virt-install \
    --name "$VM_ID" \
    --os-variant debian12 \
    --disk "$VM_IMAGE" \
    --import \
    --vcpus=2 \
    --ram=2048 \
    --network default \
    --graphics none \
    --noautoconsole

# Wait for VM to get IP
echo 'Waiting for VM to get IP'
for i in $(seq 1 300); do
    VM_IP=$(_get_vm_ip)

    if [ -n "$VM_IP" ]; then
        echo "VM got IP: $VM_IP"
        break
    fi

    if [ "$i" == "300" ]; then
        echo 'Waited 300 seconds for VM to start, exiting...'
        # Inform GitLab Runner that this is a system failure, so it
        # should be retried.
        exit "$SYSTEM_FAILURE_EXIT_CODE"
    fi

    sleep 1s
done

# Wait for ssh to become available
echo "Waiting for sshd to be available"
for i in $(seq 1 300); do
    if ssh -i /root/.ssh/id_rsa -o StrictHostKeyChecking=no gitlab-runner@$VM_IP >/dev/null 2>/dev/null; then
        break
    fi

    if [ "$i" == "300" ]; then
        echo 'Waited 300 seconds for sshd to start, exiting...'
        # Inform GitLab Runner that this is a system failure, so it
        # should be retried.
        exit "$SYSTEM_FAILURE_EXIT_CODE"
    fi

    sleep 1s
done
```

## Run

This will run the script generated by GitLab Runner by sending
the content of the script to the VM via `STDIN` through SSH.

```shell
#!/usr/bin/env bash

# /opt/libvirt-driver/run.sh

currentDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${currentDir}/base.sh # Get variables from base script.

VM_IP=$(_get_vm_ip)

ssh -i /root/.ssh/id_rsa -o StrictHostKeyChecking=no gitlab-runner@$VM_IP /bin/bash < "${1}"
if [ $? -ne 0 ]; then
    # Exit using the variable, to make the build as failure in GitLab
    # CI.
    exit "$BUILD_FAILURE_EXIT_CODE"
fi
```

## Cleanup

This script removes the VM and deletes the disk.

```shell
#!/usr/bin/env bash

# /opt/libvirt-driver/cleanup.sh

currentDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${currentDir}/base.sh # Get variables from base script.

set -eo pipefail

# Destroy VM and wait 300 second.
for i in $(seq 1 300); do
  virsh destroy "$VM_ID" >/dev/null 2>&1
  if [[ "$(virsh domstate "$VM_ID" 2>/dev/null | tr '[:upper:]' '[:lower:]')" =~ shut\ off|destroyed|^$ ]]; then
      break
  fi
  if [ $i -eq 300 ]; then
     exit "$SYSTEM_FAILURE_EXIT_CODE"
  fi
  sleep 1
done

# Undefine VM.
virsh undefine "$VM_ID" || virsh undefine "$VM_ID" --nvram

# Delete VM disk.
if [ -f "$VM_IMAGE" ]; then
    rm "$VM_IMAGE"
fi
```
