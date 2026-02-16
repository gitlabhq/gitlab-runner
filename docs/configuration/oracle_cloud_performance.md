---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Configure GitLab Runner for Oracle Cloud Infrastructure
---

GitLab Code Quality jobs that run in Oracle Cloud Infrastructure (OCI) environments with Container Runtime Interface (CRI) can experience performance degradation.

To optimize your GitLab Runner performance in OCI:

1. Add an empty directory volume to your GitLab Runner configuration.
1. Configure specific Docker driver settings in your `.gitlab-ci.yml` file.

This configuration applies to environments with:

- Cloud provider: Oracle Cloud Infrastructure (OCI)
- Container runtime: Container Runtime Interface (CRI)
- Process: GitLab Code Quality jobs
- Runner type: GitLab Self-Managed Runners

## Add an empty directory volume

To define an empty directory for GitLab Runner configuration, add the following block to the runners section of your `values.yaml` file:

```yaml
[[runners.kubernetes.volumes.empty_dir]]
  mount_path = "/var/lib"
  name = "docker-data"
```

### Example runner configuration

The following example shows a complete Helm chart `values.yaml` for the GitLab Runner that includes the fix:

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

## Update your `.gitlab-ci.yml` file

To unselect the default `overlay2` driver, add the following key as an empty variable to your existing Code Quality job:

```shell
DOCKER_DRIVER: ""
```

### Example Code Quality job configuration

The following example shows Code Quality job configuration in your `.gitlab-ci.yml` file:

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
