---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Logiciel pour les jobs CI/CD.
title: Configuration système requise et plateformes prises en charge
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

## Systèmes d'exploitation pris en charge {#supported-operating-systems}

Vous pouvez installer GitLab Runner sur :

- Linux depuis un [dépôt GitLab](linux-repository.md) ou [manuellement](linux-manually.md)
- [FreeBSD](freebsd.md)
- [macOS](osx.md)
- [Windows](windows.md)
- [z/OS](z-os.md)

[Les binaires de pointe](bleeding-edge.md) sont également disponibles.

Pour utiliser un autre système d'exploitation, assurez-vous que le système d'exploitation peut compiler un binaire Go.

## Conteneurs pris en charge {#supported-containers}

Vous pouvez installer GitLab Runner avec :

- [Docker](docker.md)
- [Le chart Helm GitLab](kubernetes.md)
- [L'agent GitLab pour Kubernetes](kubernetes-agent.md)
- [L'opérateur GitLab](operator.md)

## Architectures prises en charge {#supported-architectures}

GitLab Runner est disponible pour les architectures suivantes :

- x86
- AMD64
- ARM64
- ARM
- s390x
- ppc64le
- riscv64
- loong64

## Configuration système requise {#system-requirements}

La configuration système requise pour GitLab Runner dépend des considérations suivantes :

- Charge CPU anticipée des jobs CI/CD
- Utilisation mémoire anticipée des jobs CI/CD
- Nombre de jobs CI/CD simultanés
- Nombre de projets en développement actif
- Nombre de développeurs censés travailler en parallèle

Pour plus d'informations sur les types de machines disponibles pour GitLab.com, consultez [les runners hébergés par GitLab](https://docs.gitlab.com/ci/runners/).

## GitLab Runner conforme à la norme FIPS {#fips-compliant-gitlab-runner}

Un binaire GitLab Runner conforme à la norme FIPS 140-2 est disponible pour les distributions Red Hat Enterprise Linux (RHEL) et l'architecture AMD64. La prise en charge d'autres distributions et architectures est proposée dans le [ticket 28814](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28814).

Ce binaire est compilé avec le [compilateur Go de Red Hat](https://developers.redhat.com/blog/2019/06/24/go-and-fips-140-2-on-red-hat-enterprise-linux) et fait appel à une bibliothèque cryptographique validée FIPS 140-2. Une [image minimale UBI-8](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/8/html-single/building_running_and_managing_containers/index#con_understanding-the-ubi-minimal-images_assembly_types-of-container-images) est utilisée comme base pour créer l'image FIPS de GitLab Runner.

Pour plus d'informations sur l'utilisation de GitLab Runner conforme à la norme FIPS dans RHEL, consultez [Switching RHEL to FIPS mode](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/8/html/security_hardening/switching-rhel-to-fips-mode_security-hardening).
