---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Custom 실행기를 사용하는 libvirt
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

[libvirt](https://libvirt.org/)를 사용하면 Custom 실행기 드라이버는 실행하는 모든 작업에 대해 새로운 디스크와 VM을 생성한 후 해당 디스크와 VM을 삭제합니다.

이 문서는 범위를 벗어났기 때문에 libvirt를 설정하는 방법을 설명하지 않습니다. 하지만 이 드라이버는 [GCP 중첩 가상화](https://docs.cloud.google.com/compute/docs/instances/nested-virtualization/overview) 를 사용하여 테스트되었으며, 브리지 네트워킹을 사용하여 [libvirt를 설정하는 방법에 대한 세부 정보](https://docs.cloud.google.com/compute/docs/instances/nested-virtualization/overview#starting_a_private_bridge_between_the_host_and_nested_vms)도 있습니다. 이 예제는 libvirt 설치 시 함께 제공되는 `default` 네트워크를 사용하므로 실행 중인지 확인하세요.

이 드라이버는 각 VM이 전용 IP 주소를 갖고 있어야 하므로 GitLab 러너가 SSH를 통해 VM 내부에 접속하여 명령을 실행할 수 있도록 브리지 네트워킹이 필요합니다. SSH 키는 [다음 명령을 사용하여](https://docs.gitlab.com/user/ssh/#generate-an-ssh-key-pair) 생성할 수 있습니다.

기본 디스크 VM 이미지가 생성되므로 의존성을 빌드할 때마다 다운로드할 필요가 없습니다. 다음 예제에서는 [virt-builder](https://libguestfs.org/virt-builder.1.html)를 사용하여 디스크 VM 이미지를 생성합니다.

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

위의 명령은 앞서 지정한 모든 [필수 소프트웨어](../custom.md#prerequisite-software-for-running-a-job)를 설치합니다.

`virt-builder`은 루트 비밀번호를 자동으로 설정하며, 이는 끝에 인쇄됩니다. 비밀번호를 직접 지정하려면 [`--root-password password:$SOME_PASSWORD`](https://libguestfs.org/virt-builder.1.html#setting-the-root-password)를 전달하세요.

## 구성 {#configuration}

다음은 libvirt에 대한 GitLab 러너 구성의 예입니다:

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

## 기본 {#base}

각 스테이지([준비](#prepare) , [실행](#run) , [정리](#cleanup))는 다른 스크립트 전체에서 사용되는 변수를 생성하기 위해 아래 기본 스크립트를 사용합니다.

이 스크립트가 다른 스크립트와 동일한 디렉터리(이 경우 `/opt/libvirt-driver/`)에 위치해야 합니다.

```shell
#!/usr/bin/env bash

# /opt/libvirt-driver/base.sh

VM_IMAGES_PATH="/var/lib/libvirt/images"
BASE_VM_IMAGE="$VM_IMAGES_PATH/gitlab-runner-base.qcow2"
VM_ID="runner-$CUSTOM_ENV_CI_RUNNER_ID-project-$CUSTOM_ENV_CI_PROJECT_ID-concurrent-$CUSTOM_ENV_CI_CONCURRENT_PROJECT_ID-job-$CUSTOM_ENV_CI_JOB_ID"
VM_IMAGE="$VM_IMAGES_PATH/$VM_ID.qcow2"

_get_vm_ip() {
    virsh -q domifaddr "$VM_ID" | awk '{print $4}' | sed -E 's|/([0-9]+)?$||'
}
```

## 준비 {#prepare}

준비 스크립트:

- 디스크를 새로운 경로로 복사합니다.
- 복사한 디스크에서 새로운 VM을 설치합니다.
- VM이 IP를 받을 때까지 기다립니다.
- VM에서 SSH가 응답할 때까지 기다립니다.

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
    --os-variant debian11 \
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

## 실행 {#run}

이것은 GitLab 러너가 생성한 스크립트를 SSH를 통해 `STDIN` 방식으로 VM에 스크립트의 내용을 전송하여 실행합니다.

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

## 정리 {#cleanup}

이 스크립트는 VM을 제거하고 디스크를 삭제합니다.

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
