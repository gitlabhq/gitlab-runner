---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Oracle Cloud Infrastructure用のGitLab Runnerの設定
---

Container Runtime Interface (CRI) を使用するOracle Cloud Infrastructure (OCI) 環境で実行されるGitLabコード品質ジョブでは、パフォーマンスの低下が発生する可能性があります。

OCIでのGitLab Runnerのパフォーマンスを最適化するには、次の手順に従います:

1. 空のディレクトリボリュームをGitLab Runnerの設定に追加します。
1. `.gitlab-ci.yml`ファイルで特定のDockerドライバー設定を設定します。

この設定は、以下の環境に適用されます:

- クラウドプロバイダー: Oracle Cloud Infrastructure (OCI)
- ランタイム: Container Runtime Interface (CRI)
- プロセス: GitLabコード品質ジョブ
- Runnerタイプ: GitLab Self-Managed Runners

## 空のディレクトリボリュームを追加 {#add-an-empty-directory-volume}

GitLab Runnerの設定用に空のディレクトリを定義するには、次のブロックを`values.yaml`ファイルのrunnersセクションに追加します:

```yaml
[[runners.kubernetes.volumes.empty_dir]]
  mount_path = "/var/lib"
  name = "docker-data"
```

### Runnerの設定例 {#example-runner-configuration}

次の例は、修正を含むGitLab Runnerの完全なHelmチャート`values.yaml`を示しています:

```yaml
image:
  registry: registry.gitlab.com
  image: gitlab-org/gitlab-runner
  tag: alpine-v16.11.0

useTini: false
imagePullPolicy: IfNotPresent
gitlabUrl: https://gitlab.com/
runnerToken: ""
terminationGracePeriodSeconds: 3600
concurrent: 100
shutdown_timeout: 0
checkInterval: 5
logLevel: debug
sessionServer:
  enabled: false
## For RBAC support:
rbac:
  create: true
  rules: []
  clusterWideAccess: false
  podSecurityPolicy:
    enabled: false
    resourceNames:
    - gitlab-runner
metrics:
  enabled: false
  portName: metrics
  port: 9252
  serviceMonitor:
    enabled: false
service:
  enabled: false
  type: ClusterIP
runners:
  config: |
    [[runners]]
      output_limit = 200960
      [runners.kubernetes]
        privileged = true
        allow_privilege_escalation = true
        namespace = "{{.Release.Namespace}}"
        image = "ubuntu:22.04"
        helper_image_flavor = "ubuntu"
        pull_policy = "if-not-present"
        executor = "kubernetes"
        [[runners.kubernetes.volumes.host_path]]
          name = "buildah"
          mount_path = "/var/lib/containers/storage"
          read_only = false
        [runners.kubernetes.volumes]
        [[runners.kubernetes.volumes.empty_dir]]
          mount_path = "/var/lib"
          name = "docker-data"
        [[runners.kubernetes.services]]
          alias = "dind"
          command = [
              "--host=tcp://0.0.0.0:2375",
              "--host=unix://var/run/docker.sock",
          ]
      [runners.cache]
        Type = "s3"
        Path = "gitlab_runner"
        Shared = true
        [runners.cache.s3]
          BucketName = "gitlab-shared-caching"
          BucketLocation = "ap-singapore-1"
          ServerAddress = ".compat.objectstorage.ap-singapore-1.oraclecloud.com"
          AccessKey = ""
          SecretKey = ""

  configPath: ""
  tags: ""
  cache: {}

securityContext:
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: false
  runAsNonRoot: true
  privileged: false
  capabilities:
    drop: ["ALL"]
strategy: {}
podSecurityContext:
  runAsUser: 100
  fsGroup: 65533
resources: {}
affinity: {}
topologySpreadConstraints: {}
nodeSelector: {}
tolerations: []
hostAliases: []
deploymentAnnotations: {}
deploymentLabels: {}
podAnnotations: {}
podLabels: {}
priorityClassName: ""
secrets: []
configMaps: {}
volumeMounts: []
volumes: []
```

## `.gitlab-ci.yml`ファイルを更新します {#update-your-gitlab-ciyml-file}

デフォルトの`overlay2`ドライバーの選択を解除するには、次のキーを空の変数として既存のコード品質ジョブに追加します:

```shell
DOCKER_DRIVER: ""
```

### コード品質ジョブ設定の例 {#example-code-quality-job-configuration}

次の例は、`.gitlab-ci.yml`ファイルのコード品質ジョブ設定を示しています:

```yaml
code_quality:
  services:
    - name: $CODE_QUALITY_DIND_IMAGE
      command: ['--tls=false', '--host=tcp://0.0.0.0:2375']
  variables:
    CODECLIMATE_PREFIX: $CI_DEPENDENCY_PROXY_GROUP_IMAGE_PREFIX/
    CODECLIMATE_REGISTRY_USERNAME: $CI_DEPENDENCY_PROXY_USER
    CODECLIMATE_REGISTRY_PASSWORD: $CI_DEPENDENCY_PROXY_PASSWORD
    DOCKER_DRIVER: ""
```
