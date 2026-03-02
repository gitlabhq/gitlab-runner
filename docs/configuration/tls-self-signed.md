---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Self-signed certificates or custom Certification Authorities
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner provides two options to configure certificates to be used to verify TLS peers:

- **For connections to the GitLab server**: The certificate file can be specified as detailed in the
  [Supported options for self-signed certificates targeting the GitLab server](#supported-options-for-self-signed-certificates-targeting-the-gitlab-server)
  section.

  This solves the `x509: certificate signed by unknown authority` problem when registering a runner.

  For existing Runners, the same error can be seen in Runner logs when trying to check the jobs:

  ```plaintext
  Couldn't execute POST against https://hostname.tld/api/v4/jobs/request:
  Post https://hostname.tld/api/v4/jobs/request: x509: certificate signed by unknown authority
  ```

- **Connecting to a cache server or an external Git LFS store**: A more generic approach
  which also covers other scenarios such as user scripts, a certificate can be specified and
  installed on the container as detailed in the [Trusting TLS certificates for Docker and Kubernetes executors](#trusting-tls-certificates-for-docker-and-kubernetes-executors)
  section.

  An example job log error concerning a Git LFS operation that is missing a certificate:

  ```plaintext
  LFS: Get https://object.hostname.tld/lfs-dev/c8/95/a34909dce385b85cee1a943788044859d685e66c002dbf7b28e10abeef20?X-Amz-Expires=600&X-Amz-Date=20201006T043010Z&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=svcgitlabstoragedev%2F20201006%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-SignedHeaders=host&X-Amz-Signature=012211eb0ff0e374086e8c2d37556f2d8ca4cc948763e90896f8f5774a100b55: x509: certificate signed by unknown authority
  ```

## Supported options for self-signed certificates targeting the GitLab server

This section refers to the situation where only the GitLab server requires a custom certificate.
If other hosts (for example, object storage service without [proxy download enabled](https://docs.gitlab.com/administration/object_storage/#proxy-download))
also require a custom certificate authority (CA), see
the [next section](#trusting-tls-certificates-for-docker-and-kubernetes-executors).

GitLab Runner supports the following options:

- **Default - Read the system certificate**: GitLab Runner reads the system certificate store and verifies the
  GitLab server against the certificate authorities (CA) stored in the system.

- **Specify a custom certificate file**: GitLab Runner exposes the `tls-ca-file` option during [registration](../commands/_index.md#gitlab-runner-register)
  (`gitlab-runner register --tls-ca-file=/path`), and in [`config.toml`](advanced-configuration.md)
  under the `[[runners]]` section. This allows you to specify a custom certificate file.
  This file is read every time the Runner tries to access the GitLab server.
  If you are using GitLab Runner Helm chart, you must configure certificates as described in
  [Access GitLab with a custom certificate](../install/kubernetes_helm_chart_configuration.md#access-gitlab-with-a-custom-certificate).

- **Read a PEM certificate**: GitLab Runner reads the PEM certificate (**DER format is not supported**) from a
  predefined file:
  - `/etc/gitlab-runner/certs/gitlab.example.com.crt` on \*nix systems when GitLab Runner is executed as `root`.

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

  - `~/.gitlab-runner/certs/gitlab.example.com.crt` on \*nix systems when GitLab Runner is executed as non-`root`.
  - `./certs/gitlab.example.com.crt` on other systems. If running GitLab Runner as a Windows service,
    this does not work. Specify a custom certificate file instead.

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

- If you are updating the certificate for an existing Runner, [restart it](../commands/_index.md#gitlab-runner-restart).
- If you already have a Runner configured through HTTP, update your instance path to the new HTTPS URL of your GitLab instance in your `config.toml`.
- As a temporary and insecure workaround, to skip the verification of certificates,
  in the `variables:` section of your `.gitlab-ci.yml` file, set the CI variable `GIT_SSL_NO_VERIFY` to `true`.

### Git cloning

The Runner injects missing certificates to build the CA chain by using `CI_SERVER_TLS_CA_FILE`.
This allows `git clone` and artifacts to work with servers that do not use publicly
trusted certificates.

This approach is secure, but makes the Runner a single point of trust.

## Trusting TLS certificates for Docker and Kubernetes executors

Consider the following information when you register a certificate on a container:

- The [**user image**](https://docs.gitlab.com/ci/yaml/#image), which is used to run the user script.
  For scenarios that involve trusting the certificate for user scripts, the user must take ownership regarding how to install a certificate.
  Certificate installation procedures can vary based on the image. The Runner has no way of knowing how to install a certificate in each
  possible scenario.
- The [**Runner helper image**](advanced-configuration.md#helper-image), which is used to handle Git, artifacts, and cache operations.
  For scenarios that involve trusting the certificate for other CI/CD stages, the user only needs to make a certificate file
  available at a specific location (for example, `/etc/gitlab-runner/certs/ca.crt`), and the Docker container will
  automatically install it for the user.

### Trusting the certificate for user scripts

If your build uses TLS with a self-signed certificate or custom certificate, install the certificate
in your build job for peer communication. The Docker container running the user scripts
doesn't have the certificate files installed by default. This might be required to use
a custom cache host, perform a secondary `git clone`, or fetch a file through a tool like `wget`.

To install the certificate:

1. Map the necessary files as a Docker volume so that the Docker container that runs
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

1. **Linux-only**: Use the mapped file (for example, `ca.crt`) in a [`pre_build_script`](advanced-configuration.md#the-runners-section) that:
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
      image = "mcr.microsoft.com/windows/servercore:21H2"

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
   gitlab-runner:
     runners:
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

   The Kubernetes executor's handling of the helper image's `ENTRYPOINT` has a
   [known issue](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28484).
   When a certificate file is mapped, it isn't automatically installed
   to the system certificate store.

## Troubleshooting

Refer to the general [SSL troubleshooting](https://docs.gitlab.com/omnibus/settings/ssl/ssl_troubleshooting/)
documentation.

In addition, you can use the [`tlsctl`](https://gitlab.com/gitlab-org/ci-cd/runner-tools/tlsctl) tool to debug GitLab certificates from the Runner's end.

### Error: `x509: certificate signed by unknown authority`

This error can occur while trying to pull executor images from private registry when the Docker host
or Kubernetes node where the runner schedules the executors does not trust the private registry's certificate.

To fix the error, add the relevant root certificate authority or certificate chain to the system's trust store
and restart the container service.

If you're on Ubuntu or Alpine, run the following commands:

```shell
cp ca.crt /usr/local/share/ca-certificates/ca.crt
update-ca-certificates
systemctl restart docker.service
```

For operating systems other than Ubuntu or Alpine, see your
operating system's documentation to find appropriate commands to install
the trusted certificate.

Depending on your version of GitLab Runner and the Docker host environment,
you might also have to disable the `FF_RESOLVE_FULL_TLS_CHAIN` feature flag.

### `apt-get: not found` errors in jobs

The [`pre_build_script`](advanced-configuration.md#the-runners-section) commands are executed
before every job a runner executes. Distribution-specific commands
like `apk` or `apt-get` can cause issues. When you install a certificate for user scripts, your CI jobs might fail
if they use [images](https://docs.gitlab.com/ci/yaml/#image) based on different distributions.

For example, if your CI jobs run Ubuntu and Alpine images, Ubuntu commands fail on Alpine.
The `apt-get: not found` error occurs in jobs with Alpine-based images.
To resolve this issue, do one of the following:

- Write your `pre_build_script` so that it is distribution-independent.
- Use [tags](https://docs.gitlab.com/ci/yaml/#tags) to ensure runners only pick up jobs with compatible images.

### Error: `self-signed certificate in certificate chain`

CI/CD jobs fail with the following error:

```plaintext
fatal: unable to access 'https://gitlab.example.com/group/project.git/': SSL certificate problem: self-signed certificate in certificate chain
```

However, the [OpenSSL debugging commands](https://docs.gitlab.com/omnibus/settings/ssl/ssl_troubleshooting/#useful-openssl-debugging-commands)
do not detect any errors.

This error might occur when Git connects through a proxy that `openssl s_client` troubleshooting commands do not use by default.
To verify if Git uses a proxy to fetch the repository, enable debugging:

```yaml
variables:
  GIT_CURL_VERBOSE: 1
```

To prevent Git from using the proxy, set the `NO_PROXY` variable to include your GitLab hostname:

```yaml
variables:
  NO_PROXY: gitlab.example.com
```
