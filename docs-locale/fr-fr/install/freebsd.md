---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Installer GitLab Runner sur les systèmes FreeBSD.
title: Installer GitLab Runner sur FreeBSD
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!note]
> La version FreeBSD est également disponible en tant que [release](bleeding-edge.md) bleeding edge. Assurez-vous de lire la section [FAQ](../faq/_index.md) qui décrit certains des problèmes les plus courants avec GitLab Runner.

## Installer GitLab Runner {#installing-gitlab-runner}

Voici les étapes pour installer et configurer GitLab Runner sous FreeBSD :

1. Créez l'utilisateur et le groupe `gitlab-runner` :

   ```shell
   sudo pw group add -n gitlab-runner
   sudo pw user add -n gitlab-runner -g gitlab-runner -s /usr/local/bin/bash
   sudo mkdir /home/gitlab-runner
   sudo chown gitlab-runner:gitlab-runner /home/gitlab-runner
   ```

1. Téléchargez le binaire pour votre système :

   ```shell
   # For amd64
   sudo fetch -o /usr/local/bin/gitlab-runner https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-freebsd-amd64

   # For i386
   sudo fetch -o /usr/local/bin/gitlab-runner https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-freebsd-386
   ```

   Vous pouvez télécharger un binaire pour chaque version disponible, comme décrit dans [Bleeding Edge - télécharger toute autre release taguée](bleeding-edge.md#download-any-other-tagged-release).

1. Accordez-lui les permissions d'exécution :

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. Créez un fichier journal vide avec les permissions correctes :

   ```shell
   sudo touch /var/log/gitlab_runner.log && sudo chown gitlab-runner:gitlab-runner /var/log/gitlab_runner.log
   ```

1. Créez le répertoire `rc.d` s'il n'existe pas :

   ```shell
   mkdir -p /usr/local/etc/rc.d
   ```

1. Créez le script `gitlab_runner` dans `rc.d` :

   Les utilisateurs de Bash peuvent effectuer les opérations suivantes :

   ```shell
   sudo bash -c 'cat > /usr/local/etc/rc.d/gitlab_runner' << "EOF"
   #!/bin/sh
   # PROVIDE: gitlab_runner
   # REQUIRE: DAEMON NETWORKING
   # BEFORE:
   # KEYWORD:

   . /etc/rc.subr

   name="gitlab_runner"
   rcvar="gitlab_runner_enable"

   user="gitlab-runner"
   user_home="/home/gitlab-runner"
   command="/usr/local/bin/gitlab-runner"
   command_args="run"
   pidfile="/var/run/${name}.pid"

   start_cmd="gitlab_runner_start"

   gitlab_runner_start()
   {
      export USER=${user}
      export HOME=${user_home}
      if checkyesno ${rcvar}; then
         cd ${user_home}
         /usr/sbin/daemon -u ${user} -p ${pidfile} ${command} ${command_args} > /var/log/gitlab_runner.log 2>&1
      fi
   }

   load_rc_config $name
   run_rc_command $1
   EOF
   ```

   Si vous n'utilisez pas bash, créez un fichier nommé `/usr/local/etc/rc.d/gitlab_runner` et incluez le contenu suivant :

   ```shell
   #!/bin/sh
   # PROVIDE: gitlab_runner
   # REQUIRE: DAEMON NETWORKING
   # BEFORE:
   # KEYWORD:

   . /etc/rc.subr

   name="gitlab_runner"
   rcvar="gitlab_runner_enable"

   user="gitlab-runner"
   user_home="/home/gitlab-runner"
   command="/usr/local/bin/gitlab-runner"
   command_args="run"
   pidfile="/var/run/${name}.pid"

   start_cmd="gitlab_runner_start"

   gitlab_runner_start()
   {
      export USER=${user}
      export HOME=${user_home}
      if checkyesno ${rcvar}; then
         cd ${user_home}
         /usr/sbin/daemon -u ${user} -p ${pidfile} ${command} ${command_args} > /var/log/gitlab_runner.log 2>&1
      fi
   }

   load_rc_config $name
   run_rc_command $1
   ```

1. Rendez le script `gitlab_runner` exécutable :

   ```shell
   sudo chmod +x /usr/local/etc/rc.d/gitlab_runner
   ```

1. [Enregistrez un runner](../register/_index.md)
1. Activez le service `gitlab-runner` et démarrez-le :

   ```shell
   sudo sysrc gitlab_runner_enable=YES
   sudo service gitlab_runner start
   ```

   Si vous ne souhaitez pas activer le service `gitlab-runner` au démarrage après un redémarrage, utilisez :

   ```shell
   sudo service gitlab_runner onestart
   ```
