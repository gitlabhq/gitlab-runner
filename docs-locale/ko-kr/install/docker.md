---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Docker 컨테이너에서 GitLab 러너를 실행합니다.
title: 컨테이너에서 GitLab 러너 실행
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Docker 컨테이너에서 GitLab 러너를 실행하여 CI/CD 작업을 수행할 수 있습니다. GitLab 러너 Docker 이미지에는 다음이 필요한 모든 종속성이 포함되어 있습니다:

- GitLab 러너를 실행합니다.
- 컨테이너에서 CI/CD 작업을 수행합니다.

GitLab 러너 Docker 이미지는 [Ubuntu 또는 Alpine Linux](#docker-images)를 기반으로 사용합니다. 표준 `gitlab-runner` 명령을 래핑하며, GitLab 러너를 호스트에 직접 설치하는 것과 유사합니다.

`gitlab-runner` 명령이 Docker 컨테이너에서 실행됩니다. 이 설정은 각 GitLab 러너 컨테이너에 Docker 데몬에 대한 전체 제어를 위임합니다. 그 결과 GitLab 러너를 다른 페이로드도 실행하는 Docker 데몬 내에서 실행하면 격리 보장이 손상됩니다.

이 설정에서 실행하는 모든 GitLab 러너 명령에는 `docker run` 동등 명령이 있습니다. 다음과 같습니다:

- 러너 명령: `gitlab-runner <runner command and options...>`
- Docker 명령: `docker run <chosen docker options...> gitlab/gitlab-runner <runner command and options...>`

예를 들어 GitLab 러너에 대한 최상위 도움말 정보를 얻으려면 명령의 `gitlab-runner` 부분을 `docker run [docker options] gitlab/gitlab-runner`로 바꿉니다. 다음과 같습니다:

```shell
docker run --rm -t -i gitlab/gitlab-runner --help

NAME:
   gitlab-runner - a GitLab Runner

USAGE:
   gitlab-runner [global options] command [command options] [arguments...]

VERSION:
   18.10.1 (3b43bf9f)

(...)
```

## Docker Engine 버전 호환성 {#docker-engine-version-compatibility}

Docker Engine과 GitLab 러너 컨테이너 이미지의 버전이 일치할 필요는 없습니다. GitLab 러너 이미지는 하위 및 상위 호환성을 지원합니다. 최신 기능과 보안 업데이트가 있는지 확인하려면 항상 최신 안정 [Docker Engine 버전](https://docs.docker.com/engine/install/)을 사용하세요.

## Docker 이미지 설치 및 컨테이너 시작 {#install-the-docker-image-and-start-the-container}

전제 조건:

- [Docker를 설치했습니다](https://docs.docker.com/get-started/get-docker/).
- GitLab 러너의 일반적인 문제에 대해 알아보기 위해 [FAQ](../faq/_index.md)를 읽었습니다.

1. `gitlab-runner` Docker 이미지를 `docker pull gitlab/gitlab-runner:<version-tag>` 명령으로 다운로드합니다.

   사용 가능한 버전 태그 목록은 [GitLab 러너 태그](https://hub.docker.com/r/gitlab/gitlab-runner/tags)를 참고하세요.
1. `gitlab-runner` Docker 이미지를 `docker run -d [options] <image-uri> <runner-command>` 명령으로 실행합니다.
1. Docker 컨테이너에서 `gitlab-runner`를 실행할 때 컨테이너를 다시 시작해도 설정이 손실되지 않는지 확인하세요. 설정을 저장하기 위해 영구 볼륨을 마운트합니다. 볼륨은 다음 중 하나에 마운트할 수 있습니다:

   - [로컬 시스템 볼륨](#from-a-local-system-volume)
   - [Docker 볼륨](#from-a-docker-volume)

1. 선택사항입니다. [`session_server`](../configuration/advanced-configuration.md)를 사용하는 경우 `8093` 포트를 노출하려면 `-p 8093:8093`를 `docker run` 명령에 추가하세요.
1. 선택사항입니다. 자동 크기 조정을 위해 Docker Machine 실행기를 사용하려면 Docker Machine 저장소 경로(`/root/.docker/machine`)를 `docker run` 명령에 볼륨 마운트를 추가하여 마운트하세요:

   - 시스템 볼륨 마운트의 경우 `-v /srv/gitlab-runner/docker-machine-config:/root/.docker/machine`를 추가합니다
   - Docker 명명된 볼륨의 경우 `-v docker-machine-config:/root/.docker/machine`를 추가합니다

1. [새 러너 등록](../register/_index.md). GitLab 러너 컨테이너를 등록해야 작업을 선택합니다.

사용 가능한 구성 옵션에는 다음이 포함됩니다:

- 플래그 `--env TZ=<TIMEZONE>`를 사용하여 컨테이너의 시간대를 설정합니다. [사용 가능한 시간대 목록 보기](https://en.wikipedia.org/wiki/List_of_tz_database_time_zones).
- [FIPS 호환 GitLab 러너](requirements.md#fips-compliant-gitlab-runner) 이미지의 경우 `redhat/ubi9-micro`를 기반으로 하며 `gitlab/gitlab-runner:ubi-fips` 태그를 사용합니다.
- [신뢰할 수 있는 SSL 서버 인증서 설치](#install-trusted-ssl-server-certificates).

### 로컬 시스템 볼륨에서 {#from-a-local-system-volume}

로컬 시스템을 구성 볼륨 및 `gitlab-runner` 컨테이너로 마운트된 다른 리소스에 사용하려면:

1. 선택사항입니다. MacOS 시스템에서는 `/srv`가 기본적으로 존재하지 않습니다. `/private/srv` 또는 다른 개인 디렉터리를 만들어 설정합니다.
1. 필요에 따라 이 명령을 수정하여 실행합니다:

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /srv/gitlab-runner/config:/etc/gitlab-runner \
     -v /var/run/docker.sock:/var/run/docker.sock \
     gitlab/gitlab-runner:latest
   ```

### Docker 볼륨에서 {#from-a-docker-volume}

구성 컨테이너를 사용하여 사용자 지정 데이터 볼륨을 마운트하려면:

1. Docker 볼륨을 만듭니다:

   ```shell
   docker volume create gitlab-runner-config
   ```

1. 방금 만든 볼륨을 사용하여 GitLab 러너 컨테이너를 시작합니다:

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /var/run/docker.sock:/var/run/docker.sock \
     -v gitlab-runner-config:/etc/gitlab-runner \
     gitlab/gitlab-runner:latest
   ```

## 러너 구성 업데이트 {#update-runner-configuration}

[러너 구성 변경](../configuration/advanced-configuration.md)한 후 `config.toml`에서 컨테이너를 `docker stop` 및 `docker run`로 다시 시작하여 변경 사항을 적용합니다.

## 러너 버전 업그레이드 {#upgrade-runner-version}

전제 조건:

- 원래 수행한 것과 동일한 데이터 볼륨 마운트 방법을 사용해야 합니다(`-v /srv/gitlab-runner/config:/etc/gitlab-runner` 또는 `-v gitlab-runner-config:/etc/gitlab-runner`).

1. 최신 버전(또는 특정 태그)을 가져옵니다:

   ```shell
   docker pull gitlab/gitlab-runner:latest
   ```

1. 기존 컨테이너를 중지하고 제거합니다:

   ```shell
   docker stop gitlab-runner && docker rm gitlab-runner
   ```

1. 원래 수행한 것처럼 컨테이너를 시작합니다:

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /var/run/docker.sock:/var/run/docker.sock \
     -v /srv/gitlab-runner/config:/etc/gitlab-runner \
     gitlab/gitlab-runner:latest
   ```

## 러너 로그 보기 {#view-runner-logs}

로그 파일 위치는 러너를 시작하는 방식에 따라 다릅니다. 다음과 같이 시작할 때:

- **Foreground task**(로컬에 설치된 바이너리 또는 Docker 컨테이너)으로 시작하면 로그가 `stdout`로 출력됩니다.
- **System service**(예: `systemd`)로 시작하면 로그는 Syslog 같은 시스템 로깅 메커니즘에서 사용 가능합니다.
- **Docker-based service**의 경우 `docker logs` 명령을 사용하세요. `gitlab-runner ...` 명령이 컨테이너의 주 프로세스입니다.

예를 들어 이 명령으로 컨테이너를 시작하면 이름이 `gitlab-runner`로 설정됩니다:

```shell
docker run -d --name gitlab-runner --restart always \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /srv/gitlab-runner/config:/etc/gitlab-runner \
  gitlab/gitlab-runner:latest
```

로그를 보려면 이 명령을 실행하고 `gitlab-runner`을 컨테이너 이름으로 바꿉니다:

```shell
docker logs gitlab-runner
```

컨테이너 로그 처리에 대한 자세한 내용은 Docker 설명서의 [`docker container logs`](https://docs.docker.com/reference/cli/docker/container/logs/)를 참고하세요.

## 신뢰할 수 있는 SSL 서버 인증서 설치 {#install-trusted-ssl-server-certificates}

GitLab CI/CD 서버가 자체 서명 SSL 인증서를 사용하는 경우 러너 컨테이너가 GitLab CI 서버 인증서를 신뢰하는지 확인합니다. 이렇게 하면 통신 오류를 방지할 수 있습니다.

전제 조건:

- `ca.crt` 파일에는 GitLab 러너를 신뢰하려는 모든 서버의 루트 인증서가 포함되어야 합니다.

1. 선택사항입니다. `gitlab/gitlab-runner` 이미지는 `/etc/gitlab-runner/certs/ca.crt`에서 신뢰할 수 있는 SSL 인증서를 찾습니다. 이 동작을 변경하려면 `-e "CA_CERTIFICATES_PATH=/DIR/CERT"` 구성 옵션을 사용하세요.
1. `ca.crt` 파일을 데이터 볼륨(또는 컨테이너)의 `certs` 디렉터리로 복사합니다.
1. 선택사항입니다. 컨테이너가 이미 실행 중인 경우 시작 시 `ca.crt` 파일을 가져오도록 다시 시작합니다.

## Docker 이미지 {#docker-images}

GitLab 러너 18.8.0에서 Alpine 기반 Docker 이미지는 Alpine 3.21을 사용합니다. 다음의 다중 플랫폼 Docker 이미지를 사용할 수 있습니다:

- `gitlab/gitlab-runner:latest` (Ubuntu 기반, 약 470MB)
- `gitlab/gitlab-runner:alpine` (Alpine 기반, 약 270MB)

[GitLab 러너](https://gitlab.com/gitlab-org/gitlab-runner/tree/main/dockerfiles) 소스에서 Ubuntu 및 Alpine 이미지 모두에 대한 빌드 지침을 참고하세요.

### 러너 Docker 이미지 만들기 {#create-a-runner-docker-image}

GitLab 리포지토리에서 업데이트를 사용할 수 있기 전에 이미지의 운영 체제를 업그레이드할 수 있습니다.

전제 조건:

- IBM Z 이미지를 사용하지 않고 있습니다. `docker-machine` 종속성이 포함되어 있지 않기 때문입니다. 이 이미지는 Linux s390x 또는 Linux ppc64le 플랫폼에서 유지 관리되지 않습니다. 현재 상태는 [이슈 26551](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26551)을 참고하세요.

최신 Alpine 버전의 `gitlab-runner` Docker 이미지를 빌드하려면:

1. `alpine-upgrade/Dockerfile`를 만듭니다.

   ```dockerfile
   ARG GITLAB_RUNNER_IMAGE_TYPE
   ARG GITLAB_RUNNER_IMAGE_TAG
   FROM gitlab/${GITLAB_RUNNER_IMAGE_TYPE}:${GITLAB_RUNNER_IMAGE_TAG}

   RUN apk update
   RUN apk upgrade
   ```

1. 업그레이드된 `gitlab-runner` 이미지를 만듭니다.

   ```shell
   GITLAB_RUNNER_IMAGE_TYPE=gitlab-runner \
   GITLAB_RUNNER_IMAGE_TAG=alpine-v18.10.1 \
   docker build -t $GITLAB_RUNNER_IMAGE_TYPE:$GITLAB_RUNNER_IMAGE_TAG \
     --build-arg GITLAB_RUNNER_IMAGE_TYPE=$GITLAB_RUNNER_IMAGE_TYPE \
     --build-arg GITLAB_RUNNER_IMAGE_TAG=$GITLAB_RUNNER_IMAGE_TAG \
     -f alpine-upgrade/Dockerfile alpine-upgrade
   ```

1. 업그레이드된 `gitlab-runner-helper` 이미지를 만듭니다.

   ```shell
   GITLAB_RUNNER_IMAGE_TYPE=gitlab-runner-helper \
   GITLAB_RUNNER_IMAGE_TAG=x86_64-v18.10.1 \
   docker build -t $GITLAB_RUNNER_IMAGE_TYPE:$GITLAB_RUNNER_IMAGE_TAG \
     --build-arg GITLAB_RUNNER_IMAGE_TYPE=$GITLAB_RUNNER_IMAGE_TYPE \
     --build-arg GITLAB_RUNNER_IMAGE_TAG=$GITLAB_RUNNER_IMAGE_TAG \
     -f alpine-upgrade/Dockerfile alpine-upgrade
   ```

## 컨테이너에서 SELinux 사용 {#use-selinux-in-your-container}

CentOS, Red Hat, Fedora 같은 일부 배포판은 기본적으로 SELinux(Security-Enhanced Linux)를 사용하여 기본 시스템의 보안을 강화합니다.

이 구성을 사용할 때 주의하세요.

전제 조건:

- [Docker 실행기](../executors/docker.md)를 사용하여 컨테이너에서 빌드를 실행하려면 러너는 `/var/run/docker.sock`에 액세스해야 합니다.
- SELinux를 강제 모드로 사용하는 경우 [`selinux-dockersock`](https://github.com/dpw/selinux-dockersock)를 설치하여 러너가 `/var/run/docker.sock`에 액세스할 때 `Permission denied` 오류를 방지하세요.

1. 호스트에 영구 디렉터리 만들기: `mkdir -p /srv/gitlab-runner/config`.
1. Docker를 `:Z`로 볼륨에서 실행합니다:

   ```shell
   docker run -d --name gitlab-runner --restart always \
     -v /var/run/docker.sock:/var/run/docker.sock \
     -v /srv/gitlab-runner/config:/etc/gitlab-runner:Z \
     gitlab/gitlab-runner:latest
   ```
