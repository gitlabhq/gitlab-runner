<!--

These are the steps we should follow when we want to bump the golang version

-->

### Steps

1. [ ] Bump golang in [runner-linters](https://gitlab.com/gitlab-org/ci-cd/runner-tools/runner-linters).

   Example MR:
   - https://gitlab.com/gitlab-org/ci-cd/runner-tools/runner-linters/-/merge_requests/52

1. [ ] Bump golang in [base-images](https://gitlab.com/gitlab-org/ci-cd/runner-tools/base-images).

   Things we want to bump:
   - `GOLANG_VERSION` and `GO_FIPS_VERSION_SUFFIX` in `dockerfiles/ci/docker-bake.hcl`, get the suffix from [here](https://github.com/golang-fips/go/releases).
   - Update the Go version in `dockerfiles/tests/docker-bake.hcl` to match the Runner Go version.
   - Update `GOLANG_VERSION` in `dockerfiles/runner-helper/docker-bake.hcl` to the latest patch release of its existing Go minor version. This builds the Git wrapper using its own Go version.
   - In the base-images MR, add the Go bump entry to the newest untagged `## v0.0.<N>` section in the base-images `CHANGELOG.md`. If the newest section already has a tag, start a new `## v0.0.<N>` section instead.
   - The base-images `release-from-changelog` job releases the newest `## v0.0.<N>` section when it has no tag yet. It runs in the base-images daily scheduled pipeline, or can be run manually on the base-images default branch.

   Example MR:
   - https://gitlab.com/gitlab-org/ci-cd/runner-tools/base-images/-/merge_requests/210

1. [ ] Bump golang et al in [gitlab-runner](https://gitlab.com/gitlab-org/gitlab-runner).

   Things we want to bump:
   - Bump `GO_VERSION`, then run `make sync-updated-go-version` to update `docs/development/_index.md` and `scripts/vagrant/provision/base.ps1`. Then run `scripts/check-go-version-consistency` locally and fix every mismatch it still reports.
   - Update `RUNNER_IMAGES_VERSION` in `.gitlab/ci/_common.gitlab-ci.yml` to the new base-images version. The Runner CI image comes from base-images, so the base-images release must exist first.
   - When the runner-linters bump changes the linter version, update `GOLANGLINT_VERSION` in `.gitlab/ci/_common.gitlab-ci.yml` and `golangci-lint` in `mise.toml` to the same version.
   - `RUNNER_LINTERS_IMAGE` is `runner-linters:<GOLANGLINT_VERSION>-go<GO_VERSION>`, so that tag must exist in the runner-linters registry before you bump `GO_VERSION`.

   Example MR:
   - https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/7675

1. [ ] Backport the bump to the supported stable branches where needed.

   Example MRs:
   - https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/7681
   - https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/7688
