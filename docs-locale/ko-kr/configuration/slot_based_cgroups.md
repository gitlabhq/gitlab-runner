---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 슬롯 기반 cgroup 지원
---

슬롯 기반 cgroup 지원은 러너와 함께 자동 크기 조정을 사용할 때 리소스 격리 및 관리를 개선합니다. 슬롯 기반 cgroup은 자동 크기 조정기가 할당한 슬롯 번호를 기반으로 작업을 특정 제어 그룹(cgroup)에 자동으로 할당합니다.

## 이점 {#benefits}

- 더 나은 리소스 격리:  동일한 인스턴스의 동시 작업 간에 리소스 간섭을 방지합니다.
- 더 쉬운 모니터링:  슬롯별 리소스 사용 현황을 독립적으로 추적할 수 있습니다.
- 향상된 디버깅:  Cgroup 기반 메트릭은 리소스를 많이 사용하는 작업을 식별하는 데 도움이 됩니다.
- 미세한 제어:  예측 가능한 성능을 위해 슬롯당 리소스 제한을 설정합니다.

## 지원되는 실행기 {#supported-executors}

슬롯 기반 cgroup은 슬롯 관리를 위해 [taskscaler](https://gitlab.com/gitlab-org/fleeting/taskscaler)를 사용하는 자동 크기 조정 실행기와 함께 작동합니다:

- [Docker Autoscaler 실행기](../executors/docker_autoscaler.md#slot-based-cgroup-support)
- [인스턴스 실행기](../executors/instance.md#slot-based-cgroup-support)

## 필수 요구 사항 {#prerequisites}

- cgroup v2 지원이 있는 Linux 호스트
- 초기 cgroup 계층 구조 설정을 위한 루트 액세스
- 러너 자동 크기 조정 기능
- 슬롯 할당을 위한 Taskscaler(자동 크기 조정기에서 자동으로 제공됨)

## 구성 {#configuration}

슬롯 기반 cgroup 지원을 활성화하려면 다음을 `config.toml`에 추가합니다.

### `systemd` cgroup 드라이버를 사용하는 Docker {#for-docker-with-systemd-cgroup-driver}

Docker가 `systemd` cgroup 드라이버를 사용하는 경우(가장 일반적임), `systemd` 슬라이스 형식을 사용합니다:

```toml
[[runners]]
  name = "my-autoscaler-runner"
  executor = "docker-autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "runner-slot-${slot}.slice"

  [runners.autoscaler]
    capacity_per_instance = 4
```

### `cgroupfs` 드라이버를 사용하는 Docker {#for-docker-with-cgroupfs-driver}

Docker가 `cgroupfs` 드라이버를 사용하는 경우, 원시 `cgroup` 경로 형식을 사용합니다:

```toml
[[runners]]
  name = "my-autoscaler-runner"
  executor = "docker-autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "gitlab-runner/slot-${slot}"

  [runners.autoscaler]
    capacity_per_instance = 4
```

### 구성 옵션 {#configuration-options}

| 설정 | 설명 | 기본값 |
|---------|-------------|---------|
| `use_slot_cgroups` | 슬롯 기반 cgroup 할당 활성화 | `false` |
| `slot_cgroup_template` | cgroup 경로에 대한 템플릿입니다. `${slot}`을 자리 표시자로 사용합니다. 형식은 Docker의 cgroup 드라이버에 따라 달라집니다(systemd: `runner-slot-${slot}.slice`, cgroupfs: `gitlab-runner/slot-${slot}`) | `"gitlab-runner/slot-${slot}"` |

템플릿은 슬롯 번호에 대한 자리 표시자로 `${slot}`을 사용하는 bash 스타일 변수 확장을 사용합니다. 예를 들어:

- `systemd` 드라이버 사용: `runner-slot-${slot}.slice`는 슬롯 5에 대해 `runner-slot-5.slice`이 됩니다.
- `cgroupfs` 드라이버 사용: `gitlab-runner/slot-${slot}`는 슬롯 5에 대해 `gitlab-runner/slot-5`이 됩니다.

Docker cgroup 드라이버를 다음을 사용하여 확인합니다: `docker info | grep "Cgroup Driver"`

### Docker 특정 구성 {#docker-specific-configuration}

Docker Autoscaler 실행기를 사용할 때 서비스 컨테이너에 대한 별도의 템플릿을 지정할 수 있습니다:

```toml
[[runners]]
  executor = "docker-autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "runner-slot-${slot}.slice"

  [runners.docker]
    service_slot_cgroup_template = "runner-slot-${slot}.slice"
```

| 설정 | 설명 | 기본값 |
|---------|-------------|---------|
| `service_slot_cgroup_template` | 서비스 컨테이너 cgroup 경로에 대한 템플릿입니다. Docker의 cgroup 드라이버 형식과 일치해야 합니다. | `slot_cgroup_template`과 같음 |

## 환경 설정 {#environment-setup}

슬롯 기반 cgroup을 활성화하기 전에 러너 호스트의 cgroup 계층 구조를 준비합니다.

### systemd cgroup 드라이버에 대한 설정 스크립트 {#setup-script-for-systemd-cgroup-driver}

Docker가 `systemd` cgroup 드라이버를 사용하는 경우(`docker info | grep "Cgroup Driver"`로 확인), 원시 cgroup 디렉토리 대신 `systemd` 슬라이스를 만들어야 합니다.

설정 스크립트 만들기(`gitlab-runner-systemd-slice-setup.sh`):

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

설정 스크립트 실행:

```shell
chmod +x gitlab-runner-systemd-slice-setup.sh
sudo ./gitlab-runner-systemd-slice-setup.sh
```

### `cgroupfs` 드라이버에 대한 설정 스크립트(대안) {#setup-script-for-cgroupfs-driver-alternative}

Docker가 `systemd` 대신 `cgroupfs` 드라이버를 사용하는 경우, 원시 cgroup 디렉토리를 만드는 이 대체 스크립트를 사용합니다:

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

설정 스크립트 실행:

```shell
chmod +x gitlab-runner-cgroup-setup.sh
sudo ./gitlab-runner-cgroup-setup.sh
```

## 작동 방식 {#how-it-works}

### Docker Autoscaler 실행기 {#docker-autoscaler-executor}

Docker Autoscaler 실행기는 `--cgroup-parent` 플래그를 사용하여 Docker 컨테이너에 슬롯 기반 cgroup 경로를 자동으로 적용합니다. 빌드 컨테이너와 서비스 컨테이너 모두 작업 스크립트를 변경하지 않고도 슬롯별 cgroup에 할당됩니다.

### 인스턴스 실행기 {#instance-executor}

인스턴스 실행기는 `GITLAB_RUNNER_SLOT_CGROUP` 환경 변수를 작업에 제공합니다. 작업 스크립트에서 이 변수를 사용하여 슬롯별 cgroup 아래에서 프로세스를 실행할 수 있습니다.

#### `systemd-run` 사용 {#using-systemd-run}

```yaml
job:
  script:
    - echo "Running in cgroup $GITLAB_RUNNER_SLOT_CGROUP"
    - systemd-run --scope --slice=$GITLAB_RUNNER_SLOT_CGROUP ./my-process
```

#### `cgexec` 사용 {#using-cgexec}

```yaml
job:
  script:
    - echo "Running in cgroup $GITLAB_RUNNER_SLOT_CGROUP"
    - cgexec -g cpu,memory:$GITLAB_RUNNER_SLOT_CGROUP ./my-process
```

#### cgroup 제한 설정 {#setting-cgroup-limits}

작업 프로세스를 실행하기 전에 cgroup에 대한 리소스 제한을 설정할 수 있습니다:

```yaml
job:
  script:
    - echo "Configuring cgroup limits"
    - echo "100M" > /sys/fs/cgroup/$GITLAB_RUNNER_SLOT_CGROUP/memory.max
    - echo "50000" > /sys/fs/cgroup/$GITLAB_RUNNER_SLOT_CGROUP/cpu.max
    - ./my-process
```

## 문제 해결 {#troubleshooting}

### 컨테이너가 cgroup 오류로 인해 시작되지 않음 {#containers-fail-to-start-with-cgroup-errors}

1. cgroup 경로가 `/sys/fs/cgroup/` 아래에 존재하는지 확인합니다:

   ```shell
   ls -la /sys/fs/cgroup/gitlab-runner/
   ```

1. 러너 사용자가 cgroup 디렉토리에 대한 쓰기 액세스 권한이 있는지 확인합니다:

   ```shell
   ls -la /sys/fs/cgroup/gitlab-runner/slot-0/
   ```

1. `slot_cgroup_template`이 `${slot}` 자리 표시자와 함께 올바른 형식을 사용하는지 확인합니다:
1. 특정 cgroup 생성 오류에 대해 러너 로그를 확인합니다:
1. 수동으로 테스트합니다:

   Docker Autoscaler 실행기:

   ```shell
   docker run --rm --cgroup-parent=gitlab-runner/slot-0 alpine echo "test"
   ```

   인스턴스 실행기:

   ```yaml
   job:
     script:
       - echo "Slot cgroup: $GITLAB_RUNNER_SLOT_CGROUP"
   ```

### 작업이 동일한 cgroup을 사용합니다 {#jobs-use-the-same-cgroup}

로그에서 `${slot}` 자리 표시자를 포함하지 않는 템플릿에 대한 경고를 본 경우:

```plaintext
level=warning msg="Slot cgroup template does not contain ${slot} placeholder.
All jobs will use the same cgroup, defeating the purpose of slot-based isolation."
```

이는 `slot_cgroup_template`이 `${slot}` 변수를 누락했다는 의미입니다. 자리 표시자를 포함하도록 구성을 업데이트합니다:

```toml
[[runners]]
  slot_cgroup_template = "gitlab-runner/slot-${slot}"
```

### Cgroup v2를 사용할 수 없음 {#cgroup-v2-not-available}

설정 스크립트가 cgroup v2를 감지하지 못했으면 시스템에서 이를 활성화해야 할 수 있습니다. cgroup v2를 활성화하는 방법에 대해 Linux 배포판의 설명서를 확인합니다. 최신 배포판은 기본적으로 이를 활성화합니다.
