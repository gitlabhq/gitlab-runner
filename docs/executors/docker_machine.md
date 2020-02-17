# Install and register GitLab Runner for autoscaling with Docker Machine

> The auto scale feature was introduced in GitLab Runner 1.1.0.

For an overview of the autoscale architecture, take a look at the
[comprehensive documentation on autoscaling](../configuration/autoscale.md).

## Forked version of Docker machine

Because `docker-machine` is in [maintenance
mode](https://github.com/docker/machine/issues/4537), GitLab is
providing it's [own fork of
`docker-machine`](https://gitlab.com/gitlab-org/ci-cd/docker-machine),
which is based on the latest `master` branch of `docker-machine` with
some additional patches for the following bugs:

- [Make DigitalOcean driver RateLimit aware](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/2)
- [Add backoff to Google driver operations check](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/7)
- [Add `--google-min-cpu-platform` option for machine creation](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/9)
- [Use cached IP for Google driver](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/15)
- [Use cached IP for AWS driver](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/14)

The intent of this fork is to fix critical and bugs affecting running
costs only. No new features will be added.

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

1. [Register a GitLab Runner](../register/index.md#gnulinux) and select the
   `docker+machine` executor when asked.
1. Edit [`config.toml`](../commands/README.md#configuration-file) and configure
   the Runner to use Docker machine. Visit the dedicated page covering detailed
   information about [GitLab Runner Autoscaling](../configuration/autoscale.md).
1. The **first time** you're using Docker Machine, it's best to execute manually
   `docker-machine create ...` with your chosen driver and all options from the
   `MachineOptions` section. This will set up the Docker Machine environment
   properly and will also be a good validation of the specified options.
   After this, you can destroy the machine with `docker-machine rm [machine_name]`
   and start the Runner.

   NOTE: **Note:**
   Multiple concurrent requests to `docker-machine create` that are done
   **at first usage** are not good. When the `docker+machine` executor is used,
   the Runner may spin up few concurrent `docker-machine create` commands. If
   Docker Machine was not used before in this environment, each started process
   tries to prepare SSH keys and SSL certificates (for Docker API authentication
   between Runner and Docker Engine on the autoscaled spawned machine), and these
   concurrent processes are disturbing each other. This can end with a non-working
   environment. That's why it's important to create a test machine manually the
   very first time you set up the Runner with Docker Machine.

1. Now, you can try and start a new pipeline in your project. In a few seconds,
   if you run `docker-machine ls` you should see a new machine being created.

## Upgrading the Runner

1. Check if your operating system is configured to automatically restart the
   Runner (for example, by checking its service file):
   - **if yes**, ensure that service manager is [configured to use `SIGQUIT`](../configuration/init.md)
     and use the service's tools to stop the process:

     ```shell
     # For systemd
     sudo systemctl stop gitlab-runner

     # For upstart
     sudo service gitlab-runner stop
     ```

   - **if no**, you may stop the Runner's process manually:

     ```shell
     sudo killall -SIGQUIT gitlab-runner
     ```

   NOTE: **Note:**
   Sending the [`SIGQUIT` signal](../commands/README.md#signals) will make the
   Runner to stop gracefully. It will stop accepting new jobs, and will exit
   as soon as the current jobs are finished.

1. Wait until the Runner exits. You can check its status with `gitlab-runner status`
   or await a graceful shutdown for up to 30 minutes with:

   ```shell
   for i in `seq 1 180`; do # 1800 seconds = 30 minutes
       gitlab-runner status || break
       sleep 10
   done
   ```

1. You can now safely install the new Runner without interrupting any jobs
