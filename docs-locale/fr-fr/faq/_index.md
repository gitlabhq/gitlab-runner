---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Dépannage de GitLab Runner
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Cette section peut vous aider lors du dépannage de GitLab Runner.

## Conseils généraux de dépannage {#general-troubleshooting-tips}

### Afficher les journaux {#view-the-logs}

Le service GitLab Runner envoie les journaux à syslog. Pour afficher les journaux, consultez la documentation de votre distribution. Si votre distribution inclut la commande `journalctl`, vous pouvez l'utiliser pour afficher les journaux :

```shell
journalctl --unit=gitlab-runner.service -n 100 --no-pager
docker logs gitlab-runner-container # Docker
kubectl logs gitlab-runner-pod # Kubernetes
```

### Redémarrer le service {#restart-the-service}

```shell
systemctl restart gitlab-runner.service
```

### Afficher les machines Docker {#view-the-docker-machines}

```shell
sudo docker-machine ls
sudo su - && docker-machine ls
```

### Supprimer toutes les machines Docker {#delete-all-docker-machines}

```shell
docker-machine rm $(docker-machine ls -q)
```

### Appliquer les modifications à `config.toml` {#apply-changes-to-configtoml}

```shell
systemctl restart gitlab-runner.service
docker-machine rm $(docker-machine ls -q) # Docker machine
journalctl --unit=gitlab-runner.service -f # Tail the logs to check for potential errors
```

## Confirmer vos versions de GitLab et de GitLab Runner {#confirm-your-gitlab-and-gitlab-runner-versions}

GitLab vise à [garantir la rétrocompatibilité](../_index.md#gitlab-runner-versions). Cependant, comme première étape de dépannage, vous devez vous assurer que votre version de GitLab Runner est identique à votre version de GitLab.

## Que signifie `coordinator` ? {#what-does-coordinator-mean}

Le `coordinator` est l'installation GitLab depuis laquelle un job est demandé.

En d'autres termes, le runner est un agent isolé qui demande des jobs au `coordinator` (installation GitLab via l'API GitLab).

## Où sont stockés les journaux lorsqu'ils sont exécutés en tant que service sur Windows ? {#where-are-logs-stored-when-run-as-a-service-on-windows}

