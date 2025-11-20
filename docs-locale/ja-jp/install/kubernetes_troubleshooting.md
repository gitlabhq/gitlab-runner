---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runner Helmチャートのトラブルシューティング
---

## エラー: `Job failed (system failure): secrets is forbidden` {#error-job-failed-system-failure-secrets-is-forbidden}

次のエラーが表示された場合は、[RBACサポートを有効にする](kubernetes_helm_chart_configuration.md#enable-rbac-support)と、問題を解決できます:

```plaintext
Using Kubernetes executor with image alpine ...
ERROR: Job failed (system failure): secrets is forbidden: User "system:serviceaccount:gitlab:default"
cannot create resource "secrets" in API group "" in the namespace "gitlab"
```

## エラー: `Unable to mount volumes for pod` {#error-unable-to-mount-volumes-for-pod}

必要なシークレットのマウントボリュームに失敗する場合は、登録トークンまたはRunnerトークンがシークレットに保存されていることを確認してください。

## Google Cloud Storageへの低速なアーティファクトアップロード {#slow-artifact-uploads-to-google-cloud-storage}

Google Cloud Storageへのアーティファクトアップロードは、runnerヘルパーポッドがCPUバウンドになるため、パフォーマンスが低下する可能性があります（帯域幅レートが遅くなる）。この問題を軽減するには、ヘルパーポッドのCPU制限を増やしてください:

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        helper_cpu_limit = "250m"
```

詳細については、[issue 28393](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28393#note_722733798)を参照してください。

## エラー: `PANIC: creating directory: mkdir /nonexistent: permission denied` {#error-panic-creating-directory-mkdir-nonexistent-permission-denied}

このエラーを解決するには、[UbuntuベースのGitLab Runner Dockerイメージ](kubernetes_helm_chart_configuration.md#switch-to-the-ubuntu-based-gitlab-runner-docker-image)に切り替えてください。

## エラー: `invalid header field for "Private-Token"` {#error-invalid-header-field-for-private-token}

`gitlab-runner-secret`の`runner-token`値が、末尾に改行文字（`\n`）を使用してbase64エンコードされている場合、このエラーが表示されることがあります:

```plaintext
couldn't execute POST against "https:/gitlab.example.com/api/v4/runners/verify":
net/http: invalid header field for "Private-Token"
```

この問題を解決するには、改行（`\n`）がトークン値に追加されていないことを確認してください。例: `echo -n <GITLAB_TOKEN> | base64`。

## エラー: `FATAL: Runner configuration is reserved` {#error-fatal-runner-configuration-is-reserved}

GitLab Runner Helmチャートのインストール後、ポッドログに次のエラーが表示されることがあります:

```plaintext
FATAL: Runner configuration other than name and executor configuration is reserved
(specifically --locked, --access-level, --run-untagged, --maximum-timeout, --paused, --tag-list, and --maintenance-note)
and cannot be specified when registering with a runner authentication token. This configuration is specified
on the GitLab server. Please try again without specifying any of those arguments
```

このエラーは、認証トークンを使用し、シークレットを介してトークンを提供する場合に発生します。これを修正するには、values YAMLファイルを確認し、非推奨の値を使用していないことを確認してください。どの値が非推奨になっているかの詳細については、[GitLab RunnerをHelmチャートでインストールする](https://docs.gitlab.com/ci/runners/new_creation_workflow/#installing-gitlab-runner-with-helm-chart)を参照してください。
