---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/engineering/ux/technical-writing/#assignments
comments: false
---

# Security of running jobs

When using GitLab Runner you should be aware of potential security implications
when running your jobs.

## Usage of Shell executor

**Generally, it's unsafe to run tests with `shell` executors.** The jobs are run
with user's permissions (the GitLab Runner permissions) and can steal code from other
projects that are run on this server. Use only it for running the trusted builds.

## Usage of Docker executor

**Docker can be considered safe when running in non-privileged mode.** To make
such setup more secure it's advised to run jobs as a user (non-root) in Docker
containers with disabled sudo or dropped `SETUID` and `SETGID` capabilities.

On the other hand, there's a privileged mode which enables full access to the
host system, permission to mount and unmount volumes, and run nested containers.
It's not advised to run containers in privileged mode.

More granular permissions can be configured in non-privileged mode via the
`cap_add`/`cap_drop` settings.

## Usage of private Docker images with `if-not-present` pull policy

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

Read the [pull policies documentation](../executors/docker.md#how-pull-policies-work)
for more information.

## Systems with Docker installed

NOTE:
This applies to installations below 0.5.0 or ones that were upgraded to the
newer version.

When installing the package on Linux systems with Docker installed,
`gitlab-runner` will create a user that will have permission to access the `Docker`
daemon. This makes the jobs that run with the `shell` executor able to access `docker`
with full permissions and potentially allows root access to the server.

## Usage of SSH executor

**SSH executors are susceptible to MITM attack (man-in-the-middle)**, because of
missing `StrictHostKeyChecking` option. This will be fixed in one of the future
releases.

## Usage of Parallels executor

**Parallels executor is the safest possible option** because it uses full system
virtualization and with VM machines that are configured to run in the isolated
virtualization and VM machines that are configured to run in isolated
mode. It blocks access to all peripherals and shared folders.

## Cloning a runner

Runners use a token to identify to the GitLab Server. If you clone a runner then
the cloned runner could be picking up the same jobs for that token. This is a possible
attack vector to "steal" runner jobs.

## Network Segmentation

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
