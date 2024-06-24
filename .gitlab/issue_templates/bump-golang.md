<!--

These are the steps we should follow when we want to bump the golang version

-->

### Steps

1. [ ] bump golang in [goargs](https://gitlab.com/gitlab-org/language-tools/go/linters/goargs)

   example MR:
   - https://gitlab.com/gitlab-org/language-tools/go/linters/goargs/-/merge_requests/8

1. [ ] bump golang in [runner-linters](https://gitlab.com/gitlab-org/ci-cd/runner-tools/runner-linters)

   example MR:
   - https://gitlab.com/gitlab-org/ci-cd/runner-tools/runner-linters/-/merge_requests/7

1. [ ] bump golang et al in [gitlab-runner](https://gitlab.com/gitlab-org/gitlab-runner)

   Things we want to bump:
   - the golang version itself
   - the version of the runner-linters image
   - Update `GO_FIPS_VERSION_SUFFIX`, get the suffix from [here](https://github.com/golang-fips/go/releases)
   - Poke some files to force rebuild of images:
     ```
     find . -name '*.rebuild' | xargs -r -n1 "$SHELL" -c 'date -u > "$1"' --
     ```

   example MRs:
   - https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4838/
   - https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3889
