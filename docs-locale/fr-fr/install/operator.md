---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: "Installez GitLab Runner à l'aide de l'opérateur GitLab pour Kubernetes."
title: Installer GitLab Runner Operator
---

## Installer sur Red Hat OpenShift {#install-on-red-hat-openshift}

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Installez GitLab Runner sur Red Hat OpenShift v4 et versions ultérieures à l'aide du [GitLab Runner Operator](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator) depuis le canal stable d'OperatorHub dans la console web d'OpenShift. Une fois installé, vous pouvez exécuter vos jobs GitLab CI/CD à l'aide de l'instance GitLab Runner nouvellement déployée. Chaque job CI/CD s'exécute dans un pod séparé.

### Prérequis {#prerequisites}

- Cluster OpenShift 4.x avec des privilèges d'administrateur
- Token d'enregistrement de GitLab Runner

### Installer l'opérateur OpenShift {#install-the-openshift-operator}

Vous devez d'abord installer l'opérateur OpenShift.

1. Ouvrez l'interface utilisateur d'OpenShift et connectez-vous en tant qu'utilisateur disposant de privilèges d'administrateur.
1. Dans le volet gauche, sélectionnez **Operators**, puis **OperatorHub**.
1. Dans le volet principal, sous **All Items**, recherchez le mot-clé `GitLab Runner`.

   ![GitLab Operator](img/openshift_allitems_v13_3.png)

1. Pour procéder à l'installation, sélectionnez le GitLab Runner Operator.
1. Sur la page de résumé de GitLab Runner Operator, sélectionnez **Installer**.
1. Sur la page Install Operator :
   1. Sous **Update Channel**, sélectionnez **stable**.
   1. Sous **Installed Namespace**, sélectionnez l'espace de nommage souhaité et sélectionnez **Installer**.

   ![GitLab Operator Install Page](img/openshift_installoperator_v13_3.png)

Sur la page Installed Operators, lorsque l'opérateur GitLab est prêt, le statut passe à **Réussi**.

![GitLab Operator Install Status](img/openshift_success_v13_3.png)

