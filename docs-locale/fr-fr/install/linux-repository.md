---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: "Installez GitLab Runner depuis un dépôt GitLab à l'aide de votre gestionnaire de paquets."
title: "Installer GitLab Runner à l'aide des dépôts officiels GitLab"
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Pour installer GitLab Runner, vous pouvez utiliser un paquet provenant du [dépôt GitLab](https://packages.gitlab.com/runner/gitlab-runner).

## Distributions prises en charge {#supported-distributions}

GitLab fournit des paquets pour les versions suivantes des distributions Linux. Les nouveaux paquets runner `deb` ou `rpm` pour les nouvelles versions de distributions de systèmes d'exploitation sont ajoutés automatiquement lorsqu'ils sont pris en charge par notre système d'hébergement de paquets.

<!-- supported_os_versions_list_start -->

### Distributions basées sur Deb {#deb-based-distributions}

| Distribution | Versions prises en charge |
|--------------|--------------------|
| Debian | Duke, Forky, Trixie, Bookworm, Bullseye |
| LinuxMint | Xia, Wilma, Virginia, Victoria, Vera, Vanessa |
| Raspbian | Duke, Forky, Trixie, Bookworm, Bullseye |
| Ubuntu | Questing, Noble, Jammy, Focal, Bionic |

### Distributions basées sur Rpm {#rpm-based-distributions}

| Distribution | Versions prises en charge |
|--------------|--------------------|
| Amazon Linux | 2025, 2023, 2 |
| Red Hat Enterprise Linux | 10, 9, 8, 7 |
| Fedora | 43, 42 |
| Oracle Linux | 10, 9, 8, 7 |
| openSUSE | 16.0, 15.6 |
| SUSE Linux Enterprise Server | 15.7, 15.6, 15.5, 15.4, 12.5 |

<!-- supported_os_versions_list_end -->

Selon votre configuration, d'autres distributions basées sur Debian ou RPM peuvent également être prises en charge. Cela concerne les distributions dérivées d'une distribution GitLab Runner prise en charge et disposant de dépôts de paquets compatibles. Par exemple, Deepin est un dérivé de Debian. Ainsi, le paquet `deb` du runner devrait s'installer et s'exécuter sur Deepin. Vous pourrez peut-être aussi [installer GitLab Runner en tant que binaire](linux-manually.md#using-binary-file) sur d'autres distributions Linux.

> [!note]
> Les paquets pour les distributions qui ne figurent pas dans la liste ne sont pas disponibles dans notre dépôt de paquets. Vous pouvez les [installer](linux-manually.md#using-debrpm-package) manuellement en téléchargeant le paquet RPM ou DEB depuis notre compartiment S3.

## Installer GitLab Runner {#install-gitlab-runner}

Pour installer GitLab Runner :

1. Ajoutez le dépôt officiel GitLab :

   {{< tabs >}}

   {{< tab title="Debian/Ubuntu/Mint" >}}

   1. Téléchargez le script de configuration du dépôt :

      ```shell
      curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" -o script.deb.sh
      ```

   1. Inspectez le script avant de l'exécuter :

      ```shell
      less script.deb.sh
      ```

   1. Exécutez le script :

      ```shell
      sudo bash script.deb.sh
      ```

   {{< /tab >}}

   {{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

   1. Téléchargez le script de configuration du dépôt :

      ```shell
      curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.rpm.sh" -o script.rpm.sh
      ```

   1. Inspectez le script avant de l'exécuter :

      ```shell
      less script.rpm.sh
      ```

   1. Exécutez le script :

      ```shell
      sudo bash script.rpm.sh
      ```

   {{< /tab >}}

   {{< /tabs >}}

1. Installez la dernière version de GitLab Runner, ou passez à l'étape suivante pour installer une version spécifique :

   > [!note]
   > L'utilisation du répertoire `skel` est désactivée par défaut pour éviter les [échecs de job `No such file or directory`](#error-no-such-file-or-directory-job-failures).

   {{< tabs >}}

   {{< tab title="Debian/Ubuntu/Mint" >}}

   ```shell
   sudo apt install gitlab-runner
   ```

   {{< /tab >}}

   {{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

   ```shell
   sudo yum install gitlab-runner

   ou

   sudo dnf install gitlab-runner
   ```

   {{< /tab >}}

   {{< /tabs >}}

   > [!note]
   > Une version de GitLab Runner conforme à la norme FIPS 140-2 est disponible pour les distributions RHEL. Vous pouvez installer cette version en utilisant `gitlab-runner-fips` comme nom de paquet, à la place de `gitlab-runner`.

1. Pour installer une version spécifique de GitLab Runner :

   {{< tabs >}}

   {{< tab title="Debian/Ubuntu/Mint" >}}

   > [!note]
   > À partir de la version `v17.7.1` de `gitlab-runner`, lorsque vous installez une version spécifique de `gitlab-runner` qui n'est pas la dernière version, vous devez explicitement installer le `gitlab-runner-helper-packages` requis pour cette version. Cette exigence existe en raison d'une limitation de `apt`/`apt-get`.

   ```shell
   apt-cache madison gitlab-runner
   sudo apt install gitlab-runner=17.7.1-1 gitlab-runner-helper-images=17.7.1-1
   ```

   Si vous tentez d'installer une version spécifique de `gitlab-runner` sans installer la même version de `gitlab-runner-helper-images`, vous pourriez rencontrer l'erreur suivante :

   ```shell
   sudo apt install gitlab-runner=17.7.1-1
   ...
   The following packages have unmet dependencies:
    gitlab-runner : Depends: gitlab-runner-helper-images (= 17.7.1-1) but 17.8.3-1 is to be installed
   E: Unable to correct problems, you have held broken packages.
   ```

   {{< /tab >}}

   {{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

   ```shell
   yum list gitlab-runner --showduplicates | sort -r
   sudo yum install gitlab-runner-17.2.0-1
   ```

   {{< /tab >}}

   {{< /tabs >}}

1. [Enregistrez un runner](../register/_index.md).

Une fois les étapes ci-dessus effectuées, un runner peut être démarré et utilisé avec vos projets !

Assurez-vous de lire la section [FAQ](../faq/_index.md) qui décrit certains des problèmes les plus courants avec GitLab Runner.

## Paquet d'images d'aide {#helper-images-package}

Le paquet `gitlab-runner-helper-images` contient des images de conteneurs d'aide pré-construites que GitLab Runner utilise lors de l'exécution des jobs. Ces images fournissent les outils et utilitaires nécessaires pour cloner des dépôts, téléverser des artefacts et gérer les caches.

Le paquet `gitlab-runner-helper-images` inclut des images d'aide pour les systèmes d'exploitation et architectures suivants :

Images basées sur Alpine (dernières) :

- `alpine-arm`
- `alpine-arm64`
- `alpine-riscv64`
- `alpine-s390x`
- `alpine-x86_64`
- `alpine-x86_64-pwsh`

Images basées sur Ubuntu (24.04) :

- `ubuntu-arm`
- `ubuntu-arm64`
- `ubuntu-ppc64le`
- `ubuntu-s390x`
- `ubuntu-x86_64`
- `ubuntu-x86_64-pwsh`

### Téléchargement automatique des images d'aide {#automatic-helper-image-download}

Si une image d'aide pour une combinaison spécifique de système d'exploitation et d'architecture n'est pas disponible sur le système hôte, GitLab Runner télécharge automatiquement l'image requise lorsque nécessaire. L'installation manuelle n'est pas requise pour les architectures qui ne sont pas incluses dans le `gitlab-runner-helper-images package`. Ce téléchargement automatique garantit que le runner peut prendre en charge des architectures supplémentaires (telles que `loong64`) sans nécessiter d'intervention manuelle ni d'installations de paquets séparées.

## Mettre à niveau GitLab Runner {#upgrade-gitlab-runner}

Pour installer la dernière version de GitLab Runner :

{{< tabs >}}

{{< tab title="Debian/Ubuntu/Mint" >}}

```shell
sudo apt update
sudo apt install gitlab-runner
```

{{< /tab >}}

{{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

```shell
sudo yum update
sudo yum install gitlab-runner
```

{{< /tab >}}

{{< /tabs >}}

## Signatures GPG pour l'installation de paquets {#gpg-signatures-for-package-installation}

Le projet GitLab Runner fournit deux types de signatures GPG pour la méthode d'installation de paquets :

- [Signature des métadonnées du dépôt](#repository-metadata-signing)
- [Signature des paquets](#package-signing)

### Signature des métadonnées du dépôt {#repository-metadata-signing}

Pour vérifier que les informations de paquets téléchargées depuis le dépôt distant peuvent être approuvées, le gestionnaire de paquets utilise la signature des métadonnées du dépôt.

La signature est vérifiée lorsque vous utilisez une commande telle que `apt-get update`, de sorte que les informations sur les paquets disponibles sont mises à jour **avant que le moindre paquet ne soit téléchargé et installé**. Un échec de vérification devrait également entraîner le rejet des métadonnées par le gestionnaire de paquets. Cela signifie que vous ne pouvez pas télécharger et installer un paquet depuis le dépôt tant que le problème ayant provoqué la non-concordance de la signature n'a pas été identifié et résolu.

Les clés publiques GPG utilisées pour la vérification de la signature des métadonnées de paquets sont installées automatiquement lors de la première installation effectuée avec les instructions ci-dessus. Pour les mises à jour de clés à venir, les utilisateurs existants doivent télécharger et installer manuellement les nouvelles clés.

Nous utilisons une seule clé pour tous nos projets hébergés sous <https://packages.gitlab.com>. Vous pouvez trouver les détails sur la clé utilisée dans la [documentation du paquet Linux](https://docs.gitlab.com/omnibus/update/package_signatures/#package-repository-metadata-signing-key). Cette page de documentation liste également [toutes les clés utilisées par le passé](https://docs.gitlab.com/omnibus/update/package_signatures/#previous-package-signing-keys).

### Signature des paquets {#package-signing}

La signature des métadonnées du dépôt prouve que les informations de version téléchargées proviennent de <https://packages.gitlab.com>. Elle ne prouve pas l'intégrité des paquets eux-mêmes. Tout ce qui a été téléversé sur <https://packages.gitlab.com>, autorisé ou non, est correctement vérifié tant que le transfert des métadonnées du dépôt vers l'utilisateur n'a pas été affecté.

Avec la signature des paquets, chaque paquet est signé au moment de sa construction. Tant que vous ne pouvez pas faire confiance à l'environnement de compilation ni au secret de la clé GPG utilisée, vous ne pouvez pas vérifier l'authenticité du paquet. Une signature valide sur le paquet prouve que son origine est authentifiée et que son intégrité n'a pas été compromise.

La vérification de la signature des paquets est activée par défaut uniquement dans certaines distributions basées sur Debian/RPM. Pour utiliser ce type de vérification, vous devrez peut-être ajuster la configuration.

Les clés GPG utilisées pour la vérification de la signature des paquets peuvent être différentes pour chacun des dépôts hébergés sur <https://packages.gitlab.com>. Le projet GitLab Runner utilise sa propre paire de clés pour ce type de signature.

#### Distributions basées sur RPM {#rpm-based-distributions-1}

Le format RPM contient une implémentation complète des fonctionnalités de signature GPG, et est ainsi pleinement intégré aux systèmes de gestion de paquets basés sur ce format.

Vous pouvez trouver la description technique de la configuration de la vérification de signature des paquets pour les distributions basées sur RPM dans la [documentation du paquet Linux](https://docs.gitlab.com/omnibus/update/package_signatures/#rpm-based-distributions). Les différences pour GitLab Runner sont :

- Le paquet de clé publique à installer est nommé `gpg-pubkey-35dfa027-60ba0235`.
- Le fichier de dépôt pour les distributions basées sur RPM est nommé `/etc/yum.repos.d/runner_gitlab-runner.repo` (pour la version stable) ou `/etc/yum.repos.d/runner_unstable.repo` (pour les versions instables).
- La [clé publique de signature des paquets](#current-gpg-public-key) peut être importée depuis `https://packages.gitlab.com/gpgkey/runner/49F16C5CC3A0F81F.pub.gpg`.

#### Distributions basées sur Debian {#debian-based-distributions}

Le format `deb` ne contient pas officiellement de méthode par défaut et intégrée pour signer les paquets. Le projet GitLab Runner utilise l'outil `dpkg-sig` pour la signature et la vérification des signatures sur les paquets. Cette méthode ne prend en charge que la vérification manuelle des paquets.

Pour vérifier un paquet `deb` :

1. Installez `dpkg-sig` :

   ```shell
   apt update && apt install dpkg-sig
   ```

1. Téléchargez et importez la [clé publique de signature des paquets](#current-gpg-public-key) :

   ```shell
   curl -JLO "https://packages.gitlab.com/gpgkey/runner/49F16C5CC3A0F81F.pub.gpg"
   gpg --import runner-gitlab-runner-49F16C5CC3A0F81F.pub.gpg
   ```

1. Vérifiez le paquet téléchargé avec `dpkg-sig` :

   ```shell
   dpkg-sig --verify gitlab-runner_amd64.deb
   Processing gitlab-runner_amd64.deb...
   GOODSIG _gpgbuilder 931DA69CFA3AFEBBC97DAA8C6C57C29C6BA75A4E 1623755049
   ```

   Si un paquet a une signature invalide ou est signé avec une clé invalide (par exemple une clé révoquée), la sortie est similaire à ce qui suit :

   ```shell
   dpkg-sig --verify gitlab-runner_amd64.deb
   Processing gitlab-runner_amd64.deb...
   BADSIG _gpgbuilder
   ```

   Si la clé n'est pas présente dans le trousseau de clés de l'utilisateur, la sortie est similaire à :

   ```shell
   dpkg-sig --verify gitlab-runner_amd64.v13.1.0.deb
   Processing gitlab-runner_amd64.v13.1.0.deb...
   UNKNOWNSIG _gpgbuilder 880721D4
   ```

#### Clé publique GPG actuelle {#current-gpg-public-key}

Téléchargez la clé publique GPG actuelle utilisée pour la signature des paquets depuis `https://packages.gitlab.com/runner/gitlab-runner/gpgkey/runner-gitlab-runner-49F16C5CC3A0F81F.pub.gpg`.

| Attribut de clé | Valeur |
|---------------|-------|
| Nom          | `GitLab, Inc.` |
| E-mail         | `support@gitlab.com` |
| Empreinte   | `931D A69C FA3A FEBB C97D  AA8C 6C57 C29C 6BA7 5A4E` |
| Expiration        | `2026-04-28` |

> [!note]
> La même clé est utilisée par le projet GitLab Runner pour signer les fichiers `release.sha256` pour les releases S3 disponibles dans le compartiment `<https://gitlab-runner-downloads.s3.dualstack.us-east-1.amazonaws.com>`.

#### Clés publiques GPG précédentes {#previous-gpg-public-keys}

Les clés utilisées par le passé peuvent être trouvées dans le tableau ci-dessous.

Pour les clés qui ont été révoquées, il est fortement recommandé de les supprimer de la configuration de vérification de signature des paquets.

Les signatures réalisées par les clés suivantes ne doivent plus être approuvées.

| Index N° | Empreinte de clé                                      | Statut    | Date d'expiration  | Téléchargement (clés révoquées uniquement) |
|---------|------------------------------------------------------|-----------|--------------|------------------------------|
| 1       | `3018 3AC2 C4E2 3A40 9EFB  E705 9CE4 5ABC 8807 21D4` | `revoked` | `2021-06-08` | [clé révoquée](https://gitlab.com/gitlab-org/gitlab-runner/-/blob/main/docs/install/gpg-keys/9CE45ABC880721D4.pub.gpg) |
| 2       | `09E5 7083 F34C CA94 D541  BC58 A674 BF81 35DF A027` | `revoked` | `2023-04-26` | [clé révoquée](https://gitlab.com/gitlab-org/gitlab-runner/-/blob/main/docs/install/gpg-keys/A674BF8135DFA027.pub.gpg) |

## Dépannage {#troubleshooting}

Voici quelques conseils pour diagnostiquer et résoudre les problèmes lors de l'installation de GitLab Runner.

### Erreur : échecs de job `No such file or directory` {#error-no-such-file-or-directory-job-failures}

Parfois, le répertoire squelette par défaut (`skel`) cause des problèmes à GitLab Runner, et celui-ci échoue à exécuter un job. Consultez le [ticket 4449](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4449) et le [ticket 1379](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1379).

Pour éviter cela, lors de l'installation de GitLab Runner, un utilisateur `gitlab-runner` est créé et, par défaut, le répertoire home est créé sans aucun squelette. La configuration shell ajoutée au répertoire home avec l'utilisation de `skel` peut interférer avec l'exécution des jobs. Cette configuration peut introduire des problèmes inattendus comme ceux mentionnés ci-dessus.

Si vous avez créé le runner avant que l'évitement de `skel` ne devienne le comportement par défaut, vous pouvez essayer de supprimer les fichiers dotfiles suivants :

```shell
sudo rm /home/gitlab-runner/.profile
sudo rm /home/gitlab-runner/.bashrc
sudo rm /home/gitlab-runner/.bash_logout
```

Si vous devez utiliser le répertoire `skel` pour remplir le répertoire `$HOME` nouvellement créé, vous devez définir explicitement la variable `GITLAB_RUNNER_DISABLE_SKEL` à `false` avant d'installer le runner :

{{< tabs >}}

{{< tab title="Debian/Ubuntu/Mint" >}}

```shell
export GITLAB_RUNNER_DISABLE_SKEL=false; sudo -E apt-get install gitlab-runner
```

{{< /tab >}}

{{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

```shell
export GITLAB_RUNNER_DISABLE_SKEL=false; sudo -E yum install gitlab-runner
```

{{< /tab >}}

{{< /tabs >}}
