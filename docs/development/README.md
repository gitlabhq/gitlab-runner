# Development environment

## 1. Install dependencies and Go runtime

### For Debian/Ubuntu

```shell
sudo apt-get install -y mercurial git-core wget make build-essential
wget https://storage.googleapis.com/golang/go1.13.8.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go*-*.tar.gz
```

### For macOS

Using binary package:

```shell
wget https://storage.googleapis.com/golang/go1.13.8.darwin-amd64.tar.gz
sudo tar -C /usr/local -xzf go*-*.tar.gz
```

Using installation package:

```shell
wget https://storage.googleapis.com/golang/go1.13.8.darwin-amd64.pkg
open go*-*.pkg
```

### For FreeBSD

```shell
pkg install go-1.13.8 gmake git mercurial
```

## 2. Install Docker Engine

The Docker Engine is required to create pre-built image that is embedded into runner and loaded when using Docker executor.

To install Docker, follow the Docker [installation
instructions](https://docs.docker.com/install/) for your OS.

Make sure you have a `binfmt_misc` on the machine that is running your Docker Engine.
This is required for building ARM images that are embedded into the GitLab Runner binary.

- For Debian/Ubuntu it's sufficient to execute:

  ```shell
  sudo apt-get install binfmt-support qemu-user-static
  ```

- For Docker for MacOS/Windows `binfmt_misc` is enabled by default.

- For CoreOS (but also works on Debian and Ubuntu) you need to execute the following script on system start:

  ```shell
  #!/bin/sh

  set -xe

  /sbin/modprobe binfmt_misc

  mount -t binfmt_misc binfmt_misc /proc/sys/fs/binfmt_misc

  # Support for ARM binaries through Qemu:
  { echo ':arm:M::\x7fELF\x01\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x28\x00:\xff\xff\xff\xff\xff\xff\xff\x00\xff\xff\xff\xff\xff\xff\xff\xff\xfe\xff\xff\xff:/usr/bin/qemu-arm-static:' > /proc/sys/fs/binfmt_misc/register; } 2>/dev/null
  { echo ':armeb:M::\x7fELF\x01\x02\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x28:\xff\xff\xff\xff\xff\xff\xff\x00\xff\xff\xff\xff\xff\xff\xff\xff\xff\xfe\xff\xff:/usr/bin/qemu-armeb-static:' > /proc/sys/fs/binfmt_misc/register; } 2>/dev/null
  { echo ':aarch64:M::\x7fELF\x02\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\xb7\x00:\xff\xff\xff\xff\xff\xff\xff\x00\xff\xff\xff\xff\xff\xff\xff\xff\xfe\xff\xff\xff:/usr/bin/qemu-aarch64-static:CF' > /proc/sys/fs/binfmt_misc/register; } 2>/dev/null
  ```

## 3. Download runner sources

```shell
go get gitlab.com/gitlab-org/gitlab-runner
```

## 4. Install runner dependencies

This will download and restore all dependencies required to build runner:

```shell
make deps
```

**For FreeBSD use `gmake deps`**

## 5. Run runner

Normally you would use `gitlab-runner`. In order to compile and run the Go sources, use the Go toolchain:

```shell
make runner-and-helper-bin-host
./out/binaries/gitlab-runner run
```

You can run runner in debug-mode:

```shell
make runner-and-helper-bin-host
./out/binaries/gitlab-runner --debug run
```

`make runner-and-helper-bin-host` is a superset of `make runner-bin-host` which in addition
takes care of building the Runner Helper Docker archive dependencies.

### Building the Docker images

If you want to build the Docker images, run `make runner-and-helper-docker-host`, which will:

1. Build `gitlab-runner-helper` and create a helper Docker image from it.
1. Compile Runner for `linux/amd64`.
1. Build a DEB package for Runner. The official Runner images are based on Alpine and Ubuntu,
   and the Ubuntu image build uses the DEB package.
1. Build the Alpine and Ubuntu versions of the `gitlab/gitlab-runner` image.

## 6. Run test suite locally

GitLab Runner test suite consists of "core" tests and tests for executors.
Tests for executors require certain binaries to be installed on your local
machine. Some of these binaries cannot be installed on all operating
systems. If a binary is not installed tests requiring this binary will be
skipped.

These are the binaries that you can install:

1. [VirtualBox](https://www.virtualbox.org/wiki/Downloads) and [Vagrant](https://www.vagrantup.com/downloads.html)
1. [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) with
   [Minikube](https://github.com/kubernetes/minikube)
1. [Parallels](https://www.parallels.com/products/desktop/download/)
1. [PowerShell](https://docs.microsoft.com/en-us/powershell/)

After installing the binaries run:

```shell
make development_setup
```

To execute the tests run:

```shell
make test
```

## 8. Install optional tools

- Install `golangci-lint`, used for the `make lint` target.
- Install `markdown-lint` and `vale`, used for the `make lint-docs` target.

Installation instructions will pop up when running a Makefile target
if a tool is missing.

## 9. Contribute

You can start hacking GitLab-Runner code.
If you need an IDE to edit and debug code, there are a few free suggestions you can use:

- [JetBrains GoLand IDE](https://www.jetbrains.com/go/).
- Visual Studio Code using the
  [workspace recommended extensions](https://code.visualstudio.com/docs/editor/extension-gallery#_workspace-recommended-extensions),
  located in `.vscode/extensions.json`.

## Managing build dependencies

GitLab Runner uses [Go Modules](https://github.com/golang/go/wiki/Modules) to manage
its dependencies - they get checked into the repository under the `vendor/` directory

Don't add dependency from upstream master branch when version tags are available.

## Developing for Windows on a non-windows environment

We provide a [Vagrantfile](https://gitlab.com/gitlab-org/gitlab-runner/tree/master/Vagrantfile)
to help you run a Windows Server 2019 or Windows 10 instance, since we
are using [multiple machines](https://www.vagrantup.com/docs/multi-machine) inside of Vagrant.

The following are required:

- [Vagrant](https://www.vagrantup.com) installed.
- [Virtualbox](https://www.virtualbox.com) installed.
- Around 30GB of free hard disk space on your computer.

Which virtual machine to use depends on your use case:

- The Windows Server machine has Docker pre-installed and should always
  be used when you are developing on Runner for Windows.
- The Windows 10 machine is there for you to have a windows environment
  with a GUI which sometimes can help you debugging some Windows
  features. Note that you cannot have Docker running inside of Windows
  10 because nested virtualization is not supported.

Running `vagrant up windows_10` will start the Windows 10 machine for
you. To:

- SSH inside of the Windows 10 machine, run `vagrant ssh windows_10`.
- Access the GUI for the Windows 10, you can connect via
  RDP by running `vagrant rdp windows_10`, which will connect to the
  machine using a locally installed RDP program.

For both machines, the GitLab Runner source code is synced
bi-directionally so that you can edit from your machine with your
favorite editor. The source code can be found under the `$GOROOT`
environment variable. We have a `RUNNER_SRC` environment variable which
you can use to find out the full path so when using PowerShell,
you can use `cd $Env:RUNNER_SRC`.

## Troubleshooting

### `docker.go missing Asset symbol`

This error happens due to missing `executors/docker/bindata.go` file that is generated from Docker prebuilts.
Which is especially tricky on Windows.

Try to execute: `make deps docker`, if it doesn't help you can do that in steps:

1. Execute `go get -u github.com/jteeuwen/go-bindata/...`
1. Download <https://gitlab-runner-downloads.s3.amazonaws.com/master/docker/prebuilt-x86_64.tar.xz> and save to `out/docker/prebuilt-x86_64.tar.xz`
1. Download <https://gitlab-runner-downloads.s3.amazonaws.com/master/docker/prebuilt-arm.tar.xz> and save to `out/docker/prebuilt-arm.tar.xz`
1. Execute `make docker` or check the Makefile how this command looks like
