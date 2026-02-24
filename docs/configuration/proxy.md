---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Running GitLab Runner behind a proxy
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

This guide aims specifically to making GitLab Runner with Docker executor work behind a proxy.

Before continuing, ensure that you've already
[installed Docker](https://docs.docker.com/get-started/get-docker/) and
[GitLab Runner](../install/_index.md) on the same machine.

## Configuring `cntlm`

{{< alert type="note" >}}

If you already use a proxy without authentication, this section is optional and
you can skip straight to [configuring Docker](#configuring-docker-for-downloading-images).
Configuring `cntlm` is only needed if you are behind a proxy with authentication,
but it's recommended to use in any case.

{{< /alert >}}

[`cntlm`](https://github.com/versat/cntlm) is a Linux proxy which can be used
as a local proxy and has 2 major advantages compared to adding the proxy details
everywhere manually:

- One single source where you need to change your credentials
- The credentials can not be accessed from the Docker runners

Assuming you [have installed `cntlm`](https://www.howtoforge.com/linux-ntlm-authentication-proxy-isa-server-with-cntlm),
you need to first configure it.

### Make `cntlm` listen to the `docker0` interface

For added security and protection from the internet, bind `cntlm` to listen on
`docker0` interface, which has an IP address that containers can reach.
If you tell `cntlm` on the Docker host to bind only
to this address, Docker containers can reach it, but the outside
world can't.

1. Find the IP that Docker is using:

   ```shell
   ip -4 -oneline addr show dev docker0
   ```

   The IP address is usually `172.17.0.1`, let's call it `docker0_interface_ip`.

1. Open the configuration file for `cntlm` (`/etc/cntlm.conf`). Enter your username,
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

> [!note]
> The following apply to OSes with systemd support.

For information about how to use proxy, see [Docker documentation](https://docs.docker.com/engine/daemon/proxy/).

The service file should look like this:

```ini
[Service]
Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
```

## Adding Proxy variables to the GitLab Runner configuration

The proxy variables need to also be added to the GitLab Runner configuration, so that it can
connect to GitLab.com from behind the proxy.

This action is the same as adding the proxy to the Docker service above:

1. Create a systemd drop-in directory for the `gitlab-runner` service:

   ```shell
   mkdir /etc/systemd/system/gitlab-runner.service.d
   ```

1. Create a file called `/etc/systemd/system/gitlab-runner.service.d/http-proxy.conf`
   that adds the `HTTP_PROXY` environment variables:

   ```ini
   [Service]
   Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
   Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
   ```

   To connect GitLab Runner to any internal URLs, like a GitLab Self-Managed instance,
   set a value for the `NO_PROXY` environment variable.

   ```ini
   [Service]
   Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
   Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
   Environment="NO_PROXY=gitlab.example.com"
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

After you [register your runner](../register/_index.md), you may want to
propagate your proxy settings to the Docker containers (for example, for `git clone`).

To do this, you need to edit `/etc/gitlab-runner/config.toml` and add the
following to the `[[runners]]` section:

```toml
pre_get_sources_script = "git config --global http.proxy $HTTP_PROXY; git config --global https.proxy $HTTPS_PROXY"
environment = ["https_proxy=http://docker0_interface_ip:3128", "http_proxy=http://docker0_interface_ip:3128", "HTTPS_PROXY=docker0_interface_ip:3128", "HTTP_PROXY=docker0_interface_ip:3128"]
```

Where `docker0_interface_ip` is the IP address of the `docker0` interface.

{{< alert type="note" >}}

In our examples, we are setting both lower case and upper case variables
because certain programs expect `HTTP_PROXY` and others `http_proxy`.
Unfortunately, there is no
[standard](https://unix.stackexchange.com/questions/212894/whats-the-right-format-for-the-http-proxy-environment-variable-caps-or-no-ca#212972)
on these kinds of environment variables.

{{< /alert >}}

## Proxy settings when using `dind` service

When using the [Docker-in-Docker executor](https://docs.gitlab.com/ci/docker/using_docker_build/#use-docker-in-docker) (`dind`),
it may be necessary to specify `docker:2375,docker:2376` in the `NO_PROXY` environment variable. The ports are required, otherwise `docker push` is blocked.

Communication between `dockerd` from `dind` and the local `docker` client (as described here: <https://hub.docker.com/_/docker/>)
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

To pass on the settings to the container of the Docker executor, a `$HOME/.docker/config.json` also needs to be created inside the container. This may be scripted as a `before_script` in the `.gitlab-ci.yml`, for example:

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

{{< alert type="note" >}}

An additional level of escaping `"` is required because this creates a
JSON file with a shell specified as a single string inside a TOML file.
Because this is not YAML, do not escape the `:`.

{{< /alert >}}

If the `NO_PROXY` list needs to be extended, wildcards `*` only work for suffixes,
but not for prefixes or CIDR notation.
For more information, see
<https://github.com/moby/moby/issues/9145>
and
<https://unix.stackexchange.com/questions/23452/set-a-network-range-in-the-no-proxy-environment-variable>.

## Handling rate limited requests

A GitLab instance may be behind a reverse proxy that has rate-limiting on API requests
to prevent abuse. GitLab Runner sends multiple requests to the API and could go over these
rate limits.

As a result, GitLab Runner handles rate limited scenarios by using the following [retry logic](#retry-logic):

### Retry logic

When GitLab Runner receives a `429 Too Many Requests` response, it follows this retry sequence:

1. The runner checks the response headers for a `RateLimit-ResetTime` header.
   - The `RateLimit-ResetTime` header should have a value which is a valid HTTP date (RFC1123), like `Wed, 21 Oct 2015 07:28:00 GMT`.
   - If the header is present and has a valid value, the runner waits until the specified time and issues another request.
1. If the `RateLimit-ResetTime` header is invalid or missing, the runner checks the response headers for a `Retry-After` header.
   - The `Retry-After` header should have a value in seconds format, like `Retry-After: 30`.
   - If the header format is present and has a valid value, the runner waits until the specified time and issues another request.
1. If both headers are missing or invalid, the runner waits for the default interval and issues another request.

The runner retries failed requests up to 5 times. If all retries fail, the runner logs the error from the final response.

### Supported header formats

| Header                | Format              | Example                         |
|-----------------------|---------------------|---------------------------------|
| `RateLimit-ResetTime` | HTTP Date (RFC1123) | `Wed, 21 Oct 2015 07:28:00 GMT` |
| `Retry-After`         | Seconds             | `30`                            |

> [!note]
> The header `RateLimit-ResetTime` is case-insensitive because all header keys are run
> through the [`http.CanonicalHeaderKey`](https://pkg.go.dev/net/http#CanonicalHeaderKey) function.
