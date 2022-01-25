# Internal Executor Interface

NOTE:
As this is a documentation of the code internals, it's easier to get it outdated than
documentation of configuration, behaviors or features that we expose to the users. This
page is accurate as for the date of creation: **2022-01-26**.

## Interfaces

GitLab Runner uses a concept of what we name `executors` to define a way of how a job may be
executed.

While the current philosophy behind GitLab CI/CD job execution is that _everything is a
shell script_, this script may be executed in a different ways, for example:

- in a shell directly on a host where GitLab Runner is working,
- in a shell on an external host available through SSH,
- in a shell in a virtual machine managed by VirtualBox or Parallels,
- in a shell in a container managed by Docker,

and few others. There is also the _Custom Executor_, which allows the user to interact with
a very simple externally exposed interface to implement their own way of job execution.

All of these _executors_ are orchestrated internally by GitLab Runner process. And for that
Runner is using a set of Go interfaces that need to be implemented by the executor
to work.

Te two main interfaces (part of the `common` package) that manage an executor's lifetime
and job execution are:

- `Executor`
- `ExecutorProvider`

```golang
type Executor interface {
    // Shell returns data about the shell and scripts this executor is bound to.
    Shell() *ShellScriptInfo
    // Prepare prepares the environment for build execution. e.g. connects to SSH, creates containers.
    Prepare(options ExecutorPrepareOptions) error
    // Run executes a command on the prepared environment.
    Run(cmd ExecutorCommand) error
    // Finish marks the build execution as finished.
    Finish(err error)
    // Cleanup cleans any resources left by build execution.
    Cleanup()
    // GetCurrentStage returns current stage of build execution.
    GetCurrentStage() ExecutorStage
    // SetCurrentStage sets the current stage of build execution.
    SetCurrentStage(stage ExecutorStage)
}

type ExecutorProvider interface {
    // CanCreate returns whether the executor provider has the necessary data to create an executor.
    CanCreate() bool
    // Create creates a new executor. No resource allocation happens.
    Create() Executor
    // Acquire acquires the necessary resources for the executor to run, e.g. finds a virtual machine.
    Acquire(config *RunnerConfig) (ExecutorData, error)
    // Release releases any resources locked by Acquire.
    Release(config *RunnerConfig, data ExecutorData)
    // GetFeatures returns metadata about the features the executor supports, e.g. variables, services, shell.
    GetFeatures(features *FeaturesInfo) error
    // GetConfigInfo extracts metadata about the config the executor is using, e.g. GPUs.
    GetConfigInfo(input *RunnerConfig, output *ConfigInfo)
    // GetDefaultShell returns the name of the default shell for the executor.
    GetDefaultShell() string
}
```

All the existing executors are also extending the `executors.AbstractExecutor` struct (named
`AbstractExecutor` further in this document), which implements a small, common set of features.
While there is no protection in code that would ensure usage of `AbstractExecutor` (until the
new code implements the interfaces - it will work), it's expected that the new executors will
extend it - to ensure consistent behavior of some features across executors.

For convenience there is also the `executors.DefaultExecutorProvider` that implements the
`ExecutorProvider` interface and is suitable for most cases. However, each executor may
decide to implement its _provider_ independently (which in fact is currently done only by the
Docker Machine executor).

What's important, because both `Executor` and `ExecutorProvider` are interfaces, the implementation
allows to "stack" different structs. The usage of this possibility will be shown with one of the
examples.s

### `Executor` interface

The `Executor` interface is responsible for the job execution management.

The described methods are managing preparation of the job environment (`Prepare()`), job script
executions (`Run()` and `Finish()`; note: subsequent job steps are executed with a separate `Run()` calls)
and job environment cleanup (`Finish()`).

It also provides integration for internal Prometheus metrics exporter to label some relevant metrics
with information about the current executor usage stage (`GetCurrentStage()`, `SetCurrentStage()`).

The `Shell()` method is currently used in one place, and it's fully implemented in the mentioned
`AbstractExecutor` struct. Given the existing implementation and evolution of different executors
over time, it seems that this method should be pulled off the executor interface and handled in
some different way. Hopefully - in a way that will enforce usage of `AbstractExecutor`.

Usage of the interface, in very simplification, goes as follows:

