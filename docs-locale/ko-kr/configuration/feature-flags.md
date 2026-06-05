---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Runner 기능 플래그
---

> [!warning]
> 기본적으로 비활성화된 기능을 활성화하면 데이터 손상, 안정성 저하, 성능 저하 및 보안 문제가 발생할 수 있습니다. 기능 플래그를 활성화하기 전에 관련된 위험을 인식해야 합니다. 자세한 내용은 [개발 중인 기능 활성화 시 위험 사항](https://docs.gitlab.com/administration/feature_flags/#risks-when-enabling-features-still-in-development)을 참조하세요.

기능 플래그는 특정 기능을 활성화하거나 비활성화할 수 있는 토글입니다. 이러한 플래그는 일반적으로 다음과 같이 사용됩니다:

- 자원자가 테스트할 수 있도록 제공하지만 모든 사용자에게 활성화할 준비가 완료되지 않은 베타 기능용입니다.

  베타 기능은 때때로 불완전하거나 추가 테스트가 필요합니다. 베타 기능을 사용하고자 하는 사용자는 위험을 감수하기로 선택하고 기능 플래그를 사용하여 명시적으로 기능을 활성화할 수 있습니다. 기능이 필요하지 않거나 자신의 시스템에서 위험을 감수하기를 원하지 않는 다른 사용자는 기능이 기본적으로 비활성화되어 있으며 가능한 버그 및 회귀로 인한 영향을 받지 않습니다.

- 기능 제거 또는 기능 지원 중단으로 인한 변경이 가까운 미래에 발생하는 경우입니다.

  제품이 발전함에 따라 기능이 때때로 변경되거나 완전히 제거됩니다. 알려진 버그는 종종 수정되지만, 일부 경우에는 사용자가 이미 자신에게 영향을 준 버그에 대한 해결 방법을 찾았습니다. 사용자가 표준화된 버그 수정을 채택하도록 강제하면 자신의 사용자 정의 구성에 다른 문제가 발생할 수 있습니다.

  이러한 경우 기능 플래그를 사용하여 필요에 따라 이전 동작에서 새로운 동작으로 전환합니다. 이를 통해 사용자는 제품의 새로운 버전을 도입하면서 이전 동작에서 새로운 동작으로의 부드럽고 영구적인 전환을 계획할 시간을 얻을 수 있습니다.

기능 플래그는 환경 변수를 사용하여 토글됩니다. 다음 작업을 수행합니다:

- 기능 플래그를 활성화하려면 해당 환경 변수를 `"true"` 또는 `1`로 설정합니다.
- 기능 플래그를 비활성화하려면 해당 환경 변수를 `"false"` 또는 `0`로 설정합니다.

## 사용 가능한 기능 플래그 {#available-feature-flags}

<!--
The list of feature flags is created automatically.
If you need to update it, call `make update_feature_flags_docs` in the
root directory of this project.
The flags are defined in `./helpers/featureflags/flags.go` file.
-->

<!-- feature_flags_list_start -->

| 기능 플래그 | 기본값 | 지원 중단됨 | 제거 예정 | 설명 |
|--------------|---------------|------------|--------------------|-------------|
| `FF_NETWORK_PER_BUILD` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | `docker` 실행기를 사용하여 Docker [빌드당 네트워크](../executors/docker.md#network-configurations) 생성을 활성화합니다. `CI_BUILD_NETWORK_NAME` 변수를 사용하여 네트워크 이름을 가져옵니다. |
| `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | `false`로 설정하면 [#4119](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4119)와 같은 문제를 해결하기 위해 `attach` 대신 `exec`를 통한 원격 Kubernetes 명령 실행이 비활성화됩니다. 이 기능 플래그는 서비스 계정이 특정 권한을 가져야 합니다. 자세한 내용은 [러너 API 권한 구성](../executors/kubernetes/_index.md#configure-runner-api-permissions)을 참조하세요. |
| `FF_USE_DIRECT_DOWNLOAD` | `true` | {{< icon name="dotted-circle" >}} 아니요 |  | `true`로 설정하면 러너는 GitLab을 통해 프록시하는 대신 처음 시도 시 모든 아티팩트를 직접 다운로드합니다. GitLab에서 활성화한 경우 Object Storage의 TLS 인증서 유효성 검사 문제로 인해 다운로드 실패가 발생할 수 있습니다. [자체 서명된 인증서 또는 사용자 정의 인증 기관](tls-self-signed.md)을 참조하세요. |
| `FF_SKIP_NOOP_BUILD_STAGES` | `true` | {{< icon name="dotted-circle" >}} 아니요 |  | `false`로 설정하면 실행해도 효과가 없는 경우에도 모든 빌드 스테이지가 실행됩니다. |
| `FF_USE_FASTZIP` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | Fastzip은 캐시/아티팩트 아카이빙 및 추출을 위한 고성능 아카이버입니다. |
| `FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 `docker` 실행기로 실행되는 작업에 대해 `umask 0000` 호출 사용이 제거됩니다. 대신 러너는 빌드 컨테이너에 사용되는 이미지에 대해 구성된 사용자의 UID 및 GID를 검색하려고 시도하고 사전 정의된 컨테이너에서 `chmod` 명령을 실행하여 작업 디렉터리 및 파일의 소유권을 변경합니다(소스 업데이트, 캐시 복원 및 아티팩트 다운로드 후). POSIX 유틸리티 `id`는 이 기능 플래그를 위해 빌드 이미지에 설치되고 작동해야 합니다. 러너는 `id`를 `-u` 및 `-g` 옵션으로 실행하여 UID 및 GID를 검색합니다. |
| `FF_ENABLE_BASH_EXIT_CODE_CHECK` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화된 경우, Bash 스크립트는 `set -e`에만 의존하지 않고 각 스크립트 명령을 실행한 후 0이 아닌 종료 코드를 확인합니다. |
| `FF_USE_WINDOWS_LEGACY_PROCESS_STRATEGY` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | GitLab Runner 16.10 이상에서 기본값은 `false`입니다. GitLab Runner 16.9 이전에서 기본값은 `true`입니다. 비활성화되면 러너가 Windows(셸 및 사용자 정의 실행기)에서 생성하는 프로세스는 프로세스 종료를 개선해야 하는 추가 설정으로 생성됩니다. `true`로 설정하면 레거시 프로세스 설정이 사용됩니다. Windows 러너를 성공적으로 올바르게 드레인하려면 이 기능 플래그를 `false`로 설정해야 합니다. |
| `FF_USE_NEW_BASH_EVAL_STRATEGY` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | `true`로 설정하면 Bash `eval` 호출이 서브셸에서 실행되어 실행된 스크립트의 올바른 종료 코드 감지를 도웁니다. |
| `FF_USE_POWERSHELL_PATH_RESOLVER` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 PowerShell은 러너가 호스트된 위치에 특정한 OS별 파일 경로 함수를 사용하는 대신 경로명을 확인합니다. |
| `FF_USE_DYNAMIC_TRACE_FORCE_SEND_INTERVAL` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 로그의 추적 강제 전송 간격이 추적 업데이트 간격을 기반으로 동적으로 조정됩니다. |
| `FF_SCRIPT_SECTIONS` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 다중 라인 스크립트 명령이 작업 로그에서 축소 가능한 섹션으로 표시되고, 단일 라인 명령은 `$` 접두사를 사용하여 직접 출력됩니다. 이것은 알려진 문제입니다. 자세한 내용은 [이슈 39294](https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/39294)를 참조하세요. |
| `FF_ENABLE_JOB_CLEANUP` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 빌드 끝에 프로젝트 디렉터리가 정리됩니다. `GIT_CLONE`이 사용되면 전체 프로젝트 디렉터리가 삭제됩니다. `GIT_FETCH`이 사용되면 일련의 Git `clean` 명령이 실행됩니다. |
| `FF_KUBERNETES_HONOR_ENTRYPOINT` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY`이 참으로 설정되지 않은 경우 이미지의 Docker 진입점이 준수됩니다. 이 기능 플래그는 서비스 계정이 특정 권한을 가져야 합니다. 자세한 내용은 [러너 API 권한 구성](../executors/kubernetes/_index.md#configure-runner-api-permissions)을 참조하세요. |
| `FF_POSIXLY_CORRECT_ESCAPES` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 [POSIX 셸 이스케이프](https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_02) 가 [`bash`스타일 ANSI-C 인용](https://www.gnu.org/software/bash/manual/html_node/Quoting.html) 대신 사용됩니다. 작업 환경이 POSIX 호환 셸을 사용하는 경우 활성화해야 합니다. |
| `FF_RESOLVE_FULL_TLS_CHAIN` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | GitLab Runner 16.4 이상에서 기본값은 `false`입니다. GitLab Runner 16.3 이전에서 기본값은 `true`입니다. 활성화되면 러너는 `CI_SERVER_TLS_CA_FILE`에 대해 자체 서명된 루트 인증서까지의 전체 TLS 체인을 확인합니다. 이것은 이전에 [Git HTTPS 클론을 작동하도록 하기 위해 필요했습니다](tls-self-signed.md#git-cloning) libcurl 7.68.0 이전 및 OpenSSL로 빌드된 Git 클라이언트용입니다. 그러나 macOS와 같이 오래된 서명 알고리즘으로 서명된 루트 인증서를 거부하는 일부 운영 체제에서 인증서 확인 프로세스가 실패할 수 있습니다. 인증서 확인이 실패하면 이 기능을 비활성화해야 할 수 있습니다. 이 기능 플래그는 [`[runners.feature_flags]` 구성](#enable-feature-flag-in-runner-configuration)에서만 비활성화할 수 있습니다. |
| `FF_DISABLE_POWERSHELL_STDIN` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 셸 및 사용자 정의 실행기용 PowerShell 스크립트는 stdin을 통해 전달되고 실행되는 대신 파일로 전달됩니다. 이는 작업의 `allow_failure:exit_codes` 키워드가 올바르게 작동하기 위해 필요합니다. |
| `FF_USE_POD_ACTIVE_DEADLINE_SECONDS` | `true` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 [Pod `activeDeadlineSeconds`](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-v1/#lifecycle)가 CI/CD 작업 시간 초과로 설정됩니다. 이 플래그는 [Pod의 수명 주기](../executors/kubernetes/_index.md#pod-lifecycle)에 영향을 미칩니다. |
| `FF_USE_ADVANCED_POD_SPEC_CONFIGURATION` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 사용자는 `config.toml` 파일에서 전체 Pod 사양을 설정할 수 있습니다. 자세한 내용은 [생성된 Pod 사양 덮어쓰기(실험)](../executors/kubernetes/_index.md#overwrite-generated-pod-specifications)를 참조하세요. |
| `FF_SET_PERMISSIONS_BEFORE_CLEANUP` | `true` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 프로젝트 디렉터리의 디렉터리 및 파일에 대한 권한이 먼저 설정되어 정리 중 삭제가 성공적으로 수행되도록 합니다. |
| `FF_SECRET_RESOLVING_FAILS_IF_MISSING` | `true` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 값을 찾을 수 없는 경우 비밀 확인이 실패합니다. |
| `FF_PRINT_POD_EVENTS` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 빌드 Pod와 연결된 모든 이벤트가 시작될 때까지 출력됩니다. |
| `FF_USE_GIT_BUNDLE_URIS` | `true` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 Git `transfer.bundleURI` 구성 옵션이 `true`로 설정됩니다. 이 FF는 기본적으로 활성화됩니다. Git 번들 지원을 비활성화하려면 `false`로 설정합니다. |
| `FF_USE_GIT_NATIVE_CLONE` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되고 `GIT_STRATEGY=clone`인 경우, 프로젝트를 복제하기 위해 `git-init(1)` + `git-fetch(1)` 대신 `git-clone(1)` 명령이 사용됩니다. 이를 위해서는 Git 버전 2.49 이상이 필요하며, 사용할 수 없는 경우 `init` + `fetch`로 폴백됩니다. |
| `FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 `dumb-init`이 모든 스크립트를 실행하는 데 사용됩니다. 이를 통해 `dumb-init`가 도우미 및 빌드 컨테이너의 첫 번째 프로세스로 실행됩니다. |
| `FF_USE_INIT_WITH_DOCKER_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 Docker 실행기가 PID 1로 `tini-init`를 실행하는 `--init` 옵션을 사용하여 서비스 및 빌드 컨테이너를 시작합니다. |
| `FF_LOG_IMAGES_CONFIGURED_FOR_JOB` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 러너는 수신된 각 작업에 대해 정의된 이미지 및 서비스 이미지의 이름을 기록합니다. |
| `FF_USE_DOCKER_AUTOSCALER_DIAL_STDIO` | `true` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화된 경우(기본값), `docker system stdio`이 원격 Docker 데몬으로 터널링하는 데 사용됩니다. 비활성화되면 SSH 연결의 경우 기본 SSH 터널이 사용되고, WinRM 연결의 경우 'fleeting-proxy' 도우미 바이너리가 먼저 배포됩니다. |
| `FF_CLEAN_UP_FAILED_CACHE_EXTRACT` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 빌드 스크립트에 명령이 삽입되어 실패한 캐시 추출을 감지하고 남겨진 부분 캐시 내용을 정리합니다. |
| `FF_USE_WINDOWS_JOB_OBJECT` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 러너가 셸 및 사용자 정의 실행기를 사용하여 Windows에서 생성하는 각 프로세스에 대해 작업 개체가 생성됩니다. 프로세스를 강제 종료하려면 러너는 작업 개체를 닫습니다. 이는 종료하기 어려운 프로세스의 종료를 개선해야 합니다. |
| `FF_TIMESTAMPS` | `true` | {{< icon name="dotted-circle" >}} 아니요 |  | 비활성화되면 각 로그 추적 라인의 시작에 타임스탬프가 추가되지 않습니다. |
| `FF_DISABLE_AUTOMATIC_TOKEN_ROTATION` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 자동 토큰 회전이 제한되고 토큰이 만료될 예정일 때 경고를 기록합니다. |
| `FF_USE_LEGACY_GCS_CACHE_ADAPTER` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 레거시 GCS 캐시 어댑터가 사용됩니다. 비활성화된 경우(기본값), 인증에 Google Cloud Storage의 SDK를 사용하는 더 최신 GCS 캐시 어댑터가 사용됩니다. 이는 GKE의 워크로드 ID 구성과 같이 레거시 어댑터가 문제를 겪었던 환경에서 인증 문제를 해결해야 합니다. |
| `FF_DISABLE_UMASK_FOR_KUBERNETES_EXECUTOR` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 Kubernetes 실행기로 실행되는 작업에 대해 `umask 0000` 호출이 제거됩니다. 대신 러너는 빌드 컨테이너가 실행되는 사용자의 UID(사용자 ID) 및 GID(그룹 ID)를 검색하려고 시도합니다. 러너는 또한 사전 정의된 컨테이너에서 `chown` 명령을 실행하여 작업 디렉터리 및 파일의 소유권을 변경합니다(소스 업데이트, 캐시 복원 및 아티팩트 다운로드 후). |
| `FF_USE_LEGACY_S3_CACHE_ADAPTER` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 레거시 S3 캐시 어댑터가 사용됩니다. 비활성화된 경우(기본값), 인증에 Amazon의 S3 SDK를 사용하는 더 최신 S3 캐시 어댑터가 사용됩니다. 이는 사용자 정의 STS 끝점과 같이 레거시 어댑터가 문제를 겪었던 환경에서 인증 문제를 해결해야 합니다. |
| `FF_GIT_URLS_WITHOUT_TOKENS` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 GitLab 러너는 Git 구성 또는 명령 실행 중에 작업 토큰을 어디에도 포함하지 않습니다. 대신 작업 토큰을 얻기 위해 환경 변수를 사용하는 Git 자격증명 도우미를 설정합니다. 이 접근 방식은 토큰 저장소를 제한하고 토큰 누출 위험을 줄입니다. |
| `FF_WAIT_FOR_POD_TO_BE_REACHABLE` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 러너는 Pod 상태가 '실행 중'이 되기를 기다리고 Pod이 인증서와 함께 준비될 때까지 기다립니다. 자세한 내용은 [러너 API 권한 구성](../executors/kubernetes/_index.md#configure-runner-api-permissions)을 참조하세요. |
| `FF_MASK_ALL_DEFAULT_TOKENS` | `true` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 GitLab 러너는 모든 기본 토큰 패턴을 자동으로 마스킹합니다. |
| `FF_EXPORT_HIGH_CARDINALITY_METRICS` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 러너는 높은 카디널리티를 가진 메트릭을 내보냅니다. 이 기능 플래그를 활성화할 때 많은 양의 데이터를 수집하지 않도록 특별히 주의해야 합니다. 자세한 내용은 [플릿 스케일링](../fleet_scaling/_index.md)을 참조하세요. |
| `FF_USE_FLEETING_ACQUIRE_HEARTBEATS` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 작업이 인스턴스에 할당되기 전에 fleeting 인스턴스 연결을 확인합니다. |
| `FF_USE_EXPONENTIAL_BACKOFF_STAGE_RETRY` | `true` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 `GET_SOURCES_ATTEMPTS`, `ARTIFACT_DOWNLOAD_ATTEMPTS`, `RESTORE_CACHE_ATTEMPTS`, `EXECUTOR_JOB_SECTION_ATTEMPTS`에 대한 재시도가 지수 백오프(5초 - 5분)를 사용합니다. |
| `FF_USE_ADAPTIVE_REQUEST_CONCURRENCY` | `true` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 `request_concurrency` 설정이 최대 동시성 값이 되고, 동시 요청의 수가 성공적인 작업 요청의 속도를 기반으로 조정됩니다. |
| `FF_USE_GITALY_CORRELATION_ID` | `true` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 `X-Gitaly-Correlation-ID` 헤더가 모든 Git HTTP 요청에 추가됩니다. 비활성화되면 Git 작업이 Gitaly 상관 관계 ID 헤더 없이 실행됩니다. |
| `FF_USE_GIT_PROACTIVE_AUTH` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 러너는 `git clone` 및 `git fetch` 명령에 `http.proactiveAuth=basic` Git 구성 옵션을 전달합니다. 결과적으로 Git은 `401` 응답을 기다리는 대신 적극적으로 자격증명을 전송합니다. 이 동작은 공개 프로젝트의 경우 Gitaly에 사용자 이름이 전파되도록 합니다. |
| `FF_HASH_CACHE_KEYS` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | GitLab 러너가 캐시를 생성하거나 추출할 때 로컬 및 분산 캐시(예: S3)에 모두 사용하기 전에 캐시 키를 해시(SHA256)합니다. 자세한 내용은 [캐시 키 처리](advanced-configuration.md#cache-key-handling)를 참조하세요. |
| `FF_ENABLE_JOB_INPUTS_INTERPOLATION` | `true` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 작업 입력이 보간됩니다. 자세한 내용은 [&17833](https://gitlab.com/groups/gitlab-org/-/epics/17833)을 참조하세요. |
| `FF_USE_JOB_ROUTER` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | GitLab 러너가 GitLab에 직접 연결하는 대신 작업 라우터에 연결하여 작업을 가져옵니다. |
| `FF_SCRIPT_TO_STEP_MIGRATION` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 사용자 스크립트가 단계로 마이그레이션되고 단계 실행기로 실행됩니다. |
| `FF_USE_PARALLEL_CACHE_TRANSFER` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 캐시 업로드 및 다운로드는 병렬 객체 저장소 전송을 사용합니다:  GoCloud 쓰기는 동시 부분과 함께 다중 부분을 사용합니다. 다운로드는 동시 HTTP 범위 또는 GoCloud 범위 읽기를 사용합니다. 비활성화되면 업로드는 단일 동시 부분 스트림을 사용하고 다운로드는 하나의 스트림을 사용합니다. 활성화된 경우 높은 대역폭 링크의 처리량이 향상됩니다. `CACHE_CONCURRENCY` 및 `CACHE_CHUNK_SIZE`로 조정합니다. |
| `FF_USE_PARALLEL_ARTIFACT_TRANSFER` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 `direct_download`을 사용하고 객체 저장소로의 리디렉션을 수신하는 아티팩트 다운로드는 백엔드가 `Content-Range` 합계로 `206 Partial Content`을 지원할 때 병렬 HTTP 범위 GET을 사용할 수 있습니다. 비활성화되면 단일 다운로드 스트림이 사용됩니다. 청크 크기 및 동시성은 러너에서 고정됩니다(`CACHE_*` 변수 아님). |
| `FF_CONCRETE` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 기존 스크립트 실행이 단계 실행기로 마이그레이션되고 실행됩니다. |
| `FF_SUSPENDABLE_ENVIRONMENTS` | `false` | {{< icon name="dotted-circle" >}} 아니요 |  | 활성화되면 작업 환경을 일시 중단하거나 다시 시작할 수 있습니다. |

<!-- feature_flags_list_end -->

## 파이프라인 구성에서 기능 플래그 활성화 {#enable-feature-flag-in-pipeline-configuration}

[CI/CD 변수](https://docs.gitlab.com/ci/variables/)를 사용하여 기능 플래그를 활성화할 수 있습니다:

- 파이프라인의 모든 작업(전역):

  ```yaml
  variables:
    FEATURE_FLAG_NAME: 1
  ```

- 단일 작업:

  ```yaml
  job:
    stage: test
    variables:
      FEATURE_FLAG_NAME: 1
    script:
    - echo "Hello"
  ```

## 러너 환경 변수에서 기능 플래그 활성화 {#enable-feature-flag-in-runner-environment-variables}

러너가 실행하는 모든 작업에 대해 기능을 활성화하려면 [Runner 구성](advanced-configuration.md) 에서 [`environment`](advanced-configuration.md#the-runners-section) 변수로 기능 플래그를 지정합니다:

```toml
[[runners]]
  name = "example-runner"
  url = "https://gitlab.com/"
  token = "TOKEN"
  limit = 0
  executor = "docker"
  builds_dir = ""
  shell = ""
  environment = ["FEATURE_FLAG_NAME=1"]
```

## 러너 구성에서 기능 플래그 활성화 {#enable-feature-flag-in-runner-configuration}

`[runners.feature_flags]` 아래에서 지정하여 기능 플래그를 활성화할 수 있습니다. 이 설정은 모든 작업이 기능 플래그 값을 재정의하는 것을 방지합니다.

일부 기능 플래그는 이 설정을 구성할 때만 사용할 수 있습니다. 작업을 실행하는 방법과는 관계가 없기 때문입니다.

```toml
[[runners]]
  name = "example-runner"
  url = "https://gitlab.com/"
  token = "TOKEN"
  executor = "docker"
  [runners.feature_flags]
    FF_USE_DIRECT_DOWNLOAD = true
```
