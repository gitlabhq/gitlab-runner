---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: CI/CD 작업을 위한 소프트웨어입니다.
title: 시스템 요구 사항 및 지원되는 플랫폼
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

## 지원되는 운영 체제 {#supported-operating-systems}

GitLab Runner는 다음 환경에 설치할 수 있습니다:

- [GitLab 리포지토리](linux-repository.md) 에서 Linux를 설치하거나 [수동으로](linux-manually.md) 설치합니다
- [FreeBSD](freebsd.md)
- [macOS](osx.md)
- [Windows](windows.md)
- [z/OS](z-os.md)

[최신 버전 바이너리](bleeding-edge.md)도 사용 가능합니다.

다른 운영 체제를 사용하려면 해당 운영 체제가 Go 바이너리를 컴파일할 수 있는지 확인하세요.

## 지원되는 컨테이너 {#supported-containers}

GitLab Runner는 다음과 함께 설치할 수 있습니다:

- [Docker](docker.md)
- [GitLab Helm 차트](kubernetes.md)
- [Kubernetes용 GitLab 에이전트](kubernetes-agent.md)
- [GitLab Operator](operator.md)

## 지원되는 아키텍처 {#supported-architectures}

GitLab Runner는 다음 아키텍처에서 사용 가능합니다:

- x86
- AMD64
- ARM64
- ARM
- s390x
- ppc64le
- riscv64
- loong64

## 시스템 요구 사항 {#system-requirements}

GitLab Runner의 시스템 요구 사항은 다음 고려 사항에 따라 결정됩니다:

- CI/CD 작업의 예상 CPU 부하
- CI/CD 작업의 예상 메모리 사용량
- 동시 CI/CD 작업 수
- 활성 개발 중인 프로젝트 수
- 병렬로 작업할 것으로 예상되는 개발자 수

GitLab.com에서 사용 가능한 머신 타입에 대한 자세한 정보는 [GitLab 호스팅 러너](https://docs.gitlab.com/ci/runners/)를 참조하세요.

## FIPS 호환 GitLab Runner {#fips-compliant-gitlab-runner}

FIPS 140-2를 준수하는 GitLab Runner 바이너리는 Red Hat Enterprise Linux (RHEL) 배포판 및 AMD64 아키텍처에서 사용 가능합니다. 다른 배포판 및 아키텍처에 대한 지원은 [이슈 28814](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28814)에서 제안됩니다.

이 바이너리는 [Red Hat Go 컴파일러](https://developers.redhat.com/blog/2019/06/24/go-and-fips-140-2-on-red-hat-enterprise-linux)로 빌드되며 FIPS 140-2 검증된 암호화 라이브러리를 호출합니다. [UBI-8 최소 이미지](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/8/html-single/building_running_and_managing_containers/index#con_understanding-the-ubi-minimal-images_assembly_types-of-container-images)는 GitLab Runner FIPS 이미지를 생성하기 위한 기본으로 사용됩니다.

RHEL에서 FIPS 호환 GitLab Runner 사용에 대한 자세한 정보는 [RHEL을 FIPS 모드로 전환](https://docs.redhat.com/en/documentation/red_hat_enterprise_linux/8/html/security_hardening/switching-rhel-to-fips-mode_security-hardening)을 참조하세요.