1. The instance of an _executor_ was provided and assigned to a received job,
1. `Shell()` is called to get the configuration of a shell. It's used to prepare all the scripts
   that will be executed for the job.
1. `Prepare()` is called to prepare the job environment (for example creating a Kubernetes Pod,
   a set of Docker containers or a VirtualBox VM). It's also a place for the specific executor
   implementation to handle its own preparation. Through the usage of `AbstractExecutor`
   all the executors will also get access to some common features like for example job trace object.
1. `Run()` is called several times, each time containing details about the script for a job execution
   step to be executed with the executor.
1. `Finish()` is called after execution of all job stages is done and when job is being marked as
   finished. Some executors may take a usage of this moment. Most of them defers to
   `AbstractExecutor`.
1. `Cleanup()` is called to cleanup the job environment. It's the opposite of `Prepare()`.

Additionally `SetCurrentStage()` is called internally by the executors (however most of them
defer to `AbstractExecutor`) to mark on what _executor usage stage_ the system
is now within this executor instance. And `GetCurrentStage()` is called externally in random
moments by the metrics collector. The value is then used to summarize information about
different jobs and label some of the metrics.

### `ExecutorProvider` interface

The `ExecutorProvider` interface is responsible for preparation of the executor itself. It builds
an abstraction around the `Executor` concept. With this abstraction, what the user configures with
the `config.toml` `executor` setting is in fact the executor provider. And then for every job executed
by the runner a new, independent instance of the _executor_ is prepared. The maintenance of the _executor_
is done by the `ExecutorProvider`.

The described methods are managing creation of the executor instance (`CanCreate()`, `Create()`),
reservation of provider's resources for a potential job (`Acquire()`, `Release()`). There is also
support for gathering some information that should be reported to GitLab when requesting jobs
(`GetFeatures()`, `GetConfigInfo()`). And finally a method that gives information about the shell
that should be used with the provided executor (`GetDefaultShell()`).

Usage of the interface, in very simplification, goes as follows:

1. `CanCreate()`, `GetFeatures()` and `GetDefaultShell()` are executed at the provider registration
   to validate that the provider is able to work in general.
1. Before requesting a new job for the specific `[[runners]]` worker the `Acquire()` is called
   to check and do a reservation of provider resources for the job. This is a place where the provider
   may control its capacity and return information about some preallocated resources.
1. `GetFeatures()` is called several times to ensure that information about features supported
   by Runner can be sent back with different API requests to GitLab. On of the calls is made when
   preparing the initial request for a job.
1. Same goes for the `GetConfigInfo()` which is called only once, when preparing the initial request
   for a job. It allows to sent some information about used configuration to GitLab.
1. Same goes for the `GetDefaultShall()` which is called also onl once, when preparing the initial
   request for a job. It allows to sent information about used shell to GitLab.
1. If the job was received, it's preparation is started and at some moment `Create()` is called
   to create a new instance of the executor.
1. When the job execution is fully done, `Release()` is called. This is a place where the provider
   may handle releasing resources that were previously reserved for the job.

List of features that can be reported to GitLab can be found in the `FeaturesInfo` struct in
`common/network.go`.

## `DefaultExecutorProvider`

As `DefaultExecutorProvider` is currently one of two existing implementations of `ExecutorProvider`
interface and is used by most of the executors, let's describe how it's built.

```golang
type DefaultExecutorProvider struct {
    Creator          func() common.Executor
    FeaturesUpdater  func(features *common.FeaturesInfo)
    ConfigUpdater    func(input *common.RunnerConfig, output *common.ConfigInfo)
    DefaultShellName string
}
```

The `Creator` is the most important part. It's a function that returns a new instance
of the given `Executor` interface implementation. It is being implemented by each
of the executors. It's required to be implemented.

The interface's `CanCreate()` method will fail if the `Creator` is left empty. Call to
provider's `Create()` is proxied to the `Creator` function.

`FeaturesUpdater` and `ConfigUpdater` are functions that allow to request the feature
and config information. All executors are using these functions to expose information
about supported features or config details. The `FeaturesUpdater` is optional and every
executor have to report which features from the list are supported. `ConfiguUpdater` is
optional and can be skipped. `DefaultShellName` must be set by every executor.

Provider's `GetFeatures()`, `GetConfigInfo()`, `GetDefaultShell()` calls will use the
defined updaters and the shell name to expose needed data to the caller.

