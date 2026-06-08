---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 자체 관리 러너의 보안
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

은 단순하거나 복잡한 DevOps 자동화 작업에 사용되는 워크플로우 자동화 엔진입니다. 이러한 파이프라인은 원격 코드 실행 서비스를 지원하므로 보안 위험을 줄이기 위해 다음 프로세스를 구현해야 합니다:

- 전체 기술 스택의 보안을 구성하는 체계적인 접근 방식입니다.
- 플랫폼의 구성 및 사용에 대한 지속적이고 엄격한 검토입니다.

자체 관리 러너에서 GitLab CI/CD 을 실행할 계획이 있다면 컴퓨팅 인프라와 네트워크에 보안 위험이 존재합니다.

는 CI/CD 에 정의된 코드를 실행합니다. 프로젝트의 리포지토리에 대해 개발자 역할을 가진 모든 사용자는 의도적이든 아니든 러너를 호스팅하는 환경의 보안을 손상시킬 수 있습니다.

자체 관리 러너가 임시 저장이 아니며 여러 프로젝트에 사용되는 경우 이 위험은 훨씬 더 심각합니다.

- 악의적 코드가 포함된 리포지토리의 은 임시 저장이 아닌 러너가 서비스하는 다른 리포지토리의 보안을 손상시킬 수 있습니다.
- 실행기에 따라 은 러너가 호스팅되는 가상 머신에 악의적 코드를 설치할 수 있습니다.
- 손상된 환경에서 실행 중인 에 노출된 비밀 변수는 `CI_JOB_TOKEN`을(를) 포함하되 이에 국한되지 않는 도용될 수 있습니다.
- 개발자 역할을 가진 사용자는 프로젝트와 관련된 부분 모듈에 액세스할 수 있습니다. 부분 모듈의 업스트림 프로젝트에 액세스할 수 없는 경우에도 마찬가지입니다.

## 다양한 실행기의 보안 위험 {#security-risks-for-different-executors}

사용 중인 실행기에 따라 다양한 보안 위험에 직면할 수 있습니다.

### Shell 실행기의 사용 {#usage-of-shell-executor}

**`shell` 실행기로 빌드를 실행할 때 러너 호스트 및 네트워크에 높은 보안 위험이 존재합니다**. 은 GitLab Runner 사용자의 권한으로 실행되며 이 서버에서 실행되는 다른 프로젝트의 코드를 도용할 수 있습니다. 신뢰할 수 있는 빌드를 실행할 때만 사용하세요.

### Docker 실행기의 사용 {#usage-of-docker-executor}

**Docker can be considered safe when running in non-privileged mode**. 이러한 구성을 더 안전하게 하려면 비활성화된 `sudo`, 삭제된 `SETUID` 및 `SETGID` 기능을 갖춘 Docker 컨테이너에서 루트가 아닌 사용자로 을 실행하세요.

더 세밀한 권한은 `cap_add`/`cap_drop` 설정을 통해 권한이 없는 모드에서 구성할 수 있습니다.

> [!warning]
> Docker의 권한이 있는 컨테이너는 호스트 VM의 모든 루트 기능을 갖습니다. 자세한 내용은 [런타임 권한 및 Linux 기능](https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities)에 대한 공식 Docker 설명서를 참조하세요.

권한이 있는 모드에서 컨테이너를 실행하는 것은 **not advised**.

권한이 있는 모드를 사용하면 CI/CD 을 실행하는 사용자는 러너의 호스트 시스템에 대한 완전한 루트 액세스, 볼륨을 마운트 및 분리할 수 있는 권한, 중첩된 컨테이너를 실행할 수 있습니다.

권한이 있는 모드를 활성화하면 컨테이너의 모든 보안 메커니즘을 비활성화하고 호스트를 권한 에스컬레이션에 노출하게 되며, 이는 컨테이너 중단으로 이어질 수 있습니다.

