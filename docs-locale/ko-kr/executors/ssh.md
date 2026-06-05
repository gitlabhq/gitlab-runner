---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: SSH
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!note]
> 이 실행기는 유지보수 모드입니다. 중요한 보안 업데이트는 제공되지만 새로운 기능은 계획되지 않았습니다. 새로운 프로젝트의 경우 [활발히 개발 중인 실행기](_index.md#selecting-the-executor) 중 하나를 사용하는 것을 권장합니다.

SSH 실행기는 완전성을 위해 포함되어 있지만 가장 지원이 적은 실행기 중 하나입니다. GitLab 러너는 외부 서버에 연결하고 SSH를 통해 빌드를 실행합니다. 일부 조직에서 이 실행기를 성공적으로 사용하지만, 일반적으로 다른 실행기 유형을 사용하는 것이 더 좋습니다.

> [!note]
> SSH 실행기는 Bash에서 생성된 스크립트만 지원하며 캐싱 기능은 지원되지 않습니다.

이 실행기를 사용하면 SSH를 통해 명령을 실행하여 원격 머신에서 빌드를 실행할 수 있습니다.

> [!note]
> GitLab 러너가 SSH 실행기를 사용하는 모든 원격 시스템에서 [공통 필수 요구 사항](_index.md#git-requirements-for-non-docker-executors)을 충족하는지 확인합니다.

## SSH 실행기 사용 {#use-the-ssh-executor}

SSH 실행기를 사용하려면 [`[runners.ssh]` 섹션](../configuration/advanced-configuration.md#the-runnersssh-section)에서 `executor = "ssh"`을 지정합니다. 예를 들어:

```toml
[[runners]]
  executor = "ssh"
  [runners.ssh]
    host = "example.com"
    port = "22"
    user = "root"
    password = "password"
    identity_file = "/path/to/identity/file"
```

서버에 인증하기 위해 `password` 또는 `identity_file` 또는 둘 다를 사용할 수 있습니다. GitLab 러너는 `identity_file`를 `/home/user/.ssh/id_(rsa|dsa|ecdsa)`에서 암시적으로 읽지 않습니다. `identity_file`는 명시적으로 지정해야 합니다.

프로젝트 소스가 `~/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`로 체크아웃됩니다.

여기서:

- `<short-token>`는 러너의 토큰의 단축 버전입니다(처음 8자)
- `<concurrent-id>`는 같은 프로젝트의 빌드를 동시에 실행하는 모든 러너의 목록에서 러너의 인덱스입니다(`CI_CONCURRENT_PROJECT_ID` [미리 정의된 변수](https://docs.gitlab.com/ci/variables/predefined_variables/)를 통해 액세스 가능).
- `<namespace>`는 네임스페이스이며 GitLab에 프로젝트가 저장됩니다
- `<project-name>`은(는) GitLab에 저장된 프로젝트의 이름입니다

`~/builds` 디렉토리를 덮어쓰려면 `[[runners]]` 섹션의 [`config.toml`](../configuration/advanced-configuration.md)에서 `builds_dir` 옵션을 지정합니다.

작업 아티팩트를 업로드하려면 SSH를 통해 연결하는 호스트에 `gitlab-runner`을 설치합니다.

## 엄격한 호스트 키 확인 구성 {#configure-strict-host-key-checking}

SSH `StrictHostKeyChecking`은 기본적으로 [활성화](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28192)됩니다. SSH `StrictHostKeyChecking`을 비활성화하려면 `[runners.ssh.disable_strict_host_key_checking]`을 `true`로 설정합니다. 현재 기본값은 `false`입니다.
