---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Sécurité pour les runners auto-gérés
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Un pipeline CI/CD GitLab est un moteur d'automatisation de workflows utilisé pour des tâches d'automatisation DevOps simples ou complexes. Ces pipelines activant un service d'exécution de code à distance, vous devez mettre en œuvre le processus suivant pour réduire les risques de sécurité :

- Une approche systématique de la configuration de la sécurité de l'ensemble de la pile technologique.
- Des révisions rigoureuses et continues de la configuration et de l'utilisation de la plateforme.

Si vous prévoyez d'exécuter vos jobs GitLab CI/CD sur des runners auto-gérés, des risques de sécurité existent pour votre infrastructure de calcul et votre réseau.

Le runner exécute le code défini dans le job CI/CD. Tout utilisateur disposant du rôle Developer pour le dépôt du projet pourrait compromettre la sécurité de l'environnement hébergeant le runner, que ce soit intentionnellement ou non.

Ce risque est encore plus aigu si vos runners auto-gérés sont non éphémères et utilisés pour plusieurs projets.

- Un job provenant d'un dépôt intégrant du code malveillant peut compromettre la sécurité des autres dépôts pris en charge par le runner non éphémère.
- Selon l'exécuteur, un job peut installer du code malveillant sur la machine virtuelle où le runner est hébergé.
- Les variables secrètes exposées aux jobs s'exécutant dans un environnement compromis peuvent être volées, y compris, mais sans s'y limiter, `CI_JOB_TOKEN`.
- Les utilisateurs disposant du rôle Developer ont accès aux sous-modules associés au projet, même s'ils n'ont pas accès aux projets amont du sous-module.

## Risques de sécurité pour les différents exécuteurs {#security-risks-for-different-executors}

Selon l'exécuteur que vous utilisez, vous pouvez être confronté à différents risques de sécurité.

### Utilisation de l'exécuteur Shell {#usage-of-shell-executor}

**Des risques de sécurité élevés existent pour l'hôte de votre runner et votre réseau lors de l'exécution de builds avec l'exécuteur `shell`**. Les jobs sont exécutés avec les permissions de l'utilisateur de GitLab Runner et peuvent voler du code d'autres projets exécutés sur ce serveur. Utilisez-le uniquement pour exécuter des builds de confiance.

### Utilisation de l'exécuteur Docker {#usage-of-docker-executor}

**Docker peut être considéré comme sûr lorsqu’il est utilisé en mode non privilégié**. Pour rendre une telle configuration plus sécurisée, exécutez les jobs en tant qu'utilisateur non root dans des conteneurs Docker avec `sudo` désactivé ou les capacités `SETUID` et `SETGID` supprimées.

Des permissions plus granulaires peuvent être configurées en mode non privilégié via les paramètres `cap_add`/`cap_drop`.

> [!warning]
> Les conteneurs privilégiés dans Docker disposent de toutes les capacités root de la VM hôte. Pour plus d'informations, consultez la documentation officielle de Docker sur [Runtime privilege and Linux capabilities](https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities)

Il est **déconseillé** d'exécuter des conteneurs en mode privilégié.

Lorsque le mode privilégié est activé, un utilisateur exécutant un job CI/CD pourrait obtenir un accès root complet au système hôte du runner, l'autorisation de monter et de détacher des volumes, et d'exécuter des conteneurs imbriqués.

En activant le mode privilégié, vous désactivez effectivement tous les mécanismes de sécurité du conteneur et exposez votre hôte à une escalade de privilèges, ce qui peut entraîner une évasion de conteneur.

Si vous utilisez un exécuteur Docker Machine, nous recommandons également fortement d'utiliser le paramètre `MaxBuilds = 1`, qui garantit qu'une seule VM auto-scalée (potentiellement compromise en raison de la faiblesse de sécurité introduite par le mode privilégié) est utilisée pour traiter un et un seul job.

### Utilisation d'images Docker privées avec la politique de pull `if-not-present` {#usage-of-private-docker-images-with-if-not-present-pull-policy}

