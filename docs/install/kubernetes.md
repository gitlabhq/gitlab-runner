# Run GitLab Runner on a Kubernetes cluster

To install the GitLab CI Runner on Kubernetes there are several resources that need to be defined and then pushed to the cluster with `kubectl`.  This topic covers how to:
1. Define the runner ConfigMap in a yaml file
1. Define the runner Deployment yaml file
1. Push the definitions to a Kubernetes cluster using `kubectl`

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

Update the url and token with your values.  The parameter `image` is optional and is the default Docker image used to be used to run jobs.  

>**Notes:**
>* The `token` can be found in `/etc/gitlab-runner/config.toml` and should
have been generated after registering the Runner. It's not to be confused
with the registration token that can be found under your project's
**Settings > CI/CD > Runners settings**.  
>* Alternatively, you can obtain the runner token using the GitLab API 
`curl -X POST https://gitlab.com/api/v4/runners --form "token=<registration-token>"`
where `<registration token>` is obtained from the **Settings > CI/CD > Runners settings**.  
>* It is not recommended to use the gitlab-managed-apps namespace for this runner, that namespace should be reserved for applications installed through the GitLab UI.


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

`kubectl apply -f runner_config.yml`.

`kubectl apply -f runner_deployment.yml`.

For more details see [Kubernetes executor](../executors/kubernetes.md)
and the [[runners.kubernetes] section of advanced configuration](../configuration/advanced-configuration.md#the-runners-kubernetes-section).