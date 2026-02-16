---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Install GitLab Runner using the GitLab Operator for Kubernetes.
title: Install GitLab Runner Operator
---

## Install on Red Hat OpenShift

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Install GitLab Runner on Red Hat OpenShift v4 and later using the [GitLab Runner Operator](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator) from the stable channel of OperatorHub in OpenShift's web console. Once installed, you can run your GitLab CI/CD jobs using the newly deployed GitLab Runner instance. Each CI/CD job runs in a separate pod.

### Prerequisites

- OpenShift 4.x cluster with administrator privileges
- GitLab Runner registration token

### Install the OpenShift Operator

First you must install the OpenShift Operator.

1. Open the OpenShift UI and sign in as a user with administrator privileges.
1. In the left pane, select **Operators**, then **OperatorHub**.
1. In the main pane, below **All Items**, search for the keyword `GitLab Runner`.

   ![GitLab Operator](img/openshift_allitems_v13_3.png)

1. To install, select the GitLab Runner Operator.
1. On the GitLab Runner Operator summary page, select **Install**.
1. On the Install Operator page:
   1. Under **Update Channel**, select **stable**.
   1. Under **Installed Namespace**, select the desired namespace and select **Install**.

   ![GitLab Operator Install Page](img/openshift_installoperator_v13_3.png)

On the Installed Operators page, when the GitLab Operator is ready, the status changes to **Succeeded**.

![GitLab Operator Install Status](img/openshift_success_v13_3.png)

