---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Exécuteur Docker
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner utilise l'exécuteur Docker pour exécuter des jobs sur des images Docker.

Vous pouvez utiliser l'exécuteur Docker pour :

- Maintenir le même environnement de build pour chaque job.
- Utiliser la même image pour tester des commandes localement sans avoir à exécuter un job sur le serveur CI.

L'exécuteur Docker utilise [Docker Engine](https://www.docker.com/products/container-runtime/) pour exécuter chaque job dans un conteneur séparé et isolé. Pour se connecter à Docker Engine, l'exécuteur utilise :

- L'image et les services que vous définissez dans [`.gitlab-ci.yml`](https://docs.gitlab.com/ci/yaml/).
- Les configurations que vous définissez dans [`config.toml`](../commands/_index.md#configuration-file).

Vous ne pouvez pas enregistrer un runner et son exécuteur Docker sans définir une image par défaut dans `config.toml`. L'image définie dans `config.toml` peut être utilisée lorsqu'aucune n'est définie dans `.gitlab-ci.yml`. Si une image est définie dans `.gitlab-ci.yml`, elle remplace celle définie dans `config.toml`.

Prérequis :

- [Installer Docker](https://docs.docker.com/engine/install/).

## Workflow de l'exécuteur Docker {#docker-executor-workflow}

L'exécuteur Docker utilise une image Docker basée sur [Alpine Linux](https://alpinelinux.org/) qui contient les outils pour exécuter les étapes de préparation, pre-job et post-job. Pour afficher la définition de l'image Docker spéciale, consultez le [dépôt GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner/-/tree/main/dockerfiles/runner-helper).

L'exécuteur Docker divise le job en plusieurs étapes :

1. **Prepare** :  Crée et démarre les [services](https://docs.gitlab.com/ci/yaml/#services).
1. **Pre-job** :  Clone, restaure le [cache](https://docs.gitlab.com/ci/yaml/#cache) et télécharge les [artefacts](https://docs.gitlab.com/ci/yaml/#artifacts) des étapes précédentes. S'exécute sur une image Docker spéciale.
1. **Job** :  Exécute votre build dans l'image Docker que vous configurez pour le runner.
1. **Post-job** :  Crée le cache, téléverse les artefacts vers GitLab. S'exécute sur une image Docker spéciale.

## Configurations prises en charge {#supported-configurations}

L'exécuteur Docker prend en charge les configurations suivantes.

Pour les problèmes connus et les exigences supplémentaires des configurations Windows, consultez [Utiliser des conteneurs Windows](#use-windows-containers).

| Le runner est installé sur : | L'exécuteur est :     | Le conteneur exécute : |
|-------------------------|------------------|-----------------------|
| Windows                 | `docker-windows` | Windows               |
| Windows                 | `docker`         | Linux                 |
| Linux                   | `docker`         | Linux                 |
| macOS                   | `docker`         | Linux                 |

Ces configurations ne sont **pas** prises en charge :

| Le runner est installé sur : | L'exécuteur est :     | Le conteneur exécute : |
|-------------------------|------------------|-----------------------|
| Linux                   | `docker-windows` | Linux                 |
| Linux                   | `docker`         | Windows               |
| Linux                   | `docker-windows` | Windows               |
| Windows                 | `docker`         | Windows               |
| Windows                 | `docker-windows` | Linux                 |

> [!note]
> GitLab Runner utilise l'API Docker Engine [v1.25](https://docs.docker.com/reference/api/engine/version/v1.25/) pour communiquer avec Docker Engine. Cela signifie que la [version minimale prise en charge](https://docs.docker.com/reference/api/engine/#api-version-matrix) de Docker sur un serveur Linux est `1.13.0`. Sur Windows Server, [une version plus récente est requise](#supported-docker-versions) pour identifier la version de Windows Server.

## Utiliser l'exécuteur Docker {#use-the-docker-executor}

Pour utiliser l'exécuteur Docker, définissez manuellement Docker comme exécuteur dans `config.toml` ou utilisez la commande [`gitlab-runner register --executor "docker"`](../register/_index.md#register-with-a-runner-authentication-token) pour le définir automatiquement.

L'exemple de configuration suivant montre Docker défini comme exécuteur. Pour plus d'informations sur ces valeurs, consultez [Configuration avancée](../configuration/advanced-configuration.md)

```toml
concurrent = 4

[[runners]]
name = "myRunner"
url = "https://gitlab.com/ci"
token = "......"
executor = "docker"
[runners.docker]
  tls_verify = true
  image = "my.registry.tld:5000/alpine:latest"
  privileged = false
  disable_entrypoint_overwrite = false
  oom_kill_disable = false
  disable_cache = false
  volumes = [
    "/cache",
  ]
  shm_size = 0
  allowed_pull_policies = ["always", "if-not-present"]
  allowed_images = ["my.registry.tld:5000/*:*"]
  allowed_services = ["my.registry.tld:5000/*:*"]
  [runners.docker.volume_driver_ops]
    "size" = "50G"
```

## Configurer les images et les services {#configure-images-and-services}

Prérequis :

- L'image dans laquelle votre job s'exécute doit disposer d'un shell fonctionnel dans le `PATH` de son système d'exploitation. Les shells pris en charge sont :
  - Pour Linux :
    - `sh`
    - `bash`
    - PowerShell Core (`pwsh`). [Introduit dans la version 13.9](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4021).
  - Pour Windows :
    - PowerShell (`powershell`)
    - PowerShell Core (`pwsh`). [Introduit dans la version 13.6](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/13139).

Pour configurer l'exécuteur Docker, vous définissez les images Docker et les services dans [`.gitlab-ci.yml`](https://docs.gitlab.com/ci/yaml/) et [`config.toml`](../commands/_index.md#configuration-file).

Utilisez les mots-clés suivants :

- `image` :  Le nom de l'image Docker que le runner utilise pour exécuter les jobs.
  - Saisissez une image depuis le Docker Engine local ou toute image dans Docker Hub. Pour plus d'informations, consultez la [documentation Docker](https://docs.docker.com/get-started/introduction/).
  - Pour définir la version de l'image, utilisez un deux-points (`:`) pour ajouter un tag. Si vous ne spécifiez pas de tag, Docker utilise `latest` comme version.
- `services` :  L'image supplémentaire qui crée un autre conteneur et établit un lien vers `image`. Pour plus d'informations sur les types de services, consultez [Services](https://docs.gitlab.com/ci/services/).

### Définir des images et des services dans `.gitlab-ci.yml` {#define-images-and-services-in-gitlab-ciyml}

Définissez une image que le runner utilise pour tous les jobs et une liste de services à utiliser lors de la compilation.

Exemple :

```yaml
image: ruby:3.3

services:
  - postgres:9.3

before_script:
  - bundle install

test:
  script:
  - bundle exec rake spec
```

Pour définir différentes images et services par job :

```yaml
before_script:
  - bundle install

test:3.3:
  image: ruby:3.3
  services:
  - postgres:9.3
  script:
  - bundle exec rake spec

test:3.4:
  image: ruby:3.4
  services:
  - postgres:9.4
  script:
  - bundle exec rake spec
```

Si vous ne définissez pas d'`image` dans `.gitlab-ci.yml`, le runner utilise l'`image` défini dans `config.toml`.

### Définir des images et des services dans `config.toml` {#define-images-and-services-in-configtoml}

Pour ajouter des images et des services à tous les jobs exécutés par un runner, mettez à jour `[runners.docker]` dans le fichier `config.toml`.

Par défaut, l'exécuteur Docker utilise l'`image` définie dans `.gitlab-ci.yml`. Si vous n'en définissez pas dans `.gitlab-ci.yml`, le runner utilise l'image définie dans `config.toml`.

Exemple :

```toml
[runners.docker]
  image = "ruby:3.3"

[[runners.docker.services]]
  name = "mysql:latest"
  alias = "db"

[[runners.docker.services]]
  name = "redis:latest"
  alias = "cache"
```

Cet exemple utilise la [syntaxe des tableaux de tables](https://toml.io/en/v0.4.0#array-of-tables).

### Définir une image depuis un registre privé {#define-an-image-from-a-private-registry}

Prérequis :

- Pour accéder aux images d'un registre privé, vous devez [authentifier GitLab Runner](https://docs.gitlab.com/ci/docker/using_docker_images/#access-an-image-from-a-private-container-registry).

Pour définir une image depuis un registre privé, indiquez le nom du registre et l'image dans `.gitlab-ci.yml`.

Exemple :

```yaml
image: my.registry.tld:5000/namespace/image:tag
```

Dans cet exemple, GitLab Runner recherche dans le registre `my.registry.tld:5000` l'image `namespace/image:tag`.

## Configurations réseau {#network-configurations}

Vous devez configurer un réseau pour connecter les services à un job CI/CD.

Pour configurer un réseau, vous pouvez soit :

- Recommandé. Configurer le runner pour créer un réseau pour chaque job.
- Définir des liens de conteneurs. Les liens de conteneurs sont une fonctionnalité héritée de Docker.

### Créer un réseau pour chaque job {#create-a-network-for-each-job}

Vous pouvez configurer le runner pour créer un réseau pour chaque job.

Lorsque vous activez ce mode réseau, le runner crée et utilise un réseau bridge Docker défini par l'utilisateur pour chaque job. Les variables d'environnement Docker ne sont pas partagées entre les conteneurs. Pour plus d'informations sur les réseaux bridge définis par l'utilisateur, consultez la [documentation Docker](https://docs.docker.com/engine/network/drivers/bridge/).

Pour utiliser ce mode réseau, activez `FF_NETWORK_PER_BUILD` dans le feature flag ou la variable d'environnement dans le fichier `config.toml`.

Ne définissez pas `network_mode`.

Exemple :

```toml
[[runners]]
  (...)
  executor = "docker"
  environment = ["FF_NETWORK_PER_BUILD=1"]
```

Ou :

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.feature_flags]
    FF_NETWORK_PER_BUILD = true
```

Pour définir le pool d'adresses Docker par défaut, utilisez `default-address-pool` dans [`dockerd`](https://docs.docker.com/reference/cli/dockerd/). Si des plages CIDR sont déjà utilisées dans le réseau, les réseaux Docker peuvent entrer en conflit avec d'autres réseaux sur l'hôte, y compris d'autres réseaux Docker.

Cette fonctionnalité ne fonctionne que lorsque le daemon Docker est configuré avec IPv6 activé. Pour activer la prise en charge d'IPv6, définissez `enable_ipv6` sur `true` dans la configuration Docker. Pour plus d'informations, consultez la [documentation Docker](https://docs.docker.com/engine/daemon/ipv6/).

Le runner utilise l'alias `build` pour résoudre le conteneur du job.

Le DNS peut ne pas fonctionner correctement avec un service Docker-in-Docker (`dind`) lorsque vous utilisez cette fonctionnalité.

Ce comportement est dû à un problème avec [Docker/Moby](https://github.com/moby/moby/issues/20037#issuecomment-181659049), où les conteneurs `dind` n'héritent pas des entrées DNS personnalisées lorsque vous spécifiez un réseau.

Pour contourner ce problème, fournissez manuellement les paramètres DNS personnalisés au service `dind`. Par exemple, si votre serveur DNS personnalisé est `1.1.1.1`, vous pouvez utiliser `127.0.0.11`, qui est le service DNS interne de Docker :

```yaml
  services:
    - name: docker:dind
      command: [--dns=127.0.0.11, --dns=1.1.1.1]
```

Cette approche permet également aux conteneurs de résoudre les services sur le même réseau.

#### Comment le runner crée un réseau pour chaque job {#how-the-runner-creates-a-network-for-each-job}

Lorsqu'un job démarre, le runner :

1. Crée un réseau bridge, similaire à la commande Docker `docker network create <network>`.
1. Connecte le service et les conteneurs au réseau bridge.
1. Supprime le réseau à la fin du job.

Le conteneur exécutant le job et les conteneurs exécutant le service résolvent mutuellement leurs noms d'hôtes et alias. Cette fonctionnalité est [fournie par Docker](https://docs.docker.com/engine/network/drivers/bridge/#differences-between-user-defined-bridges-and-the-default-bridge).

### Configurer un réseau avec des liens de conteneurs {#configure-a-network-with-container-links}

GitLab Runner antérieur à la version 18.7.0 utilise le `bridge` Docker par défaut ainsi que les [liens de conteneurs hérités](https://docs.docker.com/engine/network/links/) pour lier le conteneur du job aux services. Étant donné que Docker a déprécié la fonctionnalité de liens, dans GitLab Runner 18.7.0 et versions ultérieures, le comportement des liens de conteneurs hérités est émulé en permettant la résolution des alias de service via la fonctionnalité `extra_hosts` de Docker. Ce mode réseau est le mode par défaut si [`FF_NETWORK_PER_BUILD`](#create-a-network-for-each-job) est désactivé.

Le comportement des liens émulés de GitLab Runner diffère légèrement des [liens de conteneurs hérités](https://docs.docker.com/engine/network/links/) :

- La désactivation de `icc` désactive la communication entre conteneurs et les conteneurs ne peuvent pas communiquer entre eux.
- Les variables d'environnement pour les conteneurs liés ne sont plus présentes (`<name>_PORT_<port>_<protocol>`).

Pour configurer le réseau, spécifiez le [mode réseau](https://docs.docker.com/engine/containers/run/#network-settings) dans le fichier `config.toml` :

- `bridge` :  Utiliser le réseau bridge. Par défaut.
- `host` :  Utiliser la pile réseau de l'hôte à l'intérieur du conteneur.
- `none` :  Aucun réseau. Non recommandé.

Exemple :

```toml
[[runners]]
  (...)
  executor = "docker"
[runners.docker]
  network_mode = "bridge"
```

Si vous utilisez toute autre valeur pour `network_mode`, celle-ci est prise comme le nom d'un réseau Docker existant auquel le conteneur de build se connecte.

Lors de la résolution de noms, Docker met à jour le fichier `/etc/hosts` dans le conteneur avec le nom d'hôte et l'alias du conteneur de service. Cependant, le conteneur de service n'est **pas** en mesure de résoudre le nom du conteneur. Pour résoudre le nom du conteneur, vous devez créer un réseau pour chaque job.

Les conteneurs liés partagent leurs variables d'environnement.

#### Remplacement du MTU du réseau créé {#overriding-the-mtu-of-the-created-network}

Pour certains environnements, comme les machines virtuelles dans OpenStack, un MTU personnalisé est nécessaire. Le daemon Docker ne respecte pas le MTU dans `docker.json` (voir [problème Moby 34981](https://github.com/moby/moby/issues/34981)). Vous pouvez définir `network_mtu` dans votre `config.toml` avec n'importe quelle valeur valide afin que le daemon Docker puisse utiliser le MTU correct pour le réseau nouvellement créé. Vous devez également activer [`FF_NETWORK_PER_BUILD`](#create-a-network-for-each-job) pour que le remplacement prenne effet.

La configuration suivante définit le MTU à `1402` pour le réseau créé pour chaque job. Assurez-vous d'ajuster la valeur en fonction des exigences de votre environnement spécifique.

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    network_mtu = 1402
    [runners.feature_flags]
      FF_NETWORK_PER_BUILD = true
```

#### Le MTU Docker-in-Docker n'hérite pas du runner {#docker-in-docker-mtu-does-not-inherit-from-the-runner}

Lorsque vous utilisez `docker:dind` comme service, le `dockerd` interne utilise par défaut le MTU `1500`, quelle que soit la configuration MTU sur le bridge Docker du runner. Si le MTU du bridge du runner est inférieur à `1500`, les paquets volumineux envoyés depuis les conteneurs de build à l'intérieur de dind sont silencieusement abandonnés. Étant donné que les réponses ICMP `fragmentation needed` sont souvent filtrées dans les environnements cloud et virtuels, l'expéditeur n'apprend jamais à réduire la taille de ses paquets, et les connexions se bloquent silencieusement.

Symptômes :  Des commandes comme `dotnet restore` ou `curl "https://api.nuget.org/v3/index.json"` expirent dans un job Docker-in-Docker, bien que ces commandes fonctionnent en dehors de dind.

Pour résoudre ce problème, définissez `--mtu` explicitement sur le service `docker:dind` avec une valeur inférieure ou égale au MTU du bridge Docker du runner :

```yaml
services:
  - name: docker:dind
    command: ["--mtu=1360"]
```

Si vous ne connaissez pas le MTU du bridge du runner, `1360` est une valeur sûre pour la plupart des environnements. Si vous omettez l'indicateur `--mtu` ou le définissez à une valeur supérieure au MTU du bridge du runner, les connexions se bloquent.

## Restreindre les images et services Docker {#restrict-docker-images-and-services}

Pour restreindre les images et services Docker, spécifiez un modèle générique dans les paramètres `allowed_images` et `allowed_services`. Pour plus de détails sur la syntaxe, consultez la [documentation doublestar](https://github.com/bmatcuk/doublestar).

Par exemple, pour autoriser uniquement les images de votre registre Docker privé :

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_images = ["my.registry.tld:5000/*:*"]
    allowed_services = ["my.registry.tld:5000/*:*"]
```

Pour restreindre à une liste d'images de votre registre Docker privé :

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_images = ["my.registry.tld:5000/ruby:*", "my.registry.tld:5000/node:*"]
    allowed_services = ["postgres:9.4", "postgres:latest"]
```

Pour exclure des images spécifiques comme Kali :

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_images = ["**", "!*/kali*"]
```

## Accéder aux noms d'hôtes des services {#access-services-hostnames}

Pour accéder au nom d'hôte d'un service, ajoutez le service dans `services` dans `.gitlab-ci.yml`.

```yaml
services:
- valkey/valkey:latest
```

Lorsque le job s'exécute, le service `valkey/valkey` démarre. Vous pouvez y accéder depuis votre conteneur de build sous le nom d'hôte `valkey__valkey` et `valkey-valkey`.

En plus des alias de service spécifiés, le runner attribue le nom de l'image de service comme alias au conteneur de service. Vous pouvez utiliser n'importe lequel de ces alias.

Le runner utilise les règles suivantes pour créer l'alias en fonction du nom de l'image :

- Tout ce qui suit `:` est supprimé.
- Pour le premier alias, la barre oblique (`/`) est remplacée par des doubles tirets bas (`__`).
- Pour le second alias, la barre oblique (`/`) est remplacée par un seul tiret (`-`).

Si vous utilisez une image de service privée, le runner supprime tout port spécifié et applique les règles. Le service `registry.example.com:4999/valkey/valkey` donne le nom d'hôte `registry.example.com__valkey__valkey` et `registry.example.com-valkey-valkey`.

## Configuration des services {#configuring-services}

Pour modifier des noms de bases de données ou définir des noms de comptes, vous pouvez définir des variables d'environnement pour le service.

Lorsque le runner transmet des variables :

- Les variables sont transmises à tous les conteneurs. Le runner ne peut pas transmettre des variables à des conteneurs spécifiques.
- Les variables sécurisées sont transmises au conteneur de build.

Pour plus d'informations sur les variables de configuration, consultez la documentation de chaque image fournie sur leur page Docker Hub correspondante.

### Monter un répertoire en RAM {#mount-a-directory-in-ram}

Vous pouvez utiliser l'option `tmpfs` pour monter un répertoire en RAM. Cela accélère le temps de test lorsqu'il y a beaucoup de travail lié aux E/S, comme avec les bases de données.

Si vous utilisez les options `tmpfs` et `services_tmpfs` dans la configuration du runner, vous pouvez spécifier plusieurs chemins, chacun avec ses propres options. Pour plus d'informations, consultez la [documentation Docker](https://docs.docker.com/reference/cli/docker/container/run/#tmpfs).

Par exemple, pour monter le répertoire de données du conteneur MySQL officiel en RAM, configurez le fichier `config.toml` :

```toml
[runners.docker]
  # For the main container
  [runners.docker.tmpfs]
      "/var/lib/mysql" = "rw,noexec"

  # For services
  [runners.docker.services_tmpfs]
      "/var/lib/mysql" = "rw,noexec"
```

### Créer un répertoire dans un service {#building-a-directory-in-a-service}

GitLab Runner monte un répertoire `/builds` sur tous les services partagés.

Pour plus d'informations sur l'utilisation de différents services, consultez :

- [Utiliser PostgreSQL](https://docs.gitlab.com/ci/services/postgres/)
- [Utiliser MySQL](https://docs.gitlab.com/ci/services/mysql/)

### Comment GitLab Runner effectue le contrôle de santé des services {#how-gitlab-runner-performs-the-services-health-check}

{{< history >}}

- [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4079) les vérifications de ports multiples dans GitLab 16.0.

{{< /history >}}

Après le démarrage du service, GitLab Runner attend que le service réponde. L'exécuteur Docker tente d'ouvrir une connexion TCP vers le port de service exposé dans le conteneur de service.

Seuls les 20 premiers ports exposés sont vérifiés.

La variable de service `HEALTHCHECK_TCP_PORT` peut être utilisée pour effectuer le contrôle de santé sur un port spécifique :

```yaml
job:
  services:
    - name: mongo
      variables:
        HEALTHCHECK_TCP_PORT: "27017"
```

Pour voir comment cela est implémenté, utilisez la [commande Go](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/commands/helpers/health_check.go) de contrôle de santé.

## Spécifier les opérations du pilote Docker {#specify-docker-driver-operations}

Spécifiez les arguments à fournir au pilote de volume Docker lors de la création de volumes pour les builds. Par exemple, vous pouvez utiliser ces arguments pour limiter l'espace disponible pour chaque build, en plus de toutes les autres options spécifiques au pilote. L'exemple suivant montre un fichier `config.toml` où la limite de consommation de chaque build est définie à 50 Go.

```toml
[runners.docker]
  [runners.docker.volume_driver_ops]
      "size" = "50G"
```

## Utiliser des périphériques hôtes {#using-host-devices}

{{< history >}}

- [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/6208) dans GitLab 17.10.

{{< /history >}}

Vous pouvez exposer des périphériques matériels de l'hôte GitLab Runner au conteneur qui exécute le job. Pour ce faire, configurez les options `devices` et `services_devices` du runner.

- Pour exposer des périphériques aux conteneurs `build` et [helper](../configuration/advanced-configuration.md#helper-image), utilisez l'option `devices`.
- Pour exposer des périphériques aux conteneurs de services, utilisez l'option `services_devices`. Pour restreindre l'accès aux périphériques d'un conteneur de service à des images spécifiques, utilisez des noms d'images exacts ou des modèles glob. Cette action empêche l'accès direct aux périphériques du système hôte.

Pour plus d'informations sur l'accès aux périphériques, consultez la [documentation Docker](https://docs.docker.com/reference/cli/docker/container/run/#device).

### Exemple de conteneur de build {#build-container-example}

Dans cet exemple, la section `config.toml` expose `/dev/bus/usb` aux conteneurs de build. Cette configuration permet aux pipelines d'accéder aux périphériques USB connectés à la machine hôte, tels que les smartphones Android contrôlés via [Android Debug Bridge (`adb`)](https://developer.android.com/tools/adb).

Étant donné que les conteneurs de jobs de build peuvent accéder directement aux périphériques USB de l'hôte, les exécutions simultanées de pipelines peuvent entrer en conflit lors de l'accès au même matériel. Pour éviter ces conflits, utilisez [`resource_group`](https://docs.gitlab.com/ci/yaml/#resource_group).

```toml
[[runners]]
  name = "hardware-runner"
  url = "https://gitlab.com"
  token = "__REDACTED__"
  executor = "docker"
  [runners.docker]
    # All job containers may access the host device
    devices = ["/dev/bus/usb"]
```

### Exemple de registre privé {#private-registry-example}

Cet exemple montre comment exposer les périphériques `/dev/kvm` et `/dev/dri` aux images de conteneurs d'un registre Docker privé. Ces périphériques sont couramment utilisés pour la virtualisation et le rendu accélérés par matériel. Pour atténuer les risques liés à l'accès direct des utilisateurs aux ressources matérielles, limitez l'accès aux périphériques aux images de confiance dans l'espace de nommage `myregistry:5000/emulator/*` :

```toml
[runners.docker]
  [runners.docker.services_devices]
    # Only images from an internal registry may access the host devices
    "myregistry:5000/emulator/*" = ["/dev/kvm", "/dev/dri"]
```

> [!warning]
> Le nom d'image `**/*` peut exposer des périphériques à n'importe quelle image.

## Configurer les répertoires pour le build et le cache du conteneur {#configure-directories-for-the-container-build-and-cache}

Pour définir où les données sont stockées dans le conteneur, configurez les répertoires `/builds` et `/cache` dans la section `[[runners]]` du fichier `config.toml`.

Si vous modifiez le chemin de stockage `/cache`, pour marquer le chemin comme persistant, vous devez le définir dans `volumes = ["/my/cache/"]`, sous la section `[runners.docker]` du fichier `config.toml`.

Par défaut, l'exécuteur Docker stocke les builds et les caches dans les répertoires suivants :

- Builds dans `/builds/<namespace>/<project-name>`
- Caches dans `/cache` à l'intérieur du conteneur.

## Vider le cache Docker {#clear-the-docker-cache}

Utilisez [`clear-docker-cache`](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/packaging/root/usr/share/gitlab-runner/clear-docker-cache) pour supprimer les conteneurs et volumes inutilisés créés par le runner.

Pour obtenir la liste des options, exécutez le script avec l'option `help` :

```shell
clear-docker-cache help
```

L'option par défaut est `prune-volumes`, qui supprime tous les conteneurs inutilisés (orphelins et non référencés) et les volumes.

Pour gérer efficacement le stockage du cache, vous devez :

- Exécuter `clear-docker-cache` avec `cron` régulièrement (par exemple, une fois par semaine).
- Conserver quelques conteneurs récents dans le cache pour les performances tout en récupérant de l'espace disque.

La variable d'environnement `FILTER_FLAG` contrôle les objets qui sont élagués. Pour des exemples d'utilisation, consultez la documentation [Docker image prune](https://docs.docker.com/reference/cli/docker/image/prune/#filter).

## Vider les images de build Docker {#clear-docker-build-images}

Le script [`clear-docker-cache`](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/packaging/root/usr/share/gitlab-runner/clear-docker-cache) ne supprime pas les images Docker car elles ne sont pas taguées par GitLab Runner.

Pour vider les images de build Docker :

1. Confirmez l'espace disque récupérable :

   ```shell
   clear-docker-cache space

   Show docker disk usage
   ----------------------

   TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE
   Images          14        9         1.306GB   545.8MB (41%)
   Containers      19        18        115kB     0B (0%)
   Local Volumes   0         0         0B        0B
   Build Cache     0         0         0B        0B
   ```

1. Pour supprimer tous les conteneurs, réseaux, images (orphelins et non référencés) et volumes non tagués inutilisés, exécutez [`docker system prune`](https://docs.docker.com/reference/cli/docker/system/prune/).

## Stockage persistant {#persistent-storage}

L'exécuteur Docker fournit un stockage persistant lorsqu'il exécute des conteneurs. Tous les répertoires définis dans `volumes =` sont persistants entre les builds.

La directive `volumes` prend en charge les types de stockage suivants :

- Pour le stockage dynamique, utilisez `<path>`. Le chemin `<path>` est persistant entre les exécutions successives du même job concurrent pour ce projet. Si vous ne définissez pas `runners.docker.cache_dir`, les données persistent dans les volumes Docker. Sinon, elles persistent dans le répertoire configuré sur l'hôte (monté dans le conteneur de build).

  Noms de volumes pour le stockage persistant basé sur les volumes :

  - Pour GitLab Runner antérieur à la version 18.4.0 : `runner-<short-token>-project-<project-id>-concurrent-<concurrency-id>-cache-<md5-of-path>`
  - Pour GitLab Runner 18.4.0 et versions ultérieures : `runner-<runner-id-hash>-cache-<md5-of-path><protection>`

    Les données qui ne sont plus lisibles par l'utilisateur dans le nom du volume sont déplacées vers les labels du volume.

  Répertoires hôtes pour le stockage persistant basé sur l'hôte :

  - Pour GitLab Runner antérieur à la version 18.4.0 : `<cache-dir>/runner-<short-token>-project-<project-id>-concurrent-<concurrency-id>/<md5-of-path>`
  - Pour GitLab Runner 18.4.0 et versions ultérieures : `<cache-dir>/runner-<runner-id-hash>/<md5-of-path><protection>`

  Description des parties variables :

  - `<short-token>` :  La version abrégée du token du runner (8 premières lettres)
  - `<project-id>` :  L'identifiant du projet GitLab
  - `<concurrency-id>` :  L'index du runner dans la liste de tous les runners qui exécutent un build pour le même projet simultanément (accessible via la [variable prédéfinie](https://docs.gitlab.com/ci/variables/predefined_variables/) `CI_CONCURRENT_PROJECT_ID`).
  - `<md5-of-path>` :  La somme MD5 du chemin dans le conteneur
  - `<runner-id-hash>` :  Le hash pour les données suivantes :
    - Token du runner
    - ID système du runner
    - `<project-id>`
    - `<concurrency-id>`
  - `<protection>` :  La valeur est vide pour les builds sur les branches non protégées, et `-protected` pour les builds sur les branches protégées
  - `<cache-dir>` :  La configuration dans `runners.docker.cache_dir`
- Pour le stockage lié à l'hôte, utilisez `<host-path>:<path>[:<mode>]`. GitLab Runner lie `<path>` à `<host-path>` sur le système hôte. L'option `<mode>` spécifie si ce stockage est en lecture seule ou en lecture-écriture (par défaut).

> [!warning]
> Dans GitLab Runner 18.4 et versions ultérieures, le nommage des sources pour le stockage dynamique (voir ci-dessus) a changé à la fois pour le stockage persistant basé sur les volumes Docker et sur les répertoires hôtes. Lorsque vous effectuez une mise à niveau vers la version 18.4.0, GitLab Runner ignore les données mises en cache des versions précédentes du runner et crée un nouveau stockage dynamique à la demande, soit via de nouveaux volumes Docker, soit via de nouveaux répertoires hôtes.
>
> Le stockage lié à l'hôte (avec une configuration `<host-path>`), contrairement au stockage dynamique, n'est pas affecté.

### Stockage persistant pour les builds {#persistent-storage-for-builds}

Si vous configurez le répertoire `/builds` en tant que stockage lié à l'hôte, vos builds sont stockés dans : `/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`, où :

- `<short-token>` est une version abrégée du token du runner (8 premières lettres).
- `<concurrent-id>` est l'index du runner dans la liste de tous les runners qui exécutent un build pour le même projet simultanément (accessible via la [variable prédéfinie](https://docs.gitlab.com/ci/variables/predefined_variables/) `CI_CONCURRENT_PROJECT_ID`).

## Mode IPC {#ipc-mode}

L'exécuteur Docker prend en charge le partage de l'espace de noms IPC des conteneurs avec d'autres emplacements. Cela correspond à l'indicateur `docker run --ipc`. Plus de détails sur les [paramètres IPC dans la documentation Docker](https://docs.docker.com/engine/containers/run/#ipc-settings---ipc)

## Mode privilégié {#privileged-mode}

L'exécuteur Docker prend en charge plusieurs options qui permettent d'affiner le réglage du conteneur de build. L'une de ces options est le [mode `privileged`](https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities).

### Utiliser Docker-in-Docker avec le mode privilégié {#use-docker-in-docker-with-privileged-mode}

L'indicateur `privileged` configuré est transmis au conteneur de build et à tous les services. Avec cet indicateur, vous pouvez utiliser l'approche Docker-in-Docker.

Tout d'abord, configurez votre runner (`config.toml`) pour qu'il s'exécute en mode `privileged` :

```toml
[[runners]]
  executor = "docker"
  [runners.docker]
    privileged = true
```

Ensuite, modifiez votre script de build (`.gitlab-ci.yml`) pour utiliser le conteneur Docker-in-Docker :

```yaml
image: docker:git
services:
- docker:dind

build:
  script:
  - docker build -t my-image .
  - docker push my-image
```

> [!warning]
> Les conteneurs qui s'exécutent en mode privilégié présentent des risques de sécurité. Lorsque vos conteneurs s'exécutent en mode privilégié, vous désactivez les mécanismes de sécurité du conteneur et exposez votre hôte à une élévation de privilèges. L'exécution de conteneurs en mode privilégié peut entraîner une évasion de conteneur. Pour plus d'informations, consultez la documentation Docker sur les [privilèges d'exécution et les capacités Linux](https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities).

Vous pourriez avoir besoin de [configurer Docker in Docker avec TLS, ou de désactiver TLS](https://docs.gitlab.com/ci/docker/using_docker_build/#use-the-docker-executor-with-docker-in-docker) pour éviter une erreur similaire à la suivante :

```plaintext
Cannot connect to the Docker daemon at tcp://docker:2375. Is the docker daemon running?
```

### Utiliser Docker-in-Docker rootless avec le mode privilégié restreint {#use-rootless-docker-in-docker-with-restricted-privileged-mode}

Dans cette version, seules les images Docker-in-Docker rootless sont autorisées à s'exécuter comme services en mode privilégié.

Les paramètres de configuration `services_privileged` et `allowed_privileged_services` limitent les conteneurs autorisés à s'exécuter en mode privilégié.

Pour utiliser Docker-in-Docker rootless avec le mode privilégié restreint :

1. Dans le fichier `config.toml`, configurez le runner pour utiliser `services_privileged` et `allowed_privileged_services` :

   ```toml
   [[runners]]
     executor = "docker"
     [runners.docker]
       services_privileged = true
       allowed_privileged_services = ["docker.io/library/docker:*-dind-rootless", "docker.io/library/docker:dind-rootless", "docker:*-dind-rootless", "docker:dind-rootless"]
   ```

1. Dans `.gitlab-ci.yml`, modifiez votre script de build pour utiliser le conteneur Docker-in-Docker rootless :

   ```yaml
   image: docker:git
   services:
   - docker:dind-rootless

   build:
     script:
     - docker build -t my-image .
     - docker push my-image
   ```

Seules les images Docker-in-Docker rootless que vous listez dans `allowed_privileged_services` sont autorisées à s'exécuter en mode privilégié. Tous les autres conteneurs pour les jobs et les services s'exécutent en mode non privilégié.

Étant donné qu'ils s'exécutent en tant que non-root, c'est _presque sûr_ de les utiliser avec des images en mode privilégié comme Docker-in-Docker rootless ou BuildKit rootless.

Pour plus d'informations sur les problèmes de sécurité, consultez [Risques de sécurité pour les exécuteurs Docker](../security/_index.md#usage-of-docker-executor).

## Configurer un ENTRYPOINT Docker {#configure-a-docker-entrypoint}

Par défaut, l'exécuteur Docker ne remplace pas le [`ENTRYPOINT` d'une image Docker](https://docs.docker.com/engine/containers/run/#entrypoint-default-command-to-execute-at-runtime). Il transmet `sh` ou `bash` en tant que [`COMMAND`](https://docs.docker.com/engine/containers/run/#cmd-default-command-or-options) pour démarrer un conteneur qui exécute le script du job.

Pour s'assurer qu'un job peut s'exécuter, son image Docker doit :

- Fournir `sh` ou `bash` et `grep`
- Définir un `ENTRYPOINT` qui démarre un shell lorsque `sh`/`bash` est passé comme argument

L'exécuteur Docker exécute le conteneur du job avec l'équivalent de la commande suivante :

```shell
docker run <image> sh -c "echo 'It works!'" # or bash
```

Si votre image Docker ne prend pas en charge ce mécanisme, vous pouvez [remplacer l'ENTRYPOINT de l'image](https://docs.gitlab.com/ci/yaml/#imageentrypoint) dans la configuration du projet comme suit :

```yaml
# Equivalent of
# docker run --entrypoint "" <image> sh -c "echo 'It works!'"
image:
  name: my-image
  entrypoint: [""]
```

Pour plus d'informations, consultez [Remplacer l'Entrypoint d'une image](https://docs.gitlab.com/ci/docker/using_docker_images/#override-the-entrypoint-of-an-image) et [Comment `CMD` et `ENTRYPOINT` interagissent dans Docker](https://docs.docker.com/reference/dockerfile/#understand-how-cmd-and-entrypoint-interact).

### Script de job en tant qu'ENTRYPOINT {#job-script-as-entrypoint}

Vous pouvez utiliser `ENTRYPOINT` pour créer une image Docker qui exécute le script de build dans un environnement personnalisé, ou en mode sécurisé.

Par exemple, vous pouvez créer une image Docker qui utilise un `ENTRYPOINT` qui n'exécute pas le script de build. Au lieu de cela, l'image Docker exécute un ensemble prédéfini de commandes pour construire l'image Docker depuis votre répertoire. Vous exécutez le conteneur de build en [mode privilégié](#privileged-mode) et sécurisez l'environnement de build du runner.

1. Créez un nouveau Dockerfile :

   ```dockerfile
   FROM docker:dind
   ADD / /entrypoint.sh
   ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
   ```

1. Créez un script bash (`entrypoint.sh`) qui est utilisé comme `ENTRYPOINT` :

   ```shell
   #!/bin/sh

   dind docker daemon
       --host=unix:///var/run/docker.sock \
       --host=tcp://0.0.0.0:2375 \
       --storage-driver=vf &

   docker build -t "$BUILD_IMAGE" .
   docker push "$BUILD_IMAGE"
   ```

1. Poussez l'image vers le registre Docker.
1. Exécutez l'exécuteur Docker en mode `privileged`. Dans `config.toml`, définissez :

   ```toml
   [[runners]]
     executor = "docker"
     [runners.docker]
       privileged = true
   ```

1. Dans votre projet, utilisez le fichier `.gitlab-ci.yml` suivant :

   ```yaml
   variables:
     BUILD_IMAGE: my.image
   build:
     image: my/docker-build:image
     script:
     - Dummy Script
   ```

## Utiliser Podman pour exécuter des commandes Docker {#use-podman-to-run-docker-commands}

Si GitLab Runner est installé sur Linux, vos jobs peuvent utiliser Podman pour remplacer Docker comme runtime de conteneur dans l'exécuteur Docker.

Prérequis :

- [Podman](https://podman.io/) v4.2.0 ou version ultérieure.
- Pour exécuter des [services](#services) avec Podman comme exécuteur, activez le [feature flag `FF_NETWORK_PER_BUILD`](#create-a-network-for-each-job). Les [liens de conteneurs Docker](https://docs.docker.com/engine/network/links/) sont hérités et ne sont pas pris en charge par [Podman](https://podman.io/). Pour les services qui créent un alias réseau, vous devez installer le paquet `podman-plugins`.

> [!note]
> Podman utilise `aardvark-dns` comme serveur DNS pour les conteneurs. Les versions `aardvark-dns` 1.10.0 et antérieures provoquent des échecs sporadiques de résolution DNS dans les jobs CI/CD. Assurez-vous d'avoir installé une version plus récente. Pour plus d'informations, consultez le [problème GitHub 389](https://github.com/containers/aardvark-dns/issues/389).

1. Sur votre hôte Linux, installez GitLab Runner. Si vous avez installé GitLab Runner à l'aide du gestionnaire de paquets de votre système, il crée automatiquement un utilisateur `gitlab-runner`.
1. Connectez-vous en tant qu'utilisateur qui exécute GitLab Runner. Vous devez le faire d'une manière qui ne contourne pas [`pam_systemd`](https://www.freedesktop.org/software/systemd/man/latest/pam_systemd.html). Vous pouvez utiliser SSH avec l'utilisateur approprié. Cela vous permet d'exécuter `systemctl` en tant que cet utilisateur.
1. Assurez-vous que votre système remplit les prérequis pour [une configuration Podman rootless](https://github.com/containers/podman/blob/main/docs/tutorials/rootless_tutorial.md). Plus précisément, assurez-vous que votre utilisateur dispose d'[entrées correctes dans `/etc/subuid` et `/etc/subgid`](https://github.com/containers/podman/blob/main/docs/tutorials/rootless_tutorial.md#etcsubuid-and-etcsubgid-configuration).
1. Sur l'hôte Linux, [installez Podman](https://podman.io/getting-started/installation).
1. Activez et démarrez le socket Podman :

   ```shell
   systemctl --user --now enable podman.socket
   ```

1. Vérifiez que le socket Podman est à l'écoute :

   ```shell
   systemctl status --user podman.socket
   ```

1. Copiez la chaîne du socket dans la clé `Listen` par laquelle l'API Podman est accessible.
1. Assurez-vous que le socket Podman reste disponible après la déconnexion de l'utilisateur GitLab Runner :

   ```shell
   sudo loginctl enable-linger gitlab-runner
   ```

1. Modifiez le fichier `config.toml` de GitLab Runner et ajoutez la valeur du socket à l'entrée hôte dans la section `[runners.docker]`. Par exemple :

   ```toml
   [[runners]]
     name = "podman-test-runner-2025-06-07"
     url = "https://gitlab.com"
     token = "TOKEN"
     executor = "docker"
     [runners.docker]
       host = "unix:///run/user/1012/podman/podman.sock"
       tls_verify = false
       image = "quay.io/podman/stable"
       privileged = false
   ```

   > [!note]
   > Définissez `privileged = false` pour une utilisation standard de Podman. Définissez `privileged = true` uniquement si vous devez exécuter des [services Docker-in-Docker](#use-docker-in-docker-with-privileged-mode) dans vos jobs.

### Utiliser Podman pour créer des images de conteneurs depuis un Dockerfile {#use-podman-to-build-container-images-from-a-dockerfile}

L'exemple suivant utilise Podman pour créer une image de conteneur et pousser l'image vers le registre de conteneurs GitLab.

L'image de conteneur par défaut dans le fichier `config.toml` du runner est définie sur `quay.io/podman/stable`, afin que le job CI utilise cette image pour exécuter les commandes incluses.

```yaml
variables:
  IMAGE_TAG: $CI_REGISTRY_IMAGE:$CI_COMMIT_REF_SLUG

before_script:
  - podman login -u "$CI_REGISTRY_USER" -p "$CI_REGISTRY_PASSWORD" $CI_REGISTRY

oci-container-build:
  stage: build
  script:
    - podman build -t $IMAGE_TAG .
    - podman push $IMAGE_TAG
  when: manual
```

### Utiliser Buildah pour créer des images de conteneurs depuis un Dockerfile {#use-buildah-to-build-container-images-from-a-dockerfile}

L'exemple suivant montre comment utiliser Buildah pour créer une image de conteneur et pousser l'image vers le registre de conteneurs GitLab.

```yaml
image: quay.io/buildah/stable

variables:
  IMAGE_TAG: $CI_REGISTRY_IMAGE:$CI_COMMIT_REF_SLUG

before_script:
  - buildah login -u "$CI_REGISTRY_USER" -p "$CI_REGISTRY_PASSWORD" $CI_REGISTRY

oci-container-build:
  stage: build
  script:
    - buildah bud -t $IMAGE_TAG .
    - buildah push $IMAGE_TAG
  when: manual
```

### Problèmes connus {#known-issues}

Contrairement à Docker, Podman applique les politiques SELinux par défaut. Bien que de nombreux pipelines s'exécutent sans problème, certains peuvent échouer en raison de l'héritage du contexte SELinux lorsque des outils utilisent des répertoires temporaires.

Par exemple, le pipeline suivant échoue avec Podman :

```yaml
testing:
  image: alpine:3.20
  script:
    - apk add --no-cache python3 py3-pip
    - pip3 install --target $CI_PROJECT_DIR requests==2.28.2
```

L'échec se produit car pip utilise `/tmp` comme répertoire de travail. Les fichiers créés dans `/tmp` héritent de son contexte SELinux, ce qui empêche le conteneur de modifier ces fichiers lorsqu'ils sont déplacés vers `$CI_PROJECT_DIR`.

**Solution :** Ajoutez `/tmp` aux volumes dans le fichier `config.toml` du runner sous la section `runners.docker` :

```toml
[[runners]]
  [runners.docker]
    volumes = ["/cache", "/tmp"]
```

Cet ajout garantit des contextes SELinux cohérents dans les répertoires montés.

#### Résolution des problèmes SELinux {#troubleshooting-selinux-issues}

D'autres problèmes Podman/SELinux peuvent nécessiter un dépannage supplémentaire pour identifier les modifications de configuration nécessaires.

Pour tester si un problème de runner Podman est lié à SELinux, ajoutez temporairement la directive suivante au fichier `config.toml` du runner sous la section `runners.docker` :

```toml
[[runners]]
  [runners.docker]
    security_opt = ["label:disable"]
```

> [!warning]
> Cet ajout désactive l'application de SELinux dans le conteneur (ce qui est le comportement par défaut de Docker). Utilisez cette configuration uniquement à des fins de test et non comme solution permanente, car elle peut avoir des implications en matière de sécurité.

#### Configurer SELinux MCS {#configure-selinux-mcs}

Si SELinux bloque certaines opérations d'écriture (comme la réinitialisation d'un dépôt Git existant), vous pouvez forcer un niveau de sécurité multi-catégories (MCS) sur tous les conteneurs lancés par le runner :

```toml
[[runners]]
  [runners.docker]
    security_opt = ["label=level:s0:c1000"]
```

Cette option ne désactive pas SELinux, mais définit le niveau MCS du conteneur. Cette approche est plus sûre que l'utilisation de `label:disable`.

> [!warning]
> Plusieurs conteneurs utilisant la même catégorie MCS peuvent accéder aux mêmes fichiers tagués avec cette catégorie.

## Spécifier l'utilisateur qui exécute le job {#specify-which-user-runs-the-job}

Par défaut, le runner exécute les jobs en tant qu'utilisateur `root` dans le conteneur. Pour spécifier un utilisateur différent, non-root, pour exécuter le job, utilisez la directive `USER` dans le Dockerfile de l'image Docker.

```dockerfile
FROM amazonlinux
RUN ["yum", "install", "-y", "nginx"]
RUN ["useradd", "www"]
USER "www"
CMD ["/bin/bash"]
```

Lorsque vous utilisez cette image Docker pour exécuter votre job, il s'exécute en tant qu'utilisateur spécifié :

```yaml
build:
  image: my/docker-build:image
  script:
  - whoami   # www
```

## Configurer la façon dont les runners récupèrent les images {#configure-how-runners-pull-images}

Configurez la politique de récupération dans le fichier `config.toml` pour définir la façon dont les runners récupèrent les images Docker depuis les registres. Vous pouvez définir une politique unique, [une liste de politiques](#set-multiple-pull-policies) ou [autoriser des politiques de récupération spécifiques](#allow-docker-pull-policies).

Utilisez les valeurs suivantes pour `pull_policy` :

- [`always`](#set-the-always-pull-policy) :  Par défaut. Récupérer une image même si une image locale existe. Cette politique de récupération ne s'applique pas aux images spécifiées par leur `SHA256` qui existent déjà sur le disque.
- [`if-not-present`](#set-the-if-not-present-pull-policy) :  Récupérer une image uniquement lorsqu'une version locale n'existe pas.
- [`never`](#set-the-never-pull-policy) :  Ne jamais récupérer une image et utiliser uniquement les images locales.

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "always" # available: always, if-not-present, never
```

### Définir la politique de récupération `always` {#set-the-always-pull-policy}

L'option `always`, qui est activée par défaut, initie toujours une récupération avant de créer le conteneur. Cette option garantit que l'image est à jour et vous empêche d'utiliser des images obsolètes même si une image locale existe.

Utilisez cette politique de récupération si :

- Les runners doivent toujours récupérer les images les plus récentes.
- Les runners sont accessibles publiquement et configurés pour la [mise à l'échelle automatique](../configuration/autoscale.md) ou en tant que runner d'instance dans votre instance GitLab.

**N’utilisez pas** cette politique si les runners doivent utiliser des images stockées localement.

Définissez `always` comme `pull policy` dans le fichier `config.toml` :

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "always"
```

### Définir la politique de récupération `if-not-present` {#set-the-if-not-present-pull-policy}

Lorsque vous définissez la politique de récupération sur `if-not-present`, le runner vérifie d'abord si une image locale existe. S'il n'y a pas d'image locale, le runner récupère une image depuis le registre.

Utilisez la politique `if-not-present` pour :

- Utiliser des images locales mais aussi récupérer des images si une image locale n'existe pas.
- Réduire le temps que les runners consacrent à l'analyse des différences de couches d'images pour les images volumineuses et rarement mises à jour. Dans ce cas, vous devez supprimer manuellement l'image régulièrement du store Docker Engine local pour forcer la mise à jour de l'image.

**N’utilisez pas** cette politique :

- Pour les runners d'instance où différents utilisateurs du runner peuvent avoir accès à des images privées. Pour plus d'informations sur les problèmes de sécurité, consultez [Utilisation d'images Docker privées avec la politique de récupération if-not-present](../security/_index.md#usage-of-private-docker-images-with-if-not-present-pull-policy).
- Si les jobs sont fréquemment mis à jour et doivent s'exécuter dans la version d'image la plus récente. Cela peut entraîner une réduction de la charge réseau qui l'emporte sur la valeur de la suppression fréquente des images locales.

Définissez la politique `if-not-present` dans le fichier `config.toml` :

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "if-not-present"
```

### Définir la politique de récupération `never` {#set-the-never-pull-policy}

Prérequis :

- Les images locales doivent contenir un Docker Engine installé et une copie locale des images utilisées.

Lorsque vous définissez la politique de récupération sur `never`, la récupération d'images est désactivée. Les utilisateurs ne peuvent utiliser que les images qui ont été récupérées manuellement sur l'hôte Docker où le runner s'exécute.

Utilisez la politique de récupération `never` :

- Pour contrôler les images utilisées par les utilisateurs du runner.
- Pour les runners privés dédiés à un projet qui ne peut utiliser que des images spécifiques non disponibles publiquement sur aucun registre.

**N’utilisez pas** la politique de récupération `never` pour les exécuteurs Docker à [mise à l'échelle automatique](../configuration/autoscale.md). La politique de récupération `never` n'est utilisable que lors de l'utilisation d'images d'instances cloud prédéfinies pour le fournisseur cloud choisi.

Définissez la politique `never` dans le fichier `config.toml` :

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "never"
```

### Définir plusieurs politiques de récupération {#set-multiple-pull-policies}

Vous pouvez lister plusieurs politiques de récupération à exécuter en cas d'échec d'une récupération. Le runner traite les politiques de récupération dans l'ordre indiqué jusqu'à ce qu'une tentative de récupération réussisse ou que la liste soit épuisée. Par exemple, si un runner utilise la politique de récupération `always` et que le registre n'est pas disponible, vous pouvez ajouter `if-not-present` comme deuxième politique de récupération. Cette configuration permet au runner d'utiliser une image Docker mise en cache localement.

Pour plus d'informations sur les implications de sécurité de cette politique de récupération, consultez [Utilisation d'images Docker privées avec la politique de récupération if-not-present](../security/_index.md#usage-of-private-docker-images-with-if-not-present-pull-policy).

Pour définir plusieurs politiques de récupération, ajoutez-les sous forme de liste dans le fichier `config.toml` :

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = ["always", "if-not-present"]
```

### Autoriser les politiques de récupération Docker {#allow-docker-pull-policies}

Dans le fichier `.gitlab-ci.yml`, vous pouvez spécifier une politique de récupération. Cette politique détermine la façon dont un job CI/CD récupère les images.

Pour restreindre les politiques de récupération pouvant être utilisées parmi celles spécifiées dans le fichier `.gitlab-ci.yml`, utilisez `allowed_pull_policies`.

Par exemple, pour autoriser uniquement les politiques de récupération `always` et `if-not-present`, ajoutez-les dans le fichier `config.toml` :

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_pull_policies = ["always", "if-not-present"]
```

- Si vous ne spécifiez pas `allowed_pull_policies`, la liste correspond aux valeurs spécifiées dans le mot-clé `pull_policy`.
- Si vous ne spécifiez pas `pull_policy`, la valeur par défaut est `always`.
- Le job utilise uniquement les politiques de récupération répertoriées à la fois dans `pull_policy` et `allowed_pull_policies`. La politique de récupération effective est déterminée en comparant les politiques spécifiées dans le [mot-clé `pull_policy`](#configure-how-runners-pull-images) et `allowed_pull_policies`. GitLab utilise l'[intersection](https://en.wikipedia.org/wiki/Intersection_(set_theory)) de ces deux listes de politiques. Par exemple, si `pull_policy` est `["always", "if-not-present"]` et que `allowed_pull_policies` est `["if-not-present"]`, alors le job utilise uniquement `if-not-present` car c'est la seule politique de récupération définie dans les deux listes.
- Le mot-clé `pull_policy` existant doit inclure au moins une politique de récupération spécifiée dans `allowed_pull_policies`. Le job échoue si aucune des valeurs de `pull_policy` ne correspond à `allowed_pull_policies`.

### Messages d'erreur de récupération d'image {#image-pull-error-messages}

| Message d'erreur                                                                                                                                                                                                                                                               | Description |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------|
| `Pulling docker image registry.tld/my/image:latest ... ERROR: Build failed: Error: image registry.tld/my/image:latest not found`                                                                                                                                            | Le runner ne trouve pas l'image. S'affiche lorsque la politique de récupération `always` est définie |
| `Pulling docker image local_image:latest ... ERROR: Build failed: Error: image local_image:latest not found`                                                                                                                                                                | L'image a été créée localement et n'existe dans aucun registre Docker public ou par défaut. S'affiche lorsque la politique de récupération `always` est définie. |
| `Pulling docker image registry.tld/my/image:latest ... WARNING: Cannot pull the latest version of image registry.tld/my/image:latest : Error: image registry.tld/my/image:latest not found WARNING: Locally found image will be used instead.`                              | Le runner a utilisé une image locale au lieu de récupérer une image. |
| `Pulling docker image local_image:latest ... ERROR: Build failed: Error: image local_image:latest not found`                                                                                                                                                                | L'image est introuvable localement. S'affiche lorsque la politique de récupération `never` est définie. |
| `WARNING: Failed to pull image with policy "always": Error response from daemon: received unexpected HTTP status: 502 Bad Gateway (docker.go:143:0s) Attempt #2: Trying "if-not-present" pull policy Using locally found image version due to "if-not-present" pull policy` | Le runner n'a pas réussi à récupérer une image et tente de récupérer une image en utilisant la prochaine politique de récupération listée. S'affiche lorsque plusieurs politiques de récupération sont définies. |

## Relancer une récupération échouée {#retry-a-failed-pull}

Pour configurer un runner afin de relancer une récupération d'image échouée, spécifiez la même politique plus d'une fois dans le fichier `config.toml`.

Par exemple, cette configuration relance la récupération une fois :

```toml
[runners.docker]
  pull_policy = ["always", "always"]
```

Ce paramètre est similaire à [la directive `retry`](https://docs.gitlab.com/ci/yaml/#retry) dans les fichiers `.gitlab-ci.yml` des projets individuels, mais ne prend effet que si la récupération Docker échoue initialement.

## Utiliser des conteneurs Windows {#use-windows-containers}

Pour utiliser des conteneurs Windows avec l'exécuteur Docker, notez les informations suivantes sur les limitations, les versions Windows prises en charge, la configuration d'un exécuteur Docker Windows et les images d'aide Windows.

### Versions Windows prises en charge {#supported-windows-versions}

GitLab Runner ne prend en charge que les versions suivantes de Windows, conformément à notre [cycle de vie de support pour Windows](../install/support-policy.md#windows-version-support) :

- Windows Server 2025 LTSC (24H2)
- Windows Server 2022 LTSC (21H2)
- Windows Server 2019 LTSC (1809)

Les conteneurs Windows prennent en charge la rétrocompatibilité en fonction du système d'exploitation hôte et du mode d'isolation. Les hôtes plus récents peuvent exécuter des images de conteneurs plus anciennes. Pour plus de détails sur la compatibilité, consultez les [directives de compatibilité des versions de conteneurs Windows Microsoft](https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/version-compatibility).

Vous pouvez utiliser diverses images de base Windows, notamment `Server Core`, `Nano Server`, `Server` et `Windows`. Par exemple, utilisez les images [`Windows Server Core`](https://hub.docker.com/r/microsoft/windows-servercore) avec leurs versions de système d'exploitation compatibles :

- `mcr.microsoft.com/windows/servercore:ltsc2025`
- `mcr.microsoft.com/windows/servercore:ltsc2025-amd64`
- `mcr.microsoft.com/windows/servercore:ltsc2022`
- `mcr.microsoft.com/windows/servercore:ltsc2022-amd64`
- `mcr.microsoft.com/windows/servercore:1809`
- `mcr.microsoft.com/windows/servercore:1809-amd64`
- `mcr.microsoft.com/windows/servercore:ltsc2019`

### Versions Docker prises en charge {#supported-docker-versions}

GitLab Runner utilise Docker pour détecter la version de Windows Server en cours d'exécution. Par conséquent, un Windows Server exécutant GitLab Runner doit utiliser une version récente de Docker.

Une version connue de Docker qui ne fonctionne pas avec GitLab Runner est `Docker 17.06`. Docker n'identifie pas la version de Windows Server, ce qui entraîne l'erreur suivante :

```plaintext
unsupported Windows Version: Windows Server Datacenter
```

[En savoir plus sur le dépannage de ce problème](../install/windows.md#docker-executor-unsupported-windows-version).

### Configurer un exécuteur Docker Windows {#configure-a-windows-docker-executor}

> [!note]
> Lorsqu'un runner est enregistré avec `c:\\cache` comme répertoire source lors du passage de la variable d'environnement `--docker-volumes` ou `DOCKER_VOLUMES`, il existe un [problème connu](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4312).

Voici un exemple de configuration pour un exécuteur Docker exécutant Windows.

```toml
[[runners]]
  name = "windows-docker-2019"
  url = "https://gitlab.com/"
  token = "xxxxxxx"
  executor = "docker-windows"
  [runners.docker]
    image = "mcr.microsoft.com/windows/servercore:1809_amd64"
    volumes = ["c:\\cache"]
```

Pour d'autres options de configuration de l'exécuteur Docker, consultez la section [configuration avancée](../configuration/advanced-configuration.md#the-runnersdocker-section).

### Images d'aide Windows {#windows-helper-images}

GitLab Runner fournit plusieurs images d'aide adaptées aux différentes versions de Windows et aux exigences de PowerShell. Variantes disponibles :

- `gitlab/gitlab-runner-helper:x86_64-vXYZ-nanoserver21H2`
- `gitlab/gitlab-runner-helper:x86_64-vXYZ-servercore21H2`
- `gitlab/gitlab-runner-helper:x86_64-vXYZ-nanoserver1809`
- `gitlab/gitlab-runner-helper:x86_64-vXYZ-servercore1809`

> [!note]
> En raison de la compatibilité ascendante des conteneurs Windows, Windows Server 2025 (24H2) peut utiliser les images d'aide 21H2 (Windows Server 2022).

Choisissez votre image d'aide en fonction de vos besoins en matière de shell. L'image `servercore` est l'image par défaut et prend en charge `PowerShell` et `Pwsh`. Pour les conteneurs qui n'utilisent que `pwsh`, utilisez l'image `nanoserver` plus légère.

### Services {#services}

Vous pouvez utiliser les [services](https://docs.gitlab.com/ci/services/) en activant [un réseau pour chaque job](#create-a-network-for-each-job).

### Problèmes connus liés à l'exécuteur Docker sous Windows {#known-issues-with-docker-executor-on-windows}

Voici quelques limitations liées à l'utilisation de conteneurs Windows avec l'exécuteur Docker :

- Docker-in-Docker n'est pas pris en charge, car il [n'est pas pris en charge](https://github.com/docker-library/docker/issues/49) par Docker lui-même.
- Le montage de périphériques hôtes n'est pas pris en charge.
- Lors du montage d'un répertoire de volume, celui-ci doit exister, sinon Docker échoue à démarrer le conteneur. Voir [\#3754](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/3754) pour plus de détails.
- L'exécuteur `docker-windows` ne peut être exécuté qu'avec GitLab Runner fonctionnant sous Windows.
- Les [conteneurs Linux sous Windows](https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/set-up-linux-containers) ne sont pas pris en charge, car ils sont encore expérimentaux. Consultez [le ticket concerné](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4373) pour plus de détails.
- En raison d'une [limitation de Docker](https://github.com/MicrosoftDocs/Virtualization-Documentation/pull/331), si la lettre de lecteur du chemin de destination n'est pas `c:`, les chemins ne sont pas pris en charge pour :

  - [`builds_dir`](../configuration/advanced-configuration.md#the-runners-section)
  - [`cache_dir`](../configuration/advanced-configuration.md#the-runners-section)
  - [`volumes`](../configuration/advanced-configuration.md#volumes-in-the-runnersdocker-section)

  Cela signifie que des valeurs telles que `f:\\cache_dir` ne sont pas prises en charge, mais que `f:` est pris en charge. Toutefois, si le chemin de destination se trouve sur le lecteur `c:`, les chemins sont également pris en charge (par exemple `c:\\cache_dir`).

  Pour configurer l'emplacement où le démon Docker conserve les images et les conteneurs, mettez à jour le paramètre `data-root` dans le fichier `daemon.json` du démon Docker.

  Pour plus d'informations, voir [Configure Docker with a configuration file](https://learn.microsoft.com/en-us/virtualization/windowscontainers/manage-docker/configure-docker-daemon#configure-docker-with-a-configuration-file).

## Intégration native de Step Runner {#native-step-runner-integration}

{{< history >}}

- [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5069) dans GitLab 17.6.0 derrière le feature flag `FF_USE_NATIVE_STEPS`, qui est désactivé par défaut.
- [Mis à jour](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5322) dans GitLab 17.9.0. GitLab Runner injecte le binaire `step-runner` dans le conteneur de build et ajuste la variable d'environnement `$PATH` en conséquence. Cette amélioration permet d'utiliser n'importe quelle image comme image de build.

{{< /history >}}

L'exécuteur Docker prend en charge l'exécution native des [étapes CI/CD](https://docs.gitlab.com/ci/steps/) en utilisant l'API `gRPC` fournie par [`step-runner`](https://gitlab.com/gitlab-org/step-runner).

Pour activer ce mode d'exécution, vous devez spécifier les jobs CI/CD en utilisant le mot-clé `run` au lieu du mot-clé hérité `script`. De plus, vous devez activer le feature flag `FF_USE_NATIVE_STEPS`. Vous pouvez activer ce feature flag au niveau du job ou du pipeline.

```yaml
step job:
  stage: test
  variables:
    FF_USE_NATIVE_STEPS: true
  image:
    name: alpine:latest
  run:
    - name: step1
      script: pwd
    - name: step2
      script: env
    - name: step3
      script: ls -Rlah ../
```

### Problèmes connus {#known-issues-1}

- Dans GitLab 17.9 et versions ultérieures, l'image de build doit avoir le package `ca-certificates` installé, sinon `step-runner` échouera à récupérer les étapes définies dans le job. Par exemple, les distributions Linux basées sur Debian n'installent pas `ca-certificates` par défaut.

- Dans les versions de GitLab antérieures à 17.9, l'image de build doit inclure un binaire `step-runner` dans `$PATH`. Pour ce faire, vous pouvez soit :

  - Créer votre propre image de build personnalisée et y inclure le binaire `step-runner`.
  - Utiliser l'image `registry.gitlab.com/gitlab-org/step-runner:v0` si elle inclut les dépendances nécessaires à l'exécution de votre job.

- L'exécution d'une étape qui exécute un conteneur Docker doit respecter les mêmes paramètres de configuration et contraintes que les `scripts` traditionnels. Par exemple, vous devez utiliser [Docker-in-Docker](#use-docker-in-docker-with-privileged-mode).
- Ce mode d'exécution ne prend pas encore en charge l'exécution de [`Github Actions`](https://gitlab.com/components/action-runner).
