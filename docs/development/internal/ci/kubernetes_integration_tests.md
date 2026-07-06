---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Kubernetes integration tests
---

Kubernetes integration tests run in GitLab Runner's CI/CD pipeline. These tests verify that the GitLab Runner
works correctly with Kubernetes clusters. These tests run against a dedicated Kubernetes cluster managed by the
[runner-Kubernetes-infra](https://gitlab.com/gitlab-org/ci-cd/runner-tools/runner-kubernetes-infra) repository.

## Test infrastructure

### Runner Kubernetes infrastructure repository

The test infrastructure is hosted at:

- Repository: <https://gitlab.com/gitlab-org/ci-cd/runner-tools/runner-kubernetes-infra>
- Purpose: Manages dedicated Kubernetes clusters for GitLab Runner integration testing
- Cluster: `runner-k8s` in GCP (see internal documentation for project details and zone)

The infrastructure uses a blue-green deployment model with two separate clusters to enable zero-downtime updates.

### Cluster configuration

For detailed cluster configuration including node pools, resource limits, and autoscaling settings, see the
[cluster configuration](https://gitlab.com/gitlab-org/ci-cd/runner-tools/runner-kubernetes-infra#cluster-configuration)
section in the infrastructure repository.

## Pipeline structure

### Test pipeline stages

The integration tests run through the following GitLab CI/CD stages:

1. Provision integration Kubernetes (`provision integration kubernetes`):
   - Provisions test-specific RBAC resources
   - Creates service account `k8s-runner-integration-tests-runner-$CI_PIPELINE_ID`
   - Executes `mage k8s:provisionIntegrationKubernetes $CI_PIPELINE_ID`

1. Integration test jobs (parallel execution):
   - `integration kubernetes`: Standard integration tests
   - `integration kubernetes exec legacy`: Tests with legacy execution strategy
   - `integration kubernetes attach`: Tests with attach execution strategy
   - `integration kubernetes concrete`: Tests with Concrete-mode (`FF_CONCRETE`) native-steps dispatch

1. Cleanup (`destroy integration kubernetes`):
   - Destroys test-specific resources
   - Executes `mage k8s:destroyIntegrationKubernetes $CI_PIPELINE_ID`

### Pipeline configuration

The pipeline is defined in `.gitlab/ci/test-kubernetes-integration.gitlab-ci.yml`:

```yaml
.integration kubernetes:
  extends:
    - .rules:merge_request_pipelines:no_docs:no-community-mr
  tags:
    - $KUBERNETES_RUNNER_INTEGRATION_TAG
  stage: test kubernetes integration
  variables:
    KUBERNETES_SERVICE_ACCOUNT_OVERWRITE: "k8s-runner-integration-tests-runner-$CI_PIPELINE_ID"
```

### Test execution

Integration tests are executed using `gotestsum`:

```shell
gotestsum --format=testname --format-hide-empty-pkg --rerun-fails=3 \
  --hide-summary=output --packages=gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes \
  --junitfile=junit_report.xml --junitfile-hide-empty-pkg -- \
  -timeout=10m -parallel=20 $EXTRA_GO_TEST_FLAGS \
  -tags=integration,kubernetes ./executors/kubernetes/...
```

Key parameters:

- Timeout: 10 minutes per test
- Parallel execution: Up to 20 tests simultaneously
- Retry logic: Failing tests are retried up to 3 times
- Build tags: `integration,kubernetes`

## Test categories

### Standard integration tests

- Job: `integration kubernetes`
- Purpose: Main integration test suite
- Feature flags: Uses default feature flag configuration
- Filter: Excludes feature flag-specific tests with `-skip=TestRunIntegrationTestsWithFeatureFlag`

### Legacy execution strategy tests

- Job: `integration kubernetes exec legacy`
- Feature flag: `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY=true`
- Filter: Only runs `TestRunIntegrationTestsWithFeatureFlag`
- Purpose: Validates backward compatibility

### Attach strategy tests

- Job: `integration kubernetes attach`
- Feature flag: `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY=false`
- Filter: Only runs `TestRunIntegrationTestsWithFeatureFlag`
- Purpose: Tests the newer attach-based execution strategy

### Concrete-mode tests

- Job: `integration kubernetes concrete`
- Feature flag: `FF_CONCRETE=true`
- Filter: Only runs `TestRunIntegrationTestsWithFeatureFlag`
- Purpose: Validates native-steps dispatch routed through `steps.Connector`
  (`Build.UseNativeSteps()` returns true under this flag).

## RBAC and permissions

### Dynamic permission provisioning

The provisioning system (`mage k8s:provisionIntegrationKubernetes`) analyzes the codebase to generate the minimal required RBAC permissions:

1. Code analysis: Scans `/executors/kubernetes/` for Kubernetes API calls
1. Permission generation: Creates the role YAML with only required permissions
1. Resource creation: Applies the generated RBAC to the `k8s-runner-integration-tests` namespace

This system ensures tests use the same permissions as the code under test.

### Test-specific service accounts

Each pipeline creates unique resources:

- Service account: `k8s-runner-integration-tests-runner-$CI_PIPELINE_ID`
- Role: Generated based on code analysis
- Role binding: Links service account to generated role

### Administrative permissions

Integration tests also use administrative RBAC for test management:

- Service account: `integration-tests-admin`
- Purpose: Create/delete test resources, observe cluster state
- Scope: Additional permissions beyond normal runner operations

## Test implementation

### Test environment

Tests run with the following environment variables:

- `KUBERNETES_SERVICE_ACCOUNT_OVERWRITE`: Pipeline-specific service account
- Feature flag variables (for feature flag tests)
- Cluster connection details (managed by infrastructure)

## Resource management

### Automated cleanup

The infrastructure includes automated cleanup mechanisms. For detailed information about CronJobs, scheduling, and configuration, see the
[operational automation](https://gitlab.com/gitlab-org/ci-cd/runner-tools/runner-kubernetes-infra#operational-automation) section in the infrastructure repository.

### Resource isolation

Tests use resource groups to prevent conflicts:

- `"$CI_COMMIT_REF_SLUG-k8s-integration"`
- `"$CI_COMMIT_REF_SLUG-k8s-integration-exec-legacy"`
- `"$CI_COMMIT_REF_SLUG-k8s-integration-attach"`
- `"$CI_COMMIT_REF_SLUG-k8s-integration-concrete"`

## Monitoring and observability

### Metrics and logging

The test infrastructure includes comprehensive monitoring and logging. For information on accessing Grafana,
Prometheus dashboards, log aggregation with Loki, and the available `make` commands, see the
[metrics](https://gitlab.com/gitlab-org/ci-cd/runner-tools/runner-kubernetes-infra#metrics) and
[log collection](https://gitlab.com/gitlab-org/ci-cd/runner-tools/runner-kubernetes-infra#log-collection)
sections in the infrastructure repository.

## Troubleshooting

### Common issues

- Test timeouts:
  - Check cluster resource availability.
  - Verify worker pool scaling (0-6 nodes).
  - Review test parallelism settings.
- RBAC permissions:
  - Ensure provisioning job succeeded.
  - Verify service account creation.
  - Check generated Role matches code requirements.
- Resource conflicts:
  - Check resource group isolation.
  - Verify cleanup job execution.
  - Review pipeline-specific naming.

### Debugging steps

1. Check the infrastructure status. For more information about the `make` commands and infrastructure management, see [blue-green deployment](https://gitlab.com/gitlab-org/ci-cd/runner-tools/runner-kubernetes-infra#blue-green-deployment).
1. Review test logs:
   - Check pipeline job logs for specific failures.
   - Use Grafana dashboard for aggregated logs. For more information, see [log collection](https://gitlab.com/gitlab-org/ci-cd/runner-tools/runner-kubernetes-infra#log-collection).
   - Review `gotestsum` output for test-specific issues.

1. Validate RBAC:

   ```shell
   kubectl get sa,role,rolebinding -n k8s-runner-integration-tests
   kubectl describe role k8s-runner-integration-tests-runner-$CI_PIPELINE_ID -n k8s-runner-integration-tests
   ```

## Running tests locally

Integration tests can run against any reachable Kubernetes cluster
(`kubectl cluster-info` must succeed). Tests skip silently when the
cluster is unreachable, so a missing cluster is a no-op rather than a
hard error.

For most local development, use unit tests or a local cluster
(`kind`, `minikube`, Rancher Desktop) with appropriate RBAC setup.
For full parity with the dedicated CI/CD cluster, access to the GKE
cluster, matching RBAC permissions, and CI/CD environment variables are
required.

### Selecting a matrix mode

`TestRunIntegrationTestsWithFeatureFlag` is the same test suite that
runs once per matrix CI/CD job, parameterized by two environment variables. To
reproduce a specific mode locally:

```shell
CI_RUNNER_TEST_FEATURE_FLAG=FF_CONCRETE \
CI_RUNNER_TEST_FEATURE_FLAG_VALUE=true \
go test -tags=integration,kubernetes -timeout=30m -v \
  -run TestRunIntegrationTestsWithFeatureFlag \
  ./executors/kubernetes/...
```

Substitute `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY` (with `true`
or `false`) to reproduce the `integration kubernetes exec legacy` or
`integration kubernetes attach` jobs respectively.

### Overriding the helper image

By default, the fixture pins the helper image (through
`withDevHelperImage` in `getTestBuild`) to the one built by the
`(development|bleeding|stable) docker images` jobs of the current
pipeline. Locally, when `CI_PROJECT_DIR` is unset, the fixture falls
back to the runner's compiled-in default helper image.

To run against a specific helper image (a published bleeding tag, a
locally-built image, or a debugging variant), set:

```shell
CI_RUNNER_TEST_HELPER_IMAGE=registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:bleeding
```

This override takes precedence over both the artifact lookup and the
compiled-in default. It enables an iterative loop for helper-side
changes: build the helper image locally, push it to any registry the
test cluster can pull from, and point `CI_RUNNER_TEST_HELPER_IMAGE`
at that pushed tag. No CI/CD pipeline required.

Required for `FF_CONCRETE` runs when the default helper image lacks
the `steps` subcommand.

## Related topics

- [GitLab Runner Kubernetes executor](../../../executors/kubernetes/_index.md)
- [Runner Kubernetes infrastructure repository](https://gitlab.com/gitlab-org/ci-cd/runner-tools/runner-kubernetes-infra)
- [GitLab Runner Infrastructure Toolkit (GRIT)](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit)
