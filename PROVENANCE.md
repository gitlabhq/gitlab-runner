# GitLab CI provenance

This is an official [SLSA Provenance](https://slsa.dev/provenance/v1)
`buildType` that describes the execution of a GitLab [CI/CD job](https://docs.gitlab.com/ci/jobs/).

This definition is hosted and maintained by GitLab. When enabled with the
`RUNNER_GENERATE_ARTIFACTS_METADATA` CI/CD variable, the runner produces [SLSA provenance
v1.0](https://slsa.dev/spec/v1.0/provenance) statements.

## Description

```jsonc
"buildType": "https://gitlab.com/gitlab-org/gitlab-runner/-/blob/{GITLAB_RUNNER_VERSION}/PROVENANCE.md"
```

This `buildType` describes the execution of a workflow that builds a software
artifact.

> [!note]
> Consumers should ignore unrecognized external parameters. Any changes must
> not change the semantics of existing external parameters.

## Build Definition

### Internal and external parameters

Both internal and external parameters are documented in the [Configuring runners documentation](https://docs.gitlab.com/ci/runners/configure_runners/#provenance-metadata-format).

An example provenance statement can also be found in that page.
