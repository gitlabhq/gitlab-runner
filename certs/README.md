This directory contains public certificates for signing GitLab Runner binaries.

### Certificates

#### `gitlab-inc-ssl-com.crt`

This certificate is issued by SSL.com and used to sign Windows binaries.

Valid from 2025-03-18 to 2027-11-20.

#### `apple-developer-id-app-cert.cer`

This certificate is issued by Apple and used to sign macOS binaries.
The certificate can also be [downloaded from the Apple Developer Certificates page (requires access to the GitLab group)](https://developer.apple.com/account/resources/certificates/list).
Note that [Developer ID Application certificates](https://developer.apple.com/support/developer-id/) can only be uploaded by an owner.

Valid from 2025-08-18 to 2030-08-19.

### Windows signing process

The private key for the certificates are stored in a Google Cloud
HSM. The following diagram shows how GitLab Runner binaries are signed:

```mermaid
sequenceDiagram
    participant CI as GitLab CI Job
    participant OIDC as GitLab OIDC Provider
    participant GCP as GCP STS/IAM
    participant Project as gitlab-runner-signing Project
    participant HSM as GCP HSM
    participant Binary as Windows Binary

    CI->>OIDC: Request OIDC JWT token
    OIDC-->>CI: Return JWT token with job claims

    CI->>GCP: Exchange JWT for GCP access token<br/>(sts.googleapis.com)
    GCP-->>CI: GCP access token

    CI->>Project: Impersonate service account using token
    Project-->>CI: Service account credentials

    CI->>Binary: Create binary

    CI->>HSM: Sign binary using HSM key via Google PKCS11 library<br/>(key never leaves HSM)
    HSM-->>CI: Return signature

    CI->>Binary: Apply signature to binary
```

The `binaries` CI job uses `scripts/sign-binaries` to sign binaries.

The private key is never accessed directly by the service account during
the signing process.

For Windows binaries, the script uses [`osslsigncode`](https://github.com/mtrojnar/osslsigncode)
with the [Google PKCS11 library](https://github.com/GoogleCloudPlatform/kms-integrations). See
[the user guide](https://github.com/GoogleCloudPlatform/kms-integrations/blob/master/kmsp11/docs/user_guide.md)
for more details.

For macOS binaries, the script uses [`rcodesign`](https://github.com/indygreg/apple-platform-rs) with [PKCS#11 support](https://github.com/indygreg/apple-platform-rs/pull/198).
See the [documentation](https://gregoryszorc.com/docs/apple-codesign/stable/apple_codesign_getting_started.html) for more details.
Note that we have to [compile our own binary with PKCS#11 support](https://gitlab.com/gitlab-org/ci-cd/runner-tools/base-images/-/merge_requests/54) because:

- The stock `rcodesign` only provides a Linux musl build with a limited feature set.
- `rcodesign` needs to run an RedHat's Univeral Base Image (UBI) 8,
  which ships an older glibc version than most current systems.

Note that the service account needs two [Google KMS IAM roles](https://cloud.google.com/kms/docs/reference/permissions-and-roles#cloudkms.signerVerifier)
for the Google PKCS11 library to work:

- Cloud KMS CryptoKey Signer/Verifier (`roles/cloudkms.signerVerifier`)
- Cloud KMS Viewer (`roles/cloudkms.viewer`)

The Cloud KMS Viewer role allows the account to retrieve metadata about the keys. The diagram omits the fact that
the Google PKCS11 library lists all the keys in the key ring and retrieves information about them.

Ideally, only `roles/cloudkms.signerVerifier` would be needed. There is
an [open feature request to reduce the permission](https://github.com/GoogleCloudPlatform/kms-integrations/issues/45)
when only signing is needed.
