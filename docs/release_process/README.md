# GitLab Runner release process

To handle the growth of this project, in `v1.4` we've introduced a release process correlated
with GitLab's CE/EE release process.

## Release roadmap

Starting with `v1.4` GitLab Runner will be released on the 22th day of each month - together
with GitLab CE and GitLab EE projects.

> What to describe:
> - release timeline
>     - working on features
>     - feature window closed - bugfixes and documentation update
>
>        > **Note:** there is nothing bad in moving a feature do the next release at this stage, if
>        > it's still not working well!
>
>     - release day
>     - release fixes and documentation improvements
> - _backlog_ milestone
> - no RC packages - constantly updated Bleeding Edge

## Git workflow, merging and tagging strategy

> What to describe:
> - how normal workflow works
> - when to tag
> - how merge patch release fixes into master (or vice versa)
> - x-y-stable branch and cherry-pick
> - pick-into-stable label

## Documentation

> What to describe:
> - documentation tips
> - when to create documentation
> - how to mark features that need modifications in both Runner and GitLab CE/EE

## Patch releases

> What to describe:
> - when to prepare
> - how to release
> - how many releases backward we will support