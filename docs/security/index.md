---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
comments: false
---

# Security for self-managed runners **(FREE)**

A GitLab CI/CD pipeline is a workflow automation engine used for simple or complex DevOps automation tasks. Because these pipelines enable a remote code execution service, you should implement the following process to reduce security risks:

- A systematic approach to configuring the security of the entire technology stack.
- Ongoing rigorous reviews of the configuration and use of the platform.

If you plan to run your GitLab CI/CD jobs on self-managed runners, then security risks exist for your compute infrastructure and network.

The runner executes code defined in the CI/CD job. Any user that has the Developer role for the project's repository could compromise the security of the environment hosting the runner, whether intentional or not.

This risk is even more acute if your self-managed runners are non-ephemeral and used for multiple projects.

- A job from a repository embedded with malicious code can compromise the security of other repositories serviced by the non-ephemeral runner.
- Depending on the executor, a job can install malicious code on the virtual machine where the runner is hosted.
- Secret variables exposed to jobs running in a compromised environment can be stolen, including but not limited to the CI_JOB_TOKEN.
- Users with the Developer role have access to submodules associated with the project, even if they don't have access to
  the upstream projects of the submodule.

## Security risks for different executors

Depending on the executor you are using, you can face different security risks.

### Usage of Shell executor

**High-security risks exist to your runner host and network when running builds with the `shell` executor.** The jobs are run
with the permissions of the GitLab Runner's user and can steal code from other
projects that are run on this server. Use it only for running trusted builds.

### Usage of Docker executor

**Docker can be considered safe when running in non-privileged mode.** To make
such a configuration more secure, run jobs as a non-root user in Docker
containers with disabled sudo or dropped `SETUID` and `SETGID` capabilities.

More granular permissions can be configured in non-privileged mode via the
`cap_add`/`cap_drop` settings.

WARNING:
Privileged containers in Docker have all the root capabilities of the host VM.
For more information, check out the official Docker documentation
on [Runtime privilege and Linux capabilities](https://docs.docker.com/engine/reference/run/#runtime-privilege-and-linux-capabilities)

It is **not advised** to run containers in privileged mode.

When privileged mode is enabled, a user running a CI/CD job could gain full root access
to the runner's host system, permission to mount and unmount volumes, and run nested
containers.

By enabling privileged mode, you are effectively disabling all the container's security
mechanisms and exposing your host to privilege escalation, which can lead to container breakout.

It is especially risky when runners are shared between several organizations.
For example, an instance-wide runner in a service like GitLab.com, where multiple
separate organizations can work concurrently.

If you use a Docker Machine executor, we also strongly recommend to use the `MaxBuilds = 1` setting,
which ensures that a single autoscaled VM (potentially compromised because of the security weakness
introduced by the privileged mode) is used to handle one and only one job.

### Usage of private Docker images with `if-not-present` pull policy

When using the private Docker images support described in
[advanced configuration: using a private container registry](../configuration/advanced-configuration.md#use-a-private-container-registry)
you should use `always` as the `pull_policy` value. Especially you should
use `always` pull policy if you are hosting a public, shared Runner with the
Docker or Kubernetes executors.

Let's consider an example where the pull policy is set to `if-not-present`:

1. User A has a private image at `registry.example.com/image/name`.
1. User A starts a build on a shared runner: The build receives the registry
   credentials and pulls the image after authorization in registry.
1. The image is stored on a shared runner's host.
1. User B doesn't have access to the private image at `registry.example.com/image/name`.
1. User B starts a build that is using this image on the same shared runner
   as User A: Runner finds a local version of the image and uses it **even if
   the image could not be pulled because of missing credentials**.

Therefore, if you host a runner that can be used by different users and
different projects (with mixed private, and public access levels) you should
never use `if-not-present` as the pull policy value, but use:

- `never` - If you want to limit users to use the only image pre-downloaded by you.
- `always` - If you want to give users the possibility to download any image
  from any registry.

The `if-not-present` pull policy should be used **only** for specific runners
used by trusted builds and users.

Read the [pull policies documentation](../executors/docker.md#configure-how-runners-pull-images)
for more information.

## Systems with Docker installed

NOTE:
This applies to installations below 0.5.0 or ones that were upgraded to the
newer version.

When installing the GitLab Runner package on Linux systems with Docker installed,
`gitlab-runner` creates a user that has permission to access the `Docker`
daemon. This makes the jobs that run with the `shell` executor able to access `docker`
with full permissions and potentially allows root access to the server.

### Usage of SSH executor

**SSH executors are susceptible to MITM attack (man-in-the-middle)**, because of
missing `StrictHostKeyChecking` option. This will be fixed in one of the future
releases.

### Usage of Parallels executor

**Parallels executor is the safest possible option** because it uses full system
virtualization and with VM machines that are configured to run in the isolated
virtualization and VM machines that are configured to run in isolated
mode. It blocks access to all peripherals and shared folders.

## Cloning a runner

Runners use a token to identify to the GitLab Server. If you clone a runner then
the cloned runner could be picking up the same jobs for that token. This is a possible
attack vector to "steal" runner jobs.

## Security risks when using `GIT_STRATEGY: fetch` on shared environments

When you set [`GIT_STRATEGY`](https://docs.gitlab.com/ee/ci/runners/configure_runners.html#git-strategy)
to `fetch`, the runner attempts to reuse the local working copy of the Git repository.

Using a local copy can improve the performance of CI/CD jobs. However, any user with access to that reusable copy can add code that executes in other users' pipelines.

Git stores the contents of a submodule (a repository embedded inside another repository) in the parent repository's Git
reflog. As a result, after a project's submodules have been initially cloned, subsequent jobs can access the contents of
the submodules by running `git submodule update` in their script. This applies even if the submodules have been deleted
and the user that initiated the job doesn't have access to the submodule projects.

Use `GIT_STRATEGY: fetch` only when you trust all users who have access to the shared environment.

## Security hardening options

### Reduce the security risk of using privileged containers

If you must run CI/CD jobs that require the use of Docker's `--privileged` flag, you can take these steps to reduce the security risk:

- Run Docker containers with the `--privileged` flag enabled only on isolated and ephemeral virtual machines.
- Configure dedicated runners that are meant to execute jobs that require the use of Docker's `--privileged` flag. Then configure these runners to execute jobs only on protected branches.

### Network segmentation

GitLab Runner is designed to run user-controlled scripts. To reduce the
attack surface if a job is malicious, you can consider running them in their
own network segment. This would provide network separation from other
infrastructure and services.

All needs are unique, but for a cloud environment, this could include:

- Configuring runner virtual machines in their own network segment
- Blocking SSH access from the Internet to runner virtual machines
- Restricting traffic between runner virtual machines
- Filtering access to cloud provider metadata endpoints

NOTE:
All runners will need outbound network connectivity to
GitLab.com or your GitLab instance.
Most jobs will also require outbound network connectivity to
the Internet - for dependency pulling etc.

### Secure the runner host

If you are using a static host for a runner, whether bare-metal or virtual machine, you should implement security best practices for the host operating system.

Malicious code executed in the context of a CI job could compromise the host, so security protocols can help mitigate the impact. Other points to keep in mind include securing or removing files such as SSH keys from the host system that may enable an attacker to access other endpoints in the environment.
