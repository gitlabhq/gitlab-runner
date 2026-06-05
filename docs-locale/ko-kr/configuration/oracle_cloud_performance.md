---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Oracle Cloud Infrastructure용 러너 구성
---

Oracle Cloud Infrastructure(OCI) 환경에서 Container Runtime Interface(CRI)로 실행되는 GitLab Code Quality 작업은 성능 저하를 경험할 수 있습니다.

OCI에서 GitLab 러너 성능을 최적화하려면:

1. GitLab 러너 구성에 빈 디렉터리 볼륨을 추가합니다.
1. `.gitlab-ci.yml` 파일에서 특정 Docker 드라이버 설정을 구성합니다.

이 구성은 다음 환경에 적용됩니다:

- 클라우드 공급자:  Oracle Cloud Infrastructure(OCI)
- 컨테이너 런타임:  Container Runtime Interface(CRI)
- 프로세스:  GitLab Code Quality 작업
- 러너 유형:  GitLab Self-Managed 러너

## 빈 디렉터리 볼륨 추가 {#add-an-empty-directory-volume}

GitLab 러너 구성을 위한 빈 디렉터리를 정의하려면 `values.yaml` 파일의 runners 섹션에 다음 블록을 추가합니다:

```yaml
[[runners.kubernetes.volumes.empty_dir]]
  mount_path = "/var/lib"
  name = "docker-data"
```

### 예제 러너 구성 {#example-runner-configuration}

다음 예제는 수정 사항을 포함하는 GitLab 러너의 완전한 Helm 차트 `values.yaml`을 표시합니다:

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

## `.gitlab-ci.yml` 파일 업데이트 {#update-your-gitlab-ciyml-file}

기본 `overlay2` 드라이버를 선택 해제하려면 다음 키를 기존 Code Quality 작업에 빈 변수로 추가합니다:

```shell
DOCKER_DRIVER: ""
```

### 예제 Code Quality 작업 구성 {#example-code-quality-job-configuration}

`.gitlab-ci.yml` 파일에서 Code Quality 작업 구성을 다음 예제에 표시합니다:

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
