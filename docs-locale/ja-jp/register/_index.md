---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Runnerの登録
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

{{< history >}}

- GitLab Runner 15.0で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3414)。登録リクエストの形式が変更されたため、GitLab Runnerは以前のバージョンのGitLabと通信できなくなりました。GitLabのバージョンに適したバージョンのGitLab Runnerを使用するか、GitLabアプリケーションをアップグレードする必要があります。

{{< /history >}}

Runnerの登録とは、Runnerを1つ以上のGitLabインスタンスに関連付けるプロセスです。GitLabインスタンスからジョブを取得するには、Runnerを登録する必要があります。

## 要件 {#requirements}

Runnerを登録する前に:

- [GitLab Runner](../install/_index.md)を、GitLabがインストールされているサーバーとは別のサーバーにインストールします。
- DockerでRunnerを登録するために、[DockerコンテナにGitLab Runnerをインストール](../install/docker.md)します。

## Runner認証トークンで登録する {#register-with-a-runner-authentication-token}

{{< history >}}

- GitLab 15.10で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29613)されました。

{{< /history >}}

前提要件:

- Runner認証トークンを取得します。次のいずれかの方法があります:
  - インスタンス、グループ、またはプロジェクトのRunnerを作成します。手順については、[manageランナー](https://docs.gitlab.com/ci/runners/runners_scope)を参照してください。
  - `config.toml`ファイルの中でRunner認証トークンを見つける。Runner認証トークンのプレフィックスは`glrt-`です。

Runnerを登録すると、`config.toml`に設定が保存されます。

[Runner認証トークン](https://docs.gitlab.com/security/tokens/#runner-authentication-tokens)を使用してRunnerを登録するには:

1. registerコマンドを実行します:

   {{< tabs >}}

   {{< tab title="Linux" >}}

   ```shell
   sudo gitlab-runner register
   ```

   プロキシの背後にいる場合は、環境変数を追加してから、登録コマンドを実行します:

   ```shell
   export HTTP_PROXY=http://yourproxyurl:3128
   export HTTPS_PROXY=http://yourproxyurl:3128

   sudo -E gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="macOS" >}}

   ```shell
   gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="Windows" >}}

   ```shell
   .\gitlab-runner.exe register
   ```

   {{< /tab >}}

   {{< tab title="FreeBSD" >}}

   ```shell
   sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="Docker" >}}

   コンテナを使用して登録するには、次のいずれかを実行します:

   - 適切な設定ボリュームマウントによる有効期間の短い`gitlab-runner`コンテナを使用します:

     - ローカルシステムボリュームマウントの場合:

       ```shell
       docker run --rm -it -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register
       ```

       インストール中に`/srv/gitlab-runner/config`以外の設定ボリュームを使用した場合は、適切なボリュームでコマンドを更新します。

     - Dockerボリュームマウントの場合:

       ```shell
       docker run --rm -it -v gitlab-runner-config:/etc/gitlab-runner gitlab/gitlab-runner:latest register
       ```

   - アクティブなRunnerコンテナ内で実行可能ファイルを使用します:

     ```shell
     docker exec -it gitlab-runner gitlab-runner register
     ```

   {{< /tab >}}

   {{< /tabs >}}

1. GitLabのURLを入力します:
   - GitLab Self-ManagedのRunnerの場合は、GitLabインスタンスのURLを使用します。たとえば、プロジェクトが`gitlab.example.com/yourname/yourproject`でホストされている場合、GitLabインスタンスのURLは`https://gitlab.example.com`です。
   - GitLab.comのRunnerの場合、GitLabインスタンスのURLは`https://gitlab.com`です。
1. Runner認証トークンを入力します。
1. Runnerの説明を入力します。
1. ジョブタグをカンマで区切って入力します。
1. （オプション）Runnerのメンテナンスノートを入力します。
1. [executor](../executors/_index.md)のタイプを入力します。

