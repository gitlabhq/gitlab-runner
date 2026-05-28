---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Fleeting
---

[Fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting)은 GitLab Runner가 클라우드 공급자의 인스턴스 그룹에 대한 플러그인 기반 추상화를 제공하는 데 사용하는 라이브러리입니다.

다음 실행기는 fleeting을 사용하여 러너를 확장합니다:

- [Docker Autoscaler](../executors/docker_autoscaler.md)
- [Instance](../executors/instance.md)

## fleeting 플러그인 찾기 {#find-a-fleeting-plugin}

GitLab은 이러한 공식 플러그인을 유지합니다:

| 클라우드 공급자                                                             | 참고 |
|----------------------------------------------------------------------------|-------|
| [Google Cloud](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud) | [Google Cloud 인스턴스 그룹](https://docs.cloud.google.com/compute/docs/instance-groups)을 사용합니다 |
| [AWS](https://gitlab.com/gitlab-org/fleeting/plugins/aws)                  | [AWS Auto Scaling 그룹](https://docs.aws.amazon.com/autoscaling/ec2/userguide/auto-scaling-groups.html)을 사용합니다 |
| [Azure](https://gitlab.com/gitlab-org/fleeting/plugins/azure)              | Azure [Virtual Machine Scale Sets](https://learn.microsoft.com/en-us/azure/virtual-machine-scale-sets/overview)를 사용합니다. [Uniform orchestration](https://learn.microsoft.com/en-us/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-orchestration-modes#scale-sets-with-uniform-orchestration) 모드만 지원합니다. |

다음 플러그인은 커뮤니티에서 유지 관리합니다:

| 클라우드 공급자 | OCI 참조 | 참고 |
|----------------|---------------|-------|
| [VMware vSphere](https://gitlab.com/santhanuv/fleeting-plugin-vmware-vsphere) | `registry.gitlab.com/santhanuv/fleeting-plugin-vmware-vsphere:latest` | 기존 템플릿을 복제하여 가상 머신을 생성하고 관리하는 데 VMware vSphere를 사용합니다. [`govmomi vcsim`](https://github.com/vmware/govmomi/tree/main/vcsim) 시뮬레이터로 테스트하고 커뮤니티 멤버에 의해 기본 사용 사례에 대해 검증했습니다. 제한된 vSphere 권한으로 인한 제한 사항이 있을 수 있습니다. [Fleeting Plugin VMware vSphere 프로젝트](https://gitlab.com/santhanuv/fleeting-plugin-vmware-vsphere/-/issues)에서 관련 이슈를 생성할 수 있습니다.|

커뮤니티 유지 관리 플러그인은 GitLab 외부(커뮤니티)의 기여자가 소유, 빌드, 호스트 및 유지 관리합니다. GitLab은 Fleeting 라이브러리 및 API를 소유하고 유지 관리하여 정적 코드 검토를 제공합니다. GitLab은 모든 필요한 컴퓨팅 환경에 액세스할 수 없기 때문에 커뮤니티 플러그인을 테스트할 수 없습니다. 커뮤니티 멤버는 플러그인을 빌드하고 테스트한 후 OCI 리포지토리에 게시하고 머지 리퀘스트를 통해 이 페이지에서 참조를 제공해야 합니다. OCI 참조에는 이슈를 보고할 위치, 플러그인의 지원 및 안정성 수준, 그리고 설명서를 찾을 위치에 대한 참고 사항이 포함되어야 합니다.

## fleeting 플러그인 구성 {#configure-a-fleeting-plugin}

fleeting을 구성하려면 `config.toml`에서 [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section) 구성 섹션을 사용합니다.

> [!note]
> 각 플러그인에 대한 README.md 파일에는 설치 및 구성에 관한 중요한 정보가 포함되어 있습니다.

## fleeting 플러그인 설치 {#install-a-fleeting-plugin}

fleeting 플러그인을 설치하려면 다음 중 하나를 사용합니다:

- OCI 레지스트리 배포(권장)
- 수동 바이너리 설치

## OCI 레지스트리 배포로 설치 {#install-with-the-oci-registry-distribution}

{{< history >}}

- GitLab Runner 16.11에서 [OCI 레지스트리 배포 도입](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4690)

{{< /history >}}

플러그인은 UNIX 시스템의 `~/.config/fleeting/plugins`에, Windows의 `%APPDATA%/fleeting/plugins`에 설치됩니다. 플러그인이 설치되는 위치를 재정의하려면 환경 변수 `FLEETING_PLUGIN_PATH`을 업데이트합니다.

fleeting 플러그인을 설치하려면:

1. `config.toml`의 `[runners.autoscaler]` 섹션에서 fleeting 플러그인을 추가합니다:

   {{< tabs >}}

   {{< tab title="AWS" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "aws:latest"
   ```

   {{< /tab >}}

   {{< tab title="Google Cloud" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "googlecloud:latest"
   ```

   {{< /tab >}}

   {{< tab title="Azure" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "azure:latest"
   ```

   {{< /tab >}}

   {{< /tabs >}}

1. `gitlab-runner fleeting install`을 실행합니다.

### `plugin` 형식 {#plugin-formats}

`plugin` 매개변수는 다음 형식을 지원합니다:

- `<name>`
- `<name>:<version constraint>`
- `<repository>/<name>`
- `<repository>/<name>:<version constraint>`
- `<registry>/<repository>/<name>`
- `<registry>/<repository>/<name>:<version constraint>`

다음을 참조하세요:

- `registry.gitlab.com`은 기본 레지스트리입니다.
- `gitlab-org/fleeting/plugins`은 기본 리포지토리입니다.
- `latest`은 기본 버전입니다.

### 버전 제약 형식 {#version-constraint-formats}

`gitlab-runner fleeting install` 명령은 버전 제약을 사용하여 원격 리포지토리에서 최신 일치 버전을 찾습니다.

GitLab Runner가 실행될 때 버전 제약을 사용하여 로컬에 설치된 최신 일치 버전을 찾습니다.

다음 버전 제약 형식을 사용합니다:

| 형식                    | 설명 |
|---------------------------|-------------|
| `latest`                  | 최신 버전입니다. |
| `<MAJOR>`                 | 메이저 버전을 선택합니다. 예를 들어 `1`은 `1.*.*`과(와) 일치하는 버전을 선택합니다. |
| `<MAJOR>.<MINOR>`         | 메이저 및 마이너 버전을 선택합니다. 예를 들어 `1.5`은 `1.5.*`과(와) 일치하는 최신 버전을 선택합니다. |
| `<MAJOR>.<MINOR>.<PATCH>` | 메이저, 마이너 버전 및 패치를 선택합니다. 예를 들어 `1.5.1`은 `1.5.1` 버전을 선택합니다. |

## 바이너리 수동 설치 {#install-binary-manually}

fleeting 플러그인을 수동으로 설치하려면:

1. 시스템용 fleeting 플러그인 바이너리를 다운로드합니다:
   - [AWS](https://gitlab.com/gitlab-org/fleeting/plugins/aws/-/releases).
   - [Google Cloud](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud/-/releases)
   - [Azure](https://gitlab.com/gitlab-org/fleeting/plugins/azure/-/releases)
1. 바이너리의 이름이 `fleeting-plugin-<name>` 형식인지 확인합니다. 예를 들어, `fleeting-plugin-aws`.
1. 바이너리를 `$PATH`에서 검색할 수 있는지 확인합니다. 예를 들어 `/usr/local/bin`로 이동합니다.
1. `config.toml`의 `[runners.autoscaler]` 섹션에서 fleeting 플러그인을 추가합니다. 예를 들어:

   {{< tabs >}}

   {{< tab title="AWS" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "fleeting-plugin-aws"
   ```

   {{< /tab >}}

   {{< tab title="Google Cloud" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "fleeting-plugin-googlecloud"
   ```

   {{< /tab >}}

   {{< tab title="Azure" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "fleeting-plugin-azure"
   ```

   {{< /tab >}}

   {{< /tabs >}}

## Fleeting 플러그인 관리 {#fleeting-plugin-management}

다음 `fleeting` 하위 명령을 사용하여 fleeting 플러그인을 관리합니다:

| 명령                          | 설명 |
|----------------------------------|-------------|
| `gitlab-runner fleeting install` | OCI 레지스트리 배포에서 fleeting 플러그인을 설치합니다. |
| `gitlab-runner fleeting list`    | 참조된 플러그인 및 사용된 버전을 나열합니다. |
| `gitlab-runner fleeting login`   | 전용 레지스트리에 로그인합니다. |
