---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Configuration avancée
---

{{< details >}}

- Niveau : Free, Premium, Ultimate
- Offre : GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Pour modifier le comportement de GitLab Runner et des runners enregistrés individuellement, modifiez le fichier `config.toml`.

Vous pouvez trouver le fichier `config.toml` dans :

- `/etc/gitlab-runner/` sur les systèmes *nix lorsque GitLab Runner est exécuté en tant que root. Ce répertoire est également le chemin pour la configuration du service.
- `~/.gitlab-runner/` sur les systèmes *nix lorsque GitLab Runner est exécuté en tant que non-root.
- `./` sur les autres systèmes.

GitLab Runner ne nécessite pas de redémarrage lorsque vous modifiez la plupart des options. Cela inclut les paramètres de la section `[[runners]]` et la plupart des paramètres de la section globale, à l'exception de `listen_address`. Si un runner était déjà enregistré, vous n'avez pas besoin de l'enregistrer à nouveau.

GitLab Runner vérifie les modifications de configuration toutes les 3 secondes et recharge si nécessaire. GitLab Runner recharge également la configuration en réponse au signal `SIGHUP`.

## Validation de la configuration {#configuration-validation}

{{< history >}}

- [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3924) dans GitLab Runner 15.10

{{< /history >}}

La validation de la configuration est un processus qui vérifie la structure du fichier `config.toml`. La sortie du validateur de configuration fournit uniquement des messages de niveau `info`.

Le processus de validation de la configuration est uniquement à titre informatif. Vous pouvez utiliser la sortie pour identifier les problèmes potentiels avec la configuration de votre runner. La validation de la configuration peut ne pas détecter tous les problèmes possibles, et l'absence de messages ne garantit pas que le fichier `config.toml` est irréprochable.

## La section globale {#the-global-section}

Ces paramètres sont globaux. Ils s'appliquent à tous les runners.

