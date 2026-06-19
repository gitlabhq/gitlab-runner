---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "L'exécuteur personnalisé (Custom executor)"
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!note]
> Cet exécuteur est en mode de maintenance. Il reçoit des mises à jour de sécurité critiques, mais aucune nouvelle fonctionnalité n'est prévue. Pour les nouveaux projets, envisagez d'utiliser l'un des [exécuteurs activement développés](_index.md#selecting-the-executor).

Vous pouvez utiliser l'exécuteur personnalisé (Custom executor) pour spécifier vos propres environnements d'exécution. Lorsque GitLab Runner ne prend pas en charge nativement un exécuteur (par exemple, `LXD` ou `Libvirt`), vous pouvez configurer GitLab Runner pour utiliser des exécutables personnalisés afin de provisionner, d'exécuter et de nettoyer votre environnement.

Les scripts que vous configurez pour l'exécuteur personnalisé sont appelés `Drivers`. Par exemple, vous pouvez créer un [pilote `LXD`](custom_examples/lxd.md) ou un [pilote `Libvirt`](custom_examples/libvirt.md).

## Configuration {#configuration}

Vous pouvez choisir parmi quelques clés de configuration. Certaines d'entre elles sont facultatives.

Vous trouverez ci-dessous un exemple de configuration pour l'exécuteur personnalisé (Custom executor) utilisant toutes les clés de configuration disponibles :

```toml
[[runners]]
  name = "custom"
  url = "https://gitlab.com"
  token = "TOKEN"
  executor = "custom"
  builds_dir = "/builds"
  cache_dir = "/cache"
  shell = "bash"
  [runners.custom]
    config_exec = "/path/to/config.sh"
    config_args = [ "SomeArg" ]
    config_exec_timeout = 200

    prepare_exec = "/path/to/script.sh"
    prepare_args = [ "SomeArg" ]
    prepare_exec_timeout = 200

    run_exec = "/path/to/binary"
    run_args = [ "SomeArg" ]

    cleanup_exec = "/path/to/executable"
    cleanup_args = [ "SomeArg" ]
    cleanup_exec_timeout = 200

    graceful_kill_timeout = 200
    force_kill_timeout = 200
```

