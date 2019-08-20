# Development environment

## 1. Install dependencies and Go runtime

### For Debian/Ubuntu

```bash
apt-get install -y mercurial git-core wget make
wget https://storage.googleapis.com/golang/go1.8.7.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go*-*.tar.gz
```

### For macOS

Using binary package:

```sh
wget https://storage.googleapis.com/golang/go1.8.7.darwin-amd64.tar.gz
sudo tar -C /usr/local -xzf go*-*.tar.gz
```

Using installation package:

```sh
wget https://storage.googleapis.com/golang/go1.8.7.darwin-amd64.pkg
open go*-*.pkg
```

NOTE: **Note:**
Brew doesn't provide a [Go 1.8.x formula](https://formulae.brew.sh/formula/go).

### For FreeBSD

```
pkg install go-1.8.7 gmake git mercurial
```

## 2. Install Docker Engine

The Docker Engine is required to create pre-built image that is embedded into runner and loaded when using docker executor.

Make sure that on machine that is running your Docker Engine you have a `binfmt_misc`.
This is required to be able to build ARM images that are embedded into GitLab Runner binary.

- For Debian/Ubuntu it's sufficient to execute:

  ```
  apt-get install binfmt-support qemu-user-static
  ```

- For Docker for MacOS/Windows `binfmt_misc` is enabled by default.

- For CoreOS (but also works on Debian and Ubuntu) you need to execute the following script on system start:

  ```
  #!/bin/sh

  set -xe

  /sbin/modprobe binfmt_misc

  mount -t binfmt_misc binfmt_misc /proc/sys/fs/binfmt_misc

  # Support for ARM binaries through Qemu:
  { echo ':arm:M::\x7fELF\x01\x01\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x28\x00:\xff\xff\xff\xff\xff\xff\xff\x00\xff\xff\xff\xff\xff\xff\xff\xff\xfe\xff\xff\xff:/usr/bin/qemu-arm-static:' > /proc/sys/fs/binfmt_misc/register; } 2>/dev/null
  { echo ':armeb:M::\x7fELF\x01\x02\x01\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x02\x00\x28:\xff\xff\xff\xff\xff\xff\xff\x00\xff\xff\xff\xff\xff\xff\xff\xff\xff\xfe\xff\xff:/usr/bin/qemu-armeb-static:' > /proc/sys/fs/binfmt_misc/register; } 2>/dev/null
  ```

[Install Docker Engine](https://docs.docker.com/engine/installation/)

## 3. Configure Go

Add to `.profile` or `.bash_profile`:

```bash
export GOPATH=$HOME/Go
export PATH=$PATH:$GOPATH/bin:/usr/local/go/bin
```

Create new terminal session and create $GOPATH directory:

```
mkdir -p $GOPATH
```

## 4. Download runner sources

```
go get gitlab.com/gitlab-org/gitlab-runner
cd $GOPATH/src/gitlab.com/gitlab-org/gitlab-runner/
```

## 5. Install runner dependencies

This will download and restore all dependencies required to build runner:

```
make deps
```

**For FreeBSD use `gmake deps`**

## 6. Run runner

Normally you would use `gitlab-runner`, in order to compile and run Go source use go toolchain:

```
make install
gitlab-runner run
```

You can run runner in debug-mode:

```
make install
gitlab-runner --debug run
```

## 7. Compile and install runner binary as `gitlab-runner`

```
make install
```

## 8. Run test suite locally

GitLab Runner test suite consists of "core" tests and tests for executors.
Tests for executors require certain binaries to be installed on your local
machine. Some of these binaries cannot be installed on all operating
systems. If a binary is not installed tests requiring this binary will be
skipped.

These are the binaries that you can install:

1. [VirtualBox](https://www.virtualbox.org/wiki/Downloads) and [Vagrant](https://www.vagrantup.com/downloads.html)
1. [kubectl](https://kubernetes.io/docs/user-guide/prereqs/) with
   [Minikube](https://github.com/kubernetes/minikube)
1. [Parallels](http://www.parallels.com/products/desktop/download/)
1. [PowerShell](https://msdn.microsoft.com/en-us/powershell)

After installing the binaries run:

```
make development_setup
```

To execute the tests run:

```
make test
```

## 9. Contribute

You can start hacking GitLab-Runner code. If you are interested you can use Intellij IDEA Community Edition with [go-lang-idea-plugin](https://github.com/go-lang-plugin-org/go-lang-idea-plugin) to edit and debug code.

## Managing build dependencies

GitLab Runner uses [dep](https://github.com/golang/dep) to manage
its dependencies - they get checked into the repository under the `vendor/` directory,
with a manifest stored in `Godep.toml` and in `Godep.lock` lockfile.

If your contribution adds, removes or updates any dependencies to the runner,
please ensure the vendored copies is updated with the appropriate `dep ensure` command.

Don't add dependency from upstream master branch when version tags are available.

## Developing for Windows on a non-windows environment

We provide a [Vagrantfile](https://gitlab.com/gitlab-org/gitlab-runner/tree/master/Vagrantfile)
to help you run a Windows Server 2019 or Windows 10 instance, since we
are using [multiple machines](https://www.vagrantup.com/docs/multi-machine/) inside of Vagrant.

The following are required:

- [Vagrant](https://www.vagrant.com) installed.
- [Virtualbox](https://www.virtualbox.com) installed.
- Around 30GB of free hard disk space on your computer.

Which virtual machine to use depends on your use case:

- The Windows Server machine has docker pre-installed and should always
  be used when you are developing on Runner for Windows.
- The Windows 10 machine is there for you to have a windows environment
  with a GUI which sometimes can help you debugging some Windows
  features. Note that you cannot have Docker running inside of Windows
  10 because nested virtualization is not supported.

Running `vagrant up windows_10` will start the Windows 10 machine for
you. To:

- ssh inside of the Windows 10 machine, run `vagrant ssh windows_10`.
- Access the GUI for the Windows 10, you can connect via
  RDP by running `vagrant rdp windows_10`, which will connect to the
  machine using a locally installed RDP program.

For both machines, the GitLab Runner source code is synced
bi-directionally so that you can edit from your machine with your
favorite editor. The source code can be found under the `$GOROOT`
environment variable. We have a `RUNNER_SRC` environment variable which
you can use to find out the full path so when using:

- Windows batch scripts, you can run `cd %RUNNER_SRC`.
- PowerShell, you can use `cd $Env:RUNNER_SRC`.

## Troubleshooting

### executor_docker.go missing Asset symbol

This error happens due to missing executors/docker/bindata.go file that is generated from docker prebuilts.
Which is especially tricky on Windows.

Try to execute: `make deps docker`, if it doesn't help you can do that in steps:

1. Execute `go get -u github.com/jteeuwen/go-bindata/...`
1. Download <https://gitlab-runner-downloads.s3.amazonaws.com/master/docker/prebuilt-x86_64.tar.xz> and save to `out/docker/prebuilt-x86_64.tar.xz`
1. Download <https://gitlab-runner-downloads.s3.amazonaws.com/master/docker/prebuilt-arm.tar.xz> and save to `out/docker/prebuilt-arm.tar.xz`
1. Execute `make docker` or check the Makefile how this command looks like
