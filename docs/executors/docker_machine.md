---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Install and register GitLab Runner for autoscaling with Docker Machine **(FREE)**

> The autoscaling feature was introduced in GitLab Runner 1.1.0.

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
costs. No new features will be added.

## Preparing the environment

To use the autoscale feature, Docker and GitLab Runner must be
installed in the same machine:

1. Log in to a new Linux-based machine that will serve as a bastion server
   where Docker will spawn new machines from
1. [Install GitLab Runner](../install/index.md)
1. Install Docker Machine from the [Docker Machine fork](https://gitlab.com/gitlab-org/ci-cd/docker-machine)
1. Optionally but recommended, prepare a
   [proxy container registry and a cache server](../configuration/speed_up_job_execution.md)
   to be used with the autoscaled runners

## Configuring GitLab Runner

1. Familiarize yourself with the core concepts of using `docker-machine` together
   with `gitlab-runner`:
      - Read [GitLab Runner Autoscaling](../configuration/autoscale.md)
      - Read [GitLab Runner MachineOptions](../configuration/advanced-configuration.md#the-runnersmachine-section)
1. The **first time** you're using Docker Machine, it is best to manually execute the
   `docker-machine create ...` command with your [Docker Machine Driver](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/tree/main/drivers).
   Run this command alongside with the options that you intend to configure in the
   [GitLab Runner MachineOptions](../configuration/advanced-configuration.md#the-runnersmachine-section) section.
   This will set up the Docker Machine environment properly and will also be a good
   validation of the specified options. After this, you can destroy the machine with
   `docker-machine rm [machine_name]` and start the runner.

   NOTE:
   Multiple concurrent requests to `docker-machine create` that are done
   **at first usage** are not good. When the `docker+machine` executor is used,
   the runner may spin up few concurrent `docker-machine create` commands. If
   Docker Machine was not used before in this environment, each started process
   tries to prepare SSH keys and SSL certificates (for Docker API authentication
   between GitLab Runner and Docker Engine on the autoscaled spawned machine), and these
   concurrent processes are disturbing each other. This can end with a non-working
   environment. That's why it's important to create a test machine manually the
   very first time you set up GitLab Runner with Docker Machine.
1. [Register a runner](../register/index.md#linux) and select the
   `docker+machine` executor when asked.
1. Edit [`config.toml`](../commands/index.md#configuration-file) and configure
   the runner to use Docker machine. Visit the dedicated page covering detailed
   information about [GitLab Runner Autoscaling](../configuration/autoscale.md).
1. Now, you can try and start a new pipeline in your project. In a few seconds,
   if you run `docker-machine ls` you should see a new machine being created.

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

   NOTE:
   Sending the [`SIGQUIT` signal](../commands/index.md#signals) will make the
   process stop gracefully. The process will stop accepting new jobs, and will exit
   as soon as the current jobs are finished.

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
executable. For example, to download and install `v0.16.2-gitlab.19`:

    ```shell
    curl -O "https://gitlab-docker-machine-downloads.s3.amazonaws.com/v0.16.2-gitlab.19/docker-machine-Linux-x86_64"
    cp docker-machine-Linux-x86_64 /usr/local/bin/docker-machine
    chmod +x /usr/local/bin/docker-machine
    ```

### Using GPUs on Google Compute Engine

> [Introduced](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/issues/34) in GitLab Docker Machine `0.16.2-gitlab.10` and GitLab Runner 13.9.

NOTE:
GPUs are [supported on every executor](../configuration/gpus.md). It is
not necessary to use Docker Machine just for GPU support. The Docker
Machine executor makes it easy to scale the GPU nodes up and down, but
this can also be done with the [Kubernetes executor](kubernetes.md).

You can use the Docker Machine [fork](#forked-version-of-docker-machine) to create
[Google Compute Engine instances with graphics processing units (GPUs)](https://cloud.google.com/compute/docs/gpus/).
GitLab Runner 13.9 is [required for GPUs to work in a Docker executor](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4585).

#### Docker Machine GPU options

To create an instance with GPUs, use these Docker Machine options:

|Option|Example|Description|
|------|-------|-----------|
|`--google-accelerator`|`type=nvidia-tesla-p4,count=1`|Specifies the type and number of GPU accelerators to attach to the instance (`type=TYPE,count=N` format)|
|`--google-maintenance-policy`|`TERMINATE`|Always use `TERMINATE` because [Google Cloud does not allow GPU instances to be live migrated](https://cloud.google.com/compute/docs/instances/live-migration-process).|
|`--google-machine-image`|`https://www.googleapis.com/compute/v1/projects/deeplearning-platform-release/global/images/family/tf2-ent-2-3-cu110`|The URL of a GPU-enabled operating system. See the [list of available images](https://cloud.google.com/deep-learning-vm/docs/images).|
|`--google-metadata`|`install-nvidia-driver=True`|This flag tells the image to install the NVIDIA GPU driver.|

These arguments map to [command-line arguments for `gcloud compute`](https://cloud.google.com/compute/docs/gpus/create-vm-with-gpus#gcloud_1).
See the [Google documentation on creating VMs with attached GPUs](https://cloud.google.com/compute/docs/gpus/create-vm-with-gpus)
for more details.

#### Verifying Docker Machine options

To prepare your system and test that GPUs can be created with Google Compute Engine:

1. [Set up the Google Compute Engine driver credentials](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/gce.md#credentials)
for Docker Machine. You may need to export environment variables to the
runner if your VM does not have a default service account. How
this is done depends on how the runner is launched. For example:

    - Via `systemd` or `upstart`: See the [documentation on setting custom environment variables](../configuration/init.md#setting-custom-environment-variables).
    - Via Kubernetes with the Helm Chart: Update [the `values.yaml` entry](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/blob/5e7c5c0d6e1159647d65f04ff2cc1f45bb2d5efc/values.yaml#L431-438).
    - Via Docker: Use the `-e` option (for example, `docker run -e GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json gitlab/gitlab-runner`).

1. Verify that `docker-machine` can create a virtual machine with your
   desired options. For example, to create an `n1-standard-1` machine
   with a single NVIDIA Telsa P4 accelerator, substitute
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

1. Once you have verified these options, configure the Docker executor
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
