---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "Configuration de la mise à l'échelle automatique de l'exécuteur Docker Machine"
---

{{< details >}}

- Niveau : Free, Premium, Ultimate
- Offre : GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!note]
> L'exécuteur Docker Machine a été déprécié dans GitLab 17.5 et sa suppression est prévue dans GitLab 20.0 (mai 2027). Bien que nous continuions à prendre en charge l'exécuteur Docker Machine jusqu'à GitLab 20.0, nous ne prévoyons pas d'ajouter de nouvelles fonctionnalités. Nous ne traiterons que les bugs critiques susceptibles d'empêcher l'exécution des jobs CI/CD ou d'affecter les coûts de fonctionnement. Si vous utilisez l'exécuteur Docker Machine sur Amazon Web Services (AWS) EC2, Microsoft Azure Compute ou Google Compute Engine (GCE), migrez vers le [GitLab Runner Autoscaler](../runner_autoscale/_index.md).

Avec la fonctionnalité de mise à l'échelle automatique, vous utilisez les ressources de manière plus élastique et dynamique.

GitLab Runner peut effectuer une mise à l'échelle automatique, afin que votre infrastructure ne contienne que le nombre d'instances de build nécessaires à tout moment. Lorsque vous configurez GitLab Runner pour utiliser uniquement la mise à l'échelle automatique, le système hébergeant GitLab Runner agit comme un bastion pour toutes les machines qu'il crée. Cette machine est désignée sous le nom de « Runner Manager ».

> [!note]
> Docker a déprécié Docker Machine, la technologie sous-jacente utilisée pour effectuer la mise à l'échelle automatique des runners sur des machines virtuelles cloud publiques. Vous pouvez consulter le ticket discutant de la [stratégie en réponse à la dépréciation de Docker Machine](https://gitlab.com/gitlab-org/gitlab/-/issues/341856) pour plus de détails.

L'autoscaler Docker Machine crée un conteneur par VM, quelle que soit la configuration de `limit` et `concurrent`.

Lorsque cette fonctionnalité est activée et correctement configurée, les jobs sont exécutés sur des machines créées _à la demande_. Ces machines, une fois le job terminé, peuvent attendre d'exécuter les prochains jobs ou être supprimées après le `IdleTime` configuré. Pour de nombreux fournisseurs cloud, cette approche réduit les coûts en utilisant des instances existantes.

Ci-dessous, vous pouvez voir un exemple concret de la fonctionnalité de mise à l'échelle automatique de GitLab Runner, testée sur GitLab.com pour le projet [GitLab Community Edition](https://gitlab.com/gitlab-org/gitlab-foss) :

