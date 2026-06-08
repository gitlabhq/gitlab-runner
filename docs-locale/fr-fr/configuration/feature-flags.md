---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Feature flags de GitLab Runner
---

> [!warning]
> Des corruptions de données, des dégradations de stabilité, des dégradations de performances et des problèmes de sécurité peuvent survenir si vous activez une fonctionnalité désactivée par défaut. Avant d'activer des feature flags, vous devez connaître les risques encourus. Pour plus d'informations, voir [Risques liés à l'activation de fonctionnalités encore en développement](https://docs.gitlab.com/administration/feature_flags/#risks-when-enabling-features-still-in-development).

Les feature flags sont des bascules qui vous permettent d'activer ou de désactiver des fonctionnalités spécifiques. Ces flags sont généralement utilisés :

- Pour les fonctionnalités bêta mises à disposition des volontaires pour les tester, mais qui ne sont pas encore prêtes à être activées pour tous les utilisateurs.

  Les fonctionnalités bêta sont parfois incomplètes ou nécessitent des tests supplémentaires. Un utilisateur qui souhaite utiliser une fonctionnalité bêta peut choisir d'accepter le risque et d'activer explicitement la fonctionnalité avec un feature flag. Les autres utilisateurs qui n'ont pas besoin de la fonctionnalité ou qui ne souhaitent pas accepter le risque sur leur système ont la fonctionnalité désactivée par défaut et ne sont pas impactés par les éventuels bugs et régressions.

- Pour les changements non rétrocompatibles qui entraînent la dépréciation de fonctionnalités ou la suppression de fonctionnalités dans un avenir proche.

  Au fur et à mesure que le produit évolue, les fonctionnalités sont parfois modifiées ou entièrement supprimées. Les bugs connus sont souvent corrigés, mais dans certains cas, les utilisateurs ont déjà trouvé une solution de contournement pour un bug qui les affectait ; forcer les utilisateurs à adopter le correctif standardisé pourrait causer d'autres problèmes avec leurs configurations personnalisées.

  Dans de tels cas, le feature flag est utilisé pour passer à la demande de l'ancien comportement au nouveau. Cela permet aux utilisateurs d'adopter de nouvelles versions du produit tout en leur laissant le temps de planifier une transition douce et permanente de l'ancien comportement vers le nouveau.

Les feature flags sont activés ou désactivés à l'aide de variables d'environnement. Pour :

- Activer un feature flag, définissez la variable d'environnement correspondante sur `"true"` ou `1`.
- Désactiver un feature flag, définissez la variable d'environnement correspondante sur `"false"` ou `0`.

## Feature flags disponibles {#available-feature-flags}

<!--
The list of feature flags is created automatically.
If you need to update it, call `make update_feature_flags_docs` in the
root directory of this project.
The flags are defined in `./helpers/featureflags/flags.go` file.
-->

<!-- feature_flags_list_start -->

| Feature flag | Valeur par défaut | Déprécié | À supprimer avec | Description |
|--------------|---------------|------------|--------------------|-------------|
| `FF_NETWORK_PER_BUILD` | `false` | {{< icon name="dotted-circle" >}} Non |  | Active la création d'un [réseau Docker par build](../executors/docker.md#network-configurations) avec l'exécuteur `docker`. Utilisez la variable `CI_BUILD_NETWORK_NAME` pour obtenir le nom du réseau. |
| `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est défini sur `false`, désactive l'exécution des commandes Kubernetes distantes via `exec` au profit de `attach` pour résoudre des problèmes tels que le [ticket 4119](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4119). Ce feature flag requiert que le compte de service dispose d'autorisations spécifiques. Pour plus d'informations, voir [configurer les autorisations de l'API du runner](../executors/kubernetes/_index.md#configure-runner-api-permissions). |
| `FF_USE_DIRECT_DOWNLOAD` | `true` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est défini sur `true`, le runner tente de télécharger directement tous les artefacts au lieu de les faire transiter par GitLab lors du premier essai. L'activation peut entraîner des échecs de téléchargement en raison d'un problème de validation du certificat TLS du stockage d'objets s'il est activé par GitLab. Voir [Certificats auto-signés ou autorités de certification personnalisées](tls-self-signed.md) |
| `FF_SKIP_NOOP_BUILD_STAGES` | `true` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est défini sur `false`, toutes les étapes du build sont exécutées même si leur exécution n'a aucun effet |
| `FF_USE_FASTZIP` | `false` | {{< icon name="dotted-circle" >}} Non |  | Fastzip est un archiveur performant pour l'archivage et l'extraction de caches/artefacts |
| `FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} Non |  | Si activé, supprime l'utilisation de l'appel `umask 0000` pour les jobs exécutés avec l'exécuteur `docker`. À la place, le runner tentera de découvrir l'UID et le GID de l'utilisateur configuré pour l'image utilisée par le conteneur de build et modifiera la propriété du répertoire de travail et des fichiers en exécutant la commande `chmod` dans le conteneur prédéfini (après la mise à jour des sources, la restauration du cache et le téléchargement des artefacts). L'utilitaire POSIX `id` doit être installé et opérationnel dans l'image de build pour ce feature flag. Le runner exécutera `id` avec les options `-u` et `-g` pour récupérer l'UID et le GID. |
| `FF_ENABLE_BASH_EXIT_CODE_CHECK` | `false` | {{< icon name="dotted-circle" >}} Non |  | Si activé, les scripts bash ne s'appuient pas uniquement sur `set -e`, mais vérifient un code de sortie non nul après l'exécution de chaque commande du script. |
| `FF_USE_WINDOWS_LEGACY_PROCESS_STRATEGY` | `false` | {{< icon name="dotted-circle" >}} Non |  | Dans GitLab Runner 16.10 et versions ultérieures, la valeur par défaut est `false`. Dans GitLab Runner 16.9 et versions antérieures, la valeur par défaut est `true`. Lorsqu'il est désactivé, les processus que le runner crée sur Windows (shell et exécuteur personnalisé) seront créés avec une configuration supplémentaire qui devrait améliorer la terminaison des processus. Lorsqu'il est défini sur `true`, la configuration de processus héritée est utilisée. Pour vider correctement et sans erreur un runner Windows, ce feature flag doit être défini sur `false`. |
| `FF_USE_NEW_BASH_EVAL_STRATEGY` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est défini sur `true`, l'appel Bash `eval` est exécuté dans un sous-shell pour faciliter la détection correcte du code de sortie du script exécuté. |
| `FF_USE_POWERSHELL_PATH_RESOLVER` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, PowerShell résout les noms de chemins plutôt que le runner qui utilise des fonctions de chemin de fichier spécifiques au système d'exploitation de l'hôte du runner. |
| `FF_USE_DYNAMIC_TRACE_FORCE_SEND_INTERVAL` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, l'intervalle d'envoi forcé de la trace pour les logs est ajusté dynamiquement en fonction de l'intervalle de mise à jour de la trace. |
| `FF_SCRIPT_SECTIONS` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, les commandes de script multi-lignes apparaissent comme des sections réductibles dans le job log, tandis que les commandes sur une seule ligne sont affichées directement avec le préfixe `$`. Il s'agit d'un problème connu. Pour plus d'informations, voir [ticket 39294](https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/39294). |
| `FF_ENABLE_JOB_CLEANUP` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, le répertoire du projet sera nettoyé à la fin du build. Si `GIT_CLONE` est utilisé, l'intégralité du répertoire du projet sera supprimée. Si `GIT_FETCH` est utilisé, une série de commandes Git `clean` sera émise. |
| `FF_KUBERNETES_HONOR_ENTRYPOINT` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, le point d'entrée Docker d'une image sera respecté si `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY` n'est pas défini sur true. Ce feature flag requiert que le compte de service dispose d'autorisations spécifiques. Pour plus d'informations, voir [configurer les autorisations de l'API du runner](../executors/kubernetes/_index.md#configure-runner-api-permissions). |
| `FF_POSIXLY_CORRECT_ESCAPES` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, les [échappements shell POSIX](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_02) sont utilisés plutôt que la [citation ANSI-C de style `bash`](https://www.gnu.org/software/bash/manual/html_node/Quoting.html). Cela doit être activé si l'environnement du job utilise un shell conforme à POSIX. |
| `FF_RESOLVE_FULL_TLS_CHAIN` | `false` | {{< icon name="dotted-circle" >}} Non |  | Dans GitLab Runner 16.4 et versions ultérieures, la valeur par défaut est `false`. Dans GitLab Runner 16.3 et versions antérieures, la valeur par défaut est `true`. Lorsqu'il est activé, le runner résout une chaîne TLS complète jusqu'à un certificat racine auto-signé pour `CI_SERVER_TLS_CA_FILE`. Cela était auparavant [nécessaire pour que les clones Git HTTPS fonctionnent](tls-self-signed.md#git-cloning) pour un client Git construit avec libcurl antérieur à la v7.68.0 et OpenSSL. Cependant, le processus de résolution des certificats peut échouer sur certains systèmes d'exploitation, tels que macOS, qui rejettent les certificats racines signés avec des algorithmes de signature plus anciens. Si la résolution du certificat échoue, vous devrez peut-être désactiver cette fonctionnalité. Ce feature flag ne peut être désactivé que dans la [configuration `[runners.feature_flags]`](#enable-feature-flag-in-runner-configuration). |
| `FF_DISABLE_POWERSHELL_STDIN` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, les scripts PowerShell pour les exécuteurs shell et personnalisés sont transmis par fichier, plutôt que transmis et exécutés via stdin. Cela est nécessaire pour que les mots-clés `allow_failure:exit_codes` des jobs fonctionnent correctement. |
| `FF_USE_POD_ACTIVE_DEADLINE_SECONDS` | `true` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, le [pod `activeDeadlineSeconds`](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#lifecycle) est défini sur le délai d'expiration du job CI/CD. Ce flag affecte le [cycle de vie du pod](../executors/kubernetes/_index.md#pod-lifecycle). |
| `FF_USE_ADVANCED_POD_SPEC_CONFIGURATION` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, l'utilisateur peut définir une spécification de pod entière dans le fichier `config.toml`. Pour plus d'informations, voir [Remplacer les spécifications de pod générées (Expérimental)](../executors/kubernetes/_index.md#overwrite-generated-pod-specifications). |
| `FF_SET_PERMISSIONS_BEFORE_CLEANUP` | `true` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, les permissions sur les répertoires et les fichiers du répertoire du projet sont définies en premier, afin de garantir que les suppressions lors du nettoyage se déroulent correctement. |
| `FF_SECRET_RESOLVING_FAILS_IF_MISSING` | `true` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, la résolution des secrets échoue si la valeur est introuvable. |
| `FF_PRINT_POD_EVENTS` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, tous les événements associés au pod de build sont affichés jusqu'à son démarrage. |
| `FF_USE_GIT_BUNDLE_URIS` | `true` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, l'option de configuration Git `transfer.bundleURI` est définie sur `true`. Ce feature flag est activé par défaut. Définissez sur `false` pour désactiver la prise en charge des bundles Git. |
| `FF_USE_GIT_NATIVE_CLONE` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé et que `GIT_STRATEGY=clone` est utilisé, la commande `git-clone(1)` est utilisée à la place de `git-init(1)` + `git-fetch(1)` pour cloner le projet. Cela nécessite Git version 2.49 et ultérieure, et revient à `init` + `fetch` si non disponible. |
| `FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, `dumb-init` est utilisé pour exécuter tous les scripts. Cela permet à `dumb-init` de s'exécuter comme premier processus dans le conteneur helper et de build. |
| `FF_USE_INIT_WITH_DOCKER_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, l'exécuteur Docker démarre les conteneurs de service et de build avec l'option `--init`, qui exécute `tini-init` en tant que PID 1. |
| `FF_LOG_IMAGES_CONFIGURED_FOR_JOB` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, le runner enregistre les noms de l'image et des images de service définis pour chaque job reçu. |
| `FF_USE_DOCKER_AUTOSCALER_DIAL_STDIO` | `true` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé (valeur par défaut), `docker system stdio` est utilisé pour établir un tunnel vers le démon Docker distant. Lorsqu'il est désactivé, un tunnel SSH natif est utilisé pour les connexions SSH, et un binaire helper 'fleeting-proxy' est d'abord déployé pour les connexions WinRM. |
| `FF_CLEAN_UP_FAILED_CACHE_EXTRACT` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, des commandes sont insérées dans les scripts de build pour détecter un échec d'extraction du cache et nettoyer les contenus partiels du cache laissés derrière. |
| `FF_USE_WINDOWS_JOB_OBJECT` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, un objet job est créé pour chaque processus que le runner crée sur Windows avec les exécuteurs shell et personnalisés. Pour forcer l'arrêt des processus, le runner ferme l'objet job. Cela devrait améliorer la terminaison des processus difficiles à arrêter. |
| `FF_TIMESTAMPS` | `true` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est désactivé, les horodatages ne sont pas ajoutés au début de chaque ligne de trace de log. |
| `FF_DISABLE_AUTOMATIC_TOKEN_ROTATION` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, il restreint la rotation automatique des jetons et enregistre un avertissement lorsque le jeton est sur le point d'expirer. |
| `FF_USE_LEGACY_GCS_CACHE_ADAPTER` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, l'adaptateur de cache GCS hérité est utilisé. Lorsqu'il est désactivé (par défaut), un adaptateur de cache GCS plus récent est utilisé, qui utilise le SDK de Google Cloud Storage pour l'authentification. Cela devrait résoudre les problèmes d'authentification dans les environnements avec lesquels l'adaptateur hérité avait des difficultés, tels que les configurations d'identité de charge de travail dans GKE. |
| `FF_DISABLE_UMASK_FOR_KUBERNETES_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, supprime l'appel `umask 0000` pour les jobs exécutés avec l'exécuteur Kubernetes. À la place, le runner tente de découvrir l'identifiant utilisateur (UID) et l'identifiant de groupe (GID) de l'utilisateur sous lequel le conteneur de build s'exécute. Le runner modifie également la propriété du répertoire de travail et des fichiers en exécutant la commande `chown` dans le conteneur prédéfini (après la mise à jour des sources, la restauration du cache et le téléchargement des artefacts). |
| `FF_USE_LEGACY_S3_CACHE_ADAPTER` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, l'adaptateur de cache S3 hérité est utilisé. Lorsqu'il est désactivé (par défaut), un adaptateur de cache S3 plus récent est utilisé, qui utilise le SDK S3 d'Amazon pour l'authentification. Cela devrait résoudre les problèmes d'authentification dans les environnements avec lesquels l'adaptateur hérité avait des difficultés, tels que les points de terminaison STS personnalisés. |
| `FF_GIT_URLS_WITHOUT_TOKENS` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, GitLab Runner n'intègre le jeton de job nulle part lors de la configuration Git ou de l'exécution de commandes. À la place, il configure un assistant d'informations d'identification Git qui utilise la variable d'environnement pour obtenir le jeton de job. Cette approche limite le stockage des jetons et réduit le risque de fuite de jetons. |
| `FF_WAIT_FOR_POD_TO_BE_REACHABLE` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, le runner attend que le statut du pod soit 'Running' et que le pod soit prêt avec ses certificats attachés. Pour plus d'informations, voir [configurer les autorisations de l'API du runner](../executors/kubernetes/_index.md#configure-runner-api-permissions). |
| `FF_MASK_ALL_DEFAULT_TOKENS` | `true` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, GitLab Runner masque automatiquement tous les modèles de jetons par défaut. |
| `FF_EXPORT_HIGH_CARDINALITY_METRICS` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, le runner exporte les métriques à haute cardinalité. Une attention particulière doit être portée lors de l'activation de ce feature flag pour éviter d'ingérer de grandes quantités de données. Pour plus d'informations, voir [Mise à l'échelle de la flotte](../fleet_scaling/_index.md). |
| `FF_USE_FLEETING_ACQUIRE_HEARTBEATS` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, la connectivité de l'instance fleeting est vérifiée avant qu'un job soit assigné à une instance. |
| `FF_USE_EXPONENTIAL_BACKOFF_STAGE_RETRY` | `true` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, les nouvelles tentatives pour `GET_SOURCES_ATTEMPTS`, `ARTIFACT_DOWNLOAD_ATTEMPTS`, `RESTORE_CACHE_ATTEMPTS` et `EXECUTOR_JOB_SECTION_ATTEMPTS` utilisent un backoff exponentiel (5 s - 5 min). |
| `FF_USE_ADAPTIVE_REQUEST_CONCURRENCY` | `true` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, le paramètre `request_concurrency` devient la valeur de concurrence maximale, et le nombre de requêtes simultanées s'ajuste en fonction du taux de requêtes de job réussies. |
| `FF_USE_GITALY_CORRELATION_ID` | `true` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, l'en-tête `X-Gitaly-Correlation-ID` est ajouté à toutes les requêtes HTTP Git. Lorsqu'il est désactivé, les opérations Git s'exécutent sans en-têtes Gitaly Correlation ID. |
| `FF_USE_GIT_PROACTIVE_AUTH` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, le runner passe l'option de configuration Git `http.proactiveAuth=basic` aux commandes `git clone` et `git fetch`. Par conséquent, Git envoie les informations d'identification de manière proactive au lieu d'attendre une réponse `401`. Ce comportement garantit que le nom d'utilisateur est propagé vers Gitaly pour les projets publics. |
| `FF_HASH_CACHE_KEYS` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsque GitLab Runner crée ou extrait des caches, il hache les clés de cache (SHA256) avant de les utiliser, aussi bien pour les caches locaux que distribués (par exemple, S3). Pour plus d'informations, voir [gestion des clés de cache](advanced-configuration.md#cache-key-handling). |
| `FF_ENABLE_JOB_INPUTS_INTERPOLATION` | `true` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, les entrées de job sont interpolées. Pour plus d'informations, voir [&17833](https://gitlab.com/groups/gitlab-org/-/epics/17833). |
| `FF_USE_JOB_ROUTER` | `false` | {{< icon name="dotted-circle" >}} Non |  | Permet à GitLab Runner de récupérer des jobs en se connectant au Job Router plutôt qu'à GitLab directement. |
| `FF_SCRIPT_TO_STEP_MIGRATION` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, les scripts utilisateur sont migrés vers des étapes et exécutés avec le step-runner. |
| `FF_USE_PARALLEL_CACHE_TRANSFER` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, les chargements et téléchargements de cache utilisent des transferts de stockage d'objets parallèles : Les écritures GoCloud utilisent le multipart avec des parties simultanées ; les téléchargements utilisent des lectures HTTP Range ou GoCloud range concurrentes. Lorsqu'il est désactivé, les chargements utilisent un flux de partie unique simultané et les téléchargements utilisent un seul flux. Améliore le débit sur les liens à haute bande passante lorsqu'il est activé. Ajustez avec `CACHE_CONCURRENCY` et `CACHE_CHUNK_SIZE`. |
| `FF_USE_PARALLEL_ARTIFACT_TRANSFER` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, les téléchargements d'artefacts qui utilisent `direct_download` et reçoivent une redirection vers le stockage d'objets peuvent utiliser des GET HTTP Range parallèles lorsque le backend prend en charge `206 Partial Content` avec un total `Content-Range`. Lorsqu'il est désactivé, un seul flux de téléchargement est utilisé. La taille des blocs et la concurrence sont fixes dans le runner (pas de variables `CACHE_*`). |
| `FF_CONCRETE` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, l'exécution de scripts traditionnelle est migrée vers et exécutée avec le step-runner. |
| `FF_SUSPENDABLE_ENVIRONMENTS` | `false` | {{< icon name="dotted-circle" >}} Non |  | Lorsqu'il est activé, vous pouvez suspendre ou reprendre des environnements de job. |

<!-- feature_flags_list_end -->

## Activer un feature flag dans la configuration du pipeline {#enable-feature-flag-in-pipeline-configuration}

Vous pouvez utiliser des [variables CI/CD](https://docs.gitlab.com/ci/variables/) pour activer des feature flags :

- Pour tous les jobs du pipeline (globalement) :

  ```yaml
  variables:
    FEATURE_FLAG_NAME: 1
  ```

- Pour un seul job :

  ```yaml
  job:
    stage: test
    variables:
      FEATURE_FLAG_NAME: 1
    script:
    - echo "Hello"
  ```

## Activer un feature flag dans les variables d'environnement du runner {#enable-feature-flag-in-runner-environment-variables}

Pour activer la fonctionnalité pour chaque job qu'un runner exécute, spécifiez le feature flag comme variable [`environment`](advanced-configuration.md#the-runners-section) dans la [configuration du runner](advanced-configuration.md) :

```toml
[[runners]]
  name = "example-runner"
  url = "https://gitlab.com/"
  token = "TOKEN"
  limit = 0
  executor = "docker"
  builds_dir = ""
  shell = ""
  environment = ["FEATURE_FLAG_NAME=1"]
```

## Activer un feature flag dans la configuration du runner {#enable-feature-flag-in-runner-configuration}

Vous pouvez activer des feature flags en les spécifiant sous `[runners.feature_flags]`. Ce paramètre empêche tout job de remplacer les valeurs des feature flags.

Certains feature flags ne sont également utilisables que lorsque vous configurez ce paramètre, car ils ne concernent pas la façon dont le job est exécuté.

```toml
[[runners]]
  name = "example-runner"
  url = "https://gitlab.com/"
  token = "TOKEN"
  executor = "docker"
  [runners.feature_flags]
    FF_USE_DIRECT_DOWNLOAD = true
```
