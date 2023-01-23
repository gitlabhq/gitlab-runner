---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Running GitLab Runner behind a proxy **(FREE)**

This guide aims specifically to making GitLab Runner with Docker executor work behind a proxy.

Before continuing, ensure that you've already
[installed Docker](https://docs.docker.com/get-docker/) and
[GitLab Runner](../install/index.md) on the same machine.

## Configuring CNTLM

NOTE:
If you already use a proxy without authentication, this section is optional and
you can skip straight to [configuring Docker](#configuring-docker-for-downloading-images).
Configuring CNTLM is only needed if you are behind a proxy with authentication,
but it's recommended to use in any case.

[CNTLM](https://github.com/Evengard/cntlm) is a Linux proxy which can be used
as a local proxy and has 2 major advantages compared to adding the proxy details
everywhere manually:

- One single source where you need to change your credentials
- The credentials can not be accessed from the Docker runners

Assuming you [have installed CNTLM](https://www.howtoforge.com/linux-ntlm-authentication-proxy-isa-server-with-cntlm),
you need to first configure it.

### Make CNTLM listen to the `docker0` interface

For extra security, and to protect your server from the outside world, you can
bind CNTLM to listen on the `docker0` interface which has an IP that is reachable
from inside the containers. If you tell CNTLM on the Docker host to bind only
to this address, Docker containers are be able to reach it, but the outside
world can't.

1. Find the IP that Docker is using:

   ```shell
   ip -4 -oneline addr show dev docker0
   ```

   This is usually `172.17.0.1`, let's call it `docker0_interface_ip`.

1. Open the configuration file for CNTLM (`/etc/cntlm.conf`). Enter your username,
   password, domain and proxy hosts, and configure the `Listen` IP address
   which you found from the previous step. It should look like this:

   ```plaintext
   Username     testuser
   Domain       corp-uk
   Password     password
   Proxy        10.0.0.41:8080
   Proxy        10.0.0.42:8080
   Listen       172.17.0.1:3128 # Change to your docker0 interface IP
   ```

1. Save the changes and restart its service:

   ```shell
   sudo systemctl restart cntlm
   ```

## Configuring Docker for downloading images

NOTE:
The following apply to OSes with systemd support.

Follow [Docker's documentation](https://docs.docker.com/config/daemon/systemd/#httphttps-proxy)
how to use a proxy.

The service file should look like this:

```ini
[Service]
Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
```

## Adding Proxy variables to the GitLab Runner configuration

The proxy variables need to also be added to the GitLab Runner configuration, so that it can
get builds assigned from GitLab behind the proxy.

This is basically the same as adding the proxy to the Docker service above:

1. Create a systemd drop-in directory for the `gitlab-runner` service:

   ```shell
   mkdir /etc/systemd/system/gitlab-runner.service.d
   ```

1. Create a file called `/etc/systemd/system/gitlab-runner.service.d/http-proxy.conf`
   that adds the `HTTP_PROXY` environment variable(s):

   ```ini
   [Service]
   Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
   Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
   ```

1. Save the file and flush changes:

   ```shell
   systemctl daemon-reload
   ```

1. Restart GitLab Runner:

   ```shell
   sudo systemctl restart gitlab-runner
   ```

1. Verify that the configuration has been loaded:

   ```shell
   systemctl show --property=Environment gitlab-runner
   ```

   You should see:

   ```ini
   Environment=HTTP_PROXY=http://docker0_interface_ip:3128/ HTTPS_PROXY=http://docker0_interface_ip:3128/
   ```

## Adding the Proxy to the Docker containers

After you [register your runner](../register/index.md), you may want to
propagate your proxy settings to the Docker containers (for example, for `git clone`).

To do this, you need to edit `/etc/gitlab-runner/config.toml` and add the
following to the `[[runners]]` section:

```toml
pre_get_sources_script = "git config --global http.proxy $HTTP_PROXY; git config --global https.proxy $HTTPS_PROXY"
environment = ["https_proxy=http://docker0_interface_ip:3128", "http_proxy=http://docker0_interface_ip:3128", "HTTPS_PROXY=docker0_interface_ip:3128", "HTTP_PROXY=docker0_interface_ip:3128"]
```

Where `docker0_interface_ip` is the IP address of the `docker0` interface.

NOTE:
In our examples, we are setting both lower case and upper case variables
because certain programs expect `HTTP_PROXY` and others `http_proxy`.
Unfortunately, there is no
[standard](https://unix.stackexchange.com/questions/212894/whats-the-right-format-for-the-http-proxy-environment-variable-caps-or-no-ca#212972)
on these kinds of environment variables.

## Proxy settings when using dind service

When using the [Docker-in-Docker executor](https://docs.gitlab.com/ee/ci/docker/using_docker_build.html#use-docker-in-docker-executor) (dind),
it may be necessary to specify `docker:2375,docker:2376` in the `NO_PROXY` environment variable. The ports are required, otherwise `docker push` is blocked.

Communication between `dockerd` from dind and the local `docker` client (as described here: <https://hub.docker.com/_/docker/>)
uses proxy variables held in root's Docker configuration.

To configure this, you need to edit `/root/.docker/config.json` to include your complete proxy configuration, for example:

```json
{
    "proxies": {
        "default": {
            "httpProxy": "http://proxy:8080",
            "httpsProxy": "http://proxy:8080",
            "noProxy": "docker:2375,docker:2376"
        }
    }
}
```

In order to pass on the settings to the container of the Docker executor, a `$HOME/.docker/config.json` also needs to be created inside the container. This may be scripted as a `before_script` in the `.gitlab-ci.yml`, for example:

```yaml
before_script:
  - mkdir -p $HOME/.docker/
  - 'echo "{ \"proxies\": { \"default\": { \"httpProxy\": \"$HTTP_PROXY\", \"httpsProxy\": \"$HTTPS_PROXY\", \"noProxy\": \"$NO_PROXY\" } } }" > $HOME/.docker/config.json'
```

Or alternatively, in the configuration of the `gitlab-runner` (`/etc/gitlab-runner/config.toml`) that is affected:

```toml
[[runners]]
  pre_build_script = "mkdir -p $HOME/.docker/ && echo \"{ \\\"proxies\\\": { \\\"default\\\": { \\\"httpProxy\\\": \\\"$HTTP_PROXY\\\", \\\"httpsProxy\\\": \\\"$HTTPS_PROXY\\\", \\\"noProxy\\\": \\\"$NO_PROXY\\\" } } }\" > $HOME/.docker/config.json"
```

NOTE:
An additional level of escaping `"` is needed here because this is the creation of a
JSON file with a shell specified as a single string inside a TOML file.
Because this is not YAML, do not escape the `:`.

Note that if the `NO_PROXY` list needs to be extended, wildcards `*` only work for suffixes,
but not for prefixes or CIDR notation.
For more information, see
<https://github.com/moby/moby/issues/9145>
and
<https://unix.stackexchange.com/questions/23452/set-a-network-range-in-the-no-proxy-environment-variable>.

## Handling rate limited requests

A GitLab instance may be behind a reverse proxy that has rate-limiting on API requests
to prevent abuse. GitLab Runner sends multiple requests to the API and could go over these
rate limits.

As a result, GitLab Runner handles rate limited scenarios with the following logic:

1. A response code of **429 - TooManyRequests** is received.
1. The response headers are checked for a `RateLimit-ResetTime` header. The `RateLimit-ResetTime` header should have a value which is a valid **HTTP Date (RFC1123)**, like `Wed, 21 Oct 2015 07:28:00 GMT`.
   - If the header is present and has a valid value the runner waits until the specified time and issues another request.
   - If the header is present, but isn't a valid date, a fallback of **1 minute** is used.
   - If the header is not present, no additional actions are taken, the response error is returned.
1. The process above is repeated 5 times, then a `gave up due to rate limit` error is returned.

NOTE:
The header `RateLimit-ResetTime` is case insensitive since all header keys are run
through the [`http.CanonicalHeaderKey`](https://pkg.go.dev/net/http#CanonicalHeaderKey) function.