![Exemple concret de mise à l'échelle automatique](img/autoscale-example.png)

Chaque machine du graphique est une instance cloud indépendante, exécutant des jobs dans des conteneurs Docker.

## Configuration système requise {#system-requirements}

Avant de configurer la mise à l'échelle automatique, vous devez :

- [Préparer votre propre environnement](../executors/docker_machine.md#preparing-the-environment).
- Éventuellement, utilisez une [version dupliquée](../executors/docker_machine.md#forked-version-of-docker-machine) de Docker Machine fournie par GitLab, qui inclut quelques corrections supplémentaires.

## Fournisseurs cloud pris en charge {#supported-cloud-providers}

Le mécanisme de mise à l'échelle automatique est basé sur [Docker Machine](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/tree/main/). Tous les paramètres de virtualisation et de fournisseur cloud pris en charge sont disponibles dans le fork géré par GitLab de [Docker Machine](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/tree/main/).

## Configuration du runner {#runner-configuration}

Cette section décrit les paramètres de mise à l'échelle automatique les plus importants. Pour plus de détails sur les configurations, consultez la [configuration avancée](advanced-configuration.md).

### Options globales du runner {#runner-global-options}

| Paramètre    | Valeur   | Description |
|--------------|---------|-------------|
| `concurrent` | entier | Limite le nombre de jobs pouvant être exécutés simultanément au niveau global. Ce paramètre définit le nombre maximum de jobs pouvant utiliser _tous_ les runners définis, à la fois locaux et avec mise à l'échelle automatique. Conjointement avec `limit` (de la [section `[[runners]]`](#runners-options)) et `IdleCount` (de la [section `[runners.machine]`](advanced-configuration.md#the-runnersmachine-section)), il affecte la limite supérieure des machines créées. |

### Options `[[runners]]` {#runners-options}

| Paramètre  | Valeur   | Description |
|------------|---------|-------------|
| `executor` | chaîne  | Pour utiliser la fonctionnalité de mise à l'échelle automatique, `executor` doit être défini sur `docker+machine`. |
| `limit`    | entier | Limite le nombre de jobs pouvant être traités simultanément par ce token spécifique. `0` signifie aucune limite. Pour la mise à l'échelle automatique, il s'agit de la limite supérieure des machines créées par ce fournisseur (en conjonction avec `concurrent` et `IdleCount`). |

### Options `[runners.machine]` {#runnersmachine-options}

Les détails des paramètres de configuration se trouvent dans [GitLab Runner - Configuration avancée - La section `[runners.machine]`](advanced-configuration.md#the-runnersmachine-section).

### Options `[runners.cache]` {#runnerscache-options}

Les détails des paramètres de configuration se trouvent dans [GitLab Runner - Configuration avancée - La section `[runners.cache]`](advanced-configuration.md#the-runnerscache-section)

### Informations de configuration supplémentaires {#additional-configuration-information}

Il existe également un mode spécial, lorsque vous définissez `IdleCount = 0`. Dans ce mode, les machines sont **toujours** créées **on-demand** avant chaque job (s'il n'y a pas de machine disponible en état inactif). Une fois le job terminé, l'algorithme de mise à l'échelle automatique fonctionne [de la même manière que décrit ci-dessous](#autoscaling-algorithm-and-parameters). La machine attend les prochains jobs et, si aucun n'est exécuté, après la période `IdleTime`, la machine est supprimée. S'il n'y a pas de jobs, il n'y a pas de machines en état inactif.

Si `IdleCount` est défini sur une valeur supérieure à `0`, des VM inactives sont créées en arrière-plan. Le runner acquiert une VM inactive existante avant de demander un nouveau job.

- Si le job est assigné au runner, ce job est envoyé à la VM précédemment acquise.
- Si le job n'est pas assigné au runner, le verrou sur la VM inactive est libéré et la VM est renvoyée dans le pool.

## Limiter le nombre de VM créées par l'exécuteur Docker Machine {#limit-the-number-of-vms-created-by-the-docker-machine-executor}

Pour limiter le nombre de machines virtuelles (VM) créées par l'exécuteur Docker Machine, utilisez le paramètre `limit` dans la section `[[runners]]` du fichier `config.toml`.

Le paramètre `concurrent` **ne limite pas** le nombre de VM.

Un processus peut être configuré pour gérer plusieurs workers de runner. Pour plus d'informations, consultez [Configuration de base : un gestionnaire de runner, un runner](../fleet_scaling/_index.md#basic-configuration-one-runner-manager-one-runner).

Cet exemple illustre les valeurs définies dans le fichier `config.toml` pour un processus de runner :

```toml
concurrent = 100

[[runners]]
name = "first"
executor = "shell"
limit = 40
(...)

[[runners]]
name = "second"
executor = "docker+machine"
limit = 30
(...)

[[runners]]
name = "third"
executor = "ssh"
limit = 10

[[runners]]
name = "fourth"
executor = "virtualbox"
limit = 20
(...)

```

Avec cette configuration :

- Un processus de runner peut créer quatre workers de runner différents utilisant différents environnements d'exécution.
- La valeur `concurrent` est définie à 100, donc ce runner exécute au maximum 100 jobs GitLab CI/CD simultanés.
- Seul le worker de runner `second` est configuré pour utiliser l'exécuteur Docker Machine et peut donc créer automatiquement des VM.
- Le paramètre `limit` de `30` signifie que le worker de runner `second` peut exécuter au maximum 30 jobs CI/CD sur des VM avec mise à l'échelle automatique à tout moment.
- Alors que `concurrent` définit la limite de simultanéité globale pour plusieurs workers `[[runners]]`, `limit` définit la simultanéité maximale pour un seul worker `[[runners]]`.

Dans cet exemple, le processus de runner gère :

- Pour tous les workers `[[runners]]`, jusqu'à 100 jobs simultanés.
- Pour le worker `first`, pas plus de 40 jobs, qui sont exécutés avec l'exécuteur `shell`.
- Pour le worker `second`, pas plus de 30 jobs, qui sont exécutés avec l'exécuteur `docker+machine`. De plus, GitLab Runner maintient des VM en fonction de la configuration de mise à l'échelle automatique dans `[runners.machine]`, mais pas plus de 30 VM dans tous les états (inactif, en cours d'utilisation, en cours de création, en cours de suppression).
- Pour le worker `third`, pas plus de 10 jobs, exécutés avec l'exécuteur `ssh`.
- Pour le worker `fourth`, pas plus de 20 jobs, exécutés avec l'exécuteur `virtualbox`.

Dans ce deuxième exemple, deux workers `[[runners]]` sont configurés pour utiliser l'exécuteur `docker+machine`. Avec cette configuration, chaque worker de runner gère un pool distinct de VM limité par la valeur du paramètre `limit`.

```toml
concurrent = 100

[[runners]]
name = "first"
executor = "docker+machine"
limit = 80
(...)

[[runners]]
name = "second"
executor = "docker+machine"
limit = 50
(...)

```

Dans cet exemple :

- Le runner traite au maximum 100 jobs (la valeur de `concurrent`).
- Le processus de runner exécute des jobs dans deux workers `[[runners]]`, chacun utilisant l'exécuteur `docker+machine`.
- Le runner `first` peut créer au maximum 80 VM. Ce runner peut donc exécuter au maximum 80 jobs à tout moment.
- Le runner `second` peut créer au maximum 50 VM. Ce runner peut donc exécuter au maximum 50 jobs à tout moment.

> [!note]
> Bien que la somme des valeurs de limite soit `130` (`80 + 50`), le processus de runner exécute au maximum 100 jobs simultanément car le paramètre global `concurrent` est à 100.

## Algorithme et paramètres de mise à l'échelle automatique {#autoscaling-algorithm-and-parameters}

L'algorithme de mise à l'échelle automatique est basé sur ces paramètres :

- `IdleCount`
- `IdleCountMin`
- `IdleScaleFactor`
- `IdleTime`
- `MaxGrowthRate`
- `limit`

Toute machine n'exécutant pas de job est considérée comme inactive. Lorsque GitLab Runner est en mode de mise à l'échelle automatique, il surveille toutes les machines et s'assure qu'il y a toujours un `IdleCount` de machines inactives.

Si le nombre de machines inactives est insuffisant, GitLab Runner commence à provisionner de nouvelles machines, dans la limite de `MaxGrowthRate`. Les demandes de machines au-delà de la valeur `MaxGrowthRate` sont mises en attente jusqu'à ce que le nombre de machines en cours de création tombe en dessous de `MaxGrowthRate`.

Simultanément, GitLab Runner vérifie la durée de l'état inactif de chaque machine. Si le temps dépasse la valeur `IdleTime`, la machine est automatiquement supprimée.

### Exemple de configuration {#example-configuration}

Considérez un GitLab Runner configuré avec les paramètres de mise à l'échelle automatique suivants :

```toml
[[runners]]
  limit = 10
  # (...)
  executor = "docker+machine"
  [runners.machine]
    MaxGrowthRate = 1
    IdleCount = 2
    IdleTime = 1800
    # (...)
```

Au début, lorsqu'aucun job n'est en file d'attente, GitLab Runner démarre deux machines (`IdleCount = 2`) et les place en état inactif. De plus, `IdleTime` est défini à 30 minutes (`IdleTime = 1800`).

Supposons maintenant que cinq jobs sont en file d'attente dans GitLab CI/CD. Les deux premiers jobs sont envoyés aux machines inactives dont nous disposons de deux. GitLab Runner démarre de nouvelles machines car il constate que le nombre de machines inactives est inférieur à `IdleCount` (`0 < 2`). Ces machines sont provisionnées séquentiellement, pour éviter de dépasser `MaxGrowthRate`.

Les trois jobs restants sont assignés à la première machine prête. À titre d'optimisation, il peut s'agir d'une machine qui était occupée mais a maintenant terminé son job, ou d'une machine nouvellement provisionnée. Pour cet exemple, supposons que le provisionnement est rapide et que les nouvelles machines sont prêtes avant que les jobs précédents ne soient terminés.

Nous avons maintenant une machine inactive, donc GitLab Runner démarre une nouvelle machine pour satisfaire `IdleCount`. Comme il n'y a pas de nouveaux jobs en file d'attente, ces deux machines restent en état inactif et GitLab Runner est satisfait.

**Ce qui s'est passé** :

Dans l'exemple, deux machines attendent en état inactif de nouveaux jobs. Une fois les cinq jobs mis en file d'attente, de nouvelles machines sont créées. Au total, il y a donc sept machines : cinq exécutant des jobs et deux en état inactif attendant les prochains jobs.

GitLab Runner crée une nouvelle machine inactive pour chaque machine utilisée pour l'exécution de job, jusqu'à ce que `IdleCount` soit satisfait. Les machines sont créées jusqu'au nombre défini par le paramètre `limit`. Lorsque GitLab Runner détecte que cette `limit` a été atteinte, il arrête la mise à l'échelle automatique. Les nouveaux jobs doivent attendre dans la file d'attente des jobs jusqu'à ce que les machines reviennent à l'état inactif.

Dans l'exemple ci-dessus, deux machines inactives sont toujours disponibles. Le paramètre `IdleTime` ne s'applique que lorsque le nombre dépasse `IdleCount`. À ce stade, GitLab Runner réduit le nombre de machines pour correspondre à `IdleCount`.

**Réduction** :

Une fois le job terminé, la machine est mise en état inactif et attend que de nouveaux jobs soient exécutés. Si aucun nouveau job n'apparaît dans la file d'attente, les machines inactives sont supprimées après le délai spécifié par `IdleTime`. Dans cet exemple, toutes les machines sont supprimées après 30 minutes d'inactivité (mesurée à partir de la fin de la dernière exécution de job de chaque machine). GitLab Runner maintient un `IdleCount` de machines inactives en fonctionnement, comme au début de l'exemple.

L'algorithme de mise à l'échelle automatique fonctionne comme suit :

1. GitLab Runner démarre.
1. GitLab Runner crée deux machines inactives.
1. GitLab Runner prend en charge un job.
1. GitLab Runner crée une machine supplémentaire pour maintenir deux machines inactives.
1. Le job pris en charge se termine, résultant en trois machines inactives.
1. Lorsque l'une des trois machines inactives dépasse `IdleTime` depuis le moment où elle a pris en charge le dernier job, elle est supprimée.
1. GitLab Runner maintient toujours au moins deux machines inactives pour un traitement rapide des jobs.

Le graphique suivant illustre les états des machines et des builds (jobs) dans le temps :

![Graphique des états de mise à l'échelle automatique](img/autoscale-state-chart.png)

## Comment `concurrent`, `limit` et `IdleCount` génèrent la limite supérieure des machines en cours d'exécution {#how-concurrent-limit-and-idlecount-generate-the-upper-limit-of-running-machines}

Il n'existe pas d'équation magique pour déterminer les valeurs à attribuer à `limit` ou `concurrent`. Agissez en fonction de vos besoins. Avoir `IdleCount` machines inactives est une fonctionnalité d'accélération. Vous n'avez pas besoin d'attendre 10 s/20 s/30 s pour que l'instance soit créée. Mais en tant qu'utilisateur, vous souhaiteriez que toutes vos machines (pour lesquelles vous devez payer) exécutent des jobs, plutôt que de rester en état inactif. Vous devriez donc définir `concurrent` et `limit` sur des valeurs qui permettent d'exécuter le nombre maximum de machines que vous êtes prêt à payer. Quant à `IdleCount`, il doit être défini sur une valeur qui génère un minimum de machines _non utilisées_ lorsque la file d'attente des jobs est vide.

Prenons l'exemple suivant :

```toml
concurrent=20

[[runners]]
  limit = 40
  [runners.machine]
    IdleCount = 10
```

Dans le scénario ci-dessus, le nombre total de machines que nous pourrions avoir est de 30. La `limit` du total des machines (en cours de build et inactives) peut être de 40. Nous pouvons avoir 10 machines inactives, mais les jobs `concurrent` sont au nombre de 20. Ainsi, au total, nous pouvons avoir 20 machines simultanées exécutant des jobs et 10 machines inactives, pour un total de 30.

Mais que se passe-t-il si `limit` est inférieur au nombre total de machines qui pourraient être créées ? L'exemple ci-dessous explique ce cas :

```toml
concurrent=20

[[runners]]
  limit = 25
  [runners.machine]
    IdleCount = 10
```

Dans cet exemple, vous pouvez avoir un maximum de 20 jobs simultanés et 25 machines. Dans le pire des cas, vous ne pouvez pas avoir 10 machines inactives, mais seulement 5, car `limit` est 25.

## La stratégie `IdleScaleFactor` {#the-idlescalefactor-strategy}

Le paramètre `IdleCount` définit un nombre statique de machines inactives que le runner doit maintenir. La valeur que vous attribuez dépend de votre cas d'utilisation.

Commencez par attribuer un nombre raisonnablement petit de machines en état inactif. Ensuite, faites-les s'ajuster automatiquement à un nombre plus grand, en fonction de l'utilisation actuelle. Pour ce faire, utilisez le paramètre expérimental `IdleScaleFactor`.

> [!warning]
> `IdleScaleFactor` est en interne une valeur `float64` et requiert l'utilisation du format flottant, par exemple : `0.0`, `1.0`, ou `1.5`. Si un format entier est utilisé (par exemple `IdleScaleFactor = 1`), le processus de runner échoue avec l'erreur : `FATAL: Service run failed   error=toml: cannot load TOML value of type int64 into a Go float`.

Lorsque vous utilisez ce paramètre, GitLab Runner tente de maintenir un nombre défini de machines en état inactif. Cependant, ce nombre n'est plus statique. Au lieu d'utiliser `IdleCount`, GitLab Runner compte les machines en cours d'utilisation et définit la capacité inactive souhaitée comme un facteur de ce nombre.

S'il n'y a pas de machines utilisées, `IdleScaleFactor` évalue à aucune machine inactive à maintenir. Si `IdleCount` est supérieur à `0` (et seulement dans ce cas, `IdleScaleFactor` est applicable), le runner ne demande pas de jobs s'il n'y a pas de machines inactives pour les traiter. Sans nouveaux jobs, le nombre de machines utilisées n'augmenterait pas, donc `IdleScaleFactor` évaluerait constamment à `0`. Cela bloquerait le runner dans un état inutilisable.

C'est pourquoi nous avons introduit le deuxième paramètre : `IdleCountMin`. Il définit le nombre minimum de machines inactives qui doivent être maintenues quelle que soit la valeur évaluée par `IdleScaleFactor`. **Le paramètre ne peut pas être défini à une valeur inférieure à un si `IdleScaleFactor` est utilisé. GitLab Runner définit automatiquement `IdleCountMin` à un**.

Vous pouvez également utiliser `IdleCountMin` pour définir le nombre minimum de machines inactives qui doivent toujours être disponibles. Cela permet aux nouveaux jobs entrant dans la file d'attente de démarrer rapidement. Comme pour `IdleCount`, la valeur que vous attribuez dépend de votre cas d'utilisation.

Par exemple :

```toml
concurrent=200

[[runners]]
  limit = 200
  [runners.machine]
    IdleCount = 100
    IdleCountMin = 10
    IdleScaleFactor = 1.1
```

Dans ce cas, lorsque le runner approche du point de décision, il vérifie combien de machines sont en cours d'utilisation. Par exemple, s'il y a cinq machines inactives et dix machines en cours d'utilisation. En le multipliant par `IdleScaleFactor`, le runner décide qu'il devrait avoir 11 machines inactives. Ainsi, 6 autres sont créées.

Si vous avez 90 machines inactives et 100 machines en cours d'utilisation, en se basant sur `IdleScaleFactor`, GitLab Runner constate qu'il devrait avoir `100 * 1.1 = 110` machines inactives. Il recommence donc à en créer de nouvelles. Cependant, lorsqu'il atteint le nombre de `100` machines inactives, il arrête d'en créer davantage car il s'agit de la limite supérieure définie par `IdleCount`.

Si les 100 machines inactives en cours d'utilisation tombent à 20, le nombre souhaité de machines inactives est `20 * 1.1 = 22`. GitLab Runner commence à arrêter les machines. Comme décrit ci-dessus, GitLab Runner supprime les machines qui ne sont pas utilisées pendant `IdleTime`. Par conséquent, la suppression d'un trop grand nombre de VM inactives est effectuée de manière agressive.

Si le nombre de machines inactives tombe à 0, le nombre souhaité de machines inactives est `0 * 1.1 = 0`. Cependant, cela est inférieur au paramètre `IdleCountMin` défini, donc le runner commence à supprimer les VM inactives jusqu'à ce qu'il reste 10 VM. Après ce point, la réduction s'arrête et le runner maintient 10 machines en état inactif.

## Configurer les périodes de mise à l'échelle automatique {#configure-autoscaling-periods}

La mise à l'échelle automatique peut être configurée pour avoir des valeurs différentes en fonction de la période. Les organisations peuvent avoir des périodes régulières pendant lesquelles des pics de jobs sont exécutés, et d'autres périodes avec peu ou pas de jobs. Par exemple, la plupart des entreprises commerciales travaillent du lundi au vendredi à des horaires fixes, comme de 10h à 18h. Les nuits et les week-ends pour le reste de la semaine, et pendant les week-ends, aucun pipeline n'est démarré.

Ces périodes peuvent être configurées à l'aide des sections `[[runners.machine.autoscaling]]`. Chacune d'elles prend en charge la définition de `IdleCount` et `IdleTime` en fonction d'un ensemble de `Periods`.

### Fonctionnement des périodes de mise à l'échelle automatique {#how-autoscaling-periods-work}

Dans les paramètres `[runners.machine]`, vous pouvez ajouter plusieurs sections `[[runners.machine.autoscaling]]`, chacune avec ses propres propriétés `IdleCount`, `IdleTime`, `Periods` et `Timezone`. Une section doit être définie pour chaque configuration, en procédant dans l'ordre du scénario le plus général au plus spécifique.

Toutes les sections sont analysées. La dernière à correspondre à l'heure actuelle est active. Si aucune ne correspond, les valeurs de la racine de `[runners.machine]` sont utilisées.

Par exemple :

```toml
[runners.machine]
  MachineName = "auto-scale-%s"
  MachineDriver = "google"
  IdleCount = 10
  IdleTime = 1800
  [[runners.machine.autoscaling]]
    Periods = ["* * 9-17 * * mon-fri *"]
    IdleCount = 50
    IdleTime = 3600
    Timezone = "UTC"
  [[runners.machine.autoscaling]]
    Periods = ["* * * * * sat,sun *"]
    IdleCount = 5
    IdleTime = 60
    Timezone = "UTC"
```

Dans cette configuration, chaque jour de la semaine entre 9h et 16h59 UTC, les machines sont sur-provisionnées pour gérer le trafic important pendant les heures de travail. Le week-end, `IdleCount` tombe à 5 pour tenir compte de la baisse du trafic. Le reste du temps, les valeurs sont prises par défaut depuis la racine - `IdleCount = 10` et `IdleTime = 1800`.

> [!note]
> La 59e seconde de la dernière minute de toute période que vous spécifiez n'est pas considérée comme faisant partie de la période. Pour plus d'informations, consultez le [ticket #2170](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2170).

Vous pouvez spécifier le `Timezone` d'une période, par exemple `"Australia/Sydney"`. Si vous ne le faites pas, le paramètre système de la machine hôte de chaque runner est utilisé. Ce paramètre par défaut peut être indiqué explicitement comme `Timezone = "Local"`.

Plus d'informations sur la syntaxe des sections `[[runner.machine.autoscaling]]` se trouvent dans [GitLab Runner - Configuration avancée - La section `[runners.machine]`](advanced-configuration.md#the-runnersmachine-section).

## Mise en cache distribuée des runners {#distributed-runners-caching}

> [!note]
> Consultez comment [utiliser un cache distribué](speed_up_job_execution.md#use-a-distributed-cache).

Pour accélérer vos jobs, GitLab Runner fournit un [mécanisme de cache](https://docs.gitlab.com/ci/yaml/#cache) où des répertoires et/ou fichiers sélectionnés sont sauvegardés et partagés entre les jobs successifs.

Ce mécanisme fonctionne bien lorsque les jobs sont exécutés sur le même hôte. Cependant, lorsque vous commencez à utiliser la fonctionnalité de mise à l'échelle automatique de GitLab Runner, la plupart de vos jobs s'exécutent sur un hôte nouveau (ou presque nouveau). Ce nouvel hôte exécute chaque job dans un nouveau conteneur Docker. Dans ce cas, vous ne pouvez pas tirer parti de la fonctionnalité de cache.

Pour surmonter ce problème, conjointement avec la fonctionnalité de mise à l'échelle automatique, la fonctionnalité de cache distribué des runners a été introduite.

Cette fonctionnalité utilise un serveur de stockage d'objets configuré pour partager le cache entre les hôtes Docker utilisés. GitLab Runner interroge le serveur et télécharge l'archive pour restaurer le cache, ou la téléverse pour archiver le cache.

Pour activer la mise en cache distribuée, vous devez la définir dans `config.toml` en utilisant la [directive `[runners.cache]`](advanced-configuration.md#the-runnerscache-section) :

```toml
[[runners]]
  limit = 10
  executor = "docker+machine"
  [runners.cache]
    Type = "s3"
    Path = "path/to/prefix"
    Shared = false
    [runners.cache.s3]
      ServerAddress = "s3.example.com"
      AccessKey = "access-key"
      SecretKey = "secret-key"
      BucketName = "runner"
      Insecure = false
```

Dans l'exemple ci-dessus, les URLs S3 suivent la structure `http(s)://<ServerAddress>/<BucketName>/<Path>/runner/<runner-id>/project/<id>/<cache-key>`.

Pour partager le cache entre deux runners ou plus, définissez l'indicateur `Shared` sur true. Cet indicateur supprime le token du runner de l'URL (`runner/<runner-id>`) et tous les runners configurés partagent le même cache. Vous pouvez également définir `Path` pour séparer les caches entre les runners lorsque le partage de cache est activé.

## Mise en miroir distribuée du registre de conteneurs {#distributed-container-registry-mirroring}

Pour accélérer les jobs exécutés dans des conteneurs Docker, vous pouvez utiliser le [service de mise en miroir du registre Docker](https://docs.docker.com/retired/#registry-now-cncf-distribution). Ce service fournit un proxy entre vos machines Docker et tous les registres utilisés. Les images sont téléchargées une seule fois par le miroir de registre. Sur chaque nouvel hôte, ou sur un hôte existant où l'image n'est pas disponible, l'image est téléchargée depuis le miroir de registre configuré.

À condition que le miroir existe dans le réseau LAN de vos machines Docker, l'étape de téléchargement des images devrait être beaucoup plus rapide sur chaque hôte.

Pour configurer la mise en miroir du registre Docker, vous devez ajouter `MachineOptions` à la configuration dans `config.toml` :

```toml
[[runners]]
  limit = 10
  executor = "docker+machine"
  [runners.machine]
    (...)
    MachineOptions = [
      (...)
      "engine-registry-mirror=http://10.11.12.13:12345"
    ]
```

Où `10.11.12.13:12345` est l'adresse IP et le port sur lesquels votre miroir de registre écoute les connexions provenant du service Docker. Il doit être accessible pour chaque hôte créé par Docker Machine.

Pour en savoir plus sur la façon d'[utiliser un proxy pour les conteneurs](speed_up_job_execution.md#use-a-proxy-for-containers).

## Exemple complet de `config.toml` {#a-complete-example-of-configtoml}

Le `config.toml` ci-dessous utilise le [pilote Docker Machine `google`](https://github.com/docker/docs/blob/173d3c65f8e7df2a8c0323594419c18086fc3a30/machine/drivers/gce.md) :

```toml
concurrent = 50   # All registered runners can run up to 50 concurrent jobs

[[runners]]
  url = "https://gitlab.com"
  token = "RUNNER_TOKEN"             # Note this is different from the registration token used by `gitlab-runner register`
  name = "autoscale-runner"
  executor = "docker+machine"        # This runner is using the 'docker+machine' executor
  limit = 10                         # This runner can execute up to 10 jobs (created machines)
  [runners.docker]
    image = "ruby:3.3"               # The default image used for jobs is 'ruby:3.3'
  [runners.machine]
    IdleCount = 5                    # There must be 5 machines in Idle state - when Off Peak time mode is off
    IdleTime = 600                   # Each machine can be in Idle state up to 600 seconds (after this it will be removed) - when Off Peak time mode is off
    MaxBuilds = 100                  # Each machine can handle up to 100 jobs in a row (after this it will be removed)
    MachineName = "auto-scale-%s"    # Each machine will have a unique name ('%s' is required)
    MachineDriver = "google" # Refer to Docker Machine docs on how to authenticate: https://docs.docker.com/machine/drivers/gce/#credentials
    MachineOptions = [
      "google-project=GOOGLE-PROJECT-ID",
      "google-zone=GOOGLE-ZONE", # e.g. 'us-west1'
      "google-machine-type=GOOGLE-MACHINE-TYPE", # e.g. 'n1-standard-8'
      "google-machine-image=ubuntu-os-cloud/global/images/family/ubuntu-1804-lts",
      "google-username=root",
      "google-use-internal-ip",
      "engine-registry-mirror=https://mirror.gcr.io"
    ]
    [[runners.machine.autoscaling]]  # Define periods with different settings
      Periods = ["* * 9-17 * * mon-fri *"] # Every workday between 9 and 17 UTC
      IdleCount = 50
      IdleCountMin = 5
      IdleScaleFactor = 1.5 # Means that current number of Idle machines will be 1.5*in-use machines,
                            # no more than 50 (the value of IdleCount) and no less than 5 (the value of IdleCountMin)
      IdleTime = 3600
      Timezone = "UTC"
    [[runners.machine.autoscaling]]
      Periods = ["* * * * * sat,sun *"] # During the weekends
      IdleCount = 5
      IdleTime = 60
      Timezone = "UTC"
  [runners.cache]
    Type = "s3"
    [runners.cache.s3]
      ServerAddress = "s3.eu-west-1.amazonaws.com"
      AccessKey = "AMAZON_S3_ACCESS_KEY"
      SecretKey = "AMAZON_S3_SECRET_KEY"
      BucketName = "runner"
      Insecure = false
```

Le paramètre `MachineOptions` contient des options à la fois pour le pilote `google` que Docker Machine utilise pour créer des machines sur Google Compute Engine et pour Docker Machine lui-même (`engine-registry-mirror`).
