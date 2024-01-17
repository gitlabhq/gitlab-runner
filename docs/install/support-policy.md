---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# GitLab Runner support policy

The support policy by GitLab Runner is determined by the lifecycle policy of the operating system.

## Container images support

We follow the support lifecycle of the base distributions (Ubuntu, Alpine, Red Hat Universal Base Image) used for creating the GitLab Runner container images.

The end-of-publishing dates for the base distributions will not necessarily align with the GitLab major release cycle. This means we will stop publishing a version of the GitLab Runner container image in a minor release. This ensures that we do not publish images that the upstream distribution no longer updates.

### Container images and end of publishing date

| Base container                 | Base container version | Vendor EOL date | GitLab EOL date |
|--------------------------------|------------------------|-----------------|-----------------|
| Ubuntu                         | 20.04                  | 2030-04-09      | 2030-04-22      |
| Alpine                         | 3.12                   | 2022-05-01      | 2023-05-22      |
| Alpine                         | 3.13                   | 2022-11-01      | 2023-05-22      |
| Alpine                         | 3.14                   | 2023-05-01      | 2023-05-22      |
| Alpine                         | 3.15                   | 2023-11-01      | 2024-01-18      |
| Alpine                         | 3.16                   | 2024-05-23      | 2024-06-22      |
| Alpine                         | 3.17                   | 2024‑11‑22      | 2024-12-22      |
| Alpine                         | 3.18                   | 2025‑05‑09      | 2025-05-22      |
| Alpine                         | 3.19                   | 2025‑11‑01      | 2025-11-22      |
| Red Hat Universal Base Image 8 | 8.7-1054               | 2024-05-31      | 2024-06-22      |

## Windows version support

GitLab officially supports LTS versions of Microsoft Windows operating systems and so we follow the Microsoft
[Servicing Channels](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#servicing-channels) lifecycle policy.

This means that we support:

- [Long-Term Servicing Channel](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#long-term-servicing-channel),
  versions for 5 years after their release date. Note that we don't
  support versions that are on extended support.
- [Semi-Annual Channel](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#semi-annual-channel)
  versions for 18 months after their release date. We don't support
  these versions after mainstream support ends.

This is the case for both the [Windows binaries](windows.md#installation) that we
distribute, and also for the [Docker executor](../executors/docker.md#supported-windows-versions).

NOTE:
The Docker executor for Windows containers has strict version
requirements, because containers have to match the version of the host
OS. See the [list of supported Windows containers](../executors/docker.md#supported-windows-versions)
for more information.

GitLab provides Windows operating system runner images until the EOL (End-Of-Life) date for the operating system. After the EOL date of the Windows OS, GitLab stops releasing runner images with the EOL Windows OS version.

The EOL date for a Windows OS version will not necessarily align with a GitLab major release; therefore, we will typically stop releasing an EOL image in a GitLab minor release. A removal notice will be included in the release post of the GitLab version in which we stopped publishing the image with the EOL Windows version.

As a single source of truth we use
<https://learn.microsoft.com/en-us/lifecycle/products/> which specifies
both the release and mainstream support dates.

Below is a list of versions that are commonly used and their end of life
date:

| OS                                  | Mainstream support end of life date |
|-------------------------------------|-------------------------------------|
| Windows 10 1809/2019                | January 2024                        |
| Windows Server Datacenter 1809/2019 | January 2024                        |

### Future releases

Microsoft releases new Windows Server products in the
[Semi-Annual Channel](https://learn.microsoft.com/en-us/windows-server/get-started/servicing-channels-comparison#semi-annual-channel)
twice a year, and every 2 - 3 years a new major version of Windows Sever
is released in the
[Long-Term Servicing Channel (LTSC)](https://learn.microsoft.com/en-us/windows-server/get-started/servicing-channels-comparison#long-term-servicing-channel-ltsc).

GitLab aims to test and release new GitLab Runner helper images that
include the latest Windows Server version (Semi-Annual Channel) within 1
month of the official Microsoft release date on the Google Cloud Platform. Refer to the
[Windows Server current versions by servicing option list](https://learn.microsoft.com/en-us/windows-server/get-started/windows-server-release-info#windows-server-current-versions-by-servicing-option)
for availability dates.
