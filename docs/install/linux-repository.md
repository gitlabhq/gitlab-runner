---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Install GitLab Runner using the official GitLab repositories **(FREE)**

We provide packages for the following supported versions of Linux distributions with [packagecloud](https://packages.gitlab.com/runner/gitlab-runner/).
Depending on your setup, other `deb` or `rpm` based distributions may also be supported. You may also be able to [install GitLab Runner as a binary](linux-manually.md#using-binary-file).
on other Linux distributions.

| Distribution | Support Information |
|--------------|---------------------|
| Debian       | <https://wiki.debian.org/LTS> |
| Ubuntu       | <https://wiki.ubuntu.com/Releases>
| LinuxMint    | <https://linuxmint.com/download_all.php> |
| Raspbian     | |
| RHEL         | <https://access.redhat.com/product-life-cycles?product=Red%20Hat%20Enterprise%20Linux> |
| Oracle Linux | <https://endoflife.date/oraclelinux> |
| Fedora       | <https://docs.fedoraproject.org/en-US/releases/eol/> |
| Amazon Linux | <https://aws.amazon.com/linux/> |

NOTE:
Packages for distributions that are not on the list are currently not available from our package repository. You can [install](linux-manually.md#using-debrpm-package) them manually by downloading the RPM package from our S3 bucket.

## Prerequisites

If you want to use the [Docker executor](../executors/docker.md), make sure to install Docker before
using GitLab Runner. [Read how to install Docker for your distribution](https://docs.docker.com/engine/install/).

## Installing GitLab Runner

To install GitLab Runner:

1. Add the official GitLab repository:

   For Debian/Ubuntu/Mint:

   ```shell
   curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" | sudo bash
   ```

   For RHEL/CentOS/Fedora:

   ```shell
   curl -L "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.rpm.sh" | sudo bash
   ```

   NOTE:
   Debian users should use [APT pinning](#apt-pinning).

1. Install the latest version of GitLab Runner, or skip to the next step to
   install a specific version:

   NOTE:
   [Starting with GitLab Runner 14.0](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4845)
   the `skel` directory usage is [disabled](#disable-skel) by default to prevent
   [`No such file or directory` job failures](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1379)

   For Debian/Ubuntu/Mint:

   ```shell
   sudo apt-get install gitlab-runner
   ```

   For RHEL/CentOS/Fedora:

   ```shell
   sudo yum install gitlab-runner
   ```

   NOTE:
   In GitLab 14.7 and later, a FIPS 140-2 compliant version of GitLab Runner is
   available for RHEL distributions. You can install this version by using
   `gitlab-runner-fips` as the package name, instead of `gitlab-runner`.

1. To install a specific version of GitLab Runner:

   For DEB based systems:

   ```shell
   apt-cache madison gitlab-runner
   sudo apt-get install gitlab-runner=10.0.0
   ```

   For RPM based systems:

   ```shell
   yum list gitlab-runner --showduplicates | sort -r
   sudo yum install gitlab-runner-10.0.0-1
   ```

1. [Register a runner](../register/index.md).

After completing the step above, a runner should be started and be
ready to be used by your projects!

Make sure that you read the [FAQ](../faq/index.md) section which describes
some of the most common problems with GitLab Runner.

### APT pinning

A native package called `gitlab-ci-multi-runner` is available in
Debian Stretch. By default, when installing `gitlab-runner`, that package
from the official repositories will have a higher priority.

If you want to use our package, you should manually set the source of
the package. The best way is to add the pinning configuration file.

If you do this, the next update of the GitLab Runner package - whether it will
be done manually or automatically - will be done using the same source:

```shell
cat <<EOF | sudo tee /etc/apt/preferences.d/pin-gitlab-runner.pref
Explanation: Prefer GitLab provided packages over the Debian native ones
Package: gitlab-runner
Pin: origin packages.gitlab.com
Pin-Priority: 1001
EOF
```

## Updating GitLab Runner

Simply execute to install latest version:

For Debian/Ubuntu/Mint:

```shell
sudo apt-get update
sudo apt-get install gitlab-runner
```

For RHEL/CentOS/Fedora:

```shell
sudo yum update
sudo yum install gitlab-runner
```

## GPG signatures for package installation

To increase user's confidence about installed software, the GitLab Runner project provides
two types of GPG signatures for the package installation method: repository metadata
signing and package signing.

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
the details about the currently used key and technical description of how to update the key when
needed [in Omnibus GitLab documentation](https://docs.gitlab.com/omnibus/update/package_signatures#package-repository-metadata-signing-keys).
This documentation page lists also
[all keys used in the past](https://docs.gitlab.com/omnibus/update/package_signatures#previous-keys).

### Packages signing

Repository metadata signing proves that the downloaded version information originates
at <https://packages.gitlab.com>. It does not prove the integrity of the packages themselves.
Whatever was uploaded to <https://packages.gitlab.com> - authorized or not - will be properly
verified until the metadata transfer from repository to the user was not affected.

This is where packages signing comes in.

With package signing, each package is signed when it's built. So until you can trust
the build environment and the secrecy of the used GPG key, the valid signature on the package
will prove that its origin is authenticated and its integrity was not violated.

Packages signing verification is enabled by default only in some of the DEB/RPM based distributions,
so users wanting to have this kind of verification may need to adjust the configuration.

GPG keys used for packages signature verification can be different for each of the repositories
hosted at <https://packages.gitlab.com>. The GitLab Runner project uses its own key pair for this
type of the signature.

#### RPM-based distributions

The RPM format contains a full implementation of GPG signing functionality, and thus is fully
integrated with the package management systems based upon that format.

You can find the technical description of how to configure package signature
verification for RPM-based distributions in [the Omnibus GitLab documentation](https://docs.gitlab.com/omnibus/update/package_signatures#rpm-based-distributions).
The GitLab Runner differences are:

- The public key package that should be installed is named `gpg-pubkey-35dfa027-60ba0235`.

- The repository file for RPM based distributions will be named `/etc/yum.repos.d/runner_gitlab-runner.repo`
  (for the stable release) or `/etc/yum.repos.d/runner_unstable.repo` (for the unstable releases).

- The [package signing public key](#current-gpg-public-key) can be imported from
  <https://packages.gitlab.com/runner/gitlab-runner/gpgkey/runner-gitlab-runner-4C80FB51394521E9.pub.gpg>.

#### DEB-based distributions

The DEB format does not officially contain a default and included method for signing packages.
The GitLab Runner project uses `dpkg-sig` tool for signing and verifying signatures on packages. This
method supports only manual verification of packages.

1. Install `dpkg-sig`

    ```shell
    apt-get update && apt-get install dpkg-sig
    ```

1. Download and import the [package signing public key](#current-gpg-public-key)

    ```shell
    curl -JLO "https://packages.gitlab.com/runner/gitlab-runner/gpgkey/runner-gitlab-runner-4C80FB51394521E9.pub.gpg"
    gpg --import runner-gitlab-runner-4C80FB51394521E9.pub.gpg
    ```

1. Verify downloaded package with `dpkg-sig`

    ```shell
    dpkg-sig --verify gitlab-runner_amd64.deb
    Processing gitlab-runner_amd64.deb...
    GOODSIG _gpgbuilder 09E57083F34CCA94D541BC58A674BF8135DFA027 1623755049
    ```

   Verification of package with invalid signature or signed with an invalid key (for example
   a revoked one) will generate an output similar to:

    ```shell
    dpkg-sig --verify gitlab-runner_amd64.deb
    Processing gitlab-runner_amd64.deb...
    BADSIG _gpgbuilder
    ```

    If the key is not present in the user's keyring, the output will be similar to:

    ```shell
    dpkg-sig --verify gitlab-runner_amd64.v13.1.0.deb
    Processing gitlab-runner_amd64.v13.1.0.deb...
    UNKNOWNSIG _gpgbuilder 880721D4
    ```

#### Current GPG public key

The current public GPG key used for packages signing can be downloaded from
<https://packages.gitlab.com/runner/gitlab-runner/gpgkey/runner-gitlab-runner-4C80FB51394521E9.pub.gpg>.

| Key Attribute | Value                                                |
|---------------|------------------------------------------------------|
| Name          | `GitLab, Inc.`                                       |
| EMail         | `support@gitlab.com`                                 |
| Fingerprint   | `09E5 7083 F34C CA94 D541  BC58 A674 BF81 35DF A027` |
| Expiry        | `2023-06-04`                                         |

NOTE:
The same key is used by the GitLab Runner project to sign `release.sha256` files for the S3 releases
available in the <https://gitlab-runner-downloads.s3.amazonaws.com/> bucket.

#### Previous GPG public keys

Keys used in the past can be found in the table below.

For keys that were revoked it's highly recommended to remove them from package signing
verification configuration.

Signatures made by these keys should not be trusted anymore.

| Sl. No. | Key Fingerprint                                      | Status    | Expiry Date  | Download (revoked keys only)                     |
|---------|------------------------------------------------------|-----------|--------------|--------------------------------------------------|
| 1       | `3018 3AC2 C4E2 3A40 9EFB  E705 9CE4 5ABC 8807 21D4` | `revoked` | `2021-06-08` | [revoked key](gpg-keys/9CE45ABC880721D4.pub.gpg) |

## Manually download packages

You can [manually download and install the packages](linux-manually.md#using-debrpm-package) if necessary.

## Disable `skel`

> - [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1379) in GitLab Runner 12.10.
> - [Set to `true` by default](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4845) in GitLab Runner 14.0.

Sometimes the default [skeleton (`skel`) directory](https://www.thegeekdiary.com/understanding-the-etc-skel-directory-in-linux/)
causes [issues for GitLab Runner](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4449),
and it fails to run a job.

In GitLab Runner 12.10 we've added support for a special
variable - `GITLAB_RUNNER_DISABLE_SKEL` - that when set to `true` is preventing usage of `skel`
when creating the `$HOME` directory of the newly created user.

Starting with GitLab Runner 14.0 `GITLAB_RUNNER_DISABLE_SKEL` is being set to `true` by default.

If for any reason it's needed that `skel` directory will be used to populate the newly
created `$HOME` directory, the `GITLAB_RUNNER_DISABLE_SKEL` variable should be set explicitly
to `false` before package installation. For example:

For Debian/Ubuntu/Mint:

```shell
export GITLAB_RUNNER_DISABLE_SKEL=false; sudo -E apt-get install gitlab-runner
```

For RHEL/CentOS/Fedora:

```shell
export GITLAB_RUNNER_DISABLE_SKEL=false; sudo -E yum install gitlab-runner
```

NOTE:
Shell configuration added to the `$HOME` directory with the usage of `skel` may
interfere with the job execution and introduce unexpected problems like the ones mentioned above.
