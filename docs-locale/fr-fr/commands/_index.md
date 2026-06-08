---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Commandes GitLab Runner
---

{{< details >}}

- Niveau : Free, Premium, Ultimate
- Offre : GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner contient un ensemble de commandes que vous utilisez pour enregistrer, gérer et exécuter vos builds.

Vous pouvez consulter la liste des commandes en exécutant :

```shell
gitlab-runner --help
```

Ajoutez `--help` après une commande pour afficher sa page d'aide spécifique :

```shell
gitlab-runner <command> --help
```

## Utilisation des variables d'environnement {#using-environment-variables}

La plupart des commandes prennent en charge les variables d'environnement comme méthode pour transmettre la configuration à la commande.

Vous pouvez voir le nom de la variable d'environnement en invoquant `--help` pour une commande spécifique. Par exemple, vous pouvez voir ci-dessous le message d'aide pour la commande `run` :

```shell
gitlab-runner run --help
```

La sortie est similaire à :

```plaintext
NAME:
   gitlab-runner run - run multi runner service

USAGE:
   gitlab-runner run [command options] [arguments...]

OPTIONS:
   -c, --config "/Users/ayufan/.gitlab-runner/config.toml"      Config file [$CONFIG_FILE]
```

## Exécution en mode débogage {#running-in-debug-mode}

Lorsque vous recherchez la cause d'un comportement indéfini ou d'une erreur, utilisez le mode débogage.

Pour exécuter une commande en mode débogage, faites précéder la commande de `--debug` :

```shell
gitlab-runner --debug <command>
```

## Permission super-utilisateur {#super-user-permission}

Les commandes qui accèdent à la configuration de GitLab Runner se comportent différemment lorsqu'elles sont exécutées en tant que super-utilisateur (`root`). L'emplacement du fichier dépend de l'utilisateur qui exécute la commande.

Lorsque vous exécutez des commandes `gitlab-runner`, vous voyez le mode dans lequel il s'exécute :

```shell
$ gitlab-runner run

INFO[0000] Starting multi-runner from /Users/ayufan/.gitlab-runner/config.toml ...  builds=0
WARN[0000] Running in user-mode.
WARN[0000] Use sudo for system-mode:
WARN[0000] $ sudo gitlab-runner...
```

Vous devez utiliser `user-mode` si vous êtes sûr que c'est le mode avec lequel vous souhaitez travailler. Sinon, faites précéder votre commande de `sudo` :

```shell
$ sudo gitlab-runner run

INFO[0000] Starting multi-runner from /etc/gitlab-runner/config.toml ...  builds=0
INFO[0000] Running in system-mode.
```

Dans le cas de Windows, vous devrez peut-être exécuter l'invite de commande en tant qu'administrateur.

## Fichier de configuration {#configuration-file}

