---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: "Installer GitLab Runner dans Kubernetes à l'aide du chart Helm GitLab."
title: Chart Helm GitLab Runner
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Le chart Helm GitLab Runner est la méthode officielle pour déployer une instance GitLab Runner dans votre cluster Kubernetes. Ce chart configure GitLab Runner pour :

- S'exécuter à l'aide de l'[exécuteur Kubernetes](../executors/kubernetes/_index.md) pour GitLab Runner.
- Provisionner un nouveau pod dans l'espace de nommage spécifié pour chaque nouveau job CI/CD.

## Configurer GitLab Runner avec le chart Helm {#configure-gitlab-runner-with-the-helm-chart}

Stockez vos modifications de configuration GitLab Runner dans `values.yaml`. Pour obtenir de l'aide sur la configuration de ce fichier, consultez :

- La configuration par défaut de [`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml) dans le dépôt du chart.
- La documentation Helm pour [Values Files](https://helm.sh/docs/chart_template_guide/values_files/), qui explique comment votre fichier de valeurs remplace les valeurs par défaut.

Pour que GitLab Runner fonctionne correctement, vous devez définir ces valeurs dans votre fichier de configuration :

- `gitlabUrl` :  L'URL complète du serveur GitLab (comme `https://gitlab.example.com`) auprès duquel enregistrer le runner.
- `rbac: { create: true }` :  Créer des règles RBAC (contrôle d'accès basé sur les rôles) pour que GitLab Runner puisse créer des pods dans lesquels exécuter les jobs.
  - Si vous souhaitez utiliser un compte de service existant `serviceAccount`, ajoutez le nom de votre compte de service dans `rbac` :

    ```yaml
    rbac:
      create: false
    serviceAccount:
      create: false
      name: your-service-account
    ```

  - Pour en savoir plus sur les permissions minimales requises par `serviceAccount`, consultez [Configurer les permissions API du runner](../executors/kubernetes/_index.md#configure-runner-api-permissions).
- `runnerToken` :  Le jeton d'authentification obtenu lorsque vous [créez un runner dans l'interface utilisateur GitLab](https://docs.gitlab.com/ci/runners/runners_scope/#create-an-instance-runner-with-a-runner-authentication-token).
  - Définissez ce jeton directement ou stockez-le dans un secret.

D'autres [paramètres de configuration facultatifs](kubernetes_helm_chart_configuration.md) sont disponibles.

Vous êtes maintenant prêt à [installer GitLab Runner](#install-gitlab-runner-with-the-helm-chart) !

## Installer GitLab Runner avec le chart Helm {#install-gitlab-runner-with-the-helm-chart}

Prérequis :

- L'API de votre serveur GitLab est accessible depuis le cluster.
- Kubernetes 1.4 ou version ultérieure, avec les API bêta activées.
- L'interface CLI `kubectl` est installée localement et authentifiée pour le cluster.
- Le [client Helm](https://helm.sh/docs/using_helm/#installing-the-helm-client) est installé localement sur votre machine.
- Vous avez défini toutes les [valeurs requises dans `values.yaml`](#configure-gitlab-runner-with-the-helm-chart).

Pour installer GitLab Runner à partir du chart Helm :

1. Ajoutez le dépôt Helm GitLab :

   ```shell
   helm repo add gitlab https://charts.gitlab.io
   ```

1. Si vous utilisez Helm 2, initialisez Helm avec `helm init`.
1. Vérifiez les versions de GitLab Runner auxquelles vous avez accès :

   ```shell
   helm search repo -l gitlab/gitlab-runner
   ```

1. Si vous ne pouvez pas accéder aux dernières versions de GitLab Runner, mettez à jour le chart avec cette commande :

   ```shell
   helm repo update gitlab
   ```

1. Après avoir [configuré](#configure-gitlab-runner-with-the-helm-chart) GitLab Runner dans votre fichier `values.yaml`, exécutez cette commande en modifiant les paramètres selon vos besoins :

   ```shell
   # For Helm 2
   helm install --namespace <NAMESPACE> --name gitlab-runner -f <CONFIG_VALUES_FILE> gitlab/gitlab-runner

   # For Helm 3
   helm install --namespace <NAMESPACE> gitlab-runner -f <CONFIG_VALUES_FILE> gitlab/gitlab-runner
   ```

   - `<NAMESPACE>` :  L'espace de nommage Kubernetes dans lequel vous souhaitez installer GitLab Runner.
   - `<CONFIG_VALUES_FILE>` :  Le chemin vers le fichier de valeurs contenant votre configuration personnalisée. Pour le créer, consultez [Configurer GitLab Runner avec le chart Helm](#configure-gitlab-runner-with-the-helm-chart).
   - Pour installer une version spécifique du chart Helm GitLab Runner, ajoutez `--version <RUNNER_HELM_CHART_VERSION>` à votre commande `helm install`. Vous pouvez installer n'importe quelle version du chart, mais les versions plus récentes de `values.yml` peuvent être incompatibles avec les versions plus anciennes du chart.

### Vérifier les versions disponibles du chart Helm GitLab Runner {#check-available-gitlab-runner-helm-chart-versions}

Les charts Helm et GitLab Runner ne suivent pas le même versionnement. Pour voir les correspondances de versions entre les deux, exécutez la commande pour votre version de Helm :

```shell
# For Helm 2
helm search -l gitlab/gitlab-runner

# For Helm 3
helm search repo -l gitlab/gitlab-runner
```

Un exemple de sortie :

```plaintext
NAME                  CHART VERSION APP VERSION DESCRIPTION
gitlab/gitlab-runner  0.64.0        16.11.0     GitLab Runner
gitlab/gitlab-runner  0.63.0        16.10.0     GitLab Runner
gitlab/gitlab-runner  0.62.1        16.9.1      GitLab Runner
gitlab/gitlab-runner  0.62.0        16.9.0      GitLab Runner
gitlab/gitlab-runner  0.61.3        16.8.1      GitLab Runner
gitlab/gitlab-runner  0.61.2        16.8.0      GitLab Runner
...
```

## Mettre à niveau GitLab Runner avec le chart Helm {#upgrade-gitlab-runner-with-the-helm-chart}

Prérequis :

- Vous avez installé votre chart GitLab Runner.
- Vous avez mis le runner en pause dans GitLab. Cela évite les problèmes liés aux jobs, tels que les [erreurs d'autorisation lors de leur achèvement](../faq/_index.md#helm-chart-error--unauthorized).
- Vous vous êtes assuré que tous les jobs sont terminés.

Pour modifier votre configuration ou mettre à jour les charts, utilisez `helm upgrade` en modifiant les paramètres selon vos besoins :

```shell
helm upgrade --namespace <NAMESPACE> -f <CONFIG_VALUES_FILE> <RELEASE-NAME> gitlab/gitlab-runner
```

- `<NAMESPACE>` :  L'espace de nommage Kubernetes dans lequel vous avez installé GitLab Runner.
- `<CONFIG_VALUES_FILE>` :  Le chemin vers le fichier de valeurs contenant votre configuration personnalisée. Pour le créer, consultez [Configurer GitLab Runner avec le chart Helm](#configure-gitlab-runner-with-the-helm-chart).
- `<RELEASE-NAME>` :  Le nom que vous avez donné au chart lors de son installation. Dans la section d'installation, l'exemple l'a nommé `gitlab-runner`.
- Pour mettre à jour vers une version spécifique du chart Helm GitLab Runner plutôt que la dernière, ajoutez `--version <RUNNER_HELM_CHART_VERSION>` à votre commande `helm upgrade`.

## Désinstaller GitLab Runner avec le chart Helm {#uninstall-gitlab-runner-with-the-helm-chart}

Pour désinstaller GitLab Runner :

1. Mettez le runner en pause dans GitLab et assurez-vous que tous les jobs sont terminés. Cela évite les problèmes liés aux jobs, tels que les [erreurs d'autorisation lors de l'achèvement](../faq/_index.md#helm-chart-error--unauthorized).
1. Exécutez cette commande en la modifiant selon vos besoins :

   ```shell
   helm delete --namespace <NAMESPACE> <RELEASE-NAME>
   ```

   - `<NAMESPACE>` est l'espace de nommage Kubernetes dans lequel GitLab Runner est installé.
   - `<RELEASE-NAME>` est le nom que vous avez donné au chart lors de son installation. Dans la [section d'installation](#install-gitlab-runner-with-the-helm-chart) de cette page, nous l'avons appelé `gitlab-runner`.
