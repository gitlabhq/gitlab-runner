<!-- Title suggestion: [Feature flag] Cleanup <feature-flag-name> -->

## Summary

This issue is to clean up the `<feature-flag-name>` feature flag, after its default behavior has been in production for an appropriate amount of time.

<!-- Short description of what the feature is about and link to relevant other issues. Note if the feature will be removed completely or will be productized. -->

## Owners

- Team: GitLab Runner
- Most appropriate Slack channel to reach out to: `#g_runner`
- Best individual to reach out to: NAME
- PM: NAME

## Stakeholders

<!--
Are there any other stages or teams involved that need to be kept in the loop?

- Name of a PM
- The Support Team
- The Delivery Team
-->

## Expectations

### What might happen if this goes wrong?

List all the steps that must be taken if something goes wrong:

- Any MRs that need to be rolled back?
- Communication that needs to happen?
- What are some things you can think of that could go wrong in the context of GitLab Runner and the existing setups?
- What settings need to be changed back, for example feature flags or `config.toml` settings?

### Cleaning up the feature flag

In some cases, removing a feature flag also removes a feature. That is a breaking change, and it must be planned
in accordance with the [GitLab breaking change policy](https://docs.gitlab.com/development/deprecation_guidelines/).
A flag that only switches between an old and a new implementation of the same functionality does not remove a
feature. In all cases, before removing a flag:

- Mark the flag as deprecated in `helpers/featureflags/flags.go` by setting `Deprecated: true` and
  `ToBeRemovedWith` to the removal milestone, then run `make update_feature_flags_docs`. This populates the
  **Deprecated** and **To be removed with** columns on the
  [feature flags page](https://docs.gitlab.com/runner/configuration/feature-flags/), which is where the removal
  is announced.
- Remove the flag no earlier than two milestones after it is deprecated. If the flag is deprecated in `N`, remove
  it in `N+2` or later.
- If the flag is promoted to a `config.toml` setting, don't remove the flag in the same merge request. Deprecate
  it and keep it working until it is removed, as above.

<!-- The checklist here is to help stakeholders keep track of the feature flag status -->
- [ ] Specify in the issue description if this feature will be removed completely or will be productized as part of the Feature Flag cleanup
- [ ] Mark the flag as deprecated with a removal milestone and confirm it shows on the feature flags page.
- [ ] Create a merge request to remove the `<feature-flag-name>` feature flag. Ask for review and merge it.
  - [ ] Remove all references to the feature flag from the codebase.
  - [ ] Remove the documentation for the flag from the repository.
  - [ ] Remove the documentation for the flag from related repositories (GitLab, GitLab Runner Helm Chart, GitLab Runner Operator).
- [ ] Ensure that the cleanup MR is merged before the code cutoff.
- [ ] Close [the feature issue](ISSUE LINK) to indicate the feature will be released in the current milestone.
- [ ] Close this feature flag cleanup issue.

/label ~"feature flag" ~"section::ci" ~"group::runner core" ~"devops::verify" ~"Category:Runner Core" ~"runner::core"
<!-- Uncomment the appropriate type label
/label ~"type::feature" ~"feature::addition"
/label ~"type::maintenance" ~"maintenance::removal"
/label ~"type::bug"
-->
