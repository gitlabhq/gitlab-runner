# GitLab CI provenance

When enabled with variable **RUNNER_GENERATE_ARTIFACTS_METADATA**, runner produces [SLSA provenance v0.2](https://slsa.dev/spec/v0.2/provenance) statements.

You can configure the runner by setting the variable **SLSA_PROVENANCE_SCHEMA_VERSION**.

The supported schema version is:

- [v1](https://slsa.dev/spec/v1/provenance)
