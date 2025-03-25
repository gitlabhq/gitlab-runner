This directory contains public certificates for signing GitLab Runner binaries.

### Certificates

#### `gitlab-inc-ssl-com.crt`

This certificate is issued by SSL.com and used to sign Windows binaries.

Valid from 2025-03-18 to 2027-11-20.

### Signing process

The private key for the certificates are stored in a Google Cloud
HSM. The following diagram shows how GitLab Runner binaries are signed:

```mermaid
sequenceDiagram
    participant CI as GitLab CI Job
    participant OIDC as GitLab OIDC Provider
    participant GCP as GCP
    participant Project as gitlab-runner-signing Project
    participant HSM as HSM
    participant Binary as Windows Binary

    CI->>OIDC: Request OIDC JWT token
    OIDC-->>CI: Return JWT token with job claims

    CI->>GCP: Exchange JWT for GCP access token<br/>(sts.googleapis.com)
    GCP-->>CI: GCP access token

    CI->>Project: Impersonate service account using token
    Project-->>CI: Service account credentials

    CI->>HSM: Request access to signing key<br/>(via Google PKCS11 library)
    HSM-->>CI: Provide secure access to key

    CI->>Binary: Sign binary using HSM key<br/>(without extracting private key)

    Binary-->>CI: Return signed binary
```

The `binaries` CI job uses `scripts/sign-binaries` to sign binaries. For Windows binaries, the
script uses [`osslsigncode`](https://github.com/mtrojnar/osslsigncode)
with the [Google PKCS11 library](https://github.com/GoogleCloudPlatform/kms-integrations). See
[the user guide](https://github.com/GoogleCloudPlatform/kms-integrations/blob/master/kmsp11/docs/user_guide.md)
for more details.