Pour les définitions des champs et lesquels sont obligatoires, consultez la configuration de la [section `[runners.custom]`](../configuration/advanced-configuration.md#the-runnerscustom-section).

De plus, `builds_dir` et `cache_dir` à l'intérieur de [`[[runners]]`](../configuration/advanced-configuration.md#the-runners-section) sont des champs obligatoires.

## Logiciels prérequis pour l'exécution d'un job {#prerequisite-software-for-running-a-job}

L'utilisateur doit configurer l'environnement, notamment les éléments suivants qui doivent être présents dans `PATH` :

- [Git](https://git-scm.com/download) et [Git LFS](https://git-lfs.com/) : consultez les [prérequis communs](_index.md#git-requirements-for-non-docker-executors).
- [GitLab Runner](../install/_index.md) :  Utilisé pour télécharger/mettre à jour les artefacts et le cache.

## Étapes {#stages}

L'exécuteur personnalisé (Custom executor) fournit les étapes pour configurer les détails du job, préparer et nettoyer l'environnement, et y exécuter le script du job. Chaque étape est responsable de choses spécifiques et implique différents points à garder à l'esprit.

Chaque étape exécutée par l'exécuteur personnalisé (Custom executor) est exécutée au moment où un exécuteur GitLab Runner intégré les exécuterait.

Chaque étape exécutée a accès à des variables d'environnement spécifiques qui fournissent des informations sur le job en cours d'exécution. Toutes les étapes ont les variables d'environnement suivantes disponibles :

- [Variables CI/CD](https://docs.gitlab.com/ci/variables/) standard, y compris les [variables prédéfinies](https://docs.gitlab.com/ci/variables/predefined_variables/).
- Toutes les variables d'environnement fournies par le système hôte du runner de l'exécuteur personnalisé (Custom executor).
- Tous les services et leurs [paramètres disponibles](https://docs.gitlab.com/ci/services/#available-settings-for-services). Exposées au format JSON sous la forme `CUSTOM_ENV_CI_JOB_SERVICES`.

Les variables CI/CD et les variables prédéfinies sont toutes préfixées par `CUSTOM_ENV_` pour éviter les conflits avec les variables d'environnement système. Par exemple, `CI_BUILDS_DIR` est disponible sous la forme `CUSTOM_ENV_CI_BUILDS_DIR`.

Les étapes s'exécutent dans l'ordre suivant :

1. `config_exec`
1. `prepare_exec`
1. `run_exec`
1. `cleanup_exec`

### Services {#services}

[Les services](https://docs.gitlab.com/ci/services/) sont exposés sous forme de tableau JSON sous la forme `CUSTOM_ENV_CI_JOB_SERVICES`.

Exemple :

```yaml
custom:
  script:
    - echo $CUSTOM_ENV_CI_JOB_SERVICES
  services:
    - redis:latest
    - name: my-postgres:9.4
      alias: pg
      entrypoint: ["path", "to", "entrypoint"]
      command: ["path", "to", "cmd"]
      variables:
        POSTGRES_PASSWORD: secret
        POSTGRES_DB: mydb
```

L'exemple ci-dessus définit la variable d'environnement `CUSTOM_ENV_CI_JOB_SERVICES` avec la valeur suivante :

```json
[{"name":"redis:latest","alias":"","entrypoint":null,"command":null},{"name":"my-postgres:9.4","alias":"pg","entrypoint":["path","to","entrypoint"],"command":["path","to","cmd"],"variables":{"POSTGRES_DB":"mydb","POSTGRES_PASSWORD":"secret"}}]
```

Chaque objet service dans le tableau JSON possède les champs suivants :

| Champ        | Type          | Description                                                                              |
|--------------|---------------|------------------------------------------------------------------------------------------|
| `name`       | string        | Nom de l'image du service.                                                                      |
| `alias`      | string        | Premier alias défini pour le service. Chaîne vide si aucun.                               |
| `entrypoint` | array ou null | Remplacement du point d'entrée du conteneur. `null` si non défini.                                        |
| `command`    | array ou null | Remplacement de la commande du conteneur. `null` si non définie.                                           |
| `variables`  | object        | Mappage clé-valeur des variables définies pour le service. Omis si aucune variable n'est définie. |

### Config {#config}

L'étape Config est exécutée par `config_exec`.

Il peut arriver que vous souhaitiez définir certains paramètres pendant l'exécution. Par exemple, définir un répertoire de build en fonction de l'identifiant du projet. `config_exec` lit depuis STDOUT et attend une chaîne JSON valide avec des clés spécifiques.

Par exemple :

```shell
#!/usr/bin/env bash

cat << EOS
{
  "builds_dir": "/builds/${CUSTOM_ENV_CI_CONCURRENT_PROJECT_ID}/${CUSTOM_ENV_CI_PROJECT_PATH_SLUG}",
  "cache_dir": "/cache/${CUSTOM_ENV_CI_CONCURRENT_PROJECT_ID}/${CUSTOM_ENV_CI_PROJECT_PATH_SLUG}",
  "builds_dir_is_shared": true,
  "hostname": "custom-hostname",
  "driver": {
    "name": "test driver",
    "version": "v0.0.1"
  },
  "job_env" : {
    "CUSTOM_ENVIRONMENT": "example"
  },
  "shell": "bash"
}
EOS
```

Toutes les clés supplémentaires à l'intérieur de la chaîne JSON sont ignorées. Si ce n'est pas une chaîne JSON valide, l'étape échoue et effectue deux nouvelles tentatives.

| Paramètre              | Type    | Obligatoire | Vide autorisé  | Description |
|------------------------|---------|----------|----------------|-------------|
| `builds_dir`           | string  | ✗        | ✗              | Le répertoire de base où est créé le répertoire de travail du job. |
| `cache_dir`            | string  | ✗        | ✗              | Le répertoire de base où le cache local est stocké. |
| `builds_dir_is_shared` | boolean | ✗        | non applicable | Définit si l'environnement est partagé entre les jobs simultanés ou non. |
| `hostname`             | string  | ✗        | ✓              | Le nom d'hôte à associer aux « métadonnées » du job stockées par le runner. Si non défini, le nom d'hôte n'est pas défini. |
| `driver.name`          | string  | ✗        | ✓              | Le nom défini par l'utilisateur pour le pilote. Affiché avec la ligne `Using custom executor...`. Si non défini, aucune information sur le pilote n'est affichée. |
| `driver.version`       | string  | ✗        | ✓              | La version définie par l'utilisateur pour le pilote. Affiché avec la ligne `Using custom executor...`. Si non définie, seule l'information sur le nom est affichée. |
| `job_env`              | object  | ✗        | ✓              | Paires nom-valeur disponibles via les variables d'environnement pour toutes les étapes suivantes de l'exécution du job. Elles sont disponibles pour le pilote, pas pour le job. Pour plus de détails, consultez [utilisation de `job_env`](#job_env-usage). |
| `shell`                | string  | ✗        | ✓              | Le shell utilisé pour exécuter les scripts du job. |

Le `STDERR` de l'exécutable s'affiche dans le job log.

Vous pouvez configurer [`config_exec_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section) pour définir un délai d'attente maximal avant que GitLab Runner ne termine le processus en attendant le retour de la chaîne JSON.

Si vous définissez des [`config_args`](../configuration/advanced-configuration.md#the-runnerscustom-section), ils sont ajoutés à l'exécutable `config_exec` dans le même ordre que vous les définissez. Par exemple, avec ce contenu `config.toml` :

```toml
...
[runners.custom]
  ...
  config_exec = "/path/to/config"
  config_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab Runner l'exécuterait sous la forme `/path/to/config Arg1 Arg2`.

#### Utilisation de `job_env` {#job_env-usage}

L'objectif principal de la configuration `job_env` est de transmettre des variables **au contexte des appels de driver d’un exécuteur personnalisé** pour les étapes suivantes de l'exécution du job.

Par exemple, un pilote dont la connexion avec l'environnement d'exécution du job nécessite la préparation de certaines informations d'identification. Cette opération est coûteuse. Le pilote doit demander des informations d'identification SSH temporaires à un fournisseur local avant de se connecter à l'environnement.

Avec le flux d'exécution de l'exécuteur personnalisé (Custom Executor), chaque [étape](#stages) d'exécution de job (`prepare`, plusieurs appels `run`, et `cleanup`) s'exécute comme des exécutions séparées avec leur propre contexte. Pour notre exemple de résolution des informations d'identification, la connexion au fournisseur d'informations d'identification doit être effectuée à chaque fois.

Si cette opération est coûteuse, effectuez-la une seule fois pour l'ensemble de l'exécution du job, puis réutilisez les informations d'identification pour toutes les étapes d'exécution du job. Le `job_env` peut vous aider ici. Avec cela, vous pouvez vous connecter au fournisseur une seule fois, lors de l'appel `config_exec`, puis transmettre les informations d'identification reçues avec le `job_env`. Ensuite, ils sont ajoutés à la liste des variables que les appels de l'exécuteur personnalisé pour [`prepare_exec`](#prepare) , [`run_exec`](#run) et [`cleanup_exec`](#cleanup) reçoivent. Ainsi, le pilote, au lieu de se connecter à chaque fois au fournisseur d'informations d'identification, peut simplement lire les variables et utiliser les informations d'identification présentes.

La chose importante à comprendre est que **les variables ne sont pas automatiquement disponibles pour le job lui‑même**. Cela dépend entièrement de la façon dont le pilote de l'exécuteur personnalisé (Custom Executor Driver) est implémenté, et dans de nombreux cas, il n'est pas présent.

Pour savoir comment transmettre un ensemble de variables à chaque job exécuté par un runner particulier en utilisant le paramètre `job_env`, consultez [le paramètre `environment` de `[[runners]]`](../configuration/advanced-configuration.md#the-runners-section).

Si les variables sont dynamiques avec des valeurs susceptibles de changer entre les jobs, assurez-vous que l'implémentation de votre pilote ajoute les variables transmises par `job_env` à l'appel d'exécution.

### Prepare {#prepare}

L'étape Prepare est exécutée par `prepare_exec`.

À ce stade, GitLab Runner sait tout sur le job (où et comment il s'exécute). Il ne reste plus qu'à configurer l'environnement pour que le job puisse s'exécuter. GitLab Runner exécute l'exécutable spécifié dans `prepare_exec`.

Cette action est responsable de la configuration de l'environnement (par exemple, la création de la machine virtuelle ou du conteneur, des services ou de tout autre élément). Une fois cela fait, nous nous attendons à ce que l'environnement soit prêt à exécuter le job.

Cette étape n'est exécutée qu'une seule fois, lors d'une exécution de job.

Vous pouvez configurer [`prepare_exec_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section) pour définir un délai d'attente maximal avant que GitLab Runner ne termine le processus en attendant la préparation de l'environnement.

Le `STDOUT` et le `STDERR` retournés par cet exécutable s'affichent dans le job log.

Si vous définissez des [`prepare_exec_args`](../configuration/advanced-configuration.md#the-runnerscustom-section), ils sont ajoutés à l'exécutable `prepare_exec` dans le même ordre que vous les définissez. Par exemple, avec ce contenu `config.toml` :

```toml
...
[runners.custom]
  ...
  prepare_exec = "/path/to/bin"
  prepare_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab Runner l'exécuterait sous la forme `/path/to/bin Arg1 Arg2`.

### Run {#run}

L'étape Run est exécutée par `run_exec`.

Le `STDOUT` et le `STDERR` retournés par cet exécutable s'affichent dans le job log.

Contrairement aux autres étapes, l'étape `run_exec` est exécutée plusieurs fois, car elle est divisée en sous-étapes listées ci-dessous dans l'ordre séquentiel :

1. `prepare_script`
1. `get_sources`
1. `restore_cache`
1. `download_artifacts`
1. `step_*`
1. `build_script`
1. `step_*`
1. `after_script`
1. `archive_cache` OU `archive_cache_on_failure`
1. `upload_artifacts_on_success` OU `upload_artifacts_on_failure`
1. `cleanup_file_variables`

Pour chaque étape mentionnée ci-dessus, l'exécutable `run_exec` est exécuté avec :

- Les variables d'environnement habituelles.
- Deux arguments :
  - Le chemin vers le script que GitLab Runner crée pour que l'exécuteur personnalisé (Custom executor) l'exécute.
  - Nom de l'étape.

Par exemple :

```shell
/path/to/run_exec.sh /path/to/tmp/script1 prepare_executor
/path/to/run_exec.sh /path/to/tmp/script1 prepare_script
/path/to/run_exec.sh /path/to/tmp/script1 get_sources
```

Si vous avez défini `run_args`, ils constituent le premier ensemble d'arguments transmis à l'exécutable `run_exec`, puis GitLab Runner en ajoute d'autres. Par exemple, supposons que nous ayons le `config.toml` suivant :

```toml
...
[runners.custom]
  ...
  run_exec = "/path/to/run_exec.sh"
  run_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab Runner exécute l'exécutable avec les arguments suivants :

```shell
/path/to/run_exec.sh Arg1 Arg2 /path/to/tmp/script1 prepare_executor
/path/to/run_exec.sh Arg1 Arg2 /path/to/tmp/script1 prepare_script
/path/to/run_exec.sh Arg1 Arg2 /path/to/tmp/script1 get_sources
```

Cet exécutable doit être responsable de l'exécution des scripts spécifiés dans le premier argument. Ils contiennent tous les scripts qu'un exécuteur GitLab Runner exécuterait pour cloner, télécharger des artefacts, exécuter des scripts utilisateur et toutes les autres étapes décrites ci-dessous. Les scripts peuvent utiliser les shells suivants :

- Bash
- PowerShell Desktop
- PowerShell Core
- Batch (déprécié)

Nous générons le script en utilisant le shell configuré par `shell` à l'intérieur de [`[[runners]]`](../configuration/advanced-configuration.md#the-runners-section). Si aucun n'est fourni, les valeurs par défaut pour la plateforme du système d'exploitation sont utilisées.

> [!note]
> Assurez-vous que votre configuration `shell` correspond à la version de PowerShell utilisée par votre script `run_exec`. Utilisez `shell = "pwsh"` avec `pwsh.exe` (PowerShell Core) ou `shell = "powershell"` avec `powershell.exe` (PowerShell Desktop).

Le tableau ci-dessous est une explication détaillée de ce que fait chaque script et quel est son objectif principal.

| Nom du script                   | Contenu du script |
|-------------------------------|-----------------|
| `prepare_script`              | Informations de débogage indiquant sur quelle machine le job s'exécute. |
| `get_sources`                 | Prépare la configuration Git et clone/récupère le dépôt. Nous vous suggérons de conserver ceci tel quel car vous bénéficiez de tous les avantages des stratégies Git fournies par GitLab. |
| `restore_cache`               | Extrait le cache si des entrées sont définies. Cela nécessite que le binaire `gitlab-runner` soit disponible dans `$PATH`. |
| `download_artifacts`          | Télécharge les artefacts, si des entrées sont définies. Cela nécessite que le binaire `gitlab-runner` soit disponible dans `$PATH`. |
| `step_*`                      | Généré par GitLab. Un ensemble de scripts à exécuter. Il peut ne jamais être envoyé à l'exécuteur personnalisé. Il peut comporter plusieurs étapes, comme `step_release` et `step_accessibility`. Il peut s'agir d'une fonctionnalité du fichier `.gitlab-ci.yml`. |
| `after_script`                | [`after_script`](https://docs.gitlab.com/ci/yaml/#after_script) défini depuis le job. S'exécute dans un contexte de shell séparé. S'exécute toujours, même si les étapes précédentes échouent, y compris `pre_build_script`. |
| `archive_cache`               | Crée une archive de tout le cache, si des entrées sont définies. Exécuté uniquement lorsque `build_script` a réussi. |
| `archive_cache_on_failure`    | Crée une archive de tout le cache, si des entrées sont définies. Exécuté uniquement lorsque `build_script` échoue. |
| `upload_artifacts_on_success` | Téléverse tous les artefacts définis. Exécuté uniquement lorsque `build_script` a réussi. |
| `upload_artifacts_on_failure` | Téléverse tous les artefacts définis. Exécuté uniquement lorsque `build_script` échoue. |
| `cleanup_file_variables`      | Supprime toutes les variables [basées sur des fichiers](https://docs.gitlab.com/ci/variables/#use-file-type-cicd-variables) du disque. |

### Cleanup {#cleanup}

L'étape Cleanup est exécutée par `cleanup_exec`.

Cette étape finale est exécutée même si l'une des étapes précédentes a échoué. L'objectif principal de cette étape est de nettoyer tous les environnements qui auraient pu être configurés. Par exemple, arrêter des machines virtuelles ou supprimer des conteneurs.

Le résultat de `cleanup_exec` n'affecte pas les statuts des jobs. Par exemple, un job est marqué comme réussi même si les événements suivants se produisent :

- `prepare_exec` et `run_exec` réussissent tous les deux.
- `cleanup_exec` échoue.

Vous pouvez configurer [`cleanup_exec_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section) pour définir un délai d'attente maximal avant que GitLab Runner ne termine le processus en attendant le nettoyage de l'environnement.

Le `STDOUT` de cet exécutable est affiché dans les journaux de GitLab Runner au niveau `DEBUG`. Le `STDERR` est affiché dans les journaux au niveau `WARN`.

Si vous définissez des [`cleanup_exec_args`](../configuration/advanced-configuration.md#the-runnerscustom-section), ils sont ajoutés à l'exécutable `cleanup_exec` dans le même ordre que vous les définissez. Par exemple, avec ce contenu `config.toml` :

```toml
...
[runners.custom]
  ...
  cleanup_exec = "/path/to/bin"
  cleanup_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab Runner l'exécuterait sous la forme `/path/to/bin Arg1 Arg2`.

## Arrêt et suppression des exécutables {#terminating-and-killing-executables}

GitLab Runner tente de terminer gracieusement un exécutable dans l'une des conditions suivantes :

- `config_exec_timeout`, `prepare_exec_timeout` ou `cleanup_exec_timeout` sont atteints.
- Le job [expire](https://docs.gitlab.com/ci/pipelines/settings/#set-a-limit-for-how-long-jobs-can-run).
- Le job est annulé.

Lorsqu'un délai est atteint, un `SIGTERM` est envoyé à l'exécutable, et le compte à rebours pour [`exec_terminate_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section) démarre. L'exécutable doit écouter ce signal pour s'assurer qu'il libère toutes les ressources. Si `exec_terminate_timeout` expire et que le processus est toujours en cours d'exécution, un `SIGKILL` est envoyé pour tuer le processus et [`exec_force_kill_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section) démarre. Si le processus est toujours en cours d'exécution après la fin de `exec_force_kill_timeout`, GitLab Runner abandonne le processus et ne tente plus de l'arrêter ou de le tuer. Si ces deux délais sont atteints pendant `config_exec`, `prepare_exec` ou `run_exec`, le build est marqué comme échoué.

Tout processus enfant engendré par le pilote reçoit également le processus de terminaison gracieuse décrit ci-dessus sur les systèmes basés sur UNIX. Cela est réalisé en définissant le processus principal comme un [groupe de processus](https://man7.org/linux/man-pages/man2/setpgid.2.html) auquel appartiennent tous les processus enfants.

## Gestion des erreurs {#error-handling}

GitLab Runner peut gérer deux types d'erreurs différemment. Ces erreurs ne sont gérées que lorsque l'exécutable à l'intérieur de `config_exec`, `prepare_exec`, `run_exec` et `cleanup_exec` se termine avec ces codes. Si l'utilisateur se termine avec un code de sortie non nul, il doit être propagé comme l'un des codes d'erreur ci-dessous.

Si le script utilisateur se termine avec l'un de ces codes, il doit être propagé vers le code de sortie de l'exécutable.

### Échec de build {#build-failure}

GitLab Runner fournit la variable d'environnement `BUILD_FAILURE_EXIT_CODE` que votre exécutable doit utiliser comme code de sortie pour indiquer l'échec du job. Si l'exécutable se termine avec le code de `BUILD_FAILURE_EXIT_CODE`, le build est marqué comme un échec de manière appropriée dans GitLab CI.

Si le script que l'utilisateur définit dans le fichier `.gitlab-ci.yml` se termine avec un code non nul, `run_exec` doit se terminer avec la valeur `BUILD_FAILURE_EXIT_CODE`.

> [!note]
> Nous recommandons vivement d'utiliser `BUILD_FAILURE_EXIT_CODE` pour quitter plutôt qu'une valeur codée en dur, car cette valeur peut changer dans n'importe quelle release, rendant votre binaire/script pérenne.

### Code de sortie en cas d'échec de build {#build-failure-exit-code}

Vous pouvez éventuellement fournir un fichier contenant le code de sortie lorsqu'un build échoue. Le chemin attendu pour le fichier est fourni via la variable d'environnement `BUILD_EXIT_CODE_FILE`. Par exemple :

```shell
if [ $exit_code -ne 0 ]; then
  echo $exit_code > ${BUILD_EXIT_CODE_FILE}
  exit ${BUILD_FAILURE_EXIT_CODE}
fi
```

Les jobs CI/CD nécessitent cette méthode pour exploiter la syntaxe [`allow_failure`](https://docs.gitlab.com/ci/yaml/#allow_failure).

> [!note]
> Stockez uniquement le code de sortie entier dans ce fichier. Des informations supplémentaires pourraient entraîner une erreur `unknown Custom executor executable exit code`.

### Échec système {#system-failure}

Vous pouvez envoyer un échec système à GitLab Runner en quittant le processus avec le code d'erreur spécifié dans `SYSTEM_FAILURE_EXIT_CODE`. Si ce code d'erreur est retourné, GitLab Runner réessaie certaines étapes. Si aucune des tentatives n'est réussie, le job est marqué comme échoué.

Vous trouverez ci-dessous un tableau indiquant quelles étapes font l'objet de nouvelles tentatives et combien de fois.

| Nom de l'étape           | Nombre de tentatives                                          | Durée d'attente entre chaque nouvelle tentative |
|----------------------|-------------------------------------------------------------|-------------------------------------|
| `prepare_exec`       | 3                                                           | 3 secondes                           |
| `get_sources`        | Valeur de la variable `GET_SOURCES_ATTEMPTS`. (Valeur par défaut : 1)       | 0 secondes                           |
| `restore_cache`      | Valeur de la variable `RESTORE_CACHE_ATTEMPTS`. (Valeur par défaut : 1)     | 0 secondes                           |
| `download_artifacts` | Valeur de la variable `ARTIFACT_DOWNLOAD_ATTEMPTS`. (Valeur par défaut : 1) | 0 secondes                           |

> [!note]
> Nous recommandons vivement d'utiliser `SYSTEM_FAILURE_EXIT_CODE` pour quitter plutôt qu'une valeur codée en dur, car cette valeur peut changer dans n'importe quelle release, rendant votre binaire/script pérenne.

## Réponse du job {#job-response}

Vous pouvez modifier les variables CI/CD de niveau job `CUSTOM_ENV_` car elles respectent la [précédence des variables CI/CD](https://docs.gitlab.com/ci/variables/#cicd-variable-precedence) documentée. Bien que cette fonctionnalité puisse être souhaitable, lorsque le contexte de job de confiance est requis, la réponse JSON complète du job est fournie automatiquement. Le runner génère un fichier temporaire, référencé dans la variable d'environnement `JOB_RESPONSE_FILE`. Ce fichier existe à chaque étape et est automatiquement supprimé lors du nettoyage.

```shell
$ cat ${JOB_RESPONSE_FILE}
{"id": 123456, "token": "jobT0ken",...}
```