`Acquire()` and `Release()` are a NOOP. `DefaultExecutorProvider` doesn't use the
concept of resources management and simply creates a new instance of the executor
for every call.

## Usage examples

### Shell

_Shell executor_ is the simplest executor that GitLab Runner provides. It executes
the job script in a simple shell process, created directly on the host where GitLab
Runner is running itself. There is no virtualization, no containers, no network
communication here.

#### ExecutorProvider

Shell executor uses the `DefaultExecutorProvider`. It reports usage of very limited number
of features (two in all cases, two more if the platform is not `windows`). It doesn't
expose any configuration details.

The shell depends on what's the default value for the platform where the Runner is operating.
It's configured as a login shell.

#### Executor

`Prepare()` doesn't have anything specific. As the shell executor executes everything directly
in the system where Runner process exists, it just makes sure that the builds and cache
paths are usable. After that it defers to `AbstractExecutor` steps of preparation.

`Run()` uses the provided script details to construct `os/exec.Cmd` call. Shell executor
ensures that STDIN/STDOUT/STDERR are passed properly between the script execution shell
process started by that call and the job trace object. It also detects the exit code
of the command and reports it back as expected by the interface.

There is no custom implementation of `Finish()` nor `Cleanup()`. The executor defers to the common
steps in `AbstractExecutor.`

### Docker

_Docker executor_ is probably the most powerful and mature of GitLab Runner executors. It supports
most of the features available in `.gitlab-ci.yml`. It allows to run every job in an environment
separated from other jobs. All jobs are however executed on one host and the capacity of the runner
is limited by that host's available resources.

Docker executor comes with a special variant - the SSH one. To make this documentation easier
to understand (as the executor descriptions are just examples to help understand how the
executor interface works) we will describe just the "normal" variant of Docker executor.

There is also the `windows` variant of the executor. We will not include its details in this
description as well.

In Docker executor the jobs are executed in Docker containers. Each job gets a set of connected
containers sharing at least one volume with the working directory. The main container is created
from the image specified by the user. It needs to expose a shell where Runner will execute the
script. Additionally, Runner will create what we call `predefined` container from the `helper`
image provided by Runner. This container will be used to execute scripts handling common tasks
like updating the Git and Git LFS sources, operating with cache and operating with artifacts.

Depending on the job configuration Runner may create more containers for the defined `services`.
These will be linked by the networking to the main container, so that the job script can utilise
network available services exposed by them.

#### ExecutorProvider

Docker executor also uses the `DefaultExecutorProvider`. It reports usage of few more executor-related
features, and additionally it reports some configuration details.

The shell is hardcoded and differs between the platforms. In case of the most popular `linux` variant
of Docker executor, it's configured as a non-login shell.

#### Executor

`Prepare()` is highly utilised in this executor. During that step Runner will prepare different
internal tools (like volumes manager or network manager) and set up the basic configuration that
will be next used by the containers for job execution. It's also the step when all the images
defined for the job are pulled. Creation of volumes, device binding and service containers
also happens during that step.

After `Prepare()` is done the environment should be fully ready to start creating predefined/job step
execution containers, connecting them to the whole stack and execute scripts in them.

`Run()` creates predefined or job step containers, attaches to them and executes the script
in a shell that should be running as the main process of the container. It proxies the STDOUT and
STDERR of the containers to the job trace object. It also uses the Docker Engine API to detect
the script execution exit code.

`Finish()` doesn't have any custom behavior here, and it just defers to the `AbstractExecutor`.

`Cleanup()` is the opposite of `Prepare()`, so it removes all the defined resources like containers,
volumes (that were not configured as persistent), job specific network (if used).

### Docker Machine (autoscaling capabilities)

_Docker Machine executor_ is in fact an autoscaling provider built on top of the regular _Docker executor_.

It takes advantage of the interface concept and encapsulates the Docker executor in itself. Responsibility
of Docker Machine executor is mostly focused on the `ExecutorProvider` interface. With that it manages
a pool of VMs with Docker Engine running on them. Management is done by using the _Docker Machine_ tool
by running an `os/exec.Cmd` calls to it.

