---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Install GitLab Runner from a GitLab repository using your package manager.
title: Install GitLab Runner using the official GitLab repositories
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

To install GitLab Runner, you can use a package from [the GitLab repository](https://packages.gitlab.com/runner/gitlab-runner).

## Supported distributions

GitLab provides packages for the following supported versions of Linux distributions with [Packagecloud](https://packages.gitlab.com/runner/gitlab-runner/). New runner `deb` or `rpm` packages for new OS distribution releases are added automatically when supported by Packagecloud.

<!-- supported_os_versions_list_start -->

### Deb-based Distributions

| Distribution | Supported Versions |
|--------------|--------------------|
| Debian | 15 Duke, 14 Forky, 13 Trixie, 12 Bookworm, 11 Bullseye |
| LinuxMint | 22.1 Xia, 22 Wilma, 21.3 Virginia, 21.2 Victoria, 21.1 Vera, 21 Vanessa |
| Raspbian | 15 Duke, 14 Forky, 13 Trixie, 12 Bookworm, 11 Bullseye |
| Ubuntu | 25.04 Plucky Puffin, 24.04 Lts Noble Numbat, 22.04 Jammy Jellyfish, 20.04 Focal Fossa, 18.04 Lts Bionic Beaver, 16.04 Lts Xenial Xerus |

### Rpm-based Distributions

| Distribution | Supported Versions |
|--------------|--------------------|
| Amazon Linux | 2025, 2023, 2022, 2 |
| Red Hat Enterprise Linux | 10, 9, 8, 7 |
| Fedora | 43, 42 |
| Oracle Linux | 10, 9, 8, 7 |
| openSUSE | 16.0, 15.6 |
| SUSE Linux Enterprise Server | 15.7, 15.6, 15.5, 15.4, 12.5 |

<!-- supported_os_versions_list_end -->

Depending on your setup, other Debian or RPM based distributions may also be supported. This refers to distributions that are derivative of a supported GitLab Runner distribution and that have compatible package repositories. For example, Deepin is a Debian derivative. So, the runner `deb` package should install and run on Deepin. You may also be able to [install GitLab Runner as a binary](linux-manually.md#using-binary-file)
on other Linux distributions.

> [!note]
> Packages for distributions that are not on the list are not available from our package repository. You can [install](linux-manually.md#using-debrpm-package) them manually by downloading the RPM or DEB package from our S3 bucket.

## Install GitLab Runner

To install GitLab Runner:

1. Add the official GitLab repository:

   {{< tabs >}}

   {{< tab title="Debian/Ubuntu/Mint" >}}

   1. Download the repository configuration script:

      ```shell
      curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" -o script.deb.sh
      ```

   1. Inspect the script before running it:

      ```shell
      less script.deb.sh
      ```

   1. Run the script:

      ```shell
      sudo bash script.deb.sh
      ```

   {{< /tab >}}

   {{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

   1. Download the repository configuration script:

      ```shell
      curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.rpm.sh" -o script.rpm.sh
      ```

   1. Inspect the script before running it:

      ```shell
      less script.rpm.sh
      ```

   1. Run the script:

      ```shell
      sudo bash script.rpm.sh
      ```

   {{< /tab >}}

   {{< /tabs >}}

1. Install the latest version of GitLab Runner, or skip to the next step to
   install a specific version:

   {{< alert type="note" >}}

   The `skel` directory usage is disabled by default to prevent
   [`No such file or directory` job failures](#error-no-such-file-or-directory-job-failures).

   {{< /alert >}}

   {{< tabs >}}

   {{< tab title="Debian/Ubuntu/Mint" >}}

   ```shell
   sudo apt install gitlab-runner
   ```

   {{< /tab >}}

   {{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

   ```shell
   sudo yum install gitlab-runner

   or

   sudo dnf install gitlab-runner
   ```

   {{< /tab >}}

   {{< /tabs >}}

   {{< alert type="note" >}}

   A FIPS 140-2 compliant version of GitLab Runner is
   available for RHEL distributions. You can install this version by using
   `gitlab-runner-fips` as the package name, instead of `gitlab-runner`.

   {{< /alert >}}

1. To install a specific version of GitLab Runner:

   {{< tabs >}}

   {{< tab title="Debian/Ubuntu/Mint" >}}

   {{< alert type="note" >}}

   As of `gitlab-runner` version `v17.7.1`, when you install a specific version of `gitlab-runner` that is not the latest
   version, you must explicitly install the required `gitlab-runner-helper-packages` for that version. This requirement
   exists due to an `apt`/`apt-get` limitation.

   {{< /alert >}}

   ```shell
   apt-cache madison gitlab-runner
   sudo apt install gitlab-runner=17.7.1-1 gitlab-runner-helper-images=17.7.1-1
   ```

   If you attempt to install a specific version of `gitlab-runner` without installing the same version of
   `gitlab-runner-helper-images`, you might encounter the following error:

   ```shell
   sudo apt install gitlab-runner=17.7.1-1
   ...
   The following packages have unmet dependencies:
    gitlab-runner : Depends: gitlab-runner-helper-images (= 17.7.1-1) but 17.8.3-1 is to be installed
   E: Unable to correct problems, you have held broken packages.
   ```

   {{< /tab >}}

   {{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

   ```shell
   yum list gitlab-runner --showduplicates | sort -r
   sudo yum install gitlab-runner-17.2.0-1
   ```

   {{< /tab >}}

   {{< /tabs >}}

1. [Register a runner](../register/_index.md).

After completing the above steps, a runner can be started and can be used with your projects!

Make sure that you read the [FAQ](../faq/_index.md) section which describes
some of the most common problems with GitLab Runner.

## Helper images package

The `gitlab-runner-helper-images` package contains pre-built helper container images that GitLab Runner uses during job execution.
These images provide the necessary tools and utilities to clone repositories, upload artifacts, and manage caches.

The `gitlab-runner-helper-images` package includes helper images for the following operating systems and architectures:

Alpine-based images (latest):

- `alpine-arm`
- `alpine-arm64`
- `alpine-riscv64`
- `alpine-s390x`
- `alpine-x86_64`
- `alpine-x86_64-pwsh`

Ubuntu-based images (24.04):

- `ubuntu-arm`
- `ubuntu-arm64`
- `ubuntu-ppc64le`
- `ubuntu-s390x`
- `ubuntu-x86_64`
- `ubuntu-x86_64-pwsh`

### Automatic helper image download

If a helper image for a specific operating system and architecture combination is not available on the host system,
GitLab Runner automatically downloads the required image when needed. Manual installation is not required for architectures
that are not included in the `gitlab-runner-helper-images package`. This automatic download ensures that the runner can support
additional architectures (such as `loong64`) without requiring manual intervention or separate package installations.

## Upgrade GitLab Runner

To install the latest version of GitLab Runner:

{{< tabs >}}

{{< tab title="Debian/Ubuntu/Mint" >}}

```shell
sudo apt update
sudo apt install gitlab-runner
```

{{< /tab >}}

{{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

```shell
sudo yum update
sudo yum install gitlab-runner
```

{{< /tab >}}

{{< /tabs >}}

## GPG signatures for package installation

The GitLab Runner project provides two types of GPG signatures for the package
installation method:

- [Repository metadata signing](#repository-metadata-signing)
- [Package signing](#package-signing)

### Repository metadata signing

To verify that the package information downloaded from the remote repository can be trusted,
the package manager uses repository metadata signing.

The signature is verified when you use a command like `apt-get update`, so the
information about available packages is updated **before any package is downloaded and
installed**. Verification failure should also cause the package manager to reject the
metadata. This means that you cannot download and install any package from the repository
until the problem that caused the signature mismatch is found and resolved.

GPG public keys used for package metadata signature verification are installed automatically
on first installation done with the instructions above. For key updates in the future,
existing users need to manually download and install the new keys.

We use one key for all our projects hosted under <https://packages.gitlab.com>. You can find
the details about the key used in the [Linux package documentation](https://docs.gitlab.com/omnibus/update/package_signatures/#package-repository-metadata-signing-keys).
This documentation page lists also
[all keys used in the past](https://docs.gitlab.com/omnibus/update/package_signatures/#previous-keys).

### Package signing

Repository metadata signing proves that the downloaded version information originates
at <https://packages.gitlab.com>. It does not prove the integrity of the packages themselves.
Whatever was uploaded to <https://packages.gitlab.com> - authorized or not - is properly
verified until the metadata transfer from repository to the user was not affected.

With package signing, each package is signed when it's built. Until you can trust
the build environment and the secrecy of the used GPG key, you cannot verify package authenticity.
A valid signature on the package proves that its origin is authenticated and its integrity was not violated.

Package signing verification is enabled by default only in some of the Debian/RPM based distributions.
To use this type of verification, you might need to adjust the configuration.

GPG keys used for package signature verification can be different for each of the repositories
hosted at <https://packages.gitlab.com>. The GitLab Runner project uses its own key pair for this
type of the signature.

#### RPM-based distributions

The RPM format contains a full implementation of GPG signing functionality, and thus is fully
integrated with the package management systems based upon that format.

You can find the technical description of how to configure package signature
verification for RPM-based distributions in the [Linux package documentation](https://docs.gitlab.com/omnibus/update/package_signatures/#rpm-based-distributions).
The GitLab Runner differences are:

- The public key package that should be installed is named `gpg-pubkey-35dfa027-60ba0235`.
- The repository file for RPM-based distributions is named `/etc/yum.repos.d/runner_gitlab-runner.repo`
  (for the stable release) or `/etc/yum.repos.d/runner_unstable.repo` (for the unstable releases).
- The [package signing public key](#current-gpg-public-key) can be imported from
  `https://packages.gitlab.com/runner/gitlab-runner/gpgkey/runner-gitlab-runner-49F16C5CC3A0F81F.pub.gpg`.

#### Debian-based distributions

The `deb` format does not officially contain a default and included method for signing packages.
The GitLab Runner project uses the `dpkg-sig` tool for signing and verifying signatures on packages. This
method supports only manual verification of packages.

To verify a `deb` package:

1. Install `dpkg-sig`:

   ```shell
   apt update && apt install dpkg-sig
   ```

1. Download and import the [package signing public key](#current-gpg-public-key):

   ```shell
   curl -JLO "https://packages.gitlab.com/runner/gitlab-runner/gpgkey/runner-gitlab-runner-49F16C5CC3A0F81F.pub.gpg"
   gpg --import runner-gitlab-runner-49F16C5CC3A0F81F.pub.gpg
   ```

1. Verify downloaded package with `dpkg-sig`:

   ```shell
   dpkg-sig --verify gitlab-runner_amd64.deb
   Processing gitlab-runner_amd64.deb...
   GOODSIG _gpgbuilder 931DA69CFA3AFEBBC97DAA8C6C57C29C6BA75A4E 1623755049
   ```

   If a package has an invalid signature or signed with an invalid key (for example
   a revoked one), the output is similar to the following:

   ```shell
   dpkg-sig --verify gitlab-runner_amd64.deb
   Processing gitlab-runner_amd64.deb...
   BADSIG _gpgbuilder
   ```

   If the key is not present in the user's keyring, the output is similar to:

   ```shell
   dpkg-sig --verify gitlab-runner_amd64.v13.1.0.deb
   Processing gitlab-runner_amd64.v13.1.0.deb...
   UNKNOWNSIG _gpgbuilder 880721D4
   ```

#### Current GPG public key

Download the current public GPG key used for package signing from
`https://packages.gitlab.com/runner/gitlab-runner/gpgkey/runner-gitlab-runner-49F16C5CC3A0F81F.pub.gpg`.

| Key Attribute | Value |
|---------------|-------|
| Name          | `GitLab, Inc.` |
| EMail         | `support@gitlab.com` |
| Fingerprint   | `931D A69C FA3A FEBB C97D  AA8C 6C57 C29C 6BA7 5A4E` |
| Expiry        | `2026-04-28` |

> [!note]
> The same key is used by the GitLab Runner project to sign `release.sha256` files for the S3 releases
> available in the `<https://gitlab-runner-downloads.s3.dualstack.us-east-1.amazonaws.com>` bucket.

#### Previous GPG public keys

Keys used in the past can be found in the table below.

For keys that were revoked, it's highly recommended to remove them from the package signing
verification configuration.

Signatures made by the following keys should not be trusted anymore.

| Sl. No. | Key Fingerprint                                      | Status    | Expiry Date  | Download (revoked keys only) |
|---------|------------------------------------------------------|-----------|--------------|------------------------------|
| 1       | `3018 3AC2 C4E2 3A40 9EFB  E705 9CE4 5ABC 8807 21D4` | `revoked` | `2021-06-08` | [revoked key](https://gitlab.com/gitlab-org/gitlab-runner/-/blob/main/docs/install/gpg-keys/9CE45ABC880721D4.pub.gpg) |
| 2       | `09E5 7083 F34C CA94 D541  BC58 A674 BF81 35DF A027` | `revoked` | `2023-04-26` | [revoked key](https://gitlab.com/gitlab-org/gitlab-runner/-/blob/main/docs/install/gpg-keys/A674BF8135DFA027.pub.gpg) |

## Troubleshooting

Here are some tips on troubleshooting and resolving issues when installing GitLab Runner.

### Error: `No such file or directory` job failures

Sometimes the default skeleton (`skel`) directory
causes issues for GitLab Runner, and it fails to run a job.
See [issue 4449](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4449) and
[issue 1379](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1379).

To avoid this, when you install GitLab Runner, a `gitlab-runner` user is
created, and by default, the home directory is created without any skeleton in
it. Shell configuration added to the home directory with the usage of `skel` may interfere with the job execution.
This configuration can introduce unexpected problems like the ones mentioned above.

If you had created the runner before the avoidance of `skel` was made
the default behavior, you can try removing the following dotfiles:

```shell
sudo rm /home/gitlab-runner/.profile
sudo rm /home/gitlab-runner/.bashrc
sudo rm /home/gitlab-runner/.bash_logout
```

If you need to use the `skel` directory to populate the newly
created `$HOME` directory, you must set the `GITLAB_RUNNER_DISABLE_SKEL` variable explicitly
to `false` before you install the runner:

{{< tabs >}}

{{< tab title="Debian/Ubuntu/Mint" >}}

```shell
export GITLAB_RUNNER_DISABLE_SKEL=false; sudo -E apt-get install gitlab-runner
```

{{< /tab >}}

{{< tab title="RHEL/CentOS/Fedora/Amazon Linux" >}}

```shell
export GITLAB_RUNNER_DISABLE_SKEL=false; sudo -E yum install gitlab-runner
```

{{< /tab >}}

{{< /tabs >}}
