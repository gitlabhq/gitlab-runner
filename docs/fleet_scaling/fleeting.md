---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Fleeting
---

[Fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting) is a library that GitLab Runner uses to provide a plugin-based abstraction for a cloud provider's instance groups.

The following executors use fleeting to scale runners:

- [Docker Autoscaler](../executors/docker_autoscaler.md)
- [Instance](../executors/instance.md)

## Find a fleeting plugin

GitLab maintains these official plugins:

| Cloud provider                                                             | Notes |
|----------------------------------------------------------------------------|-------|
| [Google Cloud](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud) | Uses [Google Cloud instance groups](https://docs.cloud.google.com/compute/docs/instance-groups) |
| [AWS](https://gitlab.com/gitlab-org/fleeting/plugins/aws)                  | Uses [AWS Auto Scaling groups](https://docs.aws.amazon.com/autoscaling/ec2/userguide/auto-scaling-groups.html) |
| [Azure](https://gitlab.com/gitlab-org/fleeting/plugins/azure)              | Uses Azure [Virtual Machine Scale Sets](https://learn.microsoft.com/en-us/azure/virtual-machine-scale-sets/overview). Only [Uniform orchestration](https://learn.microsoft.com/en-us/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-orchestration-modes#scale-sets-with-uniform-orchestration) mode is supported. |

The following plugins are community maintained:

| Cloud provider | OCI Reference | Notes |
|----------------|---------------|-------|
| [VMware vSphere](https://gitlab.com/santhanuv/fleeting-plugin-vmware-vsphere) | `registry.gitlab.com/santhanuv/fleeting-plugin-vmware-vsphere:latest` | Uses VMware vSphere to create and manage virtual machines by cloning from an existing template. Tested with [`govmomi vcsim`](https://github.com/vmware/govmomi/tree/main/vcsim) simulator and validated by community members against basic use cases. It might have limitations with restricted vSphere permissions. You can create related issues in the [Fleeting Plugin VMware vSphere project](https://gitlab.com/santhanuv/fleeting-plugin-vmware-vsphere/-/issues).|

Community maintained plugins are owned, built, hosted, and maintained by contributors outside of GitLab (the community).
GitLab owns and maintains the Fleeting library and API to provide static code review.
GitLab cannot test community plugins because we don't have access to all the necessary computing environments.
Community members should build, test, and publish plugins to an OCI repository and provide the reference on this page through merge requests.
The OCI reference should be accompanied by notes on the where to report issues, the support and stability level of the plugin, and where to find documentation.

## Configure a fleeting plugin

To configure fleeting, in the `config.toml`, use the [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section)
configuration section.

> [!note]
> The README.md file for each plugin contains important information regarding installation and configuration.

## Install a fleeting plugin

To install a fleeting plugin, use either the:

- OCI registry distribution (recommended)
- Manual binary installation

## Install with the OCI registry distribution

{{< history >}}

- [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4690) OCI registry distribution in GitLab Runner 16.11

{{< /history >}}

Plugins are installed to `~/.config/fleeting/plugins` on UNIX systems, and `%APPDATA%/fleeting/plugins` on Windows. To override
where plugins are installed, update the environment variable `FLEETING_PLUGIN_PATH`.

To install the fleeting plugin:

1. In the `config.toml`, in the `[runners.autoscaler]` section, add the fleeting plugin:

   {{< tabs >}}

   {{< tab title="AWS" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "aws:latest"
   ```

   {{< /tab >}}

   {{< tab title="Google Cloud" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "googlecloud:latest"
   ```

   {{< /tab >}}

   {{< tab title="Azure" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "azure:latest"
   ```

   {{< /tab >}}

   {{< /tabs >}}

1. Run `gitlab-runner fleeting install`.

### `plugin` formats

The `plugin` parameter supports the following formats:

- `<name>`
- `<name>:<version constraint>`
- `<repository>/<name>`
- `<repository>/<name>:<version constraint>`
- `<registry>/<repository>/<name>`
- `<registry>/<repository>/<name>:<version constraint>`

Where:

- `registry.gitlab.com` is the default registry.
- `gitlab-org/fleeting/plugins` is the default repository.
- `latest` is the default version.

### Version constraint formats

The `gitlab-runner fleeting install` command uses the version constraint to find the latest matching
version in the remote repository.

When GitLab Runner runs, it uses the version constraint to find the latest matching version that is installed locally.

Use the following version constraint formats:

| Format                    | Description |
|---------------------------|-------------|
| `latest`                  | Latest version. |
| `<MAJOR>`                 | Selects the major version. For example, `1` selects the version that matches `1.*.*`. |
| `<MAJOR>.<MINOR>`         | Selects the major and minor version. For example, `1.5` selects the latest version that matches `1.5.*`. |
| `<MAJOR>.<MINOR>.<PATCH>` | Selects the major and minor version, and patch. For example, `1.5.1` selects the version `1.5.1`. |

## Install binary manually

To manually install a fleeting plugin:

1. Download the fleeting plugin binary for your system:
   - [AWS](https://gitlab.com/gitlab-org/fleeting/plugins/aws/-/releases).
   - [Google Cloud](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud/-/releases)
   - [Azure](https://gitlab.com/gitlab-org/fleeting/plugins/azure/-/releases)
1. Ensure the binary has a name in the format of `fleeting-plugin-<name>`. For example, `fleeting-plugin-aws`.
1. Ensure the binary can be discovered from `$PATH`. For example, move it to `/usr/local/bin`.
1. In the `config.toml`, in the `[runners.autoscaler]` section, add the fleeting plugin. For example:

   {{< tabs >}}

   {{< tab title="AWS" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "fleeting-plugin-aws"
   ```

   {{< /tab >}}

   {{< tab title="Google Cloud" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "fleeting-plugin-googlecloud"
   ```

   {{< /tab >}}

   {{< tab title="Azure" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "fleeting-plugin-azure"
   ```

   {{< /tab >}}

   {{< /tabs >}}

## Fleeting plugin management

Use the following `fleeting` subcommands to manage fleeting plugins:

| Command                          | Description |
|----------------------------------|-------------|
| `gitlab-runner fleeting install` | Install the fleeting plugin from the OCI registry distribution. |
| `gitlab-runner fleeting list`    | List referenced plugins and the version used. |
| `gitlab-runner fleeting login`   | Sign in to private registries. |
