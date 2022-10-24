---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# Self-signed certificates or custom Certification Authorities **(FREE)**

> Introduced in GitLab Runner 0.7.0

GitLab Runner provides two options to configure certificates to be used to verify TLS peers:

- **For connections to the GitLab server**: the certificate file can be specified as detailed in the
  [Supported options for self-signed certificates targeting the GitLab server](#supported-options-for-self-signed-certificates-targeting-the-gitlab-server) section.

    **This solves the `x509: certificate signed by unknown authority` problem when registering a runner.**

    For existing Runners, the same error can be seen in Runner logs when trying to check the jobs:

    ```plaintext
    Couldn't execute POST against https://hostname.tld/api/v4/jobs/request:
    Post https://hostname.tld/api/v4/jobs/request: x509: certificate signed by unknown authority
    ```

- **A more generic approach which also covers other scenarios such as user scripts, connecting to a cache server or an external Git LFS store**:
  a certificate can be specified and installed on the container as detailed in the
  [Trusting TLS certificates for Docker and Kubernetes executors](#trusting-tls-certificates-for-docker-and-kubernetes-executors) section.

    An example job log error concerning a Git LFS operation that is missing a certificate:

    ```plaintext
    LFS: Get https://object.hostname.tld/lfs-dev/c8/95/a34909dce385b85cee1a943788044859d685e66c002dbf7b28e10abeef20?X-Amz-Expires=600&X-Amz-Date=20201006T043010Z&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=svcgitlabstoragedev%2F20201006%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-SignedHeaders=host&X-Amz-Signature=012211eb0ff0e374086e8c2d37556f2d8ca4cc948763e90896f8f5774a100b55: x509: certificate signed by unknown authority
    ```

## Supported options for self-signed certificates targeting the GitLab server

This section refers to the situation where only the GitLab server requires a custom certificate.
If other hosts (e.g. object storage service without [proxy download enabled](https://docs.gitlab.com/ee/administration/object_storage.html#proxy-download))
also require a custom certificate authority (CA), please see
the [next section](#trusting-tls-certificates-for-docker-and-kubernetes-executors).

GitLab Runner supports the following options:

- **Default - Read the system certificate**: GitLab Runner reads the system certificate store and verifies the
  GitLab server against the certificate authorities (CA) stored in the system. Note that reading from
  the system certificate store is [not supported in Windows](https://github.com/golang/go/issues/16736).

- **Specify a custom certificate file**: GitLab Runner exposes the `tls-ca-file` option during [registration](../commands/index.md#gitlab-runner-register)
  (`gitlab-runner register --tls-ca-file=/path`), and in [`config.toml`](advanced-configuration.md)
  under the `[[runners]]` section. This allows you to specify a custom certificate file.
  This file will be read every time the Runner tries to access the GitLab server.
  If you are using GitLab Runner Helm chart, you will need to configure certificates as described in
  [Providing a custom certificate for accessing GitLab](../install/kubernetes.md#providing-a-custom-certificate-for-accessing-gitlab).

- **Read a PEM certificate**: GitLab Runner reads the PEM certificate (**DER format is not supported**) from a
  predefined file:
  - `/etc/gitlab-runner/certs/gitlab.example.com.crt` on *nix systems when GitLab Runner is executed as `root`.

    If your server address is `https://gitlab.example.com:8443/`, create the
    certificate file at: `/etc/gitlab-runner/certs/gitlab.example.com.crt`.

    You can use the `openssl` client to download the GitLab instance's certificate to `/etc/gitlab-runner/certs`:

    ```shell
    openssl s_client -showcerts -connect gitlab.example.com:443 -servername gitlab.example.com < /dev/null 2>/dev/null | openssl x509 -outform PEM > /etc/gitlab-runner/certs/gitlab.example.com.crt
    ```

    To verify that the file is correctly installed, you can use a tool like `openssl`. For example:

    ```shell
    echo | openssl s_client -CAfile /etc/gitlab-runner/certs/gitlab.example.com.crt -connect gitlab.example.com:443 -servername gitlab.example.com
    ```

  - `~/.gitlab-runner/certs/gitlab.example.com.crt` on *nix systems when GitLab Runner is executed as non-`root`.
  - `./certs/gitlab.example.com.crt` on other systems. If running GitLab Runner as a Windows service,
    this will not work. Specify a custom certificate file instead.

Notes:

- If your GitLab server certificate is signed by your CA, use your CA certificate
(not your GitLab server signed certificate). You might need to add the intermediates to the chain as well.
  For example, if you have a primary, intermediate, and root certificate,
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

- If you are updating the certificate for an existing Runner, [restart it](../commands/index.md#gitlab-runner-restart).
- If you already have a Runner configured through HTTP, update your instance path to the new HTTPS URL of your GitLab instance in your `config.toml`.
- As a temporary and insecure workaround, to skip the verification of certificates,
in the `variables:` section of your `.gitlab-ci.yml` file, set the CI variable `GIT_SSL_NO_VERIFY` to `true`.

### Git cloning

The Runner injects missing certificates to build the CA chain by using `CI_SERVER_TLS_CA_FILE`.
This allows `git clone` and artifacts to work with servers that do not use publicly
trusted certificates.

This approach is secure, but makes the Runner a single point of trust.

## Trusting TLS certificates for Docker and Kubernetes executors

There are two contexts that need to be taken into account when we consider registering a certificate on a container:

- The [**user image**](https://docs.gitlab.com/ee/ci/yaml/#image), which is used to run the user script.
  In this scenario, the user must take ownership regarding how to install a certificate, since this is
  highly dependent on the image itself, and the Runner has no way of knowing how to install a certificate in each
  possible scenario.
- The [**Runner helper image**](advanced-configuration.md#helper-image), which is used to handle Git, artifacts, and cache operations.
  In this scenario, the user only needs to make a certificate file
  available at a specific location (for example, `/etc/gitlab-runner/certs/ca.crt`), and the Docker container will
  automatically install it for the user.

### Trusting the certificate for user scripts

If your build script needs to communicate with peers through TLS and needs to rely on
a self-signed certificate or custom Certificate Authority, you will need to perform the
certificate installation in the build job, as the Docker container running the user scripts
doesn't have the certificate files installed by default. This might be required to use
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

1. **Linux-only**: Use the mapped file (e.g `ca.crt`) in a [`pre_build_script`](advanced-configuration.md#the-runners-section) that:
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
              apk add ca-certificates > /dev/null
              rm -rf /var/cache/apk/*

              cp /etc/gitlab-runner/certs/ca.crt /usr/local/share/ca-certificates/ca.crt
              update-ca-certificates --fresh > /dev/null
              """
            ```

If you just need the GitLab server CA cert that can be used, you can retrieve it from the file stored in the `CI_SERVER_TLS_CA_FILE` variable:

```shell
curl --cacert "${CI_SERVER_TLS_CA_FILE}"  ${URL} -o ${FILE}
```

### Trusting the certificate for the other CI/CD stages

> [Introduced](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/3371) in GitLab 13.3.

You can map a certificate file to `/etc/gitlab-runner/certs/ca.crt` on Linux,
or `C:\GitLab-Runner\certs\ca.crt` on Windows.
The Runner helper image installs this user-defined `ca.crt` file at start-up, and uses it
when performing operations like cloning and uploading artifacts, for example.

#### Docker

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
       image = "mcr.microsoft.com/windows/servercore:2004"

       # Add directory holding your ca.crt file in the volumes list
       volumes = ["c:\\cache", "c:\\path\\to-ca-cert-dir:C:\\GitLab-Runner\\certs:ro"]
    ```

#### Kubernetes

To provide a certificate file to jobs running in Kubernetes:

1. Store the certificate as a Kubernetes secret in your namespace:

   ```shell
   kubectl create secret generic <SECRET_NAME> --namespace <NAMESPACE> --from-file=<CERT_FILE>
   ```

1. Mount the secret as a volume in your runner, replacing `<SECRET_NAME>`
   and `<LOCATION>` with appropriate values:

   ```toml
   config: |
     [[runners]]
       [runners.kubernetes]
         namespace = "{{.Release.Namespace}}"
         image = "ubuntu:latest"
       [[runners.kubernetes.volumes.secret]]
           name = "<SECRET_NAME>"
           mount_path = "<LOCATION>"
   ```

   The `mount_path` is the directory in the container where the certificate is stored.
   If you used `/etc/gitlab-runner/certs/` as the `mount_path` and `ca.crt` as your
   certificate file, your certificate is available at `/etc/gitlab-runner/certs/ca.crt`
   inside your container.
1. As part of the job, install the mapped certificate file to the system certificate store.
   For example, in an Ubuntu container:

   ```yaml
   script:
     - cp /etc/gitlab-runner/certs/ca.crt /usr/local/share/ca-certificates/
     - update-ca-certificates
   ```

  Due to a [known issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28484) in the Kubernetes executor's
  handling of the helper image's `ENTRYPOINT`, the mapped certificate file isn't automatically installed
  to the system certificate store.

## Troubleshooting

Refer to the general [SSL troubleshooting](https://docs.gitlab.com/omnibus/settings/ssl.html#troubleshooting)
documentation.

In addition, you can use the [`tlsctl`](https://gitlab.com/gitlab-org/ci-cd/runner-tools/tlsctl) tool to debug GitLab certificates from the Runner's end.
