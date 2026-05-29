---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Configurer le chart Helm GitLab Runner
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Vous pouvez ajouter une configuration optionnelle à votre chart Helm GitLab Runner.

## Utiliser le cache avec un modèle de configuration {#use-the-cache-with-a-configuration-template}

Pour utiliser le cache avec votre modèle de configuration, définissez ces variables dans `values.yaml` :

- `runners.cache.secretName` :  Le nom du secret pour votre fournisseur de stockage d'objets. Options : `s3access`, `gcsaccess`, `google-application-credentials` ou `azureaccess`.
- `runners.config` :  Autres paramètres pour [le cache](../configuration/advanced-configuration.md#the-runnerscache-section), au format TOML.

### Amazon S3 {#amazon-s3}

Pour configurer [Amazon S3 avec des identifiants statiques](https://aws.amazon.com/blogs/security/wheres-my-secret-access-key/) :

1. Ajoutez cet exemple à votre `values.yaml`, en modifiant les valeurs si nécessaire :

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "s3"
           Path = "runner"
           Shared = true
           [runners.cache.s3]
             ServerAddress = "s3.amazonaws.com"
             BucketName = "my_bucket_name"
             BucketLocation = "eu-west-1"
             Insecure = false
             AuthenticationType = "access-key"

     cache:
         secretName: s3access
   ```

1. Créez un secret Kubernetes `s3access` contenant `accesskey` et `secretkey` :

   ```shell
   kubectl create secret generic s3access \
       --from-literal=accesskey="YourAccessKey" \
       --from-literal=secretkey="YourSecretKey"
   ```

### Google Cloud Storage (GCS) {#google-cloud-storage-gcs}

Google Cloud Storage peut être configuré avec des identifiants statiques de plusieurs façons.

#### Identifiants statiques configurés directement {#static-credentials-directly-configured}

Pour configurer GCS avec des identifiants [avec un identifiant d'accès et une clé privée](../configuration/advanced-configuration.md#the-runnerscache-section) :

1. Ajoutez cet exemple à votre `values.yaml`, en modifiant les valeurs si nécessaire :

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "gcs"
           Path = "runner"
           Shared = true
           [runners.cache.gcs]
             BucketName = "runners-cache"

     cache:
       secretName: gcsaccess
   ```

1. Créez un secret Kubernetes `gcsaccess` contenant `gcs-access-id` et `gcs-private-key` :

   ```shell
   kubectl create secret generic gcsaccess \
       --from-literal=gcs-access-id="YourAccessID" \
       --from-literal=gcs-private-key="YourPrivateKey"
   ```

#### Identifiants statiques dans un fichier JSON téléchargé depuis GCP {#static-credentials-in-a-json-file-downloaded-from-gcp}

Pour [configurer GCS avec des identifiants dans un fichier JSON](../configuration/advanced-configuration.md#the-runnerscache-section) téléchargé depuis Google Cloud Platform :

1. Ajoutez cet exemple à votre `values.yaml`, en modifiant les valeurs si nécessaire :

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "gcs"
           Path = "runner"
           Shared = true
           [runners.cache.gcs]
             BucketName = "runners-cache"

     cache:
         secretName: google-application-credentials

   secrets:
     - name: google-application-credentials
   ```

1. Créez un secret Kubernetes appelé `google-application-credentials` et chargez le fichier JSON avec celui-ci. Modifiez le chemin si nécessaire :

   ```shell
   kubectl create secret generic google-application-credentials \
       --from-file=gcs-application-credentials-file=./PATH-TO-CREDENTIALS-FILE.json
   ```

### Azure {#azure}

Pour [configurer Azure Blob Storage](../configuration/advanced-configuration.md#the-runnerscacheazure-section) :

1. Ajoutez cet exemple à votre `values.yaml`, en modifiant les valeurs si nécessaire :

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "azure"
           Path = "runner"
           Shared = true
           [runners.cache.azure]
             ContainerName = "CONTAINER_NAME"
             StorageDomain = "blob.core.windows.net"

     cache:
         secretName: azureaccess
   ```

1. Créez un secret Kubernetes `azureaccess` contenant `azure-account-name` et `azure-account-key` :

   ```shell
   kubectl create secret generic azureaccess \
       --from-literal=azure-account-name="YourAccountName" \
       --from-literal=azure-account-key="YourAccountKey"
   ```

Pour en savoir plus sur la mise en cache avec le chart Helm, consultez [`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml).

### Revendication de volume persistant {#persistent-volume-claim}

Vous pouvez utiliser des revendications de volume persistant (PVC) pour la mise en cache si aucune des options de stockage d'objets ne vous convient.

Pour configurer votre cache afin d'utiliser un PVC :

1. [Créez un PVC](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#persistentvolumeclaims) dans l'espace de nommage où les pods de job seront exécutés.

   > [!note]
   > Si vous souhaitez que plusieurs pods de job accèdent au même PVC de cache, celui-ci doit avoir le mode d'accès `ReadWriteMany`.

1. Montez le PVC dans le répertoire `/cache` :

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [[runners.kubernetes.volumes.pvc]]
           name = "cache-pvc"
           mount_path = "/cache"
   ```

### Network File System {#network-file-system}

Utilisez un Network File System (NFS) pour la mise en cache lorsque le stockage d'objets n'est pas disponible.

Prérequis :

- NFS est configuré et accessible dans votre cluster Kubernetes. Pour plus d'informations, consultez [le volume `nfs`](https://kubernetes.io/docs/concepts/storage/volumes/#nfs) dans la documentation Kubernetes.

Pour configurer votre cache afin d'utiliser NFS :

1. Montez le volume NFS dans le répertoire `/cache` :

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [[runners.kubernetes.volumes.nfs]]
           name = "nfs"
           mount_path = "/cache"
           read_only = false
           server = "foo.bar.com"
           path = "/path/on/nfs-share"
   ```

## Activer la prise en charge RBAC {#enable-rbac-support}

Si votre cluster a RBAC (contrôles d'accès basés sur les rôles) activé, le chart peut créer son propre compte de service, ou vous pouvez [en fournir un](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#service-account-permissions).

- Pour que le chart crée le compte de service à votre place, définissez `rbac.create` sur true :

  ```yaml
  rbac:
    create: true
  ```

- Pour utiliser un compte de service existant, définissez un `serviceAccount.name` :

  ```yaml
  rbac:
    create: false
  serviceAccount:
    create: false
    name: your-service-account
  ```

## Contrôler la concurrence maximale des runners {#control-maximum-runner-concurrency}

Un seul runner déployé sur Kubernetes peut exécuter plusieurs jobs en parallèle en démarrant des pods Runner supplémentaires. Pour modifier le nombre maximal de pods autorisés à la fois, modifiez le [paramètre `concurrent`](../configuration/advanced-configuration.md#the-global-section). Sa valeur par défaut est `10` :

```yaml
## Configure the maximum number of concurrent jobs
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration/#the-global-section
##
concurrent: 10
```

Pour plus d'informations sur ce paramètre, consultez [la section globale](../configuration/advanced-configuration.md#the-global-section) dans la documentation de configuration avancée de GitLab Runner.

## Exécuter des conteneurs Docker-in-Docker avec GitLab Runner {#run-docker-in-docker-containers-with-gitlab-runner}

Pour utiliser des conteneurs Docker-in-Docker avec GitLab Runner :

- Pour l'activer, consultez [Utiliser des conteneurs privilégiés pour les runners](#use-privileged-containers-for-the-runners).
- Pour obtenir des instructions sur l'exécution de Docker-in-Docker, consultez la [documentation de GitLab Runner](../executors/kubernetes/_index.md#using-docker-in-builds).

## Utiliser des conteneurs privilégiés pour les runners {#use-privileged-containers-for-the-runners}

Pour utiliser l'exécutable Docker dans vos jobs GitLab CI/CD, configurez le runner pour qu'il utilise des conteneurs privilégiés.

Prérequis :

- Vous comprenez les risques, qui sont décrits dans la [documentation GitLab CI/CD Runner](../executors/kubernetes/_index.md#using-docker-in-builds).
- Votre instance GitLab Runner est enregistrée pour un projet spécifique dans GitLab, et vous faites confiance à ses jobs CI/CD.

Pour activer le mode privilégié dans `values.yaml`, ajoutez ces lignes :

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        # Run all containers with the privileged flag enabled.
        privileged = true
        ...
```

Pour plus d'informations, consultez la documentation de configuration avancée sur la section [`[runners.kubernetes]`](../configuration/advanced-configuration.md#the-runnerskubernetes-section).

## Utiliser une image depuis un registre privé {#use-an-image-from-a-private-registry}

Pour utiliser une image depuis un registre de conteneurs privé, configurez `imagePullSecrets`.

1. Créez un ou plusieurs secrets dans l'espace de nommage Kubernetes utilisé pour le job CI/CD. Cette commande crée un secret qui fonctionne avec `image_pull_secrets` :

   ```shell
   kubectl create secret docker-registry <SECRET_NAME> \
     --namespace <NAMESPACE> \
     --docker-server="https://<REGISTRY_SERVER>" \
     --docker-username="<REGISTRY_USERNAME>" \
     --docker-password="<REGISTRY_PASSWORD>"
   ```

1. Pour GitLab Runner Helm chart version 0.53.x et ultérieure, dans `config.toml`, définissez `image_pull_secret` depuis le modèle fourni dans `runners.config` :

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           ## Specify one or more imagePullSecrets
           ## ref: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
           ##
           image_pull_secrets = [your-image-pull-secret]
   ```

   Pour plus d'informations, consultez [Extraire une image depuis un registre privé](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/) dans la documentation Kubernetes.

1. Pour GitLab Runner Helm chart version 0.52 et antérieure, dans `values.yaml`, définissez une valeur pour `runners.imagePullSecrets`. Lorsque vous définissez cette valeur, le conteneur ajoute `--kubernetes-image-pull-secrets "<SECRET_NAME>"` au script de point d'entrée de l'image. Cela élimine la nécessité de configurer le paramètre `image_pull_secrets` dans les paramètres `config.toml` de l'exécuteur Kubernetes.

   ```yaml
   runners:
     imagePullSecrets: [your-image-pull-secret]
   ```

> [!note]
> La valeur de `imagePullSecrets` n'est pas préfixée par une balise `name`, contrairement à la convention dans les ressources Kubernetes. Cette valeur nécessite un tableau d'un ou plusieurs noms de secrets, même si vous n'utilisez qu'un seul identifiant de registre.

Pour plus de détails sur la création de `imagePullSecrets`, consultez [Extraire une image depuis un registre privé](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/) dans la documentation Kubernetes.

Lorsqu'un pod de job est en cours de création, GitLab Runner gère automatiquement l'accès aux images en deux étapes :

1. GitLab Runner convertit tous les identifiants Docker existants en secrets Kubernetes afin qu'ils puissent extraire des images depuis les registres. Il vérifie également que tout imagePullSecrets configuré manuellement existe bien dans le cluster. Pour plus d'informations sur les identifiants définis de manière statique, les magasins d'identifiants ou les assistants d'identifiants, consultez [Accéder à une image depuis un registre de conteneurs privé](https://docs.gitlab.com/ci/docker/using_docker_images/#access-an-image-from-a-private-container-registry).
1. GitLab Runner crée le pod de job et lui attache les deux types d'identifiants : les `imagePullSecrets` et les identifiants Docker convertis, dans cet ordre.

Lorsque Kubernetes doit extraire l'image du conteneur, il essaie les identifiants un par un jusqu'à trouver celui qui fonctionne.

## Accéder à GitLab avec un certificat personnalisé {#access-gitlab-with-a-custom-certificate}

Pour utiliser un certificat personnalisé, fournissez un [secret Kubernetes](https://kubernetes.io/docs/concepts/configuration/secret/) au chart Helm GitLab Runner. Ce secret est ajouté au répertoire `/home/gitlab-runner/.gitlab-runner/certs` du conteneur :

1. [Préparer votre certificat](#prepare-your-certificate)
1. [Créer un secret Kubernetes](#create-a-kubernetes-secret)
1. [Fournir le secret au chart](#provide-the-secret-to-the-chart)

### Préparer votre certificat {#prepare-your-certificate}

Chaque nom de clé dans le secret Kubernetes est utilisé comme nom de fichier dans le répertoire, le contenu du fichier étant la valeur associée à la clé :

- Le nom de fichier utilisé doit être au format `<gitlab.hostname>.crt`, par exemple `gitlab.your-domain.com.crt`.
- Concaténez tous les certificats intermédiaires avec votre certificat de serveur dans le même fichier.
- Le nom d'hôte utilisé doit être celui pour lequel le certificat est enregistré.

### Créer un secret Kubernetes {#create-a-kubernetes-secret}

Si vous avez installé le chart Helm GitLab en utilisant la méthode du [certificat générique auto-signé généré automatiquement](https://docs.gitlab.com/charts/installation/tls/#option-4-use-auto-generated-self-signed-wildcard-certificate), un secret a été créé pour vous.

Si vous n'avez pas installé le chart Helm GitLab avec le certificat générique auto-signé généré automatiquement, créez un secret. Ces commandes stockent votre certificat en tant que secret dans Kubernetes, et le présentent aux conteneurs GitLab Runner sous forme de fichier.

- Si votre certificat se trouve dans le répertoire courant et respecte le format `<gitlab.hostname.crt>`, modifiez cette commande si nécessaire :

  ```shell
  kubectl create secret generic <SECRET_NAME> \
    --namespace <NAMESPACE> \
    --from-file=<CERTIFICATE_FILENAME>
  ```

  - `<NAMESPACE>` :  L'espace de nommage Kubernetes dans lequel vous souhaitez installer GitLab Runner.
  - `<SECRET_NAME>` :  Le nom de la ressource Secret Kubernetes, comme `gitlab-domain-cert`.
  - `<CERTIFICATE_FILENAME>` :  Le nom de fichier du certificat dans votre répertoire courant à importer dans le secret.
- Si votre certificat se trouve dans un autre répertoire ou ne respecte pas le format `<gitlab.hostname.crt>`, vous devez spécifier le nom de fichier à utiliser comme cible :

  ```shell
  kubectl create secret generic <SECRET_NAME> \
    --namespace <NAMESPACE> \
    --from-file=<TARGET_FILENAME>=<CERTIFICATE_FILENAME>
  ```

  - `<TARGET_FILENAME>` est le nom du fichier de certificat tel qu'il est présenté aux conteneurs Runner, comme `gitlab.hostname.crt`.
  - `<CERTIFICATE_FILENAME>` est le nom de fichier du certificat, relatif à votre répertoire courant, à importer dans le secret. Par exemple : `cert-directory/my-gitlab-certificate.crt`.

### Fournir le secret au chart {#provide-the-secret-to-the-chart}

Dans `values.yaml`, définissez `certsSecretName` sur le nom de ressource d'un objet secret Kubernetes dans le même espace de nommage. Cela vous permet de transmettre votre certificat personnalisé à GitLab Runner pour qu'il l'utilise. Dans l'exemple précédent, le nom de ressource était `gitlab-domain-cert` :

```yaml
certsSecretName: <SECRET NAME>
```

Pour plus d'informations, consultez les [options prises en charge pour les certificats auto-signés](../configuration/tls-self-signed.md#supported-options-for-self-signed-certificates-targeting-the-gitlab-server) ciblant le serveur GitLab.

## Définir des labels de pod sur des clés de variables d'environnement CI {#set-pod-labels-to-ci-environment-variable-keys}

Vous ne pouvez pas utiliser des variables d'environnement comme labels de pod dans le fichier `values.yaml`. Pour plus d'informations, consultez [Impossible de définir une clé de variable d'environnement comme label de pod](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/173). Utilisez [la solution de contournement décrite dans le ticket](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/173#note_351057890) comme solution temporaire.

## Passer à l'image Docker `gitlab-runner` basée sur Ubuntu {#switch-to-the-ubuntu-based-gitlab-runner-docker-image}

Par défaut, le chart Helm GitLab Runner utilise la version Alpine de l'image `gitlab/gitlab-runner`, qui utilise `musl libc`. Vous devrez peut-être passer à l'image basée sur Ubuntu, qui utilise `glibc`.

Pour ce faire, spécifiez l'image dans votre fichier `values.yaml` avec les valeurs suivantes :

```yaml
# Specify the Ubuntu image, and set the version. You can also use the `ubuntu` or `latest` tags.
image: gitlab/gitlab-runner:v17.3.0

# Update the security context values to the user ID in the Ubuntu image
securityContext:
  fsGroup: 999
  runAsUser: 999
```

## Exécuter avec un utilisateur non root {#run-with-non-root-user}

Par défaut, les images GitLab Runner ne fonctionnent pas avec des utilisateurs non root. Les images [GitLab Runner UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766421) et [GitLab Runner Helper UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766433) sont conçues pour ce scénario.

Pour les utiliser, modifiez les images GitLab Runner et GitLab Runner Helper dans `values.yaml` :

```yaml
image:
  registry: registry.gitlab.com
  image: gitlab-org/ci-cd/gitlab-runner-ubi-images/gitlab-runner-ocp
  tag: v16.11.0

securityContext:
    runAsNonRoot: true
    runAsUser: 999

runners:
    config: |
        [[runners]]
          [runners.kubernetes]
            helper_image = "registry.gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/gitlab-runner-helper-ocp:x86_64-v16.11.0"
            [runners.kubernetes.pod_security_context]
              run_as_non_root = true
              run_as_user = 59417
```

Bien que `run_as_user` pointe vers l'ID utilisateur de l'utilisateur `nonroot` (59417), les images fonctionnent avec n'importe quel ID utilisateur. Il est important que cet ID utilisateur fasse partie du groupe root. Faire partie du groupe root ne lui confère aucun privilège spécifique.

## Utiliser un GitLab Runner conforme FIPS {#use-a-fips-compliant-gitlab-runner}

Pour utiliser un [GitLab Runner conforme FIPS](requirements.md#fips-compliant-gitlab-runner), modifiez l'image GitLab Runner et l'image Helper dans `values.yaml` :

```yaml
image:
  registry: docker.io
  image: gitlab/gitlab-runner
  tag: ubi-fips

runners:
    config: |
        [[runners]]
          [runners.kubernetes]
            helper_image_flavor = "ubi-fips"
```

## Utiliser un modèle de configuration {#use-a-configuration-template}

Pour [configurer le comportement du pod de build GitLab Runner dans Kubernetes](../executors/kubernetes/_index.md#configuration-settings), utilisez un [fichier de modèle de configuration](../register/_index.md#register-with-a-configuration-template). Les modèles de configuration peuvent configurer n'importe quel champ du runner, sans partager des options de configuration spécifiques du runner avec le chart Helm. Par exemple, ces paramètres par défaut [trouvés dans le fichier `values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml) dans le dépôt `chart` :

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        image = "ubuntu:22.04"
```

Les valeurs de la section `config:` doivent utiliser TOML (`<parameter> = <value>` au lieu de `<parameter>: <value>`), car `config.toml` est intégré dans `values.yaml`.

Pour la configuration spécifique à l'exécuteur, consultez le fichier [`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml).
