---
type: user guide
level: intermediate
date: 2020-04-22
---

# Autoscaling GitLab CI on AWS Fargate

GitLab's [AWS Fargate driver](https://gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate)
automatically launches a container on Amazon's Elastic Container Service (ECS) to
execute each GitLab CI job. The goal of this document is to cover the technical tasks
to configure and run GitLab Runner and the AWS Fargate driver with the
[Runner Custom executor](../../executors/custom.md).

## Overview

With this configuration in place, once there is a commit in a project configured
to use the Runner Manager for AWS ECS, the GitLab instance notifies the Runner Manager
that a new job is available. The Runner-Manager then starts a new `task` in the target
ECS cluster based on a task definition that you have previously configured in AWS ECS.
You can configure an AWS ECS task definition to use any Docker image, so you have
complete flexibility in regards to the type of builds that you can execute on AWS Fargate.

At this time, tasks that use the Fargate launch type [do not support](https://docs.aws.amazon.com/AmazonECS/latest/userguide/fargate-task-defs.html)
the `privileged` task definition parameter. To enable **Docker-in-Docker** builds
on AWS Fargate, we are working on a merge request, [#34](https://gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/-/merge_requests/34),
to implement a pattern for configuring the Fargate driver to use Kaniko.

**Note:**

- The configuration in this document is a basic configuration from an AWS security
  perspective. It is meant for the user to get an initial understanding of the Fargate
  driver implementation.
- For production use, implement AWS security best practices related to the setup
  and configuration of an AWS VPC, subnet, IAM roles, and security groups. For example,
  you can have a Security Group that is to be used by the EC2 instance, which hosts
  GitLab Runner and only accepts SSH connections from a restricted external IP range
  (your GitLab instance, for example). A second Security Group for the Fargate Tasks
  and that allows SSH traffic only from the EC2 instance.
- CloudFormation or Terraform can be used to automate the provisioning and setup of
  the AWS infrastructure.

![GitLab Runner Fargate Driver Architecture](../img/runner_fargate_driver_ssh.png)

## Prerequisites

- AWS IAM user with permissions to create and configure EC2, ECS and ECR resources.
- AWS VPC and subnets
- AWS Security Group

## Step 1: Prepare a base container image for the AWS Fargate task

- The container image that you create must include the tools that are required to
  build your CI job. For example, a Java project requires a `Java JDK`, and build
  tools such as Maven or Gradle. A Node.js project requires `node` and `npm`.
- Include GitLab Runner in the container image as the Runner is required to handle
  artifacts and caching. Refer to the [run](../../executors/custom.md#run)
  stage section of custom_executor docs for additional information.
- The container image needs to be able to accept an SSH connection through public-key
  authentication. This SSH connection is used by the Runner manager to send the build
  commands defined in the `gitlab-ci.yml` file to the container running on AWS Fargate.
  The SSH keys will be automatically managed by the Fargate driver,
  but the container must be instrumented to receive the public key through the
  `SSH_PUBLIC_KEY` environment variable.
- Note that if you specify an image for a job using the `image:` keyword, it will be ignored
  and the image specified in the task definition will be used.

A Debian example, that includes GitLab Runner and the SSH configuration can be found
in this [repository](https://gitlab.com/tmaczukin-test-projects/fargate-driver-debian).
A Node.js example is in this [repository](https://gitlab.com/aws-fargate-driver-demo/docker-nodejs-gitlab-ci-fargate).

## Step 2: Push the container image to GitLab Container Registry or Amazon Elastic Container Registry (ECR)

Once you have created your build image, publish the image to a container registry for
use in the ECS task definition. Detailed instructions for creating a repository and
pushing an image to ECR is in the [Amazon ECR Repositories](https://docs.aws.amazon.com/AmazonECR/latest/userguide/Repositories.html)
documentation.

If you are using the AWS CLI, then you can refer to the [Getting Started with Amazon ECR using the AWS CLI](https://docs.aws.amazon.com/AmazonECR/latest/userguide/getting-started-cli.html)
documentation for the steps required to push an image to ECR using the AWS CLI.

In the Debian example, we use the [GitLab Container Registry](https://docs.gitlab.com/ee/user/packages/container_registry/)
and the image is published to `registry.gitlab.com/tmaczukin-test-projects/fargate-driver-debian:latest`.
Similarly, the NodeJS example image is published to `registry.gitlab.com/aws-fargate-driver-demo/docker-nodejs-gitlab-ci-fargate:latest`.

## Step 3: Retrieve your project's Runner registration token

Go to `https://gitlab.com/your/project/-/settings/ci_cd`, open the Runners section
and check the registration token under `Set up a specific Runner manually`. We need
the Runner registration token and GitLab URL in one of the next steps, (leave the
page opened in a browser tab for now).

At [https://gitlab.com/tmaczukin-test-projects/test-fargate](https://gitlab.com/tmaczukin-test-projects/test-fargate)
you can find a project with a very simple CI job for testing the driver:

```yaml
test:
  script:
    - echo "It works!"
```

## Step 4: Create a Runner Manager EC2 instance

1. Go to [https://console.aws.amazon.com/ec2/v2/home#LaunchInstanceWizard](https://console.aws.amazon.com/ec2/v2/home#LaunchInstanceWizard).
1. Select the Ubuntu Server 18.04 LTS AMI for the instance, the 64-bit (x86) version.
   The name can be different depending on the AWS region selected.
1. Choose t2.micro instance size. Click `Next: Configure Instance Details`.
1. Leave the number of instances as 1. We can leave the default chosen network and
   subnet. Let's also set the `Auto-assign Public IP` to **enabled**. We also need
   to create the IAM role that this instance will use:
   1. Click the `Create new IAM role` and a new window/tab will be opened:
   1. Click `Create role` button.
   1. Choose `AWS service` type for the entity.
   1. Choose `Common use cases` -> `EC2` and click `Next: Permissions` button.
   1. Choose the `AmazonECS_FullAccess` policy. We probably could limit the policy
      a little, but for an initial configuration, this will ensure that everything
      works as expected. Click `Next: Tags`.
   1. Click `Next: Review`.
   1. In the Review Screen fill in the name of the newly created IAM role, for example `fargate-test-instance`,
      and click `Create role` to continue.
1. Go back to the window/tab where the instance is being created.
1. Click the refresh button near the `IAM role` select input. After refreshing,
   choose the `fargate-test-instance` role. Click `Next: Add Storage`.
1. Click `Next: Add Tags`.
1. Click `Next: Configure Security Group`.
1. Select the `Create a new security group`, give it a name `fargate-test`, and
   ensure that the rule for SSH is defined (`Type: SSH, Protocol: TCP, Port Range: 22`).
   Note: You will need to specify the IP ranges for inbound and outbound rules.
1. Click `Review and Launch`.
1. Click `Launch`.
1. (optional) Select `Create a new key pair`, give it a name `fargate-runner-manager`
   and hit the `Download Key Pair` button. The private key for SSH will be downloaded
   on your computer (check the directory configured in your browser).
1. Click `Launch Instances`.
1. Click `View instances`.
1. Wait for the instance to be up. Note the `IPv4 Public IP` address.

## Step 5: Install and configure GitLab Runner with the AWS Fargate Custom Executor driver

Assign the right permissions for the downloaded key via `chmod 400 path/to/downloaded/key/file`
and SSH into the EC2 instance that you created in the previous step via
`ssh ubuntu@[ip_address] -i path/to/downloaded/key/file`. After that, run the following commands on this EC2 instance:

1. `sudo mkdir -p /opt/gitlab-runner/{metadata,builds,cache}`
1. `curl -s https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh | sudo bash`
1. `sudo apt install gitlab-runner`
1. `sudo gitlab-runner register --url https://gitlab.com/ --registration-token TOKEN_HERE --name fargate-test-runner --run-untagged --executor custom -n`.
   Use the GitLab URL and registration token taken from the project settings page
   opened in step 3 above.
1. Run `sudo vim /etc/gitlab-runner/config.toml` and add the following content:

   ```toml
   concurrent = 1
   check_interval = 0

   [session_server]
     session_timeout = 1800

   [[runners]]
     name = "fargate-test"
     url = "https://gitlab.com/"
     token = "__REDACTED__"
     executor = "custom"
     builds_dir = "/opt/gitlab-runner/builds"
     cache_dir = "/opt/gitlab-runner/cache"
     [runners.custom]
       config_exec = "/opt/gitlab-runner/fargate"
       config_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "config"]
       prepare_exec = "/opt/gitlab-runner/fargate"
       prepare_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "prepare"]
       run_exec = "/opt/gitlab-runner/fargate"
       run_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "run"]
       cleanup_exec = "/opt/gitlab-runner/fargate"
       cleanup_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "cleanup"]
   ```

   Note: The section of the `config.toml` file shown below will be created by the registration command, do not change it.
   The other sections in the `config.toml` file above is what you need to add.

   ```toml
   concurrent = 1
   check_interval = 0

   [session_server]
     session_timeout = 1800

   name = "fargate-test"
   url = "https://gitlab.com/"
   token = "__REDACTED__"
   executor = "custom"
   ```

1. Run `sudo vim /etc/gitlab-runner/fargate.toml` and add the following content:

   ```toml
   LogLevel = "info"
   LogFormat = "text"

   [Fargate]
     Cluster = "test-cluster"
     Region = "us-east-2"
     Subnet = "subnet-xxxxxx"
     SecurityGroup = "sg-xxxxxxxxxxxxx"
     TaskDefinition = "test-task:1"
     EnablePublicIP = true

   [TaskMetadata]
     Directory = "/opt/gitlab-runner/metadata"

   [SSH]
     Username = "root"
     Port = 22
   ```

   - Remember the value for `Cluster`, as well as the name of the `TaskDefinition` (for example `test-task`
     with `:1` as the revision number). If a revision is not specified, the latest **active** revision is used.
   - Choose your region. Take the `Subnet` value from the Runner Manager instance
   - Get the SecurityGroup ID from its details:

     1. Find `Security groups` on the Runner Manager instance details page
     1. Click the security group you created earlier
     1. Copy the `Security group ID`

     In a production setting,
     you should follow [AWS guidelines](https://docs.aws.amazon.com/vpc/latest/userguide/VPC_SecurityGroups.html)
     for setting up and using Security groups.

   - The port number of the SSH server is optional. If omitted, will use the default SSH port (22).

1. Install the Fargate driver:

   ```shell
   sudo curl -Lo /opt/gitlab-runner/fargate https://gitlab-runner-custom-fargate-downloads.s3.amazonaws.com/master/fargate-linux-amd64
   sudo chmod +x /opt/gitlab-runner/fargate
   ```

## Step 6. Create an ECS Fargate cluster

An Amazon ECS cluster is a grouping of ECS Container Instances.

1. Go to [`https://console.aws.amazon.com/ecs/home#/clusters`](https://console.aws.amazon.com/ecs/home#/clusters).
1. Click `Create Cluster`.
1. Choose `Network only` type. Click `Next step`.
1. Give it the name `test-fargate` (the same as in `fargate.toml`). We don't
   need to specify anything else here.
1. Click `Create`.
1. Click `View cluster`. Note the region and account id parts from the `Cluster ARN` value.
1. Click `Update Cluster` button.
1. Click `Add another provider` next to `Default capacity provider strategy` and choose `FARGATE`. Click `Update`.

Refer to the AWS [documentation](https://docs.aws.amazon.com/AmazonECS/latest/userguide/create_cluster.html)
for detailed instructions on setting up and working with a cluster on ECS Fargate.

## Step 7: Create an ECS Task Definition

In this step you will create a task definition of type `Fargate` with a reference
to the container image that you are going to use for your CI builds.

1. Go to [`https://console.aws.amazon.com/ecs/home#/taskDefinitions`](https://console.aws.amazon.com/ecs/home#/taskDefinitions).
1. Click `Create new Task Definition`.
1. Choose `Fargate` and click `Next step`.
1. Give it a name `test-task` (Note: the name is the same as the value defined in
   the `fargate.toml` file but without `:1`).
1. Select values for `Task memory (GB)` and `Task CPU (vCPU)`.
1. Click `Add Container`, then:
   1. Give it the `ci-coordinator` name, so the Fargate driver
      can inject the `SSH_PUBLIC_KEY` environment variable.
   1. Define image (for example `registry.gitlab.com/tmaczukin-test-projects/fargate-driver-debian:latest`).
   1. Define port mapping for 22/TCP.
   1. Click `Add`.
1. Click `Create`.
1. Click `View task definition`.

CAUTION: **Caution:**
A single Fargate task may launch one or more containers.
The Fargate driver injects the `SSH_PUBLIC_KEY` environment variable
in containers with the `ci-coordinator` name only, so you must
have a container with this name in all task definitions used by the Fargate
driver.

Refer to the AWS [documentation](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/create-task-definition.html)
for detailed instructions on setting up and working with task definitions.

At this point the GitLab Runner Manager and Fargate Driver are configured and ready
to start executing jobs on AWS Fargate.

## Step 8: Testing the configuration

Your configuration should now be ready to use.

1. Go to your test project -> CI/CD -> Pipelines.
1. Click `Run Pipeline`.
1. Schedule a new pipeline for master branch.

## Cleanup

If you want to perform a cleanup after testing the custom executor with AWS Fargate, you should remove the following objects:

- EC2 instance, key pair, IAM role and security group created at [step 4](#step-4-create-a-runner-manager-ec2-instance)
- ECS Fargate cluster created at [step 6](#step-6-create-an-ecs-fargate-cluster)
- ECS Task Definition created at [step 7](#step-7-create-an-ecs-task-definition)

## Troubleshooting

### `Application execution failed` error when testing the configuration

`error="starting new Fargate task: running new task on Fargate: error starting AWS Fargate Task: InvalidParameterException: No Container Instances were found in your cluster."`

The AWS Fargate Driver requires the ECS Cluster to be configured with a [default capacity provider strategy](#step-6-create-an-ecs-fargate-cluster).

Further reading:

- A default [capacity provider strategy](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/cluster-capacity-providers.html) is associated with each Amazon ECS cluster. If no other capacity provider strategy or launch type is specified, the cluster uses this strategy when a task runs or a service is created.
- If a [`capacityProviderStrategy`](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_RunTask.html#ECS-RunTask-request-capacityProviderStrategy) is specified, the `launchType` parameter must be omitted. If no `capacityProviderStrategy` or `launchType` is specified, the `defaultCapacityProviderStrategy` for the cluster is used.
