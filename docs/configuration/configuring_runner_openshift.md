---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://about.gitlab.com/handbook/engineering/ux/technical-writing/#designated-technical-writers
---

# Configuring GitLab Runner on OpenShift 

This document explains how to configure the GitLab Runner on OpenShift.

## Configure a proxy environment

To create a proxy environment:

1. Edit the `custom-env.yaml` file. For example:
    
   ```yaml
   apiVersion: v1
   data:
     HTTP_PROXY: example.com
   kind: ConfigMap
   metadata:
     name: custom-env
   ```
    
1. Update OpenShift to apply the changes.
    
   ```shell
   oc apply -f custom-env.yaml
   ``` 

1. Update your [`gitlab-runner.yml`](../install/openshift.md#install-gitlab-runner) file.

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     imagePullPolicy: Always
     token: gitlab-runner-secret # Name of the secret containing the Runner token
     tags: openshift, test
     env: custom-env
   ```

## Add root CA certs to runners

1. Mount a ConfigMap to the container running the job. 

   This uses the GitLab Runner [configuration template](../register/index.md#runners-configuration-template-file) called `custom-config.toml`.

   ```toml
   [[runners]]
     [runners.kubernetes]
       [runners.kubernetes.volumes]
         [[runners.kubernetes.volumes.config_map]]
           name = "config-map-1"
           mount_path = "/path/to/directory"
   ```

1. Apply the changes.

   ```shell
    oc create configmap custom-config-toml --from-file config.toml=custom-config.toml
   ```

## Configure a custom TLS cert

To set a custom TLS cert, create a secret with key `tls.crt` and set the ca key in the `runner.yaml` file.

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.com
  imagePullPolicy: Always
  token: gitlab-runner-secret # Name of the secret containing the Runner token
  tags: openshift, test
  config: custom-config-toml
``` 

## Configure the cpu and memory size of runner pods

To set [CPU limits](../executors/kubernetes.md#cpu-requests-and-limits) and [memory limits](../executors/kubernetes.md#memory-requests-and-limits) in a custom `config.toml` file, follow the instructions in [this topic](#add-root-ca-certs-to-runners).

## Configure job concurrency per runner based on cluster resources

Job concurrency is dictated by the requirements of the specific project. 

1. Start by trying to determine the compute and memory resources required to execute a CI job.
1. Calculate how many times that job would be able to execute given the resources in the cluster.

If you set too large a concurrency value, Kubernetes still schedules the jobs as soon as it can.
