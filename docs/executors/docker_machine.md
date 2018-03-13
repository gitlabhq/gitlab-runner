# Install and register GitLab Runner for autoscaling with Docker Machine

> The auto scale feature was introduced in GitLab Runner 1.1.0.

For an overview of the autoscale architecture, take a look at the
[comprehensive documentation on autoscaling](../configuration/autoscale.md).

## Preparing the environment

In order to use the autoscale feature, Docker and GitLab Runner must be
installed in the same machine:

1. Log in to a new Linux-based machine that will serve as a bastion server
   where Docker will spawn new machines from
1. [Install GitLab Runner](../install/index.md)
1. [Install Docker Machine](https://docs.docker.com/machine/install-machine/)
1. Optionally but recommended, prepare a
   [proxy container registry and a cache server](../install/registry_and_cache_servers.md)
   to be used with the autoscaled Runners

If you need to use any virtualization/cloud providers that aren't handled by
Docker's Machine internal drivers, the appropriate driver plugin must be
installed. The Docker Machine driver plugin installation and configuration is
out of the scope of this documentation. For more details please read the
[Docker Machine documentation](https://docs.docker.com/machine/)

## Configuring GitLab Runner

1. [Register a GitLab Runner](../register/index.md#gnu-linux) and select the
   `docker+machine` executor when asked.
1. Edit [`config.toml`](../commands/README.md#configuration-file) and configure
   the Runner to use Docker machine. Visit the dedicated page covering detailed
   information about [GitLab Runner Autoscaling](../configuration/autoscale.md).
1. Try to build your project. In a few seconds, if you run `docker-machine ls`
   you should see a new machine being created.

## Upgrading the Runner

1. Ensure your operating system isn't configured to automatically restart the
   Runner if it exits (which is the default configuration on some platforms).

1. Stop the Runner:

    ```bash
    killall -SIGQUIT gitlab-runner
    ```

    Sending the [`SIGQUIT` signal](../commands/README.md#signals) will make the
    Runner to stop gracefully. It will stop accepting new jobs, and will exit
    as soon as the current jobs are finished.

1. Wait until the Runner exits. You can check its status with `gitlab-runner status`
    or await a graceful shutdown for up to 30 minutes with:

    ```bash
    for i in `seq 1 180`; do # 1800 seconds = 30 minutes
        gitlab-runner status || break
        sleep 10
    done
    ```

1. You can now safely upgrade the Runner without interrupting any jobs

## Managing the Docker Machines

1. Ensure your operating system isn't configured to automatically restart the
   Runner if it exits (which is the default configuration on some platforms).

1. Stop the Runner:

    ```bash
    killall -SIGQUIT gitlab-runner
    ```

1. Wait until the Runner exits. You can check its status with: `gitlab-runner status`
    or await a graceful shutdown for up to 30 minutes with:

    ```bash
    for i in `seq 1 180`; do # 1800 seconds = 30 minutes
        gitlab-runner status || break
        sleep 10
    done
    ```

1. You can now manage (upgrade or remove) any Docker Machines with the
   [`docker-machine` command](https://docs.docker.com/machine/reference/)
