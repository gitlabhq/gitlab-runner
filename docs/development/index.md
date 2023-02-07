---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Contribute to GitLab Runner development

GitLab Runner is a golang binary which can operate in two modes:

1. GitLab Runner executing jobs locally ("instance" executor).
1. GitLab Runner Manager delegating jobs to an autoscaled environment which uses GitLab Runner Helper to pull artifacts.

For developing GitLab Runner in instance executor mode (1) the only setup required is a working golang environment.
For developing GitLab Runner in Manager and Helper mode (2) setup also requires a Docker build environment.
Additionally running the Manager or Helper in Kubernetes will require a working cluster.

The following instructions setup your golang environment using `asdf` to manage the golang version. If you already have this or otherwise know what you're doing, you can skip step 2 ("Install dependencies and Go runtime").

In order to provide Docker and Kubernetes locally Step 3 has you setting Rancher Desktop. If you don't need one or both you can skip step 3 ("Install Rancher Desktop") or just disable `k3s` (Kubernetes) in Rancher Desktop.

## Recommended Environment

The recommended environment on which to install golang and Rancher Desktop for development is a local laptop or desktop. It is possible to use nested-virtualization to run Rancher Desktop in the cloud (which runs `k3s` in a VM) but it's more tricky to setup.

## Runner Shorts Video Tutorials

You can also follow along with the Runner Shorts (~20 minute videos) on setting up and making a change:

