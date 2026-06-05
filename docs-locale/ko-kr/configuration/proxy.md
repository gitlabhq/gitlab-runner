---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 프록시 뒤에서 GitLab 러너 실행
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

이 가이드는 프록시 뒤에서 Docker 실행기를 사용하여 GitLab 러너가 작동하도록 하는 방법을 구체적으로 설명합니다.

계속하기 전에 동일한 머신에 [Docker를 설치](https://docs.docker.com/get-started/get-docker/) 하고 [GitLab 러너](../install/_index.md)를 설치했는지 확인하세요.

## `cntlm` 구성 {#configuring-cntlm}

> [!note]
> 인증 없이 프록시를 이미 사용 중인 경우, 이 섹션은 선택 사항이며 [Docker 구성](#configuring-docker-for-downloading-images)으로 바로 이동할 수 있습니다. `cntlm` 구성은 인증이 필요한 프록시 뒤에 있는 경우에만 필요하지만, 어떤 경우든 사용하는 것이 좋습니다.

[`cntlm`](https://github.com/versat/cntlm)는 로컬 프록시로 사용할 수 있는 Linux 프록시이며, 프록시 세부 정보를 수동으로 모든 곳에 추가하는 것과 비교할 때 2가지 주요 이점이 있습니다:

- 자격 증명을 변경해야 하는 단일 소스
- Docker 러너에서 자격 증명에 액세스할 수 없습니다

[`cntlm`을 설치](https://www.howtoforge.com/linux-ntlm-authentication-proxy-isa-server-with-cntlm)했다고 가정하면, 먼저 구성해야 합니다.

### `cntlm`이 `docker0` 인터페이스를 수신하도록 설정 {#make-cntlm-listen-to-the-docker0-interface}

추가 보안 및 인터넷으로부터의 보호를 위해 `cntlm`을 `docker0` 인터페이스에서 수신하도록 바인드합니다. 이 인터페이스는 컨테이너가 도달할 수 있는 IP 주소를 가집니다. `cntlm`을 Docker 호스트에서 이 주소로만 바인드하도록 지시하면, Docker 컨테이너는 이에 도달할 수 있지만 외부 세계는 할 수 없습니다.

1. Docker가 사용 중인 IP를 찾습니다:

   ```shell
   ip -4 -oneline addr show dev docker0
   ```

   IP 주소는 일반적으로 `172.17.0.1`이며, 이를 `docker0_interface_ip`이라고 하겠습니다.

1. `cntlm`의 구성 파일 (`/etc/cntlm.conf`)을 엽니다. 사용자 이름, 암호, 도메인 및 프록시 호스트를 입력하고 이전 단계에서 찾은 `Listen` IP 주소를 구성합니다. 다음과 같이 표시되어야 합니다:

   ```plaintext
   Username     testuser
   Domain       corp-uk
   Password     password
   Proxy        10.0.0.41:8080
   Proxy        10.0.0.42:8080
   Listen       172.17.0.1:3128 # Change to your docker0 interface IP
   ```

1. 변경 사항을 저장하고 서비스를 다시 시작합니다:

   ```shell
   sudo systemctl restart cntlm
   ```

## 다운로드를 위해 Docker 구성 {#configuring-docker-for-downloading-images}

> [!note]
> 다음은 systemd 지원이 있는 OS에 적용됩니다.

프록시 사용 방법에 대한 자세한 내용은 [Docker 설명서](https://docs.docker.com/engine/daemon/proxy/)를 참조하세요.

서비스 파일은 다음과 같아야 합니다:

```ini
[Service]
Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
```

## GitLab 러너 구성에 프록시 변수 추가 {#adding-proxy-variables-to-the-gitlab-runner-configuration}

프록시 변수를 GitLab 러너 구성에도 추가해야 하므로 프록시 뒤에서 GitLab.com에 연결할 수 있습니다.

이 작업은 위의 Docker 서비스에 프록시를 추가하는 것과 동일합니다:

1. `gitlab-runner` 서비스용 systemd drop-in 디렉터리를 생성합니다:

   ```shell
   mkdir /etc/systemd/system/gitlab-runner.service.d
   ```

1. `/etc/systemd/system/gitlab-runner.service.d/http-proxy.conf` 파일을 생성하고 `HTTP_PROXY` 환경 변수를 추가합니다:

   ```ini
   [Service]
   Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
   Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
   ```

   GitLab 러너를 GitLab Self-Managed 인스턴스와 같은 내부 URL에 연결하려면 `NO_PROXY` 환경 변수의 값을 설정합니다.

   ```ini
   [Service]
   Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
   Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
   Environment="NO_PROXY=gitlab.example.com"
   ```

1. 파일을 저장하고 변경 사항을 플러시합니다:

   ```shell
   systemctl daemon-reload
   ```

1. GitLab 러너를 다시 시작합니다:

   ```shell
   sudo systemctl restart gitlab-runner
   ```

1. 구성이 로드되었는지 확인합니다:

   ```shell
   systemctl show --property=Environment gitlab-runner
   ```

   다음을 표시해야 합니다:

   ```ini
   Environment=HTTP_PROXY=http://docker0_interface_ip:3128/ HTTPS_PROXY=http://docker0_interface_ip:3128/
   ```

## Docker 컨테이너에 프록시 추가 {#adding-the-proxy-to-the-docker-containers}

[러너를 등록](../register/_index.md)한 후에는 프록시 설정을 Docker 컨테이너에 전파할 수 있습니다(예: `git clone`의 경우).

이를 수행하려면 `/etc/gitlab-runner/config.toml`을 편집하고 `[[runners]]` 섹션에 다음을 추가해야 합니다:

```toml
pre_get_sources_script = "git config --global http.proxy $HTTP_PROXY; git config --global https.proxy $HTTPS_PROXY"
environment = ["https_proxy=http://docker0_interface_ip:3128", "http_proxy=http://docker0_interface_ip:3128", "HTTPS_PROXY=docker0_interface_ip:3128", "HTTP_PROXY=docker0_interface_ip:3128"]
```

`docker0_interface_ip`는 `docker0` 인터페이스의 IP 주소입니다.

> [!note]
> 이 예제에서 특정 프로그램이 `HTTP_PROXY`을 예상하고 다른 프로그램이 `http_proxy`을 예상하기 때문에 소문자와 대문자 변수를 모두 설정합니다. 안타깝게도 이러한 종류의 환경 변수에 대한 [표준](https://unix.stackexchange.com/questions/212894/whats-the-right-format-for-the-http-proxy-environment-variable-caps-or-no-ca#212972)이 없습니다.

## `dind` 서비스 사용 시 프록시 설정 {#proxy-settings-when-using-dind-service}

[Docker-in-Docker 실행기](https://docs.gitlab.com/ci/docker/using_docker_build/#use-docker-in-docker) (`dind`)를 사용할 때 `docker:2375,docker:2376`을 `NO_PROXY` 환경 변수에 지정해야 할 수 있습니다. 포트가 필요합니다. 그렇지 않으면 `docker push`가 차단됩니다.

`dockerd`(from `dind`) 및 로컬 `docker` 클라이언트 간의 통신 (여기에 설명: <https://hub.docker.com/_/docker/>) root의 Docker 구성에 저장된 프록시 변수를 사용합니다.

이를 구성하려면 `/root/.docker/config.json`을 편집하여 완전한 프록시 구성을 포함해야 합니다. 예:

```json
{
    "proxies": {
        "default": {
            "httpProxy": "http://proxy:8080",
            "httpsProxy": "http://proxy:8080",
            "noProxy": "docker:2375,docker:2376"
        }
    }
}
```

Docker 실행기의 컨테이너에 설정을 전달하려면 `$HOME/.docker/config.json`도 컨테이너 내부에 생성해야 합니다. 이는 `before_script`로 `.gitlab-ci.yml`에서 스크립트할 수 있습니다. 예:

```yaml
before_script:
  - mkdir -p $HOME/.docker/
  - 'echo "{ \"proxies\": { \"default\": { \"httpProxy\": \"$HTTP_PROXY\", \"httpsProxy\": \"$HTTPS_PROXY\", \"noProxy\": \"$NO_PROXY\" } } }" > $HOME/.docker/config.json'
```

또는 영향을 받는 `gitlab-runner` (`/etc/gitlab-runner/config.toml`)의 구성에서:

```toml
[[runners]]
  pre_build_script = "mkdir -p $HOME/.docker/ && echo \"{ \\\"proxies\\\": { \\\"default\\\": { \\\"httpProxy\\\": \\\"$HTTP_PROXY\\\", \\\"httpsProxy\\\": \\\"$HTTPS_PROXY\\\", \\\"noProxy\\\": \\\"$NO_PROXY\\\" } } }\" > $HOME/.docker/config.json"
```

> [!note]
> 추가 수준의 `"` 이스케이핑이 필요합니다. 이는 TOML 파일 내의 단일 문자열로 지정된 셸이 있는 JSON 파일을 생성하기 때문입니다. 이것은 YAML이 아니므로 `:`을 이스케이프하지 마세요.

`NO_PROXY` 목록을 확장해야 하는 경우 와일드카드 `*`는 접미사에서만 작동하며 접두사 또는 CIDR 표기법에서는 작동하지 않습니다. 자세한 내용은 <https://github.com/moby/moby/issues/9145> 및 <https://unix.stackexchange.com/questions/23452/set-a-network-range-in-the-no-proxy-environment-variable>를 참조하세요.

## 속도 제한 요청 처리 {#handling-rate-limited-requests}

GitLab 인스턴스는 남용을 방지하기 위해 API 요청에 대한 속도 제한이 있는 역방향 프록시 뒤에 있을 수 있습니다. GitLab 러너는 API에 여러 요청을 보내고 이러한 속도 제한을 초과할 수 있습니다.

결과적으로 GitLab 러너는 다음 [재시도 로직](#retry-logic)을 사용하여 속도 제한 시나리오를 처리합니다:

### 재시도 로직 {#retry-logic}

GitLab 러너가 `429 Too Many Requests` 응답을 받으면 다음 재시도 시퀀스를 따릅니다:

1. 러너는 `RateLimit-ResetTime` 헤더의 응답 헤더를 확인합니다.
   - `RateLimit-ResetTime` 헤더는 유효한 HTTP 날짜(RFC1123)인 `Wed, 21 Oct 2015 07:28:00 GMT`와 같은 값을 가져야 합니다.
   - 헤더가 있고 유효한 값을 가지면 러너는 지정된 시간까지 기다렸다가 다른 요청을 발급합니다.
1. `RateLimit-ResetTime` 헤더가 유효하지 않거나 누락된 경우 러너는 `Retry-After` 헤더의 응답 헤더를 확인합니다.
   - `Retry-After` 헤더는 `Retry-After: 30`와 같은 초 단위 형식의 값을 가져야 합니다.
   - 헤더 형식이 있고 유효한 값을 가지면 러너는 지정된 시간까지 기다렸다가 다른 요청을 발급합니다.
1. 두 헤더가 모두 누락되거나 유효하지 않으면 러너는 기본 간격까지 기다렸다가 다른 요청을 발급합니다.

러너는 실패한 요청을 최대 5회까지 재시도합니다. 모든 재시도가 실패하면 러너는 최종 응답에서 오류를 기록합니다.

### 지원되는 헤더 형식 {#supported-header-formats}

| 헤더                | 형식              | 예                         |
|-----------------------|---------------------|---------------------------------|
| `RateLimit-ResetTime` | HTTP 날짜(RFC1123) | `Wed, 21 Oct 2015 07:28:00 GMT` |
| `Retry-After`         | 초             | `30`                            |

> [!note]
> `RateLimit-ResetTime` 헤더는 모든 헤더 키가 [`http.CanonicalHeaderKey`](https://pkg.go.dev/net/http#CanonicalHeaderKey) 함수를 통해 실행되기 때문에 대소문자를 구분하지 않습니다.
