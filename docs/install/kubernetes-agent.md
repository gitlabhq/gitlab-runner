---
stage: Configure
group: Configure
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Use the agent to install GitLab Runner **(FREE)**

After you install and configure the [GitLab agent for Kubernetes](https://docs.gitlab.com/ee/user/clusters/agent/)
you can use the agent to install GitLab Runner in your cluster.

With this [GitOps workflow](https://docs.gitlab.com/ee/user/clusters/agent/gitops.html),
your repository contains the GitLab Runner configuration file and
your cluster is automatically updated.

WARNING:
Adding an unencrypted GitLab Runner secret to `runner-manifest.yaml`
can expose the secret in your repository files. If you use a GitOps
workflow in your public projects, see
[Managing Kubernetes secrets in a GitOps workflow](https://docs.gitlab.com/ee/user/clusters/agent/gitops/secrets_management.html).

1. Review the Helm chart values for [GitLab Runner](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml).
1. Create a `runner-chart-values.yaml` file. For example:

   ```yaml
   # The GitLab Server URL (with protocol) that you want to register the runner against
   # ref: https://docs.gitlab.com/runner/commands/index.html#gitlab-runner-register
   #
   gitlabUrl: https://gitlab.my.domain.example.com/

   # The registration token for adding new runners to the GitLab server
   # Retrieve this value from your GitLab instance
   # For more info: https://docs.gitlab.com/ee/ci/runners/index.html
   #
   runnerRegistrationToken: "yrnZW46BrtBFqM7xDzE7dddd"

   # For RBAC support:
   rbac:
       create: true

   # Run all containers with the privileged flag enabled
   # This flag allows the docker:dind image to run if you need to run Docker commands
   # Read the docs before turning this on:
   # https://docs.gitlab.com/runner/executors/kubernetes.html#using-dockerdind
   runners:
       privileged: true
   ```

1. Create a single manifest file to install the GitLab Runner chart with your cluster agent:

   ```shell
   helm template --namespace GITLAB-NAMESPACE gitlab-runner -f runner-chart-values.yaml gitlab/gitlab-runner > runner-manifest.yaml
   ```

   Replace `GITLAB-NAMESPACE` with your namespace. [View an example](#example-runner-manifest).

1. Edit the `runner-manifest.yaml` file to include the `namespace` of your `ServiceAccount`. The output
of `helm template` doesn't include the `ServiceAccount` namespace in the generated resources.

   ```yaml
   ---
   # Source: gitlab-runner/templates/service-account.yaml
   apiVersion: v1
   kind: ServiceAccount
   metadata:
     annotations:
     name: gitlab-runner-gitlab-runner
     namespace: gitlab
     labels:
   ...
   ```

1. Push your `runner-manifest.yaml` to the repository where you keep your Kubernetes manifests.
1. Configure your agent to sync the runner manifest using
   [GitOps](https://docs.gitlab.com/ee/user/clusters/agent/gitops.html). For example:

   ```yaml
   gitops:
     manifest_projects:
     - id: path/to/manifest/project
       paths:
       - glob: 'path/to/runner-manifest.yaml'
   ```

   For details, see the [GitOps configuration reference](https://docs.gitlab.com/ee/user/clusters/agent/gitops.html#gitops-configuration-reference).

Now each time the agent checks the repository for manifest updates, your
cluster is updated to include GitLab Runner.

## Example runner manifest

This example shows a sample runner manifest file.
Create your own `manifest.yaml` file to meet your project's needs.

```yaml
---
# Source: gitlab-runner/templates/service-account.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  annotations:
  name: gitlab-runner-gitlab-runner
  labels:
    app: gitlab-runner-gitlab-runner
    chart: gitlab-runner-0.21.1
    release: "gitlab-runner"
    heritage: "Helm"
---
# Source: gitlab-runner/templates/secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: "gitlab-runner-gitlab-runner"
  labels:
    app: gitlab-runner-gitlab-runner
    chart: gitlab-runner-0.21.1
    release: "gitlab-runner"
    heritage: "Helm"
type: Opaque
data:
  runner-registration-token: "FAKE-TOKEN"
  runner-token: ""
---
# Source: gitlab-runner/templates/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: gitlab-runner-gitlab-runner
  labels:
    app: gitlab-runner-gitlab-runner
    chart: gitlab-runner-0.21.1
    release: "gitlab-runner"
    heritage: "Helm"
data:
  entrypoint: |
    #!/bin/bash
    set -e
    mkdir -p /home/gitlab-runner/.gitlab-runner/
    cp /scripts/config.toml /home/gitlab-runner/.gitlab-runner/

    # Register the runner
    if [[ -f /secrets/accesskey && -f /secrets/secretkey ]]; then
      export CACHE_S3_ACCESS_KEY=$(cat /secrets/accesskey)
      export CACHE_S3_SECRET_KEY=$(cat /secrets/secretkey)
    fi

    if [[ -f /secrets/gcs-application-credentials-file ]]; then
      export GOOGLE_APPLICATION_CREDENTIALS="/secrets/gcs-application-credentials-file"
    elif [[ -f /secrets/gcs-application-credentials-file ]]; then
      export GOOGLE_APPLICATION_CREDENTIALS="/secrets/gcs-application-credentials-file"
    else
      if [[ -f /secrets/gcs-access-id && -f /secrets/gcs-private-key ]]; then
        export CACHE_GCS_ACCESS_ID=$(cat /secrets/gcs-access-id)
        # echo -e used to make private key multiline (in google json auth key private key is one line with \n)
        export CACHE_GCS_PRIVATE_KEY=$(echo -e $(cat /secrets/gcs-private-key))
      fi
    fi

    if [[ -f /secrets/runner-registration-token ]]; then
      export REGISTRATION_TOKEN=$(cat /secrets/runner-registration-token)
    fi

    if [[ -f /secrets/runner-token ]]; then
      export CI_SERVER_TOKEN=$(cat /secrets/runner-token)
    fi

    if ! sh /scripts/register-the-runner; then
      exit 1
    fi

    # Run pre-entrypoint-script
    if ! bash /scripts/pre-entrypoint-script; then
      exit 1
    fi

    # Start the runner
    exec /entrypoint run --user=gitlab-runner \
      --working-directory=/home/gitlab-runner

  config.toml: |
    concurrent = 10
    check_interval = 30
    log_level = "info"
    listen_address = ':9252'
  configure: |
    set -e
    cp /init-secrets/* /secrets
  register-the-runner: |
    #!/bin/bash
    MAX_REGISTER_ATTEMPTS=30

    for i in $(seq 1 "${MAX_REGISTER_ATTEMPTS}"); do
      echo "Registration attempt ${i} of ${MAX_REGISTER_ATTEMPTS}"
      /entrypoint register \
        --non-interactive

      retval=$?

      if [ ${retval} = 0 ]; then
        break
      elif [ ${i} = ${MAX_REGISTER_ATTEMPTS} ]; then
        exit 1
      fi

      sleep 5
    done

    exit 0

  check-live: |
    #!/bin/bash
    if /usr/bin/pgrep -f .*register-the-runner; then
      exit 0
    elif /usr/bin/pgrep gitlab.*runner; then
      exit 0
    else
      exit 1
    fi

  pre-entrypoint-script: |
---
# Source: gitlab-runner/templates/role.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: "Role"
metadata:
  name: gitlab-runner-gitlab-runner
  labels:
    app: gitlab-runner-gitlab-runner
    chart: gitlab-runner-0.21.1
    release: "gitlab-runner"
    heritage: "Helm"
rules:
- apiGroups: [""]
  resources: ["*"]
  verbs: ["*"]
---
# Source: gitlab-runner/templates/role-binding.yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: "RoleBinding"
metadata:
  name: gitlab-runner-gitlab-runner
  labels:
    app: gitlab-runner-gitlab-runner
    chart: gitlab-runner-0.21.1
    release: "gitlab-runner"
    heritage: "Helm"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: "Role"
  name: gitlab-runner-gitlab-runner
subjects:
- kind: ServiceAccount
  name: gitlab-runner-gitlab-runner
  namespace: "gitlab"
---
# Source: gitlab-runner/templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: gitlab-runner-gitlab-runner
  labels:
    app: gitlab-runner-gitlab-runner
    chart: gitlab-runner-0.21.1
    release: "gitlab-runner"
    heritage: "Helm"
spec:
  replicas: 1
  selector:
    matchLabels:
      app: gitlab-runner-gitlab-runner
  template:
    metadata:
      labels:
        app: gitlab-runner-gitlab-runner
        chart: gitlab-runner-0.21.1
        release: "gitlab-runner"
        heritage: "Helm"
      annotations:
        checksum/configmap: a6623303f6fcc3a043e87ea937bb8399d2d0068a901aa9c3419ed5c7a5afa9db
        checksum/secrets: 32c7d2c16918961b7b84a005680f748e774f61c6f4e4da30650d400d781bbb30
        prometheus.io/scrape: 'true'
        prometheus.io/port: '9252'
    spec:
      securityContext:
        runAsUser: 100
        fsGroup: 65533
      terminationGracePeriodSeconds: 3600
      initContainers:
      - name: configure
        command: ['sh', '/config/configure']
        image: gitlab/gitlab-runner:alpine-v13.4.1
        imagePullPolicy: "IfNotPresent"
        env:

        - name: CI_SERVER_URL
          value: "https://gitlab.qa.joaocunha.eu/"
        - name: CLONE_URL
          value: ""
        - name: RUNNER_REQUEST_CONCURRENCY
          value: "1"
        - name: RUNNER_EXECUTOR
          value: "kubernetes"
        - name: REGISTER_LOCKED
          value: "true"
        - name: RUNNER_TAG_LIST
          value: ""
        - name: RUNNER_OUTPUT_LIMIT
          value: "4096"
        - name: KUBERNETES_IMAGE
          value: "ubuntu:16.04"

        - name: KUBERNETES_PRIVILEGED
          value: "true"

        - name: KUBERNETES_NAMESPACE
          value: "gitlab"
        - name: KUBERNETES_POLL_TIMEOUT
          value: "180"
        - name: KUBERNETES_CPU_LIMIT
          value: ""
        - name: KUBERNETES_CPU_LIMIT_OVERWRITE_MAX_ALLOWED
          value: ""
        - name: KUBERNETES_MEMORY_LIMIT
          value: ""
        - name: KUBERNETES_MEMORY_LIMIT_OVERWRITE_MAX_ALLOWED
          value: ""
        - name: KUBERNETES_CPU_REQUEST
          value: ""
        - name: KUBERNETES_CPU_REQUEST_OVERWRITE_MAX_ALLOWED
          value: ""
        - name: KUBERNETES_MEMORY_REQUEST
          value: ""
        - name: KUBERNETES_MEMORY_REQUEST_OVERWRITE_MAX_ALLOWED
          value: ""
        - name: KUBERNETES_SERVICE_ACCOUNT
          value: ""
        - name: KUBERNETES_SERVICE_CPU_LIMIT
          value: ""
        - name: KUBERNETES_SERVICE_MEMORY_LIMIT
          value: ""
        - name: KUBERNETES_SERVICE_CPU_REQUEST
          value: ""
        - name: KUBERNETES_SERVICE_MEMORY_REQUEST
          value: ""
        - name: KUBERNETES_HELPER_CPU_LIMIT
          value: ""
        - name: KUBERNETES_HELPER_MEMORY_LIMIT
          value: ""
        - name: KUBERNETES_HELPER_CPU_REQUEST
          value: ""
        - name: KUBERNETES_HELPER_MEMORY_REQUEST
          value: ""
        - name: KUBERNETES_HELPER_IMAGE
          value: ""
        - name: KUBERNETES_PULL_POLICY
          value: ""
        volumeMounts:
        - name: runner-secrets
          mountPath: /secrets
          readOnly: false
        - name: scripts
          mountPath: /config
          readOnly: true
        - name: init-runner-secrets
          mountPath: /init-secrets
          readOnly: true
        resources:
          {}
      serviceAccountName: gitlab-runner-gitlab-runner
      containers:
      - name: gitlab-runner-gitlab-runner
        image: gitlab/gitlab-runner:alpine-v13.4.1
        imagePullPolicy: "IfNotPresent"
        lifecycle:
          preStop:
            exec:
              command: ["/entrypoint", "unregister", "--all-runners"]
        command: ["/bin/bash", "/scripts/entrypoint"]
        env:

        - name: CI_SERVER_URL
          value: "https://gitlab.qa.joaocunha.eu/"
        - name: CLONE_URL
          value: ""
        - name: RUNNER_REQUEST_CONCURRENCY
          value: "1"
        - name: RUNNER_EXECUTOR
          value: "kubernetes"
        - name: REGISTER_LOCKED
          value: "true"
        - name: RUNNER_TAG_LIST
          value: ""
        - name: RUNNER_OUTPUT_LIMIT
          value: "4096"
        - name: KUBERNETES_IMAGE
          value: "ubuntu:16.04"

        - name: KUBERNETES_PRIVILEGED
          value: "true"

        - name: KUBERNETES_NAMESPACE
          value: "gitlab"
        - name: KUBERNETES_POLL_TIMEOUT
          value: "180"
        - name: KUBERNETES_CPU_LIMIT
          value: ""
        - name: KUBERNETES_CPU_LIMIT_OVERWRITE_MAX_ALLOWED
          value: ""
        - name: KUBERNETES_MEMORY_LIMIT
          value: ""
        - name: KUBERNETES_MEMORY_LIMIT_OVERWRITE_MAX_ALLOWED
          value: ""
        - name: KUBERNETES_CPU_REQUEST
          value: ""
        - name: KUBERNETES_CPU_REQUEST_OVERWRITE_MAX_ALLOWED
          value: ""
        - name: KUBERNETES_MEMORY_REQUEST
          value: ""
        - name: KUBERNETES_MEMORY_REQUEST_OVERWRITE_MAX_ALLOWED
          value: ""
        - name: KUBERNETES_SERVICE_ACCOUNT
          value: ""
        - name: KUBERNETES_SERVICE_CPU_LIMIT
          value: ""
        - name: KUBERNETES_SERVICE_MEMORY_LIMIT
          value: ""
        - name: KUBERNETES_SERVICE_CPU_REQUEST
          value: ""
        - name: KUBERNETES_SERVICE_MEMORY_REQUEST
          value: ""
        - name: KUBERNETES_HELPER_CPU_LIMIT
          value: ""
        - name: KUBERNETES_HELPER_MEMORY_LIMIT
          value: ""
        - name: KUBERNETES_HELPER_CPU_REQUEST
          value: ""
        - name: KUBERNETES_HELPER_MEMORY_REQUEST
          value: ""
        - name: KUBERNETES_HELPER_IMAGE
          value: ""
        - name: KUBERNETES_PULL_POLICY
          value: ""
        livenessProbe:
          exec:
            command: ["/bin/bash", "/scripts/check-live"]
          initialDelaySeconds: 60
          timeoutSeconds: 1
          periodSeconds: 10
          successThreshold: 1
          failureThreshold: 3
        readinessProbe:
          exec:
            command: ["/usr/bin/pgrep","gitlab.*runner"]
          initialDelaySeconds: 10
          timeoutSeconds: 1
          periodSeconds: 10
          successThreshold: 1
          failureThreshold: 3
        ports:
        - name: metrics
          containerPort: 9252
        volumeMounts:
        - name: runner-secrets
          mountPath: /secrets
        - name: etc-gitlab-runner
          mountPath: /home/gitlab-runner/.gitlab-runner
        - name: scripts
          mountPath: /scripts
        resources:
          {}
      volumes:
      - name: runner-secrets
        emptyDir:
          medium: "Memory"
      - name: etc-gitlab-runner
        emptyDir:
          medium: "Memory"
      - name: init-runner-secrets
        projected:
          sources:
            - secret:
                name: "gitlab-runner-gitlab-runner"
                items:
                  - key: runner-registration-token
                    path: runner-registration-token
                  - key: runner-token
                    path: runner-token
      - name: scripts
        configMap:
          name: gitlab-runner-gitlab-runner
```

## Troubleshooting

### `associative list with keys has an element that omits key field "protocol"`

Due to [the bug in Kubernetes v1.19](https://github.com/kubernetes-sigs/structured-merge-diff/issues/130), you may see this error when installing GitLab Runner or any other application with the GitLab agent for Kubernetes. To fix it, either:

- Upgrade your Kubernetes cluster to v1.20 or later.
- Add `protocol: TCP` to `containers.ports` subsection:

  ```yaml
  ...
  ports:
    - name: metrics
      containerPort: 9252
      protocol: TCP
  ...
  ```
