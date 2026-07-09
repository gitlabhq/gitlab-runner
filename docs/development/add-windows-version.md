---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Add Docker executor support for a Windows version
---

GitLab supports [specific versions of Windows](../install/support-policy.md#windows-version-support).

To add support for a new Windows version for the
[Docker executor](../executors/docker.md), you must release a
[helper image](../configuration/advanced-configuration.md#helper-image)
with the same Windows version. Then you can run the helper image on the
Windows host OS.

To build the helper image for the version, you need
GitLab Runner installed on that Windows version, because Windows requires
your host OS and container OS versions to match.

## Infrastructure

We must build the helper image for it to be used for the user job.

### Create a base image for infrastructure to use

To add support for a new Windows version, you might need to create a new helper image.
Windows versions can run older helper images (backward compatibility),
or might require a newly built helper image. For compatibility details, see
[Windows container version compatibility](https://learn.microsoft.com/en-us/virtualization/windowscontainers/deploy-containers/version-compatibility)

The GitLab Runner helper image is built from a base image published by the
[runner-tools/base-images](https://gitlab.com/gitlab-org/ci-cd/runner-tools/base-images) project.
The `windows` target in `dockerfiles/runner-helper/docker-bake.hcl` defines the build configuration.

To support a new Windows version or architecture, that project must first publish the matching
`runner-helper:<version>-servercore-ltsc<year>[-<arch>]` base image. For example,
[merge request 88](https://gitlab.com/gitlab-org/ci-cd/runner-tools/base-images/-/merge_requests/88)
added the `ltsc2025`, `ltsc2025-arm64`, `servercore`, and `nanoserver` base images.

The [`windows-containers`](https://gitlab.com/gitlab-org/ci-cd/shared-runners/images/gcp/windows-containers) 
repository builds the GCP host VM images for the shared Windows runner fleet. 
The [autoscaler](https://gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/autoscaler) 
provisions them.

For example, when adding support for Windows Server 2025,
backward compatibility allowed reuse of the existing 2022 helper images.
However, when adding support to Windows Server 2022,
the Windows Server 2019 helper image was not compatible with process isolation,
so a new image was required.

Some GCP base images require Docker installation during the build process. To update the CI/CD
environment for a new image, update the following files:

- `.gitlab-ci.yml`
- `.gitlab/ci/build.gitlab-ci.yml`

### Test the image generated

We recommend testing the image generated in the `dev` step. It is likely to be named `dev xxx` where `xxx` stands for the windows server version.

To test the image, the following steps can be followed:

1. Add support for the new windows server version in [`GitLab Runner project`](https://gitlab.com/gitlab-org/gitlab-runner) and generate the `gitlab-runner-helper.x86_64-windows.exe` binary (or `gitlab-runner-helper.arm64-windows.exe` for ARM64 hosts).
1. Create a VM using the disk image generated during the `dev` step.
   When adding support for `windows server ltsc2022`, the disk image name was
   [`runners-windows-21h1-core-containers-dev-40-mr`](https://gitlab.com/gitlab-org/ci-cd/shared-runners/images/gcp/windows-containers/-/jobs/2333691567#L697)
1. Generate the `gitlab-runner-helper` Docker image from this VM. To do so, you need to download the `gitlab-runner-helper.x86_64-windows.exe` binary on the VM.
   As the `Invoke-WebRequest` PowerShell command might be unavailable, you should use the `Start-BitsTransfer` command instead.
1. Create another VM using the new GCP windows server image to support.
1. Install the `gitlab-runner` executable generated for the previously update `GitLab-Runner` project and register it to a project.
1. Successfully launch a job.

An example of this procedure is summarized in [this comment](https://gitlab.com/gitlab-org/ci-cd/shared-runners/images/gcp/windows-containers/-/merge_requests/40#note_910281106).

### Publish the image

After we merge the merge request created from the
[previous step](#create-a-base-image-for-infrastructure-to-use), we need to run the
[publish job](https://gitlab.com/gitlab-org/ci-cd/shared-runners/images/gcp/windows-containers/-/blob/120b30096b2db7bb445f69b1923e161b10b589e6/.gitlab/ci/build.gitlab-ci.yml#L155-166)
manually for the image to be published to our production GCP project.

Take note of the image name that is created from the `publish` job, for
example in [this job](https://gitlab.com/gitlab-org/ci-cd/shared-runners/images/gcp/windows-containers/-/jobs/643514801)
we created an image called
`runners-windows-2019-core-containers-2020-07-17`. This will be used for
the [install part](#install).

### Add two new runner managers

At this point we should have a base image ready in our production
environment, so we can use it inside the CI pipeline for the GitLab Runner
project. The only thing that is left is to set up the Runner Managers.

#### Register

Run [`gitlab-runner register`](../register/_index.md)
to register the two new runners. These should be project-specific runners, so
we need to use the registration token from the
[project settings](https://gitlab.com/gitlab-org/gitlab-runner/-/settings/ci_cd).
The name of the runner should follow the same naming convention as the
existing ones.

For example, for `Windows Server Core 2004` we should name the Runner
Managers the following:

1. `windows-2004-private-runner-manager-1`
1. `windows-2004-private-runner-manager-2`

Once registered, make sure you safely store the runner tokens found in
the `config.toml` file since we are going to need these for the [installation](#install)
step.

Finally, we'll need to assign the new Runner Managers to the [security](https://gitlab.com/gitlab-org/security/gitlab-runner)
fork project and to the ['liveness' test support](https://gitlab.com/gitlab-org/ci-cd/tests/liveness) project. So for each of the new Runner Managers:

1. Go to the Runners section of the [Runner project CI/CD settings page](https://gitlab.com/gitlab-org/gitlab-runner/-/settings/ci_cd);
1. Unlock the new Runner by editing its properties and unchecking `Lock to current projects`;
1. For the [security](https://gitlab.com/gitlab-org/security/gitlab-runner) fork project:
   1. Go to the Runners section of the [project's CI/CD settings page](https://gitlab.com/gitlab-org/security/gitlab-runner/-/settings/ci_cd);
   1. Scroll down to the `Other available runners` section and enable the runner for this project;
1. For the ['liveness' test support](https://gitlab.com/gitlab-org/ci-cd/tests/liveness) project:
   1. Go to the Runners section of the [project's CI/CD settings page](https://gitlab.com/gitlab-org/ci-cd/tests/liveness/-/settings/ci_cd);
   1. Scroll down to the `Other available runners` section and enable the runner for this project;
1. Lock the Runner back again in the [Runner project CI/CD settings page](https://gitlab.com/gitlab-org/gitlab-runner/-/settings/ci_cd).

#### Install

Install a new instance of
[autoscaler](https://gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/autoscaler)
to have a specific `config.toml` for that Windows version. We need to
update our Ansible repository (`https://ops.gitlab.net/gitlab-com/gl-infra/ci-infrastructure-windows`)
to include the new Windows version.

For example, if we want to add support for `Windows Server Core 2004` in
the 13.7 milestone we can see this
merge request: `https://ops.gitlab.net/gitlab-com/gl-infra/ci-infrastructure-windows/-/merge_requests/70`,
where we update the following files:

1. `ansible/roles/runner/tasks/main.yml`
1. `ansible/roles/runner/tasks/autoscaler.yml`
1. `ansible/group_vars/gcp_role_runner_manager.yml`
1. `ansible/host_vars/windows-shared-runners-manager-1.yml`
1. `ansible/host_vars/windows-shared-runners-manager-2.yml`

When opening a merge request make sure that the maintainer is aware
that they need to [register](#register) 2 new runners and save them
inside the CI/CD variables with the keys defined in
`ansible/host_vars`.

## Publish `registry.gitlab.com/gitlab-org/ci-cd/tests/liveness`

The image `registry.gitlab.com/gitlab-org/ci-cd/tests/liveness` is used
as part of the CI process for [GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner).
Make sure that an image based on the new Windows version is published.

For example, if we want to add support for `Windows Server Core 2004` in
the 13.7 milestone we can see the following
[merge request](https://gitlab.com/gitlab-org/ci-cd/tests/liveness/-/merge_requests/4),
where we update the following files:

1. `.gitlab-ci.yml`
1. `Makefile`

## Update GitLab Runner to support specific Windows version

Since we need to provide a helper image for users to be able to use the
Docker executor we have specific checks inside the code base, we need to
allow the new Windows version.

We should update the following:

1. **Windows version detection**: Add the kernel build number and version constant to
   `supportedWindowsBuilds` in `helpers/container/windows/version.go`, and update the surrounding tests.
1. **Version-to-image mapping**: Add the version to the `ltsc` map in
   `helpers/container/helperimage/windows_info.go`, and update the surrounding tests. The image tag,
   prebuilt bundle name, and host architecture are all derived from this mapping.
1. **Helper image build**: Add the new `servercore:ltsc<year>` entry (and, for a new architecture,
   the `-arch` variant) to the `windows` target in `dockerfiles/runner-helper/docker-bake.hcl`.
1. **Publish mapping**: Map the built artifact to its published registry tag in
   `scripts/pusher/helper-images.json`.
1. **CI jobs**: Add or update the following:

   - Prebuilt helper image job in `.gitlab/ci/build.gitlab-ci.yml`
   - `WINDOWS_VERSION` or `WINDOWS_PREBUILT` variables in `.gitlab/ci/_common.gitlab-ci.yml` (to run tests on the new version)
   - Test jobs in `.gitlab/ci/test.gitlab-ci.yml` and `.gitlab/ci/coverage.gitlab-ci.yml`
   - Quarantine file `ci/.test-failures.servercore<version>.txt`
1. **Documentation**: Update the supported versions and helper image list in `docs/executors/docker.md`.

Example: Windows Server 2025 (LTSC2025) helper image support, including the `arm64`
variant, was implemented across several merge requests (parent
[issue 39182](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/39182)):

- [Merge request 88](https://gitlab.com/gitlab-org/ci-cd/runner-tools/base-images/-/merge_requests/88):
  Added the `ltsc2025` and `ltsc2025-arm64` base images (prerequisite in the `runner-tools/base-images` project).
- [Merge request 6033](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6033): Built
  `servercore:ltsc2025` and `servercore:ltsc2025-arm64` helper images from
  `dockerfiles/runner-helper/docker-bake.hcl`, `scripts/pusher/helper-images.json`,
  `.gitlab/ci/build.gitlab-ci.yml` base images. At this stage, the ARM64 image bundled the AMD64 helper
  binary under Windows emulation.
- [Merge request 6697](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6697): Added a native ARM64
  `gitlab-runner-helper.exe` build target (`Makefile.runner_helper.mk`, `ci/release_dir`).
- [Merge request 6716](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6716): Bundled the native
  ARM64 helper binary into the `servercore:ltsc2025-arm64` image instead of the emulated AMD64 binary
  (`dockerfiles/runner-helper/docker-bake.hcl`, `scripts/pusher/helper-images.json`).
- [Merge request 6717](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6717): Built 
  `nanoserver:ltsc2025` and `nanoserver:ltsc2025-arm64` helper images.