| Paramètre              | Description |
|----------------------|-------------|
| `concurrent`         | Limite le nombre de jobs pouvant s'exécuter simultanément, sur tous les runners enregistrés. Chaque section `[[runners]]` peut définir sa propre limite, mais cette valeur fixe un maximum pour toutes ces valeurs combinées. Par exemple, une valeur de `10` signifie qu'au plus 10 jobs peuvent s'exécuter simultanément. `0` est interdit. Si vous utilisez cette valeur, le processus du runner se termine avec une erreur critique. Consultez le fonctionnement de ce paramètre avec l'[exécuteur Docker Machine](autoscale.md#limit-the-number-of-vms-created-by-the-docker-machine-executor) , l'[exécuteur Instance](../executors/instance.md) , l'[exécuteur Docker Autoscaler](../executors/docker_autoscaler.md#example-aws-autoscaling-for-1-job-per-instance) et la [configuration `runners.custom_build_dir`](#the-runnerscustom_build_dir-section). |
| `log_level`          | Définit le niveau de journalisation. Les options sont `debug`, `info`, `warn`, `error`, `fatal` et `panic`. Ce paramètre a une priorité inférieure au niveau défini par les arguments de ligne de commande `--debug`, `-l` ou `--log-level`. |
| `log_format`         | Spécifie le format des journaux. Les options sont `runner`, `text` et `json`. Ce paramètre a une priorité inférieure au format défini par l'argument de ligne de commande `--log-format`. La valeur par défaut est `runner`, qui contient des codes d'échappement ANSI pour la coloration. |
| `check_interval`     | Définit la durée de l'intervalle, en secondes, entre les vérifications du runner pour les nouveaux jobs. La valeur par défaut est `3`. Si défini à `0` ou inférieur, la valeur par défaut est utilisée. |
| `sentry_dsn`         | Active le suivi de toutes les erreurs au niveau système dans Sentry. |
| `connection_max_age` | La durée maximale pendant laquelle une connexion TLS keepalive au serveur GitLab doit rester ouverte avant de se reconnecter. La valeur par défaut est `15m` pour 15 minutes. Si défini à `0` ou inférieur, la connexion persiste aussi longtemps que possible. |
| `listen_address`     | Définit une adresse (`<host>:<port>`) sur laquelle le serveur HTTP de métriques Prometheus doit écouter. |
| `shutdown_timeout`   | Nombre de secondes avant que l'[opération d'arrêt forcé](../commands/_index.md#signals) n'expire et ne quitte le processus. La valeur par défaut est `30`. Si défini à `0` ou inférieur, la valeur par défaut est utilisée. |

### Avertissements de configuration {#configuration-warnings}

#### Problèmes de long polling {#long-polling-issues}

GitLab Runner peut rencontrer des problèmes de long polling dans plusieurs scénarios de configuration lorsque le long polling de GitLab est activé via GitLab Workhorse. Ces problèmes vont des goulots d'étranglement des performances aux délais de traitement importants, selon la configuration. Les workers de GitLab Runner peuvent rester bloqués dans des requêtes de long polling pendant de longues périodes (correspondant à la configuration GitLab Workhorse `-apiCiLongPollingDuration`, qui est par défaut de 50 secondes), empêchant d'autres jobs d'être traités rapidement.

Ce problème est lié à la fonctionnalité de long polling CI/CD de GitLab, qui est contrôlée par le paramètre GitLab Workhorse `-apiCiLongPollingDuration`. Lorsqu'il est activé, les requêtes de jobs peuvent être bloquées jusqu'à la durée configurée pendant qu'elles attendent que des jobs deviennent disponibles.

La valeur de configuration du long polling de GitLab Workhorse par défaut est de 50 secondes (activée par défaut dans les versions récentes de GitLab).

Voici quelques exemples de configuration :

- Omnibus : `gitlab_workhorse['api_ci_long_polling_duration'] = "50s"` dans `/etc/gitlab/gitlab.rb`
- Helm chart :  Utilisez le paramètre `gitlab.webservice.workhorse.extraArgs`
- CLI : `gitlab-workhorse -apiCiLongPollingDuration 50s`

Pour plus d'informations, voir :

- [Long polling pour les runners](https://docs.gitlab.com/ci/runners/long_polling/)
- [Configuration de Workhorse](https://docs.gitlab.com/development/workhorse/configuration/)

Symptômes :

- Les jobs de certains projets subissent des délais avant de démarrer (la durée correspond au délai d'expiration du long polling de votre instance GitLab)
- Les jobs d'autres projets s'exécutent immédiatement
- Message d'avertissement dans les journaux du runner : `CONFIGURATION: Long polling issues detected`

Scénarios problématiques courants :

- Goulot d'étranglement par famine des workers : Le paramètre `concurrent` est inférieur au nombre de runners (goulot d'étranglement sévère)
- Goulot d'étranglement des requêtes : Les runners avec `request_concurrency=1` provoquent des délais de jobs pendant le long polling
- Goulot d'étranglement de la limite de build : Les runners avec des paramètres `limit` faibles (≤2) combinés avec `request_concurrency=1`

GitLab Runner détecte automatiquement les scénarios problématiques et fournit des solutions adaptées dans les messages d'avertissement. Les solutions courantes incluent :

- Augmenter le paramètre `concurrent` pour dépasser le nombre de runners.
- Définir la valeur `request_concurrency` pour les runners à fort volume à une valeur supérieure à 1 (la valeur par défaut est 1). Envisagez d'activer [la surveillance des runners](../monitoring/_index.md) pour comprendre l'état de votre système et trouver la meilleure valeur pour le paramètre. Envisagez d'utiliser le `FF_USE_ADAPTIVE_REQUEST_CONCURRENCY` pour ajuster automatiquement `request_concurrency` en fonction de la charge de travail. Pour des informations sur la concurrence adaptative, consultez la [documentation sur les feature flags](feature-flags.md).
- Équilibrer les paramètres `limit` avec le volume de jobs attendu.

##### Exemples de configurations problématiques {#example-problematic-configurations}

Scénario 1 : Goulot d'étranglement par famine des workers :

```toml
concurrent = 2  # Only 2 concurrent workers

[[runners]]
  name = "runner-1"
[[runners]]
  name = "runner-2"
[[runners]]
  name = "runner-3"  # 3 runners, only 2 workers - severe bottleneck
```

Scénario 2 : Goulot d'étranglement des requêtes :

```toml
concurrent = 4  # 4 workers available

[[runners]]
  name = "high-volume-runner"
  request_concurrency = 1  # Default: only 1 request at a time
  limit = 10               # Can handle 10 jobs, but only 1 request slot
```

Scénario 3 : Goulot d'étranglement de la limite de build :

```toml
concurrent = 4

[[runners]]
  name = "limited-runner"
  limit = 2                # Only 2 builds allowed
  request_concurrency = 1  # Only 1 request at a time
  # Creates severe bottleneck: builds at capacity + request slot blocked by long polling
```

##### Exemple de configuration corrigée {#example-corrected-configuration}

```toml
concurrent = 4  # Adequate worker capacity

[[runners]]
  name = "high-volume-runner"
  request_concurrency = 3  # Allow multiple simultaneous requests
  limit = 10

[[runners]]
  name = "balanced-runner"
  request_concurrency = 2
  limit = 5
```

Voici un exemple de configuration :

```toml

# Example `config.toml` file

concurrent = 100 # A global setting for job concurrency that applies to all runner sections defined in this `config.toml` file
log_level = "warning"
log_format = "text"
check_interval = 3 # Value in seconds

[[runners]]
  name = "first"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "shell"
  (...)

[[runners]]
  name = "second"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "docker"
  (...)

[[runners]]
  name = "third"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "docker-autoscaler"
  (...)

```

### Exemples de `log_format` (tronqués) {#log_format-examples-truncated}

#### `runner` {#runner}

```shell
Runtime platform                                    arch=amd64 os=darwin pid=37300 revision=HEAD version=development version
Starting multi-runner from /etc/gitlab-runner/config.toml...  builds=0
WARNING: Running in user-mode.
WARNING: Use sudo for system-mode:
WARNING: $ sudo gitlab-runner...

Configuration loaded                                builds=0
listen_address not defined, metrics & debug endpoints disabled  builds=0
[session_server].listen_address not defined, session endpoints disabled  builds=0
```

#### `text` {#text}

```shell
INFO[0000] Runtime platform                              arch=amd64 os=darwin pid=37773 revision=HEAD version="development version"
INFO[0000] Starting multi-runner from /etc/gitlab-runner/config.toml...  builds=0
WARN[0000] Running in user-mode.
WARN[0000] Use sudo for system-mode:
WARN[0000] $ sudo gitlab-runner...
INFO[0000]
INFO[0000] Configuration loaded                          builds=0
INFO[0000] listen_address not defined, metrics & debug endpoints disabled  builds=0
INFO[0000] [session_server].listen_address not defined, session endpoints disabled  builds=0
```

#### `json` {#json}

```shell
{"arch":"amd64","level":"info","msg":"Runtime platform","os":"darwin","pid":38229,"revision":"HEAD","time":"2025-06-05T15:57:35+02:00","version":"development version"}
{"builds":0,"level":"info","msg":"Starting multi-runner from /etc/gitlab-runner/config.toml...","time":"2025-06-05T15:57:35+02:00"}
{"level":"warning","msg":"Running in user-mode.","time":"2025-06-05T15:57:35+02:00"}
{"level":"warning","msg":"Use sudo for system-mode:","time":"2025-06-05T15:57:35+02:00"}
{"level":"warning","msg":"$ sudo gitlab-runner...","time":"2025-06-05T15:57:35+02:00"}
{"level":"info","msg":"","time":"2025-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"Configuration loaded","time":"2025-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"listen_address not defined, metrics \u0026 debug endpoints disabled","time":"2025-06-05T15:57:35+02:00"}
{"builds":0,"level":"info","msg":"[session_server].listen_address not defined, session endpoints disabled","time":"2025-06-05T15:57:35+02:00"}
```

### Fonctionnement de `check_interval` {#how-check_interval-works}

Si `config.toml` comporte plus d'une section `[[runners]]`, GitLab Runner contient une boucle qui planifie constamment des requêtes de jobs vers l'instance GitLab où GitLab Runner est configuré.

L'exemple suivant a un `check_interval` de 10 secondes et deux sections `[[runners]]` (`runner-1` et `runner-2`). GitLab Runner envoie une requête toutes les 10 secondes et se met en veille pendant cinq secondes :

1. Obtenir la valeur de `check_interval` (`10s`).
1. Obtenir la liste des runners (`runner-1`, `runner-2`).
1. Calculer l'intervalle de veille (`10s / 2 = 5s`).
1. Démarrer une boucle infinie :
   1. Demander un job pour `runner-1`.
   1. Se mettre en veille pendant `5s`.
   1. Demander un job pour `runner-2`.
   1. Se mettre en veille pendant `5s`.

Par défaut, lorsqu'un runner reçoit un job, il interroge à nouveau immédiatement pour obtenir plus de jobs jusqu'à ce qu'aucun job ne soit disponible ou que le nombre de jobs en cours atteigne `concurrent` ou `limit`. Pour modifier ce comportement, définissez `strict_check_interval` sur `true`. Lorsqu'il est activé, le runner respecte strictement l'intervalle de vérification et envoie une requête toutes les `check_interval` secondes (5 secondes dans cet exemple), que ce soit ou non qu'un job ait été reçu. Activez ce paramètre pour améliorer la distribution des jobs sur une flotte de runners et éviter qu'un runner ne gère la plupart des jobs pendant que les autres restent inactifs. Cependant, les jobs pourraient attendre plus longtemps dans la file d'attente.

Voici un exemple de configuration `check_interval` :

```toml
# Example `config.toml` file

concurrent = 100 # A global setting for job concurrency that applies to all runner sections defined in this `config.toml` file.
log_level = "warning"
log_format = "json"
check_interval = 10 # Value in seconds

[[runners]]
  name = "runner-1"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "shell"
  (...)

[[runners]]
  name = "runner-2"
  url = "Your Gitlab instance URL (for example, `https://gitlab.com`)"
  executor = "docker"
  (...)
```

Dans cet exemple, une requête de job du processus du runner est effectuée toutes les cinq secondes. Si `runner-1` et `runner-2` sont connectés à la même instance GitLab, cette instance GitLab reçoit également une nouvelle requête de ce runner toutes les cinq secondes.

Deux périodes de veille surviennent entre la première et la deuxième requête pour `runner-1`. Chaque période dure cinq secondes, il y a donc environ 10 secondes entre les requêtes successives pour `runner-1`. Il en va de même pour `runner-2`.

Si vous définissez davantage de runners, l'intervalle de veille est plus court. Cependant, une requête pour un runner est répétée après toutes les requêtes pour les autres runners et leurs périodes de veille.

## La section `[machine]` {#the-machine-section}

{{< history >}}

- Introduit dans GitLab Runner 18.10.

{{< /history >}}

La section `[machine]` configure les paramètres globaux pour le fournisseur d'`docker+machine`. Ces paramètres s'appliquent à tous les runners qui utilisent l'exécuteur `docker+machine`.

### La section `[machine.shutdown_drain]` {#the-machineshutdown_drain-section}

Lorsque le processus du runner s'arrête, les machines inactives du pool sont généralement laissées à s'exécuter. Vous devez les nettoyer en externe (par exemple, via un hook post-stop `systemd`). La section `shutdown_drain` configure le runner pour supprimer automatiquement les machines inactives lors de l'arrêt.

| Paramètre       | Type     | Description |
|-----------------|----------|-------------|
| `enabled`       | booléen  | Activer la suppression automatique des machines inactives lors de l'arrêt. Par défaut : `false`. |
| `concurrency`   | entier  | Nombre de machines à supprimer simultanément. Par défaut : `3`. |
| `max_retries`   | entier  | Nombre maximum de tentatives par machine. Par défaut : `3`. |
| `retry_backoff` | durée | Durée de backoff de base entre les tentatives (multipliée par le numéro de tentative). Par défaut : `5s`. |

> [!note]
> L'opération de vidange utilise le paramètre global [`shutdown_timeout`](#the-global-section). Le délai d'expiration par défaut de 30 secondes est généralement trop court pour vider les machines. Lorsque vous activez le vidage lors de l'arrêt, augmentez `shutdown_timeout` pour laisser suffisamment de temps pour que toutes les machines soient supprimées. Un minimum de 5 minutes est recommandé, mais des pools plus grands peuvent nécessiter des délais d'expiration plus longs. Le runner consigne un avertissement si le délai d'expiration est trop court.

Exemple :

```toml
concurrent = 10
check_interval = 0
shutdown_timeout = 600  # 10 minutes - required for draining machines

[machine]
  [machine.shutdown_drain]
    enabled = true
    concurrency = 5
    max_retries = 3
    retry_backoff = "5s"

[[runners]]
  name = "my-runner"
  url = "https://gitlab.example.com/"
  token = "xxx"
  executor = "docker+machine"

  [runners.machine]
    IdleCount = 5
    IdleTime = 600
    MachineName = "auto-scale-%s"
    MachineDriver = "google"
    MachineOptions = ["google-project=my-project", "google-zone=us-central1-a"]
```

## La section `[session_server]` {#the-session_server-section}

Pour interagir avec les jobs, spécifiez la section `[session_server]` au niveau racine, en dehors de la section `[[runners]]`. Configurez cette section une fois pour tous les runners, et non pour chaque runner individuel.

```toml
# Example `config.toml` file with session server configured

concurrent = 100 # A global setting for job concurrency that applies to all runner sections defined in this `config.toml` file
log_level = "warning"
log_format = "runner"
check_interval = 3 # Value in seconds

[session_server]
  listen_address = "[::]:8093" # Listen on all available interfaces on port `8093`
  advertise_address = "runner-host-name.tld:8093"
  session_timeout = 1800
```

Lorsque vous configurez la section `[session_server]` :

- Pour `listen_address` et `advertise_address`, utilisez le format `host:port`, où `host` est l'adresse IP (`127.0.0.1:8093`) ou le domaine (`my-runner.example.com:8093`). Le runner utilise ces informations pour créer un certificat TLS pour une connexion sécurisée.
- Assurez-vous que GitLab peut se connecter à l'adresse IP et au port définis dans `listen_address` ou `advertise_address`.
- Assurez-vous que `advertise_address` est une adresse IP publique, sauf si vous avez activé le paramètre d'application [`allow_local_requests_from_web_hooks_and_services`](https://docs.gitlab.com/api/settings/#available-settings).

| Paramètre             | Description |
|---------------------|-------------|
| `listen_address`    | Une URL interne pour le serveur de session. |
| `advertise_address` | L'URL pour accéder au serveur de session. GitLab Runner l'expose à GitLab. Si non défini, `listen_address` est utilisé. |
| `session_timeout`   | Nombre de secondes pendant lesquelles la session peut rester active après la fin du job. Le délai d'expiration empêche le job de se terminer. La valeur par défaut est `1800` (30 minutes). |

Pour désactiver le serveur de session et la prise en charge du terminal, supprimez la section `[session_server]`.

> [!note]
> Lorsque votre instance de runner est déjà en cours d'exécution, vous devrez peut-être exécuter `gitlab-runner restart` pour que les modifications dans la section `[session_server]` prennent effet.

Si vous utilisez l'image Docker de GitLab Runner, vous devez exposer le port `8093` en ajoutant `-p 8093:8093` à votre [commande `docker run`](../install/docker.md).

## La section `[[runners]]` {#the-runners-section}

Chaque section `[[runners]]` définit un runner.

| Paramètre                               | Description                                                                                                                                                                                                                                                                                                                                                                                                 |
| ------------------------------------- |-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `name`                                | La description du runner. Informatif uniquement.                                                                                                                                                                                                                                                                                                                                                               |
| `url`                                 | URL de l'instance GitLab. Prend en charge l'expansion des variables d'environnement (par exemple, `$GITLAB_URL` ou `${GITLAB_URL}`).                                                                                                                                                                                                                                                                                               |
| `token`                               | Le jeton d'authentification du runner, qui est obtenu lors de l'enregistrement du runner. [Différent du jeton d'enregistrement](https://docs.gitlab.com/api/runners/#registration-and-authentication-tokens). Prend en charge l'expansion des variables d'environnement (par exemple, `$RUNNER_TOKEN` ou `${RUNNER_TOKEN}`).                                                                                                        |
| `tls-ca-file`                         | Lors de l'utilisation de HTTPS, fichier contenant les certificats pour vérifier le pair. Voir [la documentation sur les certificats auto-signés ou les autorités de certification personnalisées](tls-self-signed.md).                                                                                                                                                                                                                             |
| `tls-cert-file`                       | Lors de l'utilisation de HTTPS, fichier contenant le certificat pour s'authentifier auprès du pair.                                                                                                                                                                                                                                                                                                                         |
| `tls-key-file`                        | Lors de l'utilisation de HTTPS, fichier contenant la clé privée pour s'authentifier auprès du pair.                                                                                                                                                                                                                                                                                                                         |
| `limit`                               | Limite le nombre de jobs pouvant être traités simultanément par ce runner enregistré. `0` (par défaut) signifie pas de limite. Consultez le fonctionnement de ce paramètre avec les exécuteurs [Docker Machine](autoscale.md#limit-the-number-of-vms-created-by-the-docker-machine-executor), [Instance](../executors/instance.md) et [Docker Autoscaler](../executors/docker_autoscaler.md#example-aws-autoscaling-for-1-job-per-instance). |
| `executor`                            | L'environnement ou le processeur de commandes sur le système d'exploitation hôte que le runner utilise pour exécuter un job CI/CD. Pour plus d'informations, consultez [les exécuteurs](../executors/_index.md).                                                                                                                                                                                                                                   |
| `shell`                               | Nom du shell pour générer le script. La valeur par défaut [dépend de la plateforme](../shells/_index.md).                                                                                                                                                                                                                                                                                                           |
| `builds_dir`                          | Chemin absolu vers un répertoire où les builds sont stockés dans le contexte de l'exécuteur sélectionné. Par exemple, localement, Docker ou SSH.                                                                                                                                                                                                                                                                         |
| `cache_dir`                           | Chemin absolu vers un répertoire où les caches de build sont stockés dans le contexte de l'exécuteur sélectionné. Par exemple, localement, Docker ou SSH. Si l'exécuteur `docker` est utilisé, ce répertoire doit être inclus dans son paramètre `volumes`.                                                                                                                                                                         |
| `environment`                         | Ajouter ou remplacer des variables d'environnement.                                                                                                                                                                                                                                                                                                                                                                  |
| `request_concurrency`                 | Limiter le nombre de requêtes simultanées pour de nouveaux jobs depuis GitLab. La valeur par défaut est `1`. Pour plus d'informations sur la façon dont `concurrency`, `limit` et `request_concurrency` interagissent pour contrôler le flux des jobs, consultez l'[article de la base de connaissances sur l'ajustement de la concurrence de GitLab Runner](https://support.gitlab.com/hc/en-us/articles/21324350882076-GitLab-Runner-Concurrency-Tuning-Understanding-request-concurrency).                     |
| `strict_check_interval`               | En fonctionnement normal, lorsqu'un runner interroge les jobs et en reçoit un, il interroge immédiatement à nouveau jusqu'à ce que le nombre de jobs en cours de traitement corresponde à `concurrent` ou `limit`, ou jusqu'à ce qu'aucun job ne soit disponible. Lorsque vous activez `strict_check_interval`, le runner désactive cette boucle d'interrogation plus rapide que `check_interval` et respecte strictement `check_interval`. La valeur par défaut est `false`.             |
| `output_limit`                        | Taille maximale du journal de build en kilo-octets. La valeur par défaut est `4096` (4 Mo).                                                                                                                                                                                                                                                                                                                                              |
| `pre_get_sources_script`              | Commandes à exécuter sur le runner avant la mise à jour du dépôt Git et la mise à jour des sous-modules. Utilisez-le pour ajuster d'abord la configuration du client Git, par exemple. Pour insérer plusieurs commandes, utilisez une chaîne multiligne (entre guillemets triples) ou le caractère `\n`.                                                                                                                                                 |
| `post_get_sources_script`             | Commandes à exécuter sur le runner après la mise à jour du dépôt Git et la mise à jour des sous-modules. Pour insérer plusieurs commandes, utilisez une chaîne multiligne (entre guillemets triples) ou le caractère `\n`.                                                                                                                                                                                                                    |
| `pre_build_script`                    | Commandes à exécuter sur le runner avant d'exécuter le job. S'exécute dans le même contexte shell que `before_script`, `script` et `post_build_script`. Si `pre_build_script` échoue, les commandes restantes dans ce contexte sont ignorées, mais `after_script` s'exécute quand même. Pour insérer plusieurs commandes, utilisez une chaîne multiligne (entre guillemets triples) ou le caractère `\n`.                                               |
| `post_build_script`                   | Commandes à exécuter sur le runner après l'exécution du job. S'exécute dans le même contexte shell que `pre_build_script`, `before_script` et `script`. Si l'un d'eux échoue, `post_build_script` est ignoré. `after_script` s'exécute dans un contexte shell séparé et n'est pas affecté par `post_build_script`. Pour insérer plusieurs commandes, utilisez une chaîne multiligne (entre guillemets triples) ou le caractère `\n`.               |
| `clone_url`                           | Remplacer l'URL de l'instance GitLab. Utilisé uniquement si le runner ne peut pas se connecter à l'URL de GitLab.                                                                                                                                                                                                                                                                                                         |
| `debug_trace_disabled`                | Désactive [le traçage de débogage](https://docs.gitlab.com/ci/variables/#enable-debug-logging). Lorsque défini sur `true`, le journal de débogage (trace) reste désactivé même si `CI_DEBUG_TRACE` est défini sur `true`.                                                                                                                                                                                                                 |
| `clean_git_config`                    | Nettoie la configuration Git. Pour plus d'informations, voir [Nettoyage de la configuration Git](#cleaning-git-configuration).                                                                                                                                                                                                                                                                                          |
| `referees`                            | Workers de surveillance de jobs supplémentaires qui transmettent leurs résultats sous forme d'artefacts de job à GitLab.                                                                                                                                                                                                                                                                                                                            |
| `unhealthy_requests_limit`            | Le nombre de réponses `unhealthy` aux nouvelles requêtes de jobs après lequel un worker du runner est désactivé.                                                                                                                                                                                                                                                                                                            |
| `unhealthy_interval`                  | Durée pendant laquelle un worker du runner est désactivé après avoir dépassé la limite des requêtes non saines. Prend en charge la syntaxe telle que `3600 s`, `1 h 30 min` et similaires.                                                                                                                                                                                                                                                      |
| `job_status_final_update_retry_limit` | Le nombre maximum de fois que GitLab Runner peut réessayer d'envoyer le statut final du job à l'instance GitLab.                                                                                                                                                                                                                                                                                                    |
| `prepare_timeout`                     | Durée maximale autorisée pour l'étape `prepare` (initialisation de l'exécuteur et configuration de l'environnement shell). Accepte une chaîne de durée telle que `30s` ou `1h30m`. Si non défini, nul ou supérieur au délai d'expiration du job, utilise le délai d'expiration du job par défaut. Pour plus d'informations, voir [le délai d'expiration de l'étape de préparation](#prepare-stage-timeout).                                                                                        |

Exemple :

```toml
[[runners]]
  name = "example-runner"
  url = "http://gitlab.example.com/"
  token = "TOKEN"
  limit = 0
  executor = "docker"
  builds_dir = ""
  shell = ""
  environment = ["ENV=value", "LC_ALL=en_US.UTF-8"]
  clone_url = "http://gitlab.example.local"
```

### Utiliser des variables d'environnement pour les valeurs sensibles {#use-environment-variables-for-sensitive-values}

Vous pouvez utiliser des variables d'environnement dans les champs `token` et `url` pour éviter de stocker des valeurs sensibles directement dans le fichier de configuration. Les syntaxes `$VAR` et `${VAR}` sont toutes deux prises en charge.

```toml
[[runners]]
  name = "runner-1"
  url = "$GITLAB_URL"
  token = "${RUNNER_TOKEN_1}"
  executor = "docker"

[[runners]]
  name = "runner-2"
  url = "$GITLAB_URL"
  token = "${RUNNER_TOKEN_2}"
  executor = "docker"
```

Ceci est utile pour :

- Les déploiements Kubernetes où les jetons sont montés à partir de secrets
- Les déploiements Docker où les jetons sont passés en tant que variables d'environnement
- Éviter les secrets dans les fichiers de configuration gérés par le contrôle de version

### Suffixe d'URL `/ci` hérité {#legacy-ci-url-suffix}

{{< history >}}

- Déprécié dans [GitLab Runner 1.0.0](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/289).
- Avertissement ajouté dans GitLab Runner 18.7.0.

{{< /history >}}

Dans les versions de GitLab Runner antérieures à 1.0.0, l'URL du runner était configurée avec un suffixe `/ci`, tel que `url = "https://gitlab.example.com/ci"`. Ce suffixe n'est plus requis et doit être supprimé de votre configuration.

Si votre `config.toml` contient une URL avec le suffixe `/ci`, GitLab Runner le supprime automatiquement lors du traitement de la configuration. Cependant, vous devriez mettre à jour votre fichier de configuration pour supprimer le suffixe afin d'éviter des problèmes potentiels.

#### Problèmes connus {#known-issues}

- Échecs d'authentification des sous-modules Git : Lorsque `GIT_SUBMODULE_FORCE_HTTPS=true` est défini, les sous-modules pourraient ne pas cloner avec des erreurs d'authentification telles que `fatal: could not read Username for 'https://gitlab.example.com': terminal prompts disabled`. Ce problème survient parce que le suffixe `/ci` interfère avec les règles de réécriture d'URL Git. Pour plus de détails, voir [ticket 581678](https://gitlab.com/gitlab-org/gitlab/-/work_items/581678#note_2934077238).

**Configuration problématique** :

```toml
[[runners]]
  name = "legacy-runner"
  url = "https://gitlab.example.com/ci"  # Remove the /ci suffix
  token = "TOKEN"
  executor = "docker"
```

**Configuration corrigée** :

```toml
[[runners]]
  name = "legacy-runner"
  url = "https://gitlab.example.com"  # /ci suffix removed
  token = "TOKEN"
  executor = "docker"
```

Lorsque GitLab Runner démarre avec une URL contenant le suffixe `/ci`, il consigne un message d'avertissement :

```plaintext
WARNING: The runner URL contains a legacy '/ci' suffix. This suffix is deprecated and should be
removed from the configuration. Git submodules may fail to clone with authentication errors if this
suffix is present. Please update the 'url' field in your config.toml to remove the '/ci' suffix.
See https://docs.gitlab.com/runner/configuration/advanced-configuration/#legacy-ci-url-suffix for more information.
```

Pour résoudre cet avertissement, modifiez votre fichier `config.toml` et supprimez le suffixe `/ci` du champ `url`.

### Fonctionnement de `clone_url` {#how-clone_url-works}

Lorsque l'instance GitLab est disponible à une URL que le runner ne peut pas utiliser, vous pouvez configurer un `clone_url`.

Par exemple, un pare-feu pourrait empêcher le runner d'atteindre l'URL. Si le runner peut atteindre le nœud sur `192.168.1.23`, définissez le `clone_url` sur `http://192.168.1.23`.

Si le `clone_url` est défini, le runner construit une URL de clone de la forme `http://gitlab-ci-token:s3cr3tt0k3n@192.168.1.23/namespace/project.git`.

> [!note]
> `clone_url` n'affecte pas les points de terminaison Git LFS ni les chargements ou téléchargements d'artefacts.

#### Modifier les points de terminaison Git LFS {#modify-git-lfs-endpoints}

Pour modifier les points de terminaison [Git LFS](https://docs.gitlab.com/topics/git/lfs/), définissez `pre_get_sources_script` dans l'un des fichiers suivants :

- `config.toml` :

  ```toml
  pre_get_sources_script = "mkdir -p $RUNNER_TEMP_PROJECT_DIR/git-template; git config -f $RUNNER_TEMP_PROJECT_DIR/git-template/config lfs.url https://<alternative-endpoint>"
  ```

- `.gitlab-ci.yml` :

  ```yaml
  default:
    hooks:
      pre_get_sources_script:
        - mkdir -p $RUNNER_TEMP_PROJECT_DIR/git-template
        - git config -f $RUNNER_TEMP_PROJECT_DIR/git-template/config lfs.url https://localhost
  ```

### Fonctionnement de `unhealthy_requests_limit` et `unhealthy_interval` {#how-unhealthy_requests_limit-and-unhealthy_interval-works}

Lorsqu'une instance GitLab est indisponible pendant une longue période (par exemple, lors d'une mise à niveau de version), ses runners deviennent inactifs. Les runners ne reprennent pas le traitement des jobs pendant 30 à 60 minutes après que l'instance GitLab soit à nouveau disponible.

Pour augmenter ou diminuer la durée pendant laquelle les runners sont inactifs, modifiez le paramètre `unhealthy_interval`.

Pour modifier le nombre de tentatives de connexion du runner au serveur GitLab et recevoir une mise en veille non saine avant de devenir inactif, modifiez le paramètre `unhealthy_requests_limit`. Pour plus d'informations, voir [Fonctionnement de `check_interval`](advanced-configuration.md#how-check_interval-works).

### Délai d'expiration de l'étape de préparation {#prepare-stage-timeout}

{{< history >}}

- [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/26583) dans GitLab Runner 19.0.0.

{{< /history >}}

Le paramètre `prepare_timeout` limite la durée que le runner consacre à la préparation de l'environnement d'exécution avant d'exécuter vos scripts de job. L'étape de préparation couvre deux phases :

1. **Executor initialization** (`prepare_executor`) : Le runner configure l'environnement d'exécution, par exemple en démarrant un conteneur Docker, en planifiant un pod Kubernetes ou en se connectant via SSH.
1. **Shell environment setup** (`prepare_script`) : Le runner génère et exécute un script pour initialiser l'environnement shell (tel que PATH, les répertoires de travail et les fonctions shell) nécessaire pour les étapes de job suivantes.

Si l'étape de préparation dépasse `prepare_timeout`, le job échoue immédiatement. Les étapes suivantes (`get_sources`, `restore_cache`, `script`, etc.) ne sont pas limitées par `prepare_timeout`. Elles utilisent plutôt le délai d'expiration global du job.

**Default behavior** : Si `prepare_timeout` n'est pas défini, est `0` ou dépasse le délai d'expiration du job, le runner utilise le délai d'expiration du job pour l'étape de préparation.

#### Quand définir `prepare_timeout` {#when-to-set-prepare_timeout}

Définissez `prepare_timeout` lorsque l'initialisation lente ou non réactive de l'environnement risque de consommer la totalité du délai d'expiration du job avant que le job ne commence. Les scénarios courants incluent :

- **Docker image pulls** : Si un registre de conteneurs est lent ou inaccessible, les récupérations d'images peuvent rester bloquées pendant toute la durée du délai d'expiration du job. Sur les runners très sollicités, les récupérations bloquées remplissent tous les emplacements de jobs disponibles et empêchent le démarrage de nouveaux jobs. `prepare_timeout` fait échouer ces jobs rapidement pour libérer la capacité du runner.
- **Custom or HPC executors** : Lorsque l'exécuteur doit attendre qu'un planificateur de ressources externe alloue de la capacité (comme une file d'attente de jobs HPC), le démarrage peut être imprévisible et potentiellement très long. Sans `prepare_timeout`, les jobs bloqués occupent des emplacements du runner pendant toute la durée du délai d'expiration du job.

#### Exemple de configuration {#example-configuration}

```toml
[[runners]]
  name = "my-runner"
  url = "https://gitlab.example.com/"
  token = "TOKEN"
  executor = "docker"
  prepare_timeout = "5m"
```

## Les exécuteurs {#the-executors}

Les exécuteurs suivants sont disponibles.

| Exécuteur            | Configuration requise                                                  | Où les jobs s'exécutent |
|---------------------|-------------------------------------------------------------------------|----------------|
| `shell`             |                                                                         | Shell local. L'exécuteur par défaut. |
| `docker`            | `[runners.docker]` et [Docker Engine](https://docs.docker.com/engine/) | Un conteneur Docker. |
| `docker-windows`    | `[runners.docker]` et [Docker Engine](https://docs.docker.com/engine/) | Un conteneur Docker Windows. |
| `ssh`               | `[runners.ssh]`                                                         | SSH, à distance. |
| `parallels`         | `[runners.parallels]` et `[runners.ssh]`                               | VM Parallels, mais connexion via SSH. |
| `virtualbox`        | `[runners.virtualbox]` et `[runners.ssh]`                              | VM VirtualBox, mais connexion via SSH. |
| `docker+machine`    | `[runners.docker]` et `[runners.machine]`                              | Comme `docker`, mais utilise des [machines Docker à mise à l'échelle automatique](autoscale.md). |
| `kubernetes`        | `[runners.kubernetes]`                                                  | Pods Kubernetes. |
| `docker-autoscaler` | `[docker-autoscaler]` et `[runners.autoscaler]`                        | Comme `docker`, mais utilise des instances à mise à l'échelle automatique pour exécuter des jobs CI/CD dans des conteneurs. |
| `instance`          | `[docker-autoscaler]` et `[runners.autoscaler]`                        | Comme `shell`, mais utilise des instances à mise à l'échelle automatique pour exécuter des jobs CI/CD directement sur l'instance hôte. |

## Les shells {#the-shells}

Les jobs CI/CD s'exécutent localement sur la machine hôte lorsqu'ils sont configurés pour utiliser l'exécuteur shell. Les shells du système d'exploitation pris en charge sont :

| Shell        | Description |
|--------------|-------------|
| `bash`       | Génère un script Bash (Bourne-shell). Toutes les commandes sont exécutées dans le contexte Bash. Par défaut pour tous les systèmes Unix. |
| `sh`         | Génère un script Sh (Bourne-shell). Toutes les commandes sont exécutées dans le contexte Sh. Le fallback de `bash` pour tous les systèmes Unix. |
| `powershell` | Génère un script PowerShell. Toutes les commandes sont exécutées dans le contexte PowerShell Desktop. Shell par défaut pour les jobs sur Windows avec les exécuteurs `kubernetes` et `docker-windows`. |
| `pwsh`       | Génère un script PowerShell. Toutes les commandes sont exécutées dans le contexte PowerShell Core. Shell par défaut pour les nouveaux enregistrements de runners sur Windows, et pour les jobs avec l'exécuteur `shell`. |

Lorsque l'option `shell` est définie sur `bash` ou `sh`, le mécanisme de [citation ANSI-C](https://www.gnu.org/software/bash/manual/html_node/ANSI_002dC-Quoting.html) de Bash est utilisé pour échapper les scripts de jobs.

### Utiliser un shell conforme POSIX {#use-a-posix-compliant-shell}

Dans GitLab Runner 14.9 et versions ultérieures, [activez le feature flag](feature-flags.md) nommé `FF_POSIXLY_CORRECT_ESCAPES` pour utiliser un shell conforme POSIX (comme `dash`). Lorsqu'il est activé, le mécanisme ["Double Quotes"](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_02), qui est le mécanisme d'échappement shell conforme POSIX, est utilisé.

## La section `[runners.docker]` {#the-runnersdocker-section}

Les paramètres suivants définissent les paramètres du conteneur Docker. Ces paramètres sont applicables lorsque le runner est configuré pour utiliser l'exécuteur Docker.

[Docker-in-Docker](https://docs.gitlab.com/ci/docker/using_docker_build/#use-docker-in-docker) en tant que service, ou tout runtime de conteneur configuré dans un job, n'hérite pas de ces paramètres.

| Paramètre                          | Exemple                                          | Description |
|------------------------------------|--------------------------------------------------|-------------|
| `allowed_images`                   | `["ruby:*", "python:*", "php:*"]`                | Liste générique d'images pouvant être spécifiées dans le fichier `.gitlab-ci.yml`. Si absent, toutes les images sont autorisées (équivalent à `["*/*:*"]`). À utiliser avec les exécuteurs [Docker](../executors/docker.md#restrict-docker-images-and-services) ou [Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services). |
| `allowed_privileged_images`        |                                                  | Sous-ensemble générique de `allowed_images` qui s'exécute en mode privilégié lorsque `privileged` est activé. Si absent, toutes les images sont autorisées (équivalent à `["*/*:*"]`). À utiliser avec les exécuteurs [Docker](../executors/docker.md#restrict-docker-images-and-services). |
| `allowed_pull_policies`            |                                                  | Liste des politiques de tirage pouvant être spécifiées dans le fichier `.gitlab-ci.yml` ou le fichier `config.toml`. Si non spécifié, seules les politiques de tirage spécifiées dans `pull-policy` sont autorisées. À utiliser avec l'exécuteur [Docker](../executors/docker.md#allow-docker-pull-policies). |
| `allowed_services`                 | `["postgres:9", "redis:*", "mysql:*"]`           | Liste générique de services pouvant être spécifiés dans le fichier `.gitlab-ci.yml`. Si absent, toutes les images sont autorisées (équivalent à `["*/*:*"]`). À utiliser avec les exécuteurs [Docker](../executors/docker.md#restrict-docker-images-and-services) ou [Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services). |
| `allowed_privileged_services`      |                                                  | Sous-ensemble générique de `allowed_services` autorisé à s'exécuter en mode privilégié, lorsque `privileged` ou `services_privileged` est activé. Si absent, toutes les images sont autorisées (équivalent à `["*/*:*"]`). À utiliser avec les exécuteurs [Docker](../executors/docker.md#restrict-docker-images-and-services). |
| `cache_dir`                        |                                                  | Répertoire où les caches Docker doivent être stockés. Ce chemin peut être absolu ou relatif au répertoire de travail actuel. Voir `disable_cache` pour plus d'informations. |
| `cap_add`                          | `["NET_ADMIN"]`                                  | Ajouter des capacités Linux supplémentaires au conteneur. |
| `cap_drop`                         | `["DAC_OVERRIDE"]`                               | Supprimer des capacités Linux supplémentaires du conteneur. |
| `cpuset_cpus`                      | `"0,1"`                                          | Le `CpusetCpus` du groupe de contrôle. Une chaîne. |
| `cpuset_mems`                      | `"0,1"`                                          | Le `CpusetMems` du groupe de contrôle. Une chaîne. |
| `cpu_shares`                       |                                                  | Nombre de partages CPU utilisés pour définir l'utilisation relative du CPU. La valeur par défaut est `1024`. |
| `cpus`                             | `"2"`                                            | Nombre de CPU (disponible dans Docker 1.13 ou ultérieur). Une chaîne. |
| `devices`                          | `["/dev/net/tun"]`                               | Partager des périphériques hôtes supplémentaires avec le conteneur. |
| `device_cgroup_rules`              |                                                  | Règles `cgroup` de périphériques personnalisées (disponible dans Docker 1.28 ou ultérieur). |
| `disable_cache`                    |                                                  | L'exécuteur Docker dispose de deux niveaux de mise en cache : un global (comme tout autre exécuteur) et un cache local basé sur les volumes Docker. Cet indicateur de configuration n'agit que sur le cache local, ce qui désactive l'utilisation de volumes de cache créés automatiquement (non mappés à un répertoire hôte). En d'autres termes, cela empêche uniquement la création d'un conteneur contenant des fichiers temporaires de builds, cela ne désactive pas le cache si le runner est configuré en [mode cache distribué](autoscale.md#distributed-runners-caching). |
| `disable_entrypoint_overwrite`     |                                                  | Désactiver le remplacement du point d'entrée de l'image. |
| `dns`                              | `["8.8.8.8"]`                                    | Liste de serveurs DNS que le conteneur doit utiliser. |
| `dns_search`                       |                                                  | Liste de domaines de recherche DNS. |
| `extra_hosts`                      | `["other-host:127.0.0.1"]`                       | Hôtes devant être définis dans l'environnement du conteneur. |
| `gpus`                             |                                                  | Périphériques GPU pour le conteneur Docker. Utilise le même format que la CLI `docker`. Voir les détails dans la [documentation Docker](https://docs.docker.com/engine/containers/resource_constraints/#gpu). Nécessite une [configuration pour activer les GPU](gpus.md#docker-executor). |
| `group_add`                        | `["docker"]`                                     | Ajouter des groupes supplémentaires pour le processus du conteneur. |
| `helper_image`                     |                                                  | (Avancé) [L'image d'aide par défaut](#helper-image) utilisée pour cloner les dépôts et télécharger les artefacts. |
| `helper_image_flavor`              |                                                  | Définit la variante de l'image d'aide (`alpine`, `alpine3.21`, `alpine-latest`, `ubi-fips` ou `ubuntu`). Par défaut `alpine`. La variante `alpine` utilise la même version que `alpine-latest`. |
| `helper_image_autoset_arch_and_os` |                                                  | Utilise le système d'exploitation sous-jacent pour définir l'architecture et le système d'exploitation de l'image d'aide. |
| `host`                             |                                                  | Point de terminaison Docker personnalisé. La valeur par défaut est l'environnement `DOCKER_HOST` ou `unix:///var/run/docker.sock`. |
| `hostname`                         |                                                  | Nom d'hôte personnalisé pour le conteneur Docker. |
| `image`                            | `"ruby:3.3"`                                     | L'image avec laquelle exécuter les jobs. |
| `links`                            | `["mysql_container:mysql"]`                      | Conteneurs qui doivent être liés au conteneur qui exécute le job. |
| `log_options`                      | `{"env": "GITLAB_CI_JOB_ID,GITLAB_CI_JOB_NAME", "labels": "com.gitlab.gitlab-runner.type"}` | Options du pilote de journalisation pour les conteneurs Docker utilisant le pilote de journalisation `json-file`. Seules les options `env` et `labels` sont autorisées. Pour plus d'informations, voir [les options de journalisation Docker](#docker-log-options). |
| `memory`                           | `"128m"`                                         | La limite de mémoire. Une chaîne. |
| `memory_swap`                      | `"256m"`                                         | La limite totale de mémoire. Une chaîne. |
| `memory_reservation`               | `"64m"`                                          | La limite de mémoire souple. Une chaîne. |
| `network_mode`                     |                                                  | Ajouter le conteneur à un réseau personnalisé. |
| `mac_address`                      | `92:d0:c6:0a:29:33`                              | Adresse MAC du conteneur |
| `oom_kill_disable`                 |                                                  | Si une erreur de dépassement de mémoire (`OOM`) se produit, ne pas terminer les processus dans un conteneur. |
| `oom_score_adjust`                 |                                                  | Ajustement du score `OOM`. Une valeur positive signifie terminer les processus plus tôt. |
| `privileged`                       | `false`                                          | Faire fonctionner le conteneur en mode privilégié. Non sécurisé. |
| `services_privileged`              |                                                  | Autoriser les services à s'exécuter en mode privilégié. Si non défini (par défaut), la valeur de `privileged` est utilisée à la place. À utiliser avec l'exécuteur [Docker](../executors/docker.md#allow-docker-pull-policies). Non sécurisé. |
| `pull_policy`                      |                                                  | La politique de tirage d'images : `never`, `if-not-present` ou `always` (par défaut). Voir les détails dans la [documentation sur les politiques de tirage](../executors/docker.md#configure-how-runners-pull-images). Vous pouvez également ajouter [plusieurs politiques de tirage](../executors/docker.md#set-multiple-pull-policies) , [réessayer un tirage échoué](../executors/docker.md#retry-a-failed-pull) ou [restreindre les politiques de tirage](../executors/docker.md#allow-docker-pull-policies). |
| `runtime`                          |                                                  | Le runtime pour le conteneur Docker. |
| `isolation`                        |                                                  | Technologie d'isolation du conteneur (`default`, `hyperv` et `process`). Windows uniquement. |
| `security_opt`                     |                                                  | Options de sécurité (--security-opt dans `docker run`). Prend une liste de paires clé/valeur séparées par `:`. La spécification `systempaths` n'est pas prise en charge. Pour plus d'informations, voir [ticket 36810](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/36810). |
| `shm_size`                         | `300000`                                         | Taille de la mémoire partagée pour les images (en octets). |
| `sysctls`                          |                                                  | Les options `sysctl`. |
| `tls_cert_path`                    | Sur macOS `/Users/<username>/.boot2docker/certs`. | Un répertoire où `ca.pem`, `cert.pem` ou `key.pem` sont stockés et utilisés pour établir une connexion TLS sécurisée à Docker. Utilisez ce paramètre avec `boot2docker`. |
| `tls_verify`                       |                                                  | Activer ou désactiver la vérification TLS des connexions au démon Docker. Désactivé par défaut. Par défaut, GitLab Runner se connecte au socket Unix Docker via SSH. Le socket Unix ne prend pas en charge RTLS et communique via HTTP avec SSH pour assurer le chiffrement et l'authentification. L'activation de `tls_verify` n'est généralement pas nécessaire et requiert une configuration supplémentaire. Pour activer `tls_verify`, le démon doit écouter sur un port (plutôt que sur le socket Unix par défaut) et l'hôte Docker de GitLab Runner doit utiliser l'adresse sur laquelle le démon écoute. |
| `user`                             |                                                  | Exécuter toutes les commandes dans le conteneur en tant qu'utilisateur spécifié. |
| `userns_mode`                      |                                                  | Le mode d'espace de nommage utilisateur pour le conteneur et les services Docker lorsque l'option de remappage d'espace de nommage utilisateur est activée. Disponible dans Docker 1.10 ou ultérieur. Pour les détails, voir la [documentation Docker](https://docs.docker.com/engine/security/userns-remap/#disable-namespace-remapping-for-a-container). |
| `ulimit`                           |                                                  | Valeurs ulimit transmises au conteneur. Utilise la même syntaxe que l'indicateur Docker `--ulimit`. |
| `volume_keep`                      |                                                  | Lorsque `true`, les volumes Docker ne sont pas supprimés lorsque le runner nettoie un conteneur après un job. Les volumes s'accumulent sur le disque. L'opérateur est responsable du nettoyage périodique (par exemple, `docker volume prune` dans un job cron). Utilisez ce paramètre dans les environnements à forte concurrence où la suppression de volumes bloque le démon Docker. La valeur par défaut est `false`. |
| `volumes`                          | `["/data", "/home/project/cache"]`               | Volumes supplémentaires à monter. Même syntaxe que l'indicateur Docker `-v`. |
| `volumes_from`                     | `["storage_container:ro"]`                       | Liste des volumes à hériter d'un autre conteneur sous la forme `<container name>[:<access_level>]`. Le niveau d'accès est par défaut en lecture-écriture, mais peut être défini manuellement sur `ro` (lecture seule) ou `rw` (lecture-écriture). |
| `volume_driver`                    |                                                  | Le pilote de volume à utiliser pour le conteneur. |
| `wait_for_services_timeout`        | `30`                                             | Durée d'attente pour les services Docker. Définir sur `-1` pour désactiver. La valeur par défaut est `30`. |
| `container_labels`                 |                                                  | Un ensemble de labels à ajouter à chaque conteneur créé par le runner. La valeur du label peut inclure des variables d'environnement pour l'expansion. |
| `services_limit`                   |                                                  | Définir le nombre maximum de services autorisés par job. `-1` (par défaut) signifie qu'il n'y a pas de limite. |
| `service_cpuset_cpus`              |                                                  | Valeur de chaîne contenant le `cgroups CpusetCpus` à utiliser pour un service. |
| `service_cpu_shares`               |                                                  | Nombre de partages CPU utilisés pour définir l'utilisation CPU relative d'un service (par défaut : [`1024`](https://docs.docker.com/engine/containers/resource_constraints/#cpu)). |
| `service_cpus`                     |                                                  | Valeur de chaîne du nombre de CPU pour un service. Disponible dans Docker 1.13 ou ultérieur. |
| `service_gpus`                     |                                                  | Périphériques GPU pour le conteneur Docker. Utilise le même format que la CLI `docker`. Voir les détails dans la [documentation Docker](https://docs.docker.com/engine/containers/resource_constraints/#gpu). Nécessite une [configuration pour activer les GPU](gpus.md#docker-executor). |
| `service_memory`                   |                                                  | Valeur de chaîne de la limite de mémoire pour un service. |
| `service_memory_swap`              |                                                  | Valeur de chaîne de la limite totale de mémoire pour un service. |
| `service_memory_reservation`       |                                                  | Valeur de chaîne de la limite de mémoire souple pour un service. |

### La section `[[runners.docker.services]]` {#the-runnersdockerservices-section}

Spécifier des [services](https://docs.gitlab.com/ci/services/) supplémentaires à exécuter avec le job. Pour une liste des images disponibles, voir le [Docker Registry](https://hub.docker.com). Chaque service s'exécute dans un conteneur séparé et est lié au job.

| Paramètre     | Exemple                            | Description |
|---------------|------------------------------------|-------------|
| `name`        | `"registry.example.com/svc1"`      | Le nom de l'image à exécuter en tant que service. |
| `alias`       | `"svc1"`                           | [Nom d'alias](https://docs.gitlab.com/ci/services/#available-settings-for-services) supplémentaire pouvant être utilisé pour accéder au service. |
| `entrypoint`  | `["entrypoint.sh"]`                | Commande ou script à exécuter comme point d'entrée du conteneur. La syntaxe est similaire à la directive [Dockerfile ENTRYPOINT](https://docs.docker.com/reference/dockerfile/#entrypoint), où chaque jeton shell est une chaîne séparée dans le tableau. Introduit dans [GitLab Runner 13.6](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27173). |
| `command`     | `["executable","param1","param2"]` | Commande ou script à utiliser comme commande du conteneur. La syntaxe est similaire à la directive [Dockerfile CMD](https://docs.docker.com/reference/dockerfile/#cmd), où chaque jeton shell est une chaîne séparée dans le tableau. Introduit dans [GitLab Runner 13.6](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27173). |
| `environment` | `["ENV1=value1", "ENV2=value2"]`   | Ajouter ou remplacer des variables d'environnement pour le conteneur de service. |

Exemple :

```toml
[runners.docker]
  host = ""
  hostname = ""
  tls_cert_path = "/Users/ayufan/.boot2docker/certs"
  image = "ruby:3.3"
  memory = "128m"
  memory_swap = "256m"
  memory_reservation = "64m"
  oom_kill_disable = false
  cpuset_cpus = "0,1"
  cpuset_mems = "0,1"
  cpus = "2"
  dns = ["8.8.8.8"]
  dns_search = [""]
  service_memory = "128m"
  service_memory_swap = "256m"
  service_memory_reservation = "64m"
  service_cpuset_cpus = "0,1"
  service_cpus = "2"
  services_limit = 5
  privileged = false
  group_add = ["docker"]
  cap_add = ["NET_ADMIN"]
  cap_drop = ["DAC_OVERRIDE"]
  devices = ["/dev/net/tun"]
  disable_cache = false
  wait_for_services_timeout = 30
  cache_dir = ""
  volumes = ["/data", "/home/project/cache"]
  extra_hosts = ["other-host:127.0.0.1"]
  shm_size = 300000
  volumes_from = ["storage_container:ro"]
  links = ["mysql_container:mysql"]
  allowed_images = ["ruby:*", "python:*", "php:*"]
  allowed_services = ["postgres:9", "redis:*", "mysql:*"]
  log_options = { env = "GITLAB_CI_JOB_ID,GITLAB_CI_JOB_NAME", labels = "com.gitlab.gitlab-runner.type" }
  [runners.docker.ulimit]
    "rtprio" = "99"
  [[runners.docker.services]]
    name = "registry.example.com/svc1"
    alias = "svc1"
    entrypoint = ["entrypoint.sh"]
    command = ["executable","param1","param2"]
    environment = ["ENV1=value1", "ENV2=value2"]
  [[runners.docker.services]]
    name = "redis:2.8"
    alias = "cache"
  [[runners.docker.services]]
    name = "postgres:9"
    alias = "postgres-db"
  [runners.docker.sysctls]
    "net.ipv4.ip_forward" = "1"
```

### Volumes dans la section `[runners.docker]` {#volumes-in-the-runnersdocker-section}

Pour plus d'informations sur les volumes, voir la [documentation Docker](https://docs.docker.com/engine/storage/volumes/).

Les exemples suivants montrent comment spécifier des volumes dans la section `[runners.docker]`.

#### Exemple 1 : Ajouter un volume de données {#example-1-add-a-data-volume}

Un volume de données est un répertoire spécialement désigné dans un ou plusieurs conteneurs qui contourne le système de fichiers Union. Les volumes de données sont conçus pour persister les données, indépendamment du cycle de vie du conteneur.

```toml
[runners.docker]
  host = ""
  hostname = ""
  tls_cert_path = "/Users/ayufan/.boot2docker/certs"
  image = "ruby:3.3"
  privileged = false
  disable_cache = true
  volumes = ["/path/to/volume/in/container"]
```

Cet exemple crée un nouveau volume dans le conteneur à l'emplacement `/path/to/volume/in/container`.

#### Exemple 2 : Monter un répertoire hôte comme volume de données {#example-2-mount-a-host-directory-as-a-data-volume}

Lorsque vous souhaitez stocker des répertoires en dehors du conteneur, vous pouvez monter un répertoire depuis l'hôte de votre démon Docker dans un conteneur :

```toml
[runners.docker]
  host = ""
  hostname = ""
  tls_cert_path = "/Users/ayufan/.boot2docker/certs"
  image = "ruby:3.3"
  privileged = false
  disable_cache = true
  volumes = ["/path/to/bind/from/host:/path/to/bind/in/container:rw"]
```

Cet exemple utilise `/path/to/bind/from/host` de l'hôte CI/CD dans le conteneur à l'emplacement `/path/to/bind/in/container`.

GitLab Runner 11.11 et versions ultérieures [montent le répertoire hôte](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1261) pour les [services](https://docs.gitlab.com/ci/services/) définis également.

### Options de journalisation Docker {#docker-log-options}

Le paramètre `log_options` vous permet de configurer les options de journalisation des conteneurs Docker pour le pilote de journalisation `json-file`. Pour des raisons de sécurité et de compatibilité, seules les options `env` et `labels` sont prises en charge.

#### Options de journalisation prises en charge {#supported-log-options}

- `env` : Liste séparée par des virgules de noms de variables d'environnement à inclure dans les entrées de journal
- `labels` : Liste séparée par des virgules de noms de labels de conteneur à inclure dans les entrées de journal

#### Exemples de configuration {#configuration-examples}

Voici quelques exemples de configuration :

```toml
[[runners]]
  [runners.docker]
    # Include specific environment variables in logs
    log_options = { env = "GITLAB_CI_JOB_ID,GITLAB_CI_JOB_NAME,CI_PIPELINE_ID" }
```

```toml
[[runners]]
  [runners.docker]
    # Include container labels in logs
    log_options = { labels = "com.gitlab.gitlab-runner.type" }
```

```toml
[[runners]]
  [runners.docker]
    # Include both environment variables and labels
    log_options = { env = "GITLAB_CI_JOB_ID,GITLAB_CI_JOB_NAME", labels = "com.gitlab.gitlab-runner.type" }
```

#### Validation et gestion des erreurs {#validation-and-error-handling}

GitLab Runner valide les options de journalisation lors de la préparation de l'exécuteur. Si vous spécifiez des options non prises en charge telles que `max-size`, `max-file` ou `compress`, le job échoue immédiatement avec une erreur de configuration.

Les options de journalisation s'appliquent au conteneur de job principal et à tous les conteneurs de service définis dans votre configuration CI/CD.

Pour plus d'informations sur la journalisation Docker, consultez la [documentation du pilote de journalisation `json-file` Docker](https://docs.docker.com/config/containers/logging/json-file/).

### Utiliser un registre de conteneurs privé {#use-a-private-container-registry}

Pour utiliser des registres privés comme source d'images pour vos jobs, configurez l'autorisation avec la [variable CI/CD](https://docs.gitlab.com/ci/variables/) `DOCKER_AUTH_CONFIG`. Vous pouvez définir la variable dans l'un des éléments suivants :

- Les paramètres CI/CD du projet en tant que [type `file`](https://docs.gitlab.com/ci/variables/#use-file-type-cicd-variables)
- Le fichier `config.toml`

L'utilisation de registres privés avec la politique de tirage `if-not-present` peut introduire des [implications en matière de sécurité](../security/_index.md#usage-of-private-docker-images-with-if-not-present-pull-policy). Pour plus d'informations sur le fonctionnement des politiques de tirage, consultez [Configurer la façon dont les runners tirent les images](../executors/docker.md#configure-how-runners-pull-images).

Pour plus d'informations sur l'utilisation de registres de conteneurs privés, consultez :

- [Accéder à une image depuis un registre de conteneurs privé](https://docs.gitlab.com/ci/docker/using_docker_images/#access-an-image-from-a-private-container-registry)
- [Référence des mots-clés `.gitlab-ci.yml`](https://docs.gitlab.com/ci/yaml/#image)

Les étapes effectuées par le runner peuvent se résumer comme suit :

1. Le nom du registre est trouvé à partir du nom de l'image.
1. Si la valeur n'est pas vide, l'exécuteur recherche la configuration d'authentification pour ce registre.
1. Enfin, si une authentification correspondant au registre spécifié est trouvée, les tirages suivants l'utilisent.

#### Prise en charge du registre intégré GitLab {#support-for-gitlab-integrated-registry}

GitLab envoie les informations d'identification de son registre intégré avec les données du job. Ces informations d'identification sont automatiquement ajoutées à la liste des paramètres d'autorisation du registre.

Après cette étape, l'autorisation auprès du registre se déroule de manière similaire à la configuration ajoutée avec la variable `DOCKER_AUTH_CONFIG`.

Dans vos jobs, vous pouvez utiliser n'importe quelle image de votre registre intégré GitLab, même si l'image est privée ou protégée. Pour obtenir des informations sur les images auxquelles les jobs ont accès, consultez la [documentation sur les jetons de job CI/CD](https://docs.gitlab.com/ci/jobs/ci_job_token/).

#### Priorité de résolution de l'autorisation Docker {#precedence-of-docker-authorization-resolving}

Comme décrit précédemment, GitLab Runner peut autoriser Docker auprès d'un registre en utilisant des informations d'identification envoyées de différentes manières. Pour trouver le registre approprié, l'ordre de priorité suivant est pris en compte :

1. Informations d'identification configurées avec `DOCKER_AUTH_CONFIG`.
1. Informations d'identification configurées localement sur l'hôte GitLab Runner avec les fichiers `~/.docker/config.json` ou `~/.dockercfg` (par exemple, en exécutant `docker login` sur l'hôte).
1. Informations d'identification envoyées par défaut avec la charge utile d'un job (par exemple, les informations d'identification du registre intégré décrit précédemment).

Les premières informations d'identification trouvées pour le registre sont utilisées. Ainsi, par exemple, si vous ajoutez des informations d'identification pour le registre intégré avec la variable `DOCKER_AUTH_CONFIG`, les informations d'identification par défaut sont remplacées.

## La section `[runners.parallels]` {#the-runnersparallels-section}

Les paramètres suivants concernent Parallels.

| Paramètre           | Description |
|---------------------|-------------|
| `base_name`         | Nom de la VM Parallels qui est clonée. |
| `template_name`     | Nom personnalisé du modèle lié à la VM Parallels. Facultatif. |
| `disable_snapshots` | Si désactivé, les VMs sont détruites lorsque les jobs sont terminés. |
| `allowed_images`    | Liste des valeurs `image`/`base_name` autorisées, représentées sous forme d'expressions régulières. Consultez la section [Remplacement de l'image VM de base](#overriding-the-base-vm-image) pour plus de détails. |

Exemple :

```toml
[runners.parallels]
  base_name = "my-parallels-image"
  template_name = ""
  disable_snapshots = false
```

## La section `[runners.virtualbox]` {#the-runnersvirtualbox-section}

Les paramètres suivants concernent VirtualBox. Cet exécuteur repose sur l'exécutable `vboxmanage` pour contrôler les machines VirtualBox, vous devez donc ajuster votre variable d'environnement `PATH` sur les hôtes Windows : `PATH=%PATH%;C:\Program Files\Oracle\VirtualBox`.

| Paramètre           | Explication |
|---------------------|-------------|
| `base_name`         | Nom de la VM VirtualBox qui est clonée. |
| `base_snapshot`     | Nom ou UUID d'un instantané spécifique de la VM à partir duquel créer un clone lié. Si cette valeur est vide ou omise, l'instantané actuel est utilisé. S'il n'existe pas d'instantané actuel, un est créé. À moins que `disable_snapshots` ne soit défini sur true, auquel cas un clone complet de la VM de base est effectué. |
| `base_folder`       | Dossier dans lequel enregistrer la nouvelle VM. Si cette valeur est vide ou omise, le dossier VM par défaut est utilisé. |
| `disable_snapshots` | Si désactivé, les VMs sont détruites lorsque les jobs sont terminés. |
| `allowed_images`    | Liste des valeurs `image`/`base_name` autorisées, représentées sous forme d'expressions régulières. Consultez la section [Remplacement de l'image VM de base](#overriding-the-base-vm-image) pour plus de détails. |
| `start_type`        | Type d'interface graphique lors du démarrage de la VM. |

Exemple :

```toml
[runners.virtualbox]
  base_name = "my-virtualbox-image"
  base_snapshot = "my-image-snapshot"
  disable_snapshots = false
  start_type = "headless"
```

Le paramètre `start_type` détermine l'interface graphique utilisée lors du démarrage de l'image virtuelle. Les valeurs valides sont `headless` (par défaut), `gui` ou `separate` selon les combinaisons hôte/invité prises en charge.

## Remplacement de l'image VM de base {#overriding-the-base-vm-image}

Pour les exécuteurs Parallels et VirtualBox, vous pouvez remplacer le nom de la VM de base spécifié par `base_name`. Pour ce faire, utilisez le paramètre [image](https://docs.gitlab.com/ci/yaml/#image) dans le fichier `.gitlab-ci.yml`.

Pour des raisons de compatibilité ascendante, vous ne pouvez pas remplacer cette valeur par défaut. Seule l'image spécifiée par `base_name` est autorisée.

Pour permettre aux utilisateurs de sélectionner une image VM à l'aide du paramètre [image](https://docs.gitlab.com/ci/yaml/#image) de `.gitlab-ci.yml` :

```toml
[runners.virtualbox]
  ...
  allowed_images = [".*"]
```

Dans l'exemple, toute image VM existante peut être utilisée.

Le paramètre `allowed_images` est une liste d'expressions régulières. La configuration peut être aussi précise que nécessaire. Par exemple, si vous souhaitez n'autoriser que certaines images VM, vous pouvez utiliser une expression régulière telle que :

```toml
[runners.virtualbox]
  ...
  allowed_images = ["^allowed_vm[1-2]$"]
```

Dans cet exemple, seuls `allowed_vm1` et `allowed_vm2` sont autorisés. Toute autre tentative entraîne une erreur.

## La section `[runners.ssh]` {#the-runnersssh-section}

Les paramètres suivants définissent la connexion SSH.

| Paramètre                          | Description |
|------------------------------------|-------------|
| `host`                             | Où se connecter. |
| `port`                             | Port. La valeur par défaut est `22`. |
| `user`                             | Nom d'utilisateur.   |
| `password`                         | Mot de passe.   |
| `identity_file`                    | Chemin de fichier vers la clé privée SSH (`id_rsa`, `id_dsa` ou `id_edcsa`). Le fichier doit être stocké sans chiffrement. |
| `disable_strict_host_key_checking` | Cette valeur détermine si le runner doit utiliser la vérification stricte de la clé d'hôte. La valeur par défaut est `true`. Dans GitLab 15.0, la valeur par défaut, ou la valeur si elle n'est pas spécifiée, est `false`. |

Exemple :

```toml
[runners.ssh]
  host = "my-production-server"
  port = "22"
  user = "root"
  password = "production-server-password"
  identity_file = ""
```

## La section `[runners.machine]` {#the-runnersmachine-section}

Les paramètres suivants définissent la fonctionnalité de mise à l'échelle automatique basée sur Docker Machine. Pour plus d'informations, consultez [la configuration de la mise à l'échelle automatique de l'exécuteur Docker Machine](autoscale.md).

| Paramètre                         | Description |
|-----------------------------------|-------------|
| `MaxGrowthRate`                   | Le nombre maximum de machines pouvant être ajoutées au runner en parallèle. La valeur par défaut est `0` (aucune limite). |
| `IdleCount`                       | Nombre de machines devant être créées et en attente à l'état _Idle_. |
| `IdleScaleFactor`                 | Le nombre de machines _Idle_ en tant que facteur du nombre de machines en cours d'utilisation. Doit être au format nombre flottant. Consultez [la documentation sur la mise à l'échelle automatique](autoscale.md#the-idlescalefactor-strategy) pour plus de détails. Par défaut `0.0`. |
| `IdleCountMin`                    | Nombre minimal de machines devant être créées et en attente à l'état _Idle_ lorsque `IdleScaleFactor` est en cours d'utilisation. La valeur par défaut est 1. |
| `IdleTime`                        | Durée (en secondes) pendant laquelle une machine doit être à l'état _Idle_ avant d'être supprimée. |
| `[[runners.machine.autoscaling]]` | Plusieurs sections, chacune contenant des remplacements pour la configuration de mise à l'échelle automatique. La dernière section avec une expression qui correspond à l'heure actuelle est sélectionnée. |
| `OffPeakPeriods`                  | Obsolète : Périodes au cours desquelles le planificateur est en mode OffPeak. Un tableau de modèles de style cron (décrits [ci-dessous](#periods-syntax)). |
| `OffPeakTimezone`                 | Obsolète : Fuseau horaire pour les heures données dans OffPeakPeriods. Une chaîne de fuseau horaire telle que `Europe/Berlin`. Par défaut, le paramètre régional du système hôte si omis ou vide. GitLab Runner tente de localiser la base de données des fuseaux horaires dans le répertoire ou le fichier zip non compressé nommé par la variable d'environnement `ZONEINFO`, puis recherche dans les emplacements d'installation connus sur les systèmes Unix, et enfin dans `$GOROOT/lib/time/zoneinfo.zip`. |
| `OffPeakIdleCount`                | Obsolète : Identique à `IdleCount`, mais pour les périodes _Off Peak_. |
| `OffPeakIdleTime`                 | Obsolète : Identique à `IdleTime`, mais pour les périodes _Off Peak_. |
| `MaxBuilds`                       | Nombre maximum de jobs (builds) avant que la machine ne soit supprimée. |
| `MachineName`                     | Nom de la machine. Elle **doit** contenir `%s`, qui est remplacé par un identifiant de machine unique. |
| `MachineDriver`                   | `driver` de Docker Machine. Consultez les détails dans la [section Fournisseurs cloud dans la configuration de Docker Machine](autoscale.md#supported-cloud-providers). |
| `MachineOptions`                  | Options Docker Machine pour le MachineDriver. Pour plus d'informations, consultez [Fournisseurs cloud pris en charge](autoscale.md#supported-cloud-providers). Pour plus d'informations sur toutes les options AWS, consultez les projets [AWS](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md) et [GCP](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/gce.md) dans le dépôt Docker Machine. |

### Les sections `[[runners.machine.autoscaling]]` {#the-runnersmachineautoscaling-sections}

Les paramètres suivants définissent la configuration disponible lors de l'utilisation des exécuteurs [Instance](../executors/instance.md) ou [Docker Autoscaler](../executors/docker_autoscaler.md#example-aws-autoscaling-for-1-job-per-instance).

| Paramètre         | Description |
|-------------------|-------------|
| `Periods`         | Périodes au cours desquelles cette planification est active. Un tableau de modèles de style cron (décrits [ci-dessous](#periods-syntax)). |
| `IdleCount`       | Nombre de machines devant être créées et en attente à l'état _Idle_. |
| `IdleScaleFactor` | (Expérimental) Le nombre de machines _Idle_ en tant que facteur du nombre de machines en cours d'utilisation. Doit être au format nombre flottant. Consultez [la documentation sur la mise à l'échelle automatique](autoscale.md#the-idlescalefactor-strategy) pour plus de détails. Par défaut `0.0`. |
| `IdleCountMin`    | Nombre minimal de machines devant être créées et en attente à l'état _Idle_ lorsque `IdleScaleFactor` est en cours d'utilisation. La valeur par défaut est 1. |
| `IdleTime`        | Durée (en secondes) pendant laquelle une machine doit être à l'état _Idle_ avant d'être supprimée. |
| `Timezone`        | Fuseau horaire pour les heures données dans `Periods`. Une chaîne de fuseau horaire telle que `Europe/Berlin`. Par défaut, le paramètre régional du système hôte si omis ou vide. GitLab Runner tente de localiser la base de données des fuseaux horaires dans le répertoire ou le fichier zip non compressé nommé par la variable d'environnement `ZONEINFO`, puis recherche dans les emplacements d'installation connus sur les systèmes Unix, et enfin dans `$GOROOT/lib/time/zoneinfo.zip`. |

Exemple :

```toml
[runners.machine]
  IdleCount = 5
  IdleTime = 600
  MaxBuilds = 100
  MachineName = "auto-scale-%s"
  MachineDriver = "google" # Refer to Docker Machine docs on how to authenticate: https://docs.docker.com/machine/drivers/gce/#credentials
  MachineOptions = [
      # Additional machine options can be added using the Google Compute Engine driver.
      # If you experience problems with an unreachable host (ex. "Waiting for SSH"),
      # you should remove optional parameters to help with debugging.
      # https://docs.docker.com/machine/drivers/gce/
      "google-project=GOOGLE-PROJECT-ID",
      "google-zone=GOOGLE-ZONE", # e.g. 'us-central1-a', full list in https://cloud.google.com/compute/docs/regions-zones/
  ]
  [[runners.machine.autoscaling]]
    Periods = ["* * 9-17 * * mon-fri *"]
    IdleCount = 50
    IdleCountMin = 5
    IdleScaleFactor = 1.5 # Means that current number of Idle machines will be 1.5*in-use machines,
                          # no more than 50 (the value of IdleCount) and no less than 5 (the value of IdleCountMin)
    IdleTime = 3600
    Timezone = "UTC"
  [[runners.machine.autoscaling]]
    Periods = ["* * * * * sat,sun *"]
    IdleCount = 5
    IdleTime = 60
    Timezone = "UTC"
```

### Syntaxe des périodes {#periods-syntax}

Le paramètre `Periods` contient un tableau de modèles de chaînes de périodes de temps représentées au format cron. La ligne contient les champs suivants :

```plaintext
[second] [minute] [hour] [day of month] [month] [day of week] [year]
```

Comme dans le fichier de configuration cron standard, les champs peuvent contenir des valeurs uniques, des plages, des listes et des astérisques. Consultez [une description détaillée de la syntaxe](https://github.com/gorhill/cronexpr#implementation).

## La section `[runners.instance]` {#the-runnersinstance-section}

| Paramètre        | Type   | Description |
|------------------|--------|-------------|
| `allowed_images` | string | Lorsque l'isolation VM est activée, `allowed_images` contrôle les images qu'un job est autorisé à spécifier. |

## La section `[runners.autoscaler]` {#the-runnersautoscaler-section}

{{< history >}}

- Introduit dans GitLab Runner v15.10.0.

{{< /history >}}

Les paramètres suivants configurent la fonctionnalité de mise à l'échelle automatique. Vous ne pouvez utiliser ces paramètres qu'avec les exécuteurs [Instance](../executors/instance.md) et [Docker Autoscaler](../executors/docker_autoscaler.md).

| Paramètre                        | Description |
|----------------------------------|-------------|
| `capacity_per_instance`          | Le nombre de jobs pouvant être exécutés simultanément par une seule instance. |
| `max_use_count`                  | Le nombre maximum de fois qu'une instance peut être utilisée avant d'être planifiée pour suppression. |
| `max_instances`                  | Le nombre maximum d'instances autorisées, quel que soit l'état de l'instance (en attente, en cours d'exécution, en cours de suppression). Par défaut : `0` (illimité). |
| `plugin`                         | Le plugin [fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting) à utiliser. Pour plus d'informations sur la façon d'installer et de référencer un plugin, consultez [Installer le plugin fleeting](../fleet_scaling/fleeting.md#install-a-fleeting-plugin). |
| `delete_instances_on_shutdown`   | Spécifie si toutes les instances provisionnées sont supprimées lorsque GitLab Runner s'arrête. Par défaut : `false`. Introduit dans [GitLab Runner 15.11](https://gitlab.com/gitlab-org/fleeting/taskscaler/-/merge_requests/24) |
| `instance_ready_command`         | Exécute cette commande sur chaque instance provisionnée par l'autoscaler pour s'assurer qu'elle est prête à être utilisée. Un échec entraîne la suppression de l'instance. Introduit dans [GitLab Runner 16.11](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37473). |
| `instance_acquire_timeout`       | La durée maximale pendant laquelle le runner attend pour acquérir une instance avant d'expirer. Par défaut : `15m` (15 minutes). Vous pouvez ajuster cette valeur pour mieux correspondre à votre environnement. Introduit dans [GitLab Runner 18.1](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5563). |
| `update_interval`                | L'intervalle de vérification auprès du plugin fleeting pour les mises à jour d'instances. Par défaut : `1m` (1 minute). Introduit dans [GitLab Runner 16.11](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4722). |
| `update_interval_when_expecting` | L'intervalle de vérification auprès du plugin fleeting pour les mises à jour d'instances lors de l'attente d'un changement d'état. Par exemple, lorsqu'une instance a provisionné une instance et que le runner attend la transition de `pending` à `running`. Par défaut : `2s` (2 secondes). Introduit dans [GitLab Runner 16.11](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4722). |
| `deletion_retry_interval` | L'intervalle pendant lequel le plugin fleeting attend avant de réessayer la suppression lorsqu'une tentative de suppression précédente n'a eu aucun effet. Par défaut : `1m` (1 minute). Introduit dans [GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777). |
| `shutdown_deletion_interval`| L'intervalle utilisé par le plugin fleeting entre la suppression des instances et la vérification de leur statut lors de l'arrêt. Par défaut : `10s` (10 secondes). Introduit dans [GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777). |
| `shutdown_deletion_retries` | Le nombre maximum de tentatives effectuées par le plugin fleeting pour s'assurer que les instances terminent leur suppression avant l'arrêt. Par défaut : `3`. Introduit dans [GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777). |
| `failure_threshold` | Le nombre maximum d'échecs de vérification de santé consécutifs avant que le plugin fleeting ne remplace une instance. Voir également la fonctionnalité heartbeat. Par défaut : `3`. Introduit dans [GitLab Runner 18.4](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5777). |
| `log_internal_ip`                | Spécifie si la sortie CI/CD consigne l'adresse IP interne de la VM. Par défaut : `false`. Introduit dans [GitLab Runner 18.1](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5519). |
| `log_external_ip`                | Spécifie si la sortie CI/CD consigne l'adresse IP externe de la VM. Par défaut : `false`. Introduit dans [GitLab Runner 18.1](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5519). |

Si `instance_ready_command` échoue fréquemment avec les règles de mise à l'échelle inactives, les instances peuvent être supprimées et créées plus rapidement que le runner n'accepte les jobs. Pour prendre en charge la limitation de la mise à l'échelle, un backoff exponentiel a été ajouté dans [GitLab 17.0](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37497).

> [!note]
> Les options de configuration de l'autoscaler ne se rechargent pas lors des modifications de configuration. Cependant, dans GitLab 17.5.0 ou version ultérieure, les entrées `[[runners.autoscaler.policy]]` se rechargent lors des modifications de configuration.

## La section `[runners.autoscaler.plugin_config]` {#the-runnersautoscalerplugin_config-section}

Cette table de hachage est ré-encodée en JSON et transmise directement au plugin configuré.

Les plugins [fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting) disposent généralement d'une documentation accompagnatrice sur la configuration prise en charge.

## La section `[runners.autoscaler.scale_throttle]` {#the-runnersautoscalerscale_throttle-section}

{{< history >}}

- Introduit dans GitLab Runner v17.0.0.

{{< /history >}}

| Paramètre | Description |
|-----------|-------------|
| `limit`   | La limite de débit de nouvelles instances par seconde pouvant être provisionnées. `-1` signifie illimité. La valeur par défaut (`0`), définit la limite à `100`. |
| `burst`   | La limite de burst de nouvelles instances. Par défaut à `max_instances` ou `limit` lorsque `max_instances` n'est pas défini. Si `limit` est infini, `burst` est ignoré. |

### Relation entre `limit` et `burst` {#relationship-between-limit-and-burst}

Le limiteur de mise à l'échelle utilise un système de quota de jetons pour créer des instances. Ce système est défini par deux valeurs :

- `burst` : La taille maximale du quota.
- `limit` : Le taux auquel le quota se rafraîchit par seconde.

Le nombre d'instances que vous pouvez créer à la fois dépend de votre quota restant. Si vous disposez d'un quota suffisant, vous pouvez créer des instances jusqu'à ce montant. Si le quota est épuisé, vous pouvez créer `limit` instances par seconde. Lorsque la création d'instances s'arrête, le quota augmente de `limit` par seconde jusqu'à atteindre la valeur `burst`.

Par exemple, si `limit` est `1` et `burst` est `60` :

- Vous pouvez créer 60 instances instantanément, mais vous êtes limité.
- Si vous attendez 60 secondes, vous pouvez instantanément créer 60 autres instances.
- Si vous n'attendez pas, vous pouvez créer 1 instance par seconde.

## La section `[runners.autoscaler.connector_config]` {#the-runnersautoscalerconnector_config-section}

Les plugins [fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting) disposent généralement d'une documentation accompagnatrice sur les options de connexion prises en charge.

Les plugins mettent automatiquement à jour la configuration du connecteur. Vous pouvez utiliser `[runners.autoscaler.connector_config]` pour remplacer la mise à jour automatique de la configuration du connecteur, ou pour remplir les valeurs vides que le plugin ne peut pas déterminer.

| Paramètre                | Description |
|--------------------------|-------------|
| `os`                     | Le système d'exploitation de l'instance. |
| `arch`                   | L'architecture de l'instance. |
| `protocol`               | `ssh`, `winrm` ou `winrm+https`. `winrm` est utilisé par défaut si Windows est détecté. |
| `protocol_port`          | Le port utilisé pour établir la connexion en fonction du protocole spécifié. Par défaut à `ssh:22`, `winrm+http:5985`, `winrm+https:5986`. |
| `username`               | Le nom d'utilisateur utilisé pour se connecter. |
| `password`               | Le mot de passe utilisé pour se connecter. |
| `key_path`               | La clé TLS utilisée pour se connecter ou pour provisionner dynamiquement des informations d'identification. |
| `use_static_credentials` | Désactive le provisionnement automatique des informations d'identification. Par défaut : `false`. |
| `keepalive`              | La durée de maintien de la connexion (keepalive). |
| `timeout`                | La durée d'expiration de la connexion. |
| `use_external_addr`      | Indique si l'adresse externe fournie par le plugin doit être utilisée. Si le plugin ne renvoie qu'une adresse interne, elle est utilisée quel que soit ce paramètre. Par défaut : `false`. |

## La section `[runners.autoscaler.state_storage]` {#the-runnersautoscalerstate_storage-section}

{{< details >}}

- Statut : Bêta

{{< /details >}}

{{< history >}}

- Introduit dans GitLab Runner 17.5.0.

{{< /history >}}

Si GitLab Runner démarre alors que le stockage d'état est désactivé (par défaut), les instances fleeting existantes sont supprimées immédiatement pour des raisons de sécurité. Par exemple, lorsque `max_use_count` est défini sur `1`, nous pourrions par inadvertance assigner un job à une instance déjà utilisée si nous ne connaissons pas son statut d'utilisation.

L'activation de la fonctionnalité de stockage d'état permet à l'état d'une instance de persister sur le disque local. Dans ce cas, si une instance existe au démarrage de GitLab Runner, elle n'est pas supprimée. Ses détails de connexion mis en cache, son nombre d'utilisations et autres configurations sont restaurés.

Tenez compte des informations suivantes lors de l'activation de la fonctionnalité de stockage d'état :

- Les détails d'authentification d'une instance (nom d'utilisateur, mot de passe, clés) restent sur le disque.
- Si une instance est restaurée alors qu'elle exécute activement un job, GitLab Runner la supprime par défaut. Ce comportement garantit la sécurité, car GitLab Runner ne peut pas reprendre les jobs. Pour conserver l'instance, définissez `keep_instance_with_acquisitions` sur `true`.

  Définir `keep_instance_with_acquisitions` sur `true` est utile lorsque vous n'êtes pas préoccupé par les jobs en cours sur l'instance. Vous pouvez également utiliser l'option de configuration `instance_ready_command` pour nettoyer l'environnement afin de conserver l'instance. Cela peut impliquer l'arrêt de toutes les commandes en cours d'exécution ou la suppression forcée des conteneurs Docker.

| Paramètre                         | Description |
|-----------------------------------|-------------|
| `enabled`                         | Indique si le stockage d'état est activé. Par défaut : `false`. |
| `dir`                             | Le répertoire du magasin d'états. Chaque entrée de configuration de runner dispose d'un sous-répertoire ici. Par défaut : `.taskscaler` dans le répertoire du fichier de configuration GitLab Runner. |
| `keep_instance_with_acquisitions` | Indique si les instances avec des jobs actifs sont supprimées. Par défaut : `false`. |

## Les sections `[[runners.autoscaler.policy]]` {#the-runnersautoscalerpolicy-sections}

**Note** - `idle_count` dans ce contexte fait référence au nombre de jobs, et non au nombre de machines à mise à l'échelle automatique comme dans la méthode de mise à l'échelle automatique héritée.

| Paramètre            | Description |
|----------------------|-------------|
| `periods`            | Un tableau de chaînes au format unix-cron pour indiquer la période pendant laquelle cette politique est activée. Par défaut : `* * * * *` |
| `timezone`           | Le fuseau horaire utilisé lors de l'évaluation de la période unix-cron. Par défaut : Le fuseau horaire local du système. |
| `idle_count`         | La capacité inactive cible que nous souhaitons rendre immédiatement disponible pour les jobs. |
| `idle_time`          | La durée pendant laquelle une instance peut être inactive avant d'être arrêtée. |
| `scale_factor`       | La capacité inactive cible que nous souhaitons rendre immédiatement disponible pour les jobs, en plus de `idle_count`, en tant que facteur de la capacité actuellement utilisée. Par défaut `0.0`. |
| `scale_factor_limit` | La capacité maximale que le calcul de `scale_factor` peut produire. |
| `preemptive_mode`    | Avec le mode préemptif activé, les jobs ne sont demandés que lorsqu'une instance est confirmée comme disponible. Cette action permet aux jobs de démarrer presque immédiatement sans délais de provisionnement. Lorsque le mode préemptif est désactivé, les jobs sont d'abord demandés, puis le système tente de trouver ou de provisionner la capacité nécessaire. |

Pour décider s'il faut supprimer une instance inactive, le taskscaler compare `idle_time` à la durée d'inactivité de l'instance. La période d'inactivité de chaque instance est calculée à partir du moment où l'instance :

- A terminé un job pour la dernière fois (si l'instance a déjà été utilisée).
- Est provisionnée (si jamais utilisée).

Cette vérification se produit lors des événements de mise à l'échelle. Les instances qui dépassent la valeur configurée `idle_time` sont supprimées, sauf si elles sont nécessaires pour maintenir la capacité de jobs `idle_count` requise.

Lorsque `scale_factor` est défini, `idle_count` devient la capacité minimale `idle` et `scaler_factor_limit` la capacité maximale `idle`.

Vous pouvez définir plusieurs politiques. La dernière politique correspondante est celle utilisée.

Dans l'exemple suivant, le nombre inactif `1` est utilisé entre 08h00 et 15h59, du lundi au vendredi. Sinon, le nombre inactif est 0.

```toml
[[runners.autoscaler.policy]]
  idle_count        = 0
  idle_time         = "0s"
  periods           = ["* * * * *"]

[[runners.autoscaler.policy]]
  idle_count        = 1
  idle_time         = "30m0s"
  periods           = ["* 8-15 * * mon-fri"]
```

### Syntaxe des périodes {#periods-syntax-1}

Le paramètre `periods` contient un tableau de chaînes au format unix-cron pour indiquer la période pendant laquelle une politique est activée. Le format cron se compose de 5 champs :

```plaintext
 ┌────────── minute (0 - 59)
 │ ┌──────── hour (0 - 23)
 │ │ ┌────── day of month (1 - 31)
 │ │ │ ┌──── month (1 - 12)
 │ │ │ │ ┌── day of week (1 - 7 or MON-SUN, 0 is an alias for Sunday)
 * * * * *
```

- `-` peut être utilisé entre deux nombres pour spécifier une plage.
- `*` peut être utilisé pour représenter toute la plage de valeurs valides pour ce champ.
- `/` suivi d'un nombre ou peut être utilisé après une plage pour sauter ce nombre dans la plage. Par exemple, 0-12/2 pour le champ heure activerait la période toutes les 2 heures entre 00:00 et 00:12.
- `,` peut être utilisé pour séparer une liste de nombres valides ou de plages pour le champ. Par exemple, `1,2,6-9`.

Il est utile de garder à l'esprit que ce cron job représente une plage dans le temps. Par exemple :

| Période               | Effet |
|----------------------|--------|
| `1 * * * * *`        | Règle activée pour la période de 1 minute chaque heure (peu susceptible d'être très efficace) |
| `* 0-12 * * *`       | Règle activée pour la période de 12 heures au début de chaque jour |
| `0-30 13,16 * * SUN` | Règle activée pour la période de chaque dimanche pendant 30 minutes à 13h et 30 minutes à 16h. |

## La section `[runners.autoscaler.vm_isolation]` {#the-runnersautoscalervm_isolation-section}

L'isolation VM utilise [`nesting`](../executors/instance.md#nested-virtualization), qui n'est pris en charge que sur macOS.

| Paramètre        | Description |
|------------------|-------------|
| `enabled`        | Spécifie si l'isolation VM est activée ou non. Par défaut : `false`. |
| `nesting_host`   | L'hôte du démon `nesting`. |
| `nesting_config` | La configuration `nesting`, qui est sérialisée en JSON et envoyée au démon `nesting`. |
| `image`          | L'image par défaut utilisée par le démon nesting si aucune image de job n'est spécifiée. |

## La section `[runners.autoscaler.vm_isolation.connector_config]` {#the-runnersautoscalervm_isolationconnector_config-section}

Les paramètres de la section `[runners.autoscaler.vm_isolation.connector_config]` sont identiques à ceux de la section [`[runners.autoscaler.connector_config]`](#the-runnersautoscalerconnector_config-section), mais sont utilisés pour se connecter à la machine virtuelle provisionnée par `nesting`, plutôt qu'à l'instance à mise à l'échelle automatique.

## La section `[runners.custom]` {#the-runnerscustom-section}

Les paramètres suivants définissent la configuration de l'[exécuteur personnalisé](../executors/custom.md).

| Paramètre               | Type         | Description |
|-------------------------|--------------|-------------|
| `config_exec`           | string       | Chemin vers un exécutable, permettant à un utilisateur de remplacer certains paramètres de configuration avant le démarrage du job. Ces valeurs remplacent celles définies dans la section [`[[runners]]`](#the-runners-section). [La documentation de l'exécuteur personnalisé](../executors/custom.md#config) contient la liste complète. |
| `config_args`           | string array | Premier ensemble d'arguments transmis à l'exécutable `config_exec`. |
| `config_exec_timeout`   | entier      | Délai d'attente, en secondes, pour que `config_exec` termine son exécution. La valeur par défaut est 3600 secondes (1 heure). |
| `prepare_exec`          | string       | Chemin vers un exécutable pour préparer l'environnement. |
| `prepare_args`          | string array | Premier ensemble d'arguments transmis à l'exécutable `prepare_exec`. |
| `prepare_exec_timeout`  | entier      | Délai d'attente, en secondes, pour que `prepare_exec` termine son exécution. La valeur par défaut est 3600 secondes (1 heure). |
| `run_exec`              | string       | **Obligatoire**. Chemin vers un exécutable pour exécuter des scripts dans les environnements. Par exemple, le script de clonage et de build. |
| `run_args`              | string array | Premier ensemble d'arguments transmis à l'exécutable `run_exec`. |
| `cleanup_exec`          | string       | Chemin vers un exécutable pour nettoyer l'environnement. |
| `cleanup_args`          | string array | Premier ensemble d'arguments transmis à l'exécutable `cleanup_exec`. |
| `cleanup_exec_timeout`  | entier      | Délai d'attente, en secondes, pour que `cleanup_exec` termine son exécution. La valeur par défaut est 3600 secondes (1 heure). |
| `graceful_kill_timeout` | entier      | Temps d'attente, en secondes, pour `prepare_exec` et `cleanup_exec` s'ils sont arrêtés (par exemple, lors d'une annulation de job). Après ce délai, le processus est tué. La valeur par défaut est 600 secondes (10 minutes). |
| `force_kill_timeout`    | entier      | Temps d'attente, en secondes, après l'envoi du signal kill au script. La valeur par défaut est 600 secondes (10 minutes). |

## La section `[runners.cache]` {#the-runnerscache-section}

Les paramètres suivants définissent la fonctionnalité de cache distribué. Consultez les détails dans la [documentation sur la mise à l'échelle automatique du runner](autoscale.md#distributed-runners-caching).

| Paramètre                | Type    | Description |
|--------------------------|---------|-------------|
| `Type`                   | string  | L'un des suivants : `s3`, `gcs`, `azure`. |
| `Path`                   | string  | Nom du chemin à ajouter en préfixe à l'URL du cache. |
| `Shared`                 | booléen | Active le partage du cache entre les runners. La valeur par défaut est `false`. |
| `MaxUploadedArchiveSize` | int64   | Limite, en octets, de l'archive de cache téléversée vers le stockage cloud. Un acteur malveillant peut contourner cette limite, c'est pourquoi l'adaptateur GCS l'applique via l'en-tête X-Goog-Content-Length-Range dans l'URL signée. Vous devriez également définir la limite auprès de votre fournisseur de stockage cloud. |

Vous pouvez utiliser les variables d'environnement suivantes pour configurer la compression du cache :

| Variable                   | Description                           | Valeur par défaut   | Valeurs                                          |
|----------------------------|---------------------------------------|-----------|-------------------------------------------------|
| `CACHE_COMPRESSION_FORMAT` | Format de compression pour les archives de cache | `zip`     | `zip`, `tarzstd`                                |
| `CACHE_COMPRESSION_LEVEL`  | Niveau de compression pour les archives de cache  | `default` | `fastest`, `fast`, `default`, `slow`, `slowest` |

Le format `tarzstd` utilise TAR avec la compression Zstandard, ce qui offre de meilleurs taux de compression que `zip`. Les niveaux de compression vont de `fastest` (compression minimale pour une vitesse maximale) à `slowest` (compression maximale pour une taille de fichier minimale). Le niveau `default` offre un compromis équilibré entre le taux de compression et la vitesse.

Exemple :

```yaml
job:
  variables:
    CACHE_COMPRESSION_FORMAT: tarzstd
    CACHE_COMPRESSION_LEVEL: fast
```

### Transferts de stockage d'objets en cache parallèles {#parallel-cache-object-storage-transfers}

Par défaut, les téléchargements de cache utilisent un seul flux HTTP GET ou GoCloud, et les téléversements de cache utilisant le chemin GoCloud (par exemple S3 avec `RoleARN`) utilisent une seule partie multipart simultanée à la fois.

Vous pouvez activer un débit plus élevé sur des liaisons rapides vers le stockage d'objets avec le `FF_USE_PARALLEL_CACHE_TRANSFER` [feature flag](feature-flags.md). Lorsqu'il est activé :

- **Téléchargements** : plusieurs requêtes GET de plage simultanées peuvent être utilisées (URL pré-signée ; une petite requête Range initiale est utilisée à la place de HEAD, qui échoue souvent pour les URL pré-signées en lecture seule telles que S3) ou des lectures de plage GoCloud simultanées, lorsque le backend prend en charge les plages et que l'objet de cache est plus grand qu'un chunk.
- **Téléversements** sur le chemin GoCloud utilisent des téléversements multipart avec des parties simultanées.

Lorsque le feature flag est désactivé, le comportement est inchangé quelles que soient les variables ci-dessous. Vous pouvez ajuster le parallélisme avec ces variables d'environnement de job (elles sont lues par les helpers `cache-extractor` et `cache-archiver`) :

| Variable                     | Description                                                                 | Valeur par défaut |
|------------------------------|-----------------------------------------------------------------------------|---------|
| `CACHE_CHUNK_SIZE`           | Taille des chunks en octets pour les téléchargements de plage parallèles et la taille des parties multipart pour les téléversements GoCloud | `16777216` (16 Mio) |
| `CACHE_CONCURRENCY`          | Nombre de téléchargements de plage simultanés ou de parties de téléversement simultanées (GoCloud). Utilisez `0` ou `1` pour les téléchargements séquentiels. | `16` |
| `CACHE_TRANSFER_BUFFER_SIZE` | Taille du tampon en octets lors du streaming vers ou depuis le fichier d'archive           | `4194304` (4 Mio) |

Exemple :

```yaml
job:
  variables:
    FF_USE_PARALLEL_CACHE_TRANSFER: "true"
    CACHE_CONCURRENCY: "8"
    CACHE_CHUNK_SIZE: "16777216"
```

### Téléchargements d'artefacts en parallèle (téléchargement direct) {#parallel-artifact-downloads-direct-download}

Par défaut, lorsque [`direct_download`](https://docs.gitlab.com/ci/jobs/job_artifacts/#download-artifacts-from-a-job) renvoie une redirection vers le stockage d'objets, le runner télécharge les artefacts avec un seul flux HTTP GET.

Activez le `FF_USE_PARALLEL_ARTIFACT_TRANSFER` [feature flag](feature-flags.md) pour autoriser les requêtes HTTP Range GET parallèles lorsque le backend de stockage d'objets prend en charge `206 Partial Content` avec un total `Content-Range`. La taille des chunks et la simultanéité sont fixées dans le runner (pas les variables `CACHE_*`). Ce flag est indépendant de `FF_USE_PARALLEL_CACHE_TRANSFER`.

Exemple :

```yaml
job:
  variables:
    FF_USE_PARALLEL_ARTIFACT_TRANSFER: "true"
```

Le mécanisme de cache utilise des URL pré-signées pour téléverser et télécharger le cache. Les URL sont signées par GitLab Runner sur sa propre instance. Peu importe si le script du job (y compris le script de téléversement/téléchargement du cache) est exécuté sur des machines locales ou externes. Par exemple, les exécuteurs `shell` ou `docker` exécutent leurs scripts sur la même machine où le processus GitLab Runner est en cours d'exécution. En même temps, `virtualbox` ou `docker+machine` se connecte à une VM séparée pour exécuter le script. Ce processus est motivé par des raisons de sécurité : minimiser la possibilité de fuite des informations d'identification de l'adaptateur de cache.

Si l'[adaptateur de cache S3](#the-runnerscaches3-section) est configuré pour utiliser un profil d'instance IAM, l'adaptateur utilise le profil attaché à la machine GitLab Runner. De même pour l'[adaptateur de cache GCS](#the-runnerscachegcs-section), s'il est configuré pour utiliser le `CredentialsFile`. Le fichier doit être présent sur la machine GitLab Runner.

Ce tableau répertorie les options `config.toml`, CLI et les variables d'environnement pour `register`. Lorsque vous définissez ces variables d'environnement, les valeurs sont enregistrées dans `config.toml` après l'enregistrement d'un nouveau GitLab Runner.

Si vous souhaitez omettre les informations d'identification S3 de `config.toml` et charger des informations d'identification statiques depuis l'environnement, vous pouvez définir `AWS_ACCESS_KEY_ID` et `AWS_SECRET_ACCESS_KEY`. Pour plus d'informations, consultez la [section sur la chaîne d'informations d'identification par défaut du SDK AWS](#aws-sdk-default-credential-chain).

| Paramètre                        | Champ TOML                                        | Option CLI pour `register`                  | Variable d'environnement pour `register` |
|--------------------------------|---------------------------------------------------|--------------------------------------------|-------------------------------------|
| `Type`                         | `[runners.cache] -> Type`                         | `--cache-type`                             | `$CACHE_TYPE`                       |
| `Path`                         | `[runners.cache] -> Path`                         | `--cache-path`                             | `$CACHE_PATH`                       |
| `Shared`                       | `[runners.cache] -> Shared`                       | `--cache-shared`                           | `$CACHE_SHARED`                     |
| `S3.ServerAddress`             | `[runners.cache.s3] -> ServerAddress`             | `--cache-s3-server-address`                | `$CACHE_S3_SERVER_ADDRESS`          |
| `S3.AccessKey`                 | `[runners.cache.s3] -> AccessKey`                 | `--cache-s3-access-key`                    | `$CACHE_S3_ACCESS_KEY`              |
| `S3.SecretKey`                 | `[runners.cache.s3] -> SecretKey`                 | `--cache-s3-secret-key`                    | `$CACHE_S3_SECRET_KEY`              |
| `S3.SessionToken`              | `[runners.cache.s3] -> SessionToken`              | `--cache-s3-session-token`                 | `$CACHE_S3_SESSION_TOKEN`           |
| `S3.BucketName`                | `[runners.cache.s3] -> BucketName`                | `--cache-s3-bucket-name`                   | `$CACHE_S3_BUCKET_NAME`             |
| `S3.BucketLocation`            | `[runners.cache.s3] -> BucketLocation`            | `--cache-s3-bucket-location`               | `$CACHE_S3_BUCKET_LOCATION`         |
| `S3.Insecure`                  | `[runners.cache.s3] -> Insecure`                  | `--cache-s3-insecure`                      | `$CACHE_S3_INSECURE`                |
| `S3.AuthenticationType`        | `[runners.cache.s3] -> AuthenticationType`        | `--cache-s3-authentication_type`           | `$CACHE_S3_AUTHENTICATION_TYPE`     |
| `S3.ServerSideEncryption`      | `[runners.cache.s3] -> ServerSideEncryption`      | `--cache-s3-server-side-encryption`        | `$CACHE_S3_SERVER_SIDE_ENCRYPTION`  |
| `S3.ServerSideEncryptionKeyID` | `[runners.cache.s3] -> ServerSideEncryptionKeyID` | `--cache-s3-server-side-encryption-key-id` | `$CACHE_S3_SERVER_SIDE_ENCRYPTION_KEY_ID` |
| `S3.DualStack`                 | `[runners.cache.s3] -> DualStack`                 | `--cache-s3-dual-stack`                    | `$CACHE_S3_DUAL_STACK`              |
| `S3.Accelerate`                | `[runners.cache.s3] -> Accelerate`                | `--cache-s3-accelerate`                    | `$CACHE_S3_ACCELERATE`              |
| `S3.PathStyle`                 | `[runners.cache.s3] -> PathStyle`                 | `--cache-s3-path-style`                    | `$CACHE_S3_PATH_STYLE`              |
| `S3.RoleARN`                   | `[runners.cache.s3] -> RoleARN`                   | `--cache-s3-role-arn`                      | `$CACHE_S3_ROLE_ARN`                |
| `S3.UploadRoleARN`             | `[runners.cache.s3] -> UploadRoleARN`             | `--cache-s3-upload-role-arn`               | `$CACHE_S3_UPLOAD_ROLE_ARN`         |
| `S3.AssumeRoleMaxConcurrency`  | `[runners.cache.s3] -> AssumeRoleMaxConcurrency`  | `--cache-s3-assume-role-max-concurrency`   | `$CACHE_S3_ASSUME_ROLE_MAX_CONCURRENCY` |
| `GCS.AccessID`                 | `[runners.cache.gcs] -> AccessID`                 | `--cache-gcs-access-id`                    | `$CACHE_GCS_ACCESS_ID`              |
| `GCS.PrivateKey`               | `[runners.cache.gcs] -> PrivateKey`               | `--cache-gcs-private-key`                  | `$CACHE_GCS_PRIVATE_KEY`            |
| `GCS.CredentialsFile`          | `[runners.cache.gcs] -> CredentialsFile`          | `--cache-gcs-credentials-file`             | `$GOOGLE_APPLICATION_CREDENTIALS`   |
| `GCS.BucketName`               | `[runners.cache.gcs] -> BucketName`               | `--cache-gcs-bucket-name`                  | `$CACHE_GCS_BUCKET_NAME`            |
| `Azure.AccountName`            | `[runners.cache.azure] -> AccountName`            | `--cache-azure-account-name`               | `$CACHE_AZURE_ACCOUNT_NAME`         |
| `Azure.AccountKey`             | `[runners.cache.azure] -> AccountKey`             | `--cache-azure-account-key`                | `$CACHE_AZURE_ACCOUNT_KEY`          |
| `Azure.ContainerName`          | `[runners.cache.azure] -> ContainerName`          | `--cache-azure-container-name`             | `$CACHE_AZURE_CONTAINER_NAME`       |
| `Azure.StorageDomain`          | `[runners.cache.azure] -> StorageDomain`          | `--cache-azure-storage-domain`             | `$CACHE_AZURE_STORAGE_DOMAIN`       |

### Gestion des clés de cache {#cache-key-handling}

{{< history >}}

- [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5751) dans GitLab Runner 18.4.0.
- Le chemin d'objet dans les caches distribués a [été modifié](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6628) dans GitLab Runner 19.0 pour inclure un préfixe de shard lorsque `FF_HASH_CACHE_KEYS` est activé.

{{< /history >}}

Dans GitLab Runner 18.4.0 et versions ultérieures, vous pouvez hacher les clés de cache avec le `FF_HASH_CACHE_KEYS` [feature flag](feature-flags.md).

Lorsque `FF_HASH_CACHE_KEYS` est désactivé (par défaut), GitLab Runner assainit la clé de cache avant de l'utiliser pour construire le chemin du fichier de cache local et de l'objet dans le bucket de stockage. Si l'assainissement modifie la clé de cache, GitLab Runner consigne ce changement. Si GitLab Runner ne peut pas assainir la clé de cache, il le consigne également et n'utilise pas ce cache spécifique.

Lorsque vous activez ce feature flag, GitLab Runner hache la clé de cache (SHA-256) avant de l'utiliser pour construire le chemin de l'artefact de cache local et de l'objet dans le bucket de stockage distant. GitLab Runner n'assainit pas la clé de cache. Pour vous aider à comprendre quelle clé de cache a créé un artefact de cache spécifique, GitLab Runner lui attache des métadonnées :

- Pour les artefacts de cache locaux, GitLab Runner place un fichier `metadata.json` à côté de l'artefact de cache `cache.zip`, avec le contenu suivant :

  ```json
  {"cachekey": "the human readable cache key"}
  ```

- Pour les artefacts de cache sur les caches distribués, GitLab Runner attache les métadonnées directement au blob de l'objet de stockage, avec la clé `cachekey`. Vous pouvez l'interroger à l'aide des mécanismes du fournisseur cloud. Pour un exemple, consultez les [métadonnées d'objet définies par l'utilisateur](https://docs.aws.amazon.com/AmazonS3/latest/userguide/UsingMetadata.html#UserMetadata) pour AWS S3.

#### Chemin d'objet de cache distribué avec `FF_HASH_CACHE_KEYS` {#distributed-cache-object-path-with-ff_hash_cache_keys}

Dans GitLab Runner 19.0 et versions ultérieures, lorsque `FF_HASH_CACHE_KEYS` est activé, GitLab Runner insère les deux premiers caractères hexadécimaux du hachage SHA-256 comme préfixe de shard dans le chemin d'objet du cache distribué :

```plaintext
[path/][runner/<token>/]project/<project_id>/<shard>/<hash>/cache.zip
```

Par exemple :

```plaintext
runner/abc123/project/42/d0/d03a852ba491ba611e907b1ef60ad5c4516a05b8f3aae6abb77f42bc60325aed/cache.zip
```

Cela distribue les objets de cache sur 256 préfixes d'objets distincts par projet, ce qui empêche les [réponses Amazon S3 503 (Slow Down)](https://docs.aws.amazon.com/AmazonS3/latest/userguide/optimizing-performance.html) lorsque de nombreux jobs parallèles accèdent au cache à des taux de requêtes élevés.

> [!warning]
> La mise à niveau vers GitLab Runner 19.0 est un changement radical si vous utilisez `FF_HASH_CACHE_KEYS`. Si vous avez déjà `FF_HASH_CACHE_KEYS` activé et que vous effectuez une mise à niveau vers GitLab Runner 19.0 ou version ultérieure, le préfixe de shard modifie le chemin d'objet pour tous les artefacts de cache dans le stockage distribué. Les objets existants stockés à l'ancien chemin (`.../<hash>/cache.zip`) deviennent inaccessibles. Attendez-vous à des défauts de cache et à une reconstruction des artefacts de cache lors de la première exécution de job après la mise à niveau.

#### Résumé du comportement de gestion des clés de cache {#cache-key-handling-behavior-summary}

Lorsque vous modifiez `FF_HASH_CACHE_KEYS`, GitLab Runner ignore les artefacts de cache existants car le hachage de la clé de cache modifie le nom et l'emplacement de l'artefact de cache. Ce changement s'applique dans les deux sens, de `FF_HASH_CACHE_KEYS=true` à `FF_HASH_CACHE_KEYS=false` et vice versa.

Si vous exécutez plusieurs runners qui partagent un cache distribué mais ont des paramètres différents pour `FF_HASH_CACHE_KEYS`, ils ne partagent pas les artefacts de cache.

Par conséquent, la bonne pratique est :

- Gardez `FF_HASH_CACHE_KEYS` synchronisé entre les runners qui partagent des caches distribués.

- Attendez-vous à des défauts de cache, à une reconstruction des artefacts de cache et à des premières exécutions de jobs plus longues après avoir modifié `FF_HASH_CACHE_KEYS`.

- Attendez-vous à des requêtes réseau supplémentaires pendant la période de transition tandis que GitLab Runner vérifie les emplacements de cache principal et alternatif.

> [!warning]
> Si vous activez `FF_HASH_CACHE_KEYS` mais exécutez une version plus ancienne du binaire helper (par exemple, parce que vous avez épinglé l'image helper à une version plus ancienne), le hachage de la clé de cache et le téléversement ou le téléchargement des caches fonctionnent toujours. Cependant, GitLab Runner ne maintient pas les métadonnées des artefacts de cache.

### La section `[runners.cache.s3]` {#the-runnerscaches3-section}

Les paramètres suivants définissent le stockage S3 pour le cache.

| Paramètre                   | Type    | Description |
|-----------------------------|---------|-------------|
| `ServerAddress`             | string  | Un `host:port` pour le serveur compatible S3. Si vous utilisez un serveur autre qu'AWS, consultez la documentation du produit de stockage pour déterminer l'adresse correcte. Pour DigitalOcean, l'adresse doit être au format `spacename.region.digitaloceanspaces.com`. |
| `AccessKey`                 | string  | La clé d'accès spécifiée pour votre instance S3. |
| `SecretKey`                 | string  | La clé secrète spécifiée pour votre instance S3. |
| `SessionToken`              | string  | Le jeton de session spécifié pour votre instance S3 lorsque des informations d'identification temporaires sont utilisées. |
| `BucketName`                | string  | Nom du bucket de stockage où le cache est stocké. |
| `BucketLocation`            | string  | Nom de la région S3. |
| `Insecure`                  | booléen | Définissez sur `true` si le service S3 est disponible via `HTTP`. La valeur par défaut est `false`. |
| `AuthenticationType`        | string  | Définissez sur `iam` ou `access-key`. La valeur par défaut est `access-key` si `ServerAddress`, `AccessKey` et `SecretKey` sont tous fournis. Par défaut à `iam` si `ServerAddress`, `AccessKey` ou `SecretKey` sont manquants. |
| `ServerSideEncryption`      | string  | Le type de chiffrement côté serveur à utiliser avec S3. Dans GitLab 15.3 et versions ultérieures, les types disponibles sont `S3` ou `KMS`. Dans GitLab 17.5 et versions ultérieures, [`DSSE-KMS`](https://docs.aws.amazon.com/AmazonS3/latest/userguide/UsingDSSEncryption.html) est pris en charge. |
| `ServerSideEncryptionKeyID` | string  | L'alias, l'ID ou l'ARN d'une clé KMS utilisée pour le chiffrement lorsque vous utilisez KMS. Si vous utilisez un alias, préfixez-le avec `alias/`. Utilisez le format ARN pour les scénarios inter-comptes. Disponible dans GitLab 15.3 et versions ultérieures. |
| `DualStack`                 | booléen | Active les points de terminaison IPv4 et IPv6. La valeur par défaut est `true`. Désactivez ce paramètre si vous utilisez AWS S3 Express. GitLab ignore ce paramètre si vous définissez `ServerAddress`. Disponible dans GitLab 17.5 et versions ultérieures. |
| `Accelerate`                | booléen | Active l'accélération de transfert AWS S3. GitLab définit automatiquement cette valeur à `true` si `ServerAddress` est configuré en tant que point de terminaison accéléré. Disponible dans GitLab 17.5 et versions ultérieures. |
| `PathStyle`                 | booléen | Active l'accès par style de chemin. Par défaut, GitLab détecte automatiquement ce paramètre en fonction de la valeur de `ServerAddress`. Disponible dans GitLab 17.5 et versions ultérieures. |
| `UploadRoleARN`             | string  | Obsolète. Utilisez plutôt `RoleARN`. Spécifie un ARN de rôle AWS pouvant être utilisé avec `AssumeRole` pour générer des requêtes S3 `PutObject` à durée limitée. Active les chargements multipartites S3. Disponible dans GitLab 17.5 et versions ultérieures. |
| `RoleARN`                   | string  | Spécifie un ARN de rôle AWS pouvant être utilisé avec `AssumeRole` pour générer des requêtes S3 `GetObject` et `PutObject` à durée limitée. Active les transferts multipartites S3. Disponible dans GitLab 17.8 et versions ultérieures. |
| `AssumeRoleMaxConcurrency`  | entier | Nombre maximum de requêtes `AssumeRole` simultanées vers AWS STS lorsque `RoleARN` est défini. Par défaut `5`. Définissez la valeur à `-1` pour supprimer la limite. |

Exemple :

```toml
[runners.cache]
  Type = "s3"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.s3]
    ServerAddress = "s3.amazonaws.com"
    AccessKey = "AWS_S3_ACCESS_KEY"
    SecretKey = "AWS_S3_SECRET_KEY"
    BucketName = "runners-cache"
    BucketLocation = "eu-west-1"
    Insecure = false
    ServerSideEncryption = "KMS"
    ServerSideEncryptionKeyID = "alias/my-key"
```

## Authentification {#authentication}

GitLab Runner utilise différentes méthodes d'authentification pour S3 en fonction de votre configuration.

### Identifiants statiques {#static-credentials}

Le runner utilise l'authentification par clé d'accès statique dans les cas suivants :

- Les paramètres `ServerAddress`, `AccessKey` et `SecretKey` sont spécifiés mais `AuthenticationType` n'est pas fourni.
- `AuthenticationType = "access-key"` est explicitement défini.

### Chaîne d'identification par défaut du SDK AWS {#aws-sdk-default-credential-chain}

Le runner utilise la [chaîne d'identification par défaut du SDK AWS](https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials) dans les cas suivants :

- L'un des paramètres `ServerAddress`, `AccessKey` ou `SecretKey` est omis et `AuthenticationType` n'est pas fourni.
- `AuthenticationType = "iam"` est explicitement défini.

La chaîne d'identification tente l'authentification dans l'ordre suivant :

1. Variables d'environnement (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
1. Fichier d'identifiants partagé (`~/.aws/credentials`)
1. Profil d'instance IAM (pour les instances EC2)
1. Autres sources d'identifiants AWS prises en charge par le SDK

Si `RoleARN` n'est pas spécifié, la chaîne d'identification par défaut est exécutée par le gestionnaire de runner, qui n'est souvent pas nécessairement sur la même machine où la build s'exécute. Par exemple, dans une configuration [de mise à l'échelle automatique](autoscale.md), le job s'exécute sur une machine différente. De même, avec l'exécuteur Kubernetes, le pod de build peut également s'exécuter sur un nœud différent de celui du gestionnaire de runner. Ce comportement permet d'accorder l'accès au niveau du compartiment uniquement au gestionnaire de runner.

Si `RoleARN` est spécifié, les identifiants sont résolus dans le contexte d'exécution de l'image d'aide. Pour plus d'informations, voir [RoleARN](#enable-multipart-transfers-with-rolearn).

Lorsque vous utilisez des charts Helm pour installer GitLab Runner et que `rbac.create` est défini sur `true` dans le fichier `values.yaml`, un compte de service est créé. Les annotations du compte de service sont récupérées depuis la section `rbac.serviceAccountAnnotations`.

Pour les runners sur Amazon EKS, vous pouvez spécifier un rôle IAM à affecter au compte de service. L'annotation spécifique nécessaire est : `eks.amazonaws.com/role-arn: arn:aws:iam::<ACCOUNT_ID>:role/<IAM_ROLE_NAME>`.

La politique IAM pour ce rôle doit avoir les autorisations pour effectuer les actions suivantes sur le compartiment spécifié :

- `s3:PutObject`
- `s3:GetObjectVersion`
- `s3:GetObject`
- `s3:DeleteObject`
- `s3:ListBucket`

Si vous utilisez `ServerSideEncryption` de type `KMS`, ce rôle doit également avoir l'autorisation d'effectuer les actions suivantes pour la clé AWS KMS spécifiée :

- `kms:Encrypt`
- `kms:Decrypt`
- `kms:ReEncrypt*`
- `kms:GenerateDataKey*`
- `kms:DescribeKey`

`ServerSideEncryption` de type `SSE-C` n'est pas pris en charge. `SSE-C` nécessite que les en-têtes contenant la clé fournie par l'utilisateur soient fournis pour la requête de téléchargement, en plus de l'URL pré-signée. Cela impliquerait de transmettre le matériel de clé au job, où la clé ne peut pas être conservée en sécurité. Cela comporte le risque de divulguer la clé de déchiffrement. Une discussion sur ce problème est disponible dans [cette merge request](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3295).

> [!note]
> La taille maximale d'un fichier unique pouvant être chargé dans le cache AWS S3 est de 5 Go. Une discussion sur les solutions de contournement potentielles pour ce comportement est disponible dans [ce ticket](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26921).

#### Utiliser le chiffrement par clé KMS dans un compartiment S3 pour le cache du runner {#use-kms-key-encryption-in-s3-bucket-for-runner-cache}

L'API `GenerateDataKey` utilise la clé symétrique KMS pour créer une clé de données pour le chiffrement côté client (<https://docs.aws.amazon.com/kms/latest/APIReference/API_GenerateDataKey.html>). La configuration de la clé KMS doit être la suivante :

| Attribut | Description |
|-----------|-------------|
| Type de clé  | Symétrique   |
| Origine    | `AWS_KMS`   |
| Spécification de clé  | `SYMMETRIC_DEFAULT` |
| Utilisation de la clé | Chiffrer et déchiffrer |

La politique IAM pour le rôle affecté au ServiceAccount défini dans `rbac.serviceAccountName` doit avoir les autorisations pour effectuer les actions suivantes pour la clé KMS :

- `kms:GetPublicKey`
- `kms:Decrypt`
- `kms:Encrypt`
- `kms:DescribeKey`
- `kms:GenerateDataKey`

#### Activer les transferts multipartites avec `RoleARN` {#enable-multipart-transfers-with-rolearn}

Pour limiter l'accès au cache, le gestionnaire de runner génère des [URL pré-signées](https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html) à durée limitée pour que les jobs puissent télécharger et charger dans le cache. Cependant, AWS S3 limite [une seule requête PUT à 5 Go](https://docs.aws.amazon.com/AmazonS3/latest/userguide/upload-objects.html). Pour les fichiers de plus de 5 Go, vous devez utiliser l'API de chargement multipartite.

Les transferts multipartites sont uniquement pris en charge avec AWS S3 et non avec d'autres fournisseurs S3. Comme le gestionnaire de runner gère les jobs pour différents projets, il ne peut pas transmettre des identifiants S3 disposant d'autorisations au niveau du compartiment. Au lieu de cela, le gestionnaire de runner utilise des URL pré-signées à durée limitée et des identifiants à portée étroite pour restreindre l'accès à un objet spécifique.

Pour utiliser les transferts multipartites S3 avec AWS, spécifiez un rôle IAM dans `RoleARN` au format `arn:aws:iam:::<ACCOUNT ID>:<YOUR ROLE NAME>`. Ce rôle génère des identifiants AWS à durée limitée dont la portée est étroitement définie pour écrire dans un blob spécifique dans le compartiment. Assurez-vous que vos identifiants S3 d'origine peuvent accéder à `AssumeRole` pour le `RoleARN` spécifié.

Le rôle IAM spécifié dans `RoleARN` doit disposer des autorisations suivantes :

- Accès `s3:GetObject` au compartiment spécifié dans `BucketName`.
- Accès `s3:PutObject` au compartiment spécifié dans `BucketName`.
- Accès `s3:ListBucket` au compartiment spécifié dans `BucketName`.
- `kms:Decrypt` et `kms:GenerateDataKey` si le chiffrement côté serveur avec KMS ou DSSE-KMS est activé.

Par exemple, supposez que vous disposez d'un rôle IAM appelé `my-instance-role` attaché à une instance EC2 avec l'ARN `arn:aws:iam::1234567890123:role/my-instance-role`.

Vous pouvez créer un nouveau rôle `arn:aws:iam::1234567890123:role/my-upload-role` qui ne dispose que des autorisations `s3:PutObject` pour `BucketName`. Dans les paramètres AWS pour `my-instance-role`, les `Trust relationships` peuvent ressembler à ceci :

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "AWS": "arn:aws:iam::1234567890123:role/my-upload-role"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}
```

Vous pouvez également réutiliser `my-instance-role` en tant que `RoleARN` pour éviter de créer un nouveau rôle. Assurez-vous que `my-instance-role` dispose de l'autorisation `AssumeRole`. Par exemple, un profil IAM associé à une instance EC2 peut avoir les `Trust relationships` suivantes :

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Service": "ec2.amazonaws.com",
                "AWS": "arn:aws:iam::1234567890123:role/my-instance-role"
            },
            "Action": "sts:AssumeRole"
        }
    ]
}
```

Vous pouvez utiliser l'interface de ligne de commande AWS pour vérifier que votre instance dispose de l'autorisation `AssumeRole`. Par exemple :

```shell
aws sts assume-role --role-arn arn:aws:iam::1234567890123:role/my-upload-role --role-session-name gitlab-runner-test1
```

##### Fonctionnement des chargements avec `RoleARN` {#how-uploads-work-with-rolearn}

Si `RoleARN` est présent, chaque fois que le runner charge des données dans le cache :

1. Le gestionnaire de runner récupère les identifiants S3 d'origine (spécifiés via `AuthenticationType`, `AccessKey` et `SecretKey`).
1. Avec les identifiants S3, le gestionnaire de runner envoie une requête à l'Amazon Security Token Service (STS) pour [`AssumeRole`](https://docs.aws.amazon.com/STS/latest/APIReference/API_AssumeRole.html) avec `RoleARN`. La requête de politique ressemble à ceci :

   ```json
   {
       "Version": "2012-10-17",
       "Statement": [
           {
               "Effect": "Allow",
               "Action": ["s3:PutObject"],
               "Resource": "arn:aws:s3:::<YOUR-BUCKET-NAME>/<CACHE-FILENAME>"
           }
       ]
   }
   ```

1. Si la requête réussit, le gestionnaire de runner obtient des identifiants AWS temporaires avec une session restreinte.
1. Le gestionnaire de runner transmet ces identifiants et l'URL au format `s3://<bucket name>/<filename>` à l'archiveur de cache, qui charge ensuite le fichier.

##### Métriques Prometheus AssumeRole {#assumerole-prometheus-metrics}

Lorsque `RoleARN` est défini, GitLab Runner expose les métriques Prometheus suivantes pour surveiller le comportement des requêtes STS :

| Métrique | Type | Description |
|--------|------|-------------|
| `gitlab_runner_cache_s3_assume_role_requests_in_flight` | Gauge | Nombre de requêtes `AssumeRole` vers AWS STS en cours. |
| `gitlab_runner_cache_s3_assume_role_wait_seconds` | Histogramme | Temps d'attente pour acquérir un créneau de simultanéité avant d'émettre une requête `AssumeRole`. |
| `gitlab_runner_cache_s3_assume_role_duration_seconds` | Histogramme | Durée des appels API `AssumeRole` vers AWS STS. |
| `gitlab_runner_cache_s3_assume_role_cache_hits_total` | Compteur | Nombre de succès du cache d'identifiants `AssumeRole` (appel STS évité). |
| `gitlab_runner_cache_s3_assume_role_cache_misses_total` | Compteur | Nombre d'échecs du cache d'identifiants `AssumeRole` (appel STS effectué). |
| `gitlab_runner_cache_s3_assume_role_cached_credentials` | Gauge | Nombre d'identifiants `AssumeRole` conservés dans le cache LRU en mémoire. |
| `gitlab_runner_cache_s3_assume_role_failures_total` | Compteur | Nombre de requêtes `AssumeRole` ayant échoué. |

#### Activer les rôles IAM pour les ressources Kubernetes ServiceAccount {#enable-iam-roles-for-kubernetes-serviceaccount-resources}

Pour utiliser les rôles IAM pour les comptes de service, un fournisseur IAM OIDC [doit exister pour votre cluster](https://docs.aws.amazon.com/eks/latest/userguide/enable-iam-roles-for-service-accounts.html). Une fois qu'un fournisseur IAM OIDC est associé à votre cluster, vous pouvez créer un rôle IAM à associer au compte de service du runner.

1. Dans la fenêtre **Créer un rôle**, sous **Sélectionner le type d'entité de confiance**, sélectionnez **Identité Web**.
1. Dans l'onglet **Relations de confiance** du rôle :

   - La section **Entités de confiance** doit avoir le format : `arn:aws:iam::<ACCOUNT_ID>:oidc-provider/oidc.eks.<AWS_REGION>.amazonaws.com/id/<OIDC_ID>`. L'**OIDC ID** se trouve dans l'onglet **Configuration** du cluster EKS.

   - La section **Condition** doit avoir le compte de service GitLab Runner défini dans `rbac.serviceAccountName` ou le compte de service par défaut créé si `rbac.create` est défini sur `true` :

     | Condition      | Clé                                                    | Valeur |
     |----------------|--------------------------------------------------------|-------|
     | `StringEquals` | `oidc.eks.<AWS_REGION>.amazonaws.com/id/<OIDC_ID>:sub` | `system:serviceaccount:<GITLAB_RUNNER_NAMESPACE>:<GITLAB_RUNNER_SERVICE_ACCOUNT>` |

#### Utiliser des compartiments S3 Express One Zone {#use-s3-express-one-zone-buckets}

{{< history >}}

- Introduit dans GitLab Runner 17.5.0.

{{< /history >}}

> [!note]
> [Les compartiments de répertoire S3 Express One Zone ne fonctionnent pas avec `RoleARN`](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38484#note_2313111840) car le gestionnaire de runner ne peut pas restreindre l'accès à un objet spécifique.

1. Configurez un compartiment S3 Express One Zone en suivant le [tutoriel Amazon](https://docs.aws.amazon.com/AmazonS3/latest/userguide/s3-express-getting-started.html).
1. Configurez `config.toml` avec `BucketName` et `BucketLocation`.
1. Définissez `DualStack` sur `false` car S3 Express ne prend pas en charge les points de terminaison dual-stack.

Exemple de `config.toml` :

```toml
[runners.cache]
  Type = "s3"
  [runners.cache.s3]
    BucketName = "example-express--usw2-az1--x-s3"
    BucketLocation = "us-west-2"
    DualStack = false
```

### La section `[runners.cache.gcs]` {#the-runnerscachegcs-section}

Les paramètres suivants définissent la prise en charge native de Google Cloud Storage. Pour plus d'informations sur ces valeurs, consultez la [documentation d'authentification Google Cloud Storage (GCS)](https://docs.cloud.google.com/storage/docs/authentication#service_accounts).

| Paramètre         | Type   | Description |
|-------------------|--------|-------------|
| `CredentialsFile` | string | Chemin d'accès au fichier de clé JSON Google. Seul le type `service_account` est pris en charge. Si configurée, cette valeur est prioritaire sur `AccessID` et `PrivateKey` configurés directement dans `config.toml`. |
| `AccessID`        | string | ID du compte de service GCP utilisé pour accéder au stockage. |
| `PrivateKey`      | string | Clé privée utilisée pour signer les requêtes GCS. |
| `BucketName`      | string | Nom du bucket de stockage où le cache est stocké. |
| `UniverseDomain`  | string | Domaine universe pour les requêtes GCS (optionnel). Pour Google Cloud public, utilisez `googleapis.com`. Pour Google Cloud Dedicated ou d'autres domaines universe personnalisés, spécifiez le domaine approprié (par exemple, `custom.universe.com`). Si vous ne spécifiez pas de domaine, la valeur par défaut est `googleapis.com`. |

Exemples :

**Identifiants configurés directement dans le fichier `config.toml`** :

```toml
[runners.cache]
  Type = "gcs"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.gcs]
    AccessID = "cache-access-account@test-project-123456.iam.gserviceaccount.com"
    PrivateKey = "-----BEGIN PRIVATE KEY-----\nXXXXXX\n-----END PRIVATE KEY-----\n"
    BucketName = "runners-cache"
    UniverseDomain = "googleapis.com"  # Optional
```

**Credentials in JSON file downloaded from GCP** :

```toml
[runners.cache]
  Type = "gcs"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.gcs]
    CredentialsFile = "/etc/gitlab-runner/service-account.json"
    BucketName = "runners-cache"
    UniverseDomain = "googleapis.com"  # Optional
```

**Application Default Credentials (ADC) from the metadata server in GCP** :

Lorsque vous utilisez GitLab Runner avec Google Cloud ADC, vous utilisez généralement le compte de service par défaut. Vous n'avez alors pas besoin de fournir des identifiants pour l'instance :

```toml
[runners.cache]
  Type = "gcs"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.gcs]
    BucketName = "runners-cache"
    UniverseDomain = "googleapis.com"  # Optional
```

Si vous utilisez ADC, assurez-vous que le compte de service que vous utilisez dispose de l'autorisation `iam.serviceAccounts.signBlob`. Cela se fait généralement en accordant le [rôle Service Account Token Creator](https://docs.cloud.google.com/iam/docs/service-account-permissions#token-creator-role) au compte de service.

#### Workload Identity Federation pour GKE {#workload-identity-federation-for-gke}

Workload Identity Federation pour GKE est pris en charge avec les identifiants par défaut de l'application (ADC). Si vous rencontrez des problèmes lors de la mise en œuvre des identités de charge de travail :

- Vérifiez les journaux du pod runner (pas le journal de build) pour le message `ERROR: generating signed URL`. Cette erreur peut indiquer un problème d'autorisation, tel que :

  ```plaintext
  IAM returned 403 Forbidden: Permission 'iam.serviceAccounts.getAccessToken' denied on resource (or it may not exist).
  ```

- Essayez les commandes `curl` suivantes depuis le pod runner :

  ```shell
  curl -H "Metadata-Flavor: Google" http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/email
  ```

   Cette commande doit renvoyer le compte de service Kubernetes correct. Ensuite, essayez d'obtenir un jeton d'accès :

  ```shell
  curl -H "Metadata-Flavor: Google" http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/token?scopes=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcloud-platform
  ```

   Si la commande réussit, le résultat renvoie une charge utile JSON avec un jeton d'accès. En cas d'échec, vérifiez les autorisations du compte de service.

### La section `[runners.cache.azure]` {#the-runnerscacheazure-section}

Les paramètres suivants définissent la prise en charge native d'Azure Blob Storage. Pour en savoir plus, consultez la [documentation Azure Blob Storage](https://learn.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction). Alors que S3 et GCS utilisent le terme `bucket` pour désigner une collection d'objets, Azure utilise le terme `container` pour désigner une collection de blobs.

| Paramètre       | Type   | Description |
|-----------------|--------|-------------|
| `AccountName`   | string | Nom du compte Azure Blob Storage utilisé pour accéder au stockage. |
| `AccountKey`    | string | Clé d'accès au compte de stockage utilisée pour accéder au conteneur. Pour omettre `AccountKey` de la configuration, utilisez les [identités de charge de travail ou identités managées Azure](#azure-workload-and-managed-identities). |
| `ContainerName` | string | Nom du [conteneur de stockage](https://learn.microsoft.com/en-us/azure/storage/blobs/storage-blobs-introduction#containers) dans lequel enregistrer les données de cache. |
| `StorageDomain` | string | Nom de domaine [utilisé pour desservir les points de terminaison de stockage Azure](https://learn.microsoft.com/en-us/azure/china/resources-developer-guide#check-endpoints-in-azure) (optionnel). La valeur par défaut est `blob.core.windows.net`. |

Exemple :

```toml
[runners.cache]
  Type = "azure"
  Path = "path/to/prefix"
  Shared = false
  [runners.cache.azure]
    AccountName = "<AZURE STORAGE ACCOUNT NAME>"
    AccountKey = "<AZURE STORAGE ACCOUNT KEY>"
    ContainerName = "runners-cache"
    StorageDomain = "blob.core.windows.net"
```

#### Identités de charge de travail et identités managées Azure {#azure-workload-and-managed-identities}

{{< history >}}

- [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27303) dans GitLab Runner v17.5.0.

{{< /history >}}

Pour utiliser les identités de charge de travail ou identités managées Azure, omettez `AccountKey` de la configuration. Lorsque `AccountKey` est vide, le runner tente de :

1. Obtenir des identifiants temporaires en utilisant [`DefaultAzureCredential`](https://github.com/Azure/azure-sdk-for-go/blob/main/sdk/azidentity/README.md#defaultazurecredential).
1. Obtenir une [clé de délégation utilisateur](https://learn.microsoft.com/en-us/rest/api/storageservices/get-user-delegation-key).
1. Générer un jeton SAS avec cette clé pour accéder à un blob de compte de stockage.

Assurez-vous que l'instance dispose du rôle `Storage Blob Data Contributor` qui lui est affecté. Si l'instance n'a pas accès pour effectuer les actions ci-dessus, GitLab Runner signale une erreur `AuthorizationPermissionMismatch`.

Pour utiliser les identités de charge de travail Azure, ajoutez le `service_account` associé à l'identité et le label de pod `azure.workload.identity/use` dans la section `runner.kubernetes`. Par exemple, si `service_account` est `gitlab-runner` :

```toml
  [runners.kubernetes]
    service_account = "gitlab-runner"
    [runners.kubernetes.pod_labels]
      "azure.workload.identity/use" = "true"
```

Assurez-vous que `service_account` dispose de l'annotation `azure.workload.identity/client-id` qui lui est associée :

```yaml
serviceAccount:
  annotations:
    azure.workload.identity/client-id: <YOUR CLIENT ID HERE>
```

Pour GitLab 17.7 et versions ultérieures, cette configuration est suffisante pour configurer les identités de charge de travail.

Cependant, pour GitLab Runner 17.5 et 17.6, vous devez également configurer le gestionnaire de runner avec :

- Le label de pod `azure.workload.identity/use`
- Un compte de service à utiliser avec l'identité de charge de travail

Par exemple, avec le chart Helm de GitLab Runner :

```yaml
serviceAccount:
  name: "gitlab-runner"
podLabels:
  azure.workload.identity/use: "true"
```

Le label est nécessaire car les identifiants sont récupérés depuis différentes sources. Pour les téléchargements de cache, les identifiants sont récupérés depuis le gestionnaire de runner. Pour les chargements de cache, les identifiants sont récupérés depuis le pod qui exécute l'[image d'aide](#helper-image).

Pour plus de détails, voir le [ticket 38330](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38330).

## La section `[runners.kubernetes]` {#the-runnerskubernetes-section}

Le tableau suivant répertorie les paramètres de configuration disponibles pour l'exécuteur Kubernetes. Pour plus de paramètres, consultez la [documentation relative à l'exécuteur Kubernetes](../executors/kubernetes/_index.md).

| Paramètre                    | Type    | Description |
|------------------------------|---------|-------------|
| `host`                       | string  | Facultatif. URL de l'hôte Kubernetes. Si non spécifié, le runner tente de la détecter automatiquement. |
| `cert_file`                  | string  | Facultatif. Certificat d'authentification Kubernetes. |
| `key_file`                   | string  | Facultatif. Clé privée d'authentification Kubernetes. |
| `ca_file`                    | string  | Facultatif. Certificat CA d'authentification Kubernetes. |
| `image`                      | string  | Image de conteneur par défaut à utiliser pour les jobs lorsqu'aucune image n'est spécifiée. |
| `allowed_images`             | array   | Liste de caractères génériques des images de conteneur autorisées dans `.gitlab-ci.yml`. Si absent, toutes les images sont autorisées (équivalent à `["*/*:*"]`). À utiliser avec les exécuteurs [Docker](../executors/docker.md#restrict-docker-images-and-services) ou [Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services). |
| `allowed_services`           | array   | Liste de caractères génériques des services autorisés dans `.gitlab-ci.yml`. Si absent, toutes les images sont autorisées (équivalent à `["*/*:*"]`). À utiliser avec les exécuteurs [Docker](../executors/docker.md#restrict-docker-images-and-services) ou [Kubernetes](../executors/kubernetes/_index.md#restrict-docker-images-and-services). |
| `namespace`                  | string  | Espace de nommage dans lequel exécuter les jobs Kubernetes. |
| `privileged`                 | booléen | Exécuter tous les conteneurs avec l'indicateur privilégié activé. |
| `allow_privilege_escalation` | booléen | Facultatif. Exécute tous les conteneurs avec l'indicateur `allowPrivilegeEscalation` activé. |
| `node_selector`              | table   | Une `table` de paires `key=value` de `string=string`. Limite la création de pods aux nœuds Kubernetes correspondant à toutes les paires `key=value`. |
| `image_pull_secrets`         | array   | Un tableau d'éléments contenant les noms de secrets Kubernetes `docker-registry` utilisés pour authentifier l'extraction d'images de conteneur depuis des registres privés. |
| `logs_base_dir`              | string  | Répertoire de base à ajouter en préfixe au chemin généré pour stocker les journaux de build. [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37760) dans GitLab Runner 17.2. |
| `scripts_base_dir`           | string  | Répertoire de base à ajouter en préfixe au chemin généré pour stocker les scripts de build. [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37760) dans GitLab Runner 17.2. |
| `service_account`            | string  | Compte de service par défaut que les pods de job/exécuteur utilisent pour communiquer avec l'API Kubernetes. |

Exemple :

```toml
[runners.kubernetes]
  host = "https://45.67.34.123:4892"
  cert_file = "/etc/ssl/kubernetes/api.crt"
  key_file = "/etc/ssl/kubernetes/api.key"
  ca_file = "/etc/ssl/kubernetes/ca.crt"
  image = "golang:1.8"
  privileged = true
  allow_privilege_escalation = true
  image_pull_secrets = ["docker-registry-credentials", "optional-additional-credentials"]
  allowed_images = ["ruby:*", "python:*", "php:*"]
  allowed_services = ["postgres:9.4", "postgres:latest"]
  logs_base_dir = "/tmp"
  scripts_base_dir = "/tmp"
  [runners.kubernetes.node_selector]
    gitlab = "true"
```

## Image d'aide {#helper-image}

Lorsque vous utilisez les exécuteurs `docker`, `docker+machine` ou `kubernetes`, GitLab Runner utilise un conteneur spécifique pour gérer les opérations Git, les artefacts et le cache. Ce conteneur est créé à partir d'une image nommée `helper image`.

L'image d'aide est disponible pour les architectures amd64, arm, arm64, s390x, ppc64le et riscv64. Elle contient un binaire `gitlab-runner-helper`, qui est une compilation spéciale du binaire GitLab Runner. Elle ne contient qu'un sous-ensemble de commandes disponibles, ainsi que Git, Git LFS et le magasin de certificats SSL.

L'image d'aide propose quelques variantes : `alpine`, `alpine3.21`, `alpine-latest`, `ubi-fips` et `ubuntu`. L'image `alpine` est celle par défaut en raison de sa faible empreinte. L'utilisation de `helper_image_flavor = "ubuntu"` sélectionne la variante `ubuntu` de l'image d'aide.

Dans GitLab Runner 16.1 à 17.1, la variante `alpine` est un alias pour `alpine3.18`. Dans GitLab Runner 17.2 à 17.6, c'est un alias pour `alpine3.19`. Dans GitLab Runner 17.7 et versions ultérieures, c'est un alias pour `alpine3.21`. Dans GitLab Runner 18.4 et versions ultérieures, c'est un alias pour `alpine-latest`.

La variante `alpine-latest` utilise `alpine:latest` comme image de base et incrémentera naturellement les versions à mesure que de nouvelles versions en amont sont publiées.

Lorsque GitLab Runner est installé depuis les packages `DEB` ou `RPM`, les images pour les architectures prises en charge sont installées sur l'hôte. Si Docker Engine ne trouve pas la version d'image spécifiée, le runner la télécharge automatiquement avant d'exécuter le job. Les exécuteurs `docker` et `docker+machine` fonctionnent de cette façon.

Pour les variantes `alpine`, seule l'image de la variante par défaut `alpine` est incluse dans le package. Toutes les autres variantes sont téléchargées depuis le registre de conteneurs.

L'exécuteur `kubernetes` et les installations manuelles de GitLab Runner fonctionnent différemment.

- Pour les installations manuelles, le binaire `gitlab-runner-helper` n'est pas inclus.
- Pour l'exécuteur `kubernetes`, l'API Kubernetes ne permet pas de charger l'image `gitlab-runner-helper` depuis une archive locale.

Dans les deux cas, GitLab Runner [télécharge l'image d'aide](#helper-image-registry). La révision et l'architecture de GitLab Runner définissent le tag à télécharger.

### Configuration de l'image d'aide pour Kubernetes sur Arm {#helper-image-configuration-for-kubernetes-on-arm}

Par défaut, la bonne [image d'aide pour votre architecture](../executors/kubernetes/_index.md#operating-system-architecture-and-windows-kernel-version) est sélectionnée. Si vous devez définir un chemin `helper_image` personnalisé pour utiliser l'image d'aide `arm64` sur des clusters Kubernetes `arm64`, définissez les valeurs suivantes dans votre [fichier de configuration](../executors/kubernetes/_index.md#configuration-settings) :

```toml
[runners.kubernetes]
  helper_image = "my.registry.local/gitlab/gitlab-runner-helper:arm64-v${CI_RUNNER_VERSION}"
```

### Images de runner utilisant une ancienne version d'Alpine Linux {#runner-images-that-use-an-old-version-of-alpine-linux}

{{< history >}}

- [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3122) dans GitLab Runner 14.5.

{{< /history >}}

Les images sont construites avec plusieurs versions d'Alpine Linux. Vous pouvez utiliser une version plus récente d'Alpine, mais également utiliser des versions plus anciennes en même temps.

Pour l'image d'aide, modifiez `helper_image_flavor` ou consultez la section [Image d'aide](#helper-image).

Pour l'image GitLab Runner, suivez la même logique, où `alpine`, `alpine3.19`, `alpine3.21` ou `alpine-latest` est utilisé comme préfixe dans l'image, avant la version :

```shell
docker pull gitlab/gitlab-runner:alpine3.19-v16.1.0
```

### Images Alpine `pwsh` {#alpine-pwsh-images}

À partir de GitLab Runner 16.1 et versions ultérieures, toutes les images d'aide `alpine` disposent d'une variante `pwsh`. La seule exception est `alpine-latest`, car les [images Docker `powershell`](https://learn.microsoft.com/en-us/powershell/scripting/install/powershell-in-docker?view=powershell-7.4) sur lesquelles les images d'aide de GitLab Runner sont basées ne prennent pas en charge `alpine:latest`.

Exemple :

```shell
docker pull registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:alpine3.21-x86_64-v17.7.0-pwsh
```

### Registre de conteneurs d'images d'aide {#helper-image-registry}

Dans GitLab 15.0 et versions antérieures, vous configurez les images d'aide pour utiliser des images depuis Docker Hub.

Dans GitLab 15.1 et versions ultérieures, l'image d'aide est extraite du registre de conteneurs GitLab sur GitLab.com à l'adresse `registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-v${CI_RUNNER_VERSION}`. Les instances GitLab Self-Managed extraient également l'image d'aide depuis le registre de conteneurs GitLab sur GitLab.com par défaut. Pour vérifier le statut du registre de conteneurs GitLab sur GitLab.com, consultez [GitLab System Status](https://status.gitlab.com/).

### Remplacer l'image d'aide {#override-the-helper-image}

Dans certains cas, vous pourriez avoir besoin de remplacer l'image d'aide pour les raisons suivantes :

1. **Accélérer l'exécution des jobs** : Dans les environnements avec une connexion internet plus lente, le téléchargement de la même image plusieurs fois peut augmenter le temps nécessaire pour exécuter un job. Le téléchargement de l'image d'aide depuis un registre de conteneurs local, où la copie exacte de `registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:XYZ` est stockée, peut accélérer les choses.
1. **Problèmes de sécurité** : Vous ne souhaitez peut-être pas télécharger des dépendances externes qui n'ont pas été vérifiées auparavant. Il peut exister une règle métier imposant d'utiliser uniquement des dépendances qui ont été examinées et stockées dans des dépôts locaux.
1. **Environnements de compilation sans accès à Internet** : Si vous avez des [clusters Kubernetes installés dans un environnement hors ligne](../install/operator.md#install-gitlab-runner-operator-on-kubernetes-clusters-in-offline-environments), vous pouvez utiliser un registre de conteneurs local ou un dépôt de packages pour extraire les images utilisées dans les jobs CI/CD.
1. **Logiciels supplémentaires** : Vous pouvez souhaiter installer des logiciels supplémentaires dans l'image d'aide, comme `openssh` pour prendre en charge les sous-modules accessibles avec `git+ssh` au lieu de `git+http`.

Dans ces cas, vous pouvez configurer une image personnalisée en utilisant le champ de configuration `helper_image`, qui est disponible pour les exécuteurs `docker`, `docker+machine` et `kubernetes` :

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:tag"
```

La version de l'image d'aide doit être considérée comme strictement couplée à la version de GitLab Runner. L'une des principales raisons de fournir ces images est que GitLab Runner utilise le binaire `gitlab-runner-helper`. Ce binaire est compilé à partir d'une partie du code source de GitLab Runner. Ce binaire utilise une API interne qui doit être identique dans les deux binaires.

Par défaut, GitLab Runner référence une image `registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:XYZ`, où `XYZ` est basé sur l'architecture et la révision Git de GitLab Runner. Vous pouvez définir la version de l'image en utilisant l'une des [variables de version](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/common/version.go#L60-61) :

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:x86_64-v${CI_RUNNER_VERSION}"
```

Avec cette configuration, GitLab Runner demande à l'exécuteur d'utiliser l'image dans la version `x86_64-v${CI_RUNNER_VERSION}`, basée sur ses données de compilation. Après la mise à jour de GitLab Runner vers une nouvelle version, GitLab Runner tente de télécharger l'image appropriée. L'image doit être chargée dans le registre de conteneurs avant de mettre à niveau GitLab Runner, sinon les jobs commenceront à échouer avec une erreur « No such image ».

L'image d'aide est taguée par `$CI_RUNNER_VERSION` en plus de `$CI_RUNNER_REVISION`. Les deux tags sont valides et pointent vers la même image.

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    helper_image = "my.registry.local/gitlab/gitlab-runner-helper:x86_64-v${CI_RUNNER_VERSION}"
```

#### Lors de l'utilisation de PowerShell Core {#when-using-powershell-core}

Une version supplémentaire de l'image d'aide pour Linux, qui contient PowerShell Core, est publiée avec le tag `registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:XYZ-pwsh`.

## La section `[runners.custom_build_dir]` {#the-runnerscustom_build_dir-section}

{{< history >}}

- [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1267) dans GitLab Runner 11.10.

{{< /history >}}

Cette section définit les paramètres des [répertoires de build personnalisés](https://docs.gitlab.com/ci/runners/configure_runners/#custom-build-directories).

Cette fonctionnalité, si elle n'est pas configurée explicitement, est activée par défaut pour les exécuteurs `kubernetes`, `docker`, `docker+machine`, `docker autoscaler` et `instance`. Pour tous les autres exécuteurs, elle est désactivée par défaut.

Cette fonctionnalité nécessite que `GIT_CLONE_PATH` se trouve dans un chemin défini dans `runners.builds_dir`. Pour utiliser le `builds_dir`, utilisez la variable `$CI_BUILDS_DIR`.

Par défaut, cette fonctionnalité est activée uniquement pour les exécuteurs `docker` et `kubernetes`, car ils offrent un bon moyen de séparer les ressources. Cette fonctionnalité peut être explicitement activée pour n'importe quel exécuteur, mais soyez prudent lorsque vous l'utilisez avec des exécuteurs qui partagent `builds_dir` et ont `concurrent > 1`.

| Paramètre | Type    | Description |
|-----------|---------|-------------|
| `enabled` | booléen | Autoriser l'utilisateur à définir un répertoire de build personnalisé pour un job. |

Exemple :

```toml
[runners.custom_build_dir]
  enabled = true
```

### Répertoire de build par défaut {#default-build-directory}

GitLab Runner clone le dépôt vers un chemin qui existe sous un chemin de base mieux connu sous le nom de _Répertoire de builds_. L'emplacement par défaut de ce répertoire de base dépend de l'exécuteur. Pour :

- Les exécuteurs [Kubernetes](../executors/kubernetes/_index.md), [Docker](../executors/docker.md) et [Docker Machine](../executors/docker_machine.md), c'est `/builds` à l'intérieur du conteneur.
- [Instance](../executors/instance.md), c'est `~/builds` dans le répertoire d'accueil de l'utilisateur configuré pour gérer la connexion SSH ou WinRM vers la machine cible.
- [Docker Autoscaler](../executors/docker_autoscaler.md), c'est `/builds` à l'intérieur du conteneur.
- L'exécuteur [Shell](../executors/shell.md), c'est `$PWD/builds`.
- Les exécuteurs [SSH](../executors/ssh.md), [VirtualBox](../executors/virtualbox.md) et [Parallels](../executors/parallels.md), c'est `~/builds` dans le répertoire d'accueil de l'utilisateur configuré pour gérer la connexion SSH vers la machine cible.
- Les exécuteurs [personnalisés](../executors/custom.md), aucune valeur par défaut n'est fournie et le répertoire doit être explicitement configuré, sinon le job échoue.

Le _Répertoire de builds_ utilisé peut être défini explicitement par l'utilisateur avec le paramètre [`builds_dir`](#the-runners-section).

> [!note]
> Vous pouvez également spécifier [`GIT_CLONE_PATH`](https://docs.gitlab.com/ci/runners/configure_runners/#custom-build-directories) si vous souhaitez cloner vers un répertoire personnalisé, et la directive ci-dessous ne s'applique pas.

GitLab Runner utilise le _Répertoire de builds_ pour tous les jobs qu'il exécute, mais les imbrique selon un modèle spécifique `{builds_dir}/$RUNNER_TOKEN_KEY/$CONCURRENT_PROJECT_ID/$NAMESPACE/$PROJECT_NAME`. Par exemple : `/builds/2mn-ncv-/0/user/playground`.

GitLab Runner ne vous empêche pas de stocker des éléments dans le _Répertoire de builds_. Par exemple, vous pouvez stocker des outils dans `/builds/tools` qui peuvent être utilisés lors de l'exécution CI. Nous déconseillons **FORTEMENT** cela, vous ne devriez jamais rien stocker dans le _Répertoire de builds_. GitLab Runner doit avoir un contrôle total sur celui-ci et ne garantit pas la stabilité dans de tels cas. Si vous avez des dépendances requises pour votre CI, vous devez les installer ailleurs.

## Nettoyage de la configuration Git {#cleaning-git-configuration}

{{< history >}}

- [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5438) dans GitLab Runner 17.10.

{{< /history >}}

Au début et à la fin de chaque build, GitLab Runner supprime les fichiers suivants du dépôt et de ses sous-modules :

- Fichiers de verrouillage Git (`{index,shallow,HEAD,config}.lock`)
- Hooks post-checkout (`hooks/post-checkout`)

Si vous activez `clean_git_config`, les fichiers ou répertoires supplémentaires suivants sont supprimés du dépôt, de ses sous-modules et du répertoire de modèle Git :

- Fichier `.git/config`
- Répertoire `.git/hooks`

Ce nettoyage empêche la mise en cache entre les jobs de configurations Git personnalisées, éphémères ou potentiellement malveillantes.

Avant GitLab Runner 17.10, les nettoyages se comportaient différemment :

- Le nettoyage des fichiers de verrouillage Git et des hooks post-checkout ne s'effectuait qu'au début d'un job et non à la fin.
- Les autres configurations Git (maintenant contrôlées par `clean_git_config`) n'étaient pas supprimées sauf si `FF_ENABLE_JOB_CLEANUP` était défini. Lorsque vous définissez ce flag, seul le fichier `.git/config` du dépôt principal était supprimé, mais pas les configurations des sous-modules.

Le paramètre `clean_git_config` a pour valeur par défaut `true`. Mais, il prend par défaut la valeur `false` dans les cas suivants :

- L'exécuteur [Shell](../executors/shell.md) est utilisé.
- La [stratégie Git](https://docs.gitlab.com/ci/runners/configure_runners/#git-strategy) est définie sur `none`.

La configuration explicite de `clean_git_config` est prioritaire sur le paramètre par défaut.

## La section `[runners.referees]` {#the-runnersreferees-section}

Utilisez les arbitres de GitLab Runner pour transmettre des données supplémentaires de surveillance des jobs à GitLab. Les arbitres sont des workers du gestionnaire de runner qui interrogent et collectent des données supplémentaires liées à un job. Les résultats sont chargés dans GitLab en tant qu'artefacts de job.

### Utiliser l'arbitre Metrics Runner {#use-the-metrics-runner-referee}

Si la machine ou le conteneur exécutant le job expose des métriques [Prometheus](https://prometheus.io), GitLab Runner peut interroger le serveur Prometheus pendant toute la durée du job. Une fois les métriques reçues, elles sont chargées en tant qu'artefact de job pouvant être utilisé pour une analyse ultérieure.

Seul l'exécuteur [`docker-machine`](../executors/docker_machine.md) prend en charge l'arbitre.

### Configurer l'arbitre Metrics Runner pour GitLab Runner {#configure-the-metrics-runner-referee-for-gitlab-runner}

Définissez `[runner.referees]` et `[runner.referees.metrics]` dans votre fichier `config.toml` dans une section `[[runner]]` et ajoutez les champs suivants :

| Paramètre              | Description |
|----------------------|-------------|
| `prometheus_address` | Le serveur qui collecte les métriques des instances GitLab Runner. Il doit être accessible par le gestionnaire de runner lorsque le job se termine. |
| `query_interval`     | La fréquence à laquelle l'instance Prometheus associée à un job est interrogée pour les données de séries temporelles, définie sous forme d'intervalle (en secondes). |
| `queries`            | Un tableau de requêtes [PromQL](https://prometheus.io/docs/prometheus/latest/querying/basics/) exécutées pour chaque intervalle. |

Voici un exemple de configuration complète pour les métriques `node_exporter` :

```toml
[[runners]]
  [runners.referees]
    [runners.referees.metrics]
      prometheus_address = "http://localhost:9090"
      query_interval = 10
      metric_queries = [
        "arp_entries:rate(node_arp_entries{{selector}}[{interval}])",
        "context_switches:rate(node_context_switches_total{{selector}}[{interval}])",
        "cpu_seconds:rate(node_cpu_seconds_total{{selector}}[{interval}])",
        "disk_read_bytes:rate(node_disk_read_bytes_total{{selector}}[{interval}])",
        "disk_written_bytes:rate(node_disk_written_bytes_total{{selector}}[{interval}])",
        "memory_bytes:rate(node_memory_MemTotal_bytes{{selector}}[{interval}])",
        "memory_swap_bytes:rate(node_memory_SwapTotal_bytes{{selector}}[{interval}])",
        "network_tcp_active_opens:rate(node_netstat_Tcp_ActiveOpens{{selector}}[{interval}])",
        "network_tcp_passive_opens:rate(node_netstat_Tcp_PassiveOpens{{selector}}[{interval}])",
        "network_receive_bytes:rate(node_network_receive_bytes_total{{selector}}[{interval}])",
        "network_receive_drops:rate(node_network_receive_drop_total{{selector}}[{interval}])",
        "network_receive_errors:rate(node_network_receive_errs_total{{selector}}[{interval}])",
        "network_receive_packets:rate(node_network_receive_packets_total{{selector}}[{interval}])",
        "network_transmit_bytes:rate(node_network_transmit_bytes_total{{selector}}[{interval}])",
        "network_transmit_drops:rate(node_network_transmit_drop_total{{selector}}[{interval}])",
        "network_transmit_errors:rate(node_network_transmit_errs_total{{selector}}[{interval}])",
        "network_transmit_packets:rate(node_network_transmit_packets_total{{selector}}[{interval}])"
      ]
```

Les requêtes de métriques sont au format `canonical_name:query_string`. La chaîne de requête prend en charge deux variables qui sont remplacées lors de l'exécution :

| Paramètre      | Description |
|--------------|-------------|
| `{selector}` | Remplacé par une paire `label_name=label_value` qui sélectionne les métriques générées dans Prometheus par une instance GitLab Runner spécifique. |
| `{interval}` | Remplacé par le paramètre `query_interval` de la configuration `[runners.referees.metrics]` pour cet arbitre. |

Par exemple, un environnement GitLab Runner partagé utilisant l'exécuteur `docker-machine` aurait un `{selector}` similaire à `node=shared-runner-123`.
