# What does this MR do?

This merge request contains human translations of GitLab product documentation. The source files are
from the `/docs` directory, and translations are returned to language-specific directories under `/docs-locale`.

Translations in this MR were produced and reviewed by professional linguists through our translation vendor workflow.

## Translation MR information

- Argo Request: [{{argo_request_key}}: {{argo_request_name}}]({{argo_request_url}})
- Source: {{source_content_origin}}

## Review workflow

For the full review workflow documentation, see the [Translation MR Review Workflow](https://gitlab.com/gitlab-com/localization/docs-site-localization/-/blob/main/translation_mr_review_workflow.md).

### Assignee checklist

- [ ] Fix conflicts (check commit history of each file in `main` to identify target changes causing conflicts, such as translation changes on production or TW shortcode/linting updates)
- [ ] Fix any pipeline issues
- [ ] Rebase if needed
- [ ] Deploy a review app and check all impacted pages. Duo can help produce a list of URLs, which you can paste in the Review App section below.
- [ ] Remove the MR from Draft mode  (this triggers the first review by GitLab Duo)
- [ ] If any linguistic questions or issues come up during review (for example, terminology, formatting, or content that looks mistranslated), ping and add **one** language content maintainer as a reviewer.
   - Reviewers can be seen here: [French](https://gitlab.com/groups/gitlab-com/localization/maintainers/french/-/group_members?with_inherited_permissions=exclude&sort=last_joined), [Japanese](https://gitlab.com/groups/gitlab-com/localization/maintainers/japanese/-/group_members?with_inherited_permissions=exclude&sort=last_joined), [Korean](https://gitlab.com/groups/gitlab-com/localization/maintainers/korean/-/group_members?with_inherited_permissions=exclude&sort=last_joined)
- [ ] Hand off for review to a [tech docs maintainer](https://gitlab.com/groups/gitlab-com/localization/maintainers/tech-docs/-/group_members?with_inherited_permissions=exclude&sort=last_joined). The MR should be ready to merge at this point

### Review App

| Review app |
| ---------- |
|  |

### Reviewer checklist

- [ ] Review changes
- [ ] Verify build pipeline
- [ ] Merge on approval

/title Product Docs Human Translation: {{argo_request_key}} #{{translation_mr_number}}
/draft

/assign @gitlab-argo-bot

/label ~documentation
/label ~"gitlab-translation-service"
/label ~"group::localization"
/label ~"docs-only"
/label ~"type::maintenance"
