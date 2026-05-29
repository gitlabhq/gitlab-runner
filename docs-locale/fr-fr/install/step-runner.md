---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Installer le step runner manuellement pour utiliser les fonctions GitLab
title: Installer le step runner manuellement
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Le step runner est un binaire qui permet à GitLab Runner d'exécuter des fonctions GitLab sur des exécuteurs sans prise en charge native des fonctions. Pour ces exécuteurs, vous devez installer le binaire step runner sur l'hôte ou le conteneur où vos jobs s'exécutent avant de pouvoir utiliser les fonctions dans vos pipelines.

## Exécuteurs nécessitant une installation manuelle du step runner {#executors-that-require-manual-step-runner-installation}

La nécessité d'installer step-runner manuellement dépend de votre exécuteur. Le tableau suivant indique quels exécuteurs nécessitent d'installer step runner manuellement :

| Exécuteur          | Installation manuelle requise |
|-------------------|------------------------------|
| Shell             | Oui                          |
| SSH               | Oui                          |
| Kubernetes        | Oui                          |
| VirtualBox        | Oui                          |
| Parallels         | Oui                          |
| Custom            | Oui                          |
| Instance          | Oui                          |
| Docker            | Uniquement sur Windows              |
| Docker Autoscaler | Uniquement sur Windows              |
| Docker Machine    | Uniquement sur Windows              |

Pour les exécuteurs qui ne nécessitent pas d'installation manuelle, `gitlab-runner-helper` fait office de step runner. Le binaire `step-runner` n'est ni présent ni requis sur ces exécuteurs.

### Restrictions d'accès aux variables {#variable-access-restrictions}

Sur les exécuteurs où vous installez step runner manuellement, le step runner dispose d'un accès restreint aux variables de job et aux variables d'environnement :

| Syntaxe               | Valeurs disponibles                                                                                                                                                                        |
|----------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `${{ vars.<name> }}` | Variables de job avec le préfixe `CI_`, `DOCKER_` ou `GITLAB_` uniquement.                                                                                                                      |
| `${{ env.<name> }}`  | `HTTPS_PROXY`, `HTTP_PROXY`, `NO_PROXY`, `http_proxy`, `https_proxy`, `no_proxy`, `all_proxy`, `LANG`, `LC_ALL`, `LC_CTYPE`, `LOGNAME`, `USER`, `PATH`, `SHELL`, `TERM`, `TMPDIR`, `TZ` |

## Installer le step runner manuellement {#install-step-runner-manually}

Des binaires précompilés pour de nombreuses plateformes sont disponibles sur la [page des releases du step runner](https://gitlab.com/gitlab-org/step-runner/-/releases). Les plateformes prises en charge incluent Windows, Linux, macOS et FreeBSD pour plusieurs architectures (amd64, arm64, 386, arm, s390x, ppc64le).

### Vérifier l'authenticité du binaire {#verify-authenticity-of-the-binary}

Avant l'installation, vérifiez que le binaire n'a pas été altéré et qu'il provient de l'équipe officielle GitLab.

1. Téléchargez et importez la clé publique GPG :

   ```shell
   # All platforms (requires gpg installed: https://gnupg.org/download/)
   curl -o step-runner.pub.gpg "https://gitlab.com/gitlab-org/step-runner/-/package_files/257922684/download"
   gpg --import step-runner.pub.gpg
   gpg --fingerprint
   ```

   Vérifiez que la clé importée correspond bien à ce qui suit :

   | Attribut de clé | Valeur                                                |
   |---------------|------------------------------------------------------|
   | Nom          | `GitLab, Inc.`                                       |
   | E-mail         | `support@gitlab.com`                                 |
   | Empreinte   | `0FCD 59B1 6F4A 62D0 3839  27A5 42FF CA71 62A5 35F5` |
   | Expiration        | `2029-01-05`                                         |

1. Depuis la [page des releases](https://gitlab.com/gitlab-org/step-runner/-/releases), téléchargez les fichiers suivants :

   - Le binaire pour votre plateforme (par exemple, `step-runner-linux-amd64` ou `step-runner-darwin-arm64`)
   - `step-runner-release.sha256`
   - `step-runner-release.sha256.asc`

1. Vérifiez la signature GPG :

   ```shell
   # All platforms (requires gpg)
   gpg --verify step-runner-release.sha256.asc step-runner-release.sha256
   ```

   La sortie doit inclure un message `Good signature`.

1. Vérifiez la somme de contrôle du binaire :

   ```shell
   # Linux
   sha256sum -c step-runner-release.sha256
   ```

   ```shell
   # macOS
   shasum -a 256 -c step-runner-release.sha256
   ```

   ```shell
   # Windows (PowerShell) — replace 'step-runner-windows-amd64.exe' with your binary name
   $binary = "step-runner-windows-amd64.exe"
   $expected = (Select-String -Path "step-runner-release.sha256" -Pattern $binary).Line.Split(" ")[0]
   $actual = (Get-FileHash -Algorithm SHA256 $binary).Hash.ToLower()
   if ($actual -eq $expected) { "OK" } else { "FAILED: checksum mismatch" }
   ```

   La sortie doit afficher `OK` pour votre binaire.

### Ajouter step-runner au PATH {#add-step-runner-to-path}

Après avoir téléchargé et vérifié le binaire, rendez-le disponible sur le `PATH` de l'instance où vos jobs s'exécutent. Cette instance peut être la machine hôte ou un conteneur, selon votre exécuteur.

1. Renommez le binaire en `step-runner` (ou `step-runner.exe` sur Windows) :

   ```shell
   mv step-runner-<os>-<arch> step-runner
   ```

1. Sur les systèmes de type Unix, rendez le binaire exécutable :

   ```shell
   chmod +x step-runner
   ```

1. Déplacez le binaire vers un répertoire présent dans votre `PATH` :

   ```shell
   mv step-runner /usr/local/bin/
   ```
