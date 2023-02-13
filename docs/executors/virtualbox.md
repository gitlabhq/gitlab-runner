---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/product/ux/technical-writing/#assignments
---

# VirtualBox **(FREE)**

NOTE:
The Parallels executor works the same as the VirtualBox executor. The
caching feature is currently not supported.

VirtualBox allows you to use VirtualBox's virtualization to provide a clean
build environment for every build. This executor supports all systems that can
be run on VirtualBox. The only requirement is that the virtual machine exposes
an SSH server and provides a shell compatible with Bash or PowerShell.

NOTE:
Ensure you meet [common prerequisites](index.md#prerequisites-for-non-docker-executors)
on any virtual machine where GitLab Runner uses the VirtualBox executor.

## Overview

The project's source code is checked out to: `~/builds/<namespace>/<project-name>`.

Where:

- `<namespace>` is the namespace where the project is stored on GitLab
- `<project-name>` is the name of the project as it is stored on GitLab

To override the `~/builds` directory, specify the `builds_dir` option under
the `[[runners]]` section in
[`config.toml`](../configuration/advanced-configuration.md).

You can also define
[custom build directories](https://docs.gitlab.com/ee/ci/yaml/index.html#custom-build-directories)
per job using the `GIT_CLONE_PATH`.

## Create a new base virtual machine

1. Install [VirtualBox](https://www.virtualbox.org).
   - If running from Windows and VirtualBox is installed at the
     default location (for example `%PROGRAMFILES%\Oracle\VirtualBox`),
     GitLab Runner will automatically detect it.
     Otherwise, you will need to add the installation folder to the `PATH` environment variable of the `gitlab-runner` process.
1. Import or create a new virtual machine in VirtualBox
1. Configure Network Adapter 1 as "NAT" (that's currently the only way the GitLab Runner is able to connect over SSH into the guest)
1. (optional) Configure another Network Adapter as "Bridged networking" to get access to the internet from the guest (for example)
1. Log into the new virtual machine
1. If Windows VM, see [Checklist for Windows VMs](#checklist-for-windows-vms)
1. Install the OpenSSH server
1. Install all other dependencies required by your build
1. If you want to download or upload job artifacts, install `gitlab-runner` inside the VM
1. Log out and shut down the virtual machine

It's completely fine to use automation tools like Vagrant to provision the
virtual machine.

## Create a new runner

1. Install GitLab Runner on the host running VirtualBox
1. Register a new runner with `gitlab-runner register`
1. Select the `virtualbox` executor
1. Enter the name of the base virtual machine you created earlier (find it under
   the settings of the virtual machine **General > Basic > Name**)
1. Enter the SSH `user` and `password` or path to `identity_file` of the
   virtual machine

## How it works

When a new build is started:

1. A unique name for the virtual machine is generated: `runner-<short-token>-concurrent-<id>`
1. The virtual machine is cloned if it doesn't exist
1. The port-forwarding rules are created to access the SSH server
1. GitLab Runner starts or restores the snapshot of the virtual machine
1. GitLab Runner waits for the SSH server to become accessible
1. GitLab Runner creates a snapshot of the running virtual machine (this is done
   to speed up any next builds)
1. GitLab Runner connects to the virtual machine and executes a build
1. If enabled, artifacts upload is done using the `gitlab-runner` binary *inside* the virtual machine.
1. GitLab Runner stops or shuts down the virtual machine

## Checklist for Windows VMs

To use VirtualBox with Windows, you can install Cygwin or PowerShell.

### Use Cygwin

- Install [Cygwin](https://cygwin.com/)
- Install `sshd` and Git from Cygwin (do not use *Git for Windows*, you will get lots of path issues!)
- Install Git LFS
- Configure `sshd` and set it up as a service (see [Cygwin wiki](https://cygwin.fandom.com/wiki/Sshd))
- Create a rule for the Windows Firewall to allow incoming TCP traffic on port 22
- Add the GitLab server(s) to `~/.ssh/known_hosts`
- To convert paths between Cygwin and Windows, use [the `cygpath` utility](https://cygwin.fandom.com/wiki/Cygpath_utility)

### Use native OpenSSH and PowerShell

> [Introduced in](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3176) GitLab Runner 14.6.

- Install [PowerShell](https://learn.microsoft.com/en-us/powershell/scripting/install/installing-powershell-on-windows?view=powershell-7.2)
- Install and configure [OpenSSH](https://learn.microsoft.com/en-us/windows-server/administration/openssh/openssh_install_firstuse?tabs=powershell#install-openssh-for-windows)
- Install [Git for Windows](https://git-scm.com/)
- Configure the [default shell as `pwsh`](https://learn.microsoft.com/en-us/windows-server/administration/openssh/openssh_server_configuration#configuring-the-default-shell-for-openssh-in-windows). Update example with the correct full path:

  ```powershell
  New-ItemProperty -Path "HKLM:\SOFTWARE\OpenSSH" -Name DefaultShell -Value "$PSHOME\pwsh.exe" -PropertyType String -Force
  ```

- Add shell `pwsh` to [`config.toml`](../configuration/advanced-configuration.md)
