---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: GitLab 러너의 최신 개발 빌드를 설치합니다.
title: GitLab 러너 블리딩 엣지 릴리스
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!warning]
> 이 GitLab 러너 릴리스는 최신 버전이며 `main` 브랜치에서 직접 빌드되었으며 테스트되지 않았을 수 있습니다. 자신의 위험으로 사용하세요.

## 독립 실행형 바이너리 다운로드 {#download-the-standalone-binaries}

- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-386>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-amd64>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-arm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-s390x>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-riscv64>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-loong64>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-darwin-amd64>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-windows-386.exe>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-windows-amd64.exe>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-windows-arm64.exe>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-freebsd-386>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-freebsd-amd64>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-freebsd-arm>

그러면 GitLab 러너를 다음과 같이 실행할 수 있습니다:

```shell
chmod +x gitlab-runner-linux-amd64
./gitlab-runner-linux-amd64 run
```

## Debian 또는 Ubuntu용 패키지 중 하나 다운로드 {#download-one-of-the-packages-for-debian-or-ubuntu}

- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_i686.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_amd64.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_armel.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_armhf.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_arm64.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_aarch64.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_riscv64.deb>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/deb/gitlab-runner_loong64.deb>

### 내보낸 러너 도우미 이미지 패키지 다운로드 {#download-the-exported-runner-helper-images-package}

러너 도우미 이미지 패키지는 GitLab 러너 `.deb` 패키지의 필수 종속성입니다.

다음 위치에서 패키지를 다운로드합니다:

```plaintext
https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/deb/gitlab-runner-helper-images.deb
```

그러면 다음과 같이 설치할 수 있습니다:

```shell
dpkg -i gitlab-runner-helper-images.deb gitlab-runner_<arch>.deb
```

## Red Hat 또는 CentOS용 패키지 중 하나 다운로드 {#download-one-of-the-packages-for-red-hat-or-centos}

- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_i686.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_x86_64.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_armhfp.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_aarch64.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_riscv64.rpm>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/rpm/gitlab-runner_loongarch64.rpm>

### 내보낸 러너 도우미 이미지 패키지 다운로드 {#download-the-exported-runner-helper-images-package-1}

러너 도우미 이미지 패키지는 GitLab 러너 `.rpm` 패키지의 필수 종속성입니다.

다음 위치에서 패키지를 다운로드합니다:

```plaintext
https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/rpm/gitlab-runner-helper-images.rpm
```

그러면 다음과 같이 설치할 수 있습니다:

```shell
rpm -i gitlab-runner-helper-images.rpm gitlab-runner_<arch>.rpm
```

## 태그가 지정된 다른 릴리스 다운로드 {#download-any-other-tagged-release}

`main`을 `tag`(예: `v16.5.0`) 또는 `latest`(최신 안정 버전)으로 바꿉니다. 태그 목록은 <https://gitlab.com/gitlab-org/gitlab-runner/-/tags>을 참조하세요. 예를 들어:

- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-386>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-386>
- <https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/v16.5.0/binaries/gitlab-runner-linux-386>

`https`을 통해 다운로드하는 데 문제가 있으면 일반 `http`로 대체합니다:

- <http://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/main/binaries/gitlab-runner-linux-386>
- <http://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-linux-386>
- <http://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/v16.5.0/binaries/gitlab-runner-linux-386>
