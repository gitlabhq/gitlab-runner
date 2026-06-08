---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 러너 등록
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

{{< history >}}

- [GitLab Runner 15.0에서 도입됨](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3414)으로, 등록 요청 형식의 변경으로 인해 GitLab Runner가 이전 버전의 GitLab과 통신하지 못합니다. GitLab 버전에 적합한 GitLab Runner 버전을 사용하거나 GitLab 애플리케이션을 업그레이드합니다.

{{< /history >}}

러너 등록은 러너를 하나 이상의 GitLab 인스턴스와 연결하는 프로세스입니다. GitLab 인스턴스에서 작업을 선택할 수 있도록 러너를 등록합니다.

## 요구 사항 {#requirements}

러너를 등록하기 전에:

- [GitLab Runner](../install/_index.md)를 GitLab이 설치된 위치와 별도의 서버에 설치합니다.
- Docker를 사용하여 러너를 등록하려면 [GitLab Runner를 Docker 컨테이너에 설치](../install/docker.md)합니다.

## 러너 인증 토큰으로 등록 {#register-with-a-runner-authentication-token}

{{< history >}}

- [GitLab 15.10에서 도입됨](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29613).

{{< /history >}}

전제 조건:

- 러너 인증 토큰을 획득합니다. 다음 중 하나를 수행할 수 있습니다:
  - 인스턴스, 그룹 또는 프로젝트 러너를 생성합니다. 지침은 [러너 관리](https://docs.gitlab.com/ci/runners/runners_scope)를 참조하세요.
  - `config.toml` 파일에서 러너 인증 토큰을 찾습니다. 러너 인증 토큰에는 `glrt-` 접두사가 있습니다.

러너를 등록한 후 구성이 `config.toml`에 저장됩니다.

[러너 인증 토큰](https://docs.gitlab.com/security/tokens/#runner-authentication-tokens)으로 러너를 등록하려면:

1. 등록 명령을 실행합니다:

   {{< tabs >}}

   {{< tab title="Linux" >}}

   ```shell
   sudo gitlab-runner register
   ```

   프록시 뒤에 있는 경우 환경 변수를 추가한 후 등록 명령을 실행합니다:

   ```shell
   export HTTP_PROXY=http://yourproxyurl:3128
   export HTTPS_PROXY=http://yourproxyurl:3128

   sudo -E gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="macOS" >}}

   ```shell
   gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="Windows" >}}

   ```shell
   .\gitlab-runner.exe register
   ```

   {{< /tab >}}

   {{< tab title="FreeBSD" >}}

   ```shell
   sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="Docker" >}}

   컨테이너에 등록하려면 다음 중 하나를 수행할 수 있습니다:

   - 올바른 구성 볼륨 마운트를 사용하여 `gitlab-runner` 컨테이너를 단기간만 사용합니다:

     - 로컬 시스템 볼륨 마운트의 경우:

       ```shell
       docker run --rm -it -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register
       ```

       설치 중에 `/srv/gitlab-runner/config` 이외의 구성 볼륨을 사용한 경우 올바른 볼륨으로 명령을 업데이트합니다.

     - Docker 볼륨 마운트의 경우:

       ```shell
       docker run --rm -it -v gitlab-runner-config:/etc/gitlab-runner gitlab/gitlab-runner:latest register
       ```

   - 활성 러너 컨테이너 내에서 실행 파일을 사용합니다:

     ```shell
     docker exec -it gitlab-runner gitlab-runner register
     ```

   {{< /tab >}}

   {{< /tabs >}}

1. GitLab URL을 입력합니다:
   - GitLab Self-Managed의 러너의 경우 GitLab 인스턴스의 URL을 사용합니다. 예를 들어 프로젝트가 `gitlab.example.com/yourname/yourproject`에 호스팅되는 경우 GitLab 인스턴스 URL은 `https://gitlab.example.com`입니다.
   - GitLab.com의 러너의 경우 GitLab 인스턴스 URL은 `https://gitlab.com`입니다.
