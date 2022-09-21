---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Using Graphical Processing Units (GPUs) **(FREE)**

> Introduced in GitLab Runner 13.9.

GitLab Runner supports the use of Graphical Processing Units (GPUs).
The following section describes the required configuration to enable GPUs
for various executors.

## Shell executor

No runner configuration is needed.

## Docker executor

Use the [`gpus` configuration option in the `runners.docker` section](advanced-configuration.md#the-runnersdocker-section).
For example:

```toml
[runners.docker]
    gpus = "all"
```

## Docker Machine executor

See the [documentation for the GitLab fork of Docker Machine](../executors/docker_machine.md#using-gpus-on-google-compute-engine).

## Kubernetes executor

No runner configuration should be needed. Be sure to check that
[the node selector chooses a node with GPU support](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/).

GitLab Runner has been [tested on Amazon Elastic Kubernetes Service](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4355)
with [GPU-enabled instances](https://docs.aws.amazon.com/dlami/latest/devguide/gpu.html).

## Validate that GPUs are enabled

You can use runners with NVIDIA GPUs. For NVIDIA GPUs, one
way to ensure that a GPU is enabled for a CI job is to run `nvidia-smi`
at the beginning of the script. For example:

```yaml
train:
  script:
    - nvidia-smi
```

If GPUs are enabled, the output of `nvdia-smi` displays the available devices. In
the following example, a single NVIDIA Tesla P4 is enabled:

```shell
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

If the hardware does not support a GPU, `nvidia-smi` should fail either because
it's missing or because it can't communicate with the driver:

```shell
modprobe: ERROR: could not insert 'nvidia': No such device
NVIDIA-SMI has failed because it couldn't communicate with the NVIDIA driver. Make sure that the latest NVIDIA driver is installed and running.
```
