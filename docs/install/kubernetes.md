# Run GitLab Runner on a Kubernetes cluster

TIP: **Tip:**
We also provide a [GitLab Runner Helm Chart](https://docs.gitlab.com/ce/install/kubernetes/gitlab_runner_chart.html).

To install the GitLab CI Runner on Kubernetes there are several resources that need to be defined and then pushed to the cluster with `kubectl`.  This topic covers how to:

1. Register the new runner using the API.
1. Define the runner ConfigMap in a yaml file.
1. Define the runner Deployment yaml file.
1. Push the definitions to a Kubernetes cluster using `kubectl`.

## Register the new runner using the API
The runner must first be registered to your project (or group or instance) so that the runner token
(not to be confused with the runner registration token) can be provided to the `ConfigMap` below.
Use the [GitLab Runners API](https://docs.gitlab.com/ee/api/runners.html#register-a-new-runner) to register
the new runner, providing the registration token from the project, group or instance CI/CD settings as described in
[Configuring GitLab Runners](https://docs.gitlab.com/ee/ci/runners/README.html).  The runner token is returned
by the API runner registration command.

## Define the Runner `ConfigMap`

Create a file named `runner_config.yml` from the following example:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gitlab-runner
  namespace: gitlab
data:
  config.toml: |
    concurrent = 4

    [[runners]]
      name = "Kubernetes Runner"
      url = "https://gitlab.com"
      token = "...."
      executor = "kubernetes"
      [runners.kubernetes]
        namespace = "gitlab"
        image = "busybox"
```

Update the `url` and `token` with your values.  The parameter `image` is optional and is the default Docker image used to be used to run jobs.

>**Note:**
> Don't use the `gitlab-managed-apps` namespace for this runner. It should be reserved for applications installed through the GitLab UI.


## Define the Runner `Deployment`

Create a file named `runner_deployment.yml` from the following example:

```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: gitlab-runner
  namespace: gitlab
spec:
  replicas: 1
  selector:
    matchLabels:
      name: gitlab-runner
  template:
    metadata:
      labels:
        name: gitlab-runner
    spec:
      containers:
      - args:
        - run
        image: gitlab/gitlab-runner:latest
        name: gitlab-runner
        volumeMounts:
        - mountPath: /etc/gitlab-runner
          name: config
        - mountPath: /etc/ssl/certs
          name: cacerts
          readOnly: true
      restartPolicy: Always
      volumes:
      - configMap:
          name: gitlab-runner
        name: config
      - hostPath:
          path: /usr/share/ca-certificates/mozilla
        name: cacerts
```

## Push the definitions to Kubernetes

Assuming that your kubectl context has already been set to the cluster in question, issue these commands:

`kubectl apply -f runner_config.yml`

`kubectl apply -f runner_deployment.yml`

The new runner will now show up in the GitLab web UI at the appropriate level (instance, group or project).

For more details see [Kubernetes executor](../executors/kubernetes.md)
and the [[runners.kubernetes] section of advanced configuration](../configuration/advanced-configuration.md#the-runnerskubernetes-section).