## Installer sur Kubernetes {#install-on-kubernetes}

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Installez GitLab Runner sur Kubernetes v1.21 et versions ultérieures à l'aide du [GitLab Runner Operator](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator) depuis le canal stable d'[OperatorHub.io](https://operatorhub.io/operator/gitlab-runner-operator). Une fois installé, vous pouvez exécuter vos jobs GitLab CI/CD à l'aide de l'instance GitLab Runner nouvellement déployée. Chaque job CI/CD s'exécute dans un pod séparé.

### Prérequis {#prerequisites-1}

- Kubernetes v1.21 et versions ultérieures
- Cert manager v1.7.1

### Installer l'opérateur Kubernetes {#install-the-kubernetes-operator}

Suivez les instructions sur [OperatorHub.io](https://operatorhub.io/operator/gitlab-runner-operator).

1. Installez les prérequis.
1. En haut à droite, sélectionnez **Installer** et suivez les instructions pour installer `olm` et l'opérateur.

#### Installer GitLab Runner {#install-gitlab-runner}

1. Obtenez un token d'authentification de runner. Vous pouvez soit :
   - Créer un runner d'[instance](https://docs.gitlab.com/ci/runners/runners_scope/#create-an-instance-runner-with-a-runner-authentication-token), de [groupe](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-group-runner-with-a-runner-authentication-token) ou de [projet](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-project-runner-with-a-runner-authentication-token).
   - Localisez le token d'authentification du runner dans le fichier `config.toml`. Les tokens d'authentification de runner ont le préfixe `glrt-`.
1. Créez le fichier secret avec votre token GitLab Runner :

   ```shell
   cat > gitlab-runner-secret.yml << EOF
   apiVersion: v1
   kind: Secret
   metadata:
     name: gitlab-runner-secret
   type: Opaque
   # Only one of the following fields can be set. The Operator fails to register the runner if both are provided.
   # NOTE: runner-registration-token is deprecated and will be removed in GitLab 18.0. You should use runner-token instead.
   stringData:
     runner-token: REPLACE_ME # your project runner token
     # runner-registration-token: "" # your project runner secret
   EOF
   ```

1. Créez le `secret` dans votre cluster en exécutant :

   ```shell
   kubectl apply -f gitlab-runner-secret.yml
   ```

1. Créez le fichier Custom Resource Definition (CRD) et incluez la configuration suivante.

   ```shell
   cat > gitlab-runner.yml << EOF
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: gitlab-runner
   spec:
     gitlabUrl: https://gitlab.example.com
     buildImage: alpine
     token: gitlab-runner-secret
   EOF
   ```

1. Appliquez maintenant le fichier `CRD` en exécutant la commande :

   ```shell
   kubectl apply -f gitlab-runner.yml
   ```

1. Confirmez que GitLab Runner est installé en exécutant :

   ```shell
   kubectl get runner
   NAME             AGE
   gitlab-runner    5m
   ```

1. Le pod du runner doit également être visible :

   ```shell
   kubectl get pods
   NAME                             READY   STATUS    RESTARTS   AGE
   gitlab-runner-bf9894bdb-wplxn    1/1     Running   0          5m
   ```

#### Installer d'autres versions de GitLab Runner Operator pour OpenShift {#install-other-versions-of-gitlab-runner-operator-for-openshift}

Si vous ne souhaitez pas utiliser la version disponible de GitLab Runner Operator dans le Red Hat OperatorHub, vous pouvez installer une version différente.

Pour connaître les versions officielles disponibles de l'opérateur, consultez les [tags dans le dépôt `gitlab-runner-operator`](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/-/tags). Pour savoir quelle version de GitLab Runner l'opérateur exécute, consultez le contenu du fichier `APP_VERSION` du commit ou du tag qui vous intéresse, par exemple <https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/-/blob/1-17-stable/APP_VERSION>.

Pour installer une version spécifique, créez ce fichier `catalogsource.yaml` et remplacez `<VERSION>` par un tag ou un commit spécifique :

> [!note]
> Lors de l'utilisation d'une image pour un commit spécifique, le format de tag est `v0.0.1-<COMMIT>`. Par exemple : `registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/gitlab-runner-operator-catalog-source:v0.0.1-f5a798af`.

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: gitlab-runner-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/gitlab-runner-operator-catalog-source:<VERSION>
  displayName: GitLab Runner Operators
  publisher: GitLab Community
```

Créez le `CatalogSource` avec :

```shell
oc apply -f catalogsource.yaml
```

Dans quelques instants, le nouveau runner devrait apparaître dans la section OperatorHub du cluster OpenShift.

## Installer GitLab Runner Operator sur des clusters Kubernetes dans des environnements hors ligne {#install-gitlab-runner-operator-on-kubernetes-clusters-in-offline-environments}

Prérequis :

- Les images requises par le processus d'installation sont accessibles.

Pour extraire des images de conteneur lors de l'installation, GitLab Runner Operator nécessite une connexion à l'internet public sur un réseau externe. Si vous avez des clusters Kubernetes installés dans un environnement hors ligne, utilisez un registre d'images local ou un référentiel de paquets pour extraire des images ou des paquets lors de l'installation.

Le dépôt local doit fournir les images suivantes :

| Image                                                 | Valeur par défaut |
|-------------------------------------------------------|---------------|
| Image **GitLab Runner Operator**                      | `registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/gitlab-runner-operator:vGITLAB_RUNNER_OPERATOR_VERSION` |
| Images **GitLab Runner** et **GitLab Runner Helper** | Ces images sont téléchargées depuis le registre GitLab Runner UBI Images et sont utilisées lors de l'installation des Runner Custom Resources. La version utilisée dépend de vos exigences. |
| Image **RBAC Proxy**                                  | `registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/openshift4/ose-kube-rbac-proxy:v4.13.0` |

1. Configurez des référentiels ou des registres locaux dans l'environnement réseau déconnecté pour héberger les paquets logiciels téléchargés et les images de conteneur. Vous pouvez utiliser :

   - Un registre Docker pour les images de conteneur.
   - Un registre de paquets local pour les binaires et dépendances Kubernetes.

1. Pour GitLab Runner Operator v1.23.2 et versions ultérieures, téléchargez la dernière version du fichier `operator.k8s.yaml` :

   ```shell
   curl -O "https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-
   operator/-/releases/vGITLAB_RUNNER_OPERATOR_VERSION/downloads/operator.k8s.yaml"
   ```

1. Dans le fichier `operator.k8s.yaml`, mettez à jour les URL suivantes :

   - `GitLab Runner Operator image`
   - `RBAC Proxy image`

1. Installez la version mise à jour du fichier `operator.k8s.yaml` :

   ```shell
   kubectl apply -f PATH_TO_UPDATED_OPERATOR_K8S_YAML
   GITLAB_RUNNER_OPERATOR_VERSION = 1.23.2+
   ```

## Désinstaller l'opérateur {#uninstall-operator}

### Désinstaller sur Red Hat OpenShift {#uninstall-on-red-hat-openshift}

1. Supprimez le runner `CRD` :

   ```shell
   kubectl delete -f gitlab-runner.yml
   ```

1. Supprimez le `secret` :

   ```shell
   kubectl delete -f gitlab-runner-secret.yml
   ```

1. Suivez les instructions dans la documentation Red Hat pour [Deleting Operators from a cluster using the web console](https://docs.redhat.com/en/documentation/openshift_container_platform/4.7/html/operators/administrator-tasks#olm-deleting-operators-from-a-cluster-using-web-console_olm-deleting-operators-from-a-cluster).

### Désinstaller sur Kubernetes {#uninstall-on-kubernetes}

1. Supprimez le runner `CRD` :

   ```shell
   kubectl delete -f gitlab-runner.yml
   ```

1. Supprimez le `secret` :

   ```shell
   kubectl delete -f gitlab-runner-secret.yml
   ```

1. Supprimez l'abonnement à l'opérateur :

   ```shell
   kubectl delete subscription my-gitlab-runner-operator -n operators
   ```

1. Déterminez la version du `CSV` installé :

   ```shell
   kubectl get clusterserviceversion -n operators
   NAME                            DISPLAY         VERSION   REPLACES   PHASE
   gitlab-runner-operator.v1.7.0   GitLab Runner   1.7.0                Succeeded
   ```

1. Supprimez le `CSV` :

   ```shell
   kubectl delete clusterserviceversion gitlab-runner-operator.v1.7.0 -n operators
   ```

#### Configuration {#configuration}

Pour configurer GitLab Runner dans OpenShift, consultez la page [Configuring GitLab Runner on OpenShift](../configuration/configuring_runner_operator.md).

#### Surveillance {#monitoring}

Pour activer la surveillance et la collecte de métriques pour les déploiements de GitLab Runner Operator, consultez [Monitor GitLab Runner Operator](../monitoring/_index.md#monitor-operator-managed-gitlab-runners).
