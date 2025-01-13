---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# GitLab Runner Infrastructure Toolkit

DETAILS:
**Tier:** Free, Premium, Ultimate
**Offering:** GitLab.com, Self-managed

The [GitLab Runner Infrastructure Toolkit (GRIT)](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit) is a library of Terraform modules you can use to create and manage many common runner configurations on public cloud providers.

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
terrafrom destroy
```

## Supported configurations

| Provider     | Service | Arch | OS    | Executors         | Feature Support |
| ------------ | ------- | ---- | ----- | ----------------- | --------------- |
| AWS          | EC2     | x64  | Linux | Docker Autoscaler | Experimental    |
| AWS          | EC2     | ARM  | Linux | Docker Autoscaler | Experimental    |
| Google Cloud | GCE     | x64  | Linux | Docker Autoscaler | Experimental    |
| Google Cloud | GKE     | x64  | Linux | Kubernetes        | Experimental    |
