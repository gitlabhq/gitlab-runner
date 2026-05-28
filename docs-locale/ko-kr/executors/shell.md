---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Shell 실행기
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!note]
> 이 실행기는 유지 보수 모드입니다. 중요한 보안 업데이트는 받지만 새로운 기능은 계획되지 않습니다. 새 프로젝트의 경우 [활발하게 개발 중인 실행기](_index.md#selecting-the-executor) 중 하나를 사용하는 것을 고려하세요.

Shell 실행기는 GitLab 러너를 위한 가장 간단한 실행기 구성입니다. GitLab 러너가 설치된 머신에서 로컬로 빌드를 실행하므로 모든 종속 항목을 동일한 머신에 설치해야 합니다. 러너를 설치할 수 있는 모든 시스템을 지원합니다. 이는 Bash, PowerShell Core, Windows PowerShell 및 Windows Batch(더 이상 사용되지 않음)용으로 생성된 스크립트를 사용할 수 있음을 의미합니다.

최소 종속 항목으로 빌드에 이상적이지만 Shell 실행기는 작업 간 제한된 격리를 제공합니다.

> [!note]
> GitLab 러너가 셸 실행기를 사용하는 머신에서 [일반적인 필수 조건](_index.md#git-requirements-for-non-docker-executors)을 충족하는지 확인하세요.

## 권한이 있는 사용자로 스크립트 실행 {#run-scripts-as-a-privileged-user}

`--user`가 [`gitlab-runner run` 명령](../commands/_index.md#gitlab-runner-run)에 추가되면 권한이 없는 사용자로 스크립트를 실행할 수 있습니다. 이 기능은 Bash에서만 지원됩니다.

소스 프로젝트는 `<working-directory>/builds/<short-token>/<concurrent-id>/<namespace>/<project-name>`로 확인됩니다.

프로젝트의 캐시는 `<working-directory>/cache/<namespace>/<project-name>`에 저장됩니다.

여기서:

- `<working-directory>`은(는) `--working-directory`의 값으로 `gitlab-runner run` 명령에 전달되거나 러너가 실행 중인 현재 디렉터리입니다.
- `<short-token>`은(는) 러너의 토큰의 축약된 버전입니다(처음 8자).
- `<concurrent-id>`은(는) 동일한 프로젝트에 대해 동시에 빌드를 실행하는 모든 러너의 목록에서 러너의 인덱스입니다(`CI_CONCURRENT_PROJECT_ID` [사전 정의된 변수](https://docs.gitlab.com/ci/variables/predefined_variables/)를 통해 액세스 가능).
- `<namespace>`은(는) GitLab에 프로젝트가 저장된 네임스페이스입니다.
- `<project-name>`은(는) GitLab에 저장된 프로젝트의 이름입니다.

`<working-directory>/builds`과(와) `<working-directory/cache`를 덮어쓰려면 [`config.toml`](../configuration/advanced-configuration.md)의 `[[runners]]` 섹션 아래에 `builds_dir` 및 `cache_dir` 옵션을 지정하세요.

## 권한이 없는 사용자로 스크립트 실행 {#run-scripts-as-an-unprivileged-user}

GitLab 러너가 [공식 `.deb` 또는 `.rpm` 패키지](https://packages.gitlab.com/runner/gitlab-runner)에서 Linux에 설치된 경우 설치 관리자는 발견된 경우 `gitlab_ci_multi_runner` 사용자를 사용하려고 합니다. 설치 관리자가 `gitlab_ci_multi_runner` 사용자를 찾을 수 없으면 `gitlab-runner` 사용자를 생성하여 대신 사용합니다.

모든 셸 빌드는 `gitlab-runner` 또는 `gitlab_ci_multi_runner` 사용자로 실행됩니다.

일부 테스트 시나리오에서는 Docker Engine 또는 VirtualBox와 같은 일부 권한이 있는 리소스에 액세스해야 할 수 있습니다. 이 경우 `gitlab-runner` 사용자를 해당 그룹에 추가해야 합니다:

```shell
usermod -aG docker gitlab-runner
usermod -aG vboxusers gitlab-runner
```

## 셸 선택 {#selecting-your-shell}

GitLab 러너는 [특정 셸을 지원합니다](../shells/_index.md). 셸을 선택하려면 `config.toml` 파일에서 지정하세요. 예를 들어:

```toml
...
[[runners]]
  name = "shell executor runner"
  executor = "shell"
  shell = "powershell"
...
```

## 보안 {#security}

일반적으로 셸 실행기로 작업을 실행하는 것은 안전하지 않습니다. 작업은 사용자의 권한(`gitlab-runner`)으로 실행되며 이 서버에서 실행되는 다른 프로젝트에서 코드를 "도용"할 수 있습니다. 구성에 따라 작업은 매우 권한이 있는 사용자로 서버에서 임의의 명령을 실행할 수 있습니다. 신뢰하는 사용자의 빌드를 실행하고 신뢰하고 소유하는 서버에서만 사용하세요.

## 프로세스 종료 및 중지 {#terminating-and-killing-processes}

셸 실행기는 각 작업에 대한 스크립트를 새 프로세스에서 시작합니다. UNIX 시스템에서는 주 프로세스를 프로세스 그룹으로 설정합니다.

GitLab 러너는 다음 경우에 프로세스를 종료합니다:

- 작업이 [시간 초과](https://docs.gitlab.com/ci/pipelines/settings/#set-a-limit-for-how-long-jobs-can-run)됩니다.
- 작업이 취소됩니다.

UNIX 시스템에서 `gitlab-runner`은(는) 프로세스 및 해당 자식 프로세스에 `SIGTERM`를 전송하고 10분 후 `SIGKILL`을(를) 전송합니다. 이는 프로세스의 정상적인 종료를 허용합니다. Windows는 `SIGTERM` 동등한 기능이 없으므로 종료 신호가 두 번 전송됩니다. 두 번째는 10분 후에 전송됩니다.
