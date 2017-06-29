# Running GitLab Runner behind a proxy

This guide aims specifically to making GitLab Runner with Docker executor to
work behind a proxy.

## Prerequisites

Before proceeding further, you need to make sure that:

1. You've already [installed Docker][docker] and [Gitlab Runner][install].
1. (Optional) You are using [CNTLM] as a local proxy. Note that CNTLM is only
   needed if you are behind a proxy with authentication. If you already use a
   proxy without authentication, just skip the section where CNTLM is configured.

Using a local proxy has 2 major advantages compared to adding the proxy details
everywhere manually:

- One single source where you need to change your credentials
- The credentials can not be accessed from the Docker Runners

## Configuring CNTLM

Assuming you [have installed CNTLM][howtoforge], you need to first configure
its config file.

1. Open the config file for CNTLM (`/etc/cntlm.conf`). Enter your username,
   password, domain and proxy hosts there and make sure `Gateway yes` is
   uncommented and present in the config file.

    >**Warning:**
    By default, CNTLM only listens on localhost, but that won't work if you use
    the Docker executor. The `Gateway` option turns the server into an open HTTP
    proxy so this should ideally be secured via iptables or some other firewall.

1. Save the changes and restart its service

    ```
    sudo systemctl restart cntlm
    ```

>**Note:**
CNTLM can be installed on the same machine that the Runner is installed, but you
can also setup a separate machine just for that and point everything to it.

## Configuring Docker for downloading images

>**Note:**
The following apply to OSes that have systemd support.

Follow [Docker's documentation][docker-proxy] how to use a proxy.
The service file should look like this:

```
[Service]
Environment="HTTP_PROXY=http://localhost:3128/"
Environment="HTTPS_PROXY=http://localhost:3128/"
```

## Adding Proxy variables to the Runner's config

The proxy variables need to also be added the Runner's config, so that it can
get builds assigned from GitLab behind the proxy.

This is basically the same as adding the proxy to the Docker service above:

1. Create a systemd drop-in directory for the gitlab-runner service:

    ```
    mkdir /etc/systemd/system/gitlab-runner.service.d
    ```

1. Create a file called `/etc/systemd/system/gitlab-runner.service.d/http-proxy.conf`
   that adds the `HTTP_PROXY` environment variable:

    ```
    [Service]
    Environment="HTTP_PROXY=http://localhost:3128/"
    Environment="HTTPS_PROXY=http://localhost:3128/"
    ```

1. Save the file and flush changes:

    ```
    systemctl daemon-reload
    ```

1. Restart GitLab Runner:

    ```
    sudo systemctl restart gitlab-runner
    ```

1. Verify that the configuration has been loaded:

    ```
    systemctl show --property=Environment gitlab-runner
    ```

      You should see:

      ```
      Environment=HTTP_PROXY=http://localhost:3128/ HTTPS_PROXY=http://localhost:3128/
      ```

## Adding the proxy to the Docker containers (for git clone and other stuff)

After you [registered your Runner](../register/index.md), you might want to
propagate your proxy settings to the Docker containers.

To do that, you need to edit `/etc/gitlab-runner/config.toml` and add the
following to the `[[runners]]` section:

```
pre_clone_script = "git config --global http.proxy $HTTP_PROXY; git config --global https.proxy $HTTPS_PROXY"
environment = ["HTTPS_PROXY=<your_host_ip>:3128", "HTTP_PROXY=<your_host_ip>:3128"]
```

Where `<your_host_ip>` is the public IP address of the Docker host. You need to
be able to reach it from within the Docker containers, so it's important to set
it right.

[docker]: https://docs.docker.com/engine/installation/
[install]: ../install/index.md
[cntlm]: https://github.com/Evengard/cntlm
[howtoforge]: https://www.howtoforge.com/linux-ntlm-authentication-proxy-isa-server-with-cntlm
[docker-proxy]: https://docs.docker.com/engine/admin/systemd/#httphttps-proxy