Lorsque vous utilisez le support des images Docker privées décrit dans [configuration avancée : utilisation d'un registre de conteneurs privé](../configuration/advanced-configuration.md#use-a-private-container-registry), vous devez utiliser `always` comme valeur de `pull_policy`. En particulier, vous devez utiliser la politique de pull `always` si vous hébergez un runner d'instance public avec les exécuteurs Docker ou Kubernetes.

Prenons un exemple où la politique de pull est définie sur `if-not-present` :

1. L'utilisateur A dispose d'une image privée à l'adresse `registry.example.com/image/name`.
1. L'utilisateur A démarre un build sur un runner d'instance :  Le build reçoit les identifiants du registre et extrait l'image après autorisation dans le registre.
1. L'image est stockée sur l'hôte du runner d'instance.
1. L'utilisateur B n'a pas accès à l'image privée à l'adresse `registry.example.com/image/name`.
1. L'utilisateur B démarre un build utilisant cette image sur le même runner d'instance que l'utilisateur A :  Le runner trouve une version locale de l'image et l'utilise **même si l’image n’a pas pu être récupérée pour cause d’absence d’identifiants**.

Par conséquent, si vous hébergez un runner pouvant être utilisé par différents utilisateurs et différents projets (avec des niveaux d'accès privés et publics mélangés), vous ne devez jamais utiliser `if-not-present` comme valeur de politique de pull, mais utiliser :

- `never` - Si vous souhaitez limiter les utilisateurs à l'utilisation uniquement de l'image pré-téléchargée par vous.
- `always` - Si vous souhaitez donner aux utilisateurs la possibilité de télécharger n'importe quelle image depuis n'importe quel registre.

La politique de pull `if-not-present` doit être utilisée **uniquement** pour des runners spécifiques utilisés par des builds et des utilisateurs de confiance.

Lisez la [documentation sur les politiques de pull](../executors/docker.md#configure-how-runners-pull-images) pour plus d'informations.

### Utilisation de l'exécuteur SSH {#usage-of-ssh-executor}

**Les exécuteurs SSH sont vulnérables aux attaques MITM (man-in-the-middle)**, en raison de l'absence de l'option `StrictHostKeyChecking`. Cela sera corrigé dans l'une des prochaines releases.

### Utilisation de l'exécuteur Parallels {#usage-of-parallels-executor}

**L’exécuteur Parallels représente l’option la plus sûre possible** car il utilise une virtualisation complète du système avec des machines VM configurées pour s'exécuter en mode de virtualisation isolée et des machines VM configurées pour s'exécuter en mode isolé. Il bloque l'accès à tous les périphériques et dossiers partagés.

## Clonage d'un runner {#cloning-a-runner}

Les runners utilisent un jeton pour s'identifier auprès du serveur GitLab. Si vous clonez un runner, le runner cloné pourrait récupérer les mêmes jobs pour ce jeton. Il s'agit d'un vecteur d'attaque possible pour « voler » les jobs du runner.

## Risques de sécurité lors de l'utilisation de `GIT_STRATEGY: fetch` sur des environnements partagés {#security-risks-when-using-git_strategy-fetch-on-shared-environments}

Lorsque vous définissez [`GIT_STRATEGY`](https://docs.gitlab.com/ci/runners/configure_runners/#git-strategy) sur `fetch`, le runner tente de réutiliser la copie de travail locale du dépôt Git.

L'utilisation d'une copie locale peut améliorer les performances des jobs CI/CD. Cependant, tout utilisateur ayant accès à cette copie réutilisable peut ajouter du code qui s'exécute dans les pipelines d'autres utilisateurs.

Git stocke le contenu d'un sous-module (un dépôt intégré dans un autre dépôt) dans le reflog Git du dépôt parent. Par conséquent, après le clonage initial des sous-modules d'un projet, les jobs suivants peuvent accéder au contenu des sous-modules en exécutant `git submodule update` dans leur script. Cela s'applique même si les sous-modules ont été supprimés et que l'utilisateur qui a initié le job n'a pas accès aux projets de sous-modules.

Utilisez `GIT_STRATEGY: fetch` uniquement lorsque vous faites confiance à tous les utilisateurs ayant accès à l'environnement partagé.

## Options de renforcement de la sécurité {#security-hardening-options}

### Réduire le risque de sécurité lié à l'utilisation de conteneurs privilégiés {#reduce-the-security-risk-of-using-privileged-containers}

Si vous devez exécuter des jobs CI/CD nécessitant l'utilisation du flag `--privileged` de Docker, vous pouvez suivre ces étapes pour réduire le risque de sécurité :

- Exécutez les conteneurs Docker avec le flag `--privileged` activé uniquement sur des machines virtuelles isolées et éphémères.
- Configurez des runners dédiés destinés à exécuter des jobs nécessitant l'utilisation du flag `--privileged` de Docker. Configurez ensuite ces runners pour exécuter des jobs uniquement sur des branches protégées.

### Segmentation réseau {#network-segmentation}

GitLab Runner est conçu pour exécuter des scripts contrôlés par l'utilisateur. Pour réduire la surface d'attaque si un job est malveillant, vous pouvez envisager de les exécuter dans leur propre segment réseau. Cela permettrait une séparation réseau des autres infrastructures et services.

Tous les besoins sont uniques, mais pour un environnement cloud, cela pourrait inclure :

- Configurer les machines virtuelles du runner dans leur propre segment réseau
- Bloquer l'accès SSH depuis Internet vers les machines virtuelles du runner
- Restreindre le trafic entre les machines virtuelles du runner
- Filtrer l'accès aux points de terminaison de métadonnées du fournisseur cloud

> [!note]
> Tous les runners auront besoin d'une connectivité réseau sortante vers GitLab.com ou votre instance GitLab. La plupart des jobs nécessiteront également une connectivité réseau sortante vers Internet - pour le téléchargement des dépendances, etc.

### Sécuriser l'hôte du runner {#secure-the-runner-host}

Si vous utilisez un hôte statique pour un runner, que ce soit un serveur bare-metal ou une machine virtuelle, vous devez mettre en œuvre les meilleures pratiques de sécurité pour le système d'exploitation hôte.

Du code malveillant exécuté dans le contexte d'un job CI pourrait compromettre l'hôte, et les protocoles de sécurité peuvent aider à atténuer l'impact. D'autres points à garder à l'esprit incluent la sécurisation ou la suppression de fichiers tels que les clés SSH du système hôte, qui pourraient permettre à un attaquant d'accéder à d'autres points de terminaison dans l'environnement.

### Nettoyer le dossier `.git` après chaque build {#clean-up-the-git-folder-after-each-build}

Si vous utilisez un hôte statique pour votre runner, vous pouvez mettre en œuvre une couche de sécurité supplémentaire en activant le feature flag `FF_ENABLE_JOB_CLEANUP` [feature flag](../configuration/feature-flags.md).

Lorsque vous activez `FF_ENABLE_JOB_CLEANUP`, le répertoire de build que votre runner utilise sur l'hôte est nettoyé après chaque build.
