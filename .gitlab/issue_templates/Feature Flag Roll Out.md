<!-- Title suggestion: [Feature flag] Enable <feature-flag-name> -->

## Summary

This issue is to roll out [the feature](<feature-issue-link>) on production,
that is currently behind the `<feature-flag-name>` feature flag.

## Owners

- Most appropriate Slack channel to reach out to: `#<slack-channel-of-dri-team>`
- Best individual to reach out to: @<gitlab-username-of-dri>

## Expectations

### What are we expecting to happen?

<!-- Describe the expected outcome when rolling out this feature -->

### What can go wrong and how would we detect it?

<!-- Data loss, broken pages, stability/availability impact? -->

<!-- Which dashboards from https://dashboards.gitlab.net are most relevant? -->

## Rollout Steps

### Rollout on non-production environments

- Verify the MR that adds the feature flag is merged to `main` and has been deployed, for the GitLab Runner context, to the privately managed runners. This might require a synchronisation with the appropriate team to make sure that the `config.toml` used by those runners are updated to include the newly added feature flag.
    Some feature flags are executor specific and deploying them on the private runners would only make sense if these executors are used. A recommendation should be to make sure that there is an existing runner, using the relevant executor and actively running jobs (GitLab Runner pipeline jobs by example) that exists.
<!-- Delete Incremental roll out if it is not relevant to this deploy -->
- [ ] Deploy the feature flag at a percentage (recommended percentage: 50%) on the concerned private runners managed by the GitLab Runner team
- [ ] Monitor that the error rates did not increase (repeat with a different percentage as necessary).
<!-- End of block for deletes -->
- [ ] Enable the feature globally on all private runners managed by the GitLab Runner team
- [ ] Verify that the feature works as expected.
- [ ] If the feature flag causes end-to-end tests to fail, disable the feature flag on private runner to avoid blocking pipelines

For assistance with end-to-end test failures, please reach out via the [`#g_runner` Slack channel](https://gitlab.enterprise.slack.com/archives/CBQ76ND6W).

### Rollout on production

TBD

## Rollback Steps

TBD

/label <group-label>
/label ~"feature flag" ~"section::ci" ~"group::runner" ~"devops::verify" ~"Category:Runner Core" ~"runner::core"
<!-- Uncomment the appropriate type label
/label ~"type::feature" ~"feature::addition"
/label ~"type::maintenance"
/label ~"type::bug"
-->
/assign @<gitlab-username-of-dri>
/due in 12 weeks