- 異なる設定の複数のRunnerを同じホストマシンに登録するには、それぞれについて`register`コマンドを繰り返します。
- 複数のホストマシンに同じ設定を登録するには、各Runnerの登録に同じRunner認証トークンを使用します。詳細については、[Runner設定の再利用](../fleet_scaling/_index.md#reusing-a-runner-configuration)を参照してください。

[非対話モード](../commands/_index.md#non-interactive-registration)を使用して、追加の引数を使用してRunnerを登録することもできます:

{{< tabs >}}

{{< tab title="Linux" >}}

```shell
sudo gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

{{< /tab >}}

{{< tab title="macOS" >}}

```shell
gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

{{< /tab >}}

{{< tab title="Windows" >}}

```shell
.\gitlab-runner.exe register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker-windows" \
  --docker-image mcr.microsoft.com/windows/servercore:1809_amd64 \
  --description "docker-runner"
```

{{< /tab >}}

{{< tab title="FreeBSD" >}}

```shell
sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

{{< /tab >}}

{{< tab title="Docker" >}}

```shell
docker run --rm -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

{{< /tab >}}

{{< /tabs >}}

## Runner登録トークンで登録する（非推奨） {#register-with-a-runner-registration-token-deprecated}

{{< alert type="warning" >}}

Runnerの登録トークンといくつかのRunnerの設定引数は[非推奨](https://gitlab.com/gitlab-org/gitlab/-/issues/380872)になりました。これらは、GitLab 20.0での削除が予定されています。代わりにRunner認証トークンを使用してください。詳細については、[新しいRunner登録ワークフローに移行する](https://docs.gitlab.com/ci/runners/new_creation_workflow/)を参照してください。

{{< /alert >}}

前提要件:

- 管理者エリアでRunner登録トークンが[有効](https://docs.gitlab.com/administration/settings/continuous_integration/#allow-runner-registrations-tokens)になっている必要があります。
- 登録したいインスタンス、グループ、またはプロジェクトでRunner登録トークンを取得します。手順については、[manageランナー](https://docs.gitlab.com/ci/runners/runners_scope)を参照してください。

Runnerを登録すると、`config.toml`に設定が保存されます。

[Runner登録トークン](https://docs.gitlab.com/security/tokens/#runner-registration-tokens-deprecated)を使用してRunnerを登録するには:

1. registerコマンドを実行します:

   {{< tabs >}}

   {{< tab title="Linux" >}}

   ```shell
   sudo gitlab-runner register
   ```

   プロキシの背後にいる場合は、環境変数を追加してから、登録コマンドを実行します:

   ```shell
   export HTTP_PROXY=http://yourproxyurl:3128
   export HTTPS_PROXY=http://yourproxyurl:3128

   sudo -E gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="macOS" >}}

   ```shell
   gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="Windows" >}}

   ```shell
   .\gitlab-runner.exe register
   ```

   {{< /tab >}}

   {{< tab title="FreeBSD" >}}

   ```shell
   sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="Docker" >}}

   インストール中に作成したコンテナを登録するため、有効期間の短い`gitlab-runner`コンテナを起動するには:

   - ローカルシステムボリュームマウントの場合:

     ```shell
     docker run --rm -it -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register
     ```

     インストール中に`/srv/gitlab-runner/config`以外の設定ボリュームを使用した場合は、適切なボリュームでコマンドを更新します。

   - Dockerボリュームマウントの場合:

     ```shell
     docker run --rm -it -v gitlab-runner-config:/etc/gitlab-runner gitlab/gitlab-runner:latest register
     ```

   {{< /tab >}}

   {{< /tabs >}}

1. GitLabのURLを入力します:
   - GitLab Self-ManagedのRunnerの場合は、GitLabインスタンスのURLを使用します。たとえば、プロジェクトが`gitlab.example.com/yourname/yourproject`でホストされている場合、GitLabインスタンスのURLは`https://gitlab.example.com`です。
   - GitLab.comの場合、GitLabインスタンスのURLは`https://gitlab.com`です。
1. Runnerを登録するために取得したトークンを入力します。
1. Runnerの説明を入力します。
1. ジョブタグをカンマで区切って入力します。
1. （オプション）Runnerのメンテナンスノートを入力します。
1. [executor](../executors/_index.md)のタイプを入力します。

異なる設定の複数のRunnerを同じホストマシンに登録するには、それぞれについて`register`コマンドを繰り返します。

[非対話モード](../commands/_index.md#non-interactive-registration)を使用して、追加の引数を使用してRunnerを登録することもできます:

{{< tabs >}}

{{< tab title="Linux" >}}

```shell
sudo gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< tab title="macOS" >}}

```shell
gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< tab title="Windows" >}}

```shell
.\gitlab-runner.exe register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker-windows" \
  --docker-image mcr.microsoft.com/windows/servercore:1809_amd64 \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< tab title="FreeBSD" >}}

```shell
sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< tab title="Docker" >}}

```shell
docker run --rm -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< /tabs >}}

- `--access-level`は、[保護されたRunner](https://docs.gitlab.com/ci/runners/configure_runners/#prevent-runners-from-revealing-sensitive-information)を作成するかどうかを設定します。
  - 保護されたRunnerの場合は、`--access-level="ref_protected"`パラメータを使用します。
  - 保護されていないRunnerの場合は、`--access-level="not_protected"`を使用するか、値を未定義のままにします。
- `--maintenance-note`を使用すると、Runnerのメンテナンスに役立つ情報を追加できます。最大長は255文字です。

### レガシー互換登録プロセス {#legacy-compatible-registration-process}

{{< history >}}

- GitLab 16.2で[導入](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4157)されました。

{{< /history >}}

Runnerの登録トークンといくつかのRunnerの設定引数は[非推奨](https://gitlab.com/gitlab-org/gitlab/-/issues/379743)になりました。これらは、GitLab 20.0での削除が予定されています。自動化ワークフローへの影響を最小限にするため、レガシーパラメータ`--registration-token`の中でRunner認証トークンが指定されている場合、`legacy-compatible registration process`がトリガーされます。

レガシー互換登録プロセスでは、次のコマンドラインパラメータは無視されます。これらのパラメータは、UIまたはAPIでRunnerが作成された場合にのみ設定可能です。

- `--locked`
- `--access-level`
- `--run-untagged`
- `--maximum-timeout`
- `--paused`
- `--tag-list`
- `--maintenance-note`

## 設定テンプレートを使用して登録する {#register-with-a-configuration-template}

設定テンプレートを使用すると、`register`コマンドでサポートされていない設定でRunnerを登録できます。

前提要件:

- テンプレートファイルの格納場所となるボリュームは、GitLab Runnerコンテナにマウントされている必要があります。
- Runner認証トークンまたは登録トークン:
  - Runner認証トークンを取得します（推奨）。次のいずれかの方法があります:
    - 登録したいインスタンス、グループ、またはプロジェクトでRunner認証トークンを取得します。手順については、[manageランナー](https://docs.gitlab.com/ci/runners/runners_scope)を参照してください。
    - `config.toml`ファイルの中でRunner認証トークンを見つける。Runner認証トークンのプレフィックスは`glrt-`です。
  - （非推奨）インスタンス、グループ、またはプロジェクトの各RunnerのためのRunner登録トークンを取得する。手順については、[manageランナー](https://docs.gitlab.com/ci/runners/runners_scope)を参照してください。

設定テンプレートは、次の理由により`register`コマンドの一部の引数をサポートしていない自動化環境で使用できます:

- 環境に基づく環境変数のサイズ制限。
- Kubernetes用のexecutorボリュームで使用できないコマンドラインオプション。

{{< alert type="warning" >}}

設定テンプレートでサポートされるのは単一の[`[[runners]]`](../configuration/advanced-configuration.md#the-runners-section)セクションだけであり、グローバルオプションはサポートされません。

{{< /alert >}}

Runnerを登録するには、次のようにします:

1. `.toml`形式の設定テンプレートファイルを作成し、仕様を追加します。次に例を示します:

   ```toml
   [[runners]]
     [runners.kubernetes]
     [runners.kubernetes.volumes]
       [[runners.kubernetes.volumes.empty_dir]]
         name = "empty_dir"
         mount_path = "/path/to/empty_dir"
         medium = "Memory"
   ```

1. ファイルのパスを追加します。次のいずれかを使用できます:
   - コマンドラインの[非対話モード](../commands/_index.md#non-interactive-registration):

     ```shell
     $ sudo gitlab-runner register \
         --template-config /tmp/test-config.template.toml \
         --non-interactive \
         --url "https://gitlab.com" \
         --token <TOKEN> \ "# --registration-token if using the deprecated runner registration token"
         --name test-runner \
         --executor kubernetes
         --host = "http://localhost:9876/"
     ```

   - `.gitlab.yaml`ファイルの中の環境変数:

     ```yaml
     variables:
       TEMPLATE_CONFIG_FILE = <file_path>
     ```

     環境変数を更新する場合、`register`コマンドでファイルパスを毎回追加する必要はありません。

Runnerを登録すると、`config.toml`内で作成された`[[runners]]`エントリと設定テンプレートの設定がマージされます:

```toml
concurrent = 1
check_interval = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = "test-runner"
  url = "https://gitlab.com"
  token = "glrt-<TOKEN>"
  executor = "kubernetes"
  [runners.kubernetes]
    host = "http://localhost:9876/"
    bearer_token_overwrite_allowed = false
    image = ""
    namespace = ""
    namespace_overwrite_allowed = ""
    privileged = false
    service_account_overwrite_allowed = ""
    pod_labels_overwrite_allowed = ""
    pod_annotations_overwrite_allowed = ""
    [runners.kubernetes.volumes]

      [[runners.kubernetes.volumes.empty_dir]]
        name = "empty_dir"
        mount_path = "/path/to/empty_dir"
        medium = "Memory"
```

テンプレートの設定がマージされるのは、次の場合のみです:

- 空の文字列
- nullまたは存在しないエントリ
- ゼロ値

コマンドライン引数と環境変数は、設定テンプレートの設定よりも優先されます。たとえば、テンプレートでは`docker`executorを指定し、コマンドラインでは`shell`を指定した場合、設定されるexecutorは`shell`になります。

## GitLab Community Editionインテグレーションテスト用にRunnerを登録する {#register-a-runner-for-gitlab-community-edition-integration-tests}

GitLab Community Editionインテグレーションをテストするには、設定テンプレートを使用して、制限付きDocker executorでRunnerを登録します。

1. [プロジェクトRunner](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-project-runner-with-a-runner-authentication-token)を作成します。
1. `[[runners.docker.services]]`セクションを含むテンプレートを作成します:

   ```shell
   $ cat > /tmp/test-config.template.toml << EOF
   [[runners]]
   [runners.docker]
   [[runners.docker.services]]
   name = "mysql:latest"
   [[runners.docker.services]]
   name = "redis:latest"

   EOF
   ```

1. Runnerを登録します:

   {{< tabs >}}

   {{< tab title="Linux" >}}

   ```shell
   sudo gitlab-runner register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< tab title="macOS" >}}

   ```shell
   gitlab-runner register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< tab title="Windows" >}}

   ```shell
   .\gitlab-runner.exe register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< tab title="FreeBSD" >}}

   ```shell
   sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< tab title="Docker" >}}

   ```shell
   docker run --rm -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< /tabs >}}

