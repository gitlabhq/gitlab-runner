---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: SSH
---

{{< details >}}

- Niveau : Free, Premium, Ultimate
- Offre : GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!note]
> Cet exécuteur est en mode maintenance. Il reçoit des mises à jour de sécurité critiques, mais aucune nouvelle fonctionnalité n'est prévue. Pour les nouveaux projets, envisagez d'utiliser l'un des [exécuteurs activement développés](_index.md#selecting-the-executor).

L'exécuteur SSH est inclus par souci d'exhaustivité, mais il fait partie des exécuteurs les moins pris en charge. GitLab Runner se connecte à un serveur externe et y exécute des builds via SSH. Bien que certaines organisations utilisent cet exécuteur avec succès, il est généralement préférable d'utiliser un autre type d'exécuteur.

> [!note]
> L'exécuteur SSH ne prend en charge que les scripts générés en Bash et la fonctionnalité de mise en cache n'est pas prise en charge.

Cet exécuteur vous permet d'exécuter des builds sur une machine distante en exécutant des commandes via SSH.

> [!note]
> Assurez-vous de satisfaire aux [prérequis communs](_index.md#git-requirements-for-non-docker-executors) sur tout système distant où GitLab Runner utilise l'exécuteur SSH.

## Utiliser l'exécuteur SSH {#use-the-ssh-executor}

Pour utiliser l'exécuteur SSH, spécifiez `executor = "ssh"` dans la section [`[runners.ssh]`](../configuration/advanced-configuration.md#the-runnersssh-section). Par exemple :

```toml
[[runners]]
  executor = "ssh"
  [runners.ssh]
    host = "example.com"
    port = "22"
    user = "root"
    password = "password"
    identity_file = "/path/to/identity/file"
```

Vous pouvez utiliser `password` ou `identity_file` ou les deux pour vous authentifier auprès du serveur. GitLab Runner ne lit pas implicitement `identity_file` depuis `/home/user/.ssh/id_(rsa|dsa|ecdsa)`. Le `identity_file` doit être explicitement spécifié.

La source du projet est extraite vers : `~/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`.

Où :

- `<short-token>` est une version abrégée du token du runner (8 premières lettres)
- `<concurrent-id>` est l'index du runner dans la liste de tous les runners qui exécutent un build pour le même projet simultanément (accessible via la [variable prédéfinie](https://docs.gitlab.com/ci/variables/predefined_variables/) `CI_CONCURRENT_PROJECT_ID`).
- `<namespace>` est l'espace de nommage dans lequel le projet est stocké sur GitLab
- `<project-name>` est le nom du projet tel qu'il est stocké sur GitLab

Pour écraser le répertoire `~/builds`, spécifiez les options `builds_dir` sous la section `[[runners]]` dans [`config.toml`](../configuration/advanced-configuration.md).

Si vous souhaitez téléverser des artefacts de job, installez `gitlab-runner` sur l'hôte auquel vous vous connectez via SSH.

## Configurer la vérification stricte de la clé hôte {#configure-strict-host-key-checking}

SSH `StrictHostKeyChecking` est [activé](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28192) par défaut. Pour désactiver SSH `StrictHostKeyChecking`, définissez `[runners.ssh.disable_strict_host_key_checking]` sur `true`. La valeur par défaut actuelle est `false`.
