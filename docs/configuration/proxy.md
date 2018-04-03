# Running GitLab Runner behind a proxy

This guide aims specifically to making GitLab Runner with Docker executor to
work behind a proxy.

Before proceeding further, you need to make sure that you've already
[installed Docker](https://docs.docker.com/install/) and
[GitLab Runner](../install/index.md) on the same machine.

## Configuring CNTLM

NOTE: **Note:**
If you already use a proxy without authentication, this section is optional and
you can skip straight to [configuring Docker](#configuring-docker-for-downloading-images).
Configuring CNTLM is only needed if you are behind a proxy with authentication,
but it's recommended to use in any case.

[CNTLM](https://github.com/Evengard/cntlm) is a Linux proxy which can be used
as a local proxy and has 2 major advantages compared to adding the proxy details
everywhere manually:

- One single source where you need to change your credentials
- The credentials can not be accessed from the Docker Runners

Assuming you [have installed CNTLM](https://www.howtoforge.com/linux-ntlm-authentication-proxy-isa-server-with-cntlm),
you need to first configure it.

### Make CNTLM listen to the `docker0` interface

For extra security, and to protect your server from the outside world, you can
bind CNTLM to listen on the `docker0` interface which has an IP that is reachable
from inside the containers. If you tell CNTLM on the Docker host to bind only
to this address, Docker containers will be able to reach it, but the outside
world won't be able to.

1. Find the IP that Docker is using:

    ```sh
    ip -4 -oneline addr show dev docker0
    ```

    This is usually `172.17.0.1`, let's call it `docker_ip`.

1. Open the config file for CNTLM (`/etc/cntlm.conf`). Enter your username,
   password, domain and proxy hosts, and configure the `Listen` IP address
   which you found from the previous step. It should look like this:

    ```
    Username     testuser
    Domain       corp-uk
    Password     password
    Proxy        10.0.0.41:8080
    Proxy        10.0.0.42:8080
    Listen       172.17.0.1:3128
    ```

1. Save the changes and restart its service:

    ```sh
    sudo systemctl restart cntlm
    ```

## Configuring Docker for downloading images

NOTE: **Note:**
The following apply to OSes that have systemd support.

Follow [Docker's documentation](https://docs.docker.com/config/daemon/systemd/#httphttps-proxy)
how to use a proxy.

The service file should look like this:

```ini
[Service]
Environment="HTTP_PROXY=http://docker_ip:3128/"
Environment="HTTPS_PROXY=http://docker_ip:3128/"
```

## Adding Proxy variables to the Runner config

The proxy variables need to also be added the Runner's config, so that it can
get builds assigned from GitLab behind the proxy.

This is basically the same as adding the proxy to the Docker service above:

1. Create a systemd drop-in directory for the `gitlab-runner` service:

    ```sh
    mkdir /etc/systemd/system/gitlab-runner.service.d
    ```

1. Create a file called `/etc/systemd/system/gitlab-runner.service.d/http-proxy.conf`
   that adds the `HTTP_PROXY` environment variable(s):

    ```ini
    [Service]
    Environment="HTTP_PROXY=http://docker_ip:3128/"
    Environment="HTTPS_PROXY=http://docker_ip:3128/"
    ```

1. Save the file and flush changes:

    ```sh
    systemctl daemon-reload
    ```

1. Restart GitLab Runner:

    ```sh
    sudo systemctl restart gitlab-runner
    ```

1. Verify that the configuration has been loaded:

    ```sh
    systemctl show --property=Environment gitlab-runner
    ```

      You should see:

      ```ini
      Environment=HTTP_PROXY=http://docker_ip:3128/ HTTPS_PROXY=http://docker_ip:3128/
      ```

## Adding the proxy to the Docker containers

After you [registered your Runner](../register/index.md), you might want to
propagate your proxy settings to the Docker containers (for git clone and other
stuff).

To do that, you need to edit `/etc/gitlab-runner/config.toml` and add the
following to the `[[runners]]` section:

```toml
pre_clone_script = "git config --global http.proxy $HTTP_PROXY; git config --global https.proxy $HTTPS_PROXY"
environment = ["HTTPS_PROXY=docker_ip:3128", "HTTP_PROXY=docker_ip:3128"]
```

Where `docker_ip` is the IP address of the `docker0` interface. You need to
be able to reach it from within the Docker containers, so it's important to set
it right.
