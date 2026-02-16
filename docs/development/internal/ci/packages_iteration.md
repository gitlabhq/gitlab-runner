---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Packages Iteration
---

The `PACKAGES_ITERATION` variable is used to set the iteration of the `deb` and `rpm` packages. From `fpm`'s help:

> --iteration The iteration to give to the package. RPM calls this the 'release'. FreeBSD calls it 'PORTREVISION'. Debian calls this 'Debian_revision'

The `PACKAGES_ITERATION` variable is intended to be incremented manually for cases where broken packages are released.
Instead of removing the packages and re-releasing them, we can increment the iteration and release new packages.

The mage target `package:verifyIterationVariable` is used to ensure that the `PACKAGES_ITERATION` variable is set correctly across branches.
For the rules governing `PACKAGES_ITERATION`, read the documentation on the `VerifyIterationVariable` function.

The workflow is as follows:

- In `main` the `PACKAGES_ITERATION` value will always be `1`.
- When a stable branch is created, the `PACKAGES_ITERATION` value will be `1`.
- When a tag is created from the stable branch the `PACKAGES_ITERATION` value will be `1`.
- When we need to release a new package
  - The tag will be deleted
  - The `PACKAGES_ITERATION` value will be incremented in the stable branch
  - The tag will be recreated
  - If we need to merge the stable branch back to main the `PACKAGES_ITERATION` value will be set to `1` beforehand, otherwise the `package` jobs will fail because the iteration checks will fail

## Future iterations

We could make the iteration check automatic by checking the package cloud packages, but technically we need to check all of them to make sure we know
the highest iteration, so we can increment off of that. This sounds time-consuming in terms of CI time. For now we'll stick to the manual approach.
