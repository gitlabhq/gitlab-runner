---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 사용자 정의 실행기
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!note]
> 이 실행기는 유지 보수 모드입니다. 중요한 보안 업데이트는 받지만 새로운 기능은 계획되지 않습니다. 새로운 프로젝트의 경우 [활발하게 개발 중인 실행기](_index.md#selecting-the-executor) 중 하나를 사용하는 것을 고려하세요.

사용자 정의 실행기를 사용하여 자신의 실행 환경을 지정할 수 있습니다. GitLab 러너가 기본적으로 `LXD` 또는 `Libvirt` 같은 실행기를 지원하지 않는 경우, 사용자 정의 실행 파일을 사용하도록 GitLab 러너를 구성하여 환경을 프로비저닝, 실행 및 정리할 수 있습니다.

사용자 정의 실행기에 대해 구성하는 스크립트를 `Drivers`라고 합니다. 예를 들어 [`LXD` 드라이버](custom_examples/lxd.md) 또는 [`Libvirt` 드라이버](custom_examples/libvirt.md)를 만들 수 있습니다.

## 구성 {#configuration}

여러 구성 키 중에서 선택할 수 있습니다. 일부는 선택 사항입니다.

다음은 사용 가능한 모든 구성 키를 사용하여 사용자 정의 실행기에 대한 구성의 예입니다:

```toml
[[runners]]
  name = "custom"
  url = "https://gitlab.com"
  token = "TOKEN"
  executor = "custom"
  builds_dir = "/builds"
  cache_dir = "/cache"
  shell = "bash"
  [runners.custom]
    config_exec = "/path/to/config.sh"
    config_args = [ "SomeArg" ]
    config_exec_timeout = 200

    prepare_exec = "/path/to/script.sh"
    prepare_args = [ "SomeArg" ]
    prepare_exec_timeout = 200

    run_exec = "/path/to/binary"
    run_args = [ "SomeArg" ]

    cleanup_exec = "/path/to/executable"
    cleanup_args = [ "SomeArg" ]
    cleanup_exec_timeout = 200

    graceful_kill_timeout = 200
    force_kill_timeout = 200
```

필드 정의 및 필수 필드를 확인하려면 [`[runners.custom]` 섹션](../configuration/advanced-configuration.md#the-runnerscustom-section) 구성을 참조하세요.

또한 [`[[runners]]`](../configuration/advanced-configuration.md#the-runners-section) 내의 `builds_dir`과 `cache_dir`는 필수 필드입니다.

## 작업 실행을 위한 필수 소프트웨어 {#prerequisite-software-for-running-a-job}

사용자는 `PATH`에 있어야 하는 다음을 포함하여 환경을 설정해야 합니다:

- [Git](https://git-scm.com/download) 및 [Git LFS](https://git-lfs.com/) : [일반 필수 사항](_index.md#git-requirements-for-non-docker-executors)을 참조하세요.
- [GitLab 러너](../install/_index.md):  아티팩트 및 캐시를 다운로드/업데이트하는 데 사용됩니다.

## 스테이지 {#stages}

사용자 정의 실행기는 작업 세부 사항을 구성하고 환경을 준비 및 정리하며 작업 스크립트를 실행할 스테이지를 제공합니다. 각 스테이지는 특정 작업을 담당하며 고려해야 할 다양한 사항이 있습니다.

사용자 정의 실행기에서 실행한 각 스테이지는 기본 제공 GitLab 러너 실행기가 실행하는 시간에 실행됩니다.

실행된 각 단계는 실행 중인 작업에 대한 정보를 제공하는 특정 환경 변수에 액세스할 수 있습니다. 모든 스테이지에서 다음 환경 변수를 사용할 수 있습니다:

- 표준 CI/CD [환경 변수](https://docs.gitlab.com/ci/variables/) , [사전 정의 변수](https://docs.gitlab.com/ci/variables/predefined_variables/) 포함.
- 사용자 정의 실행기 러너 호스트 시스템에서 제공하는 모든 환경 변수.
- 모든 서비스 및 해당 [사용 가능한 설정](https://docs.gitlab.com/ci/services/#available-settings-for-services). JSON 형식으로 `CUSTOM_ENV_CI_JOB_SERVICES`로 노출됩니다.

CI/CD 환경 변수와 사전 정의 변수는 모두 시스템 환경 변수와의 충돌을 방지하기 위해 `CUSTOM_ENV_`로 접두사가 붙습니다. 예를 들어 `CI_BUILDS_DIR`는 `CUSTOM_ENV_CI_BUILDS_DIR`로 사용할 수 있습니다.

스테이지는 다음 순서로 실행됩니다:

1. `config_exec`
1. `prepare_exec`
1. `run_exec`
1. `cleanup_exec`

### 서비스 {#services}

[서비스](https://docs.gitlab.com/ci/services/)는 JSON 배열로 `CUSTOM_ENV_CI_JOB_SERVICES`로 노출됩니다.

예:

```yaml
custom:
  script:
    - echo $CUSTOM_ENV_CI_JOB_SERVICES
  services:
    - redis:latest
    - name: my-postgres:9.4
      alias: pg
      entrypoint: ["path", "to", "entrypoint"]
      command: ["path", "to", "cmd"]
      variables:
        POSTGRES_PASSWORD: secret
        POSTGRES_DB: mydb
```

위의 예는 `CUSTOM_ENV_CI_JOB_SERVICES` 환경 변수를 다음 값으로 설정합니다:

```json
[{"name":"redis:latest","alias":"","entrypoint":null,"command":null},{"name":"my-postgres:9.4","alias":"pg","entrypoint":["path","to","entrypoint"],"command":["path","to","cmd"],"variables":{"POSTGRES_DB":"mydb","POSTGRES_PASSWORD":"secret"}}]
```

JSON 배열의 각 서비스 객체에는 다음 필드가 있습니다:

| 필드        | 유형          | 설명                                                                              |
|--------------|---------------|------------------------------------------------------------------------------------------|
| `name`       | 문자열        | 서비스 이미지 이름.                                                                      |
| `alias`      | 문자열        | 서비스에 정의된 첫 번째 별칭. 없으면 빈 문자열입니다.                               |
| `entrypoint` | 배열 또는 null | 컨테이너 진입점 재정의. 설정되지 않은 경우 `null`.                                        |
| `command`    | 배열 또는 null | 컨테이너 명령 재정의. 설정되지 않은 경우 `null`.                                           |
| `variables`  | 객체        | 서비스에 정의된 변수의 키-값 맵. 변수가 정의되지 않은 경우 생략됩니다. |

### 구성 {#config}

구성 스테이지는 `config_exec`에서 실행됩니다.

경우에 따라 실행 시간에 일부 설정을 지정할 수 있습니다. 예를 들어 프로젝트 ID에 따라 빌드 디렉토리를 설정합니다. `config_exec`은 STDOUT에서 읽고 특정 키가 있는 유효한 JSON 문자열을 기대합니다.

예를 들어:

```shell
#!/usr/bin/env bash

cat << EOS
{
  "builds_dir": "/builds/${CUSTOM_ENV_CI_CONCURRENT_PROJECT_ID}/${CUSTOM_ENV_CI_PROJECT_PATH_SLUG}",
  "cache_dir": "/cache/${CUSTOM_ENV_CI_CONCURRENT_PROJECT_ID}/${CUSTOM_ENV_CI_PROJECT_PATH_SLUG}",
  "builds_dir_is_shared": true,
  "hostname": "custom-hostname",
  "driver": {
    "name": "test driver",
    "version": "v0.0.1"
  },
  "job_env" : {
    "CUSTOM_ENVIRONMENT": "example"
  },
  "shell": "bash"
}
EOS
```

JSON 문자열 내의 추가 키는 무시됩니다. 유효한 JSON 문자열이 아닌 경우 스테이지가 실패하고 2회 더 재시도합니다.

| 매개 변수              | 유형    | 필수 | 허용되는 빈 값  | 설명 |
|------------------------|---------|----------|----------------|-------------|
| `builds_dir`           | 문자열  | ✗        | ✗              | 작업의 작업 디렉토리가 생성되는 기본 디렉토리입니다. |
| `cache_dir`            | 문자열  | ✗        | ✗              | 로컬 캐시가 저장되는 기본 디렉토리입니다. |
| `builds_dir_is_shared` | 부울 | ✗        | 해당 없음 | 환경이 동시 작업 간에 공유되는지 여부를 정의합니다. |
| `hostname`             | 문자열  | ✗        | ✓              | 러너에서 저장된 작업의 "메타데이터"와 연결할 호스트 이름입니다. 정의되지 않은 경우 호스트 이름은 설정되지 않습니다. |
| `driver.name`          | 문자열  | ✗        | ✓              | 드라이버의 사용자 정의 이름입니다. `Using custom executor...` 줄과 함께 인쇄됩니다. 정의되지 않은 경우 드라이버에 대한 정보가 인쇄되지 않습니다. |
| `driver.version`       | 문자열  | ✗        | ✓              | 드라이브의 사용자 정의 버전입니다. `Using custom executor...` 줄과 함께 인쇄됩니다. 정의되지 않은 경우 이름 정보만 인쇄됩니다. |
| `job_env`              | 객체  | ✗        | ✓              | 작업 실행의 모든 후속 스테이지에서 환경 변수를 통해 사용할 수 있는 이름-값 쌍입니다. 이들은 작업이 아닌 드라이버에서 사용할 수 있습니다. 자세한 내용은 [`job_env` 사용](#job_env-usage)을 참조하세요. |
| `shell`                | 문자열  | ✗        | ✓              | 작업 스크립트를 실행하는 데 사용되는 셸입니다. |

실행 파일의 `STDERR`이 작업 로그에 인쇄됩니다.

[`config_exec_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)를 구성하여 GitLab 러너가 JSON 문자열을 반환하기 전에 대기해야 하는 기간에 대한 기한을 설정할 수 있습니다.

[`config_args`](../configuration/advanced-configuration.md#the-runnerscustom-section)를 정의하는 경우, 정의한 순서대로 `config_exec` 실행 파일에 추가됩니다. 예를 들어 이 `config.toml` 콘텐츠로:

```toml
...
[runners.custom]
  ...
  config_exec = "/path/to/config"
  config_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab 러너는 `/path/to/config Arg1 Arg2`로 실행합니다.

#### `job_env` 사용 {#job_env-usage}

`job_env` 구성의 주요 목적은 작업 실행의 후속 스테이지를 위해 변수를 **to the context of custom executor driver calls** 전달하는 것입니다.

예를 들어 작업 실행 환경과의 연결에서 일부 자격 증명을 준비해야 하는 드라이버입니다. 이 작업은 비용이 많이 듭니다. 드라이버는 환경에 연결하기 전에 로컬 공급자로부터 임시 SSH 자격 증명을 요청해야 합니다.

사용자 정의 실행기 실행 흐름에서 각 작업 실행 [스테이지](#stages) (`prepare`, 여러 `run` 호출 및 `cleanup`)는 자체 컨텍스트를 갖는 별도의 실행으로 실행됩니다. 자격 증명 해결 예제의 경우 자격 증명 공급자에 대한 연결은 매번 수행되어야 합니다.

이 작업이 비용이 많이 드는 경우 전체 작업 실행에 대해 한 번 수행한 후 모든 작업 실행 스테이지에 대해 자격 증명을 재사용합니다. `job_env`는 여기서 도움이 될 수 있습니다. 이것을 사용하면 공급자와 한 번 연결하고 `config_exec` 호출 중에 받은 자격 증명을 `job_env`으로 전달할 수 있습니다. 다음으로 이들은 사용자 정의 실행기가 [`prepare_exec`](#prepare) , [`run_exec`](#run) 및 [`cleanup_exec`](#cleanup)에 대해 호출하는 변수 목록에 추가됩니다. 이렇게 하면 드라이버는 매번 자격 증명 공급자에 연결하는 대신 변수를 읽고 제공되는 자격 증명을 사용할 수 있습니다.

이해해야 할 중요한 점은 **the variables are not automatically available for the job itself** 것입니다. 이는 사용자 정의 실행기 드라이버가 구현되는 방식에 완전히 달려 있으며 많은 경우 거기에는 없습니다.

`job_env` 설정을 사용하여 특정 러너에서 실행되는 모든 작업에 변수 세트를 전달하는 방법에 대한 정보는 [`environment` 설정 from `[[runners]]`](../configuration/advanced-configuration.md#the-runners-section)을 참조하세요.

변수가 작업 간에 변경될 수 있는 값을 가진 동적인 경우 드라이버 구현이 `job_env`에서 전달된 변수를 실행 호출에 추가하는지 확인합니다.

### 준비 {#prepare}

준비 스테이지는 `prepare_exec`에서 실행됩니다.

이 시점에서 GitLab 러너는 작업에 대한 모든 것을 알고 있습니다 (실행되는 위치와 방법). 남은 것은 작업을 실행할 수 있도록 환경을 설정하는 것뿐입니다. GitLab 러너는 `prepare_exec`에 지정된 실행 파일을 실행합니다.

이 작업은 환경을 설정하는 것을 담당합니다 (예: 가상 머신 또는 컨테이너, 서비스 또는 기타 항목 생성). 이것이 완료된 후 환경이 작업을 실행할 준비가 되어 있을 것으로 예상합니다.

이 스테이지는 작업 실행에서 한 번만 실행됩니다.

[`prepare_exec_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)를 구성하여 GitLab 러너가 프로세스를 종료하기 전에 환경을 준비하기 위해 대기해야 하는 기간에 대한 기한을 설정할 수 있습니다.

이 실행 파일에서 반환된 `STDOUT` 및 `STDERR`는 작업 로그에 인쇄됩니다.

[`prepare_exec_args`](../configuration/advanced-configuration.md#the-runnerscustom-section)를 정의하는 경우, 정의한 순서대로 `prepare_exec` 실행 파일에 추가됩니다. 예를 들어 이 `config.toml` 콘텐츠로:

```toml
...
[runners.custom]
  ...
  prepare_exec = "/path/to/bin"
  prepare_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab 러너는 `/path/to/bin Arg1 Arg2`로 실행합니다.

### 실행 {#run}

실행 스테이지는 `run_exec`에서 실행됩니다.

이 실행 파일에서 반환된 `STDOUT` 및 `STDERR`는 작업 로그에 인쇄됩니다.

다른 스테이지와 달리 `run_exec` 스테이지는 여러 번 실행되는데, 아래에 나열된 하위 스테이지로 분할되기 때문입니다:

1. `prepare_script`
1. `get_sources`
1. `restore_cache`
1. `download_artifacts`
1. `step_*`
1. `build_script`
1. `step_*`
1. `after_script`
1. `archive_cache` 또는 `archive_cache_on_failure`
1. `upload_artifacts_on_success` 또는 `upload_artifacts_on_failure`
1. `cleanup_file_variables`

위에서 언급한 각 스테이지에 대해 `run_exec` 실행 파일이 다음으로 실행됩니다:

- 일반적인 환경 변수입니다.
- 두 개의 인수:
  - GitLab 러너가 사용자 정의 실행기를 실행하기 위해 만드는 스크립트의 경로입니다.
  - 스테이지의 이름입니다.

예를 들어:

```shell
/path/to/run_exec.sh /path/to/tmp/script1 prepare_executor
/path/to/run_exec.sh /path/to/tmp/script1 prepare_script
/path/to/run_exec.sh /path/to/tmp/script1 get_sources
```

`run_args`를 정의한 경우, 이들은 `run_exec` 실행 파일에 전달된 첫 번째 인수 세트이고 GitLab 러너가 다른 것을 추가합니다. 예를 들어 다음 `config.toml`이 있다고 가정합니다:

```toml
...
[runners.custom]
  ...
  run_exec = "/path/to/run_exec.sh"
  run_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab 러너는 다음 인수로 실행 파일을 실행합니다:

```shell
/path/to/run_exec.sh Arg1 Arg2 /path/to/tmp/script1 prepare_executor
/path/to/run_exec.sh Arg1 Arg2 /path/to/tmp/script1 prepare_script
/path/to/run_exec.sh Arg1 Arg2 /path/to/tmp/script1 get_sources
```

이 실행 파일은 첫 번째 인수에 지정된 스크립트를 실행하는 것을 담당해야 합니다. 이들은 GitLab 러너 실행기가 복제, 아티팩트 다운로드, 사용자 스크립트 실행 및 아래에 설명된 다른 모든 단계를 실행하는 모든 스크립트를 포함합니다. 스크립트는 다음 셸 중 하나일 수 있습니다:

- Bash
- PowerShell Desktop
- PowerShell Core
- 배치 (더 이상 사용되지 않음)

[`[[runners]]`](../configuration/advanced-configuration.md#the-runners-section) 내의 `shell`로 구성된 셸을 사용하여 스크립트를 생성합니다. 아무 것도 제공되지 않으면 OS 플랫폼의 기본값이 사용됩니다.

> [!note]
> `shell` 구성이 `run_exec` 스크립트에서 사용하는 PowerShell 버전과 일치하는지 확인합니다. `shell = "pwsh"`를 `pwsh.exe` (PowerShell Core)과 함께 사용하거나 `shell = "powershell"`를 `powershell.exe` (PowerShell Desktop)과 함께 사용합니다.

아래 표는 각 스크립트가 수행하는 작업과 해당 스크립트의 주요 목표에 대한 자세한 설명입니다.

| 스크립트 이름                   | 스크립트 콘텐츠 |
|-------------------------------|-----------------|
| `prepare_script`              | 작업이 실행 중인 머신에 대한 디버그 정보입니다. |
| `get_sources`                 | Git 구성을 준비하고 리포지토리를 복제/가져옵니다. 이를 그대로 유지하는 것을 권장합니다. 왜냐하면 GitLab이 제공하는 Git 전략의 모든 이점을 얻을 수 있기 때문입니다. |
| `restore_cache`               | 정의된 캐시를 추출합니다. 이는 `gitlab-runner` 바이너리가 `$PATH`에서 사용 가능할 것으로 예상합니다. |
| `download_artifacts`          | 정의된 아티팩트를 다운로드합니다. 이는 `gitlab-runner` 바이너리가 `$PATH`에서 사용 가능할 것으로 예상합니다. |
| `step_*`                      | GitLab에서 생성했습니다. 실행할 스크립트 세트입니다. 사용자 정의 실행기로 보내지지 않을 수 있습니다. 여러 단계가 있을 수 있습니다 (예: `step_release` 및 `step_accessibility`). 이것은 `.gitlab-ci.yml` 파일의 기능이 될 수 있습니다. |
| `after_script`                | 작업에서 정의된 [`after_script`](https://docs.gitlab.com/ci/yaml/#after_script). 별도의 셸 컨텍스트에서 실행됩니다. `pre_build_script`를 포함하여 이전 단계가 실패하더라도 항상 실행됩니다. |
| `archive_cache`               | 정의된 모든 캐시의 아카이브를 생성합니다. `build_script`가 성공했을 때만 실행됩니다. |
| `archive_cache_on_failure`    | 정의된 모든 캐시의 아카이브를 생성합니다. `build_script`이 실패했을 때만 실행됩니다. |
| `upload_artifacts_on_success` | 정의된 모든 아티팩트를 업로드합니다. `build_script`가 성공했을 때만 실행됩니다. |
| `upload_artifacts_on_failure` | 정의된 모든 아티팩트를 업로드합니다. `build_script`이 실패했을 때만 실행됩니다. |
| `cleanup_file_variables`      | 모든 [파일 기반](https://docs.gitlab.com/ci/variables/#use-file-type-cicd-variables) 변수를 디스크에서 삭제합니다. |

### 정리 {#cleanup}

정리 스테이지는 `cleanup_exec`에서 실행됩니다.

이 최종 스테이지는 이전 스테이지 중 하나가 실패했더라도 실행됩니다. 이 스테이지의 주요 목표는 설정되었을 수 있는 환경을 정리하는 것입니다. 예를 들어 VM을 끄거나 컨테이너를 삭제합니다.

`cleanup_exec`의 결과는 작업 상태에 영향을 주지 않습니다. 예를 들어 다음 중 하나가 발생하더라도 작업은 성공으로 표시됩니다:

- `prepare_exec` 및 `run_exec`는 모두 성공합니다.
- `cleanup_exec`이 실패합니다.

[`cleanup_exec_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)를 구성하여 GitLab 러너가 프로세스를 종료하기 전에 환경을 정리하기 위해 대기해야 하는 기간에 대한 기한을 설정할 수 있습니다.

이 실행 파일의 `STDOUT`은 `DEBUG` 수준의 GitLab 러너 로그에 인쇄됩니다. `STDERR`는 `WARN` 수준의 로그에 인쇄됩니다.

[`cleanup_exec_args`](../configuration/advanced-configuration.md#the-runnerscustom-section)를 정의하는 경우, 정의한 순서대로 `cleanup_exec` 실행 파일에 추가됩니다. 예를 들어 이 `config.toml` 콘텐츠로:

```toml
...
[runners.custom]
  ...
  cleanup_exec = "/path/to/bin"
  cleanup_args = [ "Arg1", "Arg2" ]
  ...
```

GitLab 러너는 `/path/to/bin Arg1 Arg2`로 실행합니다.

## 실행 파일 종료 및 강제 종료 {#terminating-and-killing-executables}

GitLab 러너는 다음 조건 중 하나에서 실행 파일을 정상적으로 종료하려고 합니다:

- `config_exec_timeout`, `prepare_exec_timeout` 또는 `cleanup_exec_timeout`가 충족됩니다.
- 작업이 [초과됩니다](https://docs.gitlab.com/ci/pipelines/settings/#set-a-limit-for-how-long-jobs-can-run).
- 작업이 취소됩니다.

타임아웃에 도달하면 `SIGTERM`가 실행 파일로 전송되고 [`exec_terminate_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)의 카운트다운이 시작됩니다. 실행 파일은 이 신호를 수신하여 리소스를 정리하는지 확인해야 합니다. `exec_terminate_timeout`가 지나고 프로세스가 여전히 실행 중이면 `SIGKILL`가 프로세스를 강제 종료하도록 전송되고 [`exec_force_kill_timeout`](../configuration/advanced-configuration.md#the-runnerscustom-section)이 시작됩니다. `exec_force_kill_timeout`이 완료된 후 프로세스가 여전히 실행 중이면 GitLab 러너는 프로세스를 중단하고 더 이상 중지하거나 강제 종료하려고 시도하지 않습니다. 이 두 타임아웃이 `config_exec`, `prepare_exec` 또는 `run_exec` 중에 도달하면 빌드가 실패로 표시됩니다.

드라이버에서 생성된 모든 자식 프로세스도 UNIX 기반 시스템에서 위에 설명된 정상적인 종료 프로세스를 수신합니다. 이는 주 프로세스를 [프로세스 그룹](https://man7.org/linux/man-pages/man2/setpgid.2.html)으로 설정하여 모든 자식 프로세스가 속해 있도록 함으로써 달성됩니다.

## 오류 처리 {#error-handling}

GitLab 러너는 두 가지 오류 유형을 다르게 처리할 수 있습니다. 이러한 오류는 `config_exec`, `prepare_exec`, `run_exec` 및 `cleanup_exec` 내의 실행 파일이 이러한 코드로 종료할 때만 처리됩니다. 사용자가 0이 아닌 종료 코드로 종료하면 아래 오류 코드 중 하나로 전파되어야 합니다.

사용자 스크립트가 이러한 코드 중 하나로 종료되면 실행 파일 종료 코드로 전파되어야 합니다.

### 빌드 실패 {#build-failure}

GitLab 러너는 실행 파일이 작업 실패를 나타내기 위한 종료 코드로 사용해야 하는 `BUILD_FAILURE_EXIT_CODE` 환경 변수를 제공합니다. 실행 파일이 `BUILD_FAILURE_EXIT_CODE`의 코드로 종료되면 빌드는 GitLab CI에서 적절하게 실패로 표시됩니다.

사용자가 `.gitlab-ci.yml` 파일 내에 정의한 스크립트가 0이 아닌 코드로 종료되면 `run_exec`는 `BUILD_FAILURE_EXIT_CODE` 값으로 종료되어야 합니다.

> [!note]
> `BUILD_FAILURE_EXIT_CODE`를 사용하여 종료하기를 강력하게 권장합니다. 하드 코딩된 값이 아닌 경우 모든 릴리스에서 변경될 수 있으므로 바이너리/스크립트 향후 증명이 됩니다.

### 빌드 실패 종료 코드 {#build-failure-exit-code}

빌드가 실패할 때 종료 코드를 포함하는 파일을 선택적으로 제공할 수 있습니다. 파일의 예상 경로는 `BUILD_EXIT_CODE_FILE` 환경 변수를 통해 제공됩니다. 예를 들어:

```shell
if [ $exit_code -ne 0 ]; then
  echo $exit_code > ${BUILD_EXIT_CODE_FILE}
  exit ${BUILD_FAILURE_EXIT_CODE}
fi
```

CI/CD 작업은 [`allow_failure`](https://docs.gitlab.com/ci/yaml/#allow_failure) 구문을 활용하려면 이 메서드가 필요합니다.

> [!note]
> 이 파일에 정수 종료 코드만 저장하세요. 추가 정보로 인해 `unknown Custom executor executable exit code` 오류가 발생할 수 있습니다.

### 시스템 실패 {#system-failure}

`SYSTEM_FAILURE_EXIT_CODE`에 지정된 오류 코드로 프로세스를 종료하여 GitLab 러너에 시스템 실패를 보낼 수 있습니다. 이 오류 코드가 반환되면 GitLab 러너는 특정 스테이지를 재시도합니다. 재시도 중 어느 것도 성공하지 못하면 작업이 실패로 표시됩니다.

다음은 재시도되는 스테이지와 그 횟수에 대한 표입니다.

| 스테이지 이름           | 시도 횟수                                          | 각 재시도 간 대기 시간 |
|----------------------|-------------------------------------------------------------|-------------------------------------|
| `prepare_exec`       | 3                                                           | 3초                           |
| `get_sources`        | `GET_SOURCES_ATTEMPTS` 변수의 값입니다. (기본값 1)       | 0초                           |
| `restore_cache`      | `RESTORE_CACHE_ATTEMPTS` 변수의 값입니다. (기본값 1)     | 0초                           |
| `download_artifacts` | `ARTIFACT_DOWNLOAD_ATTEMPTS` 변수의 값입니다. (기본값 1) | 0초                           |

> [!note]
> `SYSTEM_FAILURE_EXIT_CODE`를 사용하여 종료하기를 강력하게 권장합니다. 하드 코딩된 값이 아닌 경우 모든 릴리스에서 변경될 수 있으므로 바이너리/스크립트 향후 증명이 됩니다.

## 작업 응답 {#job-response}

`CUSTOM_ENV_` 변수를 변경할 수 있습니다. 문서화된 [CI/CD 변수 우선 순위](https://docs.gitlab.com/ci/variables/#cicd-variable-precedence)를 관찰합니다. 이 기능이 바람직할 수 있지만 신뢰할 수 있는 작업 컨텍스트가 필요할 때 전체 JSON 작업 응답이 자동으로 제공됩니다. 러너는 임시 파일을 생성하고 `JOB_RESPONSE_FILE` 환경 변수에서 참조됩니다. 이 파일은 모든 스테이지에 존재하며 정리 중에 자동으로 제거됩니다.

```shell
$ cat ${JOB_RESPONSE_FILE}
{"id": 123456, "token": "jobT0ken",...}
```