Management of the VMs may be done in "on-demand" or "autoscaled in background" modes. Chosen mode
depends on configuration provided by the user. In the first mode the VMs will be created for each
received job, until the limit of jobs is reached. In the second mode it will maintain a configurable
set of `Idle` s that await for jobs. Jobs are then requested only when there is at least one
`Idle` VM. When one `Idle` VM is taken for a received job, another is created to replace it. When the
VM is returned to the pool (if configured to do so) and the number of `Idle` exceeds the defined limit,
the provider starts to remove some of them. This loop that tries to maintain the desired number of `Idle`
VMs and desired total number of managed VMs works all the time in the background.

Docker Machine executor is currently implemented in a way that it allows execution of only one job at once
on a single VM.

For the execution of jobs Docker Machine executor uses the Docker executor and fact, that one can
configure access credentials of the Docker Engine API. With that the Docker Machine provider
manages the VMs, chooses a VM for a job and instantiates the Docker executor, automatically configuring
it to use the credentials and API endpoint of the VM. With that jobs are executed like with the normal
Docker executor (supporting all the different features available for it in `.gitlab-ci.yml` syntax), but
doest that on an external host, independent for each job.

#### ExecutorProvider

Docker Machine executor brings its own implementation of the `ExecutorProvider` interface!

However, as it internally uses the Docker executor, it also instantiates the Docker executor
provider (which itself is the specific configuration of `DefaultExecutorProvider`) and either
proxies some calls to it directly or calls it internally for its own purpose.

`CanCreate()` is proxied directly to Docker executor. Same goes for `GetFeatures()`, `GetConfigInfo()`
and `GetDefaultShell()`.

`Create()` is very simple as its returns the `machineExecutor` (implementation of the `Executor` interface)
with access to itself, so that steps like `Prepare()` or `Cleanup()` can use it to maintain the autoscaled
VMs (more about that will be described bellow).

This provider is also the one that finally takes the usage of `Acquire()` and `Release()` methods
of the `Executor Provider` interface.

Behavior of `Acquire()` depends on the configured mode.

In the "on-demand" mode it's used as a place to kick one of the old machines cleanup calls. It
doesn't do any real acquiring and even logs that with `IdleCount is set to 0 so the machine will
be created on demand in job context` (this is not user facing and available in the Runner process logs).
With that the provider will try to create the VM in context of the job. If there is anything that will
cause a failure: exceeding the defined limits in autoscaling configuration, wrong autoscaling configuration,
cloud provider errors, Docker Engine availability problems - it will cause a failure of the job.

In the "autoscaled in background" mode, it will check if there is any `Idle` VM that is available.
If it is, it will reserve it and allow Runner to send a request for new job. If job is received,
it will get information about the acquired VM. If there is no available `Idle` VMs, then the call to
`Acuire()` will wail, which will prevent Runner from sending a request for a job (and in Runner logs
will be logged as the `no free machines that can process builds` warning).

`Release()` behaves in the same in both modes. It will check if the VM that was used for the job
is applicable for removal and will trigger a remove in that case. In other cases, it will signal the
internal autoscaling coordination mechanism that the VM was released and it's back in the `Idle` pool,
so that it can be used again.

#### Executor

The `Executor` interface implementation is also a mix of a code specific for Docker Machine executor
and encapsulation of Docker executor. Docker Machine executor injects all the work needed
to maintain, chose and use the VMs and to configure the dedicated Docker executor instance, and
then it depends on this executor to handle the rest.

`Shell()` call defers to Docker executor, which itself defers to `AbstractExecutor` (as all the executors do).

`Prepare()` prepares the VM to use. Depending on the configured mode it may mean using the preallocated
VM or creating it on-demand. In the "on-demand" mode this is the place where eventual failure caused
by VM creation may fail the job. Having the VM details it updates the configuration of Docker executor
by pointing the host and credentials to access Docker Engine and instantiates the Docker executor provider.
Finally, it calls Docker Executor's `Prepare()` to handle all the job environment preparation as
it was described in the previous example.

`Run()` and `Finish()` have no specific behavior. They simply proxy the call to the internal Docker executor.

`GetCurrentStage()` and `SetCurrentStage()` are also proxies to the Docker executor, which itself defers
to the `AbstractExecutor` implementation.

Finally, the `Cleanup()` call does two things. First, it internally calls Docker executor's `Cleanup()` method
to clean the job environment on the VM as it was described in the previous example. Then it calls
providers `Release()` to signal that the job is done and that the VM can be released.
