---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: GitLab 러너 바이너리를 Linux에 수동으로 다운로드하고 설치합니다.
title: GNU/Linux에서 GitLab 러너를 수동으로 설치
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

`deb` 또는 `rpm` 패키지 또는 바이너리 파일을 사용하여 GitLab 러너를 수동으로 설치할 수 있습니다. 다음 중 하나에 해당하는 경우 이 방법을 마지막 수단으로 사용하세요:

- deb/rpm 리포지토리를 사용하여 GitLab 러너를 설치할 수 없습니다
- GNU/Linux OS가 지원되지 않습니다

## 필수 요구 사항 {#prerequisites}

GitLab 러너를 수동으로 실행하기 전에:

- Docker 실행기를 사용할 계획이라면 먼저 Docker를 설치하세요.
- 일반적인 문제 및 해결 방법에 대해 FAQ 섹션을 검토하세요.

## deb/rpm 패키지 사용 {#using-debrpm-package}

`deb` 또는 `rpm` 패키지를 사용하여 GitLab 러너를 다운로드하고 설치할 수 있습니다.

### 다운로드 {#download}

시스템에 맞는 패키지를 다운로드하려면:

1. <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html>에서 최신 파일명과 옵션을 찾으세요.
1. 패키지 관리자 또는 아키텍처와 일치하는 runner-helper 버전을 다운로드하세요.
1. 버전을 선택하고 [기타 태그된 릴리스 다운로드](bleeding-edge.md#download-any-other-tagged-release)에 설명된 대로 바이너리를 다운로드하여 최신 버전의 GitLab 러너를 사용하세요.

예를 들어 Debian 또는 Ubuntu의 경우:

```shell
# Replace ${arch} with any of the supported architectures, e.g. amd64, arm, arm64
# A full list of architectures can be found here https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/deb/gitlab-runner-helper-images.deb"
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/deb/gitlab-runner_${arch}.deb"
```

예를 들어 CentOS 또는 Red Hat Enterprise Linux의 경우:

```shell
# Replace ${arch} with any of the supported architectures, e.g. x86_64, aarch64, armhfp
# A full list of architectures can be found here https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner-helper-images.rpm"
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner_${arch}.rpm"
```

예를 들어 RHEL에서 FIPS 준수 GitLab 러너의 경우:

```shell
# Currently only x86_64 is a supported arch
# The FIPS compliant GitLab Runner version continues to include the helper images in one package.
# A full list of architectures can be found here https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/index.html
curl -LJO "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner_x86_64-fips.rpm"
```

### 설치 {#install}

1. 다음과 같이 시스템용 패키지를 설치하세요.

   예를 들어 Debian 또는 Ubuntu의 경우:

   ```shell
   dpkg -i gitlab-runner-helper-images.deb gitlab-runner_<arch>.deb
   ```

   예를 들어 CentOS 또는 Red Hat Enterprise Linux의 경우:

   ```shell
   dnf install -y gitlab-runner-helper-images.rpm gitlab-runner_<arch>.rpm
   ```

### 업그레이드 {#upgrade}

시스템의 최신 패키지를 다운로드한 후 다음과 같이 업그레이드하세요:

예를 들어 Debian 또는 Ubuntu의 경우:

```shell
dpkg -i gitlab-runner_<arch>.deb
```

예를 들어 CentOS 또는 Red Hat Enterprise Linux의 경우:

```shell
dnf install -y gitlab-runner-helper-images.rpm gitlab-runner_<arch>.rpm
```

## 바이너리 파일 사용 {#using-binary-file}

바이너리 파일을 사용하여 GitLab 러너를 다운로드하고 설치할 수 있습니다.

### 설치 {#install-1}

1. 시스템용 바이너리 중 하나를 다운로드하세요:

   ```shell
   # Linux x86-64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64"

   # Linux x86
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-386"

   # Linux arm
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-arm"

   # Linux arm64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-arm64"

   # Linux s390x
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-s390x"

   # Linux ppc64le
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-ppc64le"

   # Linux riscv64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-riscv64"

   # Linux loong64
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-loong64"

   # Linux x86-64 FIPS Compliant
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64-fips"
   ```

   [최신 버전 - 기타 태그된 릴리스 다운로드](bleeding-edge.md#download-any-other-tagged-release)에 설명된 대로 모든 사용 가능한 버전의 바이너리를 다운로드할 수 있습니다.

1. 실행 권한을 부여하세요:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. GitLab CI 사용자 생성:

   ```shell
   sudo useradd --comment 'GitLab Runner' --create-home gitlab-runner --shell /bin/bash
   ```

1. 설치 및 서비스로 실행:

   ```shell
   sudo gitlab-runner install --user=gitlab-runner --working-directory=/home/gitlab-runner
   sudo gitlab-runner start
   ```

   `/usr/local/bin/`이 root용 `$PATH`에 있는지 확인하세요. 그렇지 않으면 `command not found` 오류가 발생할 수 있습니다. 또는 `gitlab-runner`을 `/usr/bin/`과 같은 다른 위치에 설치할 수 있습니다.

> [!note]
> `gitlab-runner`을 서비스로 설치하고 실행하면 root로 실행되지만 `install` 명령으로 지정된 사용자로 작업을 실행합니다. 이는 캐시 및 아티팩트와 같은 일부 작업 함수가 `/usr/local/bin/gitlab-runner` 명령을 실행해야 함을 의미합니다. 따라서 작업을 실행하는 사용자는 실행 파일에 대한 액세스 권한이 있어야 합니다.

### 업그레이드 {#upgrade-1}

1. 서비스를 중지하세요 (이전과 같이 승격된 명령 프롬프트가 필요합니다):

   ```shell
   sudo gitlab-runner stop
   ```

1. GitLab 러너 실행 파일을 대체할 바이너리를 다운로드하세요. 예를 들어:

   ```shell
   sudo curl -L --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-amd64"
   ```

   [최신 버전 - 기타 태그된 릴리스 다운로드](bleeding-edge.md#download-any-other-tagged-release)에 설명된 대로 모든 사용 가능한 버전의 바이너리를 다운로드할 수 있습니다.

1. 실행 권한을 부여하세요:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. 서비스를 시작하세요:

   ```shell
   sudo gitlab-runner start
   ```

## 다음 단계 {#next-steps}

설치 후 [러너 등록](../register/_index.md)을 수행하여 설정을 완료하세요.

러너 바이너리에는 미리 빌드된 헬퍼 이미지가 포함되지 않습니다. 다음 명령을 사용하여 해당 버전의 헬퍼 이미지 아카이브를 다운로드하고 적절한 위치에 복사할 수 있습니다:

```shell
mkdir -p /usr/local/bin/out/helper-images
cd /usr/local/bin/out/helper-images
```

아키텍처에 맞는 헬퍼 이미지를 선택하세요:

<details>
<summary>Ubuntu 헬퍼 이미지</summary>

```shell
# Linux x86-64 ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-x86_64.tar.xz

# Linux x86-64 ubuntu pwsh
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-x86_64-pwsh.tar.xz

# Linux s390x ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-s390x.tar.xz

# Linux ppc64le ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-ppc64le.tar.xz

# Linux arm64 ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-arm64.tar.xz

# Linux arm ubuntu
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-ubuntu-arm.tar.xz

# Linux x86-64 ubuntu specific version - v17.10.0
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/v17.10.0/helper-images/prebuilt-ubuntu-x86_64.tar.xz
```

</details>

<details>
<summary>Alpine 헬퍼 이미지</summary>

```shell
# Linux x86-64 alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-x86_64.tar.xz

# Linux x86-64 alpine pwsh
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-x86_64-pwsh.tar.xz

# Linux s390x alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-s390x.tar.xz

# Linux riscv64 alpine edge
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-edge-riscv64.tar.xz

# Linux arm64 alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-arm64.tar.xz

# Linux arm alpine
wget https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/helper-images/prebuilt-alpine-arm.tar.xz
```

</details>

## 추가 리소스 {#additional-resources}

- [Docker 실행기 설명서](../executors/docker.md)
- [Docker 설치](https://docs.docker.com/engine/install/centos/#install-docker-ce)
- [기타 GitLab 러너 버전 다운로드](bleeding-edge.md#download-any-other-tagged-release)
- [FIPS 준수 GitLab 러너 정보](requirements.md#fips-compliant-gitlab-runner)
- [GitLab 러너 FAQ](../faq/_index.md)
- [deb/rpm 리포지토리 설치](linux-repository.md)
