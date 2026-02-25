---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Install step runner manually to use GitLab Functions
title: Install step runner manually
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

The step runner is a binary that allows GitLab Runner to execute GitLab Functions on executors without
native functions support. For these executors, you must install the step runner
binary on the host or container where your jobs run before you can use functions in your pipelines.

## Executors that require manual step runner installation

Whether you need to install step-runner manually depends on your executor.
The following table shows which executors require you to install step runner manually:

| Executor          | Manual installation required |
|-------------------|------------------------------|
| Shell             | Yes                          |
| SSH               | Yes                          |
| Kubernetes        | Yes                          |
| VirtualBox        | Yes                          |
| Parallels         | Yes                          |
| Custom            | Yes                          |
| Instance          | Yes                          |
| Docker            | Only on Windows              |
| Docker Autoscaler | Only on Windows              |
| Docker Machine    | Only on Windows              |

For executors that don't require manual installation, `gitlab-runner-helper` acts as the step runner.
The `step-runner` binary is neither present nor required on these executors.

### Variable access restrictions

On executors where you install step runner manually, the step runner has restricted access to job variables and environment variables:

| Syntax               | Available values                                                                                                                                                                        |
|----------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `${{ vars.<name> }}` | Job variables with the prefix `CI_`, `DOCKER_`, or `GITLAB_` only.                                                                                                                      |
| `${{ env.<name> }}`  | `HTTPS_PROXY`, `HTTP_PROXY`, `NO_PROXY`, `http_proxy`, `https_proxy`, `no_proxy`, `all_proxy`, `LANG`, `LC_ALL`, `LC_CTYPE`, `LOGNAME`, `USER`, `PATH`, `SHELL`, `TERM`, `TMPDIR`, `TZ` |

## Install step runner manually

Pre-compiled binaries for multiple platforms are available from the
[step runner releases page](https://gitlab.com/gitlab-org/step-runner/-/releases).
Supported platforms include Windows, Linux, macOS, and FreeBSD across multiple
architectures (amd64, arm64, 386, arm, s390x, ppc64le).

### Verify authenticity of the binary

Before you install, verify that the binary hasn't been tampered with and comes from
the official GitLab team.

1. Download and import the GPG public key:

   ```shell
   # All platforms (requires gpg installed: https://gnupg.org/download/)
   curl -o step-runner.pub.gpg "https://gitlab.com/gitlab-org/step-runner/-/package_files/257922684/download"
   gpg --import step-runner.pub.gpg
   gpg --fingerprint
   ```

   Verify the imported key matches the following:

   | Key attribute | Value                                                |
   |---------------|------------------------------------------------------|
   | Name          | `GitLab, Inc.`                                       |
   | Email         | `support@gitlab.com`                                 |
   | Fingerprint   | `0FCD 59B1 6F4A 62D0 3839  27A5 42FF CA71 62A5 35F5` |
   | Expiry        | `2029-01-05`                                         |

1. From the [releases page](https://gitlab.com/gitlab-org/step-runner/-/releases), download the following files:

   - The binary for your platform (for example, `step-runner-linux-amd64` or `step-runner-darwin-arm64`)
   - `step-runner-release.sha256`
   - `step-runner-release.sha256.asc`

1. Verify the GPG signature:

   ```shell
   # All platforms (requires gpg)
   gpg --verify step-runner-release.sha256.asc step-runner-release.sha256
   ```

   The output should include a `Good signature` message.

1. Verify the binary checksum:

   ```shell
   # Linux
   sha256sum -c step-runner-release.sha256
   ```

   ```shell
   # macOS
   shasum -a 256 -c step-runner-release.sha256
   ```

   ```shell
   # Windows (PowerShell) — replace 'step-runner-windows-amd64.exe' with your binary name
   $binary = "step-runner-windows-amd64.exe"
   $expected = (Select-String -Path "step-runner-release.sha256" -Pattern $binary).Line.Split(" ")[0]
   $actual = (Get-FileHash -Algorithm SHA256 $binary).Hash.ToLower()
   if ($actual -eq $expected) { "OK" } else { "FAILED: checksum mismatch" }
   ```

   The output should show `OK` for your binary.

### Add step-runner to PATH

After you download and verify the binary, make it available on the `PATH` of the
instance where your jobs run. This instance might be the host machine or a container,
depending on your executor.

1. Rename the binary to `step-runner` (or `step-runner.exe` on Windows):

   ```shell
   mv step-runner-<os>-<arch> step-runner
   ```

1. On Unix-like systems, make the binary executable:

   ```shell
   chmod +x step-runner
   ```

1. Move the binary to a directory on your `PATH`:

   ```shell
   mv step-runner /usr/local/bin/
   ```
