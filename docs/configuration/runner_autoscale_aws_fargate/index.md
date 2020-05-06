> **[Article Type](https://docs.gitlab.com/ee/development/writing_documentation.html#types-of-technical-articles):** Admin guide ||
> **Level:** intermediary ||
> **Publication date:** 2020-04-22

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
and the image is published to `registry.gitlab.com/tmaczukin-test-projects/fargate:latest`.
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
1. Choose t2.micro instance size. `Click Next: Configure Instance Details`.
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
   choose the `fargate-test-instance` role. `Click Next: Add Storage`.
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

1. SSH into the EC2 instance that you created in the previous step,
   `ssh ubuntu@[ip_address] -i path/to/downloaded/key/file`. Note: you need to make
   sure that the key file for accessing the EC2 instance has the right permissions.
   `chmod 400 path/to/downloaded/key/file`.
1. `sudo mkdir -p /opt/gitlab-runner/{metadata,builds,cache}`
1. `curl -s https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh | sudo bash`
1. `sudo apt install gitlab-runner`
1. `sudo gitlab-runner register --url https://gitlab.com/ --registration-token TOKEN_HERE --name fargate-test-runner --run-untagged --executor custom -n`.
   Use the GitLab URL and registration token taken from the project settings page
   opened in step 3 above.
1. `sudo vim /etc/gitlab-runner/config.toml` and add the following content:

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

   Note: This section in the `config.toml` will be created by the registration command.
   The other content sections in the `config.toml` file above is what you will need to add.

   ```toml
   name = "fargate-test"
   url = "https://gitlab.com/"
   token = "__REDACTED__"
   executor = "custom"
   ```

1. `sudo vim /etc/gitlab-runner/fargate.toml` and add the following content:

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
     PrivateKeyPath = "/root/.ssh/id_rsa"
   ```

   - Remember the value for `Cluster` - we will use it later. As well as the `test-task`,
     the name of the `TaskDefinition` (`:1` is the revision number).
   - Choose your region. Take the `Subnet` value from the Runner Manager instance
     details. Get the SecurityGroup ID from its details. Note - in a production setting,
     you should follow [AWS guidelines](https://docs.aws.amazon.com/vpc/latest/userguide/VPC_SecurityGroups.html)
     for setting up and using Security groups.

1. Create the SSH private key: `sudo ssh-keygen -t rsa -b 2048 -f /root/.ssh/id_rsa -N ""`
1. Install the Fargate driver:

   ```shell
   sudo curl -Lo /opt/gitlab-runner/fargate https://gitlab-runner-custom-fargate-downloads.s3.amazonaws.com/master/fargate-linux-amd64
   sudo chmod +x /opt/gitlab-runner/fargate
   ```

## Step 6: Securely store the SSH public key for connecting to the CI build container

In the following steps we use the AWS System Manager Parameter Store for storing
the SSH key. If you have not previously used AWS Systems Manager, then refer to the
[Setting Up AWS Systems Manager](https://docs.aws.amazon.com/systems-manager/latest/userguide/systems-manager-setting-up.html)
tutorial to get started.

To store the SSH public key in the AWS System Manager Parameter Store complete the
following steps:

1. Go to [https://console.aws.amazon.com/systems-manager/parameters/](https://console.aws.amazon.com/systems-manager/parameters/).
1. Click `Create parameter`.
1. For `Name` type `SSH_PUBLIC_KEY`.
1. Tier = `Standard`.
1. Type = `String`.
1. Copy the value from the `id_rsa_pub` file on the Runner Manager instance, from `/root/.ssh/id_rsa.pub`.
1. In the `value` field, paste the value from the `id_rsa_pub` file.

## Step 7. Create an ECS Fargate cluster

An Amazon ECS cluster is a grouping of ECS Container Instances.

1. Go to [http://console.aws.amazon.com/ecs/home#/clusters](http://console.aws.amazon.com/ecs/home#/clusters).
1. Click `create cluster`.
1. Choose `Network only` type. Click `Next`.
1. Give it the name `test-fargate` (the same as in `fargate.toml`). We don't
   need to specify anything else here.
1. Click Create.
1. Click `View cluster`. Note the region and account id parts from the `Cluster ARN` value.
1. Click `Update Cluster` button.
1. Click `Define capacity provider` and chose `FARGATE`. Click `Update`.

Refer to the AWS [documentation](https://docs.aws.amazon.com/AmazonECS/latest/userguide/create_cluster.html)
for detailed instructions on setting up and working with a cluster on ECS Fargate.

## Step 8: Create an ECS Task Definition

In this step you will create a task definition of type `Fargate` with a reference
to the container image that you are going to use for your CI builds.

1. Go to [http://console.aws.amazon.com/ecs/home#/taskDefinitions](http://console.aws.amazon.com/ecs/home#/taskDefinitions)
1. Click `Create new task definition`.
1. Choose Fargate.
1. Give it a name `test-task` (Note: the name is the same as the value defined in
   the `fargate.toml` file).
1. Select values for `Task memory (GB)` and `Task CPU (vCPU)`
1. Click `Add Container`, then:
   1. Give it a name (for example `job-container`)
   1. Define image (for example `registry.gitlab.com/tmaczukin-test-projects/fargate-driver-debian:latest`).
   1. Define port mapping for 22/TCP.
   1. Define a new environment variable `SSH_PUBLIC_KEY`, set it as `ValueFrom` and
      use `arn:aws:ssm:<region>:<account-id>:parameter/SSH_PUBLIC_KEY` as the value.
      Use the region and account-id noted previously.
   1. Click `Add`
1. Click `Create`.
1. Click `View task definitions`.

Refer to the AWS [documentation](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/create-task-definition.html)
for detailed instructions on setting up and working with task definitions.

## Step 9: Update permissions of `ecsTaskExecutionRole` role

1. Go to [https://console.aws.amazon.com/iam/home#/roles](https://console.aws.amazon.com/iam/home#/roles)
1. Choose the ecsTaskExecutionRole
1. Click Attach policies
1. Choose AmazonSSMReadOnlyAccess
1. Click Attach policy

At this point the GitLab Runner Manager and Fargate Driver are configured and ready
to start executing jobs on AWS Fargate.

### Step 10: Testing the configuration

Your configuration should now be ready to use.

1. Go to your test project -> CI/CD -> Pipelines
1. Click New
1. Schedule a new pipeline for master branch.