1. 러너 인증 토큰을 입력합니다.
1. 러너에 대한 설명을 입력합니다.
1. 작업 태그를 쉼표로 구분하여 입력합니다.
1. 러너에 대한 선택 사항의 유지 보수 메모를 입력합니다.
1. [실행기](../executors/_index.md)의 유형을 입력합니다.

- 동일한 호스트 머신에서 서로 다른 구성으로 여러 러너를 등록하려면 `register` 명령을 반복합니다.
- 여러 호스트 머신에서 동일한 구성을 등록하려면 각 러너 등록에 동일한 러너 인증 토큰을 사용합니다. 자세한 내용은 [러너 구성 재사용](../fleet_scaling/_index.md#reusing-a-runner-configuration)을 참조하세요.

[대화식이 아닌 모드](../commands/_index.md#non-interactive-registration)를 사용하여 러너를 등록할 추가 인수를 사용할 수도 있습니다:

{{< tabs >}}

{{< tab title="Linux" >}}

```shell
sudo gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

{{< /tab >}}

{{< tab title="macOS" >}}

```shell
gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

{{< /tab >}}

{{< tab title="Windows" >}}

```shell
.\gitlab-runner.exe register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker-windows" \
  --docker-image mcr.microsoft.com/windows/servercore:1809_amd64 \
  --description "docker-runner"
```

{{< /tab >}}

{{< tab title="FreeBSD" >}}

```shell
sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

{{< /tab >}}

{{< tab title="Docker" >}}

```shell
docker run --rm -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

{{< /tab >}}

{{< /tabs >}}

## 러너 등록 토큰으로 등록(더 이상 사용되지 않음) {#register-with-a-runner-registration-token-deprecated}

> [!warning]
> 러너 등록 토큰 및 여러 러너 구성 인수는 [더 이상 사용되지 않습니다](https://gitlab.com/gitlab-org/gitlab/-/issues/380872). GitLab 20.0에서 제거하도록 예정되어 있습니다. 대신 러너 인증 토큰을 사용합니다. 자세한 내용은 [새로운 러너 등록 워크플로우로 마이그레이션](https://docs.gitlab.com/ci/runners/new_creation_workflow/)을 참조하세요.

전제 조건:

- 러너 등록 토큰은 관리자 영역에서 [활성화](https://docs.gitlab.com/administration/settings/continuous_integration/#control-runner-registration)되어야 합니다.
- 원하는 인스턴스, 그룹 또는 프로젝트에서 러너 등록 토큰을 획득합니다. 지침은 [러너 관리](https://docs.gitlab.com/ci/runners/runners_scope)를 참조하세요.

러너를 등록한 후 구성이 `config.toml`에 저장됩니다.

[러너 등록 토큰](https://docs.gitlab.com/security/tokens/#runner-registration-tokens-legacy)으로 러너를 등록하려면:

1. 등록 명령을 실행합니다:

   {{< tabs >}}

   {{< tab title="Linux" >}}

   ```shell
   sudo gitlab-runner register
   ```

   프록시 뒤에 있는 경우 환경 변수를 추가한 후 등록 명령을 실행합니다:

   ```shell
   export HTTP_PROXY=http://yourproxyurl:3128
   export HTTPS_PROXY=http://yourproxyurl:3128

   sudo -E gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="macOS" >}}

   ```shell
   gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="Windows" >}}

   ```shell
   .\gitlab-runner.exe register
   ```

   {{< /tab >}}

   {{< tab title="FreeBSD" >}}

   ```shell
   sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="Docker" >}}

   설치 중에 생성한 컨테이너를 등록하기 위해 `gitlab-runner` 컨테이너를 단기간만 실행합니다:

   - 로컬 시스템 볼륨 마운트의 경우:

     ```shell
     docker run --rm -it -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register
     ```

     설치 중에 `/srv/gitlab-runner/config` 이외의 구성 볼륨을 사용한 경우 올바른 볼륨으로 명령을 업데이트합니다.

   - Docker 볼륨 마운트의 경우:

     ```shell
     docker run --rm -it -v gitlab-runner-config:/etc/gitlab-runner gitlab/gitlab-runner:latest register
     ```

   {{< /tab >}}

   {{< /tabs >}}

1. GitLab URL을 입력합니다:
   - GitLab Self-Managed의 러너의 경우 GitLab 인스턴스의 URL을 사용합니다. 예를 들어 프로젝트가 `gitlab.example.com/yourname/yourproject`에 호스팅되는 경우 GitLab 인스턴스 URL은 `https://gitlab.example.com`입니다.
   - GitLab.com의 경우 GitLab 인스턴스 URL은 `https://gitlab.com`입니다.
1. 러너를 등록하기 위해 획득한 토큰을 입력합니다.
1. 러너에 대한 설명을 입력합니다.
1. 작업 태그를 쉼표로 구분하여 입력합니다.
1. 러너에 대한 선택 사항의 유지 보수 메모를 입력합니다.
1. [실행기](../executors/_index.md)의 유형을 입력합니다.

동일한 호스트 머신에서 서로 다른 구성으로 여러 러너를 등록하려면 `register` 명령을 반복합니다.

[대화식이 아닌 모드](../commands/_index.md#non-interactive-registration)를 사용하여 러너를 등록할 추가 인수를 사용할 수도 있습니다:

{{< tabs >}}

{{< tab title="Linux" >}}

```shell
sudo gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< tab title="macOS" >}}

```shell
gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< tab title="Windows" >}}

```shell
.\gitlab-runner.exe register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker-windows" \
  --docker-image mcr.microsoft.com/windows/servercore:1809_amd64 \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< tab title="FreeBSD" >}}

```shell
sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< tab title="Docker" >}}

```shell
docker run --rm -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< /tabs >}}

- `--access-level`는 [보호되는 러너](https://docs.gitlab.com/ci/runners/configure_runners/#prevent-runners-from-revealing-sensitive-information)를 생성합니다.
  - 보호되는 러너의 경우 `--access-level="ref_protected"` 매개 변수를 사용합니다.
  - 보호되지 않는 러너의 경우 `--access-level="not_protected"`을 사용하거나 값을 정의되지 않은 상태로 둡니다.
- `--maintenance-note`은 러너 유지 보수에 도움이 될 수 있는 정보를 추가할 수 있습니다. 최대 길이는 255자입니다.

### 레거시 호환 등록 프로세스 {#legacy-compatible-registration-process}

{{< history >}}

- [GitLab 16.2에서 도입됨](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4157).

{{< /history >}}

러너 등록 토큰 및 여러 러너 구성 인수는 [더 이상 사용되지 않습니다](https://gitlab.com/gitlab-org/gitlab/-/issues/379743). GitLab 20.0에서 제거하도록 예정되어 있습니다. 자동화 워크플로우의 최소 중단을 보장하기 위해 `legacy-compatible registration process`은 러너 인증 토큰이 레거시 매개 변수 `--registration-token`에 지정된 경우 트리거됩니다.

레거시 호환 등록 프로세스는 다음 명령줄 매개 변수를 무시합니다. 이 매개 변수는 UI 또는 API를 사용하여 러너를 생성할 때만 구성할 수 있습니다.

- `--locked`
- `--access-level`
- `--run-untagged`
- `--maximum-timeout`
- `--paused`
- `--tag-list`
- `--maintenance-note`

## 구성 템플릿으로 등록 {#register-with-a-configuration-template}

구성 템플릿을 사용하여 `register` 명령에서 지원하지 않는 설정으로 러너를 등록할 수 있습니다.

전제 조건:

- 템플릿 파일 위치의 볼륨을 GitLab Runner 컨테이너에 마운트해야 합니다.
- 러너 인증 또는 등록 토큰:
  - 러너 인증 토큰을 획득합니다(권장). 다음 중 하나를 수행할 수 있습니다:
    - 원하는 인스턴스, 그룹 또는 프로젝트에서 러너 인증 토큰을 획득합니다. 지침은 [러너 관리](https://docs.gitlab.com/ci/runners/runners_scope)를 참조하세요.
    - `config.toml` 파일에서 러너 인증 토큰을 찾습니다. 러너 인증 토큰에는 `glrt-` 접두사가 있습니다.
  - 인스턴스, 그룹 또는 프로젝트에 대해 러너 등록 토큰(더 이상 사용되지 않음)을 획득합니다. 지침은 [러너 관리](https://docs.gitlab.com/ci/runners/runners_scope)를 참조하세요.

구성 템플릿은 다음과 같은 이유로 `register` 명령의 일부 인수를 지원하지 않는 자동화된 환경에 사용할 수 있습니다:

- 환경을 기반으로 환경 변수의 크기 제한입니다.
- Kubernetes를 위한 실행기 볼륨에 사용할 수 없는 명령줄 옵션입니다.

> [!warning]
> 구성 템플릿은 단일 [`[[runners]]`](../configuration/advanced-configuration.md#the-runners-section) 섹션만 지원하며 전역 옵션을 지원하지 않습니다.

러너를 등록하려면:

1. `.toml` 형식의 구성 템플릿 파일을 만들고 사양을 추가합니다. 예를 들어:

   ```toml
   [[runners]]
     [runners.kubernetes]
     [runners.kubernetes.volumes]
       [[runners.kubernetes.volumes.empty_dir]]
         name = "empty_dir"
         mount_path = "/path/to/empty_dir"
         medium = "Memory"
   ```

1. 파일의 경로를 추가합니다. 다음 중 하나를 사용할 수 있습니다:
   - 명령줄의 [대화식이 아닌 모드](../commands/_index.md#non-interactive-registration):

     ```shell
     $ sudo gitlab-runner register \
         --template-config /tmp/test-config.template.toml \
         --non-interactive \
         --url "https://gitlab.com" \
         --token <TOKEN> \ "# --registration-token if using the deprecated runner registration token"
         --name test-runner \
         --executor kubernetes
         --host = "http://localhost:9876/"
     ```

   - `.gitlab.yaml` 파일의 환경 변수:

     ```yaml
     variables:
       TEMPLATE_CONFIG_FILE = <file_path>
     ```

     환경 변수를 업데이트하면 러너를 등록할 때마다 `register` 명령에서 파일 경로를 추가할 필요가 없습니다.

러너를 등록한 후 구성 템플릿의 설정이 `config.toml`에서 생성된 `[[runners]]` 항목과 병합됩니다:

```toml
concurrent = 1
check_interval = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = "test-runner"
  url = "https://gitlab.com"
  token = "glrt-<TOKEN>"
  executor = "kubernetes"
  [runners.kubernetes]
    host = "http://localhost:9876/"
    bearer_token_overwrite_allowed = false
    image = ""
    namespace = ""
    namespace_overwrite_allowed = ""
    privileged = false
    service_account_overwrite_allowed = ""
    pod_labels_overwrite_allowed = ""
    pod_annotations_overwrite_allowed = ""
    [runners.kubernetes.volumes]

      [[runners.kubernetes.volumes.empty_dir]]
        name = "empty_dir"
        mount_path = "/path/to/empty_dir"
        medium = "Memory"
```

템플릿 설정은 다음과 같은 옵션에 대해서만 병합됩니다:

- 빈 문자열
- Null 또는 존재하지 않는 항목
- 0

명령줄 인수 또는 환경 변수는 구성 템플릿의 설정보다 우선합니다. 예를 들어 템플릿이 `docker` 실행기를 지정하지만 명령줄이 `shell`를 지정하면 구성된 실행기는 `shell`입니다.

## GitLab Community Edition 통합 테스트를 위한 러너 등록 {#register-a-runner-for-gitlab-community-edition-integration-tests}

GitLab Community Edition 통합을 테스트하려면 구성 템플릿을 사용하여 제한된 Docker 실행기를 사용하는 러너를 등록합니다.

1. [프로젝트 러너](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-project-runner-with-a-runner-authentication-token)를 생성합니다.
1. `[[runners.docker.services]]` 섹션과 함께 템플릿을 생성합니다:

   ```shell
   $ cat > /tmp/test-config.template.toml << EOF
   [[runners]]
   [runners.docker]
   [[runners.docker.services]]
   name = "mysql:latest"
   [[runners.docker.services]]
   name = "redis:latest"

   EOF
   ```

1. 러너를 등록합니다:

   {{< tabs >}}

   {{< tab title="Linux" >}}

   ```shell
   sudo gitlab-runner register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< tab title="macOS" >}}

   ```shell
   gitlab-runner register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< tab title="Windows" >}}

   ```shell
   .\gitlab-runner.exe register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< tab title="FreeBSD" >}}

   ```shell
   sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< tab title="Docker" >}}

   ```shell
   docker run --rm -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< /tabs >}}

