---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: z/OS에서 러너를 수동으로 설치합니다.
title: z/OS에서 러너를 수동으로 설치
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

IBM z/OS용 러너는 GitLab에서 인증되었으며 z/OS 메인프레임 환경에서 CI/CD 작업을 기본적으로 실행할 수 있습니다.

[`pax`](https://www.ibm.com/docs/en/aix/7.1.0?topic=p-pax-command) 아카이브에서 z/OS에 러너를 수동으로 다운로드하고 설치할 수 있습니다.

## 필수 요구 사항 {#prerequisites}

- 러너를 사용하려면 프로그램 임시 수정(`PTFs`)이 포함된 다음 권한 있는 프로그램 분석 보고서(`APARs`)가 필요합니다:
  - z/OS 2.5
    - OA62757
    - PH45182
  - z/OS 3.1
    - OA62757
    - PH57159
- 러너는 bash가 `/bin/bash`에 설치되어 있어야 셸 명령을 실행할 수 있습니다. bash가 이 위치에 설치되지 않았으면 설치된 버전으로 심볼릭 링크를 생성합니다:

  ```shell
  ln -s <TARGET_BASH> /bin/bash
  ```

## 러너 설치 {#install-gitlab-runner}

러너를 설치하려면:

1. `paxfile`을 선택한 설치 디렉터리로 다운로드합니다.
1. 시스템용 패키지를 설치합니다:

   ```shell
   pax -ppx -rf gitlab-runner-<VERSION>.pax.Z
   ```

   설치된 파일은 설치 위치의 `gitlab-runner` 디렉터리로 압축이 해제됩니다.

1. 파일 권한을 실행으로 설정합니다:

   ```shell
   chmod +x <INSTALL_PATH>/bin/gitlab-runner
   ```

1. 러너를 내보내고 `PATH`에 추가합니다:

   ```shell
   export GITLAB_RUNNER=<INSTALL_PATH>/gitlab-runner/bin
   export PATH=${GITLAB_RUNNER}:${PATH}
   ```

1. [러너 등록](../register/_index.md)합니다.

## 러너 실행 {#run-gitlab-runner}

러너를 직접 실행하거나 시작된 작업으로 실행할 수 있습니다.

### 러너 직접 실행 {#run-gitlab-runner-directly}

실행 파일을 호출하여 러너를 실행하려면:

1. `<INSTALL_PATH>/bin` 디렉터리로 이동합니다.
1. 서비스를 시작합니다:

   ```shell
   gitlab-runner start
   ```

### 러너를 시작된 작업으로 실행 {#run-gitlab-runner-as-a-started-task}

러너 작업을 계속 사용 가능하게 하려면 시작된 작업으로 실행합니다.

1. 실행 파일을 `gitlab-runner.sh` 셸 스크립트로 래핑합니다:

   ```shell
   #! /bin/sh
   <INSTALL_PATH>/bin/gitlab-runner start
   ```

1. `jcl` 시작된 작업 프로그램을 정의하고 이를 실행하여 지속적인 작업으로 실행합니다:

   ```jcl
   //GLRST  PROC CNFG='<PATH_TO_SCRIPT>'
   //*
   //GLRST  EXEC PGM=BPXBATSL,REGION=0M,TIME=NOLIMIT,
   //            PARM='PGM &CNFG./gitlab-runner.sh'
   //STDOUT   DD SYSOUT=*
   //STDERR   DD SYSOUT=*
   //*
   //        PEND
   ```
