---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Runner Infrastructure Toolkit
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated
- Status: Experiment

{{< /details >}}

The [GitLab Runner Infrastructure Toolkit (GRIT)](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit) is a library of Terraform modules you can use to create and manage many common runner configurations on public cloud providers.

{{< alert type="note" >}}

This feature is an [experiment](https://docs.gitlab.com/policy/development_stages_support/#experiment). For more information about the state of GRIT development, see [epic 1](https://gitlab.com/groups/gitlab-org/ci-cd/runner-tools/-/epics/1). To provide feedback on this feature, leave a comment on [issue 84](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/issues/84).

{{< /alert >}}

## Create a runner with GRIT

To use GRIT to deploy an autoscaling Linux Docker in AWS:

1. Set the following variables to provide access to GitLab and AWS:

   - `GITLAB_TOKEN`
   - `AWS_REGION`
   - `AWS_SECRET_ACCESS_KEY`
   - `AWS_ACCESS_KEY_ID`

1. Download the latest [GRIT release](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/releases) and extract to `.local/grit`.
1. Create a `main.tf` Terraform module:

   ```hcl
   module "runner" {
     source = ".local/grit/scenarios/aws/linux/docker-autoscaler-default"

     name               = "grit-runner"
     gitlab_project_id  = "39258790" # gitlab.com/josephburnett/hello-runner
     runner_description = "Autoscaling Linux Docker runner on AWS deployed with GRIT. "
     runner_tags        = ["aws", "linux"]
     max_instances      = 5
     min_support        = "experimental"
   }
   ```

1. Initialize and apply the module:

   ```plaintext
   terraform init
   terraform apply
   ```

These steps create a new runner in a GitLab project. The runner manager uses the `docker-autoscaler`
executor to run jobs tagged as `aws` and `linux`. The runner provisions between 1 and 5 VMs through
a new Autoscaling Group (ASG), based on workload. The ASG uses a public AMI owned by the runner team.
Both the runner manager and the ASG operate in a new VPC. All resources are named based on the provided
value (`grit-runner`), which lets you create multiple instances of this module with different names in
a single AWS project.

## Support levels and the `min_support` parameter

You must provide a `min_support` value for all GRIT modules.
This parameter specifies the minimum support level that the operator
requires for their deployment. GRIT modules are associated with a support
designation of `none`, `experimental`, `beta`, or `GA`. The goal is
for all modules to reach the `GA` status.

`none` is a special case. Modules with no support guarantees, primarily for testing and development.

`experimental`, `beta`, and `ga` modules conform to the [GitLab definitions of development stages](https://docs.gitlab.com/policy/development_stages_support/).

### Shared responsibility model

GRIT operates under a shared responsibility model between Authors (module developers) and Operators (those deploying
with GRIT). For details on the specific responsibilities of each role and how support levels are determined, see
the [Shared responsibility section](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/GORP.md#shared-responsibility)
in the GORP documentation.

## Manage runner state

To maintain runners:

1. Check the module into a GitLab project.
1. Store the Terraform state in the GitLab Terraform `backend.tf`:

   ```hcl
   terraform {
     backend "http" {}
   }
   ```

1. Apply the changes by using `.gitlab-ci.yml`:

   ```yaml
   terraform-apply:
     variables:
       TF_HTTP_LOCK_ADDRESS: "https://gitlab.com/api/v4/projects/${CI_PROJECT_ID}/terraform/state/${NAME}/lock"
       TF_HTTP_UNLOCK_ADDRESS: ${TF_HTTP_LOCK_ADDRESS}
       TF_HTTP_USERNAME: ${GITLAB_USER_LOGIN}
       TF_HTTP_PASSWORD: ${GITLAB_TOKEN}
       TF_HTTP_LOCK_METHOD: POST
       TF_HTTP_UNLOCK_METHOD: DELETE
     script:
       - terraform init
       - terraform apply -auto-approve
   ```

### Delete a runner

To remove the runner and its infrastructure:

```plaintext
terraform destroy
```

## Supported configurations

| Provider     | Service | Arch   | OS    | Executors         | Feature Support |
|--------------|---------|--------|-------|-------------------|-----------------|
| AWS          | EC2     | x86-64 | Linux | Docker Autoscaler | Experimental    |
| AWS          | EC2     | Arm64  | Linux | Docker Autoscaler | Experimental    |
| Google Cloud | GCE     | x86-64 | Linux | Docker Autoscaler | Experimental    |
| Google Cloud | GKE     | x86-64 | Linux | Kubernetes        | Experimental    |

## Advanced Configuration

### Top-Level Modules

Top-level modules in a provider represent highly-decoupled or
optional configuration aspects of runner. For example, `fleeting` and
`runner` are separate modules because they share only access credentials
and instance group names. The `vpc` is a separate module because some users
provide their own VPC. Users with existing VPCs need only create a matching
input structure to connect with other GRIT modules.

For example, the top-level VPC module can be used to create a VPC for modules that require a VPC:

   ```hcl
   module "runner" {
      source = ".local/grit/modules/aws/runner"

      vpc = {
         id         = module.vpc.id
         subnet_ids = module.vpc.subnet_ids
      }

      # ...additional config omitted
   }

   module "vpc" {
      source   = ".local/grit/modules/aws/vpc"

      zone = "us-east-1b"

      cidr        = "10.0.0.0/16"
      subnet_cidr = "10.0.0.0/24"
   }
   ```

User can provide their own VPC and not use GRIT's VPC module:

   ```hcl
   module "runner" {
      source = ".local/grit/modules/aws/runner"

      vpc = {
         id         = PREEXISTING_VPC_ID
         subnet_ids = [PREEXISTING_SUBNET_ID]
      }

      # ...additional config omitted
   }
   ```

## Contributing to GRIT

GRIT welcomes community contributions. Before contributing, review the following resources:

### Developer Certificate of Origin and license

All contributions to GRIT are subject to the [Developer Certificate of Origin and license](https://docs.gitlab.com/legal/developer_certificate_of_origin/). By contributing, you accept and agree to these terms and conditions for your present and future contributions submitted to GitLab Inc.

### Code of Conduct

GRIT follows the GitLab Code of Conduct, which is adapted from the [Contributor Covenant](https://www.contributor-covenant.org). The project is committed to making participation a harassment-free experience for everyone, regardless of background or identity.

### Contribution guidelines

When contributing to GRIT, follow these guidelines:

- Review the [GORP Guidelines](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/GORP.md) for overall architectural design.
- Adhere to [Google's best practices for using Terraform](https://docs.cloud.google.com/docs/terraform/best-practices/general-style-structure).
- Follow the composable module approach to reduce complexity and repetition.
- Include appropriate Go tests for your contributions.

### Testing and linting

GRIT uses several testing and linting tools to ensure quality:

- Integration tests: Uses [Terratest](https://terratest.gruntwork.io/) to validate Terraform plans.
- End-to-end tests: Available in the [e2e directory](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/e2e/README.md).
- Terraform linting: Uses `tflint`, `terraform fmt`, and `terraform validate`.
- Go linting: Uses [golangci-lint](https://golangci-lint.run/) for Go code (primarily tests).
- Documentation: Follows the [GitLab documentation style guide](https://docs.gitlab.com/development/documentation/styleguide/) and uses `vale` and `markdownlint`.

For detailed instructions on setting up your development environment, running tests, and linting, see [CONTRIBUTING.md](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/CONTRIBUTING.md).

## Who uses GRIT?

GRIT has been adopted by various teams and services within the GitLab ecosystem:

- **[GitLab Dedicated](https://about.gitlab.com/dedicated/)**: [Hosted runners for GitLab Dedicated](https://docs.gitlab.com/administration/dedicated/hosted_runners/) uses GRIT to provision and manage runner infrastructure.

- **GitLab Self-Managed**: GRIT is highly requested among many GitLab Self-Managed customers. Some organizations have started to adopt GRIT to manage their runner deployments in a standardized way.

If you're using GRIT in your organization and would like to be featured in this section, open a merge request!
