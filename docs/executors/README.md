# Executors

GitLab Runner implements a number of executors that can be used to run your
builds in different scenarios. If you are not sure what to select, read the
[I am not sure](#i-am-not-sure) section.
Visit the [compatibility chart](#compatibility-chart) to find
out what features each executor supports and what not.

To jump into the specific documentation of each executor, visit:

- [SSH](ssh.md)
- [Shell](shell.md)
- [Parallels](parallels.md)
- [VirtualBox](virtualbox.md)
- [Docker](docker.md)
- [Docker Machine (auto-scaling)](docker_machine.md)
- [Kubernetes](kubernetes.md)

## Selecting the executor

The executors support different platforms and methodologies for building a
project. The table below shows the key facts for each executor which will help
you decide.

| Executor                                          | SSH  | Shell   | VirtualBox | Parallels | Docker | Kubernetes |
|:--------------------------------------------------|:----:|:-------:|:----------:|:---------:|:------:|:----------:|
| Clean build environment for every build           | ✗    | ✗       | ✓          | ✓         | ✓      | ✓          |
| Migrate runner machine                            | ✗    | ✗       | partial    | partial   | ✓      | ✓          |
| Zero-configuration support for concurrent builds  | ✗    | ✗ (1)   | ✓          | ✓         | ✓      | ✓          |
| Complicated build environments                    | ✗    | ✗ (2)   | ✓ (3)      | ✓ (3)     | ✓      | ✓          |
| Debugging build problems                          | easy | easy    | hard       | hard      | medium | medium     |

1. It's possible, but in most cases it is problematic if the build uses services
   installed on the build machine
2. It requires to install all dependencies by hand
3. For example using [Vagrant](https://www.vagrantup.com/docs/virtualbox/ "Vagrant documentation for VirtualBox")

### I am not sure

#### SSH Executor

The **SSH** executor is added for completeness. It's the least supported
among all executors. It makes GitLab Runner to connect to some external server
and run the builds there. We have some success stories from organizations using
that executor, but generally we advise to use any other.

#### Shell Executor

**Shell** is the simplest executor to configure. All required dependencies for
your builds need to be installed manually on the machine that the Runner is
installed.

#### Virtual Machine Executor (VirtualBox / Parallels)

We also offer two full system virtualization options: **VirtualBox** and
**Parallels**. This type of executor allows you to use an already created
virtual machine, which will be cloned and used to run your build. It can prove
useful if you want to run your builds on different Operating Systems since it
allows to create virtual machines with Windows, Linux, OSX or FreeBSD and make
GitLab Runner to connect to the virtual machine and run the build on it. Its
usage can also be useful to reduce the cost of infrastructure.

#### Docker Executor

A better way is to use **Docker** as it allows to have a clean build environment,
with easy dependency management (all dependencies for building the project could
be put in the Docker image). The Docker executor allows you to easily create
a build environment with dependent [services], like MySQL.

##### Docker Machine

The **Docker Machine** is a special version of the **Docker** executor
with support for auto-scaling. It works like the normal **Docker** executor
but with build hosts created on demand by _Docker Machine_.

#### Kubernetes Executor

The **Kubernetes**  executor allows you to use an existing Kubernetes cluster
for your builds. The executor will call the Kubernetes cluster API
and create a new Pod (with build container and services containers) for
each GitLab CI job.

## Compatibility chart

Supported features by different executors:

| Executor                                     | SSH  | Shell   | VirtualBox | Parallels | Docker | Kubernetes |
|:---------------------------------------------|:----:|:-------:|:----------:|:---------:|:------:|:----------:|
| Secure Variables                             | ✓    | ✓       | ✓          | ✓         | ✓      | ✓          |
| GitLab Runner Exec command                   | ✗    | ✓       | ✗          | ✗         | ✓      | ✓          |
| gitlab-ci.yml: image                         | ✗    | ✗       | ✗          | ✗         | ✓      | ✓          |
| gitlab-ci.yml: services                      | ✗    | ✗       | ✗          | ✗         | ✓      | ✓          |
| gitlab-ci.yml: cache                         | ✓    | ✓       | ✓          | ✓         | ✓      | ✓          |
| gitlab-ci.yml: artifacts                     | ✓    | ✓       | ✓          | ✓         | ✓      | ✓          |
| Absolute paths: caching, artifacts           | ✗    | ✗       | ✗          | ✗         | ✗      | ✓          |
| Passing artifacts between stages             | ✓    | ✓       | ✓          | ✓         | ✓      | ✓          |
| Use GitLab Container Registry private images | n/a  | n/a     | n/a        | n/a       | ✓      | ✓          |
| Interactive Web terminal                     | ✗    | ✓ (bash)| ✗          | ✗         | ✗      | ✓          |

Supported systems by different shells:

| Shells  | Bash        | Windows Batch | PowerShell |
|:-------:|:-----------:|:-------------:|:----------:|
| Windows | ✓           | ✓ (default)   | ✓          |
| Linux   | ✓ (default) | ✗             | ✗          |
| OSX     | ✓ (default) | ✗             | ✗          |
| FreeBSD | ✓ (default) | ✗             | ✗          |

Supported systems for interactive web terminals by different shells:

| Shells  | Bash        | Windows Batch | PowerShell |
|:-------:|:-----------:|:-------------:|:----------:|
| Windows | ✗           | ✗             | ✗          |
| Linux   | ✓           | ✗             | ✗          |
| OSX     | ✓           | ✗             | ✗          |
| FreeBSD | ✓           | ✗             | ✗          |

[services]: https://docs.gitlab.com/ce/ci/services/README.html