Docker Machine 실행기를 사용하는 경우 `MaxBuilds = 1` 설정을 사용할 것을 강력히 권장합니다. 이 설정은 권한이 있는 모드로 인한 보안 약점 때문에 잠재적으로 손상될 수 있는 단일 자동 확장 VM이 정확히 하나의 을 처리하는 데 사용되도록 합니다.

### `if-not-present` 풀 정책을 사용하는 프라이빗 Docker 이미지의 사용 {#usage-of-private-docker-images-with-if-not-present-pull-policy}

[고급 구성: 프라이빗 컨테이너 레지스트리 사용](../configuration/advanced-configuration.md#use-a-private-container-registry)에서 설명하는 프라이빗 Docker 이미지 지원을 사용할 때 `always`을(를) `pull_policy` 값으로 사용해야 합니다. 특히 Docker 또는 Kubernetes 실행기를 사용하여 공용 인스턴스 러너를 호스팅하는 경우 `always` 풀 정책을 사용해야 합니다.

풀 정책이 `if-not-present`로 설정된 예제를 고려해 보겠습니다:

1. 사용자 A는 `registry.example.com/image/name`에 프라이빗 이미지를 가지고 있습니다.
1. 사용자 A가 인스턴스 러너에서 빌드를 시작합니다:  빌드는 레지스트리 자격 증명을 수신하고 레지스트리에서 인증 후 이미지를 끌어옵니다.
1. 이미지는 인스턴스 러너의 호스트에 저장됩니다.
1. 사용자 B는 `registry.example.com/image/name`의 프라이빗 이미지에 액세스할 수 없습니다.
1. 사용자 B는 사용자 A와 동일한 인스턴스 러너에서 이 이미지를 사용하는 빌드를 시작합니다:  는 로컬 버전의 이미지를 찾고 **even if the image could not be pulled because of missing credentials**.

따라서 다양한 사용자 및 다양한 프로젝트(혼합된 프라이빗 및 공용 액세스 수준 포함)에서 사용할 수 있는 러너를 호스팅하는 경우 `if-not-present`을(를) 풀 정책 값으로 사용하지 않아야 하며 대신 다음을 사용해야 합니다:

- `never` - 사용자가 미리 다운로드한 이미지만 사용하도록 제한하려는 경우입니다.
- `always` - 사용자에게 모든 레지스트리에서 이미지를 다운로드할 수 있는 가능성을 제공하려는 경우입니다.

`if-not-present` 풀 정책은 신뢰할 수 있는 빌드 및 사용자가 사용하는 특정 러너에 대해 **only** 사용해야 합니다.

자세한 내용은 [풀 정책 설명서](../executors/docker.md#configure-how-runners-pull-images)를 읽어보세요.

### SSH 실행기의 사용 {#usage-of-ssh-executor}

**SSH executors are susceptible to MITM attack (man-in-the-middle)**. 누락된 `StrictHostKeyChecking` 옵션 때문입니다. 이는 향후 릴리스 중 하나에서 수정될 예정입니다.

### Parallels 실행기의 사용 {#usage-of-parallels-executor}

**Parallels executor is the safest possible option**. 전체 시스템 가상화를 사용하고 격리 모드에서 실행하도록 구성된 VM 머신을 사용하고 격리 모드에서 실행하도록 구성된 VM 머신을 사용하기 때문입니다. 모든 주변 기기 및 공유 폴더에 대한 액세스를 차단합니다.

## 러너 복제 {#cloning-a-runner}

는 토큰을 사용하여 GitLab 서버에 식별합니다. 러너를 복제하면 복제된 러너가 해당 토큰에 대해 동일한 을 가져올 수 있습니다. 이는 러너 을 "도용"하기 위한 가능한 공격 벡터입니다.

## 공유 환경에서 `GIT_STRATEGY: fetch`을(를) 사용할 때의 보안 위험 {#security-risks-when-using-git_strategy-fetch-on-shared-environments}

[`GIT_STRATEGY`](https://docs.gitlab.com/ci/runners/configure_runners/#git-strategy)을(를) `fetch`로 설정하면 는 Git 리포지토리의 로컬 작업 복사본을 재사용하려고 합니다.

로컬 복사본을 사용하면 CI/CD 의 성능을 향상시킬 수 있습니다. 그러나 재사용 가능한 복사본에 액세스할 수 있는 모든 사용자는 다른 사용자의 파이프라인에서 실행되는 코드를 추가할 수 있습니다.

Git은 부분 모듈(다른 리포지토리 내에 포함된 리포지토리)의 내용을 상위 리포지토리의 Git reflog에 저장합니다. 따라서 프로젝트의 부분 모듈이 처음 복제된 후 후속 은 스크립트에서 `git submodule update`을(를) 실행하여 부분 모듈의 내용에 액세스할 수 있습니다. 이는 부분 모듈이 삭제되었고 을 시작한 사용자가 부분 모듈 프로젝트에 액세스할 수 없는 경우에도 적용됩니다.

공유 환경에 액세스할 수 있는 모든 사용자를 신뢰할 때만 `GIT_STRATEGY: fetch`을(를) 사용하세요.

## 보안 강화 옵션 {#security-hardening-options}

### 권한이 있는 컨테이너 사용의 보안 위험 감소 {#reduce-the-security-risk-of-using-privileged-containers}

Docker의 `--privileged` 플래그 사용이 필요한 CI/CD 을 실행해야 하는 경우 이러한 단계를 취하여 보안 위험을 줄일 수 있습니다:

- `--privileged` 플래그를 사용하여 Docker 컨테이너를 격리되고 일시적인 가상 머신에서만 실행하세요.
- Docker의 `--privileged` 플래그 사용이 필요한 을 실행하기 위한 전용 러너를 구성합니다. 그런 다음 이러한 러너를 보호된 브랜치에서만 을 실행하도록 구성합니다.

### 네트워크 분할 {#network-segmentation}

GitLab Runner는 사용자 제어 스크립트를 실행하도록 설계되었습니다. 이 악의적인 경우 공격 표면을 줄이려면 자신의 네트워크 세그먼트에서 실행하는 것을 고려할 수 있습니다. 이것은 다른 인프라 및 서비스로부터 네트워크를 분리할 것입니다.

모든 요구 사항이 고유하지만 클라우드 환경의 경우 다음을 포함할 수 있습니다:

- 자신의 네트워크 세그먼트에서 러너 가상 머신 구성
- 인터넷에서 러너 가상 머신으로의 SSH 액세스 차단
- 러너 가상 머신 간 트래픽 제한
- 클라우드 공급자 메타데이터 엔드포인트에 대한 액세스 필터링

> [!note]
> 모든 러너는 GitLab.com 또는 GitLab 인스턴스에 대한 아웃바운드 네트워크 연결이 필요합니다. 대부분의 은 종속성 당겨오기 등을 위해 인터넷으로의 아웃바운드 네트워크 연결도 필요합니다.

### 러너 호스트 보호 {#secure-the-runner-host}

베어 메탈이든 가상 머신이든 러너에 대해 정적 호스트를 사용하는 경우 호스트 운영 체제에 대한 보안 모범 사례를 구현해야 합니다.

CI 의 컨텍스트에서 실행되는 악의적 코드는 호스트를 손상시킬 수 있으므로 보안 프로토콜은 영향을 완화할 수 있습니다. 기억해야 할 다른 사항으로는 호스트 시스템에서 SSH 키와 같은 파일을 보호하거나 제거하여 공격자가 환경의 다른 엔드포인트에 액세스할 수 있도록 할 수 있는 파일을 포함합니다.

### 각 빌드 후 `.git` 폴더 정리 {#clean-up-the-git-folder-after-each-build}

정적 호스트를 러너에 사용하는 경우 `FF_ENABLE_JOB_CLEANUP` [기능 플래그](../configuration/feature-flags.md)를 활성화하여 추가 보안 계층을 구현할 수 있습니다.

`FF_ENABLE_JOB_CLEANUP`을(를) 활성화하면 러너가 호스트에서 사용하는 빌드 디렉토리가 각 빌드 후 정리됩니다.
