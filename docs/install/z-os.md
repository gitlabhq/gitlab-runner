---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Install GitLab Runner manually on z/OS.
title: Install GitLab Runner manually on z/OS
---

{{< details >}}

- Tier: Free, Premium, Ultimate
- Offering: GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner for IBM z/OS has been certified by GitLab and can run CI/CD jobs natively on z/OS mainframe environments.

You can download and install GitLab Runner on z/OS manually from a [`pax`](https://www.ibm.com/docs/en/aix/7.1.0?topic=p-pax-command) archive.

## Prerequisites

- To use GitLab Runner, you need the following authorized program analysis reports (`APARs`) with program temporary fixes (`PTFs`):

  - z/OS 2.5
    - OA62757
    - PH45182
  - z/OS 3.1
    - OA62757
    - PH57159

- GitLab Runner expects bash to be installed at `/bin/bash` to execute shell commands.
  If bash is not installed at this location, create a symlink to the installed version:

  ```shell
  ln -s <TARGET_BASH> /bin/bash
  ```

## Install GitLab Runner

To install GitLab Runner:

1. Download the `paxfile` into your chosen install directory.

1. Install the package for your system:

   ```shell
   pax -ppx -rf gitlab-runner-<VERSION>.pax.Z
   ```

   The installed files are unpacked to the `gitlab-runner` directory in the install location.

1. Give the file permissions to execute:

   ```shell
   chmod +x <INSTALL_PATH>/bin/gitlab-runner
   ```

1. Export GitLab Runner and add it to your `PATH`:

   ```shell
   export GITLAB_RUNNER=<INSTALL_PATH>/gitlab-runner/bin
   export PATH=${GITLAB_RUNNER}:${PATH}
   ```

1. [Register a runner](../register/_index.md).

## Run GitLab Runner

You can run GitLab Runner directly or as a started task.

### Run GitLab Runner directly

To run GitLab Runner by calling the executable:

1. Go to the directory `<INSTALL_PATH>/bin`.

1. Start the service:

   ```shell
   gitlab-runner start
   ```

### Run GitLab Runner as a started task

To keep the GitLab Runner process available, run it as a started task.

1. Wrap the executable in a shell script `gitlab-runner.sh`:

   ```shell
   #! /bin/sh
   <INSTALL_PATH>/bin/gitlab-runner start
   ```

1. Define a `jcl` started task program and execute it to run as an ongoing process:

   ```jcl
   //GLRST  PROC CNFG='<PATH_TO_SCRIPT>'
   //*
   //GLRST  EXEC PGM=BPXBATSL,REGION=0M,TIME=NOLIMIT,
   //            PARM='PGM &CNFG./gitlab-runner.sh'
   //STDOUT   DD SYSOUT=*
   //STDERR   DD SYSOUT=*
   //*
   //        PEND
   ```
