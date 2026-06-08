---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: 패키지 관리자를 사용하여 GitLab 리포지토리에서 GitLab 러너를 설치합니다.
title: 공식 GitLab 리포지토리를 사용하여 GitLab 러너 설치
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab 러너를 설치하려면 [GitLab 리포지토리](https://packages.gitlab.com/runner/gitlab-runner)의 패키지를 사용할 수 있습니다.

## 지원되는 배포판 {#supported-distributions}

GitLab은 다음 지원되는 Linux 배포판 버전에 대한 패키지를 제공합니다. 새로운 러너 `deb` 또는 `rpm` 패키지는 패키지 호스팅 시스템에서 지원할 때 새로운 OS 배포판 릴리스에 자동으로 추가됩니다.

<!-- supported_os_versions_list_start -->

### Deb 기반 배포판 {#deb-based-distributions}

| 배포판 | 지원되는 버전 |
|--------------|--------------------|
| Debian | Duke, Forky, Trixie, Bookworm, Bullseye |
| LinuxMint | Xia, Wilma, Virginia, Victoria, Vera, Vanessa |
| Raspbian | Duke, Forky, Trixie, Bookworm, Bullseye |
| Ubuntu | Questing, Noble, Jammy, Focal, Bionic |

### RPM 기반 배포판 {#rpm-based-distributions}

| 배포판 | 지원되는 버전 |
|--------------|--------------------|
| Amazon Linux | 2025, 2023, 2 |
| Red Hat Enterprise Linux | 10, 9, 8, 7 |
| Fedora | 43, 42 |
| Oracle Linux | 10, 9, 8, 7 |
| openSUSE | 16.0, 15.6 |
| SUSE Linux Enterprise Server | 15.7, 15.6, 15.5, 15.4, 12.5 |

<!-- supported_os_versions_list_end -->

설정에 따라 다른 Debian 또는 RPM 기반 배포판도 지원될 수 있습니다. 지원되는 GitLab 러너 배포판의 파생 배포판이고 호환되는 패키지 리포지토리를 가진 배포판을 의미합니다. 예를 들어 Deepin은 Debian 파생 배포판입니다. 따라서 러너 `deb` 패키지를 Deepin에서 설치하고 실행할 수 있습니다. 다른 Linux 배포판에서 [GitLab 러너를 바이너리로 설치](linux-manually.md#using-binary-file)할 수도 있습니다.

> [!note]
> 목록에 없는 배포판의 패키지는 패키지 리포지토리에서 사용할 수 없습니다. S3 버킷에서 RPM 또는 DEB 패키지를 다운로드하여 수동으로 [설치](linux-manually.md#using-debrpm-package)할 수 있습니다.

## GitLab Runner 설치 {#install-gitlab-runner}

GitLab Runner를 설치하려면:

1. 공식 GitLab 리포지토리를 추가합니다:

   {{< tabs >}}

   {{< tab title="Debian/Ubuntu/Mint" >}}

   1. 리포지토리 구성 스크립트를 다운로드합니다:

      ```shell
      curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" -o script.deb.sh
      ```

   1. 스크립트를 실행하기 전에 검사합니다:

      ```shell
      less script.deb.sh
      ```

   1. 스크립트를 실행합니다:

      ```shell
      sudo bash script.deb.sh
      ```

   {{< /tab >}}

   {{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

   1. 리포지토리 구성 스크립트를 다운로드합니다:

      ```shell
      curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.rpm.sh" -o script.rpm.sh
      ```

   1. 스크립트를 실행하기 전에 검사합니다:

      ```shell
      less script.rpm.sh
      ```

   1. 스크립트를 실행합니다:

      ```shell
      sudo bash script.rpm.sh
      ```

   {{< /tab >}}

   {{< /tabs >}}

1. GitLab 러너의 최신 버전을 설치하거나 다음 단계로 건너뛰어 특정 버전을 설치합니다:

   > [!note]
   > `skel` 디렉토리 사용은 기본적으로 비활성화되어 [`No such file or directory` 작업 오류](#error-no-such-file-or-directory-job-failures)를 방지합니다.

   {{< tabs >}}

   {{< tab title="Debian/Ubuntu/Mint" >}}

   ```shell
   sudo apt install gitlab-runner
   ```

   {{< /tab >}}

   {{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

   ```shell
   sudo yum install gitlab-runner

   or

   sudo dnf install gitlab-runner
   ```

   {{< /tab >}}

   {{< /tabs >}}

   > [!note]
   > RHEL 배포판용 FIPS 140-2 호환 GitLab 러너 버전을 사용할 수 있습니다. `gitlab-runner-fips`를 패키지 이름으로 사용하여 이 버전을 설치할 수 있습니다. `gitlab-runner` 대신에 사용합니다.

1. GitLab 러너의 특정 버전을 설치하려면:

   {{< tabs >}}

   {{< tab title="Debian/Ubuntu/Mint" >}}

   > [!note]
   > `gitlab-runner` 버전 `v17.7.1`부터 `gitlab-runner`의 특정 버전을 설치할 때 해당 버전에 필요한 `gitlab-runner-helper-packages`를 명시적으로 설치해야 합니다. 이 요구 사항은 `apt`/`apt-get` 제한으로 인해 존재합니다.

   ```shell
   apt-cache madison gitlab-runner
   sudo apt install gitlab-runner=17.7.1-1 gitlab-runner-helper-images=17.7.1-1
   ```

   `gitlab-runner` 의 특정 버전을 설치하려고 할 때 `gitlab-runner-helper-images`의 동일한 버전을 설치하지 않으면 다음 오류가 발생할 수 있습니다:

   ```shell
   sudo apt install gitlab-runner=17.7.1-1
   ...
   The following packages have unmet dependencies:
    gitlab-runner : Depends: gitlab-runner-helper-images (= 17.7.1-1) but 17.8.3-1 is to be installed
   E: Unable to correct problems, you have held broken packages.
   ```

   {{< /tab >}}

   {{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

   ```shell
   yum list gitlab-runner --showduplicates | sort -r
   sudo yum install gitlab-runner-17.2.0-1
   ```

   {{< /tab >}}

   {{< /tabs >}}

1. [러너 등록](../register/_index.md)합니다.

위 단계를 완료한 후 러너를 시작할 수 있고 프로젝트와 함께 사용할 수 있습니다!

GitLab 러너와 관련된 가장 일반적인 문제를 설명하는 [FAQ](../faq/_index.md) 섹션을 읽어야 합니다.

## 보조 이미지 패키지 {#helper-images-package}

`gitlab-runner-helper-images` 패키지는 GitLab 러너가 작업 실행 중에 사용하는 미리 빌드된 보조 컨테이너 이미지를 포함합니다. 이러한 이미지는 리포지토리를 복제하고 아티팩트를 업로드하고 캐시를 관리하는 데 필요한 도구와 유틸리티를 제공합니다.

`gitlab-runner-helper-images` 패키지는 다음 운영 체제 및 아키텍처에 대한 보조 이미지를 포함합니다:

Alpine 기반 이미지 (최신):

- `alpine-arm`
- `alpine-arm64`
- `alpine-riscv64`
- `alpine-s390x`
- `alpine-x86_64`
- `alpine-x86_64-pwsh`

Ubuntu 기반 이미지 (24.04):

- `ubuntu-arm`
- `ubuntu-arm64`
- `ubuntu-ppc64le`
- `ubuntu-s390x`
- `ubuntu-x86_64`
- `ubuntu-x86_64-pwsh`

### 자동 보조 이미지 다운로드 {#automatic-helper-image-download}

특정 운영 체제 및 아키텍처 조합에 대한 보조 이미지를 호스트 시스템에서 사용할 수 없으면 GitLab 러너가 필요할 때 필요한 이미지를 자동으로 다운로드합니다. `gitlab-runner-helper-images package`에 포함되지 않은 아키텍처에는 수동 설치가 필요하지 않습니다. 이 자동 다운로드는 러너가 수동 개입이나 별도의 패키지 설치 없이 추가 아키텍처(예: `loong64`)를 지원할 수 있도록 합니다.

## GitLab Runner 업그레이드 {#upgrade-gitlab-runner}

GitLab 러너의 최신 버전을 설치하려면:

{{< tabs >}}

{{< tab title="Debian/Ubuntu/Mint" >}}

```shell
sudo apt update
sudo apt install gitlab-runner
```

{{< /tab >}}

{{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

```shell
sudo yum update
sudo yum install gitlab-runner
```

{{< /tab >}}

{{< /tabs >}}

## 패키지 설치를 위한 GPG 서명 {#gpg-signatures-for-package-installation}

GitLab 러너 프로젝트는 패키지 설치 방법을 위한 두 가지 유형의 GPG 서명을 제공합니다:

- [리포지토리 메타데이터 서명](#repository-metadata-signing)
- [패키지 서명](#package-signing)

### 리포지토리 메타데이터 서명 {#repository-metadata-signing}

원격 리포지토리에서 다운로드한 패키지 정보를 신뢰할 수 있는지 확인하기 위해 패키지 관리자는 리포지토리 메타데이터 서명을 사용합니다.

`apt-get update`와 같은 명령을 사용할 때 서명이 확인되므로 사용 가능한 패키지에 대한 정보가 **before any package is downloaded and installed** 업데이트됩니다. 확인 실패로 인해 패키지 관리자가 메타데이터를 거부해야 합니다. 즉, 서명 불일치를 야기한 문제를 찾아 해결할 때까지 리포지토리에서 패키지를 다운로드하고 설치할 수 없습니다.

패키지 메타데이터 서명 확인에 사용되는 GPG 공개 키는 위의 지침으로 수행된 첫 번째 설치에서 자동으로 설치됩니다. 향후 키 업데이트의 경우 기존 사용자는 새로운 키를 수동으로 다운로드하고 설치해야 합니다.

<https://packages.gitlab.com> 아래에서 호스팅되는 모든 프로젝트에 대해 하나의 키를 사용합니다. [Linux 패키지 설명서](https://docs.gitlab.com/omnibus/update/package_signatures/#package-repository-metadata-signing-key)에서 사용된 키에 대한 세부 정보를 찾을 수 있습니다. 이 설명서 페이지에는 또한 [과거에 사용된 모든 키](https://docs.gitlab.com/omnibus/update/package_signatures/#previous-package-signing-keys)가 나열되어 있습니다.

### 패키지 서명 {#package-signing}

리포지토리 메타데이터 서명은 다운로드된 버전 정보가 <https://packages.gitlab.com>에서 시작되었음을 증명합니다. 패키지 자체의 무결성을 증명하지 않습니다. <https://packages.gitlab.com>에 업로드된 모든 것(승인되었거나 승인되지 않았거나)은 리포지토리에서 사용자로의 메타데이터 전송이 영향을 받지 않을 때까지 올바르게 확인됩니다.

패키지 서명을 사용하면 각 패키지는 빌드될 때 서명됩니다. 빌드 환경과 사용된 GPG 키의 비밀성을 신뢰할 수 있을 때까지 패키지 진정성을 확인할 수 없습니다. 패키지에 대한 유효한 서명은 원본이 인증되었고 무결성이 침해되지 않았음을 증명합니다.

패키지 서명 확인은 일부 Debian/RPM 기반 배포판에서만 기본적으로 활성화됩니다. 이 유형의 확인을 사용하려면 구성을 조정해야 할 수 있습니다.

<https://packages.gitlab.com>에서 호스팅되는 각 리포지토리에 대해 패키지 서명 확인에 사용되는 GPG 키가 다를 수 있습니다. GitLab 러너 프로젝트는 이 유형의 서명에 자체 키 쌍을 사용합니다.

#### RPM 기반 배포판 {#rpm-based-distributions-1}

RPM 형식은 GPG 서명 기능의 전체 구현을 포함하며, 따라서 해당 형식을 기반으로 하는 패키지 관리 시스템과 완전히 통합됩니다.

RPM 기반 배포판에 대한 패키지 서명 확인을 구성하는 방법에 대한 기술적 설명은 [Linux 패키지 설명서](https://docs.gitlab.com/omnibus/update/package_signatures/#rpm-based-distributions)에서 찾을 수 있습니다. GitLab 러너 차이점은:

- 설치해야 하는 공개 키 패키지의 이름은 `gpg-pubkey-35dfa027-60ba0235`입니다.
- RPM 기반 배포판의 리포지토리 파일의 이름은 `/etc/yum.repos.d/runner_gitlab-runner.repo`(안정 릴리스의 경우) 또는 `/etc/yum.repos.d/runner_unstable.repo`(불안정한 릴리스의 경우)입니다.
- [패키지 서명 공개 키](#current-gpg-public-key)는 `https://packages.gitlab.com/gpgkey/runner/49F16C5CC3A0F81F.pub.gpg`에서 가져올 수 있습니다.

#### Debian 기반 배포판 {#debian-based-distributions}

`deb` 형식은 공식적으로 패키지에 서명하는 기본 및 포함된 방법을 포함하지 않습니다. GitLab 러너 프로젝트는 패키지에 대한 서명에 서명하고 서명을 확인하기 위해 `dpkg-sig` 도구를 사용합니다. 이 방법은 패키지의 수동 확인만 지원합니다.

`deb` 패키지를 확인하려면:

1. `dpkg-sig`를 설치합니다:

   ```shell
   apt update && apt install dpkg-sig
   ```

1. [패키지 서명 공개 키](#current-gpg-public-key)를 다운로드하고 가져옵니다:

   ```shell
   curl -JLO "https://packages.gitlab.com/gpgkey/runner/49F16C5CC3A0F81F.pub.gpg"
   gpg --import runner-gitlab-runner-49F16C5CC3A0F81F.pub.gpg
   ```

1. `dpkg-sig`로 다운로드한 패키지를 확인합니다:

   ```shell
   dpkg-sig --verify gitlab-runner_amd64.deb
   Processing gitlab-runner_amd64.deb...
   GOODSIG _gpgbuilder 931DA69CFA3AFEBBC97DAA8C6C57C29C6BA75A4E 1623755049
   ```

   패키지에 유효하지 않은 서명이 있거나 유효하지 않은 키(예: 해지된 키)로 서명된 경우 출력은 다음과 유사합니다:

   ```shell
   dpkg-sig --verify gitlab-runner_amd64.deb
   Processing gitlab-runner_amd64.deb...
   BADSIG _gpgbuilder
   ```

   사용자의 키링에 키가 없으면 출력은 다음과 유사합니다:

   ```shell
   dpkg-sig --verify gitlab-runner_amd64.v13.1.0.deb
   Processing gitlab-runner_amd64.v13.1.0.deb...
   UNKNOWNSIG _gpgbuilder 880721D4
   ```

#### 현재 GPG 공개 키 {#current-gpg-public-key}

`https://packages.gitlab.com/runner/gitlab-runner/gpgkey/runner-gitlab-runner-49F16C5CC3A0F81F.pub.gpg`에서 패키지 서명에 사용되는 현재 공개 GPG 키를 다운로드합니다.

| 키 특성 | 값 |
|---------------|-------|
| 이름          | `GitLab, Inc.` |
| 이메일         | `support@gitlab.com` |
| 지문   | `931D A69C FA3A FEBB C97D  AA8C 6C57 C29C 6BA7 5A4E` |
| 만료        | `2026-04-28` |

> [!note]
> GitLab 러너 프로젝트는 `<https://gitlab-runner-downloads.s3.dualstack.us-east-1.amazonaws.com>` 버킷에서 사용 가능한 S3 릴리스를 위해 `release.sha256` 파일에 서명하는 데 동일한 키를 사용합니다.

#### 이전 GPG 공개 키 {#previous-gpg-public-keys}

과거에 사용된 키는 아래 표에서 찾을 수 있습니다.

해지된 키의 경우 패키지 서명 확인 구성에서 제거하는 것이 좋습니다.

다음 키로 만든 서명은 더 이상 신뢰해서는 안 됩니다.

| Sl. 번호 | 키 지문                                      | 상태    | 만료 날짜  | 다운로드 (해지된 키만) |
|---------|------------------------------------------------------|-----------|--------------|------------------------------|
| 1       | `3018 3AC2 C4E2 3A40 9EFB  E705 9CE4 5ABC 8807 21D4` | `revoked` | `2021-06-08` | [해지된 키](https://gitlab.com/gitlab-org/gitlab-runner/-/blob/main/docs/install/gpg-keys/9CE45ABC880721D4.pub.gpg) |
| 2       | `09E5 7083 F34C CA94 D541  BC58 A674 BF81 35DF A027` | `revoked` | `2023-04-26` | [해지된 키](https://gitlab.com/gitlab-org/gitlab-runner/-/blob/main/docs/install/gpg-keys/A674BF8135DFA027.pub.gpg) |

## 문제 해결 {#troubleshooting}

GitLab 러너를 설치할 때 문제를 해결하고 해결하기 위한 몇 가지 팁이 있습니다.

### 오류: `No such file or directory` 작업 오류 {#error-no-such-file-or-directory-job-failures}

기본 스켈레톤(`skel`) 디렉토리가 GitLab 러너에 문제를 일으키고 작업 실행에 실패하는 경우가 있습니다. [이슈 4449](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4449) 및 [이슈 1379](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1379)를 참조하세요.

이를 방지하려면 GitLab 러너를 설치할 때 `gitlab-runner` 사용자가 생성되고 기본적으로 홈 디렉토리가 스켈레톤 없이 생성됩니다. `skel`의 사용으로 홈 디렉토리에 추가된 쉘 구성이 작업 실행에 방해가 될 수 있습니다. 이 구성은 위에서 언급한 것과 같은 예상치 못한 문제를 발생시킬 수 있습니다.

`skel`의 회피가 기본 동작이 되기 전에 러너를 생성했다면 다음 dotfile을 제거해 볼 수 있습니다:

```shell
sudo rm /home/gitlab-runner/.profile
sudo rm /home/gitlab-runner/.bashrc
sudo rm /home/gitlab-runner/.bash_logout
```

`skel` 디렉토리를 사용하여 새로 생성된 `$HOME` 디렉토리를 채우려면 `GITLAB_RUNNER_DISABLE_SKEL` 변수를 명시적으로 `false`로 설정한 후 러너를 설치해야 합니다:

{{< tabs >}}

{{< tab title="Debian/Ubuntu/Mint" >}}

```shell
export GITLAB_RUNNER_DISABLE_SKEL=false; sudo -E apt-get install gitlab-runner
```

{{< /tab >}}

{{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

```shell
export GITLAB_RUNNER_DISABLE_SKEL=false; sudo -E yum install gitlab-runner
```

{{< /tab >}}

{{< /tabs >}}
