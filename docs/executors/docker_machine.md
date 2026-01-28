---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Install and register GitLab Runner for autoscaling with Docker Machine
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

{{< alert type="note" >}}

The Docker Machine executor was deprecated in GitLab 17.5 and is scheduled for removal in GitLab 20.0 (May 2027).
While we continue to support the Docker Machine executor till GitLab 20.0, we do not plan to add new features.
We will address only critical bugs that could prevent CI/CD job execution or affect running costs.
If you're using the Docker Machine executor on Amazon Web Services (AWS) EC2,
Microsoft Azure Compute, or Google Compute Engine (GCE), you should migrate to the
[GitLab Runner Autoscaler](../runner_autoscale/_index.md).

{{< /alert >}}

For an overview of the autoscale architecture, take a look at the
[comprehensive documentation on autoscaling](../configuration/autoscale.md).

## Forked version of Docker machine

Docker has [deprecated Docker Machine](https://gitlab.com/gitlab-org/gitlab/-/issues/341856). However,
GitLab maintains a [Docker Machine fork](https://gitlab.com/gitlab-org/ci-cd/docker-machine)
for GitLab Runner users who rely on the Docker Machine executor. This fork is
based on the latest `main` branch of `docker-machine` with
some additional patches for the following bugs:

- [Make DigitalOcean driver RateLimit aware](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/2)
- [Add backoff to Google driver operations check](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/7)
- [Add `--google-min-cpu-platform` option for machine creation](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/9)
- [Use cached IP for Google driver](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/15)
- [Use cached IP for AWS driver](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/14)
- [Add support for using GPUs in Google Compute Engine](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/48)
- [Support running AWS instances with IMDSv2](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/49)

The intent of the [Docker Machine fork](https://gitlab.com/gitlab-org/ci-cd/docker-machine) is to only fix critical issues and bugs which affect running
costs. We don't plan to add any new features.

## Preparing the environment

To use the autoscale feature, Docker and GitLab Runner must be
installed in the same machine:

1. Sign in to a new Linux-based machine that can function as a bastion server where Docker creates new machines.
1. [Install GitLab Runner](../install/_index.md).
1. Install Docker Machine from the [Docker Machine fork](https://gitlab.com/gitlab-org/ci-cd/docker-machine).
1. Optionally but recommended, prepare a
   [proxy container registry and a cache server](../configuration/speed_up_job_execution.md)
   to be used with the autoscaled runners.

## Configuring GitLab Runner

1. Familiarize yourself with the core concepts of using `docker-machine`
   with `gitlab-runner`:
   - Read [GitLab Runner Autoscaling](../configuration/autoscale.md)
   - Read [GitLab Runner MachineOptions](../configuration/advanced-configuration.md#the-runnersmachine-section)
1. The **first time** you're using Docker Machine, it is best to manually execute the
   `docker-machine create ...` command with your [Docker Machine Driver](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/tree/main/drivers).
   Run this command with the options that you intend to configure in the
   [MachineOptions](../configuration/advanced-configuration.md#the-runnersmachine-section) under the `[runners.machine]` section.
   This approach sets up the Docker Machine environment properly and validates
   the specified options. After this, you can destroy the machine with
   `docker-machine rm [machine_name]` and start the runner.

   {{< alert type="note" >}}

   Multiple concurrent requests to `docker-machine create` that are done
   **at first usage** are not good. When the `docker+machine` executor is used,
   the runner may spin up few concurrent `docker-machine create` commands.
   If Docker Machine is new to this environment, each process tries to create
   SSH keys and SSL certificates for Docker API authentication. This action causes the
   concurrent processes to interfere with each other. This can end with a non-working
   environment. That's why it's important to create a test machine manually the
   very first time you set up GitLab Runner with Docker Machine.

   1. [Register a runner](../register/_index.md) and select the
      `docker+machine` executor when asked.
   1. Edit [`config.toml`](../commands/_index.md#configuration-file) and configure
      the runner to use Docker machine. Visit the dedicated page covering detailed
      information about [GitLab Runner Autoscaling](../configuration/autoscale.md).
   1. Now, you can try and start a new pipeline in your project. In a few seconds,
      if you run `docker-machine ls` you should see a new machine being created.

   {{< /alert >}}

## Upgrading GitLab Runner

1. Check if your operating system is configured to automatically restart GitLab
   Runner (for example, by checking its service file):
   - **if yes**, ensure that service manager is [configured to use `SIGQUIT`](../configuration/init.md)
     and use the service's tools to stop the process:

     ```shell
     # For systemd
     sudo systemctl stop gitlab-runner

     # For upstart
     sudo service gitlab-runner stop
     ```

   - **if no**, you may stop the process manually:

     ```shell
     sudo killall -SIGQUIT gitlab-runner
     ```

   {{< alert type="note" >}}

   Sending the [`SIGQUIT` signal](../commands/_index.md#signals) makes the
   process stop gracefully. The process stops accepting new jobs, and exits
   as soon as the current jobs are finished.

   {{< /alert >}}

1. Wait until GitLab Runner exits. You can check its status with `gitlab-runner status`
   or await a graceful shutdown for up to 30 minutes with:

   ```shell
   for i in `seq 1 180`; do # 1800 seconds = 30 minutes
       gitlab-runner status || break
       sleep 10
   done
   ```

1. You can now safely install the new version of GitLab Runner without interrupting any jobs.

## Using the forked version of Docker Machine

### Install

1. Download the [appropriate `docker-machine` binary](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/releases).
   Copy the binary to a location accessible to `PATH` and make it
   executable. For example, to download and install `v0.16.2-gitlab.43`:

   ```shell
   curl -O "https://gitlab-docker-machine-downloads.s3.amazonaws.com/v0.16.2-gitlab.43/docker-machine-Linux-x86_64"
   cp docker-machine-Linux-x86_64 /usr/local/bin/docker-machine
   chmod +x /usr/local/bin/docker-machine
   ```

### Using GPUs on Google Compute Engine

{{< alert type="note" >}}

GPUs are [supported on every executor](../configuration/gpus.md). It is
not necessary to use Docker Machine just for GPU support. The Docker
Machine executor scales the GPU nodes up and down.
You can also use the [Kubernetes executor](kubernetes/_index.md) for this purpose.

{{< /alert >}}

You can use the Docker Machine [fork](#forked-version-of-docker-machine) to create
[Google Compute Engine instances with graphics processing units (GPUs)](https://docs.cloud.google.com/compute/docs/gpus).

#### Docker Machine GPU options

To create an instance with GPUs, use these Docker Machine options:

| Option                        | Example                        | Description |
|-------------------------------|--------------------------------|-------------|
| `--google-accelerator`        | `type=nvidia-tesla-p4,count=1` | Specifies the type and number of GPU accelerators to attach to the instance (`type=TYPE,count=N` format) |
| `--google-maintenance-policy` | `TERMINATE`                    | Always use `TERMINATE` because [Google Cloud does not allow live migration of GPU instances](https://docs.cloud.google.com/compute/docs/instances/live-migration-process). |
| `--google-machine-image`      | `https://www.googleapis.com/compute/v1/projects/deeplearning-platform-release/global/images/family/tf2-ent-2-3-cu110` | The URL of a GPU-enabled operating system. See the [list of available images](https://docs.cloud.google.com/deep-learning-vm/docs/images). |
| `--google-metadata`           | `install-nvidia-driver=True`   | This flag tells the image to install the NVIDIA GPU driver. |

These arguments map to [command-line arguments for `gcloud compute`](https://docs.cloud.google.com/compute/docs/gcloud-compute).
See the [Google documentation on creating VMs with attached GPUs](https://docs.cloud.google.com/compute/docs/gpus/create-vm-with-gpus)
for more details.

#### Verifying Docker Machine options

To prepare your system and test that GPUs can be created with Google Compute Engine:

1. [Set up the Google Compute Engine driver credentials](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/gce.md#credentials)
   for Docker Machine. You may need to export environment variables to the
   runner if your VM does not have a default service account. How
   this is done depends on how the runner is launched. For example, by using:

   - `systemd` or `upstart`: See the [documentation on setting custom environment variables](../configuration/init.md#setting-custom-environment-variables).
   - Kubernetes with the Helm Chart: Update [the `values.yaml` entry](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/blob/5e7c5c0d6e1159647d65f04ff2cc1f45bb2d5efc/values.yaml#L431-438).
   - Docker: Use the `-e` option (for example, `docker run -e GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json gitlab/gitlab-runner`).

1. Verify that `docker-machine` can create a virtual machine with your
   desired options. For example, to create an `n1-standard-1` machine
   with a single NVIDIA Tesla P4 accelerator, substitute
   `test-gpu` with a name and run:

   ```shell
   docker-machine create --driver google --google-project your-google-project \
     --google-disk-size 50 \
     --google-machine-type n1-standard-1 \
     --google-accelerator type=nvidia-tesla-p4,count=1 \
     --google-maintenance-policy TERMINATE \
     --google-machine-image https://www.googleapis.com/compute/v1/projects/deeplearning-platform-release/global/images/family/tf2-ent-2-3-cu110 \
     --google-metadata "install-nvidia-driver=True" test-gpu
   ```

1. To verify the GPU is active, SSH into the machine and run `nvidia-smi`:

   ```shell
   $ docker-machine ssh test-gpu sudo nvidia-smi
   +-----------------------------------------------------------------------------+
   | NVIDIA-SMI 450.51.06    Driver Version: 450.51.06    CUDA Version: 11.0     |
   |-------------------------------+----------------------+----------------------+
   | GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
   | Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
   |                               |                      |               MIG M. |
   |===============================+======================+======================|
   |   0  Tesla P4            Off  | 00000000:00:04.0 Off |                    0 |
   | N/A   43C    P0    22W /  75W |      0MiB /  7611MiB |      3%      Default |
   |                               |                      |                  N/A |
   +-------------------------------+----------------------+----------------------+

   +-----------------------------------------------------------------------------+
   | Processes:                                                                  |
   |  GPU   GI   CI        PID   Type   Process name                  GPU Memory |
   |        ID   ID                                                   Usage      |
   |=============================================================================|
   |  No running processes found                                                 |
   +-----------------------------------------------------------------------------+
   ```

1. Remove this test instance to save money:

   ```shell
   docker-machine rm test-gpu
   ```

#### Configuring GitLab Runner

1. After you have verified these options, configure the Docker executor
   to use all available GPUs in the [`runners.docker` configuration](../configuration/advanced-configuration.md#the-runnersdocker-section).
   Then add the Docker Machine options to your [`MachineOptions` settings in the GitLab Runner `runners.machine` configuration](../configuration/advanced-configuration.md#the-runnersmachine-section). For example:

   ```toml
   [runners.docker]
     gpus = "all"
   [runners.machine]
     MachineOptions = [
       "google-project=your-google-project",
       "google-disk-size=50",
       "google-disk-type=pd-ssd",
       "google-machine-type=n1-standard-1",
       "google-accelerator=count=1,type=nvidia-tesla-p4",
       "google-maintenance-policy=TERMINATE",
       "google-machine-image=https://www.googleapis.com/compute/v1/projects/deeplearning-platform-release/global/images/family/tf2-ent-2-3-cu110",
       "google-metadata=install-nvidia-driver=True"
     ]
   ```

## Troubleshooting

When working with the Docker Machine executor, you might encounter the following issues.

### Error: Error creating machine

When installing Docker Machine, you might encounter an error that states
`ERROR: Error creating machine: Error running provisioning: error installing docker`.

Docker Machine attempts to install Docker on a newly provisioned
virtual machine using this script:

```shell
if ! type docker; then curl -sSL "https://get.docker.com" | sh -; fi
```

If the `docker` command succeeds, Docker Machine assumes Docker
is installed and continues.

If it does not succeed, Docker Machine attempts to download
and run the script at `https://get.docker.com`. If the installation
fails, it's possible the operating system is no longer supported by
Docker.

To troubleshoot this issue, you can enable debugging on Docker
Machine by setting `MACHINE_DEBUG=true` in the environment
where GitLab Runner is installed.

### Error: Cannot connect to the Docker daemon

The job might fail during the prepare stage with an error message:

```plaintext
Preparing environment
ERROR: Job failed (system failure): prepare environment: Cannot connect to the Docker daemon at tcp://10.200.142.223:2376. Is the docker daemon running? (docker.go:650:120s). Check https://docs.gitlab.com/runner/shells/#shell-profile-loading for more information
```

This error occurs when the Docker daemon fails to start in the expected time in the VM created
by the Docker Machine executor. To fix this issue, increase the `wait_for_services_timeout` value in
the [`[runners.docker]`](../configuration/advanced-configuration.md#the-runnersdocker-section) section.
