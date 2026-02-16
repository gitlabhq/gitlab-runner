---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Runner support policy
---

The support policy by GitLab Runner is determined by the lifecycle policy of the operating system.

## Container images support

We follow the support lifecycle of the base distributions (Ubuntu, Alpine, Red Hat Universal Base Image) used for creating the GitLab Runner container images.

The end-of-publishing dates for the base distributions will not necessarily align with the GitLab major release cycle. This means we will stop publishing a version of the GitLab Runner container image in a minor release. This ensures that we do not publish images that the upstream distribution no longer updates.

### Container images and end of publishing date

| Base container                 | Base container version | Vendor EOL date | GitLab EOL date |
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

GitLab Runner versions 17.7 and later support only a single Alpine version (`latest`) instead of specific versions.
Alpine versions 3.21 will be supported to the stated EOL date. In contrast, Ubuntu 24.04
will be supported to its EOL date, at which point we will move to the most recent LTS release.

## Windows version support

GitLab officially supports LTS versions of Microsoft Windows operating systems and so we follow the Microsoft
[Servicing Channels](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#servicing-channels) lifecycle policy.

This means that we support:

- [Long-Term Servicing Channel](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#long-term-servicing-channel)
  versions for five years after their release date.

  After five years, Microsoft offers extended support for an additional five years.
  During this extended period, we offer support for as long as is practical.
  We can end this support, with announcement, on a GitLab major release.
- [Semi-Annual Channel](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#semi-annual-channel)
  versions for 18 months after their release date. We don't support
  these versions after mainstream support ends.

This support policy applies to the [Windows binaries](windows.md#installation) that we
distribute and the [Docker executor](../executors/docker.md#supported-windows-versions).

{{< alert type="note" >}}

The Docker executor for Windows containers has strict version
requirements, because containers have to match the version of the host
OS. See the [list of supported Windows containers](../executors/docker.md#supported-windows-versions)
for more information.

{{< /alert >}}

As a single source of truth, we use <https://learn.microsoft.com/en-us/lifecycle/products/>,
which specifies the release, mainstream, and extended support dates.

Below is a list of versions that are commonly used and their end of life
date:

| Operating system           | Mainstream support end date | Extended support end date |
|----------------------------|-----------------------------|---------------------------|
| Windows Server 2019 (1809) | January 2024                | January 2029              |
| Windows Server 2022 (21H2) | October 2026                | October 2031              |
| Windows Server 2025 (24H2) | October 2029                | October 2034              |

### Future releases

Microsoft releases new Windows Server products in the
[Semi-Annual Channel](https://learn.microsoft.com/en-us/windows-server/get-started/servicing-channels-comparison#semi-annual-channel)
twice a year, and every 2 - 3 years a new major version of Windows Sever
is released in the
[Long-Term Servicing Channel (LTSC)](https://learn.microsoft.com/en-us/windows-server/get-started/servicing-channels-comparison#long-term-servicing-channel-ltsc).

GitLab aims to test and release new GitLab Runner helper images that
include the latest Windows Server version (Semi-Annual Channel) within 1
month of the official Microsoft release date on the Google Cloud Platform. Refer to the
[Windows Server current versions by servicing option list](https://learn.microsoft.com/en-us/windows/release-health/windows-server-release-info#windows-server-current-versions-by-servicing-option)
for availability dates.
