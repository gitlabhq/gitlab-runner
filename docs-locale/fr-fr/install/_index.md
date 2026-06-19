---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Logiciel pour les jobs CI/CD.
title: Installer GitLab Runner
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

[GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner) exécute les jobs CI/CD définis dans GitLab. GitLab Runner peut s'exécuter en tant que binaire unique et n'a aucune exigence spécifique à un langage.

Pour des raisons de sécurité et de performance, installez GitLab Runner sur une machine distincte de celle qui héberge votre instance GitLab.

Avant d'effectuer l'installation, consultez les [exigences système et les plateformes prises en charge](requirements.md).

## Systèmes d'exploitation {#operating-systems}

{{< cards >}}

- [Linux](linux-repository.md)
- [Installation manuelle sous Linux](linux-manually.md)
- [FreeBSD](freebsd.md)
- [macOS](osx.md)
- [Windows](windows.md)
- [z/OS](z-os.md)

{{< /cards >}}

## Conteneurs {#containers}

{{< cards >}}

- [Docker](docker.md)
- [Chart Helm](kubernetes.md)
- [Agent GitLab](kubernetes-agent.md)
- [Operator](operator.md)

{{< /cards >}}

## Autres options d'installation {#other-installation-options}

{{< cards >}}

- [Versions bleeding edge](bleeding-edge.md)

{{< /cards >}}
