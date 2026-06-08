---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Configurer GitLab Runner pour Oracle Cloud Infrastructure
---

Les jobs GitLab Code Quality qui s'exécutent dans des environnements Oracle Cloud Infrastructure (OCI) avec Container Runtime Interface (CRI) peuvent subir une dégradation des performances.

Pour optimiser les performances de votre GitLab Runner dans OCI :

1. Ajoutez un volume de répertoire vide à votre configuration GitLab Runner.
1. Configurez des paramètres spécifiques du pilote Docker dans votre fichier `.gitlab-ci.yml`.

Cette configuration s'applique aux environnements avec :

- Fournisseur cloud : Oracle Cloud Infrastructure (OCI)
- Runtime de conteneur : Container Runtime Interface (CRI)
- Processus : jobs GitLab Code Quality
- Type de runner : GitLab Self-Managed Runners

## Ajouter un volume de répertoire vide {#add-an-empty-directory-volume}

Pour définir un répertoire vide pour la configuration de GitLab Runner, ajoutez le bloc suivant à la section runners de votre fichier `values.yaml` :

```yaml
[[runners.kubernetes.volumes.empty_dir]]
  mount_path = "/var/lib"
  name = "docker-data"
```

### Exemple de configuration de runner {#example-runner-configuration}

L'exemple suivant montre un chart Helm `values.yaml` complet pour le GitLab Runner incluant le correctif :

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

## Mettre à jour votre fichier `.gitlab-ci.yml` {#update-your-gitlab-ciyml-file}

Pour désélectionner le pilote `overlay2` par défaut, ajoutez la clé suivante en tant que variable vide à votre job Code Quality existant :

```shell
DOCKER_DRIVER: ""
```

### Exemple de configuration de job Code Quality {#example-code-quality-job-configuration}

L'exemple suivant montre la configuration du job Code Quality dans votre fichier `.gitlab-ci.yml` :

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