- Si GitLab Runner s'exécute en tant que service sur Windows, il crée des journaux d'événements système. Pour les afficher, ouvrez l'Observateur d'événements (depuis le menu Exécuter, saisissez `eventvwr.msc` ou recherchez « Observateur d'événements »). Allez ensuite dans **Windows Logs > Application**. La **Source** des journaux du Runner est `gitlab-runner`. Si vous utilisez Windows Server Core, exécutez cette commande PowerShell pour obtenir les 20 dernières entrées de journal : `get-eventlog Application -Source gitlab-runner -Newest 20 | format-table -wrap -auto`.

## Activer le mode de journalisation de débogage {#enable-debug-logging-mode}

> [!warning]
> La journalisation de débogage peut constituer un risque de sécurité sérieux. La sortie contient le contenu de toutes les variables et autres secrets disponibles pour le job. Vous devez désactiver toute agrégation de journaux susceptible de transmettre des secrets à des tiers. L'utilisation de variables masquées permet de protéger les secrets dans la sortie du job log, mais pas dans les journaux de conteneurs.

### Dans la ligne de commande {#in-the-command-line}

Depuis un terminal, connecté en tant que root, exécutez ce qui suit.

> [!warning]
> Cette opération ne doit pas être effectuée sur des runners avec l'[exécuteur Shell](../executors/shell.md), car elle redéfinit le service `systemd` et exécute tous les jobs en tant que root. Cela pose des risques de sécurité et des modifications de propriété de fichiers qui rendent difficile le retour à un compte sans privilèges.

```shell
gitlab-runner stop
gitlab-runner --debug run
```

### Dans le `config.toml` de GitLab Runner {#in-the-gitlab-runner-configtoml}

La journalisation de débogage peut être activée dans la [section globale du `config.toml`](../configuration/advanced-configuration.md#the-global-section) en définissant le paramètre `log_level` sur `debug`. Ajoutez la ligne suivante tout en haut de votre `config.toml`, avant/après la ligne concurrent :

```toml
log_level = "debug"
```

### Dans le Helm Chart {#in-the-helm-chart}

Si GitLab Runner est installé dans un cluster Kubernetes à l'aide du [Helm Chart GitLab Runner](../install/kubernetes.md), pour activer la journalisation de débogage, définissez l'option `logLevel` dans la [personnalisation `values.yaml`](../install/kubernetes.md#configure-gitlab-runner-with-the-helm-chart) :

```yaml
## Configure the GitLab Runner logging level. Available values are: debug, info, warn, error, fatal, panic
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration/#the-global-section
##
logLevel: debug
```

## ID de corrélation dans les journaux de GitLab Runner {#correlation-ids-in-gitlab-runner-logs}

GitLab Runner génère un ID de corrélation pour chaque requête API afin de tracer les interactions avec GitLab.

Lorsque la réponse API de GitLab inclut un ID de corrélation dans l'en-tête `X-Request-Id`, la valeur (généralement au format ULID) est utilisée dans les journaux. Si la réponse n'inclut pas d'ID de corrélation, GitLab Runner utilise l'UUID qu'il a généré pour la requête (format hexadécimal minuscule sans tirets). Un ID de corrélation de secours indique que la requête n'a pas atteint GitLab Workhorse. Le problème s'est probablement produit au niveau d'un nœud intermédiaire (tel qu'un WAF, un CDN, un équilibreur de charge ou un proxy).

Vous pouvez utiliser les ID de corrélation pour faire correspondre les entrées de journal entre les composants et tracer les flux de requêtes. Recherchez le champ `correlation_id` dans les journaux de GitLab Runner et l'ID correspondant dans les journaux du serveur GitLab pour corréler les événements.

Exemples d'entrées de journal :

```plaintext
# Valid correlation ID (ULID format from GitLab API response)
Appending trace to coordinator...ok correlation_id=01KKDQ7P6TRW7Z6P2PWG5808EK job=101162491 status=202 Accepted

# Fallback correlation ID (lowercase hex UUID without dashes, generated by runner)
WARNING: Appending trace to coordinator... job failed correlation_id=21fe32aee0e146c194640b075c95ec7c job=101162868 status=403 Forbidden
```

## Configurer le DNS pour un runner avec l'exécuteur Docker {#configure-dns-for-a-docker-executor-runner}

Lorsque vous configurez GitLab Runner avec l'exécuteur Docker, les conteneurs Docker peuvent ne pas parvenir à accéder à GitLab, même lorsque le démon Runner hôte y a accès. Cela peut se produire lorsque le DNS est configuré sur l'hôte mais que ces configurations ne sont pas transmises au conteneur.

**Exemple** :

Le service GitLab et GitLab Runner existent dans deux réseaux différents reliés de deux façons (par exemple, via Internet et via un VPN). Le mécanisme de routage du runner peut interroger le DNS via le service Internet par défaut au lieu du service DNS via le VPN. Cette configuration entraînerait le message suivant :

```shell
Created fresh repository.
++ echo 'Created fresh repository.'
++ git -c 'http.userAgent=gitlab-runner 16.5.0 linux/amd64' fetch origin +da39a3ee5e6b4b0d3255bfef95601890afd80709:refs/pipelines/435345 +refs/heads/master:refs/remotes/origin/master --depth 50 --prune --quiet
fatal: Authentication failed for 'https://gitlab.example.com/group/example-project.git/'
```

Dans ce cas, l'échec d'authentification est causé par un service situé entre Internet et le service GitLab. Ce service utilise des identifiants distincts, que le runner pourrait contourner en utilisant le service DNS via le VPN.

Vous pouvez indiquer à Docker quel serveur DNS utiliser en utilisant la configuration `dns` dans la section `[runners.docker]` du [fichier `config.toml` du Runner](../configuration/advanced-configuration.md#the-runnersdocker-section).

```toml
dns = ["192.168.xxx.xxx","192.168.xxx.xxx"]
```

## J'obtiens `x509: certificate signed by unknown authority` {#im-seeing-x509-certificate-signed-by-unknown-authority}

Pour plus d'informations, consultez [les certificats auto-signés](../configuration/tls-self-signed.md).

## J'obtiens `Permission Denied` lors de l'accès à `/var/run/docker.sock` {#i-get-permission-denied-when-accessing-the-varrundockersock}

Si vous souhaitez utiliser l'exécuteur Docker et que vous vous connectez au moteur Docker installé sur le serveur. Vous pouvez voir l'erreur `Permission Denied`. La cause la plus probable est que votre système utilise SELinux (activé par défaut sur CentOS, Fedora et RHEL). Vérifiez votre politique SELinux sur votre système pour détecter d'éventuels refus.

## Erreur Docker-machine : `Unable to query docker version: Cannot connect to the docker engine endpoint.` {#docker-machine-error-unable-to-query-docker-version-cannot-connect-to-the-docker-engine-endpoint}

Cette erreur concerne le provisionnement des machines et peut être due aux raisons suivantes :

- Il y a un échec TLS. Lorsque `docker-machine` est installé, certains certificats peuvent être invalides. Pour résoudre ce problème, supprimez les certificats et redémarrez le runner :

  ```shell
  sudo su -
  rm -r /root/.docker/machine/certs/*
  service gitlab-runner restart
  ```

  Après le redémarrage du runner, il détecte que les certificats sont vides et les recrée.

- Le nom d'hôte est plus long que la longueur prise en charge par la machine provisionnée. Par exemple, les machines Ubuntu ont une limite de 64 caractères pour `HOST_NAME_MAX`. Le nom d'hôte est indiqué par `docker-machine ls`. Vérifiez le `MachineName` dans la configuration du runner et réduisez la longueur du nom d'hôte si nécessaire.

> [!note]
> Cette erreur a pu se produire avant que Docker ne soit installé sur la machine.

## `dialing environment connection: ssh: rejected: connect failed (open failed)` {#dialing-environment-connection-ssh-rejected-connect-failed-open-failed}

Cette erreur se produit lorsque l'autoscaler Docker ne peut pas atteindre le démon Docker sur le système cible lorsque la connexion est tunnelisée via SSH. Assurez-vous de pouvoir vous connecter en SSH au système cible et d'exécuter avec succès des commandes Docker, par exemple `docker info`.

## Ajouter un profil d'instance AWS à vos runners en autoscaling {#adding-an-aws-instance-profile-to-your-autoscaled-runners}

Après avoir créé un rôle AWS IAM, dans votre console IAM, le rôle possède un **Role ARN** et un **Instance Profile ARNs**. Vous devez utiliser le nom du **Instance Profile**, **et non** le **Role Name**.

Ajoutez la valeur suivante à votre section `[runners.machine]` : `"amazonec2-iam-instance-profile=<instance-profile-name>",`

## L'exécuteur Docker expire lors de la compilation d'un projet Java {#the-docker-executor-gets-timeout-when-building-java-project}

Cela se produit très probablement en raison du pilote de stockage `aufs` défaillant :  [Le processus Java se bloque à l'intérieur du conteneur](https://github.com/moby/moby/issues/18502). La meilleure solution consiste à remplacer le [pilote de stockage](https://docs.docker.com/engine/storage/drivers/select-storage-driver/) par OverlayFS (plus rapide) ou DeviceMapper (plus lent).

Consultez cet article sur la [configuration et l'exécution de Docker](https://docs.docker.com/engine/daemon/) ou cet article sur le [contrôle et la configuration avec systemd](https://docs.docker.com/engine/daemon/proxy/#systemd-unit-file).

## J'obtiens une erreur 411 lors du téléversement d'artefacts {#i-get-411-when-uploading-artifacts}

Cela se produit parce que GitLab Runner utilise `Transfer-Encoding: chunked`, ce qui est défaillant sur les versions antérieures de NGINX (<https://serverfault.com/questions/164220/is-there-a-way-to-avoid-nginx-411-content-length-required-errors>).

Mettez à niveau votre NGINX vers une version plus récente. Pour plus d'informations, consultez ce ticket : <https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1031>

## Je constate d'autres erreurs de téléversement d'artefacts, comment puis-je déboguer davantage ? {#i-am-seeing-other-artifact-upload-errors-how-can-i-further-debug-this}

Les artefacts sont téléversés directement depuis l'environnement de build vers l'instance GitLab, en contournant le processus GitLab Runner. Par exemple :

- Avec l'exécuteur Docker, les téléversements s'effectuent depuis le conteneur Docker
- Avec l'exécuteur Kubernetes, les téléversements s'effectuent depuis le conteneur de build dans le pod de build

Le chemin réseau entre l'environnement de build et l'instance GitLab peut être différent du chemin entre GitLab Runner et l'instance GitLab.

Pour permettre le téléversement des artefacts, assurez-vous que tous les composants du chemin de téléversement autorisent les requêtes POST depuis l'environnement de build vers l'instance GitLab.

Par défaut, le téléverseur d'artefacts enregistre l'URL de téléversement et le code de statut HTTP de la réponse de téléversement. Ces informations ne suffisent pas à comprendre quel système a causé une erreur ou bloqué les téléversements d'artefacts. Pour résoudre les problèmes de téléversement d'artefacts, [activez la journalisation de débogage](https://docs.gitlab.com/ci/variables/#enable-debug-logging) pour les tentatives de téléversement afin de voir les en-têtes et le corps de la réponse de téléversement.

> [!note]
> La longueur du corps de réponse pour la journalisation de débogage des téléversements d'artefacts est limitée à 512 octets. Activez la journalisation uniquement pour le débogage, car des données sensibles peuvent être exposées dans les journaux.

Si les téléversements atteignent GitLab mais échouent avec un code de statut d'erreur (par exemple, un code de statut de réponse non réussi), examinez l'instance GitLab elle-même. Pour les problèmes courants de téléversement d'artefacts, consultez la [documentation GitLab](https://docs.gitlab.com/administration/cicd/job_artifacts_troubleshooting/#job-artifact-upload-fails-with-error-500).

## `No URL provided, cache will not be download`/`uploaded` {#no-url-provided-cache-will-not-be-downloaduploaded}

Cette erreur se produit lorsque l'assistant GitLab Runner reçoit une URL invalide ou ne dispose d'aucune URL pré-signée pour accéder à un cache distant. Vérifiez chaque [entrée `config.toml` liée au cache](../configuration/advanced-configuration.md#the-runnerscache-section) ainsi que les clés et valeurs spécifiques au fournisseur. Une URL invalide peut être construite à partir de tout élément qui ne respecte pas les exigences de syntaxe des URL.

De plus, assurez-vous que vos paramètres `image` et `helper_image_flavor` de l'assistant correspondent et sont à jour.

S'il y a un problème avec la configuration des identifiants, un message d'erreur de diagnostic est ajouté au journal du processus GitLab Runner.

## Erreur : `warning: You appear to have cloned an empty repository.` {#error-warning-you-appear-to-have-cloned-an-empty-repository}

Lors de l'exécution de `git clone` via HTTP(s) (avec GitLab Runner ou manuellement pour des tests) et que vous voyez la sortie suivante :

```shell
$ git clone https://git.example.com/user/repo.git

Cloning into 'repo'...
warning: You appear to have cloned an empty repository.
```

Assurez-vous que la configuration du proxy HTTP dans votre installation du serveur GitLab est effectuée correctement. Lors de l'utilisation d'un proxy HTTP avec sa propre configuration, assurez-vous que les requêtes sont transmises en proxy au **socket GitLab Workhorse**, et non au **socket GitLab Unicorn**.

Le protocole Git via HTTP(S) est résolu par GitLab Workhorse, qui est donc le **main entrypoint** de GitLab.

Si vous utilisez une installation de package Linux mais ne souhaitez pas utiliser le serveur NGINX fourni, consultez [l'utilisation d'un serveur web non fourni](https://docs.gitlab.com/omnibus/settings/nginx/#use-a-non-bundled-web-server).

Dans le dépôt GitLab Recipes, vous trouverez des [exemples de configuration de serveur web](https://gitlab.com/gitlab-org/gitlab-recipes/tree/master/web-server) pour Apache et NGINX.

Si vous utilisez GitLab installé depuis les sources, consultez la documentation et les exemples ci-dessus. Assurez-vous que tout le trafic HTTP(S) transite par le **GitLab Workhorse**.

Consultez [un exemple de ticket d'utilisateur](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1105).

## Erreur : `zoneinfo.zip: no such file or directory` lors de l'utilisation de `Timezone` ou `OffPeakTimezone` {#error-zoneinfozip-no-such-file-or-directory-error-when-using-timezone-or-offpeaktimezone}

Il est possible de configurer le fuseau horaire dans lequel les périodes `[[docker.machine.autoscaling]]` sont décrites. Cette fonctionnalité devrait fonctionner immédiatement sur la plupart des systèmes Unix. Cependant, sur certains systèmes Unix et la plupart des systèmes non Unix (comme Windows, où les binaires GitLab Runner sont disponibles), le runner peut planter au démarrage avec une erreur :

```plaintext
Failed to load config Invalid OffPeakPeriods value: open /usr/local/go/lib/time/zoneinfo.zip: no such file or directory
```

L'erreur est causée par le package `time` dans Go. Go utilise la base de données de fuseaux horaires IANA pour charger la configuration du fuseau horaire spécifié. Sur la plupart des systèmes Unix, cette base de données est déjà présente dans l'un des chemins bien connus (`/usr/share/zoneinfo`, `/usr/share/lib/zoneinfo`, `/usr/lib/locale/TZ/`). Le package `time` de Go recherche la base de données de fuseaux horaires dans ces trois chemins. S'il ne trouve aucun d'entre eux, mais que la machine dispose d'un environnement de développement Go configuré, il bascule vers le fichier `$GOROOT/lib/time/zoneinfo.zip`.

Si aucun de ces chemins n'est présent (par exemple sur un hôte Windows en production), l'erreur ci-dessus est générée.

Si votre système prend en charge la base de données de fuseaux horaires IANA, mais qu'elle n'est pas disponible par défaut, vous pouvez essayer de l'installer. Pour les systèmes Linux, cela peut être fait par exemple avec :

```shell
# on Debian/Ubuntu based systems
sudo apt-get install tzdata

# on RPM based systems
sudo yum install tzdata

# on Linux Alpine
sudo apk add -U tzdata
```

Si votre système ne fournit pas cette base de données de manière _native_, vous pouvez faire fonctionner `OffPeakTimezone` en suivant les étapes ci-dessous :

1. Téléchargez le [`zoneinfo.zip`](https://gitlab-runner-downloads.s3.amazonaws.com/latest/zoneinfo.zip). À partir de la version v9.1.0, vous pouvez télécharger le fichier depuis un chemin balisé. Dans ce cas, vous devez remplacer `latest` par le nom du tag (par exemple, `v9.1.0`) dans l'URL de téléchargement de `zoneinfo.zip`.

1. Stockez ce fichier dans un répertoire bien connu. Nous suggérons d'utiliser le même répertoire que celui où se trouve le fichier `config.toml`. Ainsi, par exemple, si vous hébergez le Runner sur une machine Windows et que votre fichier de configuration est stocké à l'emplacement `C:\gitlab-runner\config.toml`, enregistrez le `zoneinfo.zip` à l'emplacement `C:\gitlab-runner\zoneinfo.zip`.

1. Définissez la variable d'environnement `ZONEINFO` contenant le chemin complet vers le fichier `zoneinfo.zip`. Si vous démarrez le Runner à l'aide de la commande `run`, vous pouvez le faire avec :

   ```shell
   ZONEINFO=/etc/gitlab-runner/zoneinfo.zip gitlab-runner run <other options ...>
   ```

   ou si vous utilisez Windows :

   ```powershell
   C:\gitlab-runner> set ZONEINFO=C:\gitlab-runner\zoneinfo.zip
   C:\gitlab-runner> gitlab-runner run <other options ...>
   ```

   Si vous démarrez GitLab Runner en tant que service système, vous devez mettre à jour ou remplacer la configuration du service :

   - Sur les systèmes Unix, modifiez les paramètres via votre logiciel de gestion de services.
   - Sur Windows, ajoutez la variable `ZONEINFO` à la liste des variables d'environnement disponibles pour l'utilisateur GitLab Runner via les paramètres système.

## Pourquoi ne puis-je pas exécuter plus d'une instance de GitLab Runner ? {#why-cant-i-run-more-than-one-instance-of-gitlab-runner}

Vous pouvez, mais sans partager le même fichier `config.toml`.

L'exécution de plusieurs instances de GitLab Runner en utilisant le même fichier de configuration peut entraîner un comportement inattendu et difficile à déboguer. Une seule instance de GitLab Runner peut utiliser un fichier `config.toml` spécifique à la fois.

## Les jobs subissent des délais avant de démarrer {#jobs-experience-delays-before-starting}

Si les jobs de certains projets subissent des délais importants avant de démarrer tandis que les jobs d'autres projets s'exécutent immédiatement, vous rencontrez peut-être des problèmes de long polling.

**Symptômes :**

- Les jobs sont mis en file d'attente mais prennent un temps inhabituellement long pour démarrer l'exécution (correspondant généralement au délai d'expiration du long polling de votre instance GitLab).
- Certains runners semblent bloqués tandis que d'autres traitent les jobs normalement.
- Les journaux de GitLab Runner affichent `CONFIGURATION: Long polling issues detected`.

**Cause :**

Ce problème se produit lorsque les workers de GitLab Runner se bloquent dans des requêtes de long polling vers GitLab, ce qui empêche le traitement rapide d'autres jobs. Ces problèmes vont des goulots d'étranglement de performance aux blocages complets, selon la configuration. Le problème est lié à la fonctionnalité de long polling GitLab CI/CD contrôlée par le paramètre `apiCiLongPollingDuration` de GitLab Workhorse (par défaut :  50s).

**Solution :**

Ces problèmes peuvent survenir dans plusieurs scénarios de configuration. Pour des informations complètes sur les causes, les exemples de configuration et les solutions, consultez la section [Problèmes de long polling](../configuration/advanced-configuration.md#long-polling-issues) dans la documentation de configuration avancée.

## `Job failed (system failure): preparing environment:` {#job-failed-system-failure-preparing-environment}

Cette erreur est souvent due au fait que votre shell [charge votre profil](../shells/_index.md#shell-profile-loading), et l'un des scripts est à l'origine de l'échec.

Exemple de `dotfiles` connus pour causer des échecs :

- `.bash_logout`
- `.condarc`
- `.rvmrc`

SELinux peut également être la cause de cette erreur. Vous pouvez le confirmer en consultant le journal d'audit SELinux :

```shell
sealert -a /var/log/audit/audit.log
```

## Le runner se termine brusquement après l'étape `Cleaning up` {#runner-abruptly-terminates-after-cleaning-up-stage}

Il a été signalé que CrowdStrike Falcon Sensor tue les pods après l'étape `Cleaning up files` d'un job lorsque le paramètre « container drift detection » était activé. Pour vous assurer que les jobs peuvent se terminer, vous devez désactiver ce paramètre.

## Le job échoue avec `remote error: tls: bad certificate (exec.go:71:0s)` {#job-fails-with-remote-error-tls-bad-certificate-execgo710s}

Cette erreur peut se produire lorsque l'heure système change de manière significative pendant un job qui crée des artefacts. En raison du changement d'heure système, les certificats SSL expirent, ce qui provoque une erreur lorsque le runner tente de téléverser des artefacts.

Pour garantir que la vérification SSL peut réussir lors du téléversement d'artefacts, modifiez l'heure système à une date et une heure valides à la fin du job. Étant donné que l'heure de création du fichier d'artefacts a également changé, ils sont automatiquement archivés.

## Helm Chart : `ERROR .. Unauthorized` {#helm-chart-error--unauthorized}

Avant de désinstaller ou de mettre à niveau des runners déployés avec Helm, mettez-les en pause dans GitLab et attendez que les jobs en cours se terminent.

Si vous supprimez un pod runner avec `helm uninstall` ou `helm upgrade` pendant qu'un job est en cours d'exécution, des erreurs `Unauthorized` comme les suivantes peuvent se produire lorsque le job se termine :

```plaintext
ERROR: Error cleaning up pod: Unauthorized
ERROR: Error cleaning up secrets: Unauthorized
ERROR: Job failed (system failure): Unauthorized
```

Cela se produit probablement parce que lorsque le runner est supprimé, les liaisons de rôles sont supprimées. Le pod runner continue jusqu'à la fin du job, puis le runner tente de le supprimer. Sans la liaison de rôle, le pod runner n'a plus accès.

Consultez [ce ticket](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/225) pour plus de détails.

## Erreur de démarrage du service Elasticsearch `max virtual memory areas vm.max_map_count [65530] is too low` {#elasticsearch-service-startup-error-max-virtual-memory-areas-vmmax_map_count-65530-is-too-low}

Au démarrage d'un conteneur de service Elasticsearch, vous pouvez recevoir une erreur similaire à :

- `max virtual memory areas vm.max_map_count [65530] is too low, increase to at least [262144]`

Elasticsearch a une exigence `vm.max_map_count` qui doit être définie sur l'instance sur laquelle Elasticsearch est exécuté. Consultez la [documentation Elasticsearch](https://www.elastic.co/docs/deploy-manage/deploy/self-managed/install-elasticsearch-docker-prod) pour savoir comment définir cette valeur correctement selon la plateforme.

## Erreur : `Preparing the "docker+machine" executor ERROR: Preparation failed: exit status 1` {#error-preparing-the-dockermachine-executor-error-preparation-failed-exit-status-1}

Cette erreur peut se produire lorsque la machine Docker n'est pas en mesure de créer correctement les machines virtuelles de l'exécuteur. Pour obtenir plus d'informations sur l'erreur, créez manuellement la machine virtuelle avec les mêmes `MachineOptions` que ceux que vous avez définis dans votre `config.toml`.

Par exemple : `docker-machine create --driver=google --google-project=GOOGLE-PROJECT-ID --google-zone=GOOGLE-ZONE ...`.

## Erreur : `No unique index found for name` {#error-no-unique-index-found-for-name}

Cette erreur peut se produire lorsque vous créez ou mettez à jour un runner et que la base de données ne dispose pas d'un index unique pour la table `tags`. Dans l'interface GitLab, vous pouvez obtenir une erreur `Response not successful: Received status code 500`.

Ce problème peut affecter les instances ayant subi plusieurs mises à niveau majeures sur une période prolongée. Pour résoudre ce problème, consolidez les tags en double dans la table avec la [tâche Rake `gitlab:db:deduplicate_tags`](https://docs.gitlab.com/administration/raketasks/maintenance/#check-the-database-for-duplicate-cicd-tags). Pour plus d'informations, consultez les [tâches Rake](https://docs.gitlab.com/administration/raketasks/).

## Erreur : `Not authorized to perform sts:AssumeRoleWithWebIdentity` {#error-not-authorized-to-perform-stsassumerolewithwebidentity}

Si vous avez configuré un rôle IAM pour la ressource Kubernetes ServiceAccount de votre runner, mais que les journaux du runner indiquent qu'il n'est pas en mesure d'effectuer `sts:AssumeRoleWithWebIdentity`, vous pouvez obtenir une erreur indiquant :

```plaintext
{"error":"Not authorized to perform sts:AssumeRoleWithWebIdentity","level":"error","msg":"error while generating S3 pre-signed URL","time":"2025-10-15T18:07:20Z"}
```

Ce problème se produit lorsque vous incluez `https://` dans la condition `StringLike` ou `StringEquals` de la configuration des entités de confiance de votre rôle IAM.

Pour résoudre ce problème, supprimez `https://` de l'URL OIDC :

```json
"Action": "sts:AssumeRoleWithWebIdentity",
"Condition": {
  "StringLike": {
    "oidc.eks.<AWS_REGION>.amazonaws.com/id/<OIDC_ID>:sub": "system:serviceaccount:<NAMESPACE>:<SERVICE_ACCOUNT>"
  }
}
```
