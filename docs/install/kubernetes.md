---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Install GitLab Runner in Kubernetes using the GitLab Helm chart.
title: GitLab Runner Helm chart
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

The GitLab Runner Helm chart is the official way to deploy a GitLab Runner instance into your Kubernetes cluster.
This chart configures GitLab Runner to:

- Run using the [Kubernetes executor](../executors/kubernetes/_index.md) for GitLab Runner.
- Provision a new pod in the specified namespace for each new CI/CD job.

## Configure GitLab Runner with the Helm chart

Store your GitLab Runner configuration changes in `values.yaml`. For help configuring this file, see:

- The default [`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml)
  configuration in the chart repository.
- The Helm documentation for [Values Files](https://helm.sh/docs/chart_template_guide/values_files/), which explains
  how your values file overrides the default values.

For GitLab Runner to run properly, you must set these values in your configuration file:

- `gitlabUrl`: The full URL of the GitLab server (like `https://gitlab.example.com`) to register the runner against.
- `rbac: { create: true }`: Create RBAC (role-based access control) rules for the GitLab Runner to create
  pods to run jobs in.
  - If you want to use an existing `serviceAccount`, add your service account name in `rbac`:

    ```yaml
    rbac:
      create: false
    serviceAccount:
      create: false
      name: your-service-account
    ```

  - To learn about the minimal permissions the `serviceAccount` requires, see
    [Configure runner API permissions](../executors/kubernetes/_index.md#configure-runner-api-permissions).
- `runnerToken`: The authentication token obtained when you
  [create a runner in the GitLab UI](https://docs.gitlab.com/ci/runners/runners_scope/#create-an-instance-runner-with-a-runner-authentication-token).
  - Set this token directly or store it in a secret.

More [optional configuration settings](kubernetes_helm_chart_configuration.md) are available.

You're now ready to [install GitLab Runner](#install-gitlab-runner-with-the-helm-chart)!

## Install GitLab Runner with the Helm chart

Prerequisites:

- Your GitLab server's API is reachable from the cluster.
- Kubernetes 1.4 or later, with beta APIs enabled.
- The `kubectl` CLI is installed locally, and authenticated for the cluster.
- The [Helm client](https://helm.sh/docs/using_helm/#installing-the-helm-client) is installed locally on your machine.
- You've set all [required values in `values.yaml`](#configure-gitlab-runner-with-the-helm-chart).

To install GitLab Runner from the Helm chart:

1. Add the GitLab Helm repository:

   ```shell
   helm repo add gitlab https://charts.gitlab.io
   ```

1. If you use Helm 2, initialize Helm with `helm init`.
1. Check which GitLab Runner versions you have access to:

   ```shell
   helm search repo -l gitlab/gitlab-runner
   ```

1. If you can't access the latest versions of GitLab Runner, update the chart with this command:

   ```shell
   helm repo update gitlab
   ```

1. After you [configure](#configure-gitlab-runner-with-the-helm-chart) GitLab Runner in your `values.yaml` file,
   run this command, changing parameters as needed:

   ```shell
   # For Helm 2
   helm install --namespace <NAMESPACE> --name gitlab-runner -f <CONFIG_VALUES_FILE> gitlab/gitlab-runner

   # For Helm 3
   helm install --namespace <NAMESPACE> gitlab-runner -f <CONFIG_VALUES_FILE> gitlab/gitlab-runner
   ```

   - `<NAMESPACE>`: The Kubernetes namespace where you want to install the GitLab Runner.
   - `<CONFIG_VALUES_FILE>`: The path to values file containing your custom configuration. To create it, see
     [Configure GitLab Runner with the Helm chart](#configure-gitlab-runner-with-the-helm-chart).
   - To install a specific version of the GitLab Runner Helm chart, add `--version <RUNNER_HELM_CHART_VERSION>`
     to your `helm install` command. You can install any version of the chart, but more recent `values.yml` might
     be incompatible with older versions of the chart.

### Check available GitLab Runner Helm chart versions

Helm charts and GitLab Runner do not follow the same versioning. To see version mappings
between the two, run the command for your version of Helm:

```shell
# For Helm 2
helm search -l gitlab/gitlab-runner

# For Helm 3
helm search repo -l gitlab/gitlab-runner
```

An example of the output:

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

## Upgrade GitLab Runner with the Helm chart

Prerequisites:

- You've installed your GitLab Runner chart.
- You've paused the runner in GitLab. This prevents problems arising with the jobs, such as
  [authorization errors when they complete](../faq/_index.md#helm-chart-error--unauthorized).
- You've ensured all jobs have completed.

To change your configuration or update charts, use `helm upgrade`, changing parameters as needed:

```shell
helm upgrade --namespace <NAMESPACE> -f <CONFIG_VALUES_FILE> <RELEASE-NAME> gitlab/gitlab-runner
```

- `<NAMESPACE>`: The Kubernetes namespace where you've installed GitLab Runner.
- `<CONFIG_VALUES_FILE>`: The path to the values file containing your custom configuration. To create it, see
  [Configure GitLab Runner with the Helm chart](#configure-gitlab-runner-with-the-helm-chart).
- `<RELEASE-NAME>`: The name you gave the chart when you installed it.
  In the installation section, the example named it `gitlab-runner`.
- To update to a specific version of the GitLab Runner Helm chart, rather than the latest one, add
  `--version <RUNNER_HELM_CHART_VERSION>` to your `helm upgrade` command.

## Uninstall GitLab Runner with the Helm chart

To uninstall GitLab Runner:

1. Pause the runner in GitLab, and ensure any jobs have completed. This prevents job-related problems, such as
   [authorization errors on completion](../faq/_index.md#helm-chart-error--unauthorized).
1. Run, this command, modifying it as needed:

   ```shell
   helm delete --namespace <NAMESPACE> <RELEASE-NAME>
   ```

   - `<NAMESPACE>` is the Kubernetes namespace where GitLab Runner is installed.
   - `<RELEASE-NAME>` is the name you gave the chart when you installed it.
     In the [installation section](#install-gitlab-runner-with-the-helm-chart) of this page, we called it `gitlab-runner`.
