---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Configurer GitLab Runner sur OpenShift
---

{{< details >}}

- Niveau : Free, Premium, Ultimate
- Offre : GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Ce document explique comment configurer GitLab Runner sur OpenShift.

## Transmettre des propriétés à GitLab Runner Operator {#passing-properties-to-gitlab-runner-operator}

Lors de la création d'un `Runner`, vous pouvez le configurer en définissant des propriétés dans son `spec`. Par exemple, vous pouvez spécifier l'URL GitLab où le runner est enregistré, ou le nom du secret contenant le jeton d'enregistrement :

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret # Name of the secret containing the Runner token
```

Consultez toutes les propriétés disponibles dans [Propriétés de l'Operator](#operator-properties).

## Propriétés de l'Operator {#operator-properties}

Les propriétés suivantes peuvent être transmises à l'Operator.

Certaines propriétés sont uniquement disponibles avec les versions plus récentes de l'Operator.

| Paramètre            | Operator | Description |
|--------------------|----------|-------------|
| `gitlabUrl`        | tous      | Le nom de domaine complet de l'instance GitLab, par exemple `https://gitlab.example.com`. |
| `token`            | tous      | Nom du `Secret` contenant la clé `runner-registration-token` utilisée pour enregistrer le runner. |
| `tags`             | tous      | Liste de tags séparés par des virgules à appliquer au runner. |
| `concurrent`       | tous      | Limite le nombre de jobs pouvant s'exécuter simultanément. Le nombre maximum correspond à l'ensemble des runners définis. 0 ne signifie pas illimité. La valeur par défaut est `10`. |
| `interval`         | tous      | Définit le nombre de secondes entre les vérifications de nouveaux jobs. La valeur par défaut est `30`. |
| `locked`           | 1.8      | Définit si le runner doit être verrouillé sur un projet. La valeur par défaut est `false`. |
| `runUntagged`      | 1.8      | Définit si les jobs sans tags doivent être exécutés. La valeur par défaut est `true` si aucun tag n'a été spécifié. Sinon, elle est `false`. |
| `protected`        | 1.8      | Définit si le runner doit exécuter des jobs uniquement sur les branches protégées. La valeur par défaut est `false`. |
| `cloneURL`         | tous      | Remplace l'URL de l'instance GitLab. Utilisé uniquement si le runner ne peut pas se connecter à l'URL GitLab. |
| `env`              | tous      | Nom du `ConfigMap` contenant les paires clé-valeur injectées en tant que variables d'environnement dans le pod Runner. |
| `runnerImage`      | 1.7      | Remplace l'image GitLab Runner par défaut. La valeur par défaut est l'image Runner fournie avec l'opérateur. |
| `helperImage`      | tous      | Remplace l'image helper GitLab Runner par défaut. |
| `buildImage`       | tous      | L'image Docker par défaut à utiliser pour les builds lorsqu'aucune n'est spécifiée. |
| `cacheType`        | tous      | Type de cache utilisé pour les artefacts Runner. L'un des suivants : `gcs`, `s3`, `azure`. |
| `cachePath`        | tous      | Définit le chemin du cache sur le système de fichiers. |
| `cacheShared`      | tous      | Active le partage du cache entre les runners. |
| `s3`               | tous      | Options utilisées pour configurer le cache S3. Reportez-vous à [Propriétés du cache](#cache-properties). |
| `gcs`              | tous      | Options utilisées pour configurer le cache `gcs`. Reportez-vous à [Propriétés du cache](#cache-properties). |
| `azure`            | tous      | Options utilisées pour configurer le cache Azure. Reportez-vous à [Propriétés du cache](#cache-properties). |
| `ca`               | tous      | Nom du secret TLS contenant les certificats d'autorité de certification (CA) personnalisés. |
| `serviceaccount`   | tous      | Utilisé pour remplacer le compte de service utilisé pour exécuter le pod Runner. |
| `config`           | tous      | Utilisé pour fournir un `ConfigMap` personnalisé avec un [modèle de configuration](../register/_index.md#register-with-a-configuration-template). |
| `shutdownTimeout`  | 1.34     | Nombre de secondes avant l'expiration de l'[opération d'arrêt forcé](../commands/_index.md#signals) et la fermeture du processus. La valeur par défaut est `30`. Si la valeur est `0` ou inférieure, la valeur par défaut est utilisée. |
| `logLevel`         | 1.34     | Définit le niveau de journalisation. Les options sont `debug`, `info`, `warn`, `error`, `fatal` et `panic`. |
| `logFormat`        | 1.34     | Spécifie le format du journal. Les options sont `runner`, `text` et `json`. La valeur par défaut est `runner`, qui contient des codes d'échappement ANSI pour la colorisation. |
| `listenAddr`       | 1.34     | Définit une adresse (`<host>:<port>`) sur laquelle le serveur HTTP de métriques Prometheus doit écouter. Pour plus d'informations sur la configuration, consultez [Surveiller GitLab Runner Operator](../monitoring/_index.md#monitor-operator-managed-gitlab-runners). |
| `sentryDsn`        | 1.34     | Active le suivi de toutes les erreurs au niveau système dans Sentry. |
| `connectionMaxAge` | 1.34     | La durée maximale pendant laquelle une connexion TLS keepalive au serveur GitLab doit rester ouverte avant la reconnexion. La valeur par défaut est `15m` pour 15 minutes. Si la valeur est `0` ou inférieure, la connexion persiste aussi longtemps que possible. |
| `podSpec`          | 1.23     | Liste des correctifs à appliquer au pod GitLab Runner (modèle). Pour plus d'informations, consultez [Application de correctifs au modèle de pod runner](#patching-the-runner-pod-template). |
| `deploymentSpec`   | 1.40     | Liste des correctifs à appliquer au déploiement GitLab Runner. Pour plus d'informations, consultez [Application de correctifs au modèle de déploiement runner](#patching-the-runner-deployment-template). |

## Propriétés du cache {#cache-properties}

### Cache S3 {#s3-cache}

| Paramètre       | Operator | Description |
|---------------|----------|-------------|
| `server`      | tous      | L'adresse du serveur S3. |
| `credentials` | tous      | Nom du `Secret` contenant les propriétés `accesskey` et `secretkey` utilisées pour accéder au stockage d'objets. |
| `bucket`      | tous      | Nom du bucket dans lequel le cache est stocké. |
| `location`    | tous      | Nom de la région S3 dans laquelle le cache est stocké. |
| `insecure`    | tous      | Utiliser des connexions non sécurisées ou `HTTP`. |

### Cache `gcs` {#gcs-cache}

| Paramètre           | Operator | Description |
|-------------------|----------|-------------|
| `credentials`     | tous      | Nom du `Secret` contenant les propriétés `access-id` et `private-key` utilisées pour accéder au stockage d'objets. |
| `bucket`          | tous      | Nom du bucket dans lequel le cache est stocké. |
| `credentialsFile` | tous      | Prend le fichier de credentials `gcs`, `keys.json`. |

### Cache Azure {#azure-cache}

| Paramètre         | Operator | Description |
|-----------------|----------|-------------|
| `credentials`   | tous      | Nom du `Secret` contenant les propriétés `accountName` et `privateKey` utilisées pour accéder au stockage d'objets. |
| `container`     | tous      | Nom du conteneur Azure dans lequel le cache est stocké. |
| `storageDomain` | tous      | Le nom de domaine du stockage blob Azure. |

## Configurer un environnement proxy {#configure-a-proxy-environment}

Pour créer un environnement proxy :

1. Modifiez le fichier `custom-env.yaml`. Par exemple :

   ```yaml
   apiVersion: v1
   data:
     HTTP_PROXY: example.com
   kind: ConfigMap
   metadata:
     name: custom-env
   ```

1. Mettez à jour OpenShift pour appliquer les modifications.

   ```shell
   oc apply -f custom-env.yaml
   ```

1. Mettez à jour votre fichier [`gitlab-runner.yml`](../install/operator.md#install-gitlab-runner).

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     token: gitlab-runner-secret # Name of the secret containing the Runner token
     env: custom-env
   ```

Si le proxy ne peut pas atteindre l'API Kubernetes, il est possible qu'une erreur apparaisse dans votre job CI/CD :

```shell
ERROR: Job failed (system failure): prepare environment: setting up credentials: Post https://172.21.0.1:443/api/v1/namespaces/<KUBERNETES_NAMESPACE>/secrets: net/http: TLS handshake timeout. Check https://docs.gitlab.com/runner/shells/#shell-profile-loading for more information
```

Pour résoudre cette erreur, ajoutez l'adresse IP de l'API Kubernetes à la configuration `NO_PROXY` dans le fichier `custom-env.yaml` :

```yaml
   apiVersion: v1
   data:
     NO_PROXY: 172.21.0.1
     HTTP_PROXY: example.com
   kind: ConfigMap
   metadata:
     name: custom-env
```

Vous pouvez vérifier l'adresse IP de l'API Kubernetes en exécutant :

```shell
oc get services --namespace default --field-selector='metadata.name=kubernetes' | grep -v NAME | awk '{print $3}'
```

## Personnaliser `config.toml` avec un modèle de configuration {#customize-configtoml-with-a-configuration-template}

Vous pouvez personnaliser le fichier `config.toml` du runner en utilisant le [modèle de configuration](../register/_index.md#register-with-a-configuration-template).

1. Créez un fichier de modèle de configuration personnalisé. Par exemple, demandons à notre runner de monter un volume `EmptyDir` et de définir `cpu_limit`. Créez le fichier `custom-config.toml` :

   ```toml
   [[runners]]
     [runners.kubernetes]
       cpu_limit = "500m"
       [runners.kubernetes.volumes]
         [[runners.kubernetes.volumes.empty_dir]]
           name = "empty-dir"
           mount_path = "/path/to/empty_dir"
           medium = "Memory"
   ```

1. Créez un `ConfigMap` nommé `custom-config-toml` à partir de notre fichier `custom-config.toml` :

   ```shell
    oc create configmap custom-config-toml --from-file config.toml=custom-config.toml
   ```

1. Définissez la propriété `config` du `Runner` :

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     token: gitlab-runner-secret
     config: custom-config-toml
   ```

En raison d'un [problème connu](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/-/issues/229), vous devez utiliser des variables d'environnement plutôt que des modèles de configuration pour modifier les paramètres suivants :

| Paramètre                          | Variable d'environnement         | Valeur par défaut |
|----------------------------------|------------------------------|---------------|
| `runners.request_concurrency`    | `RUNNER_REQUEST_CONCURRENCY` | `1`           |
| `runners.output_limit`           | `RUNNER_OUTPUT_LIMIT`        | `4096`        |
| `kubernetes.runner.poll_timeout` | `KUBERNETES_POLL_TIMEOUT`    | `180`         |

## Configurer un certificat TLS personnalisé {#configure-a-custom-tls-cert}

1. Pour définir un certificat TLS personnalisé, créez un secret avec la clé `tls.crt`. Dans cet exemple, le fichier est nommé `custom-tls-ca-secret.yaml` :

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
       name: custom-tls-ca
   type: Opaque
   stringData:
       tls.crt: |
           -----BEGIN CERTIFICATE-----
           MIIEczCCA1ugAwIBAgIBADANBgkqhkiG9w0BAQQFAD..AkGA1UEBhMCR0Ix
           .....
           7vQMfXdGsRrXNGRGnX+vWDZ3/zWI0joDtCkNnqEpVn..HoX
           -----END CERTIFICATE-----
   ```

1. Créez le secret :

   ```shell
   oc apply -f custom-tls-ca-secret.yaml
   ```

1. Définissez la clé `ca` dans le fichier `runner.yaml` avec le même nom que notre secret :

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     token: gitlab-runner-secret
     ca: custom-tls-ca
   ```

## Configurer la taille CPU et mémoire des pods runner {#configure-the-cpu-and-memory-size-of-runner-pods}

Pour définir les [limites CPU](../executors/kubernetes/_index.md#cpu-requests-and-limits) et les [limites mémoire](../executors/kubernetes/_index.md#memory-requests-and-limits) dans un fichier `config.toml` personnalisé, suivez les instructions de [cette rubrique](#customize-configtoml-with-a-configuration-template).

## Configurer la simultanéité des jobs par runner en fonction des ressources du cluster {#configure-job-concurrency-per-runner-based-on-cluster-resources}

Définissez la propriété `concurrent` de la ressource `Runner` :

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  concurrent: 2
```

La simultanéité des jobs est dictée par les exigences du projet.

1. Commencez par essayer de déterminer les ressources de calcul et de mémoire nécessaires pour exécuter un job CI.
1. Calculez combien de fois ce job pourrait s'exécuter compte tenu des ressources disponibles dans le cluster.

Si vous définissez une valeur de simultanéité élevée, l'exécuteur Kubernetes traite les jobs dès que possible. Cependant, la capacité du planificateur du cluster Kubernetes détermine quand le job est planifié.

## Compte de service pour le gestionnaire GitLab Runner {#service-account-for-the-gitlab-runner-manager}

Pour une nouvelle installation, GitLab Runner crée un `ServiceAccount` Kubernetes nommé `gitlab-runner-app-sa` pour le pod du gestionnaire de runner si ces ressources de liaison de rôle RBAC n'existent pas :

- `gitlab-runner-app-rolebinding`
- `gitlab-runner-rolebinding`

Si l'une des liaisons de rôle existe, GitLab résout le rôle et le compte de service à partir des `subjects` et `roleRef` définis dans la liaison de rôle.

Si les deux liaisons de rôle existent, `gitlab-runner-app-rolebinding` a la priorité sur `gitlab-runner-rolebinding`.

## Dépannage {#troubleshooting}

### Root vs non-root {#root-vs-non-root}

GitLab Runner Operator et le pod GitLab Runner s'exécutent en tant qu'utilisateurs non-root. Par conséquent, l'image de build utilisée dans le job doit s'exécuter en tant qu'utilisateur non-root pour pouvoir se terminer avec succès. Cela garantit que les jobs peuvent s'exécuter correctement avec le minimum de permissions.

Pour que cela fonctionne, assurez-vous que l'image de build utilisée pour les jobs CI/CD :

- S'exécute en tant que non-root
- N'écrit pas dans des systèmes de fichiers restreints

La plupart des systèmes de fichiers des conteneurs sur un cluster OpenShift sont en lecture seule, sauf :

- Volumes montés
- `/var/tmp`
- `/tmp`
- Autres volumes montés sur des systèmes de fichiers root en tant que `tmpfs`

#### Remplacement de la variable d'environnement `HOME` {#overriding-the-home-environment-variable}

Si vous créez une image de build personnalisée ou si vous [remplacez des variables d'environnement](#configure-a-proxy-environment), assurez-vous que la variable d'environnement `HOME` n'est pas définie sur `/`, ce qui serait en lecture seule. En particulier si vos jobs doivent écrire des fichiers dans le répertoire home. Vous pourriez créer un répertoire sous `/home`, par exemple `/home/ci`, et définir `ENV HOME=/home/ci` dans votre `Dockerfile`.

Pour les pods runner, [il est prévu que `HOME` soit défini sur `/home/gitlab-runner`](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/-/blob/e265820a00a6a1b9a271dc132de2618ced43cf92/runner/Dockerfile.OCP#L14). Si cette variable est modifiée, le nouvel emplacement doit disposer des [permissions appropriées](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/-/blob/e265820a00a6a1b9a271dc132de2618ced43cf92/runner/Dockerfile.OCP#L38). Ces directives sont également documentées dans la [documentation de Red Hat Container Platform](https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/images/creating-images#images-create-guide-openshift_create-images).

### Remplacement de la variable `locked` {#overriding-locked-variable}

Lorsque vous enregistrez un jeton de runner, si vous définissez la variable `locked` sur `true`, l'erreur `Runner configuration other than name, description, and exector is reserved and cannot be specified` apparaît.

```yaml
  locked: true # REQUIRED
  tags: ""
  runUntagged: false
  protected: false
  maximumTimeout: 0
```

Pour plus d'informations, consultez le [ticket 472](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/472#note_1483346437).

#### Attention aux contraintes de contexte de sécurité {#watch-out-for-security-context-constraints}

Par défaut, lorsqu'il est installé dans un nouveau projet OpenShift, GitLab Runner Operator s'exécute en tant que non-root. Certains projets, comme le projet `default`, sont des exceptions où tous les comptes de service disposent d'un accès `anyuid`. Dans ce cas, l'utilisateur de l'image est `root`. Vous pouvez vérifier cela en exécutant `whoami` dans n'importe quel shell de conteneur, par exemple, un job. En savoir plus sur les contraintes de contexte de sécurité dans la [documentation de Red Hat Container Platform](https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/authentication_and_authorization/managing-pod-security-policies).

#### Exécuter avec les contraintes de contexte de sécurité `anyuid` {#run-as-anyuid-security-context-constraints}

> [!warning]
> L'exécution de jobs en tant que root ou l'écriture dans des systèmes de fichiers root peut exposer votre système à des risques de sécurité.

Pour exécuter un job CI/CD en tant qu'utilisateur root ou écrire dans des systèmes de fichiers root, définissez les contraintes de contexte de sécurité `anyuid` sur le compte de service `gitlab-runner-app-sa`. Le conteneur GitLab Runner utilise ce compte de service.

Dans OpenShift 4.3.8 et versions antérieures :

```shell
oc adm policy add-scc-to-user anyuid -z gitlab-runner-app-sa -n <runner_namespace>

# Check that the anyiud SCC is set:
oc get scc anyuid -o yaml
```

Dans OpenShift 4.3.8 et versions ultérieures :

```shell
oc create -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: scc-anyuid
  namespace: <runner_namespace>
rules:
- apiGroups:
  - security.openshift.io
  resourceNames:
  - anyuid
  resources:
  - securitycontextconstraints
  verbs:
  - use
EOF

oc create -f - <<EOF
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: sa-to-scc-anyuid
  namespace: <runner_namespace>
subjects:
  - kind: ServiceAccount
    name: gitlab-runner-app-sa
roleRef:
  kind: Role
  name: scc-anyuid
  apiGroup: rbac.authorization.k8s.io
EOF
```

#### Correspondance des ID utilisateur et de groupe entre le conteneur helper et le conteneur de build {#matching-helper-container-and-build-container-user-id-and-group-id}

Les déploiements GitLab Runner Operator utilisent `registry.gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/gitlab-runner-helper-ocp` comme image helper par défaut. Cette image s'exécute avec l'ID utilisateur et l'ID de groupe `1001:1001`, sauf modification explicite par un contexte de sécurité.

Lorsque l'ID utilisateur de votre conteneur de build diffère de l'ID utilisateur dans l'image helper, des erreurs liées aux permissions peuvent survenir lors de votre build. Voici un message d'erreur courant :

```shell
fatal: detected dubious ownership in repository at '/builds/gitlab-org/gitlab-runner'
```

Cette erreur indique que le dépôt a été cloné par l'ID utilisateur `1001` (conteneur helper), mais qu'un ID utilisateur différent dans le conteneur de build tente d'y accéder.

Solution : configurez le contexte de sécurité de votre conteneur de build pour qu'il corresponde à l'ID utilisateur et à l'ID de groupe du conteneur helper :

```toml
[runners.kubernetes.build_container_security_context]
run_as_user = 1001
run_as_group = 1001
```

Notes supplémentaires :

- Ces paramètres garantissent une propriété cohérente des fichiers entre le conteneur qui clone le dépôt et celui qui le compile.
- Si vous avez personnalisé votre image helper avec des ID utilisateur ou de groupe différents, ajustez ces valeurs en conséquence.
- Pour les déploiements OpenShift, vérifiez que ces paramètres de contexte de sécurité respectent les contraintes de contexte de sécurité (SCC) de votre cluster.

#### Configurer SETFCAP {#configure-setfcap}

Si vous utilisez Red Hat OpenShift Container Platform (RHOCP) 4.11 ou une version ultérieure, vous pouvez obtenir le message d'erreur suivant :

```shell
error reading allowed ID mappings:error reading subuid mappings for user
```

Certains jobs (par exemple, `buildah`) ont besoin de la capacité `SETFCAP` pour s'exécuter correctement. Pour résoudre ce problème :

1. Ajoutez la capacité SETFCAP aux contraintes de contexte de sécurité utilisées par GitLab Runner (remplacez `gitlab-scc` par les contraintes de contexte de sécurité assignées à votre pod GitLab Runner) :

   ```shell
   oc patch scc gitlab-scc --type merge -p '{"allowedCapabilities":["SETFCAP"]}'
   ```

1. Mettez à jour votre `config.toml` et ajoutez la capacité `SETFCAP` sous la section `kubernetes` :

   ```yaml
   [[runners]]
     [runners.kubernetes]
     [runners.kubernetes.pod_security_context]
       [runners.kubernetes.build_container_security_context]
       [runners.kubernetes.build_container_security_context.capabilities]
         add = ["SETFCAP"]
   ```

1. Créez un `ConfigMap` à l'aide de ce `config.toml` dans l'espace de nommage où GitLab Runner est déployé :

   ```shell
   oc create configmap custom-config-toml --from-file config.toml=config.toml
   ```

1. Modifiez le runner que vous souhaitez corriger en ajoutant le paramètre `config:` pour pointer vers le `ConfigMap` récemment créé (remplacez my-runner par le nom correct du pod runner).

   ```shell
   oc patch runner my-runner --type merge -p '{"spec": {"config": "custom-config-toml"}}'
   ```

Pour plus d'informations, consultez la [documentation Red Hat](https://access.redhat.com/solutions/7016013).

### Utiliser GitLab Runner conforme FIPS {#using-fips-compliant-gitlab-runner}

> [!note]
> Pour l'Operator, vous pouvez uniquement modifier l'image helper. Vous ne pouvez pas encore modifier l'image GitLab Runner. [Le ticket 28814](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28814) suit cette fonctionnalité.

Pour utiliser un [helper GitLab Runner conforme FIPS](../install/requirements.md#fips-compliant-gitlab-runner), modifiez l'image helper comme suit :

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  helperImage: gitlab/gitlab-runner-helper:ubi-fips
  concurrent: 2
```

#### Enregistrer GitLab Runner à l'aide d'un certificat autosigné {#register-gitlab-runner-by-using-a-self-signed-certificate}

Pour utiliser un certificat autosigné avec GitLab Self-Managed, créez un secret contenant le certificat CA que vous avez utilisé pour signer les certificats privés.

Le nom du secret est ensuite fourni en tant que CA dans la section spec du Runner :

```yaml
KIND:     Runner
VERSION:  apps.gitlab.com/v1beta2

FIELD:    ca <string>

DESCRIPTION:
     Name of tls secret containing the custom certificate authority (CA)
     certificates
```

Le secret peut être créé à l'aide de la commande suivante :

```shell
oc create secret generic mySecret --from-file=tls.crt=myCert.pem -o yaml
```

#### Enregistrer GitLab Runner avec une URL externe pointant vers une adresse IP {#register-gitlab-runner-with-an-external-url-that-points-to-an-ip-address}

Si le runner ne peut pas faire correspondre le certificat autosigné avec le nom d'hôte, il est possible qu'un message d'erreur apparaisse. Ce problème survient lorsque vous configurez GitLab Self-Managed pour utiliser une adresse IP (comme `###.##.##.##`) plutôt qu'un nom d'hôte :

```shell
[31;1mERROR: Registering runner... failed               [0;m  [31;1mrunner[0;m=A5abcdEF [31;1mstatus[0;m=couldn't execute POST against https://###.##.##.##/api/v4/runners:
Post https://###.##.##.##/api/v4/runners: x509: cannot validate certificate for ###.##.##.## because it doesn't contain any IP SANs
[31;1mPANIC: Failed to register the runner. You may be having network problems.[0;m
```

Pour résoudre ce problème :

1. Sur le serveur GitLab Self-Managed, modifiez `openssl` pour ajouter l'adresse IP au paramètre `subjectAltName` :

   ```shell
   # vim /etc/pki/tls/openssl.cnf

   [ v3_ca ]
   subjectAltName=IP:169.57.64.36 <---- Add this line. 169.57.64.36 is your GitLab server IP.
   ```

1. Ensuite, régénérez une CA autosignée avec les commandes ci-dessous :

   ```shell
   # cd /etc/gitlab/ssl
   # openssl req -x509 -nodes -days 3650 -newkey rsa:4096 -keyout /etc/gitlab/ssl/169.57.64.36.key -out /etc/gitlab/ssl/169.57.64.36.crt
   # openssl dhparam -out /etc/gitlab/ssl/dhparam.pem 4096
   # gitlab-ctl restart
   ```

1. Utilisez ce nouveau certificat pour générer un nouveau secret.

## Structure des correctifs {#patch-structure}

Chaque correctif de spécification se compose des propriétés suivantes :

| Paramètre     | Description |
|-------------|-------------|
| `name`      | Nom du correctif de spécification personnalisé. |
| `patchFile` | Chemin vers le fichier qui définit les modifications à appliquer à la spécification finale avant sa génération. Le fichier doit être un fichier JSON ou YAML. |
| `patch`     | Chaîne au format JSON ou YAML décrivant les modifications à appliquer à la spécification finale avant sa génération. |
| `patchType` | La stratégie utilisée pour appliquer les modifications spécifiées à la spécification. Les valeurs acceptées sont `merge`, `json` et `strategic` (par défaut). |

Vous ne pouvez pas définir à la fois `patchFile` et `patch` dans la même configuration de spécification.

## Application de correctifs au modèle de pod runner {#patching-the-runner-pod-template}

L'application de correctifs à la [spécification de pod](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-template-v1/#PodTemplateSpec) vous permet de personnaliser le déploiement de GitLab Runner en appliquant des correctifs au déploiement Kubernetes généré par l'opérateur. Les correctifs sont appliqués à la spécification du modèle de pod (`deployment.spec.template.spec`).

Vous pouvez contrôler les paramètres au niveau du pod, tels que :

- Demandes et limites de ressources
- Contextes de sécurité
- Montages de volumes et volumes
- Variables d'environnement
- Sélecteurs de nœuds et règles d'affinité
- Tolérances
- Configuration du nom d'hôte et du DNS

## Application de correctifs au modèle de déploiement runner {#patching-the-runner-deployment-template}

L'application de correctifs à la [spécification de déploiement](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/deployment-v1/#Deployment) vous permet de personnaliser le déploiement de GitLab Runner en appliquant des correctifs au déploiement Kubernetes généré par l'opérateur. Les correctifs sont appliqués à la spécification de déploiement (`deployment.spec`).

Vous pouvez contrôler les paramètres au niveau du déploiement, tels que :

- Nombre de réplicas
- Stratégie de déploiement (RollingUpdate, Recreate)
- Limites de l'historique des révisions
- Secondes de délai de progression
- Labels et annotations

## Ordre des correctifs {#patch-order}

Les correctifs de spécification de déploiement sont appliqués avant les correctifs de spécification de pod. Cela signifie que si les spécifications de déploiement et de pod modifient toutes les deux le même champ, la spécification de pod a la priorité.

## Exemples {#examples}

### Exemple d'application de correctifs à la spécification de pod {#pod-specification-patching-example}

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  podSpec:
    - name: "set-hostname"
      patch: |
        hostname: "custom-hostname"
      patchType: "merge"
    - name: "add-resource-requests"
      patch: |
        containers:
        - name: build
          resources:
            requests:
              cpu: "500m"
              memory: "256Mi"
      patchType: "strategic"
```

### Exemple d'application de correctifs à la spécification de déploiement {#deployment-specification-patching-example}

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  deploymentSpec:
    - name: "set-replicas"
      patch: |
        replicas: 3
      patchType: "strategic"
    - name: "configure-strategy"
      patch: |
        strategy:
          type: RollingUpdate
          rollingUpdate:
            maxUnavailable: 25%
            maxSurge: 50%
      patchType: "strategic"
    - name: "set-revision-history"
      patch: |
        [{"op": "add", "path": "/revisionHistoryLimit", "value": 10}]
      patchType: "json"
```

## Bonnes pratiques {#best-practices}

- Testez les correctifs dans un environnement hors production avant de les appliquer aux déploiements en production.
- Utilisez des correctifs au niveau du déploiement pour les paramètres qui affectent le comportement du déploiement plutôt que les paramètres de pod individuels.
- N'oubliez pas que les correctifs de spécification de pod remplacent les correctifs de spécification de déploiement pour les champs en conflit.
