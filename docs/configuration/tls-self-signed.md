# Self-signed certificates or custom Certification Authorities

> Introduced in GitLab Runner 0.7.0

GitLab Runner allows you to configure certificates that are used to verify TLS peers
when connecting to the GitLab server.

**This solves the `x509: certificate signed by unknown authority` problem when registering a runner.**

For existing Runners, the same error can be shown in Runner logs when trying to check the jobs:

```plaintext
Couldn't execute POST against https://hostname.tld/api/v4/jobs/request:
Post https://hostname.tld/api/v4/jobs/request: x509: certificate signed by unknown authority
```

## Supported options for self-signed certificates

GitLab Runner supports the following options:

- **Default**: GitLab Runner reads the system certificate store and verifies the
  GitLab server against the certificate authorities (CA) stored in the system.

- GitLab Runner reads the PEM certificate (**DER format is not supported**) from a
  predefined file:

  - `/etc/gitlab-runner/certs/hostname.crt` on *nix systems when GitLab Runner is executed as root.
  - `~/.gitlab-runner/certs/hostname.crt` on *nix systems when GitLab Runner is executed as non-root.
  - `./certs/hostname.crt` on other systems. If running Runner as a Windows service,
    this will not work. Use the last option instead.

- If your server address is: `https://my.gitlab.server.com:8443/`, create the
  certificate file at: `/etc/gitlab-runner/certs/my.gitlab.server.com.crt`.
  To verify that the file looks correct, you can use a tool like `openssl`. For example:

  ```shell
  echo | openssl s_client -CAfile /etc/gitlab-runner/certs/gitlab-hostname.tld.crt -connect gitlab-hostname.tld:443
  ```

  NOTE: **Note:**
  You may need to concatenate the intermediate and server certificate for the chain to
  be properly identified. For example, if you have a primary, intermediate, and root certificate,
  you can put all of them into one file:

    ```plaintext
    -----BEGIN CERTIFICATE-----
    (Your primary SSL certificate: your_domain_name.crt)
    -----END CERTIFICATE-----
    -----BEGIN CERTIFICATE-----
    (Your intermediate certificate)
    -----END CERTIFICATE-----
    -----BEGIN CERTIFICATE-----
    (Your root certificate)
    -----END CERTIFICATE-----
    ```

- If you are updating the certificate for an existing Runner, [restart it](../commands/README.md#gitlab-runner-restart).

- GitLab Runner exposes the `tls-ca-file` option during [registration](../commands/README.md#gitlab-runner-register)
  (`gitlab-runner register --tls-ca-file=/path`), and in [`config.toml`](advanced-configuration.md)
  under the `[[runners]]` section. This allows you to specify a custom certificate file.
  This file will be read every time the runner tries to access the GitLab server.

- If you are using GitLab Runner Helm chart, [configure custom certificates](../install/kubernetes.md).

- As a temporary and insecure workaround, to skip the verification of certificates,
in the `variables:` section of your `.gitlab-ci.yml` file, set the CI variable `GIT_SSL_NO_VERIFY` to `true`.

NOTE: **Note:**
If your GitLab server certificate is signed by your CA, use your CA certificate
(not your GitLab server signed certificate). You might need to add the intermediates to the chain as well.

## Git cloning

The runner injects missing certificates to build the CA chain in build containers.
This allows `git clone` and `artifacts` to work with servers that do not use publicly
trusted certificates.

This approach is secure, but makes the runner a single point of trust.

## Trusting TLS certificates for Docker and Kubernetes executors

### Trusting the certificate for user scripts

If your build script needs to communicate with peers through TLS and needs to rely on
a self-signed certificate or custom Certificate Authority, you will need to perform the
certificate installation in the build job, as the user scripts are run in a Docker container
that doesn't have the certificate files installed by default. This might be required to use
a custom cache host, perform a secondary `git clone`, or fetch a file through a tool like `wget`,
for example.

To install the certificate:

1. Map the necessary files as a Docker volume so that the Docker container that will run
   the scripts can see them. Do this by adding a volume inside the respective key inside
   the `[runners.docker]` in the `config.toml` file, for example:

    - **Linux**:

        ```toml
        [[runners]]
         name = "docker"
         url = "https://example.com/"
         token = "TOKEN"
         executor = "docker"

         [runners.docker]
           image = "ubuntu:latest"

           # Add path to your ca.crt file in the volumes list
           volumes = ["/cache", "/path/to-ca-cert-dir/ca.crt:/etc/gitlab-runner/certs/ca.crt:ro"]
        ```

1. **Linux-only**: Use the mapped file (e.g `ca.crt`) in a [`pre_build_script`](./advanced-configuration.md#the-runners-section) that:
    1. Copies it to `/usr/local/share/ca-certificates/ca.crt` inside the Docker container.
    1. Installs it by running `update-ca-certificates --fresh`. For example (commands
       vary based on the distribution you're using):

        - On Ubuntu:

            ```toml
            [[runners]]
              name = "docker"
              url = "https://example.com/"
              token = "TOKEN"
              executor = "docker"

              # Copy and install CA certificate before each job
              pre_build_script = """
              apt-get update -y > /dev/null
              apt-get install -y ca-certificates > /dev/null

              cp /etc/gitlab-runner/certs/ca.crt /usr/local/share/ca-certificates/ca.crt
              update-ca-certificates --fresh > /dev/null
              """
            ```

        - On Alpine:

            ```toml
            [[runners]]
              name = "docker"
              url = "https://example.com/"
              token = "TOKEN"
              executor = "docker"

              # Copy and install CA certificate before each job
              pre_build_script = """
              apk update >/dev/null
              apk add ca-certificates >/dev/null
              rm -rf /var/cache/apk/*

              cp /etc/gitlab-runner/certs/ca.crt /usr/local/share/ca-certificates/ca.crt
              update-ca-certificates --fresh > /dev/null
              """
            ```

### Trusting the certificate for the other CI/CD stages

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/3371) in GitLab 13.3.

You can map a certificate file to `/etc/gitlab-runner/certs/ca.crt` on Linux,
or `C:\GitLab-Runner\certs\ca.crt` on Windows.
This user-defined `ca.crt` file is installed by the Runner helper image at startup, and used
when performing operations like cloning and uploading artifacts, for example.

- **Linux**:

    ```toml
    [[runners]]
     name = "docker"
     url = "https://example.com/"
     token = "TOKEN"
     executor = "docker"

     [runners.docker]
       image = "ubuntu:latest"

       # Add path to your ca.crt file in the volumes list
       volumes = ["/cache", "/path/to-ca-cert-dir/ca.crt:/etc/gitlab-runner/certs/ca.crt:ro"]
    ```

- **Windows**:

    ```toml
    [[runners]]
     name = "docker"
     url = "https://example.com/"
     token = "TOKEN"
     executor = "docker"

     [runners.docker]
       image = "mcr.microsoft.com/windows/servercore:1909"

       # Add directory holding your ca.crt file in the volumes list
       volumes = ["c:\\cache", "c:\\path\\to-ca-cert-dir:C:\\GitLab-Runner\\certs:ro"]
    ```
