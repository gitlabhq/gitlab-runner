---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab 러너의 시스템 서비스
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab 러너는 [Go `service` 라이브러리](https://github.com/kardianos/service)를 사용하여 기본 OS를 감지하고 init 시스템을 기반으로 서비스 파일을 설치합니다.

> [!note]
> `service` 패키지는 프로그램을 서비스(데몬)로 설치, 제거, 시작, 중지 및 실행합니다. Windows XP+, Linux(systemd, Upstart, System V) 및 macOS(`launchd`)가 지원됩니다.

GitLab 러너를 [설치](../install/_index.md)하면 서비스 파일이 자동으로 생성됩니다:

- **systemd**: `/etc/systemd/system/gitlab-runner.service`
- **Upstart**: `/etc/init/gitlab-runner`

## 사용자 지정 환경 변수 설정 {#setting-custom-environment-variables}

GitLab 러너를 사용자 지정 환경 변수로 실행할 수 있습니다. 예를 들어 `GOOGLE_APPLICATION_CREDENTIALS`을 러너의 환경에서 정의하려고 합니다. 이 작업은 [`environment` 구성 설정](advanced-configuration.md#the-runners-section)과는 다르며, 이 설정은 러너에서 실행하는 모든 작업에 자동으로 추가되는 변수를 정의합니다.

### systemd 사용자 지정 {#customizing-systemd}

systemd를 사용하는 러너의 경우 `/etc/systemd/system/gitlab-runner.service.d/env.conf`을 만들고 내보낼 각 변수에 대해 `Environment=key=value` 줄을 하나씩 사용합니다.

예를 들어:

```toml
[Service]
Environment=GOOGLE_APPLICATION_CREDENTIALS=/etc/gitlab-runner/gce-credentials.json
```

그런 다음 구성을 다시 로드합니다:

```shell
systemctl daemon-reload
systemctl restart gitlab-runner.service
```

### Upstart 사용자 지정 {#customizing-upstart}

Upstart를 사용하는 러너의 경우 `/etc/init/gitlab-runner.override`을 만들고 원하는 변수를 내보냅니다.

예를 들어:

```shell
export GOOGLE_APPLICATION_CREDENTIALS="/etc/gitlab-runner/gce-credentials.json"
```

이 변경 사항을 적용하려면 러너를 다시 시작합니다.

## 기본 중지 동작 재정의 {#overriding-default-stopping-behavior}

경우에 따라 서비스의 기본 동작을 재정의하려고 할 수 있습니다.

예를 들어 GitLab 러너를 업그레이드할 때 모든 실행 중인 작업이 완료될 때까지 정상적으로 중지해야 합니다. 하지만 systemd, Upstart 또는 다른 서비스가 즉시 프로세스를 다시 시작할 수도 있습니다.

따라서 GitLab 러너를 업그레이드할 때 설치 스크립트는 아마도 당시에 새 작업을 처리하고 있던 러너 프로세스를 종료했다가 다시 시작합니다.

### systemd 재정의 {#overriding-systemd}

systemd를 사용하는 러너의 경우 다음 콘텐츠로 `/etc/systemd/system/gitlab-runner.service.d/kill.conf`을 만듭니다:

```toml
[Service]
TimeoutStopSec=7200
KillSignal=SIGQUIT
```

systemd 단위 구성에 이 두 설정을 추가한 후 러너를 중지할 수 있습니다. 러너가 중지되면 systemd는 `SIGQUIT`을 프로세스 중지 신호로 사용합니다. 또한 중지 명령에 2시간 시간 초과가 설정됩니다. 이 시간 초과 전에 어떤 작업도 정상적으로 종료되지 않으면 systemd는 `SIGKILL`를 사용하여 프로세스를 종료합니다.

### Upstart 재정의 {#overriding-upstart}

Upstart를 사용하는 러너의 경우 다음 콘텐츠로 `/etc/init/gitlab-runner.override`을 만듭니다:

```shell
kill signal SIGQUIT
kill timeout 7200
```

Upstart 단위 구성에 이 두 설정을 추가한 후 러너를 중지할 수 있습니다. Upstart는 위의 systemd와 동일하게 작동합니다.
