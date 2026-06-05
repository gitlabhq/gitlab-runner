---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 작업 실행 속도 높이기
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

이미지와 종속성을 캐시하여 작업 성능을 개선할 수 있습니다.

## 컨테이너용 프록시 사용 {#use-a-proxy-for-containers}

다음 중 하나를 사용하여 Docker 이미지를 다운로드하는 데 걸리는 시간을 단축할 수 있습니다:

- GitLab 종속성 프록시 또는
- DockerHub 레지스트리의 미러
- 기타 오픈 소스 솔루션

### GitLab 종속성 프록시 {#gitlab-dependency-proxy}

더 빠르게 컨테이너 이미지에 액세스하려면 [종속성 프록시를 사용](https://docs.gitlab.com/user/packages/dependency_proxy/)하여 컨테이너 이미지를 프록시할 수 있습니다.

### Docker Hub 레지스트리 미러 {#docker-hub-registry-mirror}

Docker Hub를 미러링하여 작업이 컨테이너 이미지에 액세스하는 데 걸리는 시간을 단축할 수도 있습니다. 이는 [패스스루 캐시로서의 레지스트리](https://docs.docker.com/docker-hub/image-library/mirror/)를 제공합니다. 작업 실행 속도를 높이는 것 외에도 미러는 Docker Hub 중단 및 Docker Hub 속도 제한에 대한 인프라의 복원력을 높일 수 있습니다.

Docker 데몬이 [미러를 사용하도록 구성](https://docs.docker.com/docker-hub/image-library/mirror/#configure-the-docker-daemon)되면 미러의 실행 중인 인스턴스에서 이미지를 자동으로 확인합니다. 사용할 수 없는 경우 공개 Docker 레지스트리에서 이미지를 가져와 로컬에 저장한 후 다시 전달합니다.

동일한 이미지에 대한 다음 요청은 로컬 레지스트리에서 가져옵니다.

작동 방식에 대한 자세한 내용은 [Docker 데몬 구성 설명서](https://docs.docker.com/docker-hub/image-library/mirror/#configure-the-docker-daemon)를 참조하세요.

#### Docker Hub 레지스트리 미러 사용 {#use-a-docker-hub-registry-mirror}

Docker Hub 레지스트리 미러를 생성하려면:

1. 프록시 컨테이너 레지스트리가 실행될 전용 머신에 로그인합니다.
1. [Docker Engine](https://docs.docker.com/get-started/get-docker/)이 해당 머신에 설치되어 있는지 확인합니다.
1. 새 컨테이너 레지스트리를 생성합니다:

   ```shell
   docker run -d -p 6000:5000 \
       -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
       --restart always \
       --name registry registry:2
   ```

   포트 번호(`6000`)를 수정하여 다른 포트에서 레지스트리를 노출할 수 있습니다. `http`로 서버를 시작합니다. TLS(`https`)를 켜고 싶으면 [공식 설명서](https://distribution.github.io/distribution/about/configuration/#tls)를 참조하세요.

1. 서버의 IP 주소를 확인합니다:

   ```shell
   hostname --ip-address
   ```

   개인 네트워크 IP 주소를 선택해야 합니다. 개인 네트워크는 일반적으로 DigitalOcean, AWS 또는 Azure와 같은 단일 공급자의 머신 간 내부 통신을 위한 가장 빠른 솔루션입니다. 일반적으로 개인 네트워크에서 전송되는 데이터는 월별 대역폭 제한에 적용되지 않습니다.

Docker Hub 레지스트리는 `MY_REGISTRY_IP:6000` 아래에서 액세스할 수 있습니다.

이제 새 레지스트리 서버를 사용하도록 [`config.toml`를 구성](autoscale.md#distributed-container-registry-mirroring)할 수 있습니다.

### 기타 오픈 소스 솔루션 {#other-open-source-solutions}

- [`rpardini/docker-registry-proxy`](https://github.com/rpardini/docker-registry-proxy)는 GitLab 컨테이너 레지스트리를 포함하여 대부분의 컨테이너 레지스트리를 로컬에서 프록시할 수 있습니다.

## 분산 캐시 사용 {#use-a-distributed-cache}

분산 [캐시](https://docs.gitlab.com/ci/yaml/#cache)를 사용하여 언어 종속성을 다운로드하는 데 걸리는 시간을 단축할 수 있습니다.

분산 캐시를 지정하려면 캐시 서버를 설정한 다음 [러너가 해당 캐시 서버를 사용하도록 구성](advanced-configuration.md#the-runnerscache-section)합니다.

자동 크기 조정을 사용하는 경우, 분산 러너 [캐시 기능](autoscale.md#distributed-runners-caching)에 대해 자세히 알아보세요.

지원되는 캐시 서버는 다음과 같습니다:

- [AWS S3](#use-aws-s3)
- [MinIO](#use-minio) 또는 기타 S3 호환 캐시 서버
- [Google Cloud Storage](#use-google-cloud-storage)
- [Azure Blob Storage](#use-azure-blob-storage)

GitLab CI/CD [캐시 종속성 및 모범 사례](https://docs.gitlab.com/ci/caching/)에 대해 자세히 알아보세요.

### AWS S3 사용 {#use-aws-s3}

AWS S3를 분산 캐시로 사용하려면 [러너의 `config.toml` 파일을 편집](advanced-configuration.md#the-runnerscaches3-section)하여 S3 위치를 가리키고 연결을 위한 자격 증명을 제공합니다. 러너가 S3 엔드포인트로의 네트워크 경로를 가지고 있는지 확인합니다.

NAT 게이트웨이가 있는 개인 서브넷을 사용하는 경우, 데이터 전송 비용을 절감하기 위해 S3 VPC 엔드포인트를 활성화할 수 있습니다.

### MinIO 사용 {#use-minio}

AWS S3를 사용하는 대신 자신의 캐시 저장소를 만들 수 있습니다.

1. 캐시 서버가 실행될 전용 머신에 로그인합니다.
1. [Docker Engine](https://docs.docker.com/get-started/get-docker/)이 해당 머신에 설치되어 있는지 확인합니다.
1. [MinIO](https://www.min.io), Go로 작성된 간단한 S3 호환 서버를 시작합니다:

   ```shell
   docker run -d --restart always -p 9005:9000 \
           -v /.minio:/root/.minio -v /export:/export \
           -e "MINIO_ROOT_USER=<minio_root_username>" \
           -e "MINIO_ROOT_PASSWORD=<minio_root_password>" \
           --name minio \
           minio/minio:latest server /export
   ```

   포트 `9005`를 수정하여 다른 포트에서 캐시 서버를 노출할 수 있습니다.

1. 서버의 IP 주소를 확인합니다:

   ```shell
   hostname --ip-address
   ```

1. 캐시 서버는 `MY_CACHE_IP:9005`에서 사용할 수 있습니다.
1. 러너가 사용할 버킷을 생성합니다:

   ```shell
   sudo mkdir /export/runner
   ```

   `runner`은 이 경우 버킷의 이름입니다. 다른 버킷을 선택하면 다를 것입니다. 모든 캐시는 `/export` 디렉터리에 저장됩니다.

1. `MINIO_ROOT_USER` 및 `MINIO_ROOT_PASSWORD` 값(위에서)을 러너를 구성할 때 액세스 키 및 비밀 키로 사용합니다.

이제 새 캐시 서버를 사용하도록 [`config.toml`를 구성](autoscale.md#distributed-runners-caching)할 수 있습니다.

### Google Cloud Storage 사용 {#use-google-cloud-storage}

Google Cloud Platform을 분산 캐시로 사용하려면 [러너의 `config.toml` 파일을 편집](advanced-configuration.md#the-runnerscachegcs-section)하여 GCP 위치를 가리키고 연결을 위한 자격 증명을 제공합니다. 러너가 GCS 엔드포인트로의 네트워크 경로를 가지고 있는지 확인합니다.

### Azure Blob Storage 사용 {#use-azure-blob-storage}

Azure Blob Storage를 분산 캐시로 사용하려면 [러너의 `config.toml` 파일을 편집](advanced-configuration.md#the-runnerscacheazure-section)하여 Azure 위치를 가리키고 연결을 위한 자격 증명을 제공합니다. 러너가 Azure 엔드포인트로의 네트워크 경로를 가지고 있는지 확인합니다.

### 캐시 및 아티팩트 전송 속도 높이기 {#speed-up-cache-and-artifact-transfers}

다음 옵션으로 캐시 및 아티팩트 업로드 및 다운로드 성능을 개선할 수 있습니다.

#### 백엔드별 러너 구성 {#backend-specific-runner-config}

각 캐시 백엔드에는 자체 `config.toml` 섹션이 있습니다. 백엔드에 맞게 최적화합니다:

- [S3 구성](advanced-configuration.md#the-runnerscaches3-section)):  `BucketLocation`를 러너와 같은 리전으로 설정합니다. 5GB보다 큰 아카이브의 경우 `RoleARN`를 사용하여 [멀티파트 업로드 활성화](advanced-configuration.md#enable-multipart-transfers-with-rolearn)합니다. 기본 S3 v2 어댑터를 사용합니다(`FF_USE_LEGACY_S3_CACHE_ADAPTER=true`를 설정하지 마세요). 러너가 버킷 리전에서 멀리 떨어져 있을 때 `Accelerate = true`을 선택적으로 활성화하여 [AWS S3 전송 가속화](https://docs.aws.amazon.com/AmazonS3/latest/userguide/transfer-acceleration.html)를 사용합니다. 동일한 리전의 [S3 VPC 엔드포인트](https://docs.aws.amazon.com/AmazonS3/latest/userguide/creating-s3-vpc-endpoint.html)는 지연 시간과 비용을 줄일 수 있습니다.
- [Google Cloud Storage 구성](advanced-configuration.md#the-runnerscachegcs-section)):  러너와 같은 또는 가장 가까운 리전의 버킷을 사용합니다.
- [Azure Blob 구성](advanced-configuration.md#the-runnerscacheazure-section)):  러너와 같은 또는 가장 가까운 리전의 스토리지 계정을 사용합니다.

#### 캐시 압축 {#cache-compression}

더 빠른 압축을 사용하여 캐시 아카이빙 및 다운로드 속도를 높입니다. 이렇게 하면 더 큰 아카이브가 생성됩니다. [CI/CD 변수](https://docs.gitlab.com/ee/ci/variables/)에서 작업 또는 압축 옵션을 설정합니다:

| 변수 | 속도 권장 | 설명 |
|----------|------------------------|-------------|
| `CACHE_COMPRESSION_LEVEL` | `fastest` 또는 `fast` | CPU가 감소하고 업로드 또는 다운로드가 더 빨라집니다. 아카이브가 더 큽니다. 기본값은 `default`입니다. |
| `CACHE_COMPRESSION_FORMAT` | `zip` | `zip`은(는) 생성 속도가 더 빠릅니다. `tarzstd`는 더 나은 압축 비율을 제공하지만 더 느릴 수 있습니다. |

`.gitlab-ci.yml`의 예제 구성:

```yaml
variables:
  CACHE_COMPRESSION_LEVEL: fastest
  CACHE_COMPRESSION_FORMAT: zip
```

#### 캐시 요청 타임아웃 {#cache-request-timeout}

대용량 캐시가 타임아웃되면 `CACHE_REQUEST_TIMEOUT` [CI/CD 변수](https://docs.gitlab.com/ee/ci/variables/)를 사용하여 제한값(분)을 늘립니다. 기본값은 `10`입니다. 이 설정은 전송 속도를 높이지 않지만 느리거나 대용량 업로드 및 다운로드 실패를 방지합니다.

#### 캐시 전송 버퍼 크기(처리량) {#cache-transfer-buffer-size-throughput}

캐시 다운로드 및 업로드는 단일 스트리밍 버퍼를 사용합니다. 더 큰 버퍼는 시스템 호출을 줄이고 처리량을 증가시키며, 특히 전송이 20~30MB/s 정도로 제한되는 경우 그렇습니다.

작업 환경 또는 [CI/CD 변수](https://docs.gitlab.com/ee/ci/variables/)에서 `CACHE_TRANSFER_BUFFER_SIZE`을(를) (바이트 단위로) 설정합니다. 기본값은 4MiB(4194304)입니다.

8MiB에 대한 예제 구성:

```yaml
variables:
  CACHE_TRANSFER_BUFFER_SIZE: "8388608"
```

#### 캐시 청크 크기 및 동시성 {#cache-chunk-size-and-concurrency}

청크 크기는 병렬 업로드(GoCloud) 또는 병렬 다운로드(사전 서명됨 또는 GoCloud)에 대한 각 파트 또는 청크의 바이트 크기입니다. 동시성은 병렬로 실행되는 청크의 수입니다. 메모리 사용량은 대략 청크 크기 x 동시성입니다.

| 변수 | 설명 | 기본값 |
|----------|-------------|---------|
| `CACHE_CHUNK_SIZE` | 바이트 단위 청크 크기입니다. 업로드의 경우(GoCloud 백엔드): 제한값은 백엔드에 따라 다릅니다(예: 파트당 5MiB~5GiB, S3의 최대 10,000개 파트, Azure 및 GCS는 자체 제한값 있음). 다운로드:  0 = 레거시 순차; 동시성 > 1일 때 설정되지 않으면 16MiB 사용. | 업로드:  16MiB(16777216). 다운로드:  0(레거시) |
| `CACHE_CONCURRENCY` | 동시 청크 수입니다. 업로드:  GoCloud 백엔드만 해당(S3 with RoleARN, Azure, GCS). 다운로드:  0 또는 1 = 레거시 순차 모드; 1보다 큼 = 병렬 모드(사전 서명됨 또는 GoCloud). | 업로드:  16\. 다운로드:  0(레거시) |

사용자 지정 튜닝에 대한 예제 구성(예: 32MiB 청크, 32개 동시):

```yaml
variables:
  CACHE_CHUNK_SIZE: "33554432"
  CACHE_CONCURRENCY: "32"
```

#### GitLab으로의 아티팩트 업로드 {#artifact-uploads-to-gitlab}

GitLab은 아티팩트를 GitLab 코디네이터로 보내며, 이는 아티팩트를 객체 스토리지에 저장할 수 있습니다. 러너에서 업로드 속도를 높이려면:

| 변수 | 속도 권장 | 설명 |
|----------|------------------------|-------------|
| `ARTIFACT_COMPRESSION_LEVEL` | `fastest` 또는 `fast` | CPU 및 업로드 전 압축에 소요되는 시간을 줄입니다. |

작업 또는 CI/CD 변수에서 압축 옵션을 설정합니다(예):

```yaml
variables:
  ARTIFACT_COMPRESSION_LEVEL: fastest
```

#### 객체 스토리지에서의 아티팩트 다운로드 {#artifact-downloads-from-object-storage}

코디네이터가 아티팩트 다운로드를 객체 스토리지로 리디렉션할 때(`direct_download`), `FF_USE_PARALLEL_ARTIFACT_TRANSFER` [기능 플래그](feature-flags.md)로 병렬 범위 다운로드를 활성화할 수 있습니다. 이는 병렬 캐시 전송(`FF_USE_PARALLEL_CACHE_TRANSFER`)과 별도입니다. [병렬 아티팩트 다운로드(직접 다운로드)](advanced-configuration.md#parallel-artifact-downloads-direct-download)를 참조하세요.
