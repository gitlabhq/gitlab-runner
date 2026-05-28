---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: CI/CD 작업을 위한 소프트웨어입니다.
title: GitLab 러너 설치
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

[GitLab 러너](https://gitlab.com/gitlab-org/gitlab-runner)는 GitLab에 정의된 CI/CD 작업을 실행합니다. GitLab 러너는 단일 바이너리로 실행할 수 있으며 언어별 요구 사항이 없습니다.

보안 및 성능상의 이유로, GitLab 인스턴스를 호스팅하는 머신과 별도의 머신에 GitLab 러너를 설치하세요.

설치하기 전에 [시스템 요구 사항 및 지원되는 플랫폼](requirements.md)을 검토하세요.

## 운영 체제 {#operating-systems}

{{< cards >}}

- [Linux](linux-repository.md)
- [Linux 수동 설치](linux-manually.md)
- [FreeBSD](freebsd.md)
- [macOS](osx.md)
- [Windows](windows.md)
- [z/OS](z-os.md)

{{< /cards >}}

## 컨테이너 {#containers}

{{< cards >}}

- [Docker](docker.md)
- [Helm 차트](kubernetes.md)
- [GitLab 에이전트](kubernetes-agent.md)
- [Operator](operator.md)

{{< /cards >}}

## 기타 설치 옵션 {#other-installation-options}

{{< cards >}}

- [최신 릴리스](bleeding-edge.md)

{{< /cards >}}
