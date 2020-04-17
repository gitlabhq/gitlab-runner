<!--
We use Merge Request titles to generate the CHANGELOG.md entries. Please make the title
clear and informative. Please style it so that it works as a CHANGELOG entry by itself.
-->

## What does this MR do?

<!--
See the general guidelines: http://docs.gitlab.com/ce/development/doc_styleguide.html
-->

## Moving docs to a new location?

<!--
See the guidelines: http://docs.gitlab.com/ce/development/doc_styleguide.html#changing-document-location
-->

- [ ] Make sure the old link is not removed and has its contents replaced with a link to the new location.
- [ ] Make sure internal links pointing to the document in question are not broken.
- [ ] Search and replace any links referring to old docs in GitLab Runner's source code

/label ~documentation ~"devops::verify" ~"group::runner" ~"Category:Runner"
