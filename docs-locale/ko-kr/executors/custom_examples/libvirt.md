---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Custom 실행기를 사용하는 libvirt
---

{{< details >}}

- 티어: Free, Premium, Ultimate
- 제공 서비스: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

[libvirt](https://libvirt.org/)를 사용하면 Custom 실행기 드라이버는 실행하는 모든 작업에 대해 새로운 디스크와 VM을 생성한 후 해당 디스크와 VM을 삭제합니다.

이 문서는 범위를 벗어났기 때문에 libvirt를 설정하는 방법을 설명하지 않습니다. 하지만 이 드라이버는 [GCP 중첩 가상화](https://docs.cloud.google.com/compute/docs/instances/nested-virtualization/overview) 를 사용하여 테스트되었으며, 브리지 네트워킹을 사용하여 [libvirt를 설정하는 방법에 대한 세부 정보](https://docs.cloud.google.com/compute/docs/instances/nested-virtualization/overview#starting_a_private_bridge_between_the_host_and_nested_vms)도 있습니다. 이 예제는 libvirt 설치 시 함께 제공되는 `default` 네트워크를 사용하므로 실행 중인지 확인하세요.

이 드라이버는 각 VM이 전용 IP 주소를 갖고 있어야 하므로 GitLab 러너가 SSH를 통해 VM 내부에 접속하여 명령을 실행할 수 있도록 브리지 네트워킹이 필요합니다. SSH 키는 [다음 명령을 사용하여](https://docs.gitlab.com/user/ssh/#generate-an-ssh-key-pair) 생성할 수 있습니다.

## 기본 이미지 빌드 {#build-the-base-image}

기본 디스크 VM 이미지가 생성되므로 의존성을 빌드할 때마다 다운로드할 필요가 없습니다. 실행할 게스트 운영 체제 제품군에 맞게 빌드합니다.

### Debian 및 Ubuntu (`virt-builder`) {#debian-and-ubuntu-virt-builder}

[`virt-builder`](https://libguestfs.org/virt-builder.1.html)는 템플릿에서 직접 기본 이미지를 생성합니다:

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

이전 명령은 앞서 지정한 모든 [필수 요소](../custom.md#prerequisite-software-for-running-a-job)를 설치합니다.

`virt-builder`는 루트 비밀번호를 자동으로 설정하고 마지막에 인쇄합니다. 자신의 비밀번호를 설정하려면 [`--root-password password:$SOME_PASSWORD`](https://libguestfs.org/virt-builder.1.html#setting-the-root-password)를 전달합니다.

### RHEL, CentOS 및 AlmaLinux (`virt-customize`) {#rhel-centos-and-almalinux-virt-customize}

`virt-builder`는 라이선스가 있는 RHEL 게스트 템플릿이 없습니다. 배포판의 GenericCloud `qcow2`을 다운로드하고 [`virt-customize`](https://libguestfs.org/virt-customize.1.html)로 오프라인으로 사용자 지정합니다. 이 예제는 AlmaLinux 9 `x86_64` 이미지를 사용합니다. RHEL 또는 CentOS Stream 9 이미지 또는 다른 아키텍처로 필요에 따라 대체합니다.

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

RHEL 계열 세부 사항:

- `.rpm.sh` 패키지 리포지토리 스크립트 및 `dnf`를 사용합니다. `virt-customize` 및 `virt-install`을 제공하는 도구는 `guestfs-tools` 패키지에 있습니다.
- 기본 이미지에 작업에 필요한 모든 런타임을 설치합니다. 이 예제는 `gitlab-runner`, `git`, `git-lfs` 및 `openssh-server`를 설치합니다. 작업이 VM 내부에 이미지를 빌드하는 경우 `podman`과 같은 컨테이너 엔진을 추가합니다.
- `--selinux-relabel`을 전달하면 게스트가 SELinux 강제 적용 상태에서 깔끔하게 부팅되고, 이미지를 `/var/lib/libvirt/images/` 아래에 유지하면 `virt_image_t` SELinux 레이블이 적용됩니다.
- Debian 레시피와 달리 GenericCloud 이미지는 `net.ifnames` 또는 `/etc/network/interfaces`가 필요하지 않습니다. `cloud-init` 및 `NetworkManager`로 부팅됩니다. 커널 명령줄을 변경하는 경우 `grub2-mkconfig`로 GRUB을 재생성합니다.
- libvirt 데몬을 시작하고 `virt-host-validate`로 중첩 가상화를 확인합니다. libvirt 9 이상은 모듈식 데몬(`virtqemud` 및 동료)을 제공합니다. 단일식 `libvirtd` 호환성 단위도 작동하며 이미 소켓 활성화되었을 수 있습니다. 설치에서 제공하는 것을 활성화하고 활성 상태인지 확인합니다.
- Custom 실행기 스크립트는 이러한 VM이 있는 시스템 libvirt 인스턴스와 통신해야 합니다. [base](#base) 스크립트는 이 연결을 위해 `export LIBVIRT_DEFAULT_URI="qemu:///system"`을 설정합니다.
- [prepare](#prepare) 스크립트에서 `--os-variant`을 `osinfo-db`이 인식하는 ID로 설정합니다. 이 예제는 `rhel9.0`를 사용합니다. `almalinux9` 또는 `centos-stream9`도 `osinfo-db`에 포함되어 있으면 작동합니다. `osinfo-query os`로 사용 가능한 ID를 나열합니다.

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

# Talk to the system libvirt instance, where these VMs live, rather than the
# per-user session instance.
export LIBVIRT_DEFAULT_URI="qemu:///system"

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