자세한 구성 옵션은 [고급 구성](../configuration/advanced-configuration.md)을 참조하세요.

## Docker를 사용하여 러너 등록 {#registering-runners-with-docker}

Docker 컨테이너로 러너를 등록한 후:

- 구성이 구성 볼륨에 쓰입니다. 예를 들어, `/srv/gitlab-runner/config`.
- 컨테이너는 구성 볼륨을 사용하여 러너를 로드합니다.

> [!note]
> `gitlab-runner restart`가 Docker 컨테이너에서 실행되면 GitLab Runner는 기존 프로세스를 다시 시작하는 대신 새 프로세스를 시작합니다. 구성 변경 사항을 적용하려면 대신 Docker 컨테이너를 다시 시작합니다.

## 문제 해결 {#troubleshooting}

### 오류: `Check registration token` {#error-check-registration-token}

`check registration token` 오류 메시지는 GitLab 인스턴스가 등록 중에 입력한 러너 등록 토큰을 인식하지 못할 때 표시됩니다. 이 문제는 다음 중 하나의 경우에 발생할 수 있습니다:

- 인스턴스, 그룹 또는 프로젝트 러너 등록 토큰이 GitLab에서 변경되었습니다.
- 잘못된 러너 등록 토큰이 입력되었습니다.

이 오류가 발생하면 GitLab 관리자에게 요청할 수 있습니다:

- 러너 등록 토큰이 유효한지 확인합니다.
- 프로젝트 또는 그룹에서 러너 등록이 [허용되는지](https://docs.gitlab.com/administration/settings/continuous_integration/#restrict-runner-registration-for-a-specific-group) 확인합니다.

### 오류: `410 Gone - runner registration disallowed` {#error-410-gone---runner-registration-disallowed}

`410 Gone - runner registration disallowed` 오류 메시지는 등록 토큰을 통한 러너 등록이 비활성화되었을 때 표시됩니다.

이 오류가 발생하면 GitLab 관리자에게 요청할 수 있습니다:

- 러너 등록 토큰이 유효한지 확인합니다.
- 인스턴스에서 러너 등록이 [허용되는지](https://docs.gitlab.com/administration/settings/continuous_integration/#control-runner-registration) 확인합니다.
- 그룹 또는 프로젝트 러너 등록 토큰의 경우 각 그룹 및/또는 프로젝트에서 러너 등록이 [허용되는지](https://docs.gitlab.com/ci/runners/runners_scope/#enable-use-of-runner-registration-tokens-in-projects-and-groups) 확인합니다.
