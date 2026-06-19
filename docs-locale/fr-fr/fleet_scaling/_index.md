---
stage: Verify
group: CI Functions Platform
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "Planifier et exploiter une flotte de runners d'instance ou de groupe"
---

Appliquez ces bonnes pratiques et recommandations lors de la mise à l'échelle d'une flotte de runners dans un modèle de service partagé.

Lorsque vous hébergez une flotte de runners d'instance, vous avez besoin d'une infrastructure bien planifiée qui prend en compte vos :

- Capacité de calcul
- Capacité de stockage
- Bande passante et débit réseau
- Type de jobs (notamment le langage de programmation, la plateforme OS et les bibliothèques dépendantes)

Utilisez ces recommandations pour développer une stratégie de déploiement de GitLab Runner adaptée aux besoins de votre organisation.

## Tenez compte de votre charge de travail et de votre environnement {#consider-your-workload-and-environment}

Avant de déployer des runners, tenez compte de votre charge de travail et des exigences de votre environnement.

- Créez une liste des équipes que vous prévoyez d'intégrer à GitLab.
- Répertoriez les langages de programmation, les frameworks web et les bibliothèques utilisés dans votre organisation. Par exemple, Go, C++, PHP, Java, Python, JavaScript, React, Node.js.
- Estimez le nombre de jobs CI/CD que chaque équipe peut exécuter par heure et par jour.
- Vérifiez si une équipe a des exigences d'environnement de build qui ne peuvent pas être satisfaites en utilisant des conteneurs.
- Vérifiez si une équipe a des exigences d'environnement de build qui sont mieux satisfaites par des runners dédiés à cette équipe.
- Estimez la capacité de calcul dont vous pourriez avoir besoin pour répondre à la demande attendue.

Vous pouvez choisir différentes piles d'infrastructure pour héberger différentes flottes de runners. Par exemple, vous pourriez avoir besoin de déployer certains runners dans le cloud public et d'autres sur site.

Les performances des jobs CI/CD sur la flotte de runners sont directement liées à l'environnement de la flotte. Si vous exécutez un grand nombre de jobs CI/CD gourmands en ressources, il n'est pas recommandé d'héberger la flotte sur une plateforme de calcul partagée.

## Runners, exécuteurs et capacités de mise à l'échelle automatique {#runners-executors-and-autoscaling-capabilities}

L'exécutable `gitlab-runner` exécute vos jobs CI/CD. Chaque runner est un processus isolé qui prend en charge les demandes d'exécution de jobs et les traite selon des configurations prédéfinies. En tant que processus isolé, chaque runner peut créer des « sous-processus » (également appelés « workers ») pour exécuter des jobs.

### Concurrence et limite {#concurrency-and-limit}