La configuration de GitLab Runner utilise le format [TOML](https://github.com/toml-lang/toml).

Vous pouvez trouver le fichier à modifier :

1. Sur les systèmes \*nix lorsque GitLab Runner est exécuté en tant que super-utilisateur (`root`) : `/etc/gitlab-runner/config.toml`
1. Sur les systèmes \*nix lorsque GitLab Runner est exécuté en tant que non-root : `~/.gitlab-runner/config.toml`
1. Sur les autres systèmes : `./config.toml`

La plupart des commandes acceptent un argument pour spécifier un fichier de configuration personnalisé, vous pouvez donc avoir plusieurs configurations différentes sur une seule machine. Pour spécifier un fichier de configuration personnalisé, utilisez l'indicateur `-c` ou `--config`, ou utilisez la variable d'environnement `CONFIG_FILE`.

## Signaux {#signals}

Vous pouvez utiliser des signaux système pour interagir avec GitLab Runner. Les commandes suivantes prennent en charge les signaux suivants :

| Commande             | Signal              | Action |
|---------------------|---------------------|--------|
| `register`          | `SIGINT`            | Annuler l'enregistrement du runner et supprimer s'il était déjà enregistré. |
| `run`, `run-single` | `SIGINT`, `SIGTERM` | Interrompre tous les builds en cours et quitter dès que possible. Utiliser deux fois pour quitter maintenant (**arrêt forcé**). |
| `run`, `run-single` | `SIGQUIT`           | Arrêter d'accepter de nouveaux builds. Quitter dès que les builds en cours sont terminés (**arrêt gracieux**). |
| `run`               | `SIGHUP`            | Forcer le rechargement du fichier de configuration. |

Par exemple, pour forcer le rechargement du fichier de configuration d'un runner, exécutez :

```shell
sudo kill -SIGHUP <main_runner_pid>
```

Pour les [arrêts gracieux](#gitlab-runner-stop-doesnt-shut-down-gracefully) :

```shell
sudo kill -SIGQUIT <main_runner_pid>
```

> [!warning]
> N'utilisez **pas** `killall` ou `pkill` pour les arrêts gracieux si vous utilisez des exécuteurs `shell` ou `docker`. Cela peut entraîner une gestion incorrecte des signaux en raison de la suppression des sous-processus également. Utilisez-le uniquement sur le processus principal gérant les jobs.

Certains systèmes d'exploitation sont configurés pour redémarrer automatiquement les services en cas d'échec (ce qui est le comportement par défaut sur certaines plateformes). Si votre système d'exploitation possède cette configuration, il peut redémarrer automatiquement le runner s'il est arrêté par les signaux ci-dessus.

## Aperçu des commandes {#commands-overview}

Vous verrez ce qui suit si vous exécutez `gitlab-runner` sans aucun argument :

```plaintext
NAME:
   gitlab-runner - a GitLab Runner

USAGE:
   gitlab-runner [global options] command [command options] [arguments...]

VERSION:
   17.10.1 (ef334dcc)

AUTHOR:
   GitLab Inc. <support@gitlab.com>

COMMANDS:
   list                  List all configured runners
   run                   run multi runner service
   register              register a new runner
   reset-token           reset a runner's token
   install               install service
   uninstall             uninstall service
   start                 start service
   stop                  stop service
   restart               restart service
   status                get status of a service
   run-single            start single runner
   unregister            unregister specific runner
   verify                verify all registered runners
   wrapper               start multi runner service wrapped with gRPC manager server
   fleeting              manage fleeting plugins
   artifacts-downloader  download and extract build artifacts (internal)
   artifacts-uploader    create and upload build artifacts (internal)
   cache-archiver        create and upload cache artifacts (internal)
   cache-extractor       download and extract cache artifacts (internal)
   cache-init            changed permissions for cache paths (internal)
   health-check          check health for a specific address
   proxy-exec            execute internal commands (internal)
   read-logs             reads job logs from a file, used by kubernetes executor (internal)
   help, h               Shows a list of commands or help for one command

GLOBAL OPTIONS:
   --cpuprofile value           write cpu profile to file [$CPU_PROFILE]
   --debug                      debug mode [$RUNNER_DEBUG]
   --log-format value           Choose log format (options: runner, text, json) [$LOG_FORMAT]
   --log-level value, -l value  Log level (options: debug, info, warn, error, fatal, panic) [$LOG_LEVEL]
   --help, -h                   show help
   --version, -v                print the version
```

Ci-dessous, nous expliquons en détail ce que fait chaque commande.

## Commandes liées à l'enregistrement {#registration-related-commands}

Utilisez les commandes suivantes pour enregistrer un nouveau runner, ou pour les lister et vérifier s'ils sont toujours enregistrés.

- [`gitlab-runner register`](#gitlab-runner-register)
  - [Enregistrement interactif](#interactive-registration)
  - [Enregistrement non interactif](#non-interactive-registration)
- [`gitlab-runner list`](#gitlab-runner-list)
- [`gitlab-runner verify`](#gitlab-runner-verify)
- [`gitlab-runner unregister`](#gitlab-runner-unregister)

Ces commandes prennent en charge les arguments suivants :

| Paramètre  | Valeur par défaut                                                   | Description |
|------------|-----------------------------------------------------------|-------------|
| `--config` | Voir la [section du fichier de configuration](#configuration-file) | Spécifier un fichier de configuration personnalisé à utiliser |

### `gitlab-runner register` {#gitlab-runner-register}

Cette commande enregistre votre runner dans GitLab en utilisant l'[API Runners](https://docs.gitlab.com/api/runners/) GitLab.

Le runner enregistré est ajouté au [fichier de configuration](#configuration-file). Vous pouvez utiliser plusieurs configurations dans une seule installation de GitLab Runner. L'exécution de `gitlab-runner register` ajoute une nouvelle entrée de configuration. Cela ne supprime pas les précédentes.

Vous pouvez enregistrer un runner :

- de manière interactive.
- de manière non interactive.

> [!note]
> Les runners peuvent être enregistrés directement en utilisant l'[API Runners](https://docs.gitlab.com/api/runners/) GitLab, mais la configuration n'est pas générée automatiquement.

#### Enregistrement interactif {#interactive-registration}

Cette commande est généralement utilisée en mode interactif (**par défaut**). Plusieurs questions vous sont posées lors de l'enregistrement d'un runner.

Cette question peut être pré-remplie en ajoutant des arguments lors de l'invocation de la commande d'enregistrement :

```shell
gitlab-runner register --name my-runner --url "http://gitlab.example.com" --token my-authentication-token
```

Ou en configurant la variable d'environnement avant la commande `register` :

```shell
export CI_SERVER_URL=http://gitlab.example.com
export RUNNER_NAME=my-runner
export CI_SERVER_TOKEN=my-authentication-token
gitlab-runner register
```

Pour vérifier tous les arguments et environnements possibles, exécutez :

```shell
gitlab-runner register --help
```

#### Enregistrement non interactif {#non-interactive-registration}

Il est possible d'utiliser l'enregistrement en mode non interactif / sans surveillance.

Vous pouvez spécifier les arguments lors de l'invocation de la commande d'enregistrement :

```shell
gitlab-runner register --non-interactive <other-arguments>
```

Ou en configurant la variable d'environnement avant la commande `register` :

```shell
<other-environment-variables>
export REGISTER_NON_INTERACTIVE=true
gitlab-runner register
```

> [!note]
> Les paramètres booléens doivent être transmis dans la ligne de commande avec `--key={true|false}`.

#### Fichier de modèle de configuration `[[runners]]` {#runners-configuration-template-file}

{{< history >}}

- [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4228) dans GitLab Runner 12.2.

{{< /history >}}

Des options supplémentaires peuvent être configurées lors de l'enregistrement du runner en utilisant la fonctionnalité [fichier de modèle de configuration](../register/_index.md#register-with-a-configuration-template).

### `gitlab-runner list` {#gitlab-runner-list}

Cette commande liste tous les runners enregistrés dans le [fichier de configuration](#configuration-file).

### `gitlab-runner verify` {#gitlab-runner-verify}

Cette commande vérifie que les runners enregistrés peuvent se connecter à GitLab. Cependant, elle ne vérifie pas si les runners sont utilisés par le service GitLab Runner. Un exemple de sortie est :

```plaintext
Verifying runner... is alive                        runner=fee9938e
Verifying runner... is alive                        runner=0db52b31
Verifying runner... is alive                        runner=826f687f
Verifying runner... is alive                        runner=32773c0f
```

Pour supprimer les anciens runners qui ont été retirés de GitLab, exécutez la commande suivante.

> [!warning]
> Cette opération est irréversible. Elle met à jour le fichier de configuration, alors assurez-vous d'avoir une sauvegarde de `config.toml` avant de l'exécuter.

```shell
gitlab-runner verify --delete
```

### `gitlab-runner unregister` {#gitlab-runner-unregister}

Cette commande désenregistre les runners enregistrés en utilisant l'[API Runners](https://docs.gitlab.com/api/runners/#delete-a-runner) GitLab.

Elle attend soit :

- Une URL complète et le jeton du runner.
- Le nom du runner.

Avec l'option `--all-runners`, elle désenregistre tous les runners associés.

> [!note]
> Les runners peuvent être désenregistrés avec l'[API Runners](https://docs.gitlab.com/api/runners/#delete-a-runner) GitLab, mais la configuration n'est pas modifiée pour l'utilisateur.

- Si le runner a été créé avec un jeton d'enregistrement de runner, `gitlab-runner unregister` avec le jeton d'authentification du runner supprime le runner.
- Si le runner a été créé dans l'interface utilisateur GitLab ou avec l'API Runners, `gitlab-runner unregister` avec le jeton d'authentification du runner supprime le gestionnaire de runner, mais pas le runner. Pour supprimer complètement le runner, [supprimez le runner dans la page d'administration des runners](https://docs.gitlab.com/ci/runners/runners_scope/#delete-instance-runners) ou utilisez le point de terminaison de l'API REST [`DELETE /runners`](https://docs.gitlab.com/api/runners/#delete-a-runner).

Pour désenregistrer un seul runner, obtenez d'abord les détails du runner en exécutant `gitlab-runner list` :

```plaintext
test-runner     Executor=shell Token=t0k3n URL=http://gitlab.example.com
```

Utilisez ensuite ces informations pour le désenregistrer, en utilisant l'une des commandes suivantes.

> [!warning]
> Cette opération est irréversible. Elle met à jour le fichier de configuration, alors assurez-vous d'avoir une sauvegarde de `config.toml` avant de l'exécuter.

#### Par URL et jeton {#by-url-and-token}

```shell
gitlab-runner unregister --url "http://gitlab.example.com/" --token t0k3n
```

#### Par nom {#by-name}

```shell
gitlab-runner unregister --name test-runner
```

> [!note]
> S'il y a plus d'un runner avec le nom donné, seul le premier est supprimé.

#### Tous les runners {#all-runners}

```shell
gitlab-runner unregister --all-runners
```

### `gitlab-runner reset-token` {#gitlab-runner-reset-token}

Cette commande réinitialise le jeton d'un runner en utilisant l'API Runners GitLab, avec soit l'[ID du runner](https://docs.gitlab.com/api/runners/#reset-runners-authentication-token-by-using-the-runner-id) soit le [jeton actuel](https://docs.gitlab.com/api/runners/#reset-runners-authentication-token-by-using-the-current-token).

Elle attend le nom du runner (ou l'URL et l'ID), et un PAT optionnel si la réinitialisation se fait par ID de runner. Le PAT et l'ID du runner sont destinés à être utilisés si le jeton a déjà expiré.

Avec l'option `--all-runners`, elle réinitialise les jetons de tous les runners associés.

#### Avec le jeton actuel du runner {#with-runners-current-token}

```shell
gitlab-runner reset-token --name test-runner
```

#### Avec le PAT et le nom du runner {#with-pat-and-runner-name}

```shell
gitlab-runner reset-token --name test-runner --pat PaT
```

#### Avec le PAT, l'URL GitLab et l'ID du runner {#with-pat-gitlab-url-and-runner-id}

```shell
gitlab-runner reset-token --url "https://gitlab.example.com/" --id 12345 --pat PaT
```

#### Tous les runners {#all-runners-1}

```shell
gitlab-runners reset-token --all-runners
```

## Commandes liées aux services {#service-related-commands}

Les commandes suivantes vous permettent de gérer le runner en tant que service système ou utilisateur. Utilisez-les pour installer, désinstaller, démarrer et arrêter le service du runner.

- [`gitlab-runner install`](#gitlab-runner-install)
- [`gitlab-runner uninstall`](#gitlab-runner-uninstall)
- [`gitlab-runner start`](#gitlab-runner-start)
- [`gitlab-runner stop`](#gitlab-runner-stop)
- [`gitlab-runner restart`](#gitlab-runner-restart)
- [`gitlab-runner status`](#gitlab-runner-status)
- [Services multiples](#multiple-services)
- [**Accès refusé** lors de l'exécution des commandes liées aux services](#access-denied-when-running-the-service-related-commands)

Toutes les commandes liées aux services acceptent ces arguments :

| Paramètre        | Valeur par défaut                                           | Description |
|------------------|---------------------------------------------------|-------------|
| `--service`      | `gitlab-runner`                                   | Spécifier un nom de service personnalisé |
| `--config`       | Voir le [fichier de configuration](#configuration-file) | Spécifier un fichier de configuration personnalisé à utiliser |
| `--user-service` | Voir le [service utilisateur](#user-service)                 | Configurer GitLab Runner pour s'exécuter en tant que service utilisateur (systemd) |

### `gitlab-runner install` {#gitlab-runner-install}

Cette commande installe GitLab Runner en tant que service. Elle accepte différents ensembles d'arguments selon le système sur lequel elle est exécutée.

Lorsqu'elle est exécutée sur **Windows** ou en tant que super-utilisateur, elle accepte l'indicateur `--user` qui vous permet de supprimer les privilèges des builds exécutés avec l'exécuteur **shell**.

| Paramètre             | Valeur par défaut                                           | Description |
|-----------------------|---------------------------------------------------|-------------|
| `--service`           | `gitlab-runner`                                   | Spécifier le nom du service à utiliser |
| `--config`            | Voir le [fichier de configuration](#configuration-file) | Spécifier un fichier de configuration personnalisé à utiliser |
| `--syslog`            | `true` (pour les systèmes non systemd)                  | Spécifier si le service doit s'intégrer au service de journalisation système |
| `--working-directory` | le répertoire actuel                             | Spécifier le répertoire racine où toutes les données sont stockées lorsque les builds sont exécutés avec l'exécuteur **shell** |
| `--user`              | `root`                                            | Spécifier l'utilisateur qui exécute les builds |
| `--password`          | aucun                                              | Spécifier le mot de passe de l'utilisateur qui exécute les builds |

### `gitlab-runner uninstall` {#gitlab-runner-uninstall}

Cette commande arrête et désinstalle GitLab Runner en tant que service.

### `gitlab-runner start` {#gitlab-runner-start}

Cette commande démarre le service GitLab Runner.

### `gitlab-runner stop` {#gitlab-runner-stop}

Cette commande arrête le service GitLab Runner.

### `gitlab-runner restart` {#gitlab-runner-restart}

Cette commande arrête puis démarre le service GitLab Runner.

### `gitlab-runner status` {#gitlab-runner-status}

Cette commande affiche le statut du service GitLab Runner. Le code de sortie est zéro lorsque le service est en cours d'exécution et non nul lorsque le service n'est pas en cours d'exécution.

### Services multiples {#multiple-services}

En spécifiant l'indicateur `--service`, il est possible d'avoir plusieurs services GitLab Runner installés, avec plusieurs configurations séparées.

### Service utilisateur {#user-service}

Vous pouvez utiliser certains systèmes d'initialisation (comme `systemd`) pour gérer les services en tant que [services utilisateur](https://wiki.archlinux.org/title/Systemd/User). Si votre système d'initialisation fournit cette fonctionnalité et que vous souhaitez gérer le service `gitlab-runner` en tant que service utilisateur, spécifiez l'indicateur `--user-service` lorsque vous exécutez des commandes liées aux services.

## Commandes liées à l'exécution {#run-related-commands}

Cette commande permet de récupérer et de traiter des builds depuis GitLab.

### `gitlab-runner run` {#gitlab-runner-run}

La commande `gitlab-runner run` est la commande principale qui est exécutée lorsque GitLab Runner est démarré en tant que service. Elle lit tous les runners définis dans `config.toml` et tente de les exécuter tous.

La commande est exécutée et fonctionne jusqu'à ce qu'elle [reçoive un signal](#signals).

Elle accepte les paramètres suivants.

| Paramètre             | Valeur par défaut                                       | Description |
|-----------------------|-----------------------------------------------|-------------|
| `--config`            | Voir le [fichier de configuration](#configuration-file) | Spécifier un fichier de configuration personnalisé à utiliser |
| `--working-directory` | le répertoire actuel                         | Spécifier le répertoire racine où toutes les données sont stockées lorsque les builds s'exécutent avec l'exécuteur **shell** |
| `--user`              | l'utilisateur actuel                              | Spécifier l'utilisateur qui exécute les builds |
| `--syslog`            | `false`                                       | Envoyer tous les journaux vers SysLog (Unix) ou EventLog (Windows) |
| `--listen-address`    | vide                                         | Adresse (`<host>:<port>`) sur laquelle le serveur HTTP de métriques Prometheus doit écouter |

### `gitlab-runner run-single` {#gitlab-runner-run-single}

{{< history >}}

- Possibilité d'utiliser un fichier de configuration [introduite](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37670) dans GitLab Runner 17.1.

{{< /history >}}

Utilisez cette commande supplémentaire pour exécuter un seul build à partir d'une seule instance GitLab. Elle peut :

- Prendre toutes les options soit en tant que paramètres CLI soit en tant que variables d'environnement, y compris l'URL GitLab et le jeton Runner. Par exemple, un seul job avec tous les paramètres spécifiés explicitement :

  ```shell
  gitlab-runner run-single -u http://gitlab.example.com -t my-runner-token --executor docker --docker-image ruby:3.3
  ```

- Lire depuis un fichier de configuration pour utiliser la configuration d'un runner spécifique. Par exemple, un seul job avec un fichier de configuration :

  ```shell
  gitlab-runner run-single -c ~/.gitlab-runner/config.toml -r runner-name
  ```

Vous pouvez voir toutes les options de configuration possibles en utilisant l'indicateur `--help` :

```shell
gitlab-runner run-single --help
```

Vous pouvez utiliser l'option `--max-builds` pour contrôler combien de builds le runner exécute avant de quitter. La valeur par défaut de `0` signifie que le runner n'a pas de limite de build et que les jobs s'exécutent indéfiniment.

Vous pouvez également utiliser l'option `--wait-timeout` pour contrôler combien de temps le runner attend un job avant de quitter. La valeur par défaut de `0` signifie que le runner n'a pas de délai d'expiration et attend indéfiniment entre les jobs.

## Commandes internes {#internal-commands}

GitLab Runner est distribué en tant que binaire unique et contient quelques commandes internes utilisées lors des builds.

### `gitlab-runner artifacts-downloader` {#gitlab-runner-artifacts-downloader}

Télécharger l'archive des artefacts depuis GitLab.

### `gitlab-runner artifacts-uploader` {#gitlab-runner-artifacts-uploader}

Charger l'archive des artefacts vers GitLab.

### `gitlab-runner cache-archiver` {#gitlab-runner-cache-archiver}

Créer une archive de cache, la stocker localement ou la charger vers un serveur externe.

### `gitlab-runner cache-extractor` {#gitlab-runner-cache-extractor}

Restaurer l'archive de cache à partir d'un fichier stocké localement ou en externe.

## Dépannage {#troubleshooting}

Voici quelques problèmes courants.

### **Accès refusé** lors de l'exécution des commandes liées aux services {#access-denied-when-running-the-service-related-commands}

En général, les [commandes liées aux services](#service-related-commands) nécessitent des privilèges administrateur :

- Sur les systèmes Unix (Linux, macOS, FreeBSD), faites précéder `gitlab-runner` de `sudo`
- Sur les systèmes Windows, utilisez l'invite de commande élevée. Exécutez une invite de commande `Administrator`. Pour écrire `Command Prompt` dans le champ de recherche Windows, faites un clic droit et sélectionnez `Run as administrator`. Confirmez que vous souhaitez exécuter l'invite de commande élevée.

## `gitlab-runner stop` ne s'arrête pas correctement {#gitlab-runner-stop-doesnt-shut-down-gracefully}

Lorsque GitLab Runner est installé sur un hôte et exécute des exécuteurs locaux, il démarre des processus supplémentaires pour des opérations telles que le téléchargement ou le chargement d'artefacts, ou la gestion du cache. Ces processus sont exécutés en tant que commandes `gitlab-runner`, ce qui signifie que vous pouvez utiliser `pkill -QUIT gitlab-runner` ou `killall QUIT gitlab-runner` pour les arrêter. Lorsque vous les arrêtez, les opérations dont ils sont responsables échouent.

Voici deux façons d'éviter cela :

- Enregistrez le runner en tant que service local (comme `systemd`) avec `SIGQUIT` comme signal d'arrêt, et utilisez `gitlab-runner stop` ou `systemctl stop gitlab-runner.service`. Voici un exemple de configuration pour activer ce comportement :

  ```ini
  ; /etc/systemd/system/gitlab-runner.service.d/kill.conf
  [Service]
  KillSignal=SIGQUIT
  TimeoutStopSec=infinity
  ```

  - Pour appliquer le changement de configuration, après avoir créé ce fichier, rechargez `systemd` avec `systemctl daemon-reload`.
- Arrêtez manuellement le processus avec `kill -SIGQUIT <pid>`. Vous devez trouver le `pid` du processus principal `gitlab-runner`. Vous pouvez le trouver en consultant les journaux, car il est affiché au démarrage :

  ```shell
  $ gitlab-runner run
  Runtime platform                                    arch=arm64 os=linux pid=8 revision=853330f9 version=16.5.0
  ```

### Enregistrement du fichier d'état de l'ID système : accès refusé {#saving-system-id-state-file-access-denied}

GitLab Runner 15.7 et 15.8 peuvent ne pas démarrer s'il manque des permissions d'écriture pour le répertoire contenant le fichier `config.toml`.

Lorsque GitLab Runner démarre, il recherche le fichier `.runner_system_id` dans le répertoire contenant le `config.toml`. S'il ne peut pas trouver le fichier `.runner_system_id`, il en crée un nouveau. Si GitLab Runner ne dispose pas des permissions d'écriture, il échoue à démarrer.

Pour résoudre ce problème, autorisez temporairement les permissions d'écriture de fichier, puis exécutez `gitlab-runner run`. Une fois le fichier `.runner_system_id` créé, vous pouvez réinitialiser les permissions en lecture seule.
