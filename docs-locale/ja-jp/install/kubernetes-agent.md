---
stage: Deploy
group: Environments
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: エージェントを使用してGitLab Runnerをインストールします
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

[Kubernetes向けGitLabエージェント](https://docs.gitlab.com/user/clusters/agent/)をインストールして設定すると、エージェントを使用してクラスターにGitLab Runnerをインストールできます。

この[GitOpsワークフロー](https://docs.gitlab.com/user/clusters/agent/gitops/)を使用すると、リポジトリにGitLab Runnerの設定ファイルが含まれ、クラスターが自動的に更新されます。

{{< alert type="warning" >}}

暗号化されていないGitLab Runnerのシークレットを`runner-manifest.yaml`に追加すると、リポジトリファイル内のシークレットが公開される可能性があります。GitOpsワークフローでKubernetes Secretsを安全に管理するには、[Sealed Secrets](https://fluxcd.io/flux/guides/sealed-secrets/)または[SOPS](https://fluxcd.io/flux/guides/mozilla-sops/)を使用します。

{{< /alert >}}

1. [GitLab Runner](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml)のHelmチャートの値を確認します。
1. `runner-chart-values.yaml`ファイルを作成します。次に例を示します: 

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

1. 単一のマニフェストファイルを作成して、GitLab Runnerチャートをクラスターエージェントと共にインストールします:

   ```shell
   helm template --namespace GITLAB-NAMESPACE gitlab-runner -f runner-chart-values.yaml gitlab/gitlab-runner > runner-manifest.yaml
   ```

   `GITLAB-NAMESPACE`をネームスペースに置き換えます。[例を表示](#example-runner-manifest)。

1. `runner-manifest.yaml`ファイルを編集して、`ServiceAccount`の`namespace`を含めます。`helm template`の出力には、生成されたリソースに`ServiceAccount`ネームスペースが含まれていません。

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

1. `runner-manifest.yaml`をKubernetesマニフェストを保持するリポジトリにプッシュします。
1. [GitOps](https://docs.gitlab.com/user/clusters/agent/gitops/)を使用してRunnerマニフェストを同期するようにエージェントを設定します。次に例を示します: 

   ```yaml
   gitops:
     manifest_projects:
     - id: path/to/manifest/project
       paths:
       - glob: 'path/to/runner-manifest.yaml'
   ```

これで、エージェントがマニフェストの更新についてリポジトリを確認するたびに、クラスターが更新されてGitLab Runnerが含まれるようになります。

## Runnerマニフェストの例 {#example-runner-manifest}

この例は、サンプルRunnerマニフェストファイルを示しています。プロジェクトのニーズに合わせて、独自の`manifest.yaml`ファイルを作成します。

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

## トラブルシューティング {#troubleshooting}

### エラー: `associative list with keys has an element that omits key field "protocol"`（コンポーネントビルドエラー: specは有効なJSONスキーマである必要があります） {#error-associative-list-with-keys-has-an-element-that-omits-key-field-protocol}

[Kubernetes v1.19のバグ](https://github.com/kubernetes-sigs/structured-merge-diff/issues/130)により、Kubernetes向けGitLabエージェントを使用してGitLab Runnerまたはその他のアプリケーションをインストールする際に、このエラーが表示される場合があります。これを修正するには、次のいずれかの方法があります:

- Kubernetesクラスターをv1.20以降にアップグレードします。
- `containers.ports`サブセクションに`protocol: TCP`を追加します:

  ```yaml
  ...
  ports:
    - name: metrics
      containerPort: 9252
      protocol: TCP
  ...
  ```
