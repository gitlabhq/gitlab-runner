---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Métriques Prometheus.
title: "Surveiller l'utilisation de GitLab Runner"
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner peut être surveillé à l'aide de [Prometheus](https://prometheus.io).

## Métriques Prometheus intégrées {#embedded-prometheus-metrics}

GitLab Runner inclut des métriques Prometheus natives, que vous pouvez exposer à l'aide d'un serveur HTTP intégré sur le chemin `/metrics`. Le serveur - s'il est activé - peut être collecté par le système de surveillance Prometheus ou accessible avec tout autre client HTTP.

Les informations exposées comprennent :

- Les métriques de logique métier du runner (par exemple, le nombre de jobs en cours d'exécution à ce moment)
- Les métriques de processus spécifiques à Go (par exemple, les statistiques de garbage collection, les goroutines et les memstats)
- Les métriques de processus générales (utilisation de la mémoire, utilisation du CPU, utilisation des descripteurs de fichiers, etc.)
- Les informations sur la version de build

Le format des métriques est documenté dans la spécification [Exposition formats](https://prometheus.io/docs/instrumenting/exposition_formats/) de Prometheus.

Ces métriques sont destinées à permettre aux opérateurs de surveiller et d'obtenir des informations sur vos runners. Par exemple, vous pourriez vouloir savoir si une augmentation de la charge moyenne sur l'hôte du runner est liée à une augmentation des jobs traités. Ou peut-être gérez-vous un cluster de machines et souhaitez-vous suivre les tendances de build afin de pouvoir apporter des modifications à votre infrastructure.

### En savoir plus sur Prometheus {#learning-more-about-prometheus}

Pour configurer le serveur Prometheus afin de collecter cet endpoint HTTP et d'utiliser les métriques collectées, consultez le guide [getting started](https://prometheus.io/docs/prometheus/latest/getting_started/) de Prometheus. Pour plus d'informations sur la configuration de Prometheus, consultez la section [configuration](https://prometheus.io/docs/prometheus/latest/configuration/configuration/). Pour plus d'informations sur la configuration des alertes, consultez [alerting rules](https://prometheus.io/docs/prometheus/latest/configuration/alerting_rules/) et [Alertmanager](https://prometheus.io/docs/alerting/latest/alertmanager/).

## Métriques disponibles {#available-metrics}

Pour obtenir la liste complète de toutes les métriques disponibles, utilisez `curl` sur l'endpoint des métriques une fois celui-ci configuré et activé. Par exemple, pour un runner local configuré avec le port d'écoute `9252` :

```shell
$ curl -s "http://localhost:9252/metrics" | grep -E "# HELP"

# HELP gitlab_runner_api_request_statuses_total The total number of api requests, partitioned by runner, endpoint and status.
# HELP gitlab_runner_autoscaling_machine_creation_duration_seconds Histogram of machine creation time.
# HELP gitlab_runner_autoscaling_machine_states The current number of machines per state in this provider.
# HELP gitlab_runner_concurrent The current value of concurrent setting
# HELP gitlab_runner_errors_total The number of caught errors.
# HELP gitlab_runner_limit The current value of limit setting
# HELP gitlab_runner_request_concurrency The current number of concurrent requests for a new job
# HELP gitlab_runner_request_concurrency_exceeded_total Count of excess requests above the configured request_concurrency limit
# HELP gitlab_runner_version_info A metric with a constant '1' value labeled by different build stats fields.
...
```

La liste inclut les [métriques de processus spécifiques à Go](https://github.com/prometheus/client_golang/blob/v1.19.0/prometheus/go_collector.go). Pour obtenir la liste des métriques disponibles qui n'incluent pas les processus spécifiques à Go, consultez [Monitoring runners](../fleet_scaling/_index.md#monitoring-runners).

## Endpoints HTTP `pprof` {#pprof-http-endpoints}

L'état interne du processus GitLab Runner via les métriques est précieux, mais dans certains cas, vous devez examiner le processus en cours d'exécution en temps réel. C'est pourquoi nous avons introduit les endpoints HTTP `pprof`.

Les endpoints `pprof` sont disponibles via un serveur HTTP intégré sur le chemin `/debug/pprof/`.

Vous pouvez en savoir plus sur l'utilisation de `pprof` dans sa [documentation](https://pkg.go.dev/net/http/pprof).

## Configuration du serveur HTTP de métriques {#configuration-of-the-metrics-http-server}

> [!note]
> Le serveur de métriques exporte des données sur l'état interne du processus GitLab Runner et ne doit pas être accessible publiquement !

Configurez le serveur HTTP de métriques en utilisant l'une des méthodes suivantes :

- Utilisez l'option de configuration globale `listen_address` dans le fichier `config.toml`.
- Utilisez l'option de ligne de commande `--listen-address` pour la commande `run`.
- Pour les runners utilisant Helm chart, dans le fichier `values.yaml` :

  1. Configurez l'option `metrics` :

     ```yaml
     ## Configure integrated Prometheus metrics exporter
     ##
     ## ref: https://docs.gitlab.com/runner/monitoring/#configuration-of-the-metrics-http-server
     ##
     metrics:
       enabled: true

       ## Define a name for the metrics port
       ##
       portName: metrics

       ## Provide a port number for the integrated Prometheus metrics exporter
       ##
       port: 9252

       ## Configure a prometheus-operator serviceMonitor to allow automatic detection of
       ## the scraping target. Requires enabling the service resource below.
       ##
       serviceMonitor:
         enabled: true

         ...
     ```

  1. Configurez le moniteur `service` pour récupérer les `metrics` configurées :

     ```yaml
     ## Configure a service resource to allow scraping metrics by using
     ## prometheus-operator serviceMonitor
     service:
       enabled: true

       ## Provide additional labels for the service
       ##
       labels: {}

       ## Provide additional annotations for the service
       ##
       annotations: {}

       ...
     ```

Si vous ajoutez l'adresse à votre fichier `config.toml`, vous devez redémarrer le processus du runner pour démarrer le serveur HTTP de métriques.

Dans les deux cas, l'option accepte une chaîne au format `[host]:<port>`, où :

- `host` peut être une adresse IP ou un nom d'hôte,
- `port` est un port TCP valide ou un nom de service symbolique (comme `http`). Vous devriez utiliser le port `9252` qui est déjà [alloué dans Prometheus](https://github.com/prometheus/prometheus/wiki/Default-port-allocations).

Si l'adresse d'écoute ne contient pas de port, la valeur par défaut est `9252`.

Exemples d'adresses :

- `:9252` écoute sur toutes les interfaces sur le port `9252`.
- `localhost:9252` écoute sur l'interface de loopback sur le port `9252`.
- `[2001:db8::1]:http` écoute sur l'adresse IPv6 `[2001:db8::1]` sur le port HTTP `80`.

N'oubliez pas que pour écouter sur des ports inférieurs à `1024`, du moins sur les systèmes Linux/Unix, vous devez disposer de privilèges root/administrateur.

Le serveur HTTP est ouvert sur le `host:port` sélectionné **sans aucune autorisation**. Si vous liez le serveur de métriques à une interface publique, utilisez votre pare-feu pour limiter l'accès ou ajoutez un proxy HTTP pour l'autorisation et le contrôle d'accès.

## Surveiller les GitLab Runners gérés par Operator {#monitor-operator-managed-gitlab-runners}

Les GitLab Runners gérés par GitLab Runner Operator utilisent le même serveur de métriques Prometheus intégré que les instances GitLab Runner autonomes. Le serveur de métriques est préconfiguré avec `listenAddr` défini sur `[::]:9252`, qui écoute sur toutes les interfaces IPv6 et IPv4 sur le port `9252`.

### Exposer le port des métriques {#expose-metrics-port}

Pour activer la surveillance et la collecte des métriques pour les GitLab Runners gérés par GitLab Runner Operator, consultez [Surveiller les GitLab Runners gérés par Operator](#monitor-operator-managed-gitlab-runners).

#### Configurer le port des métriques {#configure-the-metrics-port}

Ajoutez le patch suivant au champ `podSpec` dans votre configuration de runner :

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: gitlab-runner
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  buildImage: alpine
  podSpec:
    name: "metrics-config"
    patch: |
      {
        "containers": [
          {
            "name": "runner",
            "ports": [
              {
                "name": "metrics",
                "containerPort": 9252,
                "protocol": "TCP"
              }
            ]
          }
        ]
      }
    patchType: "strategic"
```

Cette configuration :

- `name` :  Attribue un nom au `PodSpec` personnalisé à des fins d'identification.
- `patch` :  Définit le patch JSON à appliquer au `PodSpec`, expose le port `9252` sur le conteneur du runner.
- `patchType` :  Utilise la stratégie de fusion `strategic` (par défaut) pour appliquer le patch.
- `port` :  Nommé `metrics` pour une identification facile dans les services Kubernetes.

#### Configurer la collecte Prometheus {#configure-prometheus-scraping}

Pour les environnements utilisant Prometheus Operator, créez une ressource `PodMonitor` pour collecter directement les métriques depuis les pods de runner :

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PodMonitor
metadata:
  name: gitlab-runner-metrics
  namespace: kube-prometheus-stack
  labels:
    release: kube-prometheus-stack
spec:
  selector:
    matchLabels:
      app.kubernetes.io/component: runner
  namespaceSelector:
    matchNames:
      - gitlab-runner-system
  podMetricsEndpoints:
    - port: metrics
      interval: 10s
      path: /metrics
```

Appliquez la configuration `PodMonitor` :

```shell
kubectl apply -f gitlab-runner-podmonitor.yaml
```

La configuration `PodMonitor` :

- `selector` :  Correspond aux pods avec le label `app.kubernetes.io/component: runner`.
- `namespaceSelector` :  Limite la collecte à l'espace de nommage `gitlab-runner-system`.
- `podMetricsEndpoints` :  Définit le port des métriques, l'intervalle de collecte et le chemin.

#### Ajouter l'identification du runner aux métriques {#add-runner-identification-to-metrics}

Pour ajouter l'identification du runner à toutes les métriques exportées, incluez la configuration de relabel dans le `PodMonitor` :

```yaml
podMetricsEndpoints:
  - port: metrics
    interval: 10s
    path: /metrics
    relabelings:
      - sourceLabels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
        targetLabel: runner_name
```

La configuration de relabel :

- Extrait le label `app.kubernetes.io/name` de chaque pod de runner (défini automatiquement par GitLab Runner Operator).
- L'ajoute en tant que label `runner_name` à toutes les métriques de ce pod.
- Permet de filtrer et d'agréger les métriques par instances de runner spécifiques.

Voici un exemple de métriques avec identification du runner :

```prometheus
gitlab_runner_concurrent{runner_name="my-gitlab-runner"} 10
gitlab_runner_jobs_running_total{runner_name="my-gitlab-runner"} 3
```

#### Configuration directe de collecte Prometheus {#direct-prometheus-scrape-configuration}

Si vous n'utilisez pas Prometheus Operator, vous pouvez ajouter la configuration de relabel directement dans la configuration de collecte Prometheus :

```yaml
scrape_configs:
  - job_name: 'gitlab-runner-operator'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - gitlab-runner-system
    relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
        target_label: runner_name
    metrics_path: /metrics
    scrape_interval: 10s
```

Cette configuration :

- Utilise la découverte de service Kubernetes pour trouver les pods dans l'espace de nommage `gitlab-runner-system`.
- Extrait le label `app.kubernetes.io/name` et l'ajoute en tant que `runner_name` aux métriques.

## Surveiller GitLab Runner avec des exécuteurs autres que Kubernetes {#monitor-gitlab-runner-with-executors-other-than-kubernetes}

Pour les déploiements GitLab Runner avec des exécuteurs autres que Kubernetes, vous pouvez ajouter l'identification du runner via des labels externes dans votre configuration Prometheus.

### Configuration statique avec des labels externes {#static-configuration-with-external-labels}

Configurez Prometheus pour collecter vos instances GitLab Runner et ajoutez des labels d'identification :

```yaml
scrape_configs:
  - job_name: 'gitlab-runner'
    static_configs:
      - targets: ['runner1.example.com:9252']
        labels:
          runner_name: 'production-runner-1'
      - targets: ['runner2.example.com:9252']
        labels:
          runner_name: 'staging-runner-1'
    metrics_path: /metrics
    scrape_interval: 30s
```

Cette configuration ajoute l'identification du runner à vos métriques :

```prometheus
gitlab_runner_concurrent{runner_name="production-runner-1"} 10
gitlab_runner_jobs_running_total{runner_name="staging-runner-1"} 3
```

Cette configuration vous permet de :

- Filtrer les métriques par instances de runner spécifiques.
- Créer des tableaux de bord et des alertes spécifiques aux runners.
- Suivre les performances sur différents déploiements de runner.

### Métriques disponibles pour les GitLab Runners gérés par Operator {#available-metrics-for-operator-managed-gitlab-runners}

Les GitLab Runners gérés par GitLab Runner Operator exposent les mêmes métriques que les déploiements GitLab Runner autonomes. Pour afficher toutes les métriques disponibles, utilisez `kubectl` pour accéder à l'endpoint des métriques :

```shell
kubectl port-forward pod/<gitlab-runner-pod-name> 9252:9252
curl -s "http://localhost:9252/metrics" | grep -E "# HELP"
```

Pour obtenir la liste complète des métriques disponibles, consultez [Métriques disponibles](#available-metrics).

### Considérations de sécurité pour les GitLab Runners gérés par Operator {#security-considerations-for-operator-managed-gitlab-runners}

Lorsque vous configurez la collecte des métriques pour les GitLab Runners gérés par GitLab Runner Operator :

- Utilisez les `NetworkPolicies` Kubernetes pour restreindre l'accès aux systèmes de surveillance autorisés.
- Envisagez d'utiliser le chiffrement TLS `mutual` pour la collecte des métriques dans les environnements de production.

### Résoudre les problèmes de surveillance des GitLab Runners gérés par Operator {#troubleshooting-operator-managed-gitlab-runner-monitoring}

#### Endpoint de métriques inaccessible {#metrics-endpoint-not-accessible}

Si vous ne pouvez pas accéder à l'endpoint des métriques :

1. Vérifiez que la spécification du pod inclut la configuration du port des métriques.
1. Assurez-vous que le pod du runner est en cours d'exécution et sain :

   ```shell
   kubectl get pods -l app.kubernetes.io/component=runner -n gitlab-runner-system
   kubectl describe pod <runner-pod-name> -n gitlab-runner-system
   ```

1. Testez la connectivité vers l'endpoint des métriques :

   ```shell
   kubectl port-forward pod/<runner-pod-name> 9252:9252 -n gitlab-runner-system
   curl "http://localhost:9252/metrics"
   ```

#### Métriques manquantes dans Prometheus {#missing-metrics-in-prometheus}

Si les métriques n'apparaissent pas dans Prometheus :

1. Vérifiez que le `PodMonitor` est correctement configuré et appliqué.
1. Vérifiez que les sélecteurs d'espace de nommage et de label correspondent à vos pods de runner.
1. Examinez les journaux Prometheus pour détecter les erreurs de collecte.
1. Validez que le `PodMonitor` est détectable par Prometheus Operator :

   ```shell
   kubectl get podmonitor gitlab-runner-metrics -n kube-prometheus-stack
   kubectl describe podmonitor gitlab-runner-metrics -n kube-prometheus-stack
   ```
