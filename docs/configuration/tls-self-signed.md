# Self-signed certificates or custom Certification Authorities

> Introduced in GitLab Runner 0.7.0

GitLab Runner allows you to configure certificates that are used to verify TLS peers
when connecting to the GitLab server.

**This solves the `x509: certificate signed by unknown authority` problem when registering a runner.**

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

  If your server address is: `https://my.gitlab.server.com:8443/`, create the
  certificate file at: `/etc/gitlab-runner/certs/my.gitlab.server.com.crt`.

  NOTE: **Note:**
  You may need to concatenate the intermediate and server certificate for the chain to
  be properly identified.

- GitLab Runner exposes the `tls-ca-file` option during [registration](../commands/README.md#gitlab-runner-register)
  (`gitlab-runner register --tls-ca-file=/path`), and in [`config.toml`](advanced-configuration.md)
  under the `[[runners]]` section. This allows you to specify a custom certificate file.
  This file will be read every time the runner tries to access the GitLab server.

NOTE: **Note:**
If your GitLab server certificate is signed by your CA, use your CA certificate
(not your GitLab server signed certificate). You might need to add the intermediates to the chain as well.

## Git cloning

The runner injects missing certificates to build the CA chain in build containers.
This allows `git clone` and `artifacts` to work with servers that do not use publicly
trusted certificates.

This approach is secure, but makes the runner a single point of trust.
