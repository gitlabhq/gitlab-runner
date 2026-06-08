---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: 러너 지원 정책
---

러너의 지원 정책은 운영 체제의 수명 주기 정책에 따라 결정됩니다.

## 컨테이너 이미지 지원 {#container-images-support}

저희는 러너 컨테이너 이미지를 생성하기 위해 사용되는 기본 배포판(Ubuntu, Alpine, Red Hat Universal Base Image)의 지원 수명 주기를 따릅니다.

기본 배포판의 배포 종료 날짜가 GitLab 메이저 릴리스 주기와 반드시 일치하지는 않습니다. 이는 저희가 러너 컨테이너 이미지의 버전을 마이너 릴리스에서 배포 중단할 것을 의미합니다. 이를 통해 업스트림 배포판에서 더 이상 업데이트하지 않는 이미지를 배포하지 않습니다.

### 컨테이너 이미지 및 배포 종료 날짜 {#container-images-and-end-of-publishing-date}

| 기본 컨테이너                 | 기본 컨테이너 버전 | 공급업체 EOL 날짜 | GitLab EOL 날짜 |
|--------------------------------|------------------------|-----------------|-----------------|
| Ubuntu                         | 24.04                  | 2027-04-30      | 2027-05-20      |
| Ubuntu                         | 20.04                  | 2025-05-31      | 2025-06-19      |
| Alpine                         | 3.12                   | 2022-05-01      | 2023-05-22      |
| Alpine                         | 3.13                   | 2022-11-01      | 2023-05-22      |
| Alpine                         | 3.14                   | 2023-05-01      | 2023-05-22      |
| Alpine                         | 3.15                   | 2023-11-01      | 2024-01-18      |
| Alpine                         | 3.16                   | 2024-05-23      | 2024-06-22      |
| Alpine                         | 3.17                   | 2024‑11‑22      | 2024-12-22      |
| Alpine                         | 3.18                   | 2025‑05‑09      | 2025-05-22      |
| Alpine                         | 3.19                   | 2025‑11‑01      | 2025-11-22      |
| Alpine                         | 3.21                   | 2026‑11‑01      | 2026-11-22      |
| Alpine                         | latest                 |                 |                 |
| Red Hat Universal Base Image 9 | 9.5                    | 2025-04-31      | 2025-05-22      |

러너 버전 17.7 이상은 특정 버전 대신 단일 Alpine 버전(`latest`)만 지원합니다. Alpine 버전 3.21은 명시된 EOL 날짜까지 지원됩니다. 이와 달리 Ubuntu 24.04는 EOL 날짜까지 지원되며, 그 시점에 저희는 가장 최근의 LTS 릴리스로 이동할 것입니다.

## Windows 버전 지원 {#windows-version-support}

GitLab은 공식적으로 Microsoft Windows 운영 체제의 LTS 버전을 지원하므로 저희는 Microsoft [Servicing Channels](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#servicing-channels) 수명 주기 정책을 따릅니다.

이는 저희가 다음을 지원한다는 것을 의미합니다:

- [Long-Term Servicing Channel](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#long-term-servicing-channel) 버전은 릴리스 날짜 이후 5년 동안 지원됩니다.

  5년 후에 Microsoft는 추가 5년 동안 연장 지원을 제공합니다. 이 연장 기간 동안 저희는 실질적인 범위 내에서 계속 지원을 제공합니다. 저희는 GitLab 메이저 릴리스에서 공지 후 이 지원을 종료할 수 있습니다.
- [Semi-Annual Channel](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#semi-annual-channel) 버전은 릴리스 날짜 이후 18개월 동안 지원됩니다. 저희는 주류 지원이 종료된 후 이러한 버전을 지원하지 않습니다.

이 지원 정책은 저희가 배포하는 [Windows binaries](windows.md#installation) 및 [Docker 실행기](../executors/docker.md#supported-windows-versions)에 적용됩니다.

> [!note]
> Docker 실행기는 Windows 컨테이너에 대해 엄격한 버전 요구 사항이 있습니다. 컨테이너가 호스트 OS의 버전과 일치해야 하기 때문입니다. 자세한 내용은 [지원되는 Windows 컨테이너 목록](../executors/docker.md#supported-windows-versions)을 참조하세요.

유일한 정보 출처로 저희는 <https://learn.microsoft.com/en-us/lifecycle/products/>를 사용하며, 여기에는 릴리스, 주류 및 연장 지원 날짜가 명시되어 있습니다.

다음은 일반적으로 사용되는 버전과 그 수명 종료 날짜의 목록입니다:

| 운영 체제           | 주류 지원 종료 날짜 | 연장 지원 종료 날짜 |
|----------------------------|-----------------------------|---------------------------|
| Windows Server 2019 (1809) | 2024년 1월                | 2029년 1월              |
| Windows Server 2022 (21H2) | 2026년 10월                | 2031년 10월              |
| Windows Server 2025 (24H2) | 2029년 10월                | 2034년 10월              |

### 향후 릴리스 {#future-releases}

Microsoft는 [Semi-Annual Channel](https://learn.microsoft.com/en-us/windows-server/get-started/servicing-channels-comparison#semi-annual-channel) 에서 새로운 Windows Server 제품을 1년에 2번 릴리스하며, 매 2~3년마다 새로운 메이저 버전의 Windows Server를 [Long-Term Servicing Channel (LTSC)](https://learn.microsoft.com/en-us/windows-server/get-started/servicing-channels-comparison#long-term-servicing-channel-ltsc)에서 릴리스합니다.

GitLab은 Google Cloud Platform에서 Microsoft의 공식 릴리스 날짜로부터 1개월 이내에 최신 Windows Server 버전(Semi-Annual Channel)을 포함하는 새로운 러너 도우미 이미지를 테스트하고 릴리스하는 것을 목표로 합니다. 사용 가능 날짜는 [Windows Server 서비싱 옵션별 현재 버전 목록](https://learn.microsoft.com/en-us/windows/release-health/windows-server-release-info#windows-server-current-versions-by-servicing-option)을 참조하세요.
