---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Téléchargez et installez manuellement le binaire GitLab Runner sur Linux.
title: Installer GitLab Runner manuellement sur GNU/Linux
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Vous pouvez installer GitLab Runner manuellement en utilisant un package `deb` ou `rpm` ou un fichier binaire. Utilisez cette approche en dernier recours si :

- Vous ne pouvez pas utiliser le dépôt deb/rpm pour installer GitLab Runner
- Votre système d'exploitation GNU/Linux n'est pas pris en charge

## Prérequis {#prerequisites}

Avant d'exécuter GitLab Runner manuellement :

- Si vous prévoyez d'utiliser l'exécuteur Docker, installez Docker en premier.
- Consultez la section FAQ pour les problèmes courants et leurs solutions.

## Utilisation d'un package deb/rpm {#using-debrpm-package}

Vous pouvez télécharger et installer GitLab Runner en utilisant un package `deb` ou `rpm`.

### Télécharger {#download}

Pour télécharger le package approprié pour votre système :

1. Trouvez le dernier nom de fichier et les options disponibles à l'adresse <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html>.
1. Téléchargez la version runner-helper correspondant à votre gestionnaire de packages ou à votre architecture.
1. Choisissez une version et téléchargez un binaire, comme décrit dans la documentation pour [télécharger n'importe quelle autre release taguée](bleeding-edge.md#download-any-other-tagged-release) pour les releases bleeding edge de GitLab Runner.

Par exemple, pour Debian ou Ubuntu :

```shell
# Replace ${arch} with any of the supported architectures, e.g. amd64, arm, arm64
# A full list of architectures can be found here https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/deb/gitlab-runner-helper-images.deb"
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/deb/gitlab-runner_${arch}.deb"
```

Par exemple, pour CentOS ou Red Hat Enterprise Linux :

```shell
# Replace ${arch} with any of the supported architectures, e.g. x86_64, aarch64, armhfp
# A full list of architectures can be found here https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner-helper-images.rpm"
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner_${arch}.rpm"
```

Par exemple, pour GitLab Runner conforme FIPS sur RHEL :

```shell
# Currently only x86_64 is a supported arch
# The FIPS compliant GitLab Runner version continues to include the helper images in one package.
# A full list of architectures can be found here https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner_x86_64-fips.rpm"
```

### Installer {#install}

1. Installez le package pour votre système comme suit.

   Par exemple, pour Debian ou Ubuntu :

   ```shell
   dpkg -i gitlab-runner-helper-images.deb gitlab-runner_<arch>.deb
   ```

   Par exemple, pour CentOS ou Red Hat Enterprise Linux :

   ```shell
   dnf install -y gitlab-runner-helper-images.rpm gitlab-runner_<arch>.rpm
   ```

### Mettre à niveau {#upgrade}

Téléchargez le dernier package pour votre système, puis effectuez la mise à niveau comme suit :

Par exemple, pour Debian ou Ubuntu :

```shell
dpkg -i gitlab-runner_<arch>.deb
```

Par exemple, pour CentOS ou Red Hat Enterprise Linux :

```shell
dnf install -y gitlab-runner-helper-images.rpm gitlab-runner_<arch>.rpm
```

## Utilisation d'un fichier binaire {#using-binary-file}

Vous pouvez télécharger et installer GitLab Runner en utilisant un fichier binaire.

### Installer {#install-1}

1. Téléchargez l'un des binaires pour votre système :

   ```shell
   # Linux x86-64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64"

   # Linux x86
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-386"

   # Linux arm
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-arm"

   # Linux arm64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-arm64"

   # Linux s390x
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-s390x"

   # Linux ppc64le
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-ppc64le"

   # Linux riscv64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-riscv64"

   # Linux loong64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-loong64"

   # Linux x86-64 FIPS Compliant
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64-fips"
   ```

   Vous pouvez télécharger un binaire pour chaque version disponible comme décrit dans [Bleeding Edge - télécharger n'importe quelle autre release taguée](bleeding-edge.md#download-any-other-tagged-release).

1. Accordez-lui les permissions d'exécution :

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. Créez un utilisateur GitLab CI :

   ```shell
   sudo useradd --comment 'GitLab Runner' --create-home gitlab-runner --shell /bin/bash
   ```

1. Installez et exécutez en tant que service :

   ```shell
   sudo gitlab-runner install --user=gitlab-runner --working-directory=/home/gitlab-runner
   sudo gitlab-runner start
   ```

   Assurez-vous d'avoir `/usr/local/bin/` dans `$PATH` pour root, sinon vous pourriez obtenir une erreur `command not found`. Vous pouvez également installer `gitlab-runner` dans un autre emplacement, comme `/usr/bin/`.

> [!note]
> Si `gitlab-runner` est installé et exécuté en tant que service, il s'exécute en tant que root, mais exécute les jobs en tant qu'utilisateur spécifié par la commande `install`. Cela signifie que certaines fonctions de job comme le cache et les artefacts doivent exécuter la commande `/usr/local/bin/gitlab-runner`. Par conséquent, l'utilisateur sous lequel les jobs sont exécutés doit avoir accès à l'exécutable.

### Mettre à niveau {#upgrade-1}

1. Arrêtez le service (vous avez besoin d'une invite de commande avec élévation de privilèges comme précédemment) :

   ```shell
   sudo gitlab-runner stop
   ```

1. Téléchargez le binaire pour remplacer l'exécutable GitLab Runner. Par exemple :

   ```shell
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64"
   ```

   Vous pouvez télécharger un binaire pour chaque version disponible comme décrit dans [Bleeding Edge - télécharger n'importe quelle autre release taguée](bleeding-edge.md#download-any-other-tagged-release).

1. Accordez-lui les permissions d'exécution :

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. Démarrez le service :

   ```shell
   sudo gitlab-runner start
   ```

## Étapes suivantes {#next-steps}

Après l'installation, [enregistrez un runner](../register/_index.md) pour terminer la configuration.

Le binaire du runner n'inclut pas d'images helper pré-construites. Vous pouvez utiliser ces commandes pour télécharger la version correspondante de l'archive d'images helper et la copier à l'emplacement approprié :

```shell
mkdir -p /usr/local/bin/out/helper-images
cd /usr/local/bin/out/helper-images
```

Choisissez l'image helper appropriée pour votre architecture :

<details>
<summary>Images helper Ubuntu</summary>

```shell
# Linux x86-64 ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-x86_64.tar.xz

# Linux x86-64 ubuntu pwsh
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-x86_64-pwsh.tar.xz

# Linux s390x ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-s390x.tar.xz

# Linux ppc64le ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-ppc64le.tar.xz

# Linux arm64 ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-arm64.tar.xz

# Linux arm ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-arm.tar.xz

# Linux x86-64 ubuntu specific version - v17.10.0
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/v17.10.0/helper-images/prebuilt-ubuntu-x86_64.tar.xz
```

</details>

<details>
<summary>Images helper Alpine</summary>

```shell
# Linux x86-64 alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-x86_64.tar.xz

# Linux x86-64 alpine pwsh
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-x86_64-pwsh.tar.xz

# Linux s390x alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-s390x.tar.xz

# Linux riscv64 alpine edge
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-edge-riscv64.tar.xz

# Linux arm64 alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-arm64.tar.xz

# Linux arm alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-arm.tar.xz
```

</details>

## Ressources supplémentaires {#additional-resources}

- [Documentation de l'exécuteur Docker](../executors/docker.md)
- [Installer Docker](https://docs.docker.com/engine/install/centos/#install-docker-ce)
- [Télécharger d'autres versions de GitLab Runner](bleeding-edge.md#download-any-other-tagged-release)
- [Informations sur GitLab Runner conforme FIPS](requirements.md#fips-compliant-gitlab-runner)
- [FAQ GitLab Runner](../faq/_index.md)
- [Installation via le dépôt deb/rpm](linux-repository.md)
