---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Docker 실행기
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab 러너는 Docker 실행기를 사용하여 Docker 이미지에서 작업을 실행합니다.

Docker 실행기를 사용하여 다음을 수행할 수 있습니다:

- 각 작업에 대해 동일한 빌드 환경을 유지합니다.
- CI 서버에서 작업을 실행할 필요 없이 동일한 이미지를 사용하여 로컬에서 명령어를 테스트합니다.

Docker 실행기는 [Docker Engine](https://www.docker.com/products/container-runtime/)을 사용하여 각 작업을 별도의 격리된 컨테이너에서 실행합니다. Docker Engine에 연결하기 위해 실행기는 다음을 사용합니다:

- [`.gitlab-ci.yml`](https://docs.gitlab.com/ci/yaml/)에서 정의하는 이미지 및 서비스입니다.
- [`config.toml`](../commands/_index.md#configuration-file)에서 정의하는 구성입니다.

`config.toml`에서 기본 이미지를 정의하지 않고는 러너 및 Docker 실행기를 등록할 수 없습니다. `config.toml`에서 정의된 이미지는 `.gitlab-ci.yml`에서 정의된 이미지가 없을 때 사용할 수 있습니다. `.gitlab-ci.yml`에서 이미지를 정의하면 `config.toml`에서 정의한 이미지를 재정의합니다.

전제 조건:

- [Docker 설치](https://docs.docker.com/engine/install/)

## Docker 실행기 워크플로우 {#docker-executor-workflow}

Docker 실행기는 [Alpine Linux](https://alpinelinux.org/)를 기반으로 하는 Docker 이미지를 사용하며, prepare, pre-작업, post-작업 단계를 실행할 도구를 포함합니다. 특수 Docker 이미지의 정의를 보려면 [GitLab 러너 리포지토리](https://gitlab.com/gitlab-org/gitlab-runner/-/tree/main/dockerfiles/runner-helper)를 참조하세요.

Docker 실행기는 작업을 여러 단계로 나눕니다:

1. **Prepare**:  [서비스](https://docs.gitlab.com/ci/yaml/#services)를 생성하고 시작합니다.
1. **Pre-job**:  이전 스테이지에서 [캐시](https://docs.gitlab.com/ci/yaml/#cache) 를 복제, 복원하고 [아티팩트](https://docs.gitlab.com/ci/yaml/#artifacts)를 다운로드합니다. 특수 Docker 이미지에서 실행됩니다.
1. **작업**:  러너에 대해 구성한 Docker 이미지에서 빌드를 실행합니다.
1. **Post-job**:  캐시를 생성하고 아티팩트를 GitLab에 업로드합니다. 특수 Docker 이미지에서 실행됩니다.

## 지원되는 구성 {#supported-configurations}

Docker 실행기는 다음 구성을 지원합니다.

알려진 문제 및 Windows 구성의 추가 요구사항은 [Windows 컨테이너 사용](#use-windows-containers)을 참조하세요.

| 러너가 설치된 위치: | 실행기:     | 컨테이너 실행 위치: |
|-------------------------|------------------|-----------------------|
| Windows                 | `docker-windows` | Windows               |
| Windows                 | `docker`         | Linux                 |
| Linux                   | `docker`         | Linux                 |
| macOS                   | `docker`         | Linux                 |

이러한 구성은 **not**:

| 러너가 설치된 위치: | 실행기:     | 컨테이너 실행 위치: |
|-------------------------|------------------|-----------------------|
| Linux                   | `docker-windows` | Linux                 |
| Linux                   | `docker`         | Windows               |
| Linux                   | `docker-windows` | Windows               |
| Windows                 | `docker`         | Windows               |
| Windows                 | `docker-windows` | Linux                 |

> [!note]
> GitLab 러너는 Docker Engine API [v1.25](https://docs.docker.com/reference/api/engine/version/v1.25/)를 사용하여 Docker Engine과 통신합니다. 이는 Linux 서버의 Docker의 [최소 지원 버전](https://docs.docker.com/reference/api/engine/#api-version-matrix)이 `1.13.0`임을 의미합니다. Windows Server의 경우 [더 최신 버전이 필요합니다](#supported-docker-versions). Windows Server 버전을 식별하기 위해서 말입니다.

## Docker 실행기 사용 {#use-the-docker-executor}

Docker 실행기를 사용하려면 `config.toml`에서 Docker를 실행기로 수동으로 정의하거나 [`gitlab-runner register --executor "docker"`](../register/_index.md#register-with-a-runner-authentication-token) 명령어를 사용하여 자동으로 정의합니다.

다음 샘플 구성은 실행기로 정의된 Docker를 보여줍니다. 이러한 값에 대한 자세한 내용은 [고급 구성](../configuration/advanced-configuration.md)을 참조하세요.

```toml
concurrent = 4

[[runners]]
name = "myRunner"
url = "https://gitlab.com/ci"
token = "......"
executor = "docker"
[runners.docker]
  tls_verify = true
  image = "my.registry.tld:5000/alpine:latest"
  privileged = false
  disable_entrypoint_overwrite = false
  oom_kill_disable = false
  disable_cache = false
  volumes = [
    "/cache",
  ]
  shm_size = 0
  allowed_pull_policies = ["always", "if-not-present"]
  allowed_images = ["my.registry.tld:5000/*:*"]
  allowed_services = ["my.registry.tld:5000/*:*"]
  [runners.docker.volume_driver_ops]
    "size" = "50G"
```

## 이미지 및 서비스 구성 {#configure-images-and-services}

전제 조건:

- 작업이 실행되는 이미지에는 운영 체제 `PATH`에 작동하는 셸이 있어야 합니다. 지원되는 셸은 다음과 같습니다:
  - Linux의 경우:
    - `sh`
    - `bash`
    - PowerShell Core (`pwsh`)입니다. [13.9에서 도입됨](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4021)입니다.
  - Windows의 경우:
    - PowerShell (`powershell`)
    - PowerShell Core (`pwsh`)입니다. [13.6에서 도입됨](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/13139)입니다.

Docker 실행기를 구성하려면 [`.gitlab-ci.yml`](https://docs.gitlab.com/ci/yaml/) 와 [`config.toml`](../commands/_index.md#configuration-file)에서 Docker 이미지 및 서비스를 정의합니다.

다음 키워드를 사용합니다:

- `image`: 러너가 작업을 실행하는 데 사용하는 Docker 이미지의 이름입니다.
  - 로컬 Docker Engine의 이미지 또는 Docker Hub의 모든 이미지를 입력합니다. 자세한 내용은 [Docker 설명서](https://docs.docker.com/get-started/introduction/)를 참조하세요.
  - 이미지 버전을 정의하려면 콜론(`:`)을 사용하여 태그를 추가합니다. 태그를 지정하지 않으면 Docker는 `latest`을 버전으로 사용합니다.
- `services`: 다른 컨테이너를 생성하고 `image`에 연결하는 추가 이미지입니다. 서비스 유형에 대한 자세한 내용은 [서비스](https://docs.gitlab.com/ci/services/)를 참조하세요.

### `.gitlab-ci.yml`에서 이미지 및 서비스 정의 {#define-images-and-services-in-gitlab-ciyml}

모든 작업에 러너가 사용하는 이미지와 빌드 시간 중에 사용할 서비스 목록을 정의합니다.

예:

```yaml
image: ruby:3.3

services:
  - postgres:9.3

before_script:
  - bundle install

test:
  script:
  - bundle exec rake spec
```

작업마다 다른 이미지 및 서비스를 정의하려면:

```yaml
before_script:
  - bundle install

test:3.3:
  image: ruby:3.3
  services:
  - postgres:9.3
  script:
  - bundle exec rake spec

test:3.4:
  image: ruby:3.4
  services:
  - postgres:9.4
  script:
  - bundle exec rake spec
```

`.gitlab-ci.yml`에서 `image`을 정의하지 않으면 러너는 `config.toml`에서 정의한 `image`을 사용합니다.

### `config.toml`에서 이미지 및 서비스 정의 {#define-images-and-services-in-configtoml}

러너가 실행하는 모든 작업에 이미지 및 서비스를 추가하려면 `config.toml`에서 `[runners.docker]`을 업데이트합니다.

기본적으로 Docker 실행기는 `.gitlab-ci.yml`에서 정의된 `image`을 사용합니다. `.gitlab-ci.yml`에서 정의하지 않으면 러너는 `config.toml`에서 정의한 이미지를 사용합니다.

예:

```toml
[runners.docker]
  image = "ruby:3.3"

[[runners.docker.services]]
  name = "mysql:latest"
  alias = "db"

[[runners.docker.services]]
  name = "redis:latest"
  alias = "cache"
```

이 예제는 [테이블 배열 구문](https://toml.io/en/v0.4.0#array-of-tables)을 사용합니다.

### 비공개 레지스트리에서 이미지 정의 {#define-an-image-from-a-private-registry}

전제 조건:

- 비공개 레지스트리의 이미지에 접근하려면 [GitLab 러너 인증](https://docs.gitlab.com/ci/docker/using_docker_images/#access-an-image-from-a-private-container-registry)을 해야 합니다.

비공개 레지스트리에서 이미지를 정의하려면 `.gitlab-ci.yml`에서 레지스트리 이름과 이미지를 제공합니다.

예:

```yaml
image: my.registry.tld:5000/namespace/image:tag
```

이 예제에서 GitLab 러너는 `my.registry.tld:5000` 레지스트리에서 `namespace/image:tag` 이미지를 검색합니다.

## 네트워크 구성 {#network-configurations}

서비스를 CI/CD 작업에 연결하기 위해 네트워크를 구성해야 합니다.

네트워크를 구성하려면 다음 중 하나를 수행할 수 있습니다:

- 권장됨 각 작업에 대해 네트워크를 생성하도록 러너를 구성합니다.
- 컨테이너 링크를 정의합니다. 컨테이너 링크는 Docker의 레거시 기능입니다.

### 각 작업에 대해 네트워크 생성 {#create-a-network-for-each-job}

각 작업에 대해 네트워크를 생성하도록 러너를 구성할 수 있습니다.

이 네트워킹 모드를 활성화하면 러너는 각 작업에 대해 사용자 정의 Docker 브리지 네트워크를 생성하고 사용합니다. Docker 환경 변수는 컨테이너 간에 공유되지 않습니다. 사용자 정의 브리지 네트워크에 대한 자세한 내용은 [Docker 설명서](https://docs.docker.com/engine/network/drivers/bridge/)를 참조하세요.

이 네트워킹 모드를 사용하려면 `config.toml`의 기능 플래그 또는 환경 변수에서 `FF_NETWORK_PER_BUILD`을 활성화합니다.

`network_mode`을 설정하지 마세요.

예:

```toml
[[runners]]
  (...)
  executor = "docker"
  environment = ["FF_NETWORK_PER_BUILD=1"]
```

또는:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.feature_flags]
    FF_NETWORK_PER_BUILD = true
```

기본 Docker 주소 풀을 설정하려면 [`dockerd`](https://docs.docker.com/reference/cli/dockerd/)에서 `default-address-pool`을 사용합니다. CIDR 범위가 이미 네트워크에서 사용 중인 경우 Docker 네트워크는 호스트의 다른 네트워크(다른 Docker 네트워크 포함)와 충돌할 수 있습니다.

이 기능은 Docker 데몬이 IPv6을 활성화하도록 구성된 경우에만 작동합니다. IPv6 지원을 활성화하려면 Docker 구성에서 `enable_ipv6`을 `true`로 설정합니다. 자세한 내용은 [Docker 설명서](https://docs.docker.com/engine/daemon/ipv6/)를 참조하세요.

러너는 `build` 별칭을 사용하여 작업 컨테이너를 확인합니다.

이 기능을 사용할 때 DNS가 Docker-in-Docker(`dind`) 서비스에서 올바르게 작동하지 않을 수 있습니다.

이 동작은 [Docker/Moby](https://github.com/moby/moby/issues/20037#issuecomment-181659049)의 문제 때문입니다. `dind` 컨테이너는 네트워크를 지정할 때 사용자 정의 DNS 항목을 상속하지 않습니다.

해결 방법으로, `dind` 서비스에 사용자 정의 DNS 설정을 수동으로 제공합니다. 예를 들어, 사용자 정의 DNS 서버가 `1.1.1.1`인 경우 Docker의 내부 DNS 서비스인 `127.0.0.11`을 사용할 수 있습니다:

```yaml
  services:
    - name: docker:dind
      command: [--dns=127.0.0.11, --dns=1.1.1.1]
```

이 접근 방식을 통해 컨테이너는 동일한 네트워크의 서비스를 확인할 수 있습니다.

#### 러너가 각 작업에 대해 네트워크를 생성하는 방법 {#how-the-runner-creates-a-network-for-each-job}

작업이 시작되면 러너는:

1. Docker 명령어 `docker network create <network>`과 유사하게 브리지 네트워크를 생성합니다.
1. 서비스 및 컨테이너를 브리지 네트워크에 연결합니다.
1. 작업 종료 시 네트워크를 제거합니다.

작업을 실행하는 컨테이너 및 서비스를 실행하는 컨테이너는 서로의 호스트명과 별칭을 확인합니다. 이 기능은 [Docker에서 제공합니다](https://docs.docker.com/engine/network/drivers/bridge/#differences-between-user-defined-bridges-and-the-default-bridge).

### 컨테이너 링크로 네트워크 구성 {#configure-a-network-with-container-links}

18.7.0 이전의 GitLab 러너는 기본 Docker `bridge`과 함께 [레거시 컨테이너 링크](https://docs.docker.com/engine/network/links/)를 사용하여 작업 컨테이너를 서비스와 연결합니다. Docker가 링크 기능을 더 이상 지원하지 않기 때문에, GitLab 러너 18.7.0 이상에서는 레거시 컨테이너 링크 동작을 Docker의 `extra_hosts` 기능을 사용하여 서비스 별칭을 확인하도록 에뮬레이션합니다. 이 네트워크 모드는 [`FF_NETWORK_PER_BUILD`](#create-a-network-for-each-job)이 비활성화된 경우 기본값입니다.

GitLab 러너 에뮬레이션된 링크 동작은 [레거시 컨테이너 링크](https://docs.docker.com/engine/network/links/)와 약간 다릅니다:

- `icc`을 비활성화하면 컨테이너 간 통신이 비활성화되고 컨테이너는 서로 통신할 수 없습니다.
- 연결된 컨테이너의 환경 변수는 더 이상 존재하지 않습니다(`<name>_PORT_<port>_<protocol>`).

네트워크를 구성하려면 `config.toml` 파일에서 [네트워킹 모드](https://docs.docker.com/engine/containers/run/#network-settings)를 지정합니다:

- `bridge`: 브리지 네트워크를 사용합니다. 기본값
- `host`: 컨테이너 내 호스트의 네트워크 스택을 사용합니다.
- `none`: 네트워킹 없음 권장되지 않음

예:

```toml
[[runners]]
  (...)
  executor = "docker"
[runners.docker]
  network_mode = "bridge"
```

다른 `network_mode` 값을 사용하는 경우 이는 빌드 컨테이너가 연결되는 기존 Docker 네트워크의 이름으로 간주됩니다.

이름 확인 중에 Docker는 컨테이너의 `/etc/hosts` 파일을 서비스 컨테이너 호스트명 및 별칭으로 업데이트합니다. 그러나 서비스 컨테이너는 **not**. 컨테이너 이름을 확인하려면 각 작업에 대해 네트워크를 생성해야 합니다.

연결된 컨테이너는 환경 변수를 공유합니다.

#### 생성된 네트워크의 MTU 재정의 {#overriding-the-mtu-of-the-created-network}

OpenStack의 가상 머신 같은 일부 환경에서는 사용자 정의 MTU가 필요합니다. Docker 데몬은 `docker.json`의 MTU를 존중하지 않습니다([Moby 문제 34981](https://github.com/moby/moby/issues/34981) 참조). `config.toml`의 `network_mtu`을 유효한 모든 값으로 설정하여 Docker 데몬이 새로 생성된 네트워크에 올바른 MTU를 사용하도록 할 수 있습니다. 또한 [`FF_NETWORK_PER_BUILD`](#create-a-network-for-each-job)을 활성화하여 재정의가 적용되도록 해야 합니다.

다음 구성은 각 작업에 대해 생성된 네트워크의 MTU를 `1402`로 설정합니다. 특정 환경 요구사항에 맞게 값을 조정해야 합니다.

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    network_mtu = 1402
    [runners.feature_flags]
      FF_NETWORK_PER_BUILD = true
```

#### Docker-in-Docker MTU는 러너에서 상속되지 않음 {#docker-in-docker-mtu-does-not-inherit-from-the-runner}

`docker:dind`을 서비스로 사용할 때 내부 `dockerd`은 러너의 Docker 브리지에 구성된 MTU와 관계없이 MTU `1500`으로 기본 설정됩니다. 러너의 브리지 MTU가 `1500`보다 낮으면 dind 내 빌드 컨테이너에서 전송된 큰 패킷이 자동으로 삭제됩니다. ICMP `fragmentation needed` 회신이 클라우드 및 가상 환경에서 자주 필터링되기 때문에 발신자는 패킷 크기를 낮추는 방법을 알지 못하며 연결이 무음으로 중단됩니다.

증상:  `dotnet restore` 또는 `curl "https://api.nuget.org/v3/index.json"` 같은 명령어는 Docker-in-Docker 작업에서 시간 초과되지만 이러한 명령어는 dind 외부에서 작동합니다.

이 문제를 해결하려면 `docker:dind` 서비스에서 `--mtu`을 명시적으로 설정하여 러너의 Docker 브리지 MTU 이하의 값을 사용합니다:

```yaml
services:
  - name: docker:dind
    command: ["--mtu=1360"]
```

러너의 브리지 MTU를 모르는 경우 `1360`은 대부분의 환경에 안전한 값입니다. `--mtu` 플래그를 생략하거나 러너의 브리지 MTU보다 큰 값으로 설정하면 연결이 중단됩니다.

## Docker 이미지 및 서비스 제한 {#restrict-docker-images-and-services}

Docker 이미지 및 서비스를 제한하려면 `allowed_images` 및 `allowed_services` 매개변수에서 와일드카드 패턴을 지정합니다. 구문에 대한 자세한 내용은 [doublestar 설명서](https://github.com/bmatcuk/doublestar)를 참조하세요.

예를 들어, 비공개 Docker 레지스트리의 이미지만 허용하려면:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_images = ["my.registry.tld:5000/*:*"]
    allowed_services = ["my.registry.tld:5000/*:*"]
```

비공개 Docker 레지스트리에서 이미지 목록으로 제한하려면:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_images = ["my.registry.tld:5000/ruby:*", "my.registry.tld:5000/node:*"]
    allowed_services = ["postgres:9.4", "postgres:latest"]
```

Kali 같은 특정 이미지를 제외하려면:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_images = ["**", "!*/kali*"]
```

## 서비스 호스트명 접근 {#access-services-hostnames}

서비스 호스트명에 접근하려면 `.gitlab-ci.yml`에서 `services`에 서비스를 추가합니다.

```yaml
services:
- valkey/valkey:latest
```

작업이 실행되면 `valkey/valkey` 서비스가 시작됩니다. 빌드 컨테이너에서 호스트명 `valkey__valkey` 및 `valkey-valkey`으로 접근할 수 있습니다.

지정된 서비스 별칭 외에도 러너는 서비스 이미지의 이름을 서비스 컨테이너에 대한 별칭으로 할당합니다. 이러한 별칭 중 하나를 사용할 수 있습니다.

러너는 이미지 이름을 기반으로 별칭을 생성하기 위해 다음 규칙을 사용합니다:

- `:` 이후의 모든 내용이 제거됩니다.
- 첫 번째 별칭의 경우 슬래시(`/`)가 이중 밑줄(`__`)로 바뀝니다.
- 두 번째 별칭의 경우 슬래시(`/`)가 단일 대시(`-`)로 바뀝니다.

비공개 서비스 이미지를 사용하는 경우 러너는 지정된 모든 포트를 제거하고 규칙을 적용합니다. 서비스 `registry.example.com:4999/valkey/valkey`은 호스트명 `registry.example.com__valkey__valkey` 및 `registry.example.com-valkey-valkey`을 생성합니다.

## 서비스 구성 {#configuring-services}

데이터베이스 이름을 변경하거나 계정 이름을 설정하려면 서비스에 대한 환경 변수를 정의할 수 있습니다.

러너가 변수를 전달할 때:

- 변수는 모든 컨테이너에 전달됩니다. 러너는 특정 컨테이너에 변수를 전달할 수 없습니다.
- 보안 변수는 빌드 컨테이너에 전달됩니다.

구성 변수에 대한 자세한 내용은 해당 Docker Hub 페이지에서 제공하는 각 이미지의 설명서를 참조하세요.

### RAM에 디렉토리 마운트 {#mount-a-directory-in-ram}

`tmpfs` 옵션을 사용하여 디렉토리를 RAM에 마운트할 수 있습니다. 데이터베이스처럼 많은 I/O 관련 작업이 있는 경우 테스트하는 데 필요한 시간을 단축합니다.

러너 구성에서 `tmpfs` 및 `services_tmpfs` 옵션을 사용하는 경우 각각 고유한 옵션을 사용하는 여러 경로를 지정할 수 있습니다. 자세한 내용은 [Docker 설명서](https://docs.docker.com/reference/cli/docker/container/run/#tmpfs)를 참조하세요.

예를 들어, 공식 MySQL 컨테이너의 데이터 디렉토리를 RAM에 마운트하려면 `config.toml`을 구성합니다:

```toml
[runners.docker]
  # For the main container
  [runners.docker.tmpfs]
      "/var/lib/mysql" = "rw,noexec"

  # For services
  [runners.docker.services_tmpfs]
      "/var/lib/mysql" = "rw,noexec"
```

### 서비스에서 디렉토리 빌드 {#building-a-directory-in-a-service}

GitLab 러너는 `/builds` 디렉토리를 모든 공유 서비스에 마운트합니다.

다양한 서비스 사용에 대한 자세한 내용은 다음을 참조하세요:

- [PostgreSQL 사용](https://docs.gitlab.com/ci/services/postgres/)
- [MySQL 사용](https://docs.gitlab.com/ci/services/mysql/)

### GitLab 러너가 서비스 상태 검사를 수행하는 방법 {#how-gitlab-runner-performs-the-services-health-check}

{{< history >}}

- [GitLab 16.0에서 여러 포트 검사를 도입했습니다](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4079).

{{< /history >}}

서비스가 시작된 후 GitLab 러너는 서비스가 응답할 때까지 기다립니다. Docker 실행기는 서비스 컨테이너에서 노출된 서비스 포트로 TCP 연결을 열려고 시도합니다.

처음 20개의 노출된 포트만 검사됩니다.

`HEALTHCHECK_TCP_PORT` 서비스 변수를 사용하여 특정 포트에서 상태 검사를 수행할 수 있습니다:

```yaml
job:
  services:
    - name: mongo
      variables:
        HEALTHCHECK_TCP_PORT: "27017"
```

이 구현 방법을 보려면 상태 검사 [Go 명령어](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/commands/helpers/health_check.go)를 사용합니다.

## Docker 드라이버 작업 지정 {#specify-docker-driver-operations}

빌드에 대한 볼륨을 생성할 때 Docker 볼륨 드라이버에 제공할 인수를 지정합니다. 예를 들어 이러한 인수를 사용하여 각 빌드가 실행할 수 있는 공간을 제한하고 다른 모든 드라이버 특정 옵션을 추가할 수 있습니다. 다음 예제는 각 빌드가 소비할 수 있는 제한이 50GB로 설정된 `config.toml`을 보여줍니다.

```toml
[runners.docker]
  [runners.docker.volume_driver_ops]
      "size" = "50G"
```

## 호스트 디바이스 사용 {#using-host-devices}

{{< history >}}

- [GitLab 17.10에서 도입되었습니다](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/6208).

{{< /history >}}

GitLab 러너 호스트의 하드웨어 디바이스를 작업을 실행하는 컨테이너에 노출할 수 있습니다. 이를 위해 러너의 `devices` 및 `services_devices` 옵션을 구성합니다.

- `build` 및 [도우미](../configuration/advanced-configuration.md#helper-image) 컨테이너에 디바이스를 노출하려면 `devices` 옵션을 사용합니다.
- 서비스 컨테이너에 디바이스를 노출하려면 `services_devices` 옵션을 사용합니다. 서비스 컨테이너의 디바이스 접근을 특정 이미지로 제한하려면 정확한 이미지 이름 또는 glob 패턴을 사용합니다. 이 조치는 호스트 시스템 디바이스에 대한 직접 접근을 방지합니다.

디바이스 접근에 대한 자세한 내용은 [Docker 설명서](https://docs.docker.com/reference/cli/docker/container/run/#device)를 참조하세요.

### 컨테이너 예제 빌드 {#build-container-example}

이 예제에서 `config.toml` 섹션은 `/dev/bus/usb`를 빌드 컨테이너에 노출합니다. 이 구성을 통해 작업 파이프라인은 [Android Debug Bridge(`adb`)](https://developer.android.com/tools/adb)를 통해 제어되는 Android 스마트폰과 같이 호스트 머신에 연결된 USB 디바이스에 접근할 수 있습니다.

빌드 작업 컨테이너가 호스트 USB 디바이스에 직접 접근할 수 있으므로 동일한 하드웨어에 접근할 때 동시 작업 파이프라인 실행이 서로 충돌할 수 있습니다. 이러한 충돌을 방지하려면 [`resource_group`](https://docs.gitlab.com/ci/yaml/#resource_group)를 사용합니다.

```toml
[[runners]]
  name = "hardware-runner"
  url = "https://gitlab.com"
  token = "__REDACTED__"
  executor = "docker"
  [runners.docker]
    # All job containers may access the host device
    devices = ["/dev/bus/usb"]
```

### 비공개 레지스트리 예제 {#private-registry-example}

이 예제는 `/dev/kvm` 및 `/dev/dri` 디바이스를 비공개 Docker 레지스트리의 컨테이너 이미지에 노출하는 방법을 보여줍니다. 이러한 디바이스는 일반적으로 하드웨어 가속 가상화 및 렌더링에 사용됩니다. 하드웨어 리소스에 대한 사용자의 직접 접근 제공과 관련된 위험을 완화하려면 디바이스 접근을 `myregistry:5000/emulator/*` 네임스페이스의 신뢰할 수 있는 이미지로 제한합니다:

```toml
[runners.docker]
  [runners.docker.services_devices]
    # Only images from an internal registry may access the host devices
    "myregistry:5000/emulator/*" = ["/dev/kvm", "/dev/dri"]
```

> [!warning]
> 이미지 이름 `**/*`는 모든 이미지에 디바이스를 노출할 수 있습니다.

## 컨테이너 빌드 및 캐시의 디렉토리 구성 {#configure-directories-for-the-container-build-and-cache}

데이터가 컨테이너에 저장되는 위치를 정의하려면 `[[runners]]` 섹션의 `config.toml`에서 `/builds` 및 `/cache` 디렉토리를 구성합니다.

`/cache` 저장소 경로를 수정하는 경우 경로를 영구 경로로 표시하려면 `config.toml`의 `[runners.docker]` 섹션에서 `volumes = ["/my/cache/"]`을 정의해야 합니다.

기본적으로 Docker 실행기는 다음 디렉토리에 빌드 및 캐시를 저장합니다:

- `/builds/<namespace>/<project-name>`에서 빌드
- 컨테이너 내부 `/cache`에서 캐시합니다.

## Docker 캐시 삭제 {#clear-the-docker-cache}

[`clear-docker-cache`](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/packaging/root/usr/share/gitlab-runner/clear-docker-cache)를 사용하여 러너가 생성한 사용하지 않는 컨테이너 및 볼륨을 제거합니다.

옵션 목록을 보려면 `help` 옵션으로 스크립트를 실행합니다:

```shell
clear-docker-cache help
```

기본 옵션은 `prune-volumes`이며, 사용하지 않는 모든 컨테이너(분리 및 참조되지 않음) 및 볼륨을 제거합니다.

캐시 저장소를 효율적으로 관리하려면 다음을 수행해야 합니다:

- `clear-docker-cache`을 `cron`로 정기적으로(예: 주 1회) 실행합니다.
- 성능을 위해 캐시에 일부 최근 컨테이너를 유지하면서 디스크 공간을 확보합니다.

`FILTER_FLAG` 환경 변수는 정리할 개체를 제어합니다. 예를 들어 [Docker 이미지 정리](https://docs.docker.com/reference/cli/docker/image/prune/#filter) 설명서를 참조하세요.

## Docker 빌드 이미지 삭제 {#clear-docker-build-images}

[`clear-docker-cache`](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/packaging/root/usr/share/gitlab-runner/clear-docker-cache) 스크립트는 GitLab 러너에서 태그를 지정하지 않았으므로 Docker 이미지를 제거하지 않습니다.

Docker 빌드 이미지를 삭제하려면:

1. 복구할 수 있는 디스크 공간을 확인합니다:

   ```shell
   clear-docker-cache space

   Show docker disk usage
   ----------------------

   TYPE            TOTAL     ACTIVE    SIZE      RECLAIMABLE
   Images          14        9         1.306GB   545.8MB (41%)
   Containers      19        18        115kB     0B (0%)
   Local Volumes   0         0         0B        0B
   Build Cache     0         0         0B        0B
   ```

1. 사용하지 않는 모든 컨테이너, 네트워크, 이미지(분리 및 참조되지 않음) 및 태그 지정되지 않은 볼륨을 제거하려면 [`docker system prune`](https://docs.docker.com/reference/cli/docker/system/prune/)를 실행합니다.

## 영구 저장소 {#persistent-storage}

Docker 실행기는 컨테이너를 실행할 때 영구 저장소를 제공합니다. `volumes =`에서 정의된 모든 디렉토리는 빌드 간에 영구적입니다.

`volumes` 지시문은 다음 저장소 유형을 지원합니다:

- 동적 저장소의 경우 `<path>`을 사용합니다. `<path>`는 해당 작업에 대한 동일한 동시 실행의 후속 실행 간에 영구적입니다. `runners.docker.cache_dir`을 설정하지 않으면 데이터는 Docker 볼륨에 유지됩니다. 그렇지 않으면 호스트의 구성된 디렉토리에 유지됩니다(빌드 컨테이너에 마운트됨).

  볼륨 기반 영구 저장소의 볼륨 이름:

  - GitLab 러너 18.4.0 이전: `runner-<short-token>-project-<project-id>-concurrent-<concurrency-id>-cache-<md5-of-path>`
  - GitLab 러너 18.4.0 이상: `runner-<runner-id-hash>-cache-<md5-of-path><protection>`

    볼륨 이름에서 더 이상 인간이 읽을 수 없는 데이터는 볼륨의 레이블로 이동됩니다.

  호스트 기반 영구 저장소의 호스트 디렉토리:

  - GitLab 러너 18.4.0 이전: `<cache-dir>/runner-<short-token>-project-<project-id>-concurrent-<concurrency-id>/<md5-of-path>`
  - GitLab 러너 18.4.0 이상: `<cache-dir>/runner-<runner-id-hash>/<md5-of-path><protection>`

  변수 부분 설명:

  - `<short-token>`: 러너의 토큰의 축약된 버전(처음 8글자)
  - `<project-id>`: GitLab 작업의 ID
  - `<concurrency-id>`: 동일한 작업에 대한 빌드를 동시에 실행하는 모든 러너 목록의 러너 인덱스(`CI_CONCURRENT_PROJECT_ID` [사전 정의된 변수](https://docs.gitlab.com/ci/variables/predefined_variables/)를 통해 접근 가능)입니다.
  - `<md5-of-path>`: 컨테이너 내 경로의 MD5 합계
  - `<runner-id-hash>`: 다음 데이터의 해시입니다:
    - 러너의 토큰
    - 러너의 시스템 ID
    - `<project-id>`
    - `<concurrency-id>`
  - `<protection>`: 보호되지 않은 작업 빌드의 경우 값이 비어 있고 보호된 작업 빌드의 경우 `-protected`
  - `<cache-dir>`: `runners.docker.cache_dir`의 구성
- 호스트 바운드 저장소의 경우 `<host-path>:<path>[:<mode>]`을 사용합니다. GitLab 러너는 `<path>`를 호스트 시스템의 `<host-path>`에 바인드합니다. 선택적인 `<mode>`는 이 저장소가 읽기 전용인지 읽기-쓰기(기본값)인지 지정합니다.

> [!warning]
> GitLab 러너 18.4 이상에서 동적 저장소 소스의 명명(위 참조)이 Docker 볼륨 기반 및 호스트 디렉토리 기반 영구 저장소 모두에 대해 변경되었습니다. 18.4.0으로 업그레이드하면 GitLab 러너는 이전 러너 버전의 캐시된 데이터를 무시하고 새 Docker 볼륨 또는 새 호스트 디렉토리를 통해 필요에 따라 새 동적 저장소를 생성합니다.
>
> 호스트 바운드 저장소(`<host-path>` 구성)는 동적 저장소와 달리 영향을 받지 않습니다.

### 빌드의 영구 저장소 {#persistent-storage-for-builds}

`/builds` 디렉토리를 호스트 바운드 저장소로 설정하면 빌드가 `/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`에 저장됩니다. 여기서:

- `<short-token>`은 러너의 토큰의 축약된 버전(처음 8글자)입니다.
- `<concurrent-id>`은 동일한 작업에 대한 빌드를 동시에 실행하는 모든 러너 목록의 러너 인덱스(`CI_CONCURRENT_PROJECT_ID` [사전 정의된 변수](https://docs.gitlab.com/ci/variables/predefined_variables/)를 통해 접근 가능)입니다.

## IPC 모드 {#ipc-mode}

Docker 실행기는 다른 위치와 컨테이너의 IPC 네임스페이스를 공유하는 것을 지원합니다. 이는 `docker run --ipc` 플래그에 매핑됩니다. [Docker 설명서의 IPC 설정](https://docs.docker.com/engine/containers/run/#ipc-settings---ipc)에 대한 자세한 내용

## 권한 있는 모드 {#privileged-mode}

Docker 실행기는 빌드 컨테이너의 미세 조정을 허용하는 여러 옵션을 지원합니다. 이러한 옵션 중 하나는 [`privileged` 모드](https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities)입니다.

### 권한 있는 모드로 Docker-in-Docker 사용 {#use-docker-in-docker-with-privileged-mode}

구성된 `privileged` 플래그는 빌드 컨테이너 및 모든 서비스에 전달됩니다. 이 플래그를 사용하면 Docker-in-Docker 접근 방식을 사용할 수 있습니다.

먼저 러너(`config.toml`)를 `privileged` 모드에서 실행하도록 구성합니다:

```toml
[[runners]]
  executor = "docker"
  [runners.docker]
    privileged = true
```

그런 다음 빌드 스크립트(`.gitlab-ci.yml`)를 Docker-in-Docker 컨테이너를 사용하도록 설정합니다:

```yaml
image: docker:git
services:
- docker:dind

build:
  script:
  - docker build -t my-image .
  - docker push my-image
```

> [!warning]
> 권한 있는 모드에서 실행되는 컨테이너에는 보안 위험이 있습니다. 컨테이너가 권한 있는 모드에서 실행되면 컨테이너 보안 메커니즘을 비활성화하고 호스트를 권한 에스컬레이션에 노출합니다. 권한 있는 모드에서 컨테이너를 실행하면 컨테이너 탈출로 이어질 수 있습니다. 자세한 내용은 [런타임 권한 및 Linux 기능](https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities)에 대한 Docker 설명서를 참조하세요.

다음과 같은 오류를 방지하려면 [Docker-in-Docker를 TLS로 구성하거나 TLS를 비활성화](https://docs.gitlab.com/ci/docker/using_docker_build/#use-the-docker-executor-with-docker-in-docker)해야 할 수 있습니다:

```plaintext
Cannot connect to the Docker daemon at tcp://docker:2375. Is the docker daemon running?
```

### 제한된 권한 있는 모드로 루트리스 Docker-in-Docker 사용 {#use-rootless-docker-in-docker-with-restricted-privileged-mode}

이 버전에서는 Docker-in-Docker 루트리스 이미지만 권한 있는 모드에서 서비스로 실행할 수 있습니다.

`services_privileged` 및 `allowed_privileged_services` 구성 매개변수는 권한 있는 모드에서 실행할 수 있는 컨테이너를 제한합니다.

제한된 권한 있는 모드로 루트리스 Docker-in-Docker를 사용하려면:

1. `config.toml`에서 `services_privileged` 및 `allowed_privileged_services`을 사용하도록 러너를 구성합니다:

   ```toml
   [[runners]]
     executor = "docker"
     [runners.docker]
       services_privileged = true
       allowed_privileged_services = ["docker.io/library/docker:*-dind-rootless", "docker.io/library/docker:dind-rootless", "docker:*-dind-rootless", "docker:dind-rootless"]
   ```

1. `.gitlab-ci.yml`에서 빌드 스크립트를 편집하여 Docker-in-Docker 루트리스 컨테이너를 사용하도록 하세요:

   ```yaml
   image: docker:git
   services:
   - docker:dind-rootless

   build:
     script:
     - docker build -t my-image .
     - docker push my-image
   ```

`allowed_privileged_services`에 나열한 Docker-in-Docker 루트리스 이미지만 권한 있는 모드에서 실행할 수 있습니다. 작업 및 서비스의 다른 모든 컨테이너는 권한 없는 모드에서 실행됩니다.

루트로 실행되지 않으므로 Docker-in-Docker 루트리스 또는 BuildKit 루트리스와 같은 권한 있는 모드 이미지와 함께 사용하기에 _거의 안전합니다_.

보안 문제에 대한 자세한 내용은 [Docker 실행기의 보안 위험](../security/_index.md#usage-of-docker-executor)을 참조하세요.

## Docker ENTRYPOINT 구성 {#configure-a-docker-entrypoint}

기본적으로 Docker 실행기는 Docker 이미지의 [`ENTRYPOINT`](https://docs.docker.com/engine/containers/run/#entrypoint-default-command-to-execute-at-runtime)를 재정의하지 않습니다. `sh` 또는 `bash`를 [`COMMAND`](https://docs.docker.com/engine/containers/run/#cmd-default-command-or-options)로 전달하여 작업 스크립트를 실행하는 컨테이너를 시작합니다.

작업을 실행할 수 있도록 Docker 이미지는 다음을 수행해야 합니다:

- `sh` 또는 `bash` 및 `grep`을 제공합니다.
- 인수로 `sh`/`bash`을 전달할 때 셸을 시작하는 `ENTRYPOINT`을 정의합니다.

Docker 실행기는 다음 명령어와 동등한 것으로 작업의 컨테이너를 실행합니다:

```shell
docker run <image> sh -c "echo 'It works!'" # or bash
```

Docker 이미지가 이 메커니즘을 지원하지 않으면 프로젝트 구성에서 [이미지의 ENTRYPOINT 재정의](https://docs.gitlab.com/ci/yaml/#imageentrypoint)할 수 있습니다:

```yaml
# Equivalent of
# docker run --entrypoint "" <image> sh -c "echo 'It works!'"
image:
  name: my-image
  entrypoint: [""]
```

자세한 내용은 [이미지의 진입점 재정의](https://docs.gitlab.com/ci/docker/using_docker_images/#override-the-entrypoint-of-an-image) 및 [`CMD` 및 `ENTRYPOINT`가 Docker에서 상호작용하는 방법](https://docs.docker.com/reference/dockerfile/#understand-how-cmd-and-entrypoint-interact)을 참조하세요.

### ENTRYPOINT로 스크립트 작업 {#job-script-as-entrypoint}

`ENTRYPOINT`을 사용하여 빌드 스크립트를 사용자 정의 환경에서 또는 보안 모드에서 실행하는 Docker 이미지를 만들 수 있습니다.

예를 들어 빌드 스크립트를 실행하지 않는 `ENTRYPOINT`을 사용하는 Docker 이미지를 만들 수 있습니다. 대신 Docker 이미지는 미리 정의된 명령어 세트를 실행하여 디렉토리에서 Docker 이미지를 빌드합니다. 빌드 컨테이너를 [권한 있는 모드](#privileged-mode)에서 실행하고 러너의 빌드 환경을 보호합니다.

1. 새 Dockerfile을 만듭니다:

   ```dockerfile
   FROM docker:dind
   ADD / /entrypoint.sh
   ENTRYPOINT ["/bin/sh", "/entrypoint.sh"]
   ```

1. `entrypoint.sh` bash 스크립트를 만듭니다. `ENTRYPOINT`로 사용됩니다:

   ```shell
   #!/bin/sh

   dind docker daemon
       --host=unix:///var/run/docker.sock \
       --host=tcp://0.0.0.0:2375 \
       --storage-driver=vf &

   docker build -t "$BUILD_IMAGE" .
   docker push "$BUILD_IMAGE"
   ```

1. Docker 레지스트리에 이미지를 푸시합니다.
1. Docker 실행기를 `privileged` 모드에서 실행합니다. `config.toml`에서 정의합니다:

   ```toml
   [[runners]]
     executor = "docker"
     [runners.docker]
       privileged = true
   ```

1. 프로젝트에서 다음 `.gitlab-ci.yml`을 사용합니다:

   ```yaml
   variables:
     BUILD_IMAGE: my.image
   build:
     image: my/docker-build:image
     script:
     - Dummy Script
   ```

## Docker 명령어 실행용 Podman 사용 {#use-podman-to-run-docker-commands}

Linux에 GitLab 러너가 설치되어 있으면 작업에서 Podman을 사용하여 Docker 실행기에서 컨테이너 런타임으로 Docker를 바꿀 수 있습니다.

전제 조건:

- [Podman](https://podman.io/) v4.2.0 이상
- 작업 실행기로 Podman을 사용하여 [서비스](#services) 를 실행하려면 [`FF_NETWORK_PER_BUILD` 기능 플래그](#create-a-network-for-each-job)를 활성화합니다. [Docker 컨테이너 링크](https://docs.docker.com/engine/network/links/) 는 레거시이며 [Podman](https://podman.io/)에서 지원되지 않습니다. 네트워크 별칭을 생성하는 서비스의 경우 `podman-plugins` 패키지를 설치해야 합니다.

> [!note]
> Podman은 `aardvark-dns`를 컨테이너의 DNS 서버로 사용합니다. `aardvark-dns` 버전 1.10.0 이하는 CI/CD 작업에서 산발적인 DNS 확인 실패를 유발합니다. 최신 버전이 설치되어 있는지 확인하세요. 자세한 내용은 [GitHub 문제 389](https://github.com/containers/aardvark-dns/issues/389)를 참조하세요.

1. Linux 호스트에서 GitLab 러너를 설치합니다. 시스템의 패키지 관리자를 사용하여 GitLab 러너를 설치한 경우 `gitlab-runner` 사용자가 자동으로 생성됩니다.
1. GitLab 러너를 실행하는 사용자로 로그인하세요. [`pam_systemd`](https://www.freedesktop.org/software/systemd/man/latest/pam_systemd.html) 주위를 우회하지 않는 방식으로 이를 수행해야 합니다. 올바른 사용자와 함께 SSH를 사용할 수 있습니다. 이렇게 하면 이 사용자로 `systemctl`을 실행할 수 있습니다.
1. 시스템이 [루트리스 Podman 설정](https://github.com/containers/podman/blob/main/docs/tutorials/rootless_tutorial.md)의 필수 조건을 충족하는지 확인하세요. 특히 사용자에게 [`/etc/subuid` 및 `/etc/subgid`의 올바른 항목](https://github.com/containers/podman/blob/main/docs/tutorials/rootless_tutorial.md#etcsubuid-and-etcsubgid-configuration)이 있는지 확인하세요.
1. Linux 호스트에서 [Podman을 설치](https://podman.io/getting-started/installation)합니다.
1. Podman 소켓을 활성화하고 시작합니다:

   ```shell
   systemctl --user --now enable podman.socket
   ```

1. Podman 소켓이 수신 중인지 확인합니다:

   ```shell
   systemctl status --user podman.socket
   ```

1. Podman API에 접근하는 `Listen` 키의 소켓 문자열을 복사합니다.
1. GitLab 러너 사용자가 로그아웃한 후 Podman 소켓이 사용 가능한 상태로 유지되도록 하세요:

   ```shell
   sudo loginctl enable-linger gitlab-runner
   ```

1. GitLab 러너 `config.toml` 파일을 편집하고 소켓 값을 `[runners.docker]` 섹션의 호스트 항목에 추가합니다. 예를 들어:

   ```toml
   [[runners]]
     name = "podman-test-runner-2025-06-07"
     url = "https://gitlab.com"
     token = "TOKEN"
     executor = "docker"
     [runners.docker]
       host = "unix:///run/user/1012/podman/podman.sock"
       tls_verify = false
       image = "quay.io/podman/stable"
       privileged = false
   ```

   > [!note]
   > `privileged = false`을 표준 Podman 사용으로 설정합니다. 작업 내에서 [Docker-in-Docker 서비스](#use-docker-in-docker-with-privileged-mode)를 실행해야 하는 경우에만 `privileged = true`을 설정합니다.

### Dockerfile에서 컨테이너 이미지 빌드용 Podman 사용 {#use-podman-to-build-container-images-from-a-dockerfile}

다음 예제는 Podman을 사용하여 컨테이너 이미지를 빌드하고 이미지를 GitLab 컨테이너 레지스트리에 푸시합니다.

러너 `config.toml`의 기본 컨테이너 이미지는 `quay.io/podman/stable`로 설정되므로 CI 작업이 해당 이미지를 사용하여 포함된 명령을 실행합니다.

```yaml
variables:
  IMAGE_TAG: $CI_REGISTRY_IMAGE:$CI_COMMIT_REF_SLUG

before_script:
  - podman login -u "$CI_REGISTRY_USER" -p "$CI_REGISTRY_PASSWORD" $CI_REGISTRY

oci-container-build:
  stage: build
  script:
    - podman build -t $IMAGE_TAG .
    - podman push $IMAGE_TAG
  when: manual
```

### Dockerfile에서 컨테이너 이미지 빌드용 Buildah 사용 {#use-buildah-to-build-container-images-from-a-dockerfile}

다음 예제는 Buildah를 사용하여 컨테이너 이미지를 빌드하고 이미지를 GitLab 컨테이너 레지스트리에 푸시하는 방법을 보여줍니다.

```yaml
image: quay.io/buildah/stable

variables:
  IMAGE_TAG: $CI_REGISTRY_IMAGE:$CI_COMMIT_REF_SLUG

before_script:
  - buildah login -u "$CI_REGISTRY_USER" -p "$CI_REGISTRY_PASSWORD" $CI_REGISTRY

oci-container-build:
  stage: build
  script:
    - buildah bud -t $IMAGE_TAG .
    - buildah push $IMAGE_TAG
  when: manual
```

### 알려진 문제 {#known-issues}

Docker와 달리 Podman은 기본적으로 SELinux 정책을 적용합니다. 많은 작업 파이프라인이 문제 없이 실행되지만 도구가 임시 디렉토리를 사용할 때 SELinux 컨텍스트 상속으로 인해 일부가 실패할 수 있습니다.

예를 들어 다음 작업 파이프라인은 Podman에서 실패합니다:

```yaml
testing:
  image: alpine:3.20
  script:
    - apk add --no-cache python3 py3-pip
    - pip3 install --target $CI_PROJECT_DIR requests==2.28.2
```

pip가 `/tmp`을 작업 디렉토리로 사용하기 때문에 실패가 발생합니다. `/tmp`에서 생성된 파일은 SELinux 컨텍스트를 상속하므로 `$CI_PROJECT_DIR`으로 이동할 때 컨테이너가 이러한 파일을 수정하지 못하도록 합니다.

**Solution:** 러너의 `config.toml` 섹션의 `runners.docker`의 볼륨에 `/tmp`을 추가합니다:

```toml
[[runners]]
  [runners.docker]
    volumes = ["/cache", "/tmp"]
```

이 추가는 마운트된 디렉토리 전체에서 일관된 SELinux 컨텍스트를 보장합니다.

#### SELinux 문제 해결 {#troubleshooting-selinux-issues}

기타 Podman/SELinux 문제는 필요한 구성 변경사항을 식별하기 위해 추가 문제 해결이 필요할 수 있습니다.

Podman 러너 문제가 SELinux 관련인지 테스트하려면 일시적으로 다음 지시문을 러너의 `config.toml` 아래의 `runners.docker` 섹션에 추가합니다:

```toml
[[runners]]
  [runners.docker]
    security_opt = ["label:disable"]
```

> [!warning]
> 이 추가는 컨테이너의 SELinux 적용을 끕니다(Docker의 기본 동작). 보안 의미가 있을 수 있으므로 이 구성을 테스트 목적으로만 사용하고 영구 솔루션으로는 사용하지 마세요.

#### Configure SELinux MCS {#configure-selinux-mcs}

SELinux가 일부 쓰기 작업(예: 기존 Git 리포지토리 재초기화)을 차단하면 러너가 시작한 모든 컨테이너에 다중 범주 보안(MCS)을 강제할 수 있습니다:

```toml
[[runners]]
  [runners.docker]
    security_opt = ["label=level:s0:c1000"]
```

이 옵션은 SELinux를 비활성화하지 않지만 컨테이너의 MCS 수준을 설정합니다. 이 접근 방식은 `label:disable`을 사용하는 것보다 더 안전합니다.

> [!warning]
> 동일한 MCS 범주를 사용하는 여러 컨테이너는 해당 범주로 태그가 지정된 동일한 파일에 접근할 수 있습니다.

## 작업을 실행할 사용자 지정 {#specify-which-user-runs-the-job}

기본적으로 러너는 컨테이너의 `root` 사용자로 작업을 실행합니다. 작업을 실행할 다른 루트 이외 사용자를 지정하려면 Docker 이미지의 Dockerfile에서 `USER` 지시문을 사용합니다.

```dockerfile
FROM amazonlinux
RUN ["yum", "install", "-y", "nginx"]
RUN ["useradd", "www"]
USER "www"
CMD ["/bin/bash"]
```

해당 Docker 이미지를 사용하여 작업을 실행하면 지정된 사용자로 실행됩니다:

```yaml
build:
  image: my/docker-build:image
  script:
  - whoami   # www
```

## 러너가 이미지를 풀하는 방식 구성 {#configure-how-runners-pull-images}

러너가 레지스트리에서 Docker 이미지를 풀하는 방식을 정의하기 위해 `config.toml`의 풀 정책을 구성합니다. 단일 정책, [정책 목록](#set-multiple-pull-policies) , 또는 [특정 풀 정책 허용](#allow-docker-pull-policies)을 설정할 수 있습니다.

`pull_policy`에 대해 다음 값을 사용합니다:

- [`always`](#set-the-always-pull-policy): 기본값 로컬 이미지가 있어도 이미지를 풀합니다. 이 풀 정책은 `SHA256`으로 지정된 이미지에 적용되지 않습니다.
- [`if-not-present`](#set-the-if-not-present-pull-policy): 로컬 버전이 없을 때만 이미지를 풀합니다.
- [`never`](#set-the-never-pull-policy): 이미지를 풀하지 않고 로컬 이미지만 사용합니다.

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "always" # available: always, if-not-present, never
```

### `always` 풀 정책 설정 {#set-the-always-pull-policy}

`always` 옵션은 기본적으로 활성화되어 있으며 항상 컨테이너를 만들기 전에 풀을 시작합니다. 이 옵션은 이미지가 최신 상태인지 확인하고 로컬 이미지가 있어도 오래된 이미지 사용을 방지합니다.

이 풀 정책을 사용하는 경우:

- 러너는 항상 최신 이미지를 풀해야 합니다.
- 러너는 공개되어 있고 [자동 확장](../configuration/autoscale.md)하거나 GitLab 인스턴스의 작업 러너로 구성되어 있습니다.

**Do not use**. 러너가 로컬에 저장된 이미지를 사용해야 하는 경우입니다.

`config.toml`의 `pull policy`로 `always`을 설정합니다:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "always"
```

### `if-not-present` 풀 정책 설정 {#set-the-if-not-present-pull-policy}

풀 정책을 `if-not-present`로 설정하면 러너는 먼저 로컬 이미지가 있는지 확인합니다. 로컬 이미지가 없으면 러너는 레지스트리에서 이미지를 풀합니다.

`if-not-present` 정책을 사용하여:

- 로컬 이미지를 사용하되 로컬 이미지가 없으면 이미지를 풀합니다.
- 무겁고 거의 업데이트되지 않은 이미지의 경우 러너가 이미지 계층 차이를 분석하는 데 걸리는 시간을 줄입니다. 이 경우 이미지를 강제로 업데이트하기 위해 로컬 Docker Engine 저장소에서 이미지를 정기적으로 수동으로 제거해야 합니다.

**Do not use**:

- 러너를 사용하는 다른 사용자가 비공개 이미지에 접근할 수 있는 작업 러너. 보안 문제에 대한 자세한 내용은 [if-not-present 풀 정책이 포함된 비공개 Docker 이미지 사용](../security/_index.md#usage-of-private-docker-images-with-if-not-present-pull-policy)을 참조하세요.
- 작업을 자주 업데이트하고 최신 이미지 버전에서 실행해야 하는 경우입니다. 이로 인해 로컬 이미지를 자주 삭제하는 값을 초과할 수 있는 네트워크 로드 감소가 발생할 수 있습니다.

`config.toml`에서 `if-not-present` 정책을 설정합니다:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "if-not-present"
```

### `never` 풀 정책 설정 {#set-the-never-pull-policy}

전제 조건:

- 로컬 이미지에는 설치된 Docker Engine 및 사용된 이미지의 로컬 복사본이 포함되어야 합니다.

풀 정책을 `never`로 설정하면 이미지 풀이 비활성화됩니다. 사용자는 러너가 실행되는 Docker 호스트에서 수동으로 풀한 이미지만 사용할 수 있습니다.

`never` 풀 정책을 사용합니다:

- 러너 사용자가 사용하는 이미지를 제어합니다.
- 작업에만 사용할 수 있으며 공개 레지스트리에서 사용할 수 없는 특정 이미지만 사용할 수 있는 비공개 러너입니다.

**Do not use**. [자동 확장](../configuration/autoscale.md) Docker 실행기에서 `never` pull 정책을 사용하지 마세요. `never` 풀 정책은 선택한 클라우드 공급자의 사전 정의된 클라우드 인스턴스 이미지를 사용할 때만 사용 가능합니다.

`config.toml`에서 `never` 정책을 설정합니다:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = "never"
```

### 여러 풀 정책 설정 {#set-multiple-pull-policies}

풀 실패 시 여러 풀 정책을 실행할 수 있습니다. 러너는 풀 시도가 성공하거나 목록이 소진될 때까지 나열된 순서대로 풀 정책을 처리합니다. 예를 들어 러너가 `always` 풀 정책을 사용하고 레지스트리를 사용할 수 없는 경우 `if-not-present`을 두 번째 풀 정책으로 추가할 수 있습니다. 이 구성을 통해 러너는 로컬로 캐시된 Docker 이미지를 사용할 수 있습니다.

이 풀 정책의 보안 의미에 대한 자세한 내용은 [if-not-present 풀 정책이 포함된 비공개 Docker 이미지 사용](../security/_index.md#usage-of-private-docker-images-with-if-not-present-pull-policy)을 참조하세요.

`config.toml`에 목록으로 추가하려면:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    pull_policy = ["always", "if-not-present"]
```

### Docker 풀 정책 허용 {#allow-docker-pull-policies}

`.gitlab-ci.yml` 파일에서 풀 정책을 지정할 수 있습니다. 이 정책은 CI/CD 작업이 이미지를 가져오는 방식을 결정합니다.

`.gitlab-ci.yml` 파일에서 지정한 정책 중 사용할 수 있는 풀 정책을 제한하려면 `allowed_pull_policies`을 사용합니다.

예를 들어 `always` 및 `if-not-present` 풀 정책만 허용하려면 `config.toml`에 추가합니다:

```toml
[[runners]]
  (...)
  executor = "docker"
  [runners.docker]
    (...)
    allowed_pull_policies = ["always", "if-not-present"]
```

- `allowed_pull_policies`을 지정하지 않으면 목록이 `pull_policy` 키워드에 지정된 값과 일치합니다.
- `pull_policy`을 지정하지 않으면 기본값은 `always`입니다.
- 작업은 `pull_policy` 및 `allowed_pull_policies`에 나열된 풀 정책만 사용합니다. 유효한 풀 정책은 [`pull_policy` 키워드](#configure-how-runners-pull-images)에 지정된 정책 및 `allowed_pull_policies`를 비교하여 결정합니다. GitLab은 이러한 두 정책 목록의 [교집합](https://en.wikipedia.org/wiki/Intersection_(set_theory))을 사용합니다. 예를 들어 `pull_policy`이 `["always", "if-not-present"]`이고 `allowed_pull_policies`이 `["if-not-present"]`인 경우 작업은 두 목록에 정의된 유일한 풀 정책이므로 `if-not-present`만 사용합니다.
- 기존 `pull_policy` 키워드는 `allowed_pull_policies`에서 지정한 최소 하나의 풀 정책을 포함해야 합니다. 작업은 `pull_policy` 값 중 `allowed_pull_policies`와 일치하는 것이 없으면 실패합니다.

### 이미지 풀 오류 메시지 {#image-pull-error-messages}

| 오류 메시지                                                                                                                                                                                                                                                               | 설명 |
|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-------------|
| `Pulling docker image registry.tld/my/image:latest ... ERROR: Build failed: Error: image registry.tld/my/image:latest not found`                                                                                                                                            | 러너를 찾을 수 없습니다. `always` 풀 정책이 설정된 경우 표시됩니다. |
| `Pulling docker image local_image:latest ... ERROR: Build failed: Error: image local_image:latest not found`                                                                                                                                                                | 이미지가 로컬로 빌드되었으므로 공개 또는 기본 Docker 레지스트리에 없습니다. `always` 풀 정책이 설정된 경우 표시됩니다. |
| `Pulling docker image registry.tld/my/image:latest ... WARNING: Cannot pull the latest version of image registry.tld/my/image:latest : Error: image registry.tld/my/image:latest not found WARNING: Locally found image will be used instead.`                              | 러너가 이미지를 풀하는 대신 로컬 이미지를 사용했습니다. |
| `Pulling docker image local_image:latest ... ERROR: Build failed: Error: image local_image:latest not found`                                                                                                                                                                | 이미지를 로컬에서 찾을 수 없습니다. `never` 풀 정책이 설정된 경우 표시됩니다. |
| `WARNING: Failed to pull image with policy "always": Error response from daemon: received unexpected HTTP status: 502 Bad Gateway (docker.go:143:0s) Attempt #2: Trying "if-not-present" pull policy Using locally found image version due to "if-not-present" pull policy` | 러너가 이미지 풀에 실패했으며 목록에 있는 다음 풀 정책을 사용하여 이미지를 풀하려고 시도합니다. 여러 풀 정책이 설정된 경우 표시됩니다. |

## 실패한 풀 재시도 {#retry-a-failed-pull}

실패한 이미지 풀을 재시도하도록 러너를 구성하려면 `config.toml`에서 동일한 정책을 두 번 이상 지정합니다.

예를 들어 이 구성은 풀을 한 번 재시도합니다:

```toml
[runners.docker]
  pull_policy = ["always", "always"]
```

이 설정은 개별 작업의 `.gitlab-ci.yml` 파일에서 [`retry` 지시문](https://docs.gitlab.com/ci/yaml/#retry)과 유사하지만, Docker 풀이 초기에 실패한 경우에만 적용됩니다.

## Windows 컨테이너 사용 {#use-windows-containers}

Docker 실행기로 Windows 컨테이너를 사용하려면 제한사항, 지원되는 Windows 버전, Windows Docker 실행기 구성 및 Windows 도우미 이미지에 대한 다음 정보를 참조하세요.

### 지원되는 Windows 버전 {#supported-windows-versions}

GitLab 러너는 [Windows 지원 수명 주기](../install/support-policy.md#windows-version-support)를 따르는 다음 Windows 버전만 지원합니다:

- Windows Server 2025 LTSC (24H2)
- Windows Server 2022 LTSC (21H2)
- Windows Server 2019 LTSC (1809)

Windows 컨테이너는 호스트 OS 및 격리 모드를 기반으로 하는 역방향 호환성을 지원합니다. 최신 호스트는 이전 컨테이너 이미지를 실행할 수 있습니다. 호환성 세부정보는 [Microsoft Windows 컨테이너 버전 호환성 가이드라인](https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/version-compatibility)을 참조하세요.

`Server Core`, `Nano Server`, `Server`, `Windows` 등 다양한 Windows 기본 이미지를 사용할 수 있습니다. 예를 들어 호환 OS 버전이 있는 [`Windows Server Core`](https://hub.docker.com/r/microsoft/windows-servercore) 이미지를 사용합니다:

- `mcr.microsoft.com/windows/servercore:ltsc2025`
- `mcr.microsoft.com/windows/servercore:ltsc2025-amd64`
- `mcr.microsoft.com/windows/servercore:ltsc2022`
- `mcr.microsoft.com/windows/servercore:ltsc2022-amd64`
- `mcr.microsoft.com/windows/servercore:1809`
- `mcr.microsoft.com/windows/servercore:1809-amd64`
- `mcr.microsoft.com/windows/servercore:ltsc2019`

### 지원되는 Docker 버전 {#supported-docker-versions}

GitLab 러너는 Docker를 사용하여 실행 중인 Windows Server 버전을 감지합니다. 따라서 GitLab 러너를 실행하는 Windows Server는 최근 버전의 Docker를 실행해야 합니다.

GitLab 러너에서 작동하지 않는 알려진 Docker 버전은 `Docker 17.06`입니다. Docker가 Windows Server 버전을 식별하지 않으므로 다음 오류가 발생합니다:

```plaintext
unsupported Windows Version: Windows Server Datacenter
```

[이 문제 해결에 대해 자세히 알아보세요](../install/windows.md#docker-executor-unsupported-windows-version).

### Windows Docker 실행기 구성 {#configure-a-windows-docker-executor}

> [!note]
> `c:\\cache`이 소스 디렉토리로 러너가 등록되고 `--docker-volumes` 또는 `DOCKER_VOLUMES` 환경 변수를 전달할 때 [알려진 문제](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4312)가 있습니다.

다음은 Windows를 실행하는 Docker 실행기에 대한 구성 예입니다.

```toml
[[runners]]
  name = "windows-docker-2019"
  url = "https://gitlab.com/"
  token = "xxxxxxx"
  executor = "docker-windows"
  [runners.docker]
    image = "mcr.microsoft.com/windows/servercore:1809_amd64"
    volumes = ["c:\\cache"]
```

Docker 실행기의 다른 구성 옵션은 [고급 구성](../configuration/advanced-configuration.md#the-runnersdocker-section) 섹션을 참조하세요.

### Windows 헬퍼 이미지 {#windows-helper-images}

GitLab 러너는 다양한 Windows 버전과 PowerShell 요구 사항에 맞춘 여러 헬퍼 이미지를 제공합니다. 사용 가능한 변형:

- `gitlab/gitlab-runner-helper:x86_64-vXYZ-nanoserver21H2`
- `gitlab/gitlab-runner-helper:x86_64-vXYZ-servercore21H2`
- `gitlab/gitlab-runner-helper:x86_64-vXYZ-nanoserver1809`
- `gitlab/gitlab-runner-helper:x86_64-vXYZ-servercore1809`

> [!note]
> Windows 컨테이너 하위 호환성으로 인해 Windows Server 2025 (24H2)는 21H2 (Windows Server 2022) 헬퍼 이미지를 사용할 수 있습니다.

셸 요구 사항에 따라 헬퍼 이미지를 선택하세요. `servercore` 이미지는 기본값이며 `PowerShell`와 `Pwsh` 모두를 지원합니다. `pwsh`만 사용하는 컨테이너의 경우 더 가벼운 `nanoserver` 이미지를 사용하세요.

### 서비스 {#services}

[서비스](https://docs.gitlab.com/ci/services/) 를 사용할 수 있습니다. [작업마다 네트워크](#create-a-network-for-each-job)를 활성화하면 됩니다.

### Windows의 Docker 실행기 관련 알려진 문제 {#known-issues-with-docker-executor-on-windows}

다음은 Docker 실행기에서 Windows 컨테이너를 사용할 때의 몇 가지 제한 사항입니다:

- Docker-in-Docker는 지원되지 않습니다. Docker 자체에서 [지원되지 않기](https://github.com/docker-library/docker/issues/49) 때문입니다.
- 호스트 디바이스 마운팅은 지원되지 않습니다.
- 볼륨 디렉토리를 마운트할 때는 디렉토리가 존재해야 합니다. 그렇지 않으면 Docker가 컨테이너를 시작하지 못합니다. 자세한 내용은 [\#3754](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/3754)를 참조하세요.
- `docker-windows` 실행기는 Windows에서 실행 중인 GitLab 러너를 사용해야만 실행할 수 있습니다.
- [Windows의 Linux 컨테이너](https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/set-up-linux-containers)는 지원되지 않습니다. 아직 실험 단계이기 때문입니다. 자세한 내용은 [관련 이슈](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4373)를 참조하세요.
- [Docker의 제한](https://github.com/MicrosoftDocs/Virtualization-Documentation/pull/331) 때문에 대상 경로 드라이브 문자가 `c:`가 아니면 다음 경로는 지원되지 않습니다:

  - [`builds_dir`](../configuration/advanced-configuration.md#the-runners-section)
  - [`cache_dir`](../configuration/advanced-configuration.md#the-runners-section)
  - [`volumes`](../configuration/advanced-configuration.md#volumes-in-the-runnersdocker-section)

  이는 `f:\\cache_dir`과 같은 값은 지원되지 않지만 `f:`는 지원된다는 의미입니다. 그러나 대상 경로가 `c:` 드라이브에 있으면 경로도 지원됩니다(예: `c:\\cache_dir`).

  Docker 데몬이 이미지와 컨테이너를 저장할 위치를 구성하려면 Docker 데몬의 `daemon.json` 파일에서 `data-root` 매개변수를 업데이트하세요.

  자세한 내용은 [구성 파일로 Docker 구성](https://learn.microsoft.com/en-us/virtualization/windowscontainers/manage-docker/configure-docker-daemon#configure-docker-with-a-configuration-file)을 참조하세요.

## 네이티브 Step 러너 통합 {#native-step-runner-integration}

{{< history >}}

- GitLab 17.6.0에서 [도입](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5069)되었습니다. `FF_USE_NATIVE_STEPS` 기능 플래그 뒤에 있으며 기본적으로 비활성화되어 있습니다.
- GitLab 17.9.0에서 [업데이트](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/5322)되었습니다. GitLab 러너는 `step-runner` 바이너리를 빌드 컨테이너에 주입하고 `$PATH` 환경 변수를 그에 따라 조정합니다. 이 향상 기능을 통해 빌드 이미지로 모든 이미지를 사용할 수 있습니다.

{{< /history >}}

Docker 실행기는 [`step-runner`](https://gitlab.com/gitlab-org/step-runner)에서 제공하는 `gRPC` API를 사용하여 [CI/CD 단계](https://docs.gitlab.com/ci/steps/)를 기본적으로 실행할 수 있습니다.

이 실행 모드를 활성화하려면 레거시 `script` 키워드 대신 `run` 키워드를 사용하여 CI/CD 작업을 지정해야 합니다. 또한 `FF_USE_NATIVE_STEPS` 기능 플래그를 활성화해야 합니다. 이 기능 플래그는 작업 또는 파이프라인 수준에서 활성화할 수 있습니다.

```yaml
step job:
  stage: test
  variables:
    FF_USE_NATIVE_STEPS: true
  image:
    name: alpine:latest
  run:
    - name: step1
      script: pwd
    - name: step2
      script: env
    - name: step3
      script: ls -Rlah ../
```

### 알려진 문제 {#known-issues-1}

- GitLab 17.9 이상에서는 빌드 이미지에 `ca-certificates` 패키지가 설치되어 있어야 합니다. 그렇지 않으면 `step-runner`이 작업에서 정의한 단계를 가져오지 못합니다. 예를 들어 Debian 기반 Linux 배포판은 기본적으로 `ca-certificates`을 설치하지 않습니다.

- GitLab 17.9 이전 버전에서는 빌드 이미지에 `$PATH`에 `step-runner` 바이너리가 포함되어 있어야 합니다. 이를 수행하려면 다음 중 하나를 실행할 수 있습니다:

  - 사용자 정의 빌드 이미지를 만들고 `step-runner` 바이너리를 여기에 포함합니다.
  - 작업을 실행하는 데 필요한 종속성이 포함되어 있으면 `registry.gitlab.com/gitlab-org/step-runner:v0` 이미지를 사용하세요.

- Docker 컨테이너를 실행하는 단계를 실행하려면 전통적인 `scripts`과 동일한 구성 매개변수 및 제약 사항을 따라야 합니다. 예를 들어 [Docker-in-Docker](#use-docker-in-docker-with-privileged-mode)를 사용해야 합니다.
- 이 실행 모드는 아직 [`Github Actions`](https://gitlab.com/components/action-runner)를 실행할 수 없습니다.
