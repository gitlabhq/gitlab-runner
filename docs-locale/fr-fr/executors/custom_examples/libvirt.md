---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "Utilisation de libvirt avec l'exécuteur personnalisé"
---

{{< details >}}

- Niveau :  Gratuite, GitLab Premium, GitLab Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

En utilisant [libvirt](https://libvirt.org/), le pilote de l'exécuteur personnalisé créera un nouveau disque et une nouvelle VM pour chaque job qu'il exécute, après quoi le disque et la VM seront supprimés.

Ce document n'a pas pour but d'expliquer comment configurer libvirt, car cela est hors de sa portée. Cependant, ce pilote a été testé à l'aide de [GCP Nested Virtualization](https://docs.cloud.google.com/compute/docs/instances/nested-virtualization/overview), qui contient également des [informations sur la configuration de libvirt](https://docs.cloud.google.com/compute/docs/instances/nested-virtualization/overview#starting_a_private_bridge_between_the_host_and_nested_vms) avec la mise en réseau par pont. Cet exemple utilisera le réseau `default` fourni lors de l'installation de libvirt, veillez donc à ce qu'il soit en cours d'exécution.

Ce pilote nécessite une mise en réseau par pont, car chaque VM doit disposer de sa propre adresse IP dédiée pour que GitLab Runner puisse se connecter en SSH à l'intérieur et exécuter des commandes. Une clé SSH peut être générée [à l'aide des commandes suivantes](https://docs.gitlab.com/user/ssh/#generate-an-ssh-key-pair).

## Créer l'image de base {#build-the-base-image}

Une image de disque VM de base est créée afin que les dépendances ne soient pas téléchargées à chaque build. Créez-la pour la famille de systèmes d'exploitation invités que vous utilisez.

### Debian et Ubuntu (`virt-builder`) {#debian-and-ubuntu-virt-builder}

[`virt-builder`](https://libguestfs.org/virt-builder.1.html) crée l'image de base directement à partir d'un modèle :

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

La commande précédente installe tous les [prérequis](../custom.md#prerequisite-software-for-running-a-job) spécifiés précédemment.

`virt-builder` définit automatiquement un mot de passe root et l'affiche à la fin. Pour définir le vôtre, passez [`--root-password password:$SOME_PASSWORD`](https://libguestfs.org/virt-builder.1.html#setting-the-root-password).

### RHEL, CentOS et AlmaLinux (`virt-customize`) {#rhel-centos-and-almalinux-virt-customize}

`virt-builder` ne fournit aucun modèle invité RHEL sous licence. Téléchargez le `qcow2` GenericCloud de la distribution et personnalisez-le hors ligne avec [`virt-customize`](https://libguestfs.org/virt-customize.1.html). Cet exemple utilise l'image AlmaLinux 9 `x86_64` ; remplacez-la par l'image RHEL ou CentOS Stream 9, ou par une architecture différente, selon vos besoins.

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

Spécificités de la famille RHEL :

- Utilisez les scripts de dépôt de paquets `.rpm.sh` et `dnf`. Les outils fournissant `virt-customize` et `virt-install` se trouvent dans le paquet `guestfs-tools`.
- Installez dans l'image de base l'environnement d'exécution dont vos jobs ont besoin. Cet exemple installe `gitlab-runner`, `git`, `git-lfs` et `openssh-server`. Ajoutez un moteur de conteneurs tel que `podman` si des jobs créent des images à l'intérieur de la VM.
- Passez `--selinux-relabel` pour que l'invité démarre proprement avec SELinux en mode enforcing, et conservez les images sous `/var/lib/libvirt/images/` afin qu'elles portent le label SELinux `virt_image_t`.
- Contrairement à la recette Debian, l'image GenericCloud n'a pas besoin de `net.ifnames` ni de `/etc/network/interfaces`. Elle démarre avec `cloud-init` et `NetworkManager`. Si vous modifiez la ligne de commande du noyau, régénérez GRUB avec `grub2-mkconfig`.
- Démarrez un démon libvirt et confirmez la virtualisation imbriquée avec `virt-host-validate`. libvirt 9 et les versions ultérieures fournissent les démons modulaires (`virtqemud` et ses compagnons). L'unité de compatibilité monolithique `libvirtd` fonctionne également et peut déjà être activée via socket. Activez celle que votre installation fournit et confirmez qu'elle est active.
- Les scripts de l'exécuteur personnalisé doivent communiquer avec l'instance libvirt système, où ces VM résident. Le script [base](#base) définit `export LIBVIRT_DEFAULT_URI="qemu:///system"` pour cette connexion.
- Dans le script [prepare](#prepare), définissez `--os-variant` sur un identifiant reconnu par `osinfo-db`. Cet exemple utilise `rhel9.0`. `almalinux9` ou `centos-stream9` fonctionnent également si `osinfo-db` les inclut. Listez les identifiants disponibles avec `osinfo-query os`.

## Configuration {#configuration}

Voici un exemple de configuration de GitLab Runner pour libvirt :

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

Chaque étape ([prepare](#prepare) , [run](#run) et [cleanup](#cleanup)) utilisera le script de base ci-dessous pour générer des variables utilisées dans les autres scripts.

Il est important que ce script se trouve dans le même répertoire que les autres scripts, dans ce cas `/opt/libvirt-driver/`.

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

## Préparation {#prepare}

Le script de préparation :

- Copie le disque vers un nouveau chemin.
- Installe une nouvelle VM à partir du disque copié.
- Attend que la VM obtienne une adresse IP.
- Attend que SSH réponde sur la VM.

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

## Exécution {#run}

Cela exécutera le script généré par GitLab Runner en envoyant le contenu du script à la VM via `STDIN` par SSH.

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

## Nettoyage {#cleanup}

Ce script supprime la VM et efface le disque.

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
