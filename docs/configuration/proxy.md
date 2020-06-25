# Running GitLab Runner behind a proxy

This guide aims specifically to making GitLab Runner with Docker executor work behind a proxy.

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
world won't.

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

NOTE: **Note:**
The following apply to OSes that have systemd support.

Follow [Docker's documentation](https://docs.docker.com/config/daemon/systemd/#httphttps-proxy)
how to use a proxy.

The service file should look like this:

```ini
[Service]
Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
```

## Adding Proxy variables to the Runner configuration

The proxy variables need to also be added to the Runner's configuration, so that it can
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

## Adding the proxy to the Docker containers

After you [registered your Runner](../register/index.md), you might want to
propagate your proxy settings to the Docker containers (for `git clone` and other
stuff).

To do that, you need to edit `/etc/gitlab-runner/config.toml` and add the
following to the `[[runners]]` section:

```toml
pre_clone_script = "git config --global http.proxy $HTTP_PROXY; git config --global https.proxy $HTTPS_PROXY"
environment = ["https_proxy=http://docker0_interface_ip:3128", "http_proxy=http://docker0_interface_ip:3128", "HTTPS_PROXY=docker0_interface_ip:3128", "HTTP_PROXY=docker0_interface_ip:3128"]
```

Where `docker0_interface_ip` is the IP address of the `docker0` interface. You need to
be able to reach it from within the Docker containers, so it's important to set
it right.

NOTE: **Note:**
In our examples, we are setting both lower case and upper case variables
because certain programs expect `HTTP_PROXY` and others `http_proxy`.
Unfortunately, there is no
[standard](https://unix.stackexchange.com/questions/212894/whats-the-right-format-for-the-http-proxy-environment-variable-caps-or-no-ca#212972)
on these kinds of environment variables.

## Proxy settings when using dind service

When using the [Docker-in-Docker executor](https://docs.gitlab.com/ee/ci/docker/using_docker_build.html#use-docker-in-docker-executor) (dind),
it can be necessary to specify `docker:2375,docker:2376` in the `NO_PROXY`
environment variable. This is because the proxy intercepts the TCP connection between:

- `dockerd` from the dind container.
- `docker` from the client container.

The ports can be required because otherwise `docker push` will be blocked
as it originates from the IP mapped to Docker. However, in that case, it is meant to go through the proxy.

When testing the communication between `dockerd` from dind and a `docker` client locally
(as described here: <https://hub.docker.com/_/docker/>),
`dockerd` from dind is initially started as a client on the host system by root,
and the proxy variables are taken from `/root/.docker/config.json`.

For example:

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

However, the container started for executing `.gitlab-ci.yml` scripts will have
the environment variables set by the settings of the `gitlab-runner` configuration (`/etc/gitlab-runner/config.toml`).
These are available as environment variables as is (in contrast to `.docker/config.json` of the local test above)
in the dind containers running `dockerd` as a service and `docker` client executing `.gitlab-ci.yml`.
In `.gitlab-ci.yml`, the environment variables will be picked up by any program honouring the proxy settings from default environment variables. For example,
`wget`, `apt`, `apk`, `docker info` and `docker pull` (but not by `docker run` or `docker build` as per:
<https://github.com/moby/moby/issues/24697#issuecomment-366680499>).

`docker run` or `docker build` executed inside the container of the Docker executor
will look for the proxy settings in `$HOME/.docker/config.json`,
which is now inside the executor container (and initially empty).
Therefore, `docker run` or `docker build` executions will have no proxy settings. In order to pass on the settings,
a `$HOME/.docker/config.json` needs to be created in the executor container. For example:

```yaml
before_script:
  - mkdir -p $HOME/.docker/
  - 'echo "{ \"proxies\": { \"default\": { \"httpProxy\": \"$HTTP_PROXY\", \"httpsProxy\": \"$HTTPS_PROXY\", \"noProxy\": \"$NO_PROXY\" } } }" > $HOME/.docker/config.json'
```

Because it is confusing to add additional lines in a `.gitlab-ci.yml` file that are only needed in case of a proxy,
it is better to move the creation of the `$HOME/.docker/config.json` into the
configuration of the `gitlab-runner` (`/etc/gitlab-runner/config.toml`) that is actually affected:

```toml
[[runners]]
  pre_build_script = "mkdir -p $HOME/.docker/ && echo \"{ \\\"proxies\\\": { \\\"default\\\": { \\\"httpProxy\\\": \\\"$HTTP_PROXY\\\", \\\"httpsProxy\\\": \\\"$HTTPS_PROXY\\\", \\\"noProxy\\\": \\\"$NO_PROXY\\\" } } }\" > $HOME/.docker/config.json"
```

NOTE: **Note:**
An additional level of escaping `"` is needed here because this is the creation of a
JSON file with a shell specified as a single string inside a TOML file.
Because it is not YAML anymore, do not escape the `:`.

Note that if the `NO_PROXY` list needs to be extended, wildcards `*` only work for suffixes
but not for prefixes or CIDR notation.
For more information, see
<https://github.com/moby/moby/issues/9145>
and
<https://unix.stackexchange.com/questions/23452/set-a-network-range-in-the-no-proxy-environment-variable>.

## Handling rate limited requests

A GitLab instance may be behind a reverse proxy that has rate-limiting on API requests
to prevent abuse. GitLab Runner sends multiple requests to the API and could go over these
rate limits. As a result, GitLab Runner handles rate limited scenarios with the following logic:

1. A response code of **429 - TooManyRequests** is received.
1. The response headers are checked for a `RateLimit-ResetTime` header. The `RateLimit-ResetTime` header should have a value which is a valid **HTTP Date (RFC1123)**, like `Wed, 21 Oct 2015 07:28:00 GMT`.
   - If the header is present and has a valid value the Runner waits until the specified time and issues another request.
   - If the header is present, but isn't a valid date, a fallback of **1 minute** is used.
   - If the header is not present, no additional actions are taken, the response error is returned.
1. The process above is repeated 5 times, then a `gave up due to rate limit` error is returned.

NOTE: **Note:**
The header `RateLimit-ResetTime` is case insensitive since all header keys are run
through the [`http.CanonicalHeaderKey`](https://golang.org/pkg/net/http/#CanonicalHeaderKey) function.
