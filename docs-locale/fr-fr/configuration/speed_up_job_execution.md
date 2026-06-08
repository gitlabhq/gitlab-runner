---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "Accélérer l'exécution des jobs"
---

{{< details >}}

- Niveau : Free, Premium, Ultimate
- Offre : GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Vous pouvez améliorer les performances de vos jobs en mettant en cache vos images et vos dépendances.

## Utiliser un proxy pour les conteneurs {#use-a-proxy-for-containers}

Vous pouvez accélérer le téléchargement des images Docker en utilisant :

- Le proxy de dépendances GitLab ou
- Un miroir du registre DockerHub
- D'autres solutions open source

### Proxy de dépendances GitLab {#gitlab-dependency-proxy}

Pour accéder plus rapidement aux images de conteneurs, vous pouvez [utiliser le proxy de dépendances](https://docs.gitlab.com/user/packages/dependency_proxy/) pour mettre en proxy les images de conteneurs.

### Miroir du registre Docker Hub {#docker-hub-registry-mirror}

Vous pouvez également accélérer l'accès de vos jobs aux images de conteneurs en créant un miroir de Docker Hub. Cela aboutit au [registre comme cache pull-through](https://docs.docker.com/docker-hub/image-library/mirror/). En plus d'accélérer l'exécution des jobs, un miroir peut rendre votre infrastructure plus résiliente aux pannes de Docker Hub et aux limites de débit de Docker Hub.

Lorsque le daemon Docker est [configuré pour utiliser le miroir](https://docs.docker.com/docker-hub/image-library/mirror/#configure-the-docker-daemon), il vérifie automatiquement la présence de l'image sur votre instance en cours d'exécution du miroir. Si elle n'est pas disponible, il extrait l'image du registre Docker public et la stocke localement avant de vous la remettre.

La prochaine requête pour la même image est extraite de votre registre local.

Pour plus d'informations sur son fonctionnement, consultez la [documentation de configuration du daemon Docker](https://docs.docker.com/docker-hub/image-library/mirror/#configure-the-docker-daemon).

#### Utiliser un miroir du registre Docker Hub {#use-a-docker-hub-registry-mirror}

Pour créer un miroir du registre Docker Hub :

1. Connectez-vous à une machine dédiée sur laquelle le registre de conteneurs proxy s'exécutera.
1. Assurez-vous que [Docker Engine](https://docs.docker.com/get-started/get-docker/) est installé sur cette machine.
1. Créez un nouveau registre de conteneurs :

   ```shell
   docker run -d -p 6000:5000 \
       -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
       --restart always \
       --name registry registry:2
   ```

   Vous pouvez modifier le numéro de port (`6000`) pour exposer le registre sur un port différent. Cela démarrera le serveur avec `http`. Si vous souhaitez activer TLS (`https`), suivez la [documentation officielle](https://distribution.github.io/distribution/about/configuration/#tls).

1. Vérifiez l'adresse IP du serveur :

   ```shell
   hostname --ip-address
   ```

   Vous devriez choisir l'adresse IP du réseau privé. Le réseau privé est généralement la solution la plus rapide pour la communication interne entre les machines d'un même fournisseur, comme DigitalOcean, AWS ou Azure. En général, les données transférées sur un réseau privé ne sont pas comptabilisées dans votre limite de bande passante mensuelle.

Le registre Docker Hub est accessible sous `MY_REGISTRY_IP:6000`.

Vous pouvez maintenant [configurer `config.toml`](autoscale.md#distributed-container-registry-mirroring) pour utiliser le nouveau serveur de registre.

### Autres solutions open source {#other-open-source-solutions}

- [`rpardini/docker-registry-proxy`](https://github.com/rpardini/docker-registry-proxy) peut mettre en proxy la plupart des registres de conteneurs localement, y compris le registre de conteneurs GitLab.

## Utiliser un cache distribué {#use-a-distributed-cache}

Vous pouvez accélérer le téléchargement des dépendances de langage en utilisant un [cache](https://docs.gitlab.com/ci/yaml/#cache) distribué.

Pour spécifier un cache distribué, vous configurez le serveur de cache, puis [configurez le runner pour utiliser ce serveur de cache](advanced-configuration.md#the-runnerscache-section).

Si vous utilisez la mise à l'échelle automatique, apprenez-en plus sur la [fonctionnalité de cache](autoscale.md#distributed-runners-caching) des runners distribués.

Les serveurs de cache suivants sont pris en charge :

- [AWS S3](#use-aws-s3)
- [MinIO](#use-minio) ou un autre serveur de cache compatible S3
- [Google Cloud Storage](#use-google-cloud-storage)
- [Azure Blob storage](#use-azure-blob-storage)

En savoir plus sur les [dépendances de cache et les bonnes pratiques](https://docs.gitlab.com/ci/caching/) GitLab CI/CD.

### Utiliser AWS S3 {#use-aws-s3}

Pour utiliser AWS S3 comme cache distribué, [modifiez le fichier `config.toml` du runner](advanced-configuration.md#the-runnerscaches3-section) pour pointer vers l'emplacement S3 et fournir les informations d'identification pour la connexion. Assurez-vous que le runner dispose d'un chemin réseau vers le point de terminaison S3.

Si vous utilisez un sous-réseau privé avec une passerelle NAT, pour réduire les coûts de transfert de données, vous pouvez activer un point de terminaison S3 VPC.

### Utiliser MinIO {#use-minio}

Au lieu d'utiliser AWS S3, vous pouvez créer votre propre stockage de cache.

1. Connectez-vous à une machine dédiée sur laquelle le serveur de cache s'exécutera.
1. Assurez-vous que [Docker Engine](https://docs.docker.com/get-started/get-docker/) est installé sur cette machine.
1. Démarrez [MinIO](https://www.min.io), un simple serveur compatible S3 écrit en Go :

   ```shell
   docker run -d --restart always -p 9005:9000 \
           -v /.minio:/root/.minio -v /export:/export \
           -e "MINIO_ROOT_USER=<minio_root_username>" \
           -e "MINIO_ROOT_PASSWORD=<minio_root_password>" \
           --name minio \
           minio/minio:latest server /export
   ```

   Vous pouvez modifier le port `9005` pour exposer le serveur de cache sur un port différent.

1. Vérifiez l'adresse IP du serveur :

   ```shell
   hostname --ip-address
   ```

1. Votre serveur de cache sera disponible à l'adresse `MY_CACHE_IP:9005`.
1. Créez un bucket qui sera utilisé par le runner :

   ```shell
   sudo mkdir /export/runner
   ```

   `runner` est le nom du bucket dans ce cas. Si vous choisissez un bucket différent, il sera différent. Tous les caches seront stockés dans le répertoire `/export`.

1. Utilisez les valeurs `MINIO_ROOT_USER` et `MINIO_ROOT_PASSWORD` (indiquées ci-dessus) comme clés d'accès et clés secrètes lors de la configuration de votre runner.

Vous pouvez maintenant [configurer `config.toml`](autoscale.md#distributed-runners-caching) pour utiliser le nouveau serveur de cache.

### Utiliser Google Cloud Storage {#use-google-cloud-storage}

Pour utiliser Google Cloud Platform comme cache distribué, [modifiez le fichier `config.toml` du runner](advanced-configuration.md#the-runnerscachegcs-section) pour pointer vers l'emplacement GCP et fournir les informations d'identification pour la connexion. Assurez-vous que le runner dispose d'un chemin réseau vers le point de terminaison GCS.

### Utiliser Azure Blob storage {#use-azure-blob-storage}

Pour utiliser Azure Blob storage comme cache distribué, [modifiez le fichier `config.toml` du runner](advanced-configuration.md#the-runnerscacheazure-section) pour pointer vers l'emplacement Azure et fournir les informations d'identification pour la connexion. Assurez-vous que le runner dispose d'un chemin réseau vers le point de terminaison Azure.

### Accélérer les transferts de cache et d'artefacts {#speed-up-cache-and-artifact-transfers}

Vous pouvez améliorer les performances de chargement et de téléchargement du cache et des artefacts avec les options suivantes.

#### Configuration du runner spécifique au backend {#backend-specific-runner-config}

Chaque backend de cache possède sa propre section `config.toml`. Optimisez selon votre backend :

- [Configuration S3](advanced-configuration.md#the-runnerscaches3-section)) :  Définissez `BucketLocation` sur la même région que vos runners. Utilisez `RoleARN` pour les archives de plus de 5 Go afin d'[activer les chargements en plusieurs parties](advanced-configuration.md#enable-multipart-transfers-with-rolearn). Utilisez l'adaptateur S3 v2 par défaut (ne définissez pas `FF_USE_LEGACY_S3_CACHE_ADAPTER=true`). Activez éventuellement `Accelerate = true` pour l'[accélération de transfert AWS S3](https://docs.aws.amazon.com/AmazonS3/latest/userguide/transfer-acceleration.html) lorsque les runners sont éloignés de la région du bucket. Un [point de terminaison S3 VPC](https://docs.aws.amazon.com/AmazonS3/latest/userguide/creating-s3-vpc-endpoint.html) dans la même région peut réduire la latence et les coûts.
- [Configuration Google Cloud Storage](advanced-configuration.md#the-runnerscachegcs-section)) : Utilisez un bucket dans la même région que vos runners ou la plus proche.
- [Configuration Azure Blob](advanced-configuration.md#the-runnerscacheazure-section)) : Utilisez un compte de stockage dans la même région que vos runners ou la plus proche.

#### Compression du cache {#cache-compression}

Utilisez une compression plus rapide pour accélérer l'archivage et le téléchargement du cache. Cela crée des archives plus volumineuses. Définissez les options de compression dans votre job ou dans les [variables CI/CD](https://docs.gitlab.com/ee/ci/variables/) :

| Variable | Recommandé pour la vitesse | Description |
|----------|------------------------|-------------|
| `CACHE_COMPRESSION_LEVEL` | `fastest` ou `fast` | Moins de CPU et chargement ou téléchargement plus rapide. Les archives sont plus volumineuses. La valeur par défaut est `default`. |
| `CACHE_COMPRESSION_FORMAT` | `zip` | `zip` est souvent plus rapide à créer. `tarzstd` offre un meilleur taux de compression mais peut être plus lent. |

Exemple de configuration dans `.gitlab-ci.yml` :

```yaml
variables:
  CACHE_COMPRESSION_LEVEL: fastest
  CACHE_COMPRESSION_FORMAT: zip
```

#### Délai d'expiration des requêtes de cache {#cache-request-timeout}

Si les caches volumineux atteignent des délais d'expiration, augmentez la limite (en minutes) avec la variable CI/CD `CACHE_REQUEST_TIMEOUT` [CI/CD variable](https://docs.gitlab.com/ee/ci/variables/). La valeur par défaut est `10`. Ce paramètre n'accélère pas les transferts, mais empêche les échecs lors de chargements et de téléchargements lents ou volumineux.

#### Taille du tampon de transfert de cache (débit) {#cache-transfer-buffer-size-throughput}

Le téléchargement et le chargement du cache utilisent un seul tampon de streaming. Un tampon plus grand réduit les appels système et augmente souvent le débit, surtout si vous constatez que les transferts plafonnent autour de 20 à 30 Mo/s.

Définissez `CACHE_TRANSFER_BUFFER_SIZE` (en octets) dans l'environnement du job ou dans les [variables CI/CD](https://docs.gitlab.com/ee/ci/variables/). La valeur par défaut est 4 Mio (4194304).

Exemple de configuration pour 8 Mio :

```yaml
variables:
  CACHE_TRANSFER_BUFFER_SIZE: "8388608"
```

#### Taille des chunks et concurrence du cache {#cache-chunk-size-and-concurrency}

La taille de chunk est la taille en octets de chaque partie ou chunk pour le chargement parallèle (GoCloud) ou le téléchargement parallèle (présigné ou GoCloud). La concurrence correspond au nombre de chunks exécutés en parallèle. L'utilisation de la mémoire est approximativement égale à la taille du chunk x la concurrence.

| Variable | Description | Valeur par défaut |
|----------|-------------|---------|
| `CACHE_CHUNK_SIZE` | Taille du chunk en octets. Pour le chargement (backends GoCloud) : les limites dépendent du backend (par exemple, de 5 Mio à 5 Gio par partie, 10 000 parties maximum pour S3 ; Azure et GCS ont leurs propres limites). Pour le téléchargement : 0 = séquentiel hérité ; lorsque la concurrence est > 1, 16 Mio est utilisé si non défini. | Chargement : 16 Mio (16777216). Téléchargement : 0 (hérité) |
| `CACHE_CONCURRENCY` | Nombre de chunks simultanés. Chargement : Backends GoCloud uniquement (S3 avec RoleARN, Azure, GCS). Téléchargement : 0 ou 1 = mode séquentiel hérité ; valeurs supérieures à 1 = mode parallèle (présigné ou GoCloud). | Chargement : 16\. Téléchargement : 0 (hérité) |

Exemple de configuration pour un réglage personnalisé (par exemple, chunks de 32 Mio, 32 simultanés) :

```yaml
variables:
  CACHE_CHUNK_SIZE: "33554432"
  CACHE_CONCURRENCY: "32"
```

#### Chargements d'artefacts vers GitLab {#artifact-uploads-to-gitlab}

GitLab envoie les artefacts au coordinateur GitLab, qui peut les stocker dans un stockage objet. Pour accélérer le chargement depuis le runner :

| Variable | Recommandé pour la vitesse | Description |
|----------|------------------------|-------------|
| `ARTIFACT_COMPRESSION_LEVEL` | `fastest` ou `fast` | Réduit le CPU et le temps de compression avant le chargement. |

Définissez les options de compression dans votre job ou dans les variables CI/CD, par exemple :

```yaml
variables:
  ARTIFACT_COMPRESSION_LEVEL: fastest
```

#### Téléchargements d'artefacts depuis le stockage objet {#artifact-downloads-from-object-storage}

Lorsque le coordinateur redirige les téléchargements d'artefacts vers le stockage objet (`direct_download`), vous pouvez activer les téléchargements en plages parallèles avec le feature flag `FF_USE_PARALLEL_ARTIFACT_TRANSFER` [feature flag](feature-flags.md). Ceci est distinct des transferts de cache parallèles (`FF_USE_PARALLEL_CACHE_TRANSFER`). Voir [Téléchargements d'artefacts parallèles (téléchargement direct)](advanced-configuration.md#parallel-artifact-downloads-direct-download).
