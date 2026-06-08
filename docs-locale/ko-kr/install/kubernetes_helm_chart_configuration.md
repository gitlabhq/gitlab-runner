---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab 러너 Helm 차트 구성
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab 러너 Helm 차트에 선택적 구성을 추가할 수 있습니다.

## 구성 템플릿을 통해 캐시 사용 {#use-the-cache-with-a-configuration-template}

구성 템플릿과 함께 캐시를 사용하려면 `values.yaml`에서 다음 변수를 설정합니다:

- `runners.cache.secretName`: 객체 스토리지 제공자의 시크릿 이름입니다. 옵션: `s3access`, `gcsaccess`, `google-application-credentials` 또는 `azureaccess`.
- `runners.config`: [캐시](../configuration/advanced-configuration.md#the-runnerscache-section)의 기타 설정(TOML 형식).

### Amazon S3 {#amazon-s3}

[Amazon S3을 정적 자격 증명으로 구성](https://aws.amazon.com/blogs/security/wheres-my-secret-access-key/)하려면:

1. 필요에 따라 값을 변경하여 이 예제를 `values.yaml`에 추가합니다:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "s3"
           Path = "runner"
           Shared = true
           [runners.cache.s3]
             ServerAddress = "s3.amazonaws.com"
             BucketName = "my_bucket_name"
             BucketLocation = "eu-west-1"
             Insecure = false
             AuthenticationType = "access-key"

     cache:
         secretName: s3access
   ```

1. `s3access` Kubernetes 시크릿을 생성하고 `accesskey` 및 `secretkey`을 포함합니다:

   ```shell
   kubectl create secret generic s3access \
       --from-literal=accesskey="YourAccessKey" \
       --from-literal=secretkey="YourSecretKey"
   ```

### Google Cloud Storage(GCS) {#google-cloud-storage-gcs}

Google Cloud Storage는 여러 가지 방법으로 정적 자격 증명으로 구성할 수 있습니다.

#### 정적 자격 증명 직접 구성 {#static-credentials-directly-configured}

GCS를 [액세스 ID 및 개인 키를 사용](../configuration/advanced-configuration.md#the-runnerscache-section)하여 구성하려면:

1. 필요에 따라 값을 변경하여 이 예제를 `values.yaml`에 추가합니다:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "gcs"
           Path = "runner"
           Shared = true
           [runners.cache.gcs]
             BucketName = "runners-cache"

     cache:
       secretName: gcsaccess
   ```

1. `gcsaccess` Kubernetes 시크릿을 생성하고 `gcs-access-id` 및 `gcs-private-key`을 포함합니다:

   ```shell
   kubectl create secret generic gcsaccess \
       --from-literal=gcs-access-id="YourAccessID" \
       --from-literal=gcs-private-key="YourPrivateKey"
   ```

#### GCP에서 다운로드한 JSON 파일의 정적 자격 증명 {#static-credentials-in-a-json-file-downloaded-from-gcp}

Google Cloud Platform에서 다운로드한 [GCS를 JSON 파일의 자격 증명으로 구성](../configuration/advanced-configuration.md#the-runnerscache-section)하려면:

1. 필요에 따라 값을 변경하여 이 예제를 `values.yaml`에 추가합니다:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "gcs"
           Path = "runner"
           Shared = true
           [runners.cache.gcs]
             BucketName = "runners-cache"

     cache:
         secretName: google-application-credentials

   secrets:
     - name: google-application-credentials
   ```

1. `google-application-credentials`이라는 Kubernetes 시크릿을 생성하고 JSON 파일을 로드합니다. 필요에 따라 경로를 변경합니다:

   ```shell
   kubectl create secret generic google-application-credentials \
       --from-file=gcs-application-credentials-file=./PATH-TO-CREDENTIALS-FILE.json
   ```

### Azure {#azure}

[Azure Blob Storage를 구성](../configuration/advanced-configuration.md#the-runnerscacheazure-section)하려면:

1. 필요에 따라 값을 변경하여 이 예제를 `values.yaml`에 추가합니다:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "azure"
           Path = "runner"
           Shared = true
           [runners.cache.azure]
             ContainerName = "CONTAINER_NAME"
             StorageDomain = "blob.core.windows.net"

     cache:
         secretName: azureaccess
   ```

1. `azureaccess` Kubernetes 시크릿을 생성하고 `azure-account-name` 및 `azure-account-key`을 포함합니다:

   ```shell
   kubectl create secret generic azureaccess \
       --from-literal=azure-account-name="YourAccountName" \
       --from-literal=azure-account-key="YourAccountKey"
   ```

Helm 차트 캐싱에 대한 자세한 내용은 [`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml)를 참조하세요.

### 영구 볼륨 클레임 {#persistent-volume-claim}

객체 스토리지 옵션 중 작동하는 옵션이 없으면 캐싱을 위해 영구 볼륨 클레임(PVC)을 사용할 수 있습니다.

캐시를 PVC로 사용하도록 구성하려면:

1. [PVC를 생성](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#persistentvolumeclaims)하고 작업 포드가 실행될 네임스페이스에서 생성합니다.

   > [!note]
   > 여러 작업 포드가 동일한 캐시 PVC에 액세스하려는 경우 `ReadWriteMany` 액세스 모드가 있어야 합니다.

1. PVC를 `/cache` 디렉터리에 마운트합니다:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [[runners.kubernetes.volumes.pvc]]
           name = "cache-pvc"
           mount_path = "/cache"
   ```

### 네트워크 파일 시스템 {#network-file-system}

객체 스토리지를 사용할 수 없을 때 캐싱을 위해 네트워크 파일 시스템(NFS)을 사용합니다.

전제 조건:

- NFS가 Kubernetes 클러스터에서 구성되고 액세스 가능합니다. 자세한 내용은 Kubernetes 설명서에서 [`nfs` 볼륨](https://kubernetes.io/docs/concepts/storage/volumes/#nfs)을 참조하세요.

NFS를 사용하도록 캐시를 구성하려면:

1. NFS 볼륨을 `/cache` 디렉터리에 마운트합니다:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [[runners.kubernetes.volumes.nfs]]
           name = "nfs"
           mount_path = "/cache"
           read_only = false
           server = "foo.bar.com"
           path = "/path/on/nfs-share"
   ```

## RBAC 지원 활성화 {#enable-rbac-support}

클러스터에 RBAC(역할 기반 액세스 제어)가 활성화되어 있으면 차트가 자체 서비스 계정을 생성할 수 있으거나 [기존 계정을 제공](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#service-account-permissions)할 수 있습니다.

- 차트가 서비스 계정을 생성하도록 하려면 `rbac.create`을 true로 설정합니다:

  ```yaml
  rbac:
    create: true
  ```

- 기존 서비스 계정을 사용하려면 `serviceAccount.name`을 설정합니다:

  ```yaml
  rbac:
    create: false
  serviceAccount:
    create: false
    name: your-service-account
  ```

## 최대 러너 동시성 제어 {#control-maximum-runner-concurrency}

Kubernetes에 배포된 단일 러너는 추가 Runner 포드를 시작하여 여러 작업을 병렬로 실행할 수 있습니다. 한 번에 허용되는 최대 포드 수를 변경하려면 [`concurrent` 설정](../configuration/advanced-configuration.md#the-global-section)을 편집합니다. 기본값은 `10`입니다:

```yaml
## Configure the maximum number of concurrent jobs
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration/#the-global-section
##
concurrent: 10
```

이 설정에 대한 자세한 내용은 GitLab 러너의 고급 구성 설명서에서 [전역 섹션](../configuration/advanced-configuration.md#the-global-section)을 참조하세요.

## GitLab 러너에서 Docker-in-Docker 컨테이너 실행 {#run-docker-in-docker-containers-with-gitlab-runner}

GitLab 러너에서 Docker-in-Docker 컨테이너를 사용하려면:

- 활성화하려면 [러너를 위한 권한 있는 컨테이너 사용](#use-privileged-containers-for-the-runners)을 참조하세요.
- Docker-in-Docker 실행에 대한 지침은 [GitLab 러너 설명서](../executors/kubernetes/_index.md#using-docker-in-builds)를 참조하세요.

## 러너를 위한 권한 있는 컨테이너 사용 {#use-privileged-containers-for-the-runners}

GitLab CI/CD 작업에서 Docker 실행 파일을 사용하려면 러너를 권한 있는 컨테이너를 사용하도록 구성합니다.

전제 조건:

- 위험성을 이해하고 있으며, 이는 [GitLab CI/CD 러너 설명서](../executors/kubernetes/_index.md#using-docker-in-builds)에 설명되어 있습니다.
- GitLab 러너 인스턴스가 GitLab의 특정 프로젝트에 등록되어 있으며 CI/CD 작업을 신뢰합니다.

`values.yaml`에서 권한 있는 모드를 활성화하려면 다음 줄을 추가합니다:

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        # Run all containers with the privileged flag enabled.
        privileged = true
        ...
```

자세한 내용은 [`[runners.kubernetes]`](../configuration/advanced-configuration.md#the-runnerskubernetes-section) 섹션의 고급 구성 정보를 참조하세요.

## 개인 레지스트리에서 이미지 사용 {#use-an-image-from-a-private-registry}

개인 레지스트리에서 이미지를 사용하려면 `imagePullSecrets`을 구성합니다.

1. CI/CD 작업에 사용되는 Kubernetes 네임스페이스에 하나 이상의 시크릿을 생성합니다. 이 명령은 `image_pull_secrets`과 함께 작동하는 시크릿을 생성합니다:

   ```shell
   kubectl create secret docker-registry <SECRET_NAME> \
     --namespace <NAMESPACE> \
     --docker-server="https://<REGISTRY_SERVER>" \
     --docker-username="<REGISTRY_USERNAME>" \
     --docker-password="<REGISTRY_PASSWORD>"
   ```

1. GitLab 러너 Helm 차트 버전 0.53.x 이상의 경우, `config.toml`에서 `runners.config`에 제공된 템플릿에서 `image_pull_secret`을 설정합니다:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           ## Specify one or more imagePullSecrets
           ## ref: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
           ##
           image_pull_secrets = [your-image-pull-secret]
   ```

   자세한 내용은 Kubernetes 설명서에서 [개인 레지스트리에서 이미지 가져오기](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)를 참조하세요.

1. GitLab 러너 Helm 차트 버전 0.52 이상의 경우, `values.yaml`에서 `runners.imagePullSecrets`의 값을 설정합니다. 이 값을 설정하면 컨테이너가 `--kubernetes-image-pull-secrets "<SECRET_NAME>"`을 이미지 진입점 스크립트에 추가합니다. 이렇게 하면 Kubernetes 실행기 `config.toml` 설정에서 `image_pull_secrets` 매개변수를 구성할 필요가 없습니다.

   ```yaml
   runners:
     imagePullSecrets: [your-image-pull-secret]
   ```

> [!note]
> `imagePullSecrets`의 값은 Kubernetes 리소스의 규칙과 같이 `name` 태그로 접두사가 붙지 않습니다. 이 값은 하나의 레지스트리 자격 증명만 사용하더라도 하나 이상의 시크릿 이름 배열이 필요합니다.

`imagePullSecrets`을 생성하는 방법에 대한 자세한 내용은 Kubernetes 설명서에서 [개인 레지스트리에서 이미지 가져오기](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)를 참조하세요.

작업 Pod가 생성되면 GitLab 러너는 자동으로 두 단계에서 이미지 액세스를 처리합니다:

1. GitLab 러너는 기존 Docker 자격 증명을 Kubernetes 시크릿으로 변환하여 레지스트리에서 이미지를 가져올 수 있습니다. 또한 수동으로 구성된 imagePullSecrets이 클러스터에 실제로 존재하는지 확인합니다. 정적으로 정의된 자격 증명, 자격 증명 저장소 또는 자격 증명 도우미에 대한 자세한 내용은 [개인 컨테이너 레지스트리에서 이미지 액세스](https://docs.gitlab.com/ci/docker/using_docker_images/#access-an-image-from-a-private-container-registry)를 참조하세요.
1. GitLab 러너는 작업 Pod를 생성하고 `imagePullSecrets` 및 변환된 Docker 자격 증명을 순서대로 연결합니다.

Kubernetes가 컨테이너 이미지를 가져와야 할 때 작동하는 자격 증명을 찾을 때까지 자격 증명을 하나씩 시도합니다.

## 사용자 정의 인증서를 사용하여 GitLab에 액세스 {#access-gitlab-with-a-custom-certificate}

사용자 정의 인증서를 사용하려면 [Kubernetes 시크릿](https://kubernetes.io/docs/concepts/configuration/secret/)을 GitLab 러너 Helm 차트에 제공합니다. 이 시크릿은 컨테이너의 `/home/gitlab-runner/.gitlab-runner/certs` 디렉터리에 추가됩니다:

1. [인증서 준비](#prepare-your-certificate)
1. [Kubernetes 시크릿 생성](#create-a-kubernetes-secret)
1. [차트에 시크릿 제공](#provide-the-secret-to-the-chart)

### 인증서 준비 {#prepare-your-certificate}

Kubernetes 시크릿의 각 키 이름은 디렉터리의 파일 이름으로 사용되며, 파일 내용은 키와 연결된 값입니다:

- 사용할 파일 이름은 `<gitlab.hostname>.crt` 형식이어야 하며, 예를 들어 `gitlab.your-domain.com.crt`입니다.
- 중간 인증서를 서버 인증서와 함께 같은 파일에 연결합니다.
- 사용되는 호스트 이름은 인증서가 등록된 호스트 이름이어야 합니다.

### Kubernetes 시크릿 생성 {#create-a-kubernetes-secret}

GitLab Helm 차트를 [자동 생성된 자체 서명 와일드카드 인증서](https://docs.gitlab.com/charts/installation/tls/#option-4-use-auto-generated-self-signed-wildcard-certificate) 방법을 사용하여 설치한 경우 시크릿이 생성되었습니다.

자동 생성된 자체 서명 와일드카드 인증서를 사용하여 GitLab Helm 차트를 설치하지 않은 경우 시크릿을 생성합니다. 이 명령은 Kubernetes에 인증서를 시크릿으로 저장하고 이를 GitLab 러너 컨테이너에 파일로 제시합니다.

- 인증서가 현재 디렉터리에 있고 `<gitlab.hostname.crt>` 형식을 따르는 경우 필요에 따라 이 명령을 수정합니다:

  ```shell
  kubectl create secret generic <SECRET_NAME> \
    --namespace <NAMESPACE> \
    --from-file=<CERTIFICATE_FILENAME>
  ```

  - `<NAMESPACE>`: GitLab 러너를 설치하려는 Kubernetes 네임스페이스입니다.
  - `<SECRET_NAME>`: `gitlab-domain-cert`와 같은 Kubernetes Secret 리소스 이름입니다.
  - `<CERTIFICATE_FILENAME>`: 시크릿으로 가져올 현재 디렉터리의 인증서 파일 이름입니다.
- 인증서가 다른 디렉터리에 있거나 `<gitlab.hostname.crt>` 형식을 따르지 않는 경우 대상으로 사용할 파일 이름을 지정해야 합니다:

  ```shell
  kubectl create secret generic <SECRET_NAME> \
    --namespace <NAMESPACE> \
    --from-file=<TARGET_FILENAME>=<CERTIFICATE_FILENAME>
  ```

  - `<TARGET_FILENAME>`은 `gitlab.hostname.crt`와 같은 Runner 컨테이너에 표시되는 인증서 파일의 이름입니다.
  - `<CERTIFICATE_FILENAME>`은 현재 디렉터리를 기준으로 한 인증서의 파일 이름으로 시크릿으로 가져옵니다. 예: `cert-directory/my-gitlab-certificate.crt`.

### 차트에 시크릿 제공 {#provide-the-secret-to-the-chart}

`values.yaml`에서 `certsSecretName`을 동일한 네임스페이스의 Kubernetes 시크릿 객체의 리소스 이름으로 설정합니다. 이렇게 하면 GitLab 러너가 사용할 사용자 정의 인증서를 전달할 수 있습니다. 이전 예제에서 리소스 이름은 `gitlab-domain-cert`입니다:

```yaml
certsSecretName: <SECRET NAME>
```

자세한 내용은 GitLab 서버를 대상으로 하는 [자체 서명 인증서의 지원되는 옵션](../configuration/tls-self-signed.md#supported-options-for-self-signed-certificates-targeting-the-gitlab-server)을 참조하세요.

## Pod 레이블을 CI 환경 변수 키로 설정 {#set-pod-labels-to-ci-environment-variable-keys}

`values.yaml` 파일에서 환경 변수를 Pod 레이블로 사용할 수 없습니다. 자세한 내용은 [Pod 레이블로 환경 변수 키를 설정할 수 없음](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/173)을 참조하세요. [이슈에 설명된 해결 방법](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/173#note_351057890)을 임시 솔루션으로 사용합니다.

## Ubuntu 기반 `gitlab-runner` Docker 이미지로 전환 {#switch-to-the-ubuntu-based-gitlab-runner-docker-image}

기본적으로 GitLab 러너 Helm 차트는 `musl libc`를 사용하는 `gitlab/gitlab-runner` 이미지의 Alpine 버전을 사용합니다. `glibc`을 사용하는 Ubuntu 기반 이미지로 전환해야 할 수도 있습니다.

이렇게 하려면 `values.yaml` 파일에서 이미지를 지정하고 다음 값을 사용합니다:

```yaml
# Specify the Ubuntu image, and set the version. You can also use the `ubuntu` or `latest` tags.
image: gitlab/gitlab-runner:v17.3.0

# Update the security context values to the user ID in the Ubuntu image
securityContext:
  fsGroup: 999
  runAsUser: 999
```

## 루트가 아닌 사용자로 실행 {#run-with-non-root-user}

기본적으로 GitLab 러너 이미지는 루트가 아닌 사용자와 함께 작동하지 않습니다. [GitLab 러너 UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766421) 및 [GitLab 러너 Helper UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766433) 이미지는 해당 시나리오를 위해 설계되었습니다.

사용하려면 `values.yaml`에서 GitLab 러너 및 GitLab 러너 Helper 이미지를 변경합니다:

```yaml
image:
  registry: registry.gitlab.com
  image: gitlab-org/ci-cd/gitlab-runner-ubi-images/gitlab-runner-ocp
  tag: v16.11.0

securityContext:
    runAsNonRoot: true
    runAsUser: 999

runners:
    config: |
        [[runners]]
          [runners.kubernetes]
            helper_image = "registry.gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/gitlab-runner-helper-ocp:x86_64-v16.11.0"
            [runners.kubernetes.pod_security_context]
              run_as_non_root = true
              run_as_user = 59417
```

`run_as_user`이 `nonroot` 사용자(59417)의 사용자 ID를 가리키더라도 이미지는 모든 사용자 ID와 함께 작동합니다. 이 사용자 ID가 루트 그룹의 일부인 것이 중요합니다. 루트 그룹의 일부인 것이 특정 권한을 부여하지는 않습니다.

## FIPS 호환 GitLab 러너 사용 {#use-a-fips-compliant-gitlab-runner}

[FIPS 호환 GitLab 러너](requirements.md#fips-compliant-gitlab-runner)를 사용하려면 `values.yaml`에서 GitLab 러너 이미지 및 Helper 이미지를 변경합니다:

```yaml
image:
  registry: docker.io
  image: gitlab/gitlab-runner
  tag: ubi-fips

runners:
    config: |
        [[runners]]
          [runners.kubernetes]
            helper_image_flavor = "ubi-fips"
```

## 구성 템플릿 사용 {#use-a-configuration-template}

[GitLab 러너 빌드 포드의 동작을 Kubernetes에서 구성](../executors/kubernetes/_index.md#configuration-settings) 하려면 [구성 템플릿 파일](../register/_index.md#register-with-a-configuration-template)을 사용합니다. 구성 템플릿은 Helm 차트와 특정 러너 구성 옵션을 공유하지 않고 러너의 모든 필드를 구성할 수 있습니다. 예를 들어, 이러한 기본 설정은 `chart` 리포지토리의 [`values.yaml` 파일에 있습니다](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml):

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        image = "ubuntu:22.04"
```

`config:` 섹션의 값은 TOML을 사용해야 하며, `values.yaml`에 포함된 `config.toml`이므로 `<parameter>: <value>` 대신 `<parameter> = <value>`를 사용해야 합니다.

실행기 관련 구성을 보려면 [`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml) 파일을 참조하세요.
