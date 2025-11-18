---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: スロットベースのcgroupサポート
---

スロットベースのcgroupサポートにより、オートスケールでGitLab Runnerを使用する際のリソースの分離と管理が向上します。スロットベースのcgroupは、オートスケーラーによって割り当てられたスロット番号に基づいて、ジョブを特定のコントロールグループ（cgroup）に自動的に割り当てます。

## メリット {#benefits}

- リソース分離の向上: 同じインスタンス上の同時実行ジョブ間のリソース干渉を防ぎます。
- モニタリングの簡素化: スロットごとのリソース使用量を個別に追跡できます。
- デバッグの改善: Cgroupベースのメトリクスは、リソースを大量に消費するジョブの特定に役立ちます。
- きめ細かい制御: 予測可能なパフォーマンスのために、スロットごとにリソース制限を設定します。

## サポートされているexecutor {#supported-executors}

スロットベースのcgroupは、スロット管理に[taskscaler](https://gitlab.com/gitlab-org/fleeting/taskscaler)を使用するオートスケールexecutorで動作します:

- [Docker Autoscaler executor](../executors/docker_autoscaler.md#slot-based-cgroup-support)
- [インスタンスexecutor](../executors/instance.md#slot-based-cgroup-support)

## 前提要件 {#prerequisites}

- cgroup v2をサポートするLinuxホスト
- 初期cgroup階層セットアップのためのルートアクセス
- オートスケーラー機能を備えたGitLab Runner
- スロット割り当て用のtaskscaler（オートスケーラーによって自動的に提供されます）

## 設定 {#configuration}

スロットベースのcgroupサポートを有効にするには、以下を`config.toml`に追加します。

### `systemd` cgroupドライバーを使用するDockerの場合 {#for-docker-with-systemd-cgroup-driver}

Dockerが`systemd` cgroupドライバー（最も一般的）を使用している場合は、`systemd`スライス形式を使用します:

```toml
[[runners]]
  name = "my-autoscaler-runner"
  executor = "docker-autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "runner-slot-${slot}.slice"

  [runners.autoscaler]
    capacity_per_instance = 4
```

### `cgroupfs`ドライバーを使用するDockerの場合 {#for-docker-with-cgroupfs-driver}

Dockerが`cgroupfs`ドライバーを使用している場合は、raw `cgroup`パス形式を使用します:

```toml
[[runners]]
  name = "my-autoscaler-runner"
  executor = "docker-autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "gitlab-runner/slot-${slot}"

  [runners.autoscaler]
    capacity_per_instance = 4
```

### 設定オプション {#configuration-options}

| 設定 | 説明 | デフォルト |
|---------|-------------|---------|
| `use_slot_cgroups` | スロットベースのcgroup割り当てを有効にする | `false` |
| `slot_cgroup_template` | cgroupパスのテンプレート。プレースホルダーとして`${slot}`を使用してください。形式はDockerのcgroupドライバーによって異なります（systemd: `runner-slot-${slot}.slice`、cgroupfs: `gitlab-runner/slot-${slot}`）。 | `"gitlab-runner/slot-${slot}"` |

テンプレートは、スロット番号のプレースホルダーとして`${slot}`を使用したbashスタイルの変数展開を使用します。例: 

- `systemd`ドライバーの場合: スロット5の場合、`runner-slot-${slot}.slice`は`runner-slot-5.slice`になります
- `cgroupfs`ドライバーの場合: スロット5の場合、`gitlab-runner/slot-${slot}`は`gitlab-runner/slot-5`になります

`docker info | grep "Cgroup Driver"`を使用して、Docker cgroupドライバーを確認してください:

### Docker固有の設定 {#docker-specific-configuration}

Docker Autoscaler executorを使用する場合、サービスコンテナに対して個別のテンプレートを指定できます:

```toml
[[runners]]
  executor = "docker-autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "runner-slot-${slot}.slice"

  [runners.docker]
    service_slot_cgroup_template = "runner-slot-${slot}.slice"
```

| 設定 | 説明 | デフォルト |
|---------|-------------|---------|
| `service_slot_cgroup_template` | サービスコンテナcgroupパスのテンプレート。Dockerのcgroupドライバー形式と一致する必要があります | `slot_cgroup_template`と同じです。 |

## 環境セットアップ {#environment-setup}

スロットベースのcgroupを有効にする前に、Runnerホストでcgroup階層を準備します。

### systemd cgroupドライバーのセットアップスクリプト {#setup-script-for-systemd-cgroup-driver}

Dockerが`systemd` cgroupドライバー（`docker info | grep "Cgroup Driver"`で確認）を使用している場合は、raw cgroupディレクトリの代わりに`systemd`スライスを作成する必要があります。

セットアップスクリプトを作成します（`gitlab-runner-systemd-slice-setup.sh`）:

```shell
#!/bin/bash
# gitlab-runner-systemd-slice-setup.sh
# Script to set up systemd slices for GitLab Runner slot-based cgroups
# This example configures 4 slots on an 8-core machine, with each slot pinned to 2 CPUs

set -e

MAX_SLOTS=4  # Adjust based on your capacity_per_instance configuration

# CPU pinning configuration (2 CPUs per slot on an 8-core machine)
# Format: comma-separated CPU list for systemd AllowedCPUs
declare -a CPU_ASSIGNMENTS=(
    "0,1"    # Slot 0: CPUs 0 and 1
    "2,3"    # Slot 1: CPUs 2 and 3
    "4,5"    # Slot 2: CPUs 4 and 5
    "6,7"    # Slot 3: CPUs 6 and 7
)

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root for systemd slice setup"
   exit 1
fi

# Verify systemd is available
if ! command -v systemctl &> /dev/null; then
    echo "Error: systemctl not found. This script requires systemd."
    exit 1
fi

echo "Setting up systemd slices for GitLab Runner"
echo "Configuration: $MAX_SLOTS slots on an 8-core machine (2 CPUs per slot)"

for ((slot=0; slot<MAX_SLOTS; slot++)); do
    slice_name="runner-slot-${slot}.slice"
    echo "Creating systemd slice: $slice_name (CPUs: ${CPU_ASSIGNMENTS[$slot]})"

    # Create systemd slice configuration
    cat > "/etc/systemd/system/$slice_name" <<EOF
[Unit]
Description=GitLab Runner Slot $slot
Before=slices.target

[Slice]
CPUAccounting=true
MemoryAccounting=true
AllowedCPUs=${CPU_ASSIGNMENTS[$slot]}
EOF

done

# Reload systemd to pick up new slice units
systemctl daemon-reload

# Start all slices
for ((slot=0; slot<MAX_SLOTS; slot++)); do
    slice_name="runner-slot-${slot}.slice"
    systemctl start "$slice_name"
done

echo ""
echo "Systemd slices created successfully!"
echo ""
echo "Verifying slices:"
for ((slot=0; slot<MAX_SLOTS; slot++)); do
    slice_name="runner-slot-${slot}.slice"
    status=$(systemctl is-active "$slice_name" 2>/dev/null || echo "inactive")
    echo "  $slice_name: $status"
done

echo ""
echo "To verify CPU assignments, check:"
echo "  systemctl show runner-slot-0.slice | grep AllowedCPUs"
```

セットアップスクリプトを実行します:

```shell
chmod +x gitlab-runner-systemd-slice-setup.sh
sudo ./gitlab-runner-systemd-slice-setup.sh
```

### `cgroupfs`ドライバーのセットアップスクリプト（代替） {#setup-script-for-cgroupfs-driver-alternative}

Dockerが`cgroupfs`の代わりに`systemd`ドライバーを使用している場合は、raw cgroupディレクトリを作成するこの代替スクリプトを使用します:

```shell
#!/bin/bash
# gitlab-runner-cgroup-setup.sh
# Script to set up cgroup v2 hierarchy for GitLab Runner slot-based cgroups
# This example configures 4 slots on an 8-core machine, with each slot pinned to 2 CPUs
# Use this script only if Docker is using the cgroupfs driver (not systemd)

set -e

CGROUP_ROOT="/sys/fs/cgroup"
RUNNER_CGROUP="gitlab-runner"
MAX_SLOTS=4  # Adjust based on your capacity_per_instance configuration

# CPU pinning configuration (2 CPUs per slot on an 8-core machine)
# Format: "cpu_list" - adjust based on your CPU topology
declare -a CPU_ASSIGNMENTS=(
    "0-1"    # Slot 0: CPUs 0 and 1
    "2-3"    # Slot 1: CPUs 2 and 3
    "4-5"    # Slot 2: CPUs 4 and 5
    "6-7"    # Slot 3: CPUs 6 and 7
)

# Check if running as root
if [[ $EUID -ne 0 ]]; then
   echo "This script must be run as root for cgroup setup"
   exit 1
fi

# Verify cgroup v2 is available
if [[ ! -f "$CGROUP_ROOT/cgroup.controllers" ]]; then
    echo "Error: cgroup v2 not detected. This script requires cgroup v2."
    exit 1
fi

echo "Setting up cgroup v2 hierarchy for GitLab Runner"
echo "Configuration: $MAX_SLOTS slots on an 8-core machine (2 CPUs per slot)"

# Create base runner cgroup
mkdir -p "$CGROUP_ROOT/$RUNNER_CGROUP"

# Enable controllers if available
if [[ -f "$CGROUP_ROOT/cgroup.controllers" ]]; then
    echo "+memory +cpu +cpuset" > "$CGROUP_ROOT/cgroup.subtree_control" 2>/dev/null || true
fi

# Create slot-specific cgroups
for ((slot=0; slot<MAX_SLOTS; slot++)); do
    slot_path="$CGROUP_ROOT/$RUNNER_CGROUP/slot-$slot"
    echo "Creating cgroup for slot $slot (CPUs: ${CPU_ASSIGNMENTS[$slot]})"

    mkdir -p "$slot_path"

    # Enable controllers for this slot
    if [[ -f "$CGROUP_ROOT/$RUNNER_CGROUP/cgroup.controllers" ]]; then
        echo "+memory +cpu +cpuset" > "$CGROUP_ROOT/$RUNNER_CGROUP/cgroup.subtree_control" 2>/dev/null || true
    fi

    # Pin slot to specific CPUs
    echo "${CPU_ASSIGNMENTS[$slot]}" > "$slot_path/cpuset.cpus"

    # Set memory nodes (usually 0 for single NUMA node systems)
    echo "0" > "$slot_path/cpuset.mems"

    # Set permissions for GitLab Runner user
    chown -R gitlab-runner:gitlab-runner "$slot_path" 2>/dev/null || true
done

echo "Cgroup setup complete!"

# Verify setup
echo ""
echo "Verifying cgroup setup:"
for ((slot=0; slot<MAX_SLOTS; slot++)); do
    slot_path="$CGROUP_ROOT/$RUNNER_CGROUP/slot-$slot"
    cpus=$(cat "$slot_path/cpuset.cpus")
    echo "  Slot $slot: CPUs $cpus"
done
```

セットアップスクリプトを実行します:

```shell
chmod +x gitlab-runner-cgroup-setup.sh
sudo ./gitlab-runner-cgroup-setup.sh
```

## 動作の仕組み {#how-it-works}

### Docker Autoscaler executor {#docker-autoscaler-executor}

Docker Autoscaler executorは、`--cgroup-parent`フラグを使用して、スロットベースのcgroupパスをDockerコンテナに自動的に適用します。ビルドコンテナとサービスコンテナは両方とも、ジョブスクリプトを変更しなくても、スロット固有のcgroupに割り当てられます。

### インスタンスexecutor {#instance-executor}

インスタンスexecutorは、ジョブに`GITLAB_RUNNER_SLOT_CGROUP`環境変数を提供します。この変数をジョブスクリプトで使用して、スロット固有のcgroupでプロセスを実行できます。

#### `systemd-run`を使用する {#using-systemd-run}

```yaml
job:
  script:
    - echo "Running in cgroup $GITLAB_RUNNER_SLOT_CGROUP"
    - systemd-run --scope --slice=$GITLAB_RUNNER_SLOT_CGROUP ./my-process
```

#### `cgexec`を使用する {#using-cgexec}

```yaml
job:
  script:
    - echo "Running in cgroup $GITLAB_RUNNER_SLOT_CGROUP"
    - cgexec -g cpu,memory:$GITLAB_RUNNER_SLOT_CGROUP ./my-process
```

#### cgroup制限の設定 {#setting-cgroup-limits}

ジョブプロセスを実行する前に、cgroupのリソース制限を設定できます:

```yaml
job:
  script:
    - echo "Configuring cgroup limits"
    - echo "100M" > /sys/fs/cgroup/$GITLAB_RUNNER_SLOT_CGROUP/memory.max
    - echo "50000" > /sys/fs/cgroup/$GITLAB_RUNNER_SLOT_CGROUP/cpu.max
    - ./my-process
```

## トラブルシューティング {#troubleshooting}

### コンテナがcgroupエラーで起動に失敗する {#containers-fail-to-start-with-cgroup-errors}

1. cgroupパスが`/sys/fs/cgroup/`の下に存在することを確認します:

   ```shell
   ls -la /sys/fs/cgroup/gitlab-runner/
   ```

1. GitLab Runnerユーザーにcgroupディレクトリへの書き込みアクセス権があることを確認します:

   ```shell
   ls -la /sys/fs/cgroup/gitlab-runner/slot-0/
   ```

1. `slot_cgroup_template`が`${slot}`プレースホルダーで正しい形式を使用していることを確認します:

1. 特定のcgroup作成エラーについてGitLab Runnerログを確認してください:

1. 手動でテストします:

   Docker Autoscaler executorの場合:

   ```shell
   docker run --rm --cgroup-parent=gitlab-runner/slot-0 alpine echo "test"
   ```

   インスタンスexecutorの場合:

   ```yaml
   job:
     script:
       - echo "Slot cgroup: $GITLAB_RUNNER_SLOT_CGROUP"
   ```

### ジョブが同じcgroupを使用する {#jobs-use-the-same-cgroup}

テンプレートに`${slot}`プレースホルダーが含まれていないという警告がログに表示された場合:

```plaintext
level=warning msg="Slot cgroup template does not contain ${slot} placeholder.
All jobs will use the same cgroup, defeating the purpose of slot-based isolation."
```

これは、`slot_cgroup_template`に`${slot}`変数がないことを意味します。プレースホルダーを含めるように設定を更新します:

```toml
[[runners]]
  slot_cgroup_template = "gitlab-runner/slot-${slot}"
```

### Cgroup v2が利用できません {#cgroup-v2-not-available}

セットアップスクリプトがcgroup v2が検出されないことをレポートする場合は、システムで有効にする必要があるかもしれません。cgroup v2を有効にするには、Linuxディストリビューションのドキュメントを確認してください。最新のディストリビューションでは、通常、デフォルトで有効になっています。