- [Concurrence](../configuration/advanced-configuration.md#the-global-section) :  Définit le nombre de jobs pouvant s'exécuter simultanément lorsque vous utilisez tous les runners configurés sur un système hôte.
- [Limite](../configuration/advanced-configuration.md#the-runners-section) :  Définit le nombre de sous-processus qu'un runner peut créer pour exécuter des jobs simultanément.

La limite est différente pour les runners avec mise à l'échelle automatique (comme Docker Machine et Kubernetes) par rapport aux runners sans mise à l'échelle automatique.

- Sur les runners sans mise à l'échelle automatique, `limit` définit la capacité du runner sur un système hôte.
- Sur les runners avec mise à l'échelle automatique, `limit` correspond au nombre de runners que vous souhaitez exécuter au total.

Pour plus d'informations sur la façon dont `concurrency`, `limit` et `request_concurrency` interagissent pour contrôler le flux de jobs, consultez l'[article de la base de connaissances sur le réglage de la concurrence de GitLab Runner](https://support.gitlab.com/hc/en-us/articles/21324350882076-GitLab-Runner-Concurrency-Tuning-Understanding-request-concurrency).

### Configuration de base : un gestionnaire de runner, un runner {#basic-configuration-one-runner-manager-one-runner}

Pour la configuration la plus basique, vous installez le logiciel GitLab Runner sur une architecture de calcul et un système d'exploitation pris en charge. Par exemple, vous pourriez avoir une machine virtuelle (VM) x86-64 exécutant Ubuntu Linux.

Une fois l'installation terminée, vous exécutez la commande d'enregistrement du runner une seule fois et sélectionnez l'exécuteur `shell`. Ensuite, vous modifiez le fichier `config.toml` du runner pour définir la concurrence sur `1`.

```toml
concurrent = 1

[[runners]]
  name = "instance-level-runner-001"
  url = ""
  token = ""
  executor = "shell"
```

Les jobs GitLab CI/CD que ce runner peut traiter sont exécutés directement sur le système hôte où vous avez installé le runner. C'est comme si vous exécutiez vous-même les commandes du job CI/CD dans un terminal. Dans ce cas, comme vous n'avez exécuté la commande d'enregistrement qu'une seule fois, le fichier `config.toml` ne contient qu'une seule section `[[runners]]`. En supposant que vous ayez défini la valeur de concurrence sur `1`, un seul « worker » de runner peut exécuter des jobs CI/CD pour le processus du runner sur ce système.

### Configuration intermédiaire : un gestionnaire de runner, plusieurs runners {#intermediate-configuration-one-runner-manager-multiple-runners}

Vous pouvez également enregistrer plusieurs runners sur la même machine. Dans ce cas, le fichier `config.toml` du runner contient plusieurs sections `[[runners]]`. Si tous les workers de runner supplémentaires utilisent l'exécuteur shell et que vous mettez à jour la valeur du paramètre global `concurrent` à `3`, l'hôte peut exécuter au maximum trois jobs simultanément.

```toml
concurrent = 3

[[runners]]
  name = "instance_level_shell_001"
  url = ""
  token = ""
  executor = "shell"

[[runners]]
  name = "instance_level_shell_002"
  url = ""
  token = ""
  executor = "shell"

[[runners]]
  name = "instance_level_shell_003"
  url = ""
  token = ""
  executor = "shell"

```

Vous pouvez enregistrer de nombreux workers de runner sur la même machine, et chacun est un processus isolé. Les performances des jobs CI/CD pour chaque worker dépendent de la capacité de calcul du système hôte.

### Configuration de mise à l'échelle automatique : un ou plusieurs gestionnaires de runners, plusieurs workers {#autoscaling-configuration-one-or-more-runner-managers-multiple-workers}

Lorsque GitLab Runner est configuré pour la mise à l'échelle automatique, vous pouvez configurer un runner pour qu'il agisse en tant que gestionnaire d'autres runners. Vous pouvez le faire avec les exécuteurs `docker-machine` ou `kubernetes`. Dans ce type de configuration réservée au gestionnaire, l'agent runner n'exécute lui-même aucun job CI/CD.

#### Exécuteur Docker Machine {#docker-machine-executor}

Avec l'[exécuteur Docker Machine](../executors/docker_machine.md) :

- Le gestionnaire de runner provisionne des instances de machines virtuelles à la demande avec Docker.
- Sur ces VMs, GitLab Runner exécute les jobs CI/CD en utilisant une image de conteneur que vous spécifiez dans votre fichier `.gitlab-ci.yml`.
- Vous devriez tester les performances de vos jobs CI/CD sur différents types de machines.
- Vous devriez envisager d'optimiser vos hôtes de calcul en fonction de la vitesse ou du coût.

#### Exécuteur Kubernetes {#kubernetes-executor}

Avec l'[exécuteur Kubernetes](../executors/kubernetes/_index.md) :

- Le gestionnaire de runner provisionne des pods sur le cluster Kubernetes cible.
- Les jobs CI/CD sont exécutés sur chaque pod, qui est composé de plusieurs conteneurs.
- Les pods utilisés pour l'exécution des jobs nécessitent généralement plus de ressources de calcul et de mémoire que le pod qui héberge le gestionnaire de runner.

#### Réutilisation d'une configuration de runner {#reusing-a-runner-configuration}

Chaque gestionnaire de runner associé au même jeton d'authentification de runner se voit attribuer un identifiant `system_id`. Le `system_id` identifie la machine sur laquelle le runner est utilisé. Les runners enregistrés avec le même jeton d'authentification sont regroupés sous une seule entrée de runner par un `system_id.` unique

Le regroupement de runners similaires sous une configuration unique simplifie les opérations de la flotte de runners.

Voici un exemple de scénario dans lequel vous pouvez regrouper des runners similaires sous une configuration unique :

Un administrateur de plateforme doit fournir plusieurs runners avec les mêmes tailles d'instance de machine virtuelle sous-jacente (2 vCPU, 8 Go de RAM) en utilisant le tag `docker-builds-2vCPU-8GB`. Ils veulent au moins deux runners de ce type, soit pour la haute disponibilité, soit pour la mise à l'échelle. Au lieu de créer deux entrées de runner distinctes dans l'interface utilisateur, les administrateurs peuvent créer une configuration de runner pour tous les runners ayant la même taille d'instance de calcul. Ils peuvent réutiliser le jeton d'authentification pour la configuration du runner afin d'enregistrer plusieurs runners. Chaque runner enregistré hérite du tag `docker-builds-2vCPU-8GB`. Pour tous les runners enfants d'une configuration de runner unique, `system_id` agit comme un identifiant unique.

Les runners groupés peuvent être réutilisés pour exécuter différents jobs par plusieurs gestionnaires de runners.

GitLab Runner génère le `system_id` au démarrage ou lors de l'enregistrement de la configuration. Le `system_id` est enregistré dans le fichier `.runner_system_id` dans le même répertoire que le [`config.toml`](../configuration/advanced-configuration.md), et s'affiche dans les job logs et la page d'administration des runners.

##### Génération des identifiants `system_id` {#generating-system_id-identifiers}

Pour générer le `system_id`, GitLab Runner tente de dériver un identifiant système unique à partir d'identifiants matériels (par exemple, `/etc/machine-id` dans certaines distributions Linux). En cas d'échec, GitLab Runner utilise un identifiant aléatoire pour générer le `system_id`.

Le `system_id` possède l'un des préfixes suivants :

- `r_` :  GitLab Runner a attribué un identifiant aléatoire.
- `s_` :  GitLab Runner a attribué un identifiant système unique à partir d'identifiants matériels.

Il est important d'en tenir compte lors de la création d'images de conteneurs, par exemple, afin que le `system_id` ne soit pas codé en dur dans l'image. Si le `system_id` est codé en dur, vous ne pouvez pas distinguer les hôtes exécutant un job donné.

##### Supprimer des runners et des gestionnaires de runners {#delete-runners-and-runner-managers}

Pour supprimer des runners et des gestionnaires de runners enregistrés avec un jeton d'enregistrement de runner (obsolète), utilisez la commande `gitlab-runner unregister`.

Pour supprimer des runners et des gestionnaires de runners créés avec un jeton d'authentification de runner, utilisez l'[interface utilisateur](https://docs.gitlab.com/ci/runners/runners_scope/#delete-instance-runners) ou l'[API](https://docs.gitlab.com/api/runners/#delete-a-runner). Les runners créés avec un jeton d'authentification de runner sont des configurations réutilisables pouvant être utilisées sur plusieurs machines. Si vous utilisez la commande [`gitlab-runner unregister`](../commands/_index.md#gitlab-runner-unregister), seul le gestionnaire de runner est supprimé, pas le runner.

## Configurer les runners d'instance {#configure-instance-runners}

Utiliser des runners d'instance dans une configuration de mise à l'échelle automatique (où un runner agit comme un « gestionnaire de runner ») est un moyen efficace et efficient de démarrer.

La capacité de calcul de la pile d'infrastructure où vous hébergez vos VM ou pods dépend de :

- Les exigences que vous avez identifiées lors de l'analyse de votre charge de travail et de votre environnement.
- La pile technologique que vous utilisez pour héberger votre flotte de runners.

Vous pourriez avoir à ajuster votre capacité de calcul après avoir commencé à exécuter des charges de travail CI/CD et à analyser les performances au fil du temps.

Pour les configurations utilisant des runners d'instance avec un exécuteur de mise à l'échelle automatique, vous devez démarrer avec un minimum de deux gestionnaires de runners.

Le nombre total de gestionnaires de runners dont vous pourriez avoir besoin au fil du temps dépend de :

- Les ressources de calcul de la pile qui héberge les gestionnaires de runners.
- La concurrence que vous choisissez de configurer pour chaque gestionnaire de runner.
- La charge générée par les jobs CI/CD que chaque gestionnaire exécute par heure, par jour et par mois.

Par exemple, sur GitLab.com, nous exécutons sept gestionnaires de runners avec l'exécuteur Docker Machine. Chaque job CI/CD est exécuté dans une VM `n1-standard-1` de Google Cloud Platform (GCP). Avec cette configuration, nous traitons des millions de jobs par mois.

## Surveillance des runners {#monitoring-runners}

Une étape essentielle dans l'exploitation d'une flotte de runners à grande échelle est de configurer et d'utiliser les fonctionnalités de [surveillance des runners](../monitoring/_index.md) incluses dans GitLab.

Le tableau suivant inclut un résumé des métriques de GitLab Runner. La liste n'inclut pas les métriques de processus spécifiques à Go. Pour afficher ces métriques sur un runner, exécutez la commande indiquée dans [les métriques disponibles](../monitoring/_index.md#available-metrics).

| Nom de la métrique                                                    | Description |
|----------------------------------------------------------------|-------------|
| `gitlab_runner_api_request_statuses_total`                     | Le nombre total de requêtes API, partitionné par runner, point de terminaison et statut. |
| `gitlab_runner_autoscaling_machine_creation_duration_seconds`  | Histogramme du temps de création des machines. |
| `gitlab_runner_autoscaling_machine_states`                     | Le nombre de machines par état dans ce fournisseur. |
| `gitlab_runner_concurrent`                                     | La valeur du paramètre de concurrence. |
| `gitlab_runner_errors_total`                                   | Le nombre d'erreurs interceptées. Cette métrique est un compteur qui suit les lignes de log. La métrique inclut le label `level`. Les valeurs possibles sont `warning` et `error`. Si vous prévoyez d'inclure cette métrique, utilisez `rate()` ou `increase()` lors de l'observation. En d'autres termes, si vous constatez que le taux d'avertissements ou d'erreurs augmente, cela pourrait indiquer un problème nécessitant une investigation plus approfondie. |
| `gitlab_runner_jobs`                                           | Indique combien de jobs sont en cours d'exécution (avec différentes portées dans les labels). |
| `gitlab_runner_job_duration_seconds`                           | Histogramme des durées de jobs. |
| `gitlab_runner_job_queue_duration_seconds`                     | Un histogramme représentant la durée de file d'attente des jobs. |
| `gitlab_runner_acceptable_job_queuing_duration_exceeded_total` | Compte le nombre de fois où les jobs dépassent le seuil de temps de mise en file d'attente configuré. |
| `gitlab_runner_job_stage_duration_seconds`                     | Un histogramme représentant la durée des jobs pour chaque étape. Cette métrique est une **high cardinality metric**. Pour plus d'informations, consultez la [section sur les métriques de haute cardinalité](#high-cardinality-metrics). |
| `gitlab_runner_jobs_total`                                     | Affiche le total des jobs exécutés. |
| `gitlab_runner_job_execution_mode_total`                       | Affiche le total des jobs exécutés par mode (`steps` ou `traditional`) et par exécuteur. |
| `gitlab_runner_limit`                                          | La valeur actuelle du paramètre de limite. |
| `gitlab_runner_request_concurrency`                            | Le nombre actuel de requêtes simultanées pour un nouveau job. |
| `gitlab_runner_request_concurrency_exceeded_total`             | Nombre de requêtes excédentaires au-dessus de la limite `request_concurrency` configurée. |
| `gitlab_runner_version_info`                                   | Une métrique avec une valeur constante `1` étiquetée par différents champs de statistiques de build. |
| `process_cpu_seconds_total`                                    | Temps CPU total utilisateur et système passé en secondes. |
| `process_max_fds`                                              | Nombre maximum de descripteurs de fichiers ouverts. |
| `process_open_fds`                                             | Nombre de descripteurs de fichiers ouverts. |
| `process_resident_memory_bytes`                                | Taille de la mémoire résidente en octets. |
| `process_start_time_seconds`                                   | Heure de démarrage du processus, mesurée en secondes depuis l'époque Unix. |
| `process_virtual_memory_bytes`                                 | Taille de la mémoire virtuelle en octets. |
| `process_virtual_memory_max_bytes`                             | Quantité maximale de mémoire virtuelle disponible en octets. |

### Conseils de configuration du tableau de bord Grafana {#grafana-dashboard-configuration-tips}

Dans ce [dépôt public](https://gitlab.com/gitlab-com/runbooks/-/tree/master/dashboards/ci-runners), vous pouvez trouver le code source des tableaux de bord Grafana que nous utilisons pour exploiter la flotte de runners sur GitLab.com.

Nous suivons de nombreuses métriques pour GitLab.com. En tant que grand fournisseur de CI/CD dans le cloud, nous avons besoin de nombreuses vues différentes du système pour pouvoir déboguer les problèmes. Dans la plupart des cas, les flottes de runners autogérées n'ont pas besoin de suivre le volume de métriques que nous suivons avec GitLab.com.

#### Processus de génération des tableaux de bord {#dashboard-generation-process}

Grafana n'accepte que le format JSON, vous devez donc convertir les fichiers `jsonnet` en JSON.

Le [dépôt de runbooks](https://gitlab.com/gitlab-com/runbooks/-/tree/master/dashboards) contient des scripts automatisés uniquement pour l'infrastructure GitLab. Pour générer ces tableaux de bord pour votre propre environnement :

1. Créez des tableaux de bord en utilisant le langage de configuration `jsonnet` (fichiers `.dashboard.jsonnet`).
1. Traitez les fichiers `jsonnet` avec la bibliothèque `jsonnet` pour produire une sortie JSON.
1. Téléchargez les fichiers JSON résultants vers Grafana (via l'API ou l'interface utilisateur).

#### Tableaux de bord de runner disponibles {#available-runner-dashboards}

Voici quelques tableaux de bord essentiels à utiliser pour surveiller votre flotte de runners :

Jobs démarrés sur les runners :

- Affichez une vue d'ensemble du total des jobs exécutés sur votre flotte de runners pour un intervalle de temps sélectionné.
- Affichez les tendances d'utilisation. Vous devriez analyser ce tableau de bord au minimum chaque semaine.
- Corrélez ces données avec des métriques comme la durée des jobs pour déterminer si vous avez besoin de modifications de configuration ou de mises à niveau de capacité pour atteindre vos SLO de performance des jobs CI/CD.

Durée des jobs :

- Analysez les performances et la mise à l'échelle de votre flotte de runners.
- Identifiez les goulets d'étranglement de performance et les opportunités d'optimisation.

Capacité des runners :

- Affichez le nombre de jobs en cours d'exécution divisé par la valeur de la limite ou de la concurrence.
- Déterminez s'il reste de la capacité pour exécuter des jobs supplémentaires.
- Planifiez des mises à niveau de capacité en fonction des tendances d'utilisation.

Les tableaux de bord supplémentaires comprennent :

- Tableau de bord principal (`main.dashboard.jsonnet`) :  Vue d'ensemble de l'infrastructure des runners et des métriques HAProxy.
- Métriques métier (`business-stats.dashboard.jsonnet`) :  Statistiques des jobs, minutes de jobs terminés et saturation des runners.
- Algorithme de mise à l'échelle automatique (`autoscaling-algorithm.dashboard.jsonnet`) :  Visualisation du comportement de mise à l'échelle automatique et des états des machines.
- Vue d'ensemble de la file d'attente (`queuing-overview.dashboard.jsonnet`) :  Profondeur de la file d'attente des jobs et temps d'attente.
- Concurrence des requêtes (`request-concurrency.dashboard.jsonnet`) :  Analyse des requêtes simultanées.
- Déploiement (`deployment.dashboard.jsonnet`) :  Métriques liées au déploiement.
- Tableaux de bord d'incidents :  Tableaux de bord spécialisés pour le dépannage des problèmes de mise à l'échelle automatique, de base de données, d'application et de gestionnaire de runner.

Chaque tableau de bord inclut des descriptions et du contexte dans les fichiers source `jsonnet` pour expliquer quelles métriques sont affichées.

### Variables de template {#template-variables}

Les tableaux de bord utilisent des variables de template Grafana pour créer des modèles de tableaux de bord réutilisables dans différents contextes :

- Environnements :  Par exemple, `production`, `staging`, `development`.
- Étape :  Par exemple, `main`, `canary`.
- Type :  Par exemple, `ci`, `verify`. Varie selon le cas d'utilisation.
- Shard :  Facultatif. Pour les déploiements de runners distribués.

Les organisations qui implémentent ces tableaux de bord doivent ajuster ces variables pour correspondre à la structure de leur propre environnement. Mettez à jour ces variables dans les paramètres du tableau de bord Grafana après l'importation.

### Runners pris en charge {#supported-runners}

Ces tableaux de bord fonctionnent avec tous les types d'exécuteurs GitLab Runner :

- Kubernetes
- Shell
- VM (Docker Machine)
- Windows

La collecte des métriques est indépendante de l'exécuteur et disponible pour tous les types de flottes de runners.

### Personnaliser les tableaux de bord {#customize-dashboards}

Pour modifier les tableaux de bord pour votre environnement :

1. Modifiez les fichiers `.dashboard.jsonnet` dans le répertoire `dashboards/ci-runners/`.
1. Utilisez la syntaxe de la [bibliothèque Grafonnet](https://grafana.github.io/grafonnet-lib/) (basée sur `jsonnet`).
1. Testez les modifications en utilisant le playground :

   ```shell
   ./test-dashboard.sh dashboards/ci-runners/your-dashboard.dashboard.jsonnet
   ```

1. Régénérez et déployez en utilisant `./generate-dashboards.sh`.

Pour plus d'informations, consultez le [guide vidéo sur l'extension des tableaux de bord](https://www.youtube.com/watch?v=yZ2RiY_Akz0).

### Considérations pour la surveillance des runners sur Kubernetes {#considerations-for-monitoring-runners-on-kubernetes}

Pour les flottes de runners hébergées sur des plateformes Kubernetes comme OpenShift, EKS ou GKE, utilisez une approche différente pour configurer les tableaux de bord Grafana.

Sur Kubernetes, les pods d'exécution de jobs CI/CD des runners peuvent être créés et supprimés fréquemment. Dans ces cas, vous devriez prévoir de surveiller le pod du gestionnaire de runner et potentiellement implémenter ce qui suit :

- Jauges :  Affiche l'agrégat de la même métrique provenant de différentes sources.
- Compteurs :  Réinitialisez le compteur lors de l'application des fonctions `rate` ou `increase`.

## Métriques de haute cardinalité {#high-cardinality-metrics}

Certaines métriques peuvent être gourmandes en ressources à ingérer et à stocker en raison de leur haute cardinalité. La haute cardinalité se produit lorsqu'une métrique inclut des labels ayant de nombreuses valeurs possibles, conduisant à un grand nombre de points de données de séries temporelles uniques.

Pour optimiser les performances, ces métriques ne sont pas activées par défaut et peuvent être activées ou désactivées en utilisant le [feature flag FF_EXPORT_HIGH_CARDINALITY_METRICS](../configuration/feature-flags.md).

### Liste des métriques de haute cardinalité {#list-of-high-cardinality-metrics}

- `gitlab_runner_job_stage_duration_seconds` :  Mesure la durée des étapes de job individuelles en secondes. Cette métrique inclut le label `stage`, qui peut avoir les valeurs prédéfinies suivantes :

  - `resolve_secrets`
  - `prepare_executor`
  - `prepare_script`
  - `get_sources`
  - `clear_worktree`
  - `restore_cache`
  - `download_artifacts`
  - `after_script`
  - `step_script`
  - `archive_cache`
  - `archive_cache_on_failure`
  - `upload_artifacts_on_success`
  - `upload_artifacts_on_failure`
  - `cleanup_file_variables`

  De plus, cette liste peut inclure des étapes personnalisées définies par l'utilisateur, telles que `step_run`.

### Gestion des métriques de haute cardinalité {#managing-high-cardinality-metrics}

Vous pouvez contrôler et réduire la cardinalité en utilisant la [configuration de relabeling Prometheus](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#relabel_config) pour supprimer des valeurs de label inutiles ou l'ensemble des métriques.

#### Exemple de configuration pour supprimer des étapes spécifiques {#example-configuration-to-remove-specific-stages}

La configuration suivante supprime toutes les métriques ayant la valeur `prepare_executor` dans le label `stage` :

```yaml
scrape_configs:
  - job_name: 'gitlab_runner_metrics'
    static_configs:
      - targets: ['localhost:9252']
    metric_relabel_configs:
      - source_labels: [__name__, "stage"]
        regex: "gitlab_runner_job_stage_duration_seconds;prepare_executor"
        action: drop
```

#### Exemple pour ne conserver que les étapes pertinentes {#example-to-keep-only-relevant-stages}

La configuration suivante ne conserve que les métriques pour l'étape `step_script` et supprime entièrement les autres métriques :

```yaml
scrape_configs:
  - job_name: 'gitlab_runner_metrics'
    static_configs:
      - targets: ['localhost:9252']
    metric_relabel_configs:
      - source_labels: [__name__, "stage"]
        regex: "gitlab_runner_job_stage_duration_seconds;step_script"
        action: keep
```
