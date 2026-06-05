---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Parallels
---

Parallels 실행기는 [Parallels Desktop](https://www.parallels.com/) 가상화 소프트웨어를 사용하여 macOS의 가상 머신(VM)에서 CI/CD 작업을 실행합니다. Parallels Desktop은 Windows, Linux 및 기타 운영 체제를 macOS와 함께 실행할 수 있습니다.

Parallels 실행기는 VirtualBox 실행기와 유사하게 작동합니다. 가상 머신을 생성 및 관리하고 GitLab CI/CD 작업을 실행합니다. 각 작업은 깨끗한 VM 환경에서 실행되므로 빌드 간에 격리를 제공합니다. 구성 정보는 [VirtualBox 실행기](virtualbox.md)를 참조하세요.

> [!note]
> Parallels 실행기는 로컬 캐시를 지원하지 않습니다. [분산 캐시](../configuration/speed_up_job_execution.md)가 지원됩니다.
