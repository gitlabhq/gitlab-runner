---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Prise en charge des cgroups basés sur les slots
---

La prise en charge des cgroups basés sur les slots améliore l'isolation et la gestion des ressources lorsque vous utilisez GitLab Runner avec la mise à l'échelle automatique. Les cgroups basés sur les slots assignent automatiquement les jobs à des groupes de contrôle (cgroups) spécifiques en fonction du numéro de slot alloué par l'autoscaler.

## Avantages {#benefits}

- Meilleure isolation des ressources : Empêche les interférences de ressources entre les jobs simultanés sur la même instance.
- Surveillance simplifiée : L'utilisation des ressources par slot peut être suivie indépendamment.
- Débogage amélioré : Les métriques basées sur les cgroups aident à identifier les jobs gourmands en ressources.
- Contrôle précis : Définissez des limites de ressources par slot pour des performances prévisibles.

## Exécuteurs pris en charge {#supported-executors}

Les cgroups basés sur les slots fonctionnent avec les exécuteurs de mise à l'échelle automatique qui utilisent [taskscaler](https://gitlab.com/gitlab-org/fleeting/taskscaler) pour la gestion des slots :

- [Exécuteur Docker Autoscaler](../executors/docker_autoscaler.md#slot-based-cgroup-support)
- [Exécuteur d'instance](../executors/instance.md#slot-based-cgroup-support)

## Prérequis {#prerequisites}

- Hôte Linux avec prise en charge de cgroup v2
- Accès root pour la configuration initiale de la hiérarchie des cgroups
- GitLab Runner avec la fonctionnalité d'autoscaler
- Taskscaler pour l'assignation des slots (fourni automatiquement par l'autoscaler)

## Configuration {#configuration}

Pour activer la prise en charge des cgroups basés sur les slots, ajoutez ce qui suit à votre `config.toml`.

### Pour Docker avec le pilote cgroup `systemd` {#for-docker-with-systemd-cgroup-driver}

Si Docker utilise le pilote cgroup `systemd` (le plus courant), utilisez le format de slice `systemd` :

```toml
[[runners]]
  name = "my-autoscaler-runner"
  executor = "docker-autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "runner-slot-${slot}.slice"

  [runners.autoscaler]
    capacity_per_instance = 4
```

### Pour Docker avec le pilote `cgroupfs` {#for-docker-with-cgroupfs-driver}

Si Docker utilise le pilote `cgroupfs`, utilisez le format de chemin `cgroup` brut :

```toml
[[runners]]
  name = "my-autoscaler-runner"
  executor = "docker-autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "gitlab-runner/slot-${slot}"

  [runners.autoscaler]
    capacity_per_instance = 4
```

### Options de configuration {#configuration-options}

| Paramètre | Description | Valeur par défaut |
|---------|-------------|---------|
| `use_slot_cgroups` | Activer l'assignation de cgroup basée sur les slots | `false` |
| `slot_cgroup_template` | Modèle pour les chemins de cgroup. Utilisez `${slot}` comme espace réservé. Le format dépend du pilote cgroup de Docker (systemd : `runner-slot-${slot}.slice`, cgroupfs : `gitlab-runner/slot-${slot}`) | `"gitlab-runner/slot-${slot}"` |

Les modèles utilisent l'expansion de variables de style bash avec `${slot}` comme espace réservé pour le numéro de slot. Par exemple :

- Avec le pilote `systemd` : `runner-slot-${slot}.slice` devient `runner-slot-5.slice` pour le slot 5
- Avec le pilote `cgroupfs` : `gitlab-runner/slot-${slot}` devient `gitlab-runner/slot-5` pour le slot 5

Vérifiez votre pilote cgroup Docker avec : `docker info | grep "Cgroup Driver"`

### Configuration spécifique à Docker {#docker-specific-configuration}

Lorsque vous utilisez l'exécuteur Docker Autoscaler, vous pouvez spécifier un modèle distinct pour les conteneurs de services :

```toml
[[runners]]
  executor = "docker-autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "runner-slot-${slot}.slice"

  [runners.docker]
    service_slot_cgroup_template = "runner-slot-${slot}.slice"
```

| Paramètre | Description | Valeur par défaut |
|---------|-------------|---------|
| `service_slot_cgroup_template` | Modèle pour les chemins de cgroup des conteneurs de services. Doit correspondre au format du pilote cgroup de Docker | Identique à `slot_cgroup_template` |

## Configuration de l'environnement {#environment-setup}

Avant d'activer les cgroups basés sur les slots, préparez la hiérarchie des cgroups sur vos hôtes runner.

### Script de configuration pour le pilote cgroup systemd {#setup-script-for-systemd-cgroup-driver}

Si Docker utilise le pilote cgroup `systemd` (vérifiez avec `docker info | grep "Cgroup Driver"`), vous devez créer des slices `systemd` au lieu de répertoires cgroup bruts.

Créez un script de configuration (`gitlab-runner-systemd-slice-setup.sh`) :

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

Exécutez le script de configuration :

```shell
chmod +x gitlab-runner-systemd-slice-setup.sh
sudo ./gitlab-runner-systemd-slice-setup.sh
```

### Script de configuration pour le pilote `cgroupfs` (alternative) {#setup-script-for-cgroupfs-driver-alternative}

Si Docker utilise le pilote `cgroupfs` au lieu de `systemd`, utilisez ce script alternatif qui crée des répertoires cgroup bruts :

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

Exécutez le script de configuration :

```shell
chmod +x gitlab-runner-cgroup-setup.sh
sudo ./gitlab-runner-cgroup-setup.sh
```

## Fonctionnement {#how-it-works}

### Exécuteur Docker Autoscaler {#docker-autoscaler-executor}

L'exécuteur Docker Autoscaler applique automatiquement les chemins de cgroup basés sur les slots aux conteneurs Docker à l'aide du flag `--cgroup-parent`. Les conteneurs de build et les conteneurs de services sont tous deux assignés à leurs cgroups spécifiques au slot sans nécessiter de modifications de vos scripts de job.

### Exécuteur d'instance {#instance-executor}

L'exécuteur d'instance fournit la variable d'environnement `GITLAB_RUNNER_SLOT_CGROUP` aux jobs. Vous pouvez utiliser cette variable dans vos scripts de job pour exécuter des processus sous le cgroup spécifique au slot.

#### Utilisation de `systemd-run` {#using-systemd-run}

```yaml
job:
  script:
    - echo "Running in cgroup $GITLAB_RUNNER_SLOT_CGROUP"
    - systemd-run --scope --slice=$GITLAB_RUNNER_SLOT_CGROUP ./my-process
```

#### Utilisation de `cgexec` {#using-cgexec}

```yaml
job:
  script:
    - echo "Running in cgroup $GITLAB_RUNNER_SLOT_CGROUP"
    - cgexec -g cpu,memory:$GITLAB_RUNNER_SLOT_CGROUP ./my-process
```

#### Définition des limites de cgroup {#setting-cgroup-limits}

Vous pouvez définir des limites de ressources sur le cgroup avant d'exécuter vos processus de job :

```yaml
job:
  script:
    - echo "Configuring cgroup limits"
    - echo "100M" > /sys/fs/cgroup/$GITLAB_RUNNER_SLOT_CGROUP/memory.max
    - echo "50000" > /sys/fs/cgroup/$GITLAB_RUNNER_SLOT_CGROUP/cpu.max
    - ./my-process
```

## Dépannage {#troubleshooting}

### Les conteneurs ne parviennent pas à démarrer avec des erreurs cgroup {#containers-fail-to-start-with-cgroup-errors}

1. Vérifiez que les chemins cgroup existent sous `/sys/fs/cgroup/` :

   ```shell
   ls -la /sys/fs/cgroup/gitlab-runner/
   ```

1. Assurez-vous que l'utilisateur GitLab Runner dispose d'un accès en écriture aux répertoires cgroup :

   ```shell
   ls -la /sys/fs/cgroup/gitlab-runner/slot-0/
   ```

1. Confirmez que `slot_cgroup_template` utilise le format correct avec l'espace réservé `${slot}` :
1. Consultez les journaux de GitLab Runner pour les erreurs spécifiques de création de cgroup :
1. Testez manuellement :

   Pour l'exécuteur Docker Autoscaler :

   ```shell
   docker run --rm --cgroup-parent=gitlab-runner/slot-0 alpine echo "test"
   ```

   Pour l'exécuteur d'instance :

   ```yaml
   job:
     script:
       - echo "Slot cgroup: $GITLAB_RUNNER_SLOT_CGROUP"
   ```

### Les jobs utilisent le même cgroup {#jobs-use-the-same-cgroup}

Si vous voyez un avertissement dans les journaux indiquant que les modèles ne contiennent pas l'espace réservé `${slot}` :

```plaintext
level=warning msg="Slot cgroup template does not contain ${slot} placeholder.
All jobs will use the same cgroup, defeating the purpose of slot-based isolation."
```

Cela signifie que votre `slot_cgroup_template` ne contient pas la variable `${slot}`. Mettez à jour votre configuration pour inclure l'espace réservé :

```toml
[[runners]]
  slot_cgroup_template = "gitlab-runner/slot-${slot}"
```

### Cgroup v2 non disponible {#cgroup-v2-not-available}

Si le script de configuration signale que cgroup v2 n'est pas détecté, vous devrez peut-être l'activer sur votre système. Consultez la documentation de votre distribution Linux pour activer cgroup v2. Les distributions modernes l'activent généralement par défaut.
