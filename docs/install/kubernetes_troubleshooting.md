---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Troubleshooting GitLab Runner Helm chart
---

## Error: `Job failed (system failure): secrets is forbidden`

If you see the following error, [enable RBAC support](kubernetes_helm_chart_configuration.md#enable-rbac-support) to correct it:

```plaintext
Using Kubernetes executor with image alpine ...
ERROR: Job failed (system failure): secrets is forbidden: User "system:serviceaccount:gitlab:default"
cannot create resource "secrets" in API group "" in the namespace "gitlab"
```

## Error: `Unable to mount volumes for pod`

If you see mount volume failures for a required secret, ensure that you have
stored registration tokens or runner tokens in secrets.

## Slow artifact uploads to Google Cloud Storage

Artifact uploads to Google Cloud Storage can experience reduced performance (a slower bandwidth rate)
due to the runner helper pod becoming CPU bound. To mitigate this problem, increase the Helper pod CPU Limit:

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        helper_cpu_limit = "250m"
```

For more information, see [issue 28393](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28393#note_722733798).

## Error: `PANIC: creating directory: mkdir /nonexistent: permission denied`

To resolve this error, switch to the
[Ubuntu-based GitLab Runner Docker image](kubernetes_helm_chart_configuration.md#switch-to-the-ubuntu-based-gitlab-runner-docker-image).

## Error: `invalid header field for "Private-Token"`

You might see this error if the `runner-token` value in `gitlab-runner-secret`
is base64-encoded with a newline character (`\n`) at the end:

```plaintext
couldn't execute POST against "https:/gitlab.example.com/api/v4/runners/verify":
net/http: invalid header field for "Private-Token"
```

To resolve this issue, ensure a newline (`\n`) is not appended to the token value.
For example: `echo -n <gitlab-runner-token> | base64`.

## Error: `FATAL: Runner configuration is reserved`

You might get the following error in the pod logs after installing the GitLab Runner Helm chart:

```plaintext
FATAL: Runner configuration other than name and executor configuration is reserved
(specifically --locked, --access-level, --run-untagged, --maximum-timeout, --paused, --tag-list, and --maintenance-note)
and cannot be specified when registering with a runner authentication token. This configuration is specified
on the GitLab server. Please try again without specifying any of those arguments
```

This error happens when you use an authentication token, and
provide a token through a secret.
To fix it, review your values YAML file and make sure that you are not using any deprecated values.
For more information about which values are deprecated, see
[Installing GitLab Runner with Helm chart](https://docs.gitlab.com/ci/runners/new_creation_workflow/#installing-gitlab-runner-with-helm-chart).