その他の設定オプションについては、[高度な設定](../configuration/advanced-configuration.md)を参照してください。

## DockerによるRunnerの登録 {#registering-runners-with-docker}

DockerコンテナによるRunner登録後:

- 設定が設定ボリュームに書き込まれます。たとえば、`/srv/gitlab-runner/config`などです。
- コンテナが設定ボリュームを使用してRunnerを読み込みます。

{{< alert type="note" >}}

`gitlab-runner restart`がDockerコンテナ内で実行される場合、GitLab Runnerは既存のプロセスを再起動せず、新しいプロセスを開始します。設定変更を適用するには、Dockerコンテナを再起動します。

{{< /alert >}}

## トラブルシューティング {#troubleshooting}

### エラー: `Check registration token` {#error-check-registration-token}

`check registration token`（登録トークンを確認してください）エラーメッセージは、登録中に入力したRunner登録トークンをGitLabインスタンスが認識しない場合に表示されます。この問題は、次のいずれかの場合に発生する可能性があります:

- GitLabで、インスタンス、グループ、またはプロジェクトのRunner登録トークンが変更された。
- 正しくないRunner登録トークンが入力された。

このエラーが発生した場合は、GitLab管理者に次のことを依頼できます:

- Runner登録トークンが有効であることを確認する。
- プロジェクトまたはグループでRunner登録が[許可されている](https://docs.gitlab.com/administration/settings/continuous_integration/#restrict-runner-registration-by-all-members-in-a-group)ことを確認する。

### エラー: `410 Gone - runner registration disallowed` {#error-410-gone---runner-registration-disallowed}

`410 Gone - runner registration disallowed`（Runner登録が無効です）エラーメッセージは、登録トークンによるRunner登録が無効になっている場合に表示されます。

このエラーが発生した場合は、GitLab管理者に次のことを依頼できます:

- Runner登録トークンが有効であることを確認する。
- インスタンスでのRunner登録が[許可されている](https://docs.gitlab.com/administration/settings/continuous_integration/#allow-runner-registrations-tokens)ことを確認する。
- グループまたはプロジェクトのRunner登録トークンの場合、それぞれ対応するグループやプロジェクトでのRunner登録が[許可されている](https://docs.gitlab.com/ci/runners/runners_scope/#enable-use-of-runner-registration-tokens-in-projects-and-groups)ことを確認する。