## Install on Kubernetes

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Install GitLab Runner on Kubernetes v1.21 and later using the [GitLab Runner Operator](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator) from the stable channel of [OperatorHub.io](https://operatorhub.io/operator/gitlab-runner-operator). Once installed, you can run your GitLab CI/CD jobs using the newly deployed GitLab Runner instance. Each CI/CD job runs in a separate pod.

### Prerequisites

- Kubernetes v1.21 and later
- Cert manager v1.7.1

### Install the Kubernetes Operator

Follow the instructions at [OperatorHub.io](https://operatorhub.io/operator/gitlab-runner-operator).

1. Install the prerequisites.
1. On the top right, select **Install** and follow the instructions to install `olm` and the Operator.

#### Install GitLab Runner

1. Obtain a runner authentication token. You can either:
   - Create an [instance](https://docs.gitlab.com/ci/runners/runners_scope/#create-an-instance-runner-with-a-runner-authentication-token),
     [group](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-group-runner-with-a-runner-authentication-token), or
     [project](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-project-runner-with-a-runner-authentication-token) runner.
   - Locate the runner authentication token in the `config.toml` file. Runner authentication tokens have the prefix, `glrt-`.
1. Create the secret file with your GitLab Runner token:

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

1. Create the `secret` in your cluster by running:

   ```shell
   kubectl apply -f gitlab-runner-secret.yml
   ```

1. Create the Custom Resource Definition (CRD) file and include
   the following configuration.

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

1. Now apply the `CRD` file by running the command:

   ```shell
   kubectl apply -f gitlab-runner.yml
   ```

1. Confirm that GitLab Runner is installed by running:

   ```shell
   kubectl get runner
   NAME             AGE
   gitlab-runner    5m
   ```

1. The runner pod should also be visible:

   ```shell
   kubectl get pods
   NAME                             READY   STATUS    RESTARTS   AGE
   gitlab-runner-bf9894bdb-wplxn    1/1     Running   0          5m
   ```

#### Install other versions of GitLab Runner Operator for OpenShift

If you do not want to use the available GitLab Runner Operator version in the Red Hat OperatorHub, you can install a different version.

To find out the official available Operator versions, view the [tags in the `gitlab-runner-operator` repository](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/-/tags).
To find out which version of GitLab Runner the Operator is running, view the contents of the
`APP_VERSION` file of the commit or tag you are interested in, for example, [https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/-/blob/1-17-stable/APP_VERSION](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/-/blob/1-17-stable/APP_VERSION).

To install a specific version, create this `catalogsource.yaml` file and replace `<VERSION>` with a tag or a specific commit:

{{< alert type="note" >}}

When using an image for a specific commit, the tag format is `v0.0.1-<COMMIT>`. For example: `registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/gitlab-runner-operator-catalog-source:v0.0.1-f5a798af`.

{{< /alert >}}

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

Create the `CatalogSource` with:

```shell
oc apply -f catalogsource.yaml
```

In a minute the new Runner should show up in the OpenShift cluster's OperatorHub section.

## Install GitLab Runner Operator on Kubernetes clusters in offline environments

Prerequisites:

- Images required by the installation process are accessible.

To pull container images during installation,
the GitLab Runner Operator requires a connection to the public internet on an external
network. If you have Kubernetes clusters installed
in an offline environment, use a local image registry or package repository
to pull images or packages during installation.

The local repository must provide the following images:

| Image                                                 | Default value |
|-------------------------------------------------------|---------------|
| **GitLab Runner Operator** image                      | `registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/gitlab-runner-operator:vGITLAB_RUNNER_OPERATOR_VERSION` |
| **GitLab Runner** and **GitLab Runner Helper** images | These images are downloaded from the GitLab Runner UBI Images registry and are used when installing the Runner Custom Resources. The version used depends on your requirements. |
| **RBAC Proxy** image                                  | `registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/openshift4/ose-kube-rbac-proxy:v4.13.0` |

1. Set up local repositories or registries in the disconnected network environment
   to host the downloaded software packages and container images. You can use:

   - A Docker registry for container images.
   - A local package registry for Kubernetes binaries and dependencies.

1. For GitLab Runner Operator v1.23.2 and later, download the latest version of `operator.k8s.yaml` file:

   ```shell
   curl -O "https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-
   operator/-/releases/vGITLAB_RUNNER_OPERATOR_VERSION/downloads/operator.k8s.yaml"
   ```

1. In the `operator.k8s.yaml` file, update the following URLs:

   - `GitLab Runner Operator image`
   - `RBAC Proxy image`

1. Install the updated version of the `operator.k8s.yaml` file:

   ```shell
   kubectl apply -f PATH_TO_UPDATED_OPERATOR_K8S_YAML
   GITLAB_RUNNER_OPERATOR_VERSION = 1.23.2+
   ```

## Uninstall Operator

### Uninstall on Red Hat OpenShift

1. Delete Runner `CRD`:

   ```shell
   kubectl delete -f gitlab-runner.yml
   ```

1. Delete `secret`:

   ```shell
   kubectl delete -f gitlab-runner-secret.yml
   ```

1. Follow the instructions at the Red Hat documentation for [Deleting Operators from a cluster using the web console](https://docs.redhat.com/en/documentation/openshift_container_platform/4.7/html/operators/administrator-tasks#olm-deleting-operators-from-a-cluster-using-web-console_olm-deleting-operators-from-a-cluster).

### Uninstall on Kubernetes

1. Delete Runner `CRD`:

   ```shell
   kubectl delete -f gitlab-runner.yml
   ```

1. Delete `secret`:

   ```shell
   kubectl delete -f gitlab-runner-secret.yml
   ```

1. Delete the Operator subscription:

   ```shell
   kubectl delete subscription my-gitlab-runner-operator -n operators
   ```

1. Find out the version of the installed `CSV`:

   ```shell
   kubectl get clusterserviceversion -n operators
   NAME                            DISPLAY         VERSION   REPLACES   PHASE
   gitlab-runner-operator.v1.7.0   GitLab Runner   1.7.0                Succeeded
   ```

1. Delete the `CSV`:

   ```shell
   kubectl delete clusterserviceversion gitlab-runner-operator.v1.7.0 -n operators
   ```

#### Configuration

To configure GitLab Runner in OpenShift, see the [Configuring GitLab Runner on OpenShift](../configuration/configuring_runner_operator.md) page.

#### Monitoring

To enable monitoring and metrics collection for GitLab Runner Operator deployments, see
[Monitor GitLab Runner Operator](../monitoring/_index.md#monitor-operator-managed-gitlab-runners).
