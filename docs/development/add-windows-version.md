---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Add Docker executor support for a Windows version

GitLab supports [specific versions of Windows](../install/windows.md#windows-version-support-policy).

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

Windows requires us to have the host OS version match the container
OS, so if we are building `Windows Server Core 2004` image we need to
have `gitlab-runner` installed on `Windows Server Core 2004`.

To do this we must update the
[windows-containers](https://gitlab.com/gitlab-org/ci-cd/shared-runners/images/gcp/windows-containers)
repository to build a base image. The base image will be used by the
[autoscaler](https://gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/autoscaler)
for our CI. The new base image will be used to build the GitLab Runner
helper image.

For example, if we want to add support for `Windows Server Core 2004` in
the 13.7 milestone we can see the following
[merge request](https://gitlab.com/gitlab-org/ci-cd/shared-runners/images/gcp/windows-containers/-/merge_requests/29).
Depending on the base image provided by GCP, we might have to install
Docker as part of the build process or not. In this MR we update the
following files:

1. `.gitlab-ci.yml`
1. `.gitlab/ci/build.gitlab-ci.yml`

### Test the image generated

We recommend testing the image generated in the `dev` step. It is likely to be name `dev xxx` where `xxx` stands for the windows server version.

To test the image, the following steps can be followed:

1. Add support for the new windows server version in [`GitLab Runner project`](https://gitlab.com/gitlab-org/gitlab-runner) and generate the `gitlab-runner-helper.x86_64-windows.exe` binary.
1. Create a VM using the disk image generated during the `dev` step.
When adding support for `windows server ltsc2022`, the disk image name was
[`runners-windows-21h1-core-containers-dev-40-mr`](https://gitlab.com/gitlab-org/ci-cd/shared-runners/images/gcp/windows-containers/-/jobs/2333691567#L697)
1. Generate the `gitlab-runner-helper` Docker image from this VM. To do so, you will need to download the `gitlab-runner-helper.x86_64-windows.exe` binary on the VM.
As the `Invoke-WebRequest` PowerShell command might be unavailable, we recommend using the `Start-BitsTransfer` command instead.
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

Run [`gitlab-runner register`](../register/index.md)
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
update our [Ansible repository](https://ops.gitlab.net/gitlab-com/gl-infra/ci-infrastructure-windows)
to include the new Windows version.

For example, if we want to add support for `Windows Server Core 2004` in
the 13.7 milestone we can see the following
[merge request](https://ops.gitlab.net/gitlab-com/gl-infra/ci-infrastructure-windows/-/merge_requests/70),
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

1. [List of support versions](https://gitlab.com/gitlab-org/gitlab-runner/-/blob/v13.4.1/helpers/container/windows/version.go#L38-42), and tests surrounding it.
1. [List of base images](https://gitlab.com/gitlab-org/gitlab-runner/-/blob/v13.4.1/helpers/container/helperimage/windows_info.go#L10-21), and tests surrounding it.
1. [Update GitLab CI to run tests on the default branch](https://gitlab.com/gitlab-org/gitlab-runner/-/blob/v13.4.1/.gitlab/ci/test.gitlab-ci.yml#L176-180).
1. [Update the `release` stage](https://gitlab.com/gitlab-org/gitlab-runner/-/blob/v13.4.1/.gitlab-ci.yml#L8).

For example, if we want to add support for `Windows Server Core 2004` in
the 13.7 milestone we can see the following
[merge request](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2459),
where we update the following files:

1. `helpers/container/helperimage/windows_info.go`
1. `helpers/container/helperimage/windows_info_test.go`
1. `helpers/container/windows/version.go`
1. `helpers/container/windows/version_test.go`
1. `.gitlab/ci/test.gitlab-ci.yml`
1. `.gitlab/ci/coverage.gitlab-ci.yml`
1. `.gitlab/ci/_common.gitlab-ci.yml`
1. `.gitlab/ci/release.gitlab-ci.yml`
1. `ci/.test-failures.servercore2004.txt`
1. `docs/executors/docker.md`
