---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Installer GitLab Runner manuellement sur z/OS.
title: Installer GitLab Runner manuellement sur z/OS
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner pour IBM z/OS a été certifié par GitLab et peut exécuter des jobs CI/CD nativement dans les environnements mainframe z/OS.

Vous pouvez télécharger et installer GitLab Runner sur z/OS manuellement depuis une archive [`pax`](https://www.ibm.com/docs/en/aix/7.1.0?topic=p-pax-command).

## Prérequis {#prerequisites}

- Pour utiliser GitLab Runner, vous avez besoin des rapports d'analyse de programme autorisés (`APARs`) suivants avec les correctifs temporaires de programme (`PTFs`) :
  - z/OS 2.5
    - OA62757
    - PH45182
  - z/OS 3.1
    - OA62757
    - PH57159
- GitLab Runner s'attend à ce que bash soit installé à `/bin/bash` pour exécuter des commandes shell. Si bash n'est pas installé à cet emplacement, créez un lien symbolique vers la version installée :

  ```shell
  ln -s <TARGET_BASH> /bin/bash
  ```

## Installer GitLab Runner {#install-gitlab-runner}

Pour installer GitLab Runner :

1. Téléchargez le `paxfile` dans le répertoire d'installation de votre choix.
1. Installez le paquet pour votre système :

   ```shell
   pax -ppx -rf gitlab-runner-<VERSION>.pax.Z
   ```

   Les fichiers installés sont extraits dans le répertoire `gitlab-runner` à l'emplacement d'installation.

1. Accordez les permissions d'exécution au fichier :

   ```shell
   chmod +x <INSTALL_PATH>/bin/gitlab-runner
   ```

1. Exportez GitLab Runner et ajoutez-le à votre `PATH` :

   ```shell
   export GITLAB_RUNNER=<INSTALL_PATH>/gitlab-runner/bin
   export PATH=${GITLAB_RUNNER}:${PATH}
   ```

1. [Enregistrer un runner](../register/_index.md).

## Exécuter GitLab Runner {#run-gitlab-runner}

Vous pouvez exécuter GitLab Runner directement ou en tant que tâche démarrée.

### Exécuter GitLab Runner directement {#run-gitlab-runner-directly}

Pour exécuter GitLab Runner en appelant l'exécutable :

1. Accédez au répertoire `<INSTALL_PATH>/bin`.
1. Démarrez le service :

   ```shell
   gitlab-runner start
   ```

### Exécuter GitLab Runner en tant que tâche démarrée {#run-gitlab-runner-as-a-started-task}

Pour maintenir le processus GitLab Runner disponible, exécutez-le en tant que tâche démarrée.

1. Enveloppez l'exécutable dans un script shell `gitlab-runner.sh` :

   ```shell
   #! /bin/sh
   <INSTALL_PATH>/bin/gitlab-runner start
   ```

1. Définissez un programme de tâche démarrée `jcl` et exécutez-le pour qu'il fonctionne comme un processus continu :

   ```jcl
   //GLRST  PROC CNFG='<PATH_TO_SCRIPT>'
   //*
   //GLRST  EXEC PGM=BPXBATSL,REGION=0M,TIME=NOLIMIT,
   //            PARM='PGM &CNFG./gitlab-runner.sh'
   //STDOUT   DD SYSOUT=*
   //STDERR   DD SYSOUT=*
   //*
   //        PEND
   ```