1. Please read the [recommended environment](#recommended-environment) section above before beginning
1. [Setting up a GitLab Runner development environment](https://www.youtube.com/watch?v=-KlaXpUdJOI)
1. [Code walkthrough of GitLab Runner](https://www.youtube.com/watch?v=pEtfmZ0Ssc4)
1. [Making and testing locally a GitLab Runner change](https://www.youtube.com/watch?v=45H4WIuu8Fc)

## 1. Clone GitLab Runner

```shell
git clone https://gitlab.com/gitlab-org/gitlab-runner.git
```

If you are developing for GitLab Runner in autoscaled mode (Manager and Helper) you might want to check out
one or more of Taskscaler, Fleeting and associated plugins. To make local changes from one package visible
to the others, use golang workspaces.

```shell
git clone https://gitlab.com/gitlab-org/fleeting/taskscaler.git
git clone https://gitlab.com/gitlab-org/fleeting/fleeting.git
git clone https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-aws.git
git clone https://gitlab.com/gitlab-org/fleeting/fleeting-plugin-googlecompute.git
go work init
go work use gitlab-runner
go work use taskscaler
go work use fleeting
go work use fleeting-plugin-aws
go work use fleeting-plugin-googlecompute
```

## 2. Install dependencies and Go runtime

The GitLab Runner project uses [`asdf`](https://asdf-vm.com/) to manage dependencies.
The simplest way to get your development environment setup is to use `asdf`:

```shell
cd gitlab-runner
asdf plugin add golang
asdf install
```

NOTE:
If you are not using `asdf`, follow the instructions below for the relevant distribution.

### For Debian/Ubuntu

```shell
sudo apt-get install -y mercurial git-core wget make build-essential
wget https://storage.googleapis.com/golang/go1.18.10.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go*-*.tar.gz
export PATH="$(go env GOBIN):$PATH"
```

### For CentOS

```shell
sudo yum install mercurial wget make
sudo yum groupinstall 'Development Tools'
wget https://storage.googleapis.com/golang/go1.18.10.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go*-*.tar.gz
export PATH="$(go env GOBIN):$PATH"
```

### For macOS

Using binary package:

```shell
wget https://storage.googleapis.com/golang/go1.18.10.darwin-amd64.tar.gz
sudo tar -C /usr/local -xzf go*-*.tar.gz
export PATH="$(go env GOBIN):$PATH"
```

Using installation package:

```shell
wget https://storage.googleapis.com/golang/go1.18.10.darwin-amd64.pkg
open go*-*.pkg
export PATH="$(go env GOBIN):$PATH"
```

### For FreeBSD

```shell
pkg install go-1.18.10 gmake git mercurial
export PATH="$(go env GOBIN):$PATH"
```

## 3. Install Rancher Desktop

The Docker Engine is required to create pre-built image that is embedded into GitLab Runner and loaded when using Docker executor. A local Kubernetes cluster is helpful for developing Kubernetes executor. Rancher Desktop provides both.

To install Rancher Desktop, follow the
[installation instructions](https://docs.rancherdesktop.io/getting-started/installation/) for your OS.

NOTE:
Be sure to configure Rancher Desktop to use `dockerd (moby)` and not `containerd`.

## 4. Install GitLab Runner dependencies

```shell
make deps
asdf reshim
```

**For FreeBSD use `gmake deps`**

## 5. Build GitLab Runner

Compile GitLab Runner using the Go toolchain:

```shell
make runner-and-helper-bin-host
```

`make runner-and-helper-bin-host` is a superset of `make runner-bin-host` which in addition
takes care of building the Runner Helper Docker archive dependencies.

## 6. Run GitLab Runner

```shell
./out/binaries/gitlab-runner run
```

You can use the any of the usual command-line arguments (including `--debug`):

```shell
./out/binaries/gitlab-runner --debug run
```

### Building the Docker images

If you want to build the Docker images, run `make runner-and-helper-docker-host`, which will:

1. Build `gitlab-runner-helper` and create a helper Docker image from it.
1. Compile GitLab Runner for `linux/amd64`.
1. Build a DEB package for Runner. The official GitLab Runner images are based on Alpine and Ubuntu,
   and the Ubuntu image build uses the DEB package.
1. Build the Alpine and Ubuntu versions of the `gitlab/gitlab-runner` image.

### New auto-scaling (Taskscaler) in GitLab Runner (since 15.6.0)

The [Next Runner Auto-scaling Architecture](https://docs.gitlab.com/ee/architecture/blueprints/runner_scaling/index.html#taskscaler-provider) adds a new mechanism for autoscaling which will work with all environments.
It will replace all current autoscaling mechanisms (e.g. Docker Machine).
This new mechanism is in a pre-alpha state and actively being developed.
There are two new libraries being used in GitLab Runner:

1. [Taskscaler](https://gitlab.com/gitlab-org/fleeting/taskscaler)
1. [Fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting)

You don't need to check out these libraries to use GitLab Runner at HEAD, but some development in the autoscaling space may take place there.
In addition Taskscaler and Fleeting, there are a number of Fleeting Plugins which adapt GitLab Runner to a specific cloud providers (e.g. Google Computer or AWS EC2).
The written instructions above ("Clone GitLab Runner") show how to check out the code and the videos ("Runner Shorts") show how to use it.
These instructions show how to use GitLab Runner with a plugin.

Each plugin will come with instructions on how to build the binary and configure the underlying instance group.
This work is being done in [this issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29400).
The canonical build and configuration instructions will live with each plugin, but in the meantime, here are some general instructions.

#### Build the plugin

Each plugin can be built with `go build -o <plugin-name> ./cmd/`.
The resulting binary should be placed somewhere on the local `$PATH`.

#### Use the plugin

GitLab Runner is started in the usual way but specifies an `instance` executor.
It also specifies under `plugin_config` and `connector_config` an Instance Group, its location, and some details about how to connect to the underlying instances.
GitLab Runner should find the Instance Group and create an initial number of idle VMs.
When a job is picked up the configured instance runner, it will consume a running VM and replace it via AWS service calls in the `fleeting-plugin-aws` plugin.

```toml
[[runners]]
  name = "local-taskrunner"
  url = "https://gitlab.com/"
  token = "REDACTED"
  executor = "instance"
  shell = "bash"
  [runners.autoscaler]
    max_use_count = 1
    max_instances = 20
    plugin = "fleeting-plugin-aws"                                 # Fleeting plugin name as built above [1].
    [runners.autoscaler.plugin_config]
      credentials_file = "/Users/josephburnett/.aws/credentials".  # Credentials which can scale an Autoscaling Group (ASG) [2].
      name = "jburnett-taskrunner-asg"                             # ASG name.
      project = "jburnett-ad8e5d54"                                # ASG project.
      region = "us-east-2"                                         # ASG region.
    [runners.autoscaler.connector_config]
      username = "ubuntu"                                          # ASG instance template username for login.
    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = 0
      scale_factor = 0.0
      scale_factor_limit = 0
```

If you terminate GitLab Runner with SIGTERM you may see some of these processes hanging around. Instead terminate with SIGQUIT.

Note that ASGs should have autoscaling disabled. GitLab Runner takes care of autoscaling via the Taskscaler library.

## 7. Run test suite locally

GitLab Runner test suite consists of "core" tests and tests for executors.
Tests for executors require certain binaries to be installed on your local
machine. Some of these binaries cannot be installed on all operating
systems. If a binary is not installed tests requiring this binary will be
skipped.

These are the binaries that you can install:

1. [VirtualBox](https://www.virtualbox.org/wiki/Downloads) and [Vagrant](https://developer.hashicorp.com/vagrant/downloads); the [Vagrant Parallels plugin](https://github.com/Parallels/vagrant-parallels) is also required
1. [kubectl](https://kubernetes.io/docs/tasks/tools/) with
   [minikube](https://github.com/kubernetes/minikube)
1. [Parallels Pro or Business edition](https://www.parallels.com/products/desktop/)
1. [PowerShell](https://learn.microsoft.com/en-us/powershell/)

After installing the binaries run:

```shell
make development_setup
```

To execute the tests run:

```shell
make test
```

### Kubernetes Integration tests

To run correctly, some Kubernetes integration tests require specific configuration or runtime
arguments of the Kubernetes cluster they run against. These tests will be skipped if the
cluster configuration is incorrect. Below is a sample configuration for Kubernetes clusters
that would commonly be used on a developer workstation:

- `minikube`

```shell
minikube delete
minikube config set container-runtime containerd
minikube config set feature-gates "ProcMountType=true"
minikube start
```

- `k3s`

```shell
k3s server --tls-san=k3s --kube-apiserver-arg=feature-gates=ProcMountType=true
```

## 8. Run tests with helper image version of choice

If you are developing functionality inside a helper, you'll most likely want to run tests with
the version of the Docker image that contains the newest changes.

If you run tests without passing `-ldflags`, the default version in `version.go` is `development`.
This means that the runner defaults to pulling a [helper image](../configuration/advanced-configuration.md#helper-image)
with the `latest` tag.

### Make targets

`make` targets inject `-ldflags` automatically. You can run all tests by using:

```shell
make simple-test
```

`make` targets also inject `-ldflags` for `parallel_test_execute`, which is most commonly used by the CI/CD jobs.

### Custom `go test` arguments

In case you want a more customized `go test` command, you can use `print_ldflags` as `make` target:

```shell
go test -ldflags "$(make print_ldflags)" -run TestDockerCommandBuildCancel -v ./executors/docker/...
```

### In GoLand

Currently, GoLand doesn't support dynamic Go tool arguments, so you'll need to run `make print_ldflags` first
and then paste it in the configuration.

NOTE:
To use the debugger, make sure to remove the last two flags (`-s -w`).

### Helper image

Build the newest version of the helper image with:

```shell
make helper-dockerarchive-host
```

Then you'll have the image ready for use:

```shell
REPOSITORY                                                    TAG                      IMAGE ID            CREATED             SIZE
gitlab/gitlab-runner-helper                                   x86_64-a6bc0800          f10d9b5bbb41        32 seconds ago      57.2MB
```

### Helper image with Kubernetes

If you are running a local Kubernetes cluster make sure to reuse the cluster's Docker daemon to build images.
For example, with minikube:

```shell
eval $(minikube docker-env)
```

## 9. Install optional tools

- Install `golangci-lint`, used for the `make lint` target.
- Install `markdown-lint` and `vale`, used for the `make lint-docs` target.

Installation instructions will pop up when running a Makefile target
if a tool is missing.

## 10. Contribute

You can start hacking `gitlab-runner` code.
If you need an IDE to edit and debug code, there are a few free suggestions you can use:

- [JetBrains GoLand IDE](https://www.jetbrains.com/go/).
- Visual Studio Code using the
  [workspace recommended extensions](https://code.visualstudio.com/docs/editor/extension-marketplace#_workspace-recommended-extensions),
  located in `.vscode/extensions.json`.

## Managing build dependencies

GitLab Runner uses [Go Modules](https://github.com/golang/go/wiki/Modules) to manage
its dependencies.

Don't add dependency from upstream default branch when version tags are available.

## Tests

The Runner codebase makes a distinction between [unit](https://en.wikipedia.org/wiki/Unit_testing)
and [integration tests](https://en.wikipedia.org/wiki/Integration_testing) in the following way:

- Unit test files have a suffix of `_test.go` and contain the following build directive in the header:

    ```golang
    // go:build !integration

    ```

- Integration test files have a suffix of `_integration_test.go` and contain the following build directive in the header:

    ```golang
    // go:build integration

    ```

  They can be run by adding `-tags=integration` to the `go test` command.

To test the state of the build directives in test files, `make check_test_directives` can be used.

## Developing for Windows on a non-windows environment

We provide a [Vagrantfile](https://gitlab.com/gitlab-org/gitlab-runner/tree/main/Vagrantfile)
to help you run a Windows Server 2019 or Windows 10 instance, since we
are using [multiple machines](https://developer.hashicorp.com/vagrant/docs/multi-machine) inside of Vagrant.

The following are required:

- [Vagrant](https://www.vagrantup.com) installed.
- [Virtualbox](https://www.virtualbox.org/) installed.
- Around 30GB of free hard disk space on your computer.

Which virtual machine to use depends on your use case:

- The Windows Server machine has Docker pre-installed and should always
  be used when you are developing on GitLab Runner for Windows.
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

## Other resources

1. [Reviewing GitLab Runner merge requests](reviewing-gitlab-runner.md)
1. [Add support for new Windows Version](add-windows-version.md)
1. [Runner Group - Team Resources](https://about.gitlab.com/handbook/engineering/development/ops/verify/runner/team-resources/#overview)
