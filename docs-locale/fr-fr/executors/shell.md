---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "L'exécuteur Shell"
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!note]
> Cet exécuteur est en mode maintenance. Il reçoit des mises à jour de sécurité critiques, mais aucune nouvelle fonctionnalité n'est prévue. Pour les nouveaux projets, envisagez d'utiliser l'un des [exécuteurs activement développés](_index.md#selecting-the-executor).

L'exécuteur Shell est la configuration d'exécuteur la plus simple pour GitLab Runner. Il exécute les builds localement sur la machine où GitLab Runner est installé, donc toutes les dépendances doivent être installées sur la même machine. Il prend en charge tous les systèmes sur lesquels le runner peut être installé. Cela signifie qu'il est possible d'utiliser des scripts générés pour Bash, PowerShell Core, Windows PowerShell et Windows Batch (déprécié).

Bien qu'il soit idéal pour les builds avec des dépendances minimales, l'exécuteur Shell offre une isolation limitée entre les jobs.

> [!note]
> Assurez-vous de satisfaire aux [prérequis communs](_index.md#git-requirements-for-non-docker-executors) sur la machine où GitLab Runner utilise l'exécuteur shell.

## Exécuter des scripts en tant qu'utilisateur privilégié {#run-scripts-as-a-privileged-user}

Les scripts peuvent être exécutés en tant qu'utilisateur non privilégié si le `--user` est ajouté à la [commande `gitlab-runner run`](../commands/_index.md#gitlab-runner-run). Cette fonctionnalité est uniquement prise en charge par Bash.

Le projet source est extrait vers : `<working-directory>/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`.

Les caches du projet sont stockés dans `<working-directory>/cache/<namespace>/<project-name>`.

Où :

- `<working-directory>` est la valeur de `--working-directory` telle que passée à la commande `gitlab-runner run` ou le répertoire courant où le runner s'exécute
- `<short-token>` est une version raccourcie du jeton du runner (les 8 premières lettres)
- `<concurrent-id>` est l'index du runner dans la liste de tous les runners qui exécutent un build pour le même projet de manière simultanée (accessible via la [variable prédéfinie](https://docs.gitlab.com/ci/variables/predefined_variables/) `CI_CONCURRENT_PROJECT_ID`).
- `<namespace>` est l'espace de nommage où le projet est stocké sur GitLab
- `<project-name>` est le nom du projet tel qu'il est stocké sur GitLab

Pour remplacer `<working-directory>/builds` et `<working-directory/cache`, spécifiez les options `builds_dir` et `cache_dir` dans la section `[[runners]]` de [`config.toml`](../configuration/advanced-configuration.md).

## Exécuter des scripts en tant qu'utilisateur non privilégié {#run-scripts-as-an-unprivileged-user}

Si GitLab Runner est installé sur Linux à partir des [packages officiels `.deb` ou `.rpm`](https://packages.gitlab.com/runner/gitlab-runner), le programme d'installation tente d'utiliser l'utilisateur `gitlab_ci_multi_runner` s'il est trouvé. Si le programme d'installation ne parvient pas à trouver l'utilisateur `gitlab_ci_multi_runner`, il crée un utilisateur `gitlab-runner` et l'utilise à la place.

Tous les builds shell sont ensuite exécutés en tant qu'utilisateur `gitlab-runner` ou `gitlab_ci_multi_runner`.

Dans certains scénarios de test, vos builds peuvent nécessiter l'accès à des ressources privilégiées, comme Docker Engine ou VirtualBox. Dans ce cas, vous devez ajouter l'utilisateur `gitlab-runner` au groupe correspondant :

```shell
usermod -aG docker gitlab-runner
usermod -aG vboxusers gitlab-runner
```

## Sélectionner votre shell {#selecting-your-shell}

GitLab Runner [prend en charge certains shells](../shells/_index.md). Pour sélectionner un shell, spécifiez-le dans votre fichier `config.toml`. Par exemple :

```toml
...
[[runners]]
  name = "shell executor runner"
  executor = "shell"
  shell = "powershell"
...
```

## Sécurité {#security}

En général, il est risqué d'exécuter des jobs avec des exécuteurs shell. Les jobs sont exécutés avec les permissions de l'utilisateur (`gitlab-runner`) et peuvent « voler » du code d'autres projets exécutés sur ce serveur. Selon votre configuration, le job pourrait exécuter des commandes arbitraires sur le serveur en tant qu'utilisateur hautement privilégié. Utilisez-le uniquement pour exécuter des builds à partir d'utilisateurs en qui vous avez confiance, sur un serveur en qui vous avez confiance et que vous possédez.

## Terminer et arrêter des processus {#terminating-and-killing-processes}

L'exécuteur shell démarre le script de chaque job dans un nouveau processus. Sur les systèmes UNIX, il définit le processus principal comme un groupe de processus.

GitLab Runner termine les processus dans les cas suivants :

- Un job [expire](https://docs.gitlab.com/ci/pipelines/settings/#set-a-limit-for-how-long-jobs-can-run).
- Un job est annulé.

Sur les systèmes UNIX, `gitlab-runner` envoie `SIGTERM` au processus et à ses processus enfants, puis envoie `SIGKILL` après 10 minutes. Cela permet une terminaison progressive du processus. Windows n'a pas d'équivalent à `SIGTERM`, donc le signal d'arrêt est envoyé deux fois. Le second est envoyé après 10 minutes.
