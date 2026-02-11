---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Custom executorでlibvirtを使用する
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

[libvirt](https://libvirt.org/)を使用すると、Custom executorドライバーは、実行するジョブごとに新しいディスクとVMを作成し、その後、ディスクとVMは削除されます。

このドキュメントでは、libvirtのセットアップ方法については、スコープ外であるため説明しません。ただし、このドライバーは[GCPネストされた](https://docs.cloud.google.com/compute/docs/instances/nested-virtualization/overview)仮想化を使用してテストされており、ブリッジネットワーキングで[libvirtをセットアップする方法の詳細](https://docs.cloud.google.com/compute/docs/instances/nested-virtualization/overview#starting_a_private_bridge_between_the_host_and_nested_vms)も記載されています。この例では、libvirtのインストール時に付属する`default`ネットワークを使用するため、実行されていることを確認してください。

このドライバーはブリッジネットワーキングを必要とします。これは、各VMが専用のIPアドレスを持っている必要があるため、GitLab RunnerがSSH内部でコマンドを実行できるためです。SSHキーは、[次のコマンドを使用して](https://docs.gitlab.com/user/ssh/#generate-an-ssh-key-pair)生成できます。

依存関係がすべてのビルドでダウンロードされないように、ベースディスクVMイメージが作成されます。次の例では、ディスクVMイメージを作成するために[virt-builder](https://libguestfs.org/virt-builder.1.html)が使用されています。

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

上記のコマンドは、[前提条件](../custom.md#prerequisite-software-for-running-a-job)以前に指定されたすべてをインストールします。

`virt-builder`は、最後に印刷されるルートパスワードを自動的に設定します。パスワードを自分で指定する場合は、[`--root-password password:$SOME_PASSWORD`](https://libguestfs.org/virt-builder.1.html#setting-the-root-password)を渡します。

## 設定 {#configuration}

以下は、libvirtのGitLab Runner設定の例です:

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

## Base {#base}

各ステージ（[prepare](#prepare) 、[run](#run) 、および[cleanup](#cleanup)）は、他のスクリプト全体で使用される変数を生成するために、以下のベーススクリプトを使用します。

このスクリプトが他のスクリプトと同じディレクトリにあることが重要です。この場合、`/opt/libvirt-driver/`です。

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

## Prepare {#prepare}

準備スクリプト:

- ディスクを新しいパスにコピーします。
- コピーされたディスクから新しいVMをインストールします。
- VMがIPを取得するのを待ちます。
- VMでSSHが応答するのを待ちます。

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

## Run {#run}

これにより、SSHを介して`STDIN`経由でスクリプトのコンテンツをVMに送信することにより、GitLab Runnerによって生成されたスクリプトが実行されます。

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

## Cleanup {#cleanup}

このスクリプトは、VMを削除し、ディスクを削除します。

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
