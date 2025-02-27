---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Executors
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner implements different executors that can be used to run your
builds in different environments.

If you are not sure about which executor to select, see [Selecting the executor](#selecting-the-executor).

For more information about features supported by each executor, see the [compatibility chart](#compatibility-chart).

GitLab Runner provides the following executors:

- [SSH](ssh.md)
- [Shell](shell.md)
- [Parallels](parallels.md)
- [VirtualBox](virtualbox.md)
- [Docker](docker.md)
- [Docker Autoscaler](docker_autoscaler.md)
- [Docker Machine (auto-scaling)](docker_machine.md)
- [Kubernetes](../executors/kubernetes/_index.md)
- [Instance](instance.md)
- [Custom](custom.md)

These executors are locked and we are no longer developing or accepting
new ones. For more information, see [Contributing new executors](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/CONTRIBUTING.md#contributing-new-executors).

## Prerequisites for non-Docker executors

Executors that do not [rely on a helper image](../configuration/advanced-configuration.md#helper-image) require a Git
installation on the target machine and in the `PATH`. Always use the [latest available version of Git](https://git-scm.com/downloads).

GitLab Runner uses the `git lfs` command if [Git LFS](https://git-lfs.com/) is installed
on the target machine. Ensure Git LFS is up to date on any systems where GitLab Runner uses these executors.

Be sure to initialize Git LFS for the user that executes GitLab Runner commands with `git lfs install`. You can initialize Git LFS on an entire system with `git lfs install --system`.

When you enable [FF_GIT_URLS_WITHOUT_TOKENS](../configuration/feature-flags.md), do not cache Git
credentials across builds, for example, by using Git credential helpers. If you cache credentials,
the [`CI_JOB_TOKEN`](https://docs.gitlab.com/ci/jobs/ci_job_token/) might be shared between
concurrent or consecutive builds, which might cause authentication errors and failed builds.

## Selecting the executor

The executors support different platforms and methodologies for building a
project. The table below shows the key facts for each executor which helps
you decide which executor to use.

| Executor                                         | SSH  |     Shell      |   VirtualBox   |   Parallels    | Docker | Docker Autoscaler |                 Instance |   Kubernetes   |          Custom          |
|:-------------------------------------------------|:----:|:--------------:|:--------------:|:--------------:|:------:|:-----------------:|-------------------------:|:--------------:|:------------------------:|
| Clean build environment for every build          |  ✗   |       ✗        |       ✓        |       ✓        |   ✓    |         ✓         | conditional <sup>4</sup> |       ✓        | conditional <sup>4</sup> |
| Reuse previous clone if it exists                |  ✓   |       ✓        |       ✗        |       ✗        |   ✓    |         ✓         | conditional <sup>4</sup> | ✓ <sup>6</sup> | conditional <sup>4</sup> |
| Runner file system access protected <sup>5</sup> |  ✓   |       ✗        |       ✓        |       ✓        |   ✓    |         ✓         |                        ✗ |       ✓        |       conditional        |
| Migrate runner machine                           |  ✗   |       ✗        |    partial     |    partial     |   ✓    |         ✓         |                        ✓ |       ✓        |            ✓             |
| Zero-configuration support for concurrent builds |  ✗   | ✗ <sup>1</sup> |       ✓        |       ✓        |   ✓    |         ✓         |                        ✓ |       ✓        | conditional <sup>4</sup> |
| Complicated build environments                   |  ✗   | ✗ <sup>2</sup> | ✓ <sup>3</sup> | ✓ <sup>3</sup> |   ✓    |         ✓         |           ✗ <sup>2</sup> |       ✓        |            ✓             |
| Debugging build problems                         | easy |      easy      |      hard      |      hard      | medium |      medium       |                   medium |     medium     |          medium          |

**Footnotes:**

1. If the builds use services installed on the build machine, selecting executors is possible but problematic.
1. Requires manual dependency installation.
1. For example, using [Vagrant](https://developer.hashicorp.com/vagrant/docs/providers/virtualbox "Vagrant documentation for VirtualBox").
1. Depends on the environment you are provisioning. Can be completely isolated or shared between builds.
1. When a runner's file system access is not protected, jobs can access the entire system,
   including the runner's token and other jobs' cache and code. 
   Executors marked ✓ don't allow the runner to access the file system by default.
   However, security flaws or certain configurations could allow jobs
   to break out of their container and access the file system hosting the runner.
1. Requires [persistent per-concurrency build volumes](kubernetes/_index.md#persistent-per-concurrency-build-volumes) configuration.

### Shell executor

The Shell executor is simplest configuration option for GitLab Runner. It executes jobs locally on
the system where GitLab Runner is installed, requiring all dependencies to be manually installed on the same
machine.

This executor supports Bash for Linux, macOS, and FreeBSD operating systems, while offering PowerShell
support for Windows environments.

While ideal for builds with minimal dependencies, it only provides limited isolation between jobs.

### Docker executor

Docker executor provides clean build environments through containers. Dependency management is straightforward,
with all dependencies packaged in the Docker image. This executor requires Docker installation on the Runner host.

This executor supports additional [services](https://docs.gitlab.com/ee/ci/services/index.html) like MySQL.
It also accommodates Podman as an alternative container runtime.

This executor maintains consistent, isolated build environments.

### Docker Machine executor (deprecated)

{{< alert type="warning" >}}

This feature was [deprecated](https://gitlab.com/gitlab-org/gitlab/-/issues/498268) in GitLab 17.5
and is planned for removal in 20.0. Use [GitLab Runner Autoscaler](../runner_autoscale/_index.md) instead.

{{< /alert >}}

The Docker Machine executor is a special version of the Docker executor with support for auto-scaling. It works like the typical
Docker executor but with build hosts created on demand by Docker Machine. This capability makes it particularly effective
in cloud environments like AWS EC2, offering excellent isolation and scalability for variable workloads.

### Docker Autoscaler executor

The Docker Autoscaler executor is an autoscale-enabled Docker executor that creates instances on demand to
accommodate the jobs that the runner manager processes. It wraps the [Docker executor](docker.md) so that all
Docker executor options and features are supported.

The Docker Autoscaler uses [fleeting plugins](https://gitlab.com/gitlab-org/fleeting/fleeting) to autoscale.
Fleeting is an abstraction for a group of autoscaled instances, which uses plugins that support cloud providers,
like Google Cloud, AWS, and Azure. This executor particularly suits environments with dynamic workload requirements.

### Instance executor

The Instance executor is an autoscale-enabled executor that creates instances on demand to accommodate
the expected volume of jobs that the runner manager processes.

This executor and the related Docker Autoscale executor are the new autoscaling executors that works in conjunction with the GitLab Runner Fleeting and Taskscaler technologies.

The Instance executor also uses [fleeting plugins](https://gitlab.com/gitlab-org/fleeting/fleeting) to autoscale.

You can use the Instance executor when jobs need full access to the host instance, operating system, and
attached devices. The Instance executor can also be configured to accommodate single-tenant and multi-tenant jobs.

### Kubernetes executor

You can use the Kubernetes executor to use an existing Kubernetes cluster for your builds. The executor calls the
Kubernetes cluster API and creates a new Pod (with a build container and services containers) for each GitLab CI/CD job.
This executor particularly suits cloud-native environments, offering superior scalability and resource utilization.

### SSH executor

The SSH executor is added for completeness, but it's among the least supported executors.
When you use the SSH executor, GitLab Runner connects to an external server and runs the builds there.
We have some success stories from organizations using this executor, but usually you should use one of the other types.

### Custom executor

You can use the Custom executor to specify your own execution environments.
When GitLab Runner does not provide an executor (for example, Linux containers),
it allows you to use custom executables to provision and clean up environments.

## Compatibility chart

Supported features by different executors:

| Executor                                     | SSH            | Shell          |VirtualBox      | Parallels      | Docker  | Docker Autoscaler | Instance       | Kubernetes | Custom                                                       |
|:---------------------------------------------|:--------------:|:--------------:|:--------------:|:--------------:|:-------:|:-----------------:|:--------------:| :---------:| :-----------------------------------------------------------:|
| Secure Variables                             | ✓              | ✓              | ✓              | ✓              | ✓       | ✓                 | ✓              | ✓          | ✓                                                           |
| `.gitlab-ci.yml`: image                      | ✗              | ✗              | ✓ (1)          | ✓ (1)          | ✓       | ✗                 | ✗              | ✓          | ✓ (by using [`$CUSTOM_ENV_CI_JOB_IMAGE`](custom.md#stages)) |
| `.gitlab-ci.yml`: services                   | ✗              | ✗              | ✗              | ✗              | ✓       | ✗                 | ✗              | ✓          | ✓      |
| `.gitlab-ci.yml`: cache                      | ✓              | ✓              | ✓              | ✓              | ✓       | ✓                 | ✓              | ✓          | ✓      |
| `.gitlab-ci.yml`: artifacts                  | ✓              | ✓              | ✓              | ✓              | ✓       | ✓                 | ✓              | ✓          | ✓      |
| Passing artifacts between stages             | ✓              | ✓              | ✓              | ✓              | ✓       | ✓                 | ✓              | ✓          | ✓      |
| Use GitLab Container Registry private images | not applicable | not applicable | not applicable | not applicable | ✓       | ✓                 | not applicable | ✓          | not applicable |
| Interactive Web terminal                     | ✗              | ✓ (UNIX)       | ✗              | ✗              | ✓       | ✗                 | ✗              | ✓          | ✗              |

1. Support [added](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/1257) in GitLab Runner 14.2.
   Refer to the [Overriding the base VM image](../configuration/advanced-configuration.md#overriding-the-base-vm-image) section for further details.

Supported systems by different shells:

| Shells  | Bash        | PowerShell Desktop | PowerShell Core | Windows Batch (deprecated) |
|:-------:|:-----------:|:------------------:|:---------------:|:--------------------------:|
| Windows | ✗ (4)       | ✓ (3)              | ✓               | ✓ (2)                      |
| Linux   | ✓ (1)       | ✗                  | ✓               | ✗                          |
| macOS   | ✓ (1)       | ✗                  | ✓               | ✗                          |
| FreeBSD | ✓ (1)       | ✗                  | ✗               | ✗                          |

1. Default shell.
1. Deprecated. Default shell if no
   [`shell`](../configuration/advanced-configuration.md#the-runners-section)
   is specified.
1. Default shell when a new runner is registered.
1. Bash shell on Windows is not supported.

Supported systems for interactive web terminals by different shells:

| Shells  | Bash        | PowerShell Desktop    | PowerShell Core    | Windows Batch (deprecated) |
|:-------:|:-----------:|:---------------------:|:------------------:|:--------------------------:|
| Windows | ✗           | ✗                     | ✗                  | ✗                          |
| Linux   | ✓           | ✗                     | ✗                  | ✗                          |
| macOS   | ✓           | ✗                     | ✗                  | ✗                          |
| FreeBSD | ✓           | ✗                     | ✗                  | ✗                          |

The following diagram shows which executor to choose based on your operating system and platform:

```mermaid
graph TD
    Start[Which executor to choose?] --> BuildType{Autoscaling or No Autosclaing?}


    BuildType -->|No| BuildType2{Container or OS Shell builds?}
    BuildType-->|Yes| Platform{Platform}
    BuildType2 -->|Shell| ShellOptions{Operating System}
    BuildType2 -->|Container| ContainerOptions{Operating System}


    Platform -->|Cloud Native| Kubernetes[Kubernetes]
    Platform -->|Cloud VMs| OSType{Operating System}

    OSType -->|Windows| WinExec{Executor Type}
    OSType -->|macOS| MacExec{Executor Type}
    OSType -->|Linux| LinuxExec{Executor Type}


    WinExec --> AutoscalerWin[Fleeting: Docker Autoscaler Executor]
    WinExec --> InstanceWin[Fleeting:Instance Executor]

    MacExec --> AutoscalerMac[Fleeting: Docker Autoscaler Executor]
    MacExec --> InstanceMac[Fleeting:Instance Executor]

    LinuxExec --> AutoscalerLin[Fleeting: Docker Autoscaler Executor]
    LinuxExec --> InstanceLin[Fleeting:Instance Executor]


    ShellOptions -->|Linux| Linux_Shell[Bash;Zsh]
    ShellOptions -->|macOS| MacOS[Bash;Zsh]
    ShellOptions -->|Windows| Windows[Powershell 5.1; PowerShell 7.x]
    ShellOptions -->|Remote Machine| SSH[SSH]


    ContainerOptions -->|Linux| Linux_Shell2[Docker;Podman]
    ContainerOptions -->|macOS| macOS2[Docker]
    ContainerOptions -->|Windows| Windows2[Docker]

    %% Styling
    classDef default fill:#f9f9f9,stroke:#333,stroke-width:2px;
    classDef decision fill:#e1f3fe,stroke:#333,stroke-width:2px;
    classDef executor fill:#dcffe4,stroke:#333,stroke-width:2px;

    class Start default;
    class BuildType,BuildType2,Container,Scaling,AutoScale,NoAutoScale,ShellOptions,ContainerOptions,OSType,WinExec,MacExec,Platform,LinuxExec decision;
    class Kubernetes,Docker,Custom,Shell,Windows,SSH,DockerMachineWin,AutoscalerWin,InstanceWin,DockerMachineMac,AutoscalerMac,InstanceMac,DockerMachineLin,AutoscalerLin,InstanceLin executor;
```
