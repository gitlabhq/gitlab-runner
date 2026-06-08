---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: GitLab 에이전트를 사용하여 GitLab 러너를 설치합니다.
title: 에이전트를 사용하여 GitLab 러너 설치
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

[GitLab agent for Kubernetes](https://docs.gitlab.com/user/clusters/agent/)를 설치하고 구성한 후 에이전트를 사용하여 클러스터에 GitLab 러너를 설치할 수 있습니다.

이 [GitOps workflow](https://docs.gitlab.com/user/clusters/agent/gitops/)를 사용하면 리포지토리에 GitLab 러너 구성 파일이 포함되어 있으며 클러스터가 자동으로 업데이트됩니다.

> [!warning]
> 암호화되지 않은 GitLab 러너 시크릿을 `runner-manifest.yaml`에 추가하면 리포지토리 파일에서 시크릿이 노출될 수 있습니다. GitOps 워크플로우에서 Kubernetes 시크릿을 안전하게 관리하려면 [Sealed Secrets](https://fluxcd.io/flux/guides/sealed-secrets/) 또는 [SOPS](https://fluxcd.io/flux/guides/mozilla-sops/)를 사용하세요.

1. [GitLab Runner](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml)의 Helm 차트 값을 검토하세요.
1. `runner-chart-values.yaml` 파일을 생성합니다. 예를 들어:

   ```yaml
   # The GitLab Server URL (with protocol) that you want to register the runner against
   # ref: https://docs.gitlab.com/runner/commands/#gitlab-runner-register
   #
   gitlabUrl: https://gitlab.my.domain.example.com/

   # The registration token for adding new runners to the GitLab server
   # Retrieve this value from your GitLab instance
   # For more info: https://docs.gitlab.com/ci/runners/
   #
   runnerRegistrationToken: "yrnZW46BrtBFqM7xDzE7dddd"

   # For RBAC support:
   rbac:
       create: true

   # Run all containers with the privileged flag enabled
   # This flag allows the docker:dind image to run if you need to run Docker commands
   # Read the docs before turning this on:
   # https://docs.gitlab.com/runner/executors/kubernetes/#using-dockerdind
   runners:
       privileged: true
   ```

1. 클러스터 에이전트와 함께 GitLab 러너 차트를 설치하기 위한 단일 매니페스트 파일을 생성합니다:

   ```shell
   helm template --namespace GITLAB-NAMESPACE gitlab-runner -f runner-chart-values.yaml gitlab/gitlab-runner > runner-manifest.yaml
   ```

   `GITLAB-NAMESPACE`을(를) 네임스페이스로 바꾸세요. [예시 보기](#example-runner-manifest)

1. `runner-manifest.yaml` 파일을 편집하여 `ServiceAccount`의 `namespace`을(를) 포함시키세요. `helm template`의 출력에는 생성된 리소스에 `ServiceAccount` 네임스페이스가 포함되어 있지 않습니다.

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

1. `runner-manifest.yaml`을(를) Kubernetes 매니페스트를 보관하는 리포지토리로 푸시합니다.
1. 에이전트를 구성하여 [GitOps](https://docs.gitlab.com/user/clusters/agent/gitops/)를 사용하여 러너 매니페스트를 동기화합니다. 예를 들어:

   ```yaml
   gitops:
     manifest_projects:
     - id: path/to/manifest/project
       paths:
       - glob: 'path/to/runner-manifest.yaml'
   ```

이제 에이전트가 매니페스트 업데이트를 위해 리포지토리를 확인할 때마다 클러스터가 업데이트되어 GitLab 러너가 포함됩니다.

## 예시 러너 매니페스트 {#example-runner-manifest}

이 예시는 샘플 러너 매니페스트 파일을 보여줍니다. 프로젝트의 필요에 맞게 자신만의 `manifest.yaml` 파일을 생성하세요.

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
    chart: gitlab-runner-0.58.2
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
    chart: gitlab-runner-0.58.2
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
    chart: gitlab-runner-0.58.2
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
    chart: gitlab-runner-0.58.2
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
    chart: gitlab-runner-0.58.2
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
    chart: gitlab-runner-0.58.2
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
        chart: gitlab-runner-0.58.2
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

## 문제 해결 {#troubleshooting}

### 오류: `associative list with keys has an element that omits key field "protocol"` {#error-associative-list-with-keys-has-an-element-that-omits-key-field-protocol}

[Kubernetes v1.19의 버그](https://github.com/kubernetes-sigs/structured-merge-diff/issues/130)로 인해 GitLab 러너 또는 GitLab agent for Kubernetes를 사용한 다른 애플리케이션을 설치할 때 이 오류가 나타날 수 있습니다. 이를 해결하려면 다음 중 하나를 수행하세요:

- Kubernetes 클러스터를 v1.20 이상으로 업그레이드합니다.
- `protocol: TCP`을(를) `containers.ports` 섹션에 추가합니다:

  ```yaml
  ...
  ports:
    - name: metrics
      containerPort: 9252
      protocol: TCP
  ...
  ```
