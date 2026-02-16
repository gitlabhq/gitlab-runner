---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Slot-based cgroup support
---

Slot-based cgroup support improves resource isolation and management when you use GitLab Runner with autoscaling.
Slot-based cgroups automatically assign jobs to specific control groups (cgroups) based on the slot number allocated by the autoscaler.

## Benefits

- Better resource isolation: Prevents resource interference between concurrent jobs on the same instance.
- Easier monitoring: Per-slot resource usage can be tracked independently.
- Improved debugging: Cgroup-based metrics help identify resource-hungry jobs.
- Fine-grained control: Set resource limits per slot for predictable performance.

## Supported executors

Slot-based cgroups work with autoscaling executors that use [taskscaler](https://gitlab.com/gitlab-org/fleeting/taskscaler) for slot management:

- [Docker Autoscaler executor](../executors/docker_autoscaler.md#slot-based-cgroup-support)
- [Instance executor](../executors/instance.md#slot-based-cgroup-support)

## Prerequisites

- Linux host with cgroup v2 support
- Root access for initial cgroup hierarchy setup
- GitLab Runner with autoscaler functionality
- Taskscaler for slot assignment (automatically provided by autoscaler)

## Configuration

To enable slot-based cgroup support, add the following to your `config.toml`.

### For Docker with `systemd` cgroup driver

If Docker is using the `systemd` cgroup driver (most common), use the `systemd` slice format:

```toml
[[runners]]
  name = "my-autoscaler-runner"
  executor = "docker-autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "runner-slot-${slot}.slice"

  [runners.autoscaler]
    capacity_per_instance = 4
```

### For Docker with `cgroupfs` driver

If Docker is using the `cgroupfs` driver, use the raw `cgroup` path format:

```toml
[[runners]]
  name = "my-autoscaler-runner"
  executor = "docker-autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "gitlab-runner/slot-${slot}"

  [runners.autoscaler]
    capacity_per_instance = 4
```

### Configuration options

| Setting | Description | Default |
|---------|-------------|---------|
| `use_slot_cgroups` | Enable slot-based cgroup assignment | `false` |
| `slot_cgroup_template` | Template for cgroup paths. Use `${slot}` as placeholder. Format depends on Docker's cgroup driver (systemd: `runner-slot-${slot}.slice`, cgroupfs: `gitlab-runner/slot-${slot}`) | `"gitlab-runner/slot-${slot}"` |

Templates use bash-style variable expansion with `${slot}` as the placeholder for the slot number. For example:

- With `systemd` driver: `runner-slot-${slot}.slice` becomes `runner-slot-5.slice` for slot 5
- With `cgroupfs` driver: `gitlab-runner/slot-${slot}` becomes `gitlab-runner/slot-5` for slot 5

Check your Docker cgroup driver with: `docker info | grep "Cgroup Driver"`

### Docker-specific configuration

When using the Docker Autoscaler executor, you can specify a separate template for service containers:

```toml
[[runners]]
  executor = "docker-autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "runner-slot-${slot}.slice"

  [runners.docker]
    service_slot_cgroup_template = "runner-slot-${slot}.slice"
```

| Setting | Description | Default |
|---------|-------------|---------|
| `service_slot_cgroup_template` | Template for service container cgroup paths. Must match Docker's cgroup driver format | Same as `slot_cgroup_template` |

## Environment setup

Before enabling slot-based cgroups, prepare the cgroup hierarchy on your runner hosts.

### Setup script for systemd cgroup driver

If Docker is using the `systemd` cgroup driver (check with `docker info | grep "Cgroup Driver"`), you must create `systemd` slices instead of raw cgroup directories.

Create a setup script (`gitlab-runner-systemd-slice-setup.sh`):

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

Run the setup script:

```shell
chmod +x gitlab-runner-systemd-slice-setup.sh
sudo ./gitlab-runner-systemd-slice-setup.sh
```

### Setup script for `cgroupfs` driver (alternative)

If Docker is using the `cgroupfs` driver instead of `systemd`, use this alternative script that creates raw cgroup directories:

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

Run the setup script:

```shell
chmod +x gitlab-runner-cgroup-setup.sh
sudo ./gitlab-runner-cgroup-setup.sh
```

## How it works

### Docker Autoscaler executor

The Docker Autoscaler executor automatically applies slot-based cgroup paths to Docker containers using the `--cgroup-parent` flag. Both build containers and service containers are assigned to their slot-specific cgroups without requiring any changes to your job scripts.

### Instance executor

The Instance executor provides the `GITLAB_RUNNER_SLOT_CGROUP` environment variable to jobs. You can use this variable in your job scripts to run processes under the slot-specific cgroup.

#### Using `systemd-run`

```yaml
job:
  script:
    - echo "Running in cgroup $GITLAB_RUNNER_SLOT_CGROUP"
    - systemd-run --scope --slice=$GITLAB_RUNNER_SLOT_CGROUP ./my-process
```

#### Using `cgexec`

```yaml
job:
  script:
    - echo "Running in cgroup $GITLAB_RUNNER_SLOT_CGROUP"
    - cgexec -g cpu,memory:$GITLAB_RUNNER_SLOT_CGROUP ./my-process
```

#### Setting cgroup limits

You can set resource limits on the cgroup before running your job processes:

```yaml
job:
  script:
    - echo "Configuring cgroup limits"
    - echo "100M" > /sys/fs/cgroup/$GITLAB_RUNNER_SLOT_CGROUP/memory.max
    - echo "50000" > /sys/fs/cgroup/$GITLAB_RUNNER_SLOT_CGROUP/cpu.max
    - ./my-process
```

## Troubleshooting

### Containers fail to start with cgroup errors

1. Check that the cgroup paths exist under `/sys/fs/cgroup/`:

   ```shell
   ls -la /sys/fs/cgroup/gitlab-runner/
   ```

1. Ensure the GitLab Runner user has write access to the cgroup directories:

   ```shell
   ls -la /sys/fs/cgroup/gitlab-runner/slot-0/
   ```

1. Confirm `slot_cgroup_template` uses the correct format with `${slot}` placeholder:

1. Check GitLab Runner logs for specific cgroup creation errors:

1. Test manually:

   For Docker Autoscaler executor:

   ```shell
   docker run --rm --cgroup-parent=gitlab-runner/slot-0 alpine echo "test"
   ```

   For Instance executor:

   ```yaml
   job:
     script:
       - echo "Slot cgroup: $GITLAB_RUNNER_SLOT_CGROUP"
   ```

### Jobs use the same cgroup

If you see a warning in the logs about templates not containing `${slot}` placeholder:

```plaintext
level=warning msg="Slot cgroup template does not contain ${slot} placeholder.
All jobs will use the same cgroup, defeating the purpose of slot-based isolation."
```

This means your `slot_cgroup_template` is missing the `${slot}` variable. Update your configuration to include the placeholder:

```toml
[[runners]]
  slot_cgroup_template = "gitlab-runner/slot-${slot}"
```

### Cgroup v2 not available

If the setup script reports that cgroup v2 is not detected, you might need to enable it on your system.
Check your Linux distribution's documentation for enabling cgroup v2. Modern distributions typically enable it by default.
