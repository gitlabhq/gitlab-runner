---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Installez les dernières versions de développement de GitLab Runner.
title: Versions bleeding edge de GitLab Runner
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!warning]
> Ces versions de GitLab Runner sont les plus récentes et sont compilées directement depuis la branche `main` ; elles peuvent ne pas avoir été testées. Utilisez à vos propres risques.

## Télécharger les binaires autonomes {#download-the-standalone-binaries}

- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-386>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-amd64>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-arm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-s390x>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-riscv64>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-loong64>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-darwin-amd64>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-windows-386.exe>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-windows-amd64.exe>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-windows-arm64.exe>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-freebsd-386>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-freebsd-amd64>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-freebsd-arm>

Vous pouvez ensuite exécuter GitLab Runner avec :

```shell
chmod +x gitlab-runner-linux-amd64
./gitlab-runner-linux-amd64 run
```

## Télécharger l'un des packages pour Debian ou Ubuntu {#download-one-of-the-packages-for-debian-or-ubuntu}

- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_i686.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_amd64.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_armel.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_armhf.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_arm64.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_aarch64.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_riscv64.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_loong64.deb>

### Télécharger le package d'images runner-helper exporté {#download-the-exported-runner-helper-images-package}

Le package d'images runner-helper est une dépendance requise pour le package `.deb` de GitLab Runner.

Téléchargez le package depuis :

```plaintext
https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/deb/gitlab-runner-helper-images.deb
```

Vous pouvez ensuite l'installer avec :

```shell
dpkg -i gitlab-runner-helper-images.deb gitlab-runner_<arch>.deb
```

## Télécharger l'un des packages pour Red Hat ou CentOS {#download-one-of-the-packages-for-red-hat-or-centos}

- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_i686.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_x86_64.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_armhfp.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_aarch64.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_riscv64.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_loongarch64.rpm>

### Télécharger le package d'images runner-helper exporté {#download-the-exported-runner-helper-images-package-1}

Le package d'images runner-helper est une dépendance requise pour le package `.rpm` de GitLab Runner.

Téléchargez le package depuis :

```plaintext
https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner-helper-images.rpm
```

Vous pouvez ensuite l'installer avec :

```shell
rpm -i gitlab-runner-helper-images.rpm gitlab-runner_<arch>.rpm
```

## Télécharger toute autre release taguée {#download-any-other-tagged-release}

Remplacez `main` par `tag` (par exemple, `v16.5.0`) ou `latest` (la dernière version stable). Pour obtenir la liste des tags, consultez <https://gitlab.com/gitlab-org/gitlab-runner/-/tags>. Par exemple :

- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-386>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-386>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/v16.5.0/binaries/gitlab-runner-linux-386>

Si vous rencontrez des problèmes de téléchargement via `https`, utilisez le protocole `http` simple à la place :

- <http://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-386>
- <http://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-386>
- <http://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/v16.5.0/binaries/gitlab-runner-linux-386>
