---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 자체 서명된 인증서 또는 사용자 지정 인증 기관
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab 러너는 TLS 피어를 확인하는 데 사용할 인증서를 구성하는 두 가지 옵션을 제공합니다:

- **For connections to the GitLab server**:  인증서 파일은 [GitLab 서버를 대상으로 하는 자체 서명된 인증서의 지원되는 옵션](#supported-options-for-self-signed-certificates-targeting-the-gitlab-server) 섹션에서 자세히 설명한 대로 지정할 수 있습니다.

  이는 러너를 등록할 때 `x509: certificate signed by unknown authority` 문제를 해결합니다.

  기존 러너의 경우, 러너 로그에서 작업을 확인하려고 할 때 동일한 오류를 볼 수 있습니다:

  ```plaintext
  Couldn't execute POST against https://hostname.tld/api/v4/jobs/request:
  Post https://hostname.tld/api/v4/jobs/request: x509: certificate signed by unknown authority
  ```

- **Connecting to a cache server or an external Git LFS store**:  사용자 스크립트와 같은 다른 시나리오도 포함하는 보다 일반적인 접근 방식으로, 인증서를 지정하고 [Docker 및 Kubernetes 실행기에 대한 TLS 인증서 신뢰](#trusting-tls-certificates-for-docker-and-kubernetes-executors) 섹션에서 자세히 설명한 대로 컨테이너에 설치할 수 있습니다.

  인증서가 누락된 Git LFS 작업과 관련된 예제 작업 로그 오류:

  ```plaintext
  LFS: Get https://object.hostname.tld/lfs-dev/c8/95/a34909dce385b85cee1a943788044859d685e66c002dbf7b28e10abeef20?X-Amz-Expires=600&X-Amz-Date=20201006T043010Z&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=svcgitlabstoragedev%2F20201006%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-SignedHeaders=host&X-Amz-Signature=012211eb0ff0e374086e8c2d37556f2d8ca4cc948763e90896f8f5774a100b55: x509: certificate signed by unknown authority
  ```

## GitLab 서버를 대상으로 하는 자체 서명된 인증서의 지원되는 옵션 {#supported-options-for-self-signed-certificates-targeting-the-gitlab-server}

이 섹션은 GitLab 서버만 사용자 지정 인증서가 필요한 경우를 나타냅니다. 다른 호스트(예: [프록시 다운로드 활성화](https://docs.gitlab.com/administration/object_storage/#proxy-download) 없는 객체 저장소 서비스)도 사용자 지정 인증 기관(CA)이 필요한 경우 [다음 섹션](#trusting-tls-certificates-for-docker-and-kubernetes-executors)을 참조하세요.

GitLab 러너는 다음 옵션을 지원합니다:

- **Default - Read the system certificate**:  GitLab 러너는 시스템 인증서 저장소를 읽고 시스템에 저장된 인증 기관(CA)에 대해 GitLab 서버를 확인합니다.

- **Specify a custom certificate file**:  GitLab 러너는 `tls-ca-file` 옵션을 [등록](../commands/_index.md#gitlab-runner-register) 중에(`gitlab-runner register --tls-ca-file=/path`) 노출하고, [`config.toml`](advanced-configuration.md)의 `[[runners]]` 섹션에서도 노출합니다. 이를 통해 사용자 지정 인증서 파일을 지정할 수 있습니다. 이 파일은 러너가 GitLab 서버에 액세스하려고 할 때마다 읽힙니다. GitLab 러너 Helm 차트를 사용하는 경우 [사용자 지정 인증서로 GitLab에 액세스](../install/kubernetes_helm_chart_configuration.md#access-gitlab-with-a-custom-certificate)에서 설명한 대로 인증서를 구성해야 합니다.

- **Read a PEM certificate**:  GitLab 러너는 미리 정의된 파일에서 PEM 인증서(**DER format is not supported**)를 읽습니다:
  - `/etc/gitlab-runner/certs/gitlab.example.com.crt`는 GitLab 러너가 `root`로 실행되는 \*nix 시스템에서 사용됩니다.

    서버 주소가 `https://gitlab.example.com:8443/`인 경우 인증서 파일을 `/etc/gitlab-runner/certs/gitlab.example.com.crt`에 만듭니다.

    `openssl` 클라이언트를 사용하여 GitLab 인스턴스의 인증서를 `/etc/gitlab-runner/certs`로 다운로드할 수 있습니다:

    ```shell
    openssl s_client -showcerts -connect gitlab.example.com:443 -servername gitlab.example.com < /dev/null 2>/dev/null | openssl x509 -outform PEM > /etc/gitlab-runner/certs/gitlab.example.com.crt
    ```

    파일이 올바르게 설치되었는지 확인하려면 `openssl`과 같은 도구를 사용할 수 있습니다. 예를 들어:

    ```shell
    echo | openssl s_client -CAfile /etc/gitlab-runner/certs/gitlab.example.com.crt -connect gitlab.example.com:443 -servername gitlab.example.com
    ```

  - `~/.gitlab-runner/certs/gitlab.example.com.crt`는 GitLab 러너가 non-`root`로 실행되는 \*nix 시스템에서 사용됩니다.
  - 다른 시스템에서 `./certs/gitlab.example.com.crt`. GitLab 러너를 Windows 서비스로 실행하는 경우 작동하지 않습니다. 대신 사용자 지정 인증서 파일을 지정하세요.

참고:

- GitLab 서버 인증서가 CA에 의해 서명된 경우 CA 인증서를 사용하세요(GitLab 서버 서명된 인증서가 아님). 중간 인증서를 체인에 추가해야 할 수도 있습니다. 예를 들어 기본, 중간 및 루트 인증서가 있는 경우 모두 하나의 파일에 넣을 수 있습니다:

  ```plaintext
  -----BEGIN CERTIFICATE-----
  (Your primary SSL certificate: your_domain_name.crt)
  -----END CERTIFICATE-----
  -----BEGIN CERTIFICATE-----
  (Your intermediate certificate)
  -----END CERTIFICATE-----
  -----BEGIN CERTIFICATE-----
  (Your root certificate)
  -----END CERTIFICATE-----
  ```

- 기존 러너의 인증서를 업데이트하는 경우 [다시 시작](../commands/_index.md#gitlab-runner-restart)하세요.
- 이미 HTTP를 통해 구성된 러너가 있는 경우 `config.toml`에서 GitLab 인스턴스의 새 HTTPS URL로 인스턴스 경로를 업데이트하세요.
- 임시 및 안전하지 않은 해결 방법으로 인증서 확인을 건너뛰려면 `.gitlab-ci.yml` 파일의 `variables:` 섹션에서 CI 변수 `GIT_SSL_NO_VERIFY`을 `true`로 설정하세요.

### Git 복제 {#git-cloning}

러너는 `CI_SERVER_TLS_CA_FILE`를 사용하여 CA 체인을 구축하기 위해 누락된 인증서를 주입합니다. 이를 통해 `git clone` 및 아티팩트는 공개적으로 신뢰할 수 있는 인증서를 사용하지 않는 서버에서 작동할 수 있습니다.

이 접근 방식은 안전하지만 러너를 단일 신뢰 지점으로 만듭니다.

## Docker 및 Kubernetes 실행기에 대한 TLS 인증서 신뢰 {#trusting-tls-certificates-for-docker-and-kubernetes-executors}

컨테이너에 인증서를 등록할 때 다음 정보를 고려하세요:

- [**user image**](https://docs.gitlab.com/ci/yaml/#image)는 사용자 스크립트를 실행하는 데 사용됩니다. 사용자 스크립트에 대한 인증서를 신뢰해야 하는 시나리오의 경우 사용자는 인증서 설치 방법에 관해 책임을 져야 합니다. 인증서 설치 절차는 이미지를 기반으로 달라질 수 있습니다. 러너는 각 시나리오에서 인증서를 설치하는 방법을 알 수 있는 방법이 없습니다.
- [**Runner helper image**](advanced-configuration.md#helper-image)는 Git, 아티팩트 및 캐시 작업을 처리하는 데 사용됩니다. 다른 CI/CD 스테이지에 대한 인증서를 신뢰해야 하는 시나리오의 경우 사용자는 특정 위치(예: `/etc/gitlab-runner/certs/ca.crt`)에서 인증서 파일을 사용 가능하게 만들기만 하면 되고, Docker 컨테이너가 사용자를 위해 자동으로 설치합니다.

### 사용자 스크립트에 대한 인증서 신뢰 {#trusting-the-certificate-for-user-scripts}

빌드에서 자체 서명된 인증서 또는 사용자 지정 인증서를 사용하는 TLS를 사용하는 경우 피어 통신을 위해 빌드 작업에 인증서를 설치하세요. 사용자 스크립트를 실행하는 Docker 컨테이너에는 기본적으로 설치된 인증서 파일이 없습니다. 이는 사용자 지정 캐시 호스트를 사용하거나 보조 `git clone`를 수행하거나 `wget`와 같은 도구를 통해 파일을 가져오는 데 필요할 수 있습니다.

인증서를 설치하려면:

1. 필요한 파일을 Docker 볼륨으로 매핑하여 스크립트를 실행하는 Docker 컨테이너가 이를 볼 수 있도록 하세요. `config.toml` 파일의 `[runners.docker]` 내에 있는 해당 키 내부에 볼륨을 추가하여 이를 수행하세요. 예:

   - **Linux**:

     ```toml
     [[runners]]
       name = "docker"
       url = "https://example.com/"
       token = "TOKEN"
       executor = "docker"

       [runners.docker]
          image = "ubuntu:latest"

          # Add path to your ca.crt file in the volumes list
          volumes = ["/cache", "/path/to-ca-cert-dir/ca.crt:/etc/gitlab-runner/certs/ca.crt:ro"]
     ```

1. **Linux-only**:  매핑된 파일(예: `ca.crt`)을 다음을 수행하는 [`pre_build_script`](advanced-configuration.md#the-runners-section)에서 사용하세요:
   1. Docker 컨테이너 내부의 `/usr/local/share/ca-certificates/ca.crt`로 복사합니다.
   1. `update-ca-certificates --fresh`을 실행하여 설치합니다. 예를 들어(사용 중인 배포판에 따라 명령이 다름):

      - Ubuntu에서:

        ```toml
        [[runners]]
          name = "docker"
          url = "https://example.com/"
          token = "TOKEN"
          executor = "docker"

          # Copy and install CA certificate before each job
          pre_build_script = """
          apt-get update -y > /dev/null
          apt-get install -y ca-certificates > /dev/null

          cp /etc/gitlab-runner/certs/ca.crt /usr/local/share/ca-certificates/ca.crt
          update-ca-certificates --fresh > /dev/null
          """
        ```

      - Alpine에서:

        ```toml
        [[runners]]
          name = "docker"
          url = "https://example.com/"
          token = "TOKEN"
          executor = "docker"

          # Copy and install CA certificate before each job
          pre_build_script = """
          apk update >/dev/null
          apk add ca-certificates > /dev/null
          rm -rf /var/cache/apk/*

          cp /etc/gitlab-runner/certs/ca.crt /usr/local/share/ca-certificates/ca.crt
          update-ca-certificates --fresh > /dev/null
          """
        ```

GitLab 서버 CA 인증서만 필요한 경우 `CI_SERVER_TLS_CA_FILE` 변수에 저장된 파일에서 이를 검색할 수 있습니다:

```shell
curl --cacert "${CI_SERVER_TLS_CA_FILE}"  ${URL} -o ${FILE}
```

### 다른 CI/CD 스테이지에 대한 인증서 신뢰 {#trusting-the-certificate-for-the-other-cicd-stages}

Linux에서는 `/etc/gitlab-runner/certs/ca.crt`로, Windows에서는 `C:\GitLab-Runner\certs\ca.crt`로 인증서 파일을 매핑할 수 있습니다. 러너 도우미 이미지는 시작 시 이 사용자 정의 `ca.crt` 파일을 설치하고 복제 및 아티팩트 업로드와 같은 작업을 수행할 때 이를 사용합니다.

#### Docker {#docker}

- **Linux**:

  ```toml
  [[runners]]
    name = "docker"
    url = "https://example.com/"
    token = "TOKEN"
    executor = "docker"

    [runners.docker]
      image = "ubuntu:latest"

      # Add path to your ca.crt file in the volumes list
      volumes = ["/cache", "/path/to-ca-cert-dir/ca.crt:/etc/gitlab-runner/certs/ca.crt:ro"]
  ```

- **Windows**:

  ```toml
  [[runners]]
    name = "docker"
    url = "https://example.com/"
    token = "TOKEN"
    executor = "docker"

    [runners.docker]
      image = "mcr.microsoft.com/windows/servercore:21H2"

      # Add directory holding your ca.crt file in the volumes list
      volumes = ["c:\\cache", "c:\\path\\to-ca-cert-dir:C:\\GitLab-Runner\\certs:ro"]
  ```

#### Kubernetes {#kubernetes}

Kubernetes에서 실행 중인 작업에 인증서 파일을 제공하려면:

1. 인증서를 네임스페이스의 Kubernetes 시크릿으로 저장하세요:

   ```shell
   kubectl create secret generic <SECRET_NAME> --namespace <NAMESPACE> --from-file=<CERT_FILE>
   ```

1. 러너에 시크릿을 볼륨으로 마운트하고 `<SECRET_NAME>` 및 `<LOCATION>`을 적절한 값으로 바꾸세요:

   ```toml
   gitlab-runner:
     runners:
      config: |
        [[runners]]
          [runners.kubernetes]
            namespace = "{{.Release.Namespace}}"
            image = "ubuntu:latest"
          [[runners.kubernetes.volumes.secret]]
              name = "<SECRET_NAME>"
              mount_path = "<LOCATION>"
   ```

   `mount_path`는 인증서가 저장되는 컨테이너의 디렉토리입니다. `/etc/gitlab-runner/certs/`을 `mount_path` 으로 사용했고 `ca.crt`를 인증서 파일로 사용한 경우 인증서는 컨테이너 내부의 `/etc/gitlab-runner/certs/ca.crt`에서 사용할 수 있습니다.
1. 작업의 일부로 매핑된 인증서 파일을 시스템 인증서 저장소에 설치하세요. 예를 들어 Ubuntu 컨테이너에서:

   ```yaml
   script:
     - cp /etc/gitlab-runner/certs/ca.crt /usr/local/share/ca-certificates/
     - update-ca-certificates
   ```

   Kubernetes 실행기의 도우미 이미지 `ENTRYPOINT`에 대한 처리에는 [알려진 문제](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28484)가 있습니다. 인증서 파일이 매핑되면 시스템 인증서 저장소에 자동으로 설치되지 않습니다.

## 문제 해결 {#troubleshooting}

일반 [SSL 문제 해결](https://docs.gitlab.com/omnibus/settings/ssl/ssl_troubleshooting/) 설명서를 참조하세요.

또한 [`tlsctl`](https://gitlab.com/gitlab-org/ci-cd/runner-tools/tlsctl) 도구를 사용하여 러너의 끝에서 GitLab 인증서를 디버그할 수 있습니다.

### 오류: `x509: certificate signed by unknown authority` {#error-x509-certificate-signed-by-unknown-authority}

이 오류는 개인 레지스트리에서 실행기 이미지를 끌어오려고 할 때 발생할 수 있습니다. 러너가 실행기를 예약하는 Docker 호스트 또는 Kubernetes 노드가 개인 레지스트리의 인증서를 신뢰하지 않는 경우입니다.

오류를 해결하려면 관련 루트 인증 기관 또는 인증서 체인을 시스템의 신뢰 저장소에 추가하고 컨테이너 서비스를 다시 시작하세요.

Ubuntu 또는 Alpine을 사용하는 경우 다음 명령을 실행하세요:

```shell
cp ca.crt /usr/local/share/ca-certificates/ca.crt
update-ca-certificates
systemctl restart docker.service
```

Ubuntu 또는 Alpine 이외의 운영 체제의 경우 운영 체제의 설명서를 참조하여 신뢰할 수 있는 인증서를 설치하는 적절한 명령을 찾으세요.

GitLab 러너 버전 및 Docker 호스트 환경에 따라 `FF_RESOLVE_FULL_TLS_CHAIN` 기능 플래그를 비활성화해야 할 수도 있습니다.

### `apt-get: not found` 작업 오류 {#apt-get-not-found-errors-in-jobs}

[`pre_build_script`](advanced-configuration.md#the-runners-section) 명령은 러너가 실행하는 모든 작업 전에 실행됩니다. `apk` 또는 `apt-get`와 같은 배포판 특정 명령은 문제를 일으킬 수 있습니다. 사용자 스크립트에 대한 인증서를 설치할 때 CI 작업이 다른 배포판을 기반으로 하는 [이미지](https://docs.gitlab.com/ci/yaml/#image)를 사용하는 경우 실패할 수 있습니다.

예를 들어 CI 작업이 Ubuntu 및 Alpine 이미지를 실행하는 경우 Ubuntu 명령이 Alpine에서 실패합니다. `apt-get: not found` 오류는 Alpine 기반 이미지를 사용하는 작업에서 발생합니다. 이 문제를 해결하려면 다음 중 하나를 수행하세요:

- 배포판 독립적이 되도록 `pre_build_script`를 작성하세요.
- [태그](https://docs.gitlab.com/ci/yaml/#tags)를 사용하여 러너가 호환되는 이미지의 작업만 선택하도록 하세요.

### 오류: `self-signed certificate in certificate chain` {#error-self-signed-certificate-in-certificate-chain}

CI/CD 작업이 다음 오류로 실패합니다:

```plaintext
fatal: unable to access 'https://gitlab.example.com/group/project.git/': SSL certificate problem: self-signed certificate in certificate chain
```

하지만 [OpenSSL 디버깅 명령](https://docs.gitlab.com/omnibus/settings/ssl/ssl_troubleshooting/#useful-openssl-debugging-commands)은 오류를 감지하지 못합니다.

이 오류는 Git이 `openssl s_client` 문제 해결 명령이 기본적으로 사용하지 않는 프록시를 통해 연결될 때 발생할 수 있습니다. Git이 프록시를 사용하여 리포지토리를 가져오는지 확인하려면 디버깅을 활성화하세요:

```yaml
variables:
  GIT_CURL_VERBOSE: 1
```

Git이 프록시를 사용하지 않도록 하려면 `NO_PROXY` 변수를 GitLab 호스트명을 포함하도록 설정하세요:

```yaml
variables:
  NO_PROXY: gitlab.example.com
```
