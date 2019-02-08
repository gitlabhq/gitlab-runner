# Run GitLab Runner on a Kubernetes cluster

To get started with the GitLab CI Runner on Kubernetes you need to define
resources that you can then push to the cluster with `kubectl`.

A recommended approach to this is to create a `ConfigMap` in Kubernetes such as
the following:

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

Where `image` (optional) is the default Docker image to run jobs on top of.  

Put this config map in a file named `runner_config.yml` and run the command 
`kubectl apply -f runner_config.yml`.

>**Note:**
The `token` can be found in `/etc/gitlab-runner/config.toml` and should
have been generated after registering the Runner. It's not to be confused
with the registration token that can be found under your project's
**Settings > CI/CD > Runners settings**.  Alternatively, you can obtain the
runner token using the GitLab API 
`curl -X POST https://gitlab.com/api/v4/runners --form "token=<registration-token>"`
where `<registration token>` is obtained from the **Settings > CI/CD > Runners settings**.



Then create a `Deployment` or `ReplicationController` which uses the `ConfigMap`.
This is an example of a `Deployment`:

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

Put this deployment in a file named `runner_deployment.yml` and run the command 
`kubectl apply -f runner_deployment.yml`.

For more details see [Kubernetes executor](../executors/kubernetes.md)
and the [[runners.kubernetes] section of advanced configuration](../configuration/advanced-configuration.md#the-runners-kubernetes-section).