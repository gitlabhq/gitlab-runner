---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Exécuter GitLab Runner dans un conteneur Docker.
title: Exécuter GitLab Runner dans un conteneur
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Vous pouvez exécuter GitLab Runner dans un conteneur Docker pour exécuter des jobs CI/CD. L'image Docker de GitLab Runner inclut toutes les dépendances nécessaires pour :

- Exécuter GitLab Runner.
- Exécuter des jobs CI/CD dans des conteneurs.

Les images Docker de GitLab Runner utilisent [Ubuntu ou Alpine Linux](#docker-images) comme base. Elles encapsulent la commande standard `gitlab-runner`, de manière similaire à l'installation de GitLab Runner directement sur l'hôte.

La commande `gitlab-runner` s'exécute dans un conteneur Docker. Cette configuration délègue le contrôle total du démon Docker à chaque conteneur GitLab Runner. L'effet est que les garanties d'isolation sont rompues si vous exécutez GitLab Runner dans un démon Docker qui exécute également d'autres charges de travail.

Dans cette configuration, chaque commande GitLab Runner que vous exécutez a un équivalent `docker run`, comme ceci :

- Commande Runner : `gitlab-runner <runner command and options...>`
- Commande Docker : `docker run <chosen docker options...> gitlab/gitlab-runner <runner command and options...>`

Par exemple, pour obtenir les informations d'aide de premier niveau pour GitLab Runner, remplacez la partie `gitlab-runner` de la commande par `docker run [docker options] gitlab/gitlab-runner`, comme ceci :

```shell
docker run --rm -t -i gitlab/gitlab-runner --help

NAME:
   gitlab-runner - a GitLab Runner

USAGE:
   gitlab-runner [global options] command [command options] [arguments...]

VERSION:
   18.10.1 (3b43bf9f)

(...)
```

## Compatibilité des versions du moteur Docker {#docker-engine-version-compatibility}

Les versions du moteur Docker et de l'image de conteneur GitLab Runner n'ont pas à correspondre. Les images GitLab Runner sont rétrocompatibles et compatibles avec les versions futures. Pour vous assurer de disposer des dernières fonctionnalités et mises à jour de sécurité, vous devriez toujours utiliser la dernière version stable de [Docker Engine](https://docs.docker.com/engine/install/).

## Installer l'image Docker et démarrer le conteneur {#install-the-docker-image-and-start-the-container}

Prérequis :

- Vous avez [installé Docker](https://docs.docker.com/get-started/get-docker/).
- Vous avez lu la [FAQ](../faq/_index.md) pour en savoir plus sur les problèmes courants dans GitLab Runner.

1. Téléchargez l'image Docker `gitlab-runner` en utilisant la commande `docker pull gitlab/gitlab-runner:<version-tag>`.

   Pour la liste des balises de version disponibles, consultez [les balises GitLab Runner](https://hub.docker.com/r/gitlab/gitlab-runner/tags).
1. Exécutez l'image Docker `gitlab-runner` en utilisant la commande `docker run -d [options] <image-uri> <runner-command>`.
1. Lorsque vous exécutez `gitlab-runner` dans un conteneur Docker, assurez-vous que la configuration n'est pas perdue lorsque vous redémarrez le conteneur. Montez un volume permanent pour stocker la configuration. Le volume peut être monté dans :

   - [Un volume système local](#from-a-local-system-volume)
   - [Un volume Docker](#from-a-docker-volume)

1. Facultatif. Si vous utilisez un [`session_server`](../configuration/advanced-configuration.md), exposez le port `8093` en ajoutant `-p 8093:8093` à vos commandes `docker run`.
1. Facultatif. Pour utiliser l'exécuteur Docker Machine pour la mise à l'échelle automatique, montez le chemin de stockage Docker Machine (`/root/.docker/machine`) en ajoutant un montage de volume à vos commandes `docker run` :

   - Pour les montages de volumes système, ajoutez `-v /srv/gitlab-runner/docker-machine-config:/root/.docker/machine`
   - Pour les volumes Docker nommés, ajoutez `-v docker-machine-config:/root/.docker/machine`

1. [Enregistrez un nouveau runner](../register/_index.md). Le conteneur GitLab Runner doit être enregistré pour prendre en charge des jobs.

Parmi les options de configuration disponibles, on trouve :

- Définissez le fuseau horaire du conteneur avec l'indicateur `--env TZ=<TIMEZONE>`. [Voir la liste des fuseaux horaires disponibles](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones).
- Pour une image [GitLab Runner conforme FIPS](requirements.md#fips-compliant-gitlab-runner), basée sur `redhat/ubi9-micro`, utilisez les balises `gitlab/gitlab-runner:ubi-fips`.
- [Installer les certificats SSL de serveur approuvés](#install-trusted-ssl-server-certificates).

### À partir d'un volume système local {#from-a-local-system-volume}

Pour utiliser votre système local pour le volume de configuration et les autres ressources montées dans le conteneur `gitlab-runner` :

1. Facultatif. Sur les systèmes MacOS, `/srv` n'existe pas par défaut. Créez `/private/srv`, ou un autre répertoire privé, pour la configuration.
1. Exécutez cette commande, en la modifiant selon vos besoins :

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /srv/gitlab-runner/config:/etc/gitlab-runner \
     -v /var/run/docker.sock:/var/run/docker.sock \
     gitlab/gitlab-runner:latest
   ```

### À partir d'un volume Docker {#from-a-docker-volume}

Pour utiliser un conteneur de configuration afin de monter votre volume de données personnalisé :

1. Créez le volume Docker :

   ```shell
   docker volume create gitlab-runner-config
   ```

1. Démarrez le conteneur GitLab Runner en utilisant le volume que vous venez de créer :

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /var/run/docker.sock:/var/run/docker.sock \
     -v gitlab-runner-config:/etc/gitlab-runner \
     gitlab/gitlab-runner:latest
   ```

## Mettre à jour la configuration du runner {#update-runner-configuration}

Après avoir [modifié la configuration du runner](../configuration/advanced-configuration.md) dans `config.toml`, appliquez vos modifications en redémarrant le conteneur avec `docker stop` et `docker run`.

## Mettre à niveau la version du runner {#upgrade-runner-version}

Prérequis :

- Vous devez utiliser la même méthode pour monter votre volume de données que celle utilisée initialement (`-v /srv/gitlab-runner/config:/etc/gitlab-runner` ou `-v gitlab-runner-config:/etc/gitlab-runner`).

1. Récupérez la dernière version (ou une balise spécifique) :

   ```shell
   docker pull gitlab/gitlab-runner:latest
   ```

1. Arrêtez et supprimez le conteneur existant :

   ```shell
   docker stop gitlab-runner && docker rm gitlab-runner
   ```

1. Démarrez le conteneur comme vous l'avez fait initialement :

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /var/run/docker.sock:/var/run/docker.sock \
     -v /srv/gitlab-runner/config:/etc/gitlab-runner \
     gitlab/gitlab-runner:latest
   ```

## Afficher les journaux du runner {#view-runner-logs}

Les emplacements des fichiers journaux dépendent de la façon dont vous démarrez un runner. Lorsque vous le démarrez en tant que :

- **Tâche en premier plan**, qu'il s'agisse d'un binaire installé localement ou dans un conteneur Docker, les journaux s'affichent dans `stdout`.
- **Service système**, comme avec `systemd`, les journaux sont disponibles dans le mécanisme de journalisation du système, comme Syslog.
- **Service basé sur Docker**, utilisez la commande `docker logs`, car la commande `gitlab-runner ...` est le processus principal du conteneur.

Par exemple, si vous démarrez un conteneur avec cette commande, son nom est défini sur `gitlab-runner` :

```shell
docker run -d --name gitlab-runner --restart always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /srv/gitlab-runner/config:/etc/gitlab-runner \
  gitlab/gitlab-runner:latest
```

Pour afficher ses journaux, exécutez cette commande, en remplaçant `gitlab-runner` par le nom de votre conteneur :

```shell
docker logs gitlab-runner
```

Pour plus d'informations sur la gestion des journaux de conteneurs, consultez [`docker container logs`](https://docs.docker.com/reference/cli/docker/container/logs/) dans la documentation Docker.

## Installer les certificats SSL de serveur approuvés {#install-trusted-ssl-server-certificates}

Si votre serveur GitLab CI/CD utilise des certificats SSL auto-signés, assurez-vous que votre conteneur runner approuve le certificat du serveur GitLab CI. Cela évite les échecs de communication.

Prérequis :

- Votre fichier `ca.crt` doit contenir les certificats racine de tous les serveurs auxquels vous souhaitez que GitLab Runner fasse confiance.

1. Facultatif. L'image `gitlab/gitlab-runner` recherche les certificats SSL approuvés dans `/etc/gitlab-runner/certs/ca.crt`. Pour modifier ce comportement, utilisez l'option de configuration `-e "CA_CERTIFICATES_PATH=/DIR/CERT"`.
1. Copiez votre fichier `ca.crt` dans le répertoire `certs` sur le volume de données (ou le conteneur).
1. Facultatif. Si votre conteneur est déjà en cours d'exécution, redémarrez-le pour importer le fichier `ca.crt` au démarrage.

## Images Docker {#docker-images}

Dans GitLab Runner 18.8.0, l'image Docker basée sur Alpine utilise Alpine 3.21. Ces images Docker multi-plateformes sont disponibles :

- `gitlab/gitlab-runner:latest` basée sur Ubuntu, environ 470 Mo.
- `gitlab/gitlab-runner:alpine` basée sur Alpine, environ 270 Mo.

Consultez la source de [GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner/tree/main/dockerfiles) pour obtenir d'éventuelles instructions de compilation pour les images Ubuntu et Alpine.

### Créer une image Docker de runner {#create-a-runner-docker-image}

Vous pouvez mettre à niveau le système d'exploitation de votre image avant que la mise à jour soit disponible dans les dépôts GitLab.

Prérequis :

- Vous n'utilisez pas l'image IBM Z, car elle ne contient pas la dépendance `docker-machine`. Cette image n'est pas maintenue pour les plateformes Linux s390x ou Linux ppc64le. Pour connaître l'état actuel, consultez le [ticket 26551](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26551).

Pour créer une image Docker `gitlab-runner` pour la dernière version d'Alpine :

1. Créez `alpine-upgrade/Dockerfile`.

   ```dockerfile
   ARG GITLAB_RUNNER_IMAGE_TYPE
   ARG GITLAB_RUNNER_IMAGE_TAG
   FROM gitlab/${GITLAB_RUNNER_IMAGE_TYPE}:${GITLAB_RUNNER_IMAGE_TAG}

   RUN apk update
   RUN apk upgrade
   ```

1. Créez une image `gitlab-runner` mise à niveau.

   ```shell
   GITLAB_RUNNER_IMAGE_TYPE=gitlab-runner \
   GITLAB_RUNNER_IMAGE_TAG=alpine-v18.10.1 \
   docker build -t $GITLAB_RUNNER_IMAGE_TYPE:$GITLAB_RUNNER_IMAGE_TAG \
     --build-arg GITLAB_RUNNER_IMAGE_TYPE=$GITLAB_RUNNER_IMAGE_TYPE \
     --build-arg GITLAB_RUNNER_IMAGE_TAG=$GITLAB_RUNNER_IMAGE_TAG \
     -f alpine-upgrade/Dockerfile alpine-upgrade
   ```

1. Créez une image `gitlab-runner-helper` mise à niveau.

   ```shell
   GITLAB_RUNNER_IMAGE_TYPE=gitlab-runner-helper \
   GITLAB_RUNNER_IMAGE_TAG=x86_64-v18.10.1 \
   docker build -t $GITLAB_RUNNER_IMAGE_TYPE:$GITLAB_RUNNER_IMAGE_TAG \
     --build-arg GITLAB_RUNNER_IMAGE_TYPE=$GITLAB_RUNNER_IMAGE_TYPE \
     --build-arg GITLAB_RUNNER_IMAGE_TAG=$GITLAB_RUNNER_IMAGE_TAG \
     -f alpine-upgrade/Dockerfile alpine-upgrade
   ```

## Utiliser SELinux dans votre conteneur {#use-selinux-in-your-container}

Certaines distributions, comme CentOS, Red Hat et Fedora, utilisent SELinux (Security-Enhanced Linux) par défaut pour renforcer la sécurité du système sous-jacent.

Utilisez cette configuration avec précaution.

Prérequis :

- Pour utiliser l'[exécuteur Docker](../executors/docker.md) afin d'exécuter des compilations dans des conteneurs, les runners ont besoin d'accéder à `/var/run/docker.sock`.
- Si vous utilisez SELinux en mode d'application, installez [`selinux-dockersock`](https://github.com/dpw/selinux-dockersock) pour éviter une erreur `Permission denied` lorsqu'un runner accède à `/var/run/docker.sock`.

1. Créez un répertoire persistant sur l'hôte : `mkdir -p /srv/gitlab-runner/config`.
1. Exécutez Docker avec `:Z` sur les volumes :

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /var/run/docker.sock:/var/run/docker.sock \
     -v /srv/gitlab-runner/config:/etc/gitlab-runner:Z \
     gitlab/gitlab-runner:latest
   ```
