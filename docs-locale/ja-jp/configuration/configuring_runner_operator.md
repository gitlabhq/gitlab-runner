---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: OpenShiftでのGitLab Runnerの設定
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

このドキュメントでは、OpenShiftでGitLab Runnerを設定する方法について説明します。

## GitLab Runner Operatorへのプロパティの引き渡し {#passing-properties-to-gitlab-runner-operator}

`Runner`を作成する際、その`spec`にプロパティを設定することで、それを設定できます。たとえば、runnerが登録されているGitLab URLや、登録トークンを含むシークレットの名前を指定できます:

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret # Name of the secret containing the Runner token
```

使用可能なすべてのプロパティについては、[Operatorのプロパティ](#operator-properties)をお読みください。

## Operatorのプロパティ {#operator-properties}

次のプロパティをOperatorに渡すことができます。

一部のプロパティは、より新しいバージョンのOperatorでのみ使用できます。

| 設定            | オペレーター | 説明 |
|--------------------|----------|-------------|
| `gitlabUrl`        | すべて      | GitLabインスタンスの完全修飾ドメイン名（例：`https://gitlab.example.com`）。 |
| `token`            | すべて      | Runnerの登録に使用される`Secret``runner-registration-token`キーを含むシークレットの名前。 |
| `tags`             | すべて      | Runnerに適用されるコンマ区切りのトピックのリスト。 |
| `concurrent`       | すべて      | 同時に実行できるジョブの数を制限します。最大数は、定義されているすべてのrunnerです。0は無制限を意味しません。デフォルトは`10`です。 |
| `interval`         | すべて      | 新しいジョブのチェック間隔（秒数）を定義します。デフォルトは`30`です。 |
| `locked`           | 1.8      | Runnerをプロジェクトにロックするかどうかを定義します。デフォルトは`false`です。 |
| `runUntagged`      | 1.8      | タグなしのジョブを実行するかどうかを定義します。タグが指定されていない場合、`true`がデフォルトです。それ以外の場合は、`false`になります。 |
| `protected`        | 1.8      | Runnerが保護ブランチでのみジョブを実行するかどうかを定義します。デフォルトは`false`です。 |
| `cloneURL`         | すべて      | GitLabインスタンスのURLを上書きします。RunnerがGitlab URLに接続できない場合にのみ使用されます。 |
| `env`              | すべて      | Runnerポッドの環境変数として挿入されるキー/バリューペアを含む`ConfigMap`の名前。 |
| `runnerImage`      | 1.7      | デフォルトのGitLab Runner Dockerイメージを上書きします。デフォルトは、オペレーターにバンドルされていたRunnerイメージです。 |
| `helperImage`      | すべて      | デフォルトのGitLab Runnerヘルパーイメージを上書きします。 |
| `buildImage`       | すべて      | 指定されていない場合にビルドに使用するデフォルトのDockerイメージ。 |
| `cacheType`        | すべて      | Runnerアーティファクトに使用されるキャッシュのタイプ。`gcs`、`s3`、`azure`のいずれか。 |
| `cachePath`        | すべて      | ファイルシステム上のキャッシュパスを定義します。 |
| `cacheShared`      | すべて      | Runner間でキャッシュの共有を有効にします。 |
| `s3`               | すべて      | S3キャッシュの設定に使用されるオプション。[キャッシュプロパティ](#cache-properties)を参照してください。 |
| `gcs`              | すべて      | `gcs`キャッシュの設定に使用されるオプション。[キャッシュプロパティ](#cache-properties)を参照してください。 |
| `azure`            | すべて      | Azureキャッシュの設定に使用されるオプション。[キャッシュプロパティ](#cache-properties)を参照してください。 |
| `ca`               | すべて      | カスタム認証局 () 証明書を含むTLSシークレットの名前。 |
| `serviceaccount`   | すべて      | Runnerポッドの実行に使用されるサービスアカウントをオーバーライドするために使用します。 |
| `config`           | すべて      | [設定テンプレート](../register/_index.md#register-with-a-configuration-template)を使用して、カスタム`ConfigMap`を提供するために使用します。 |
| `shutdownTimeout`  | 1.34     | [強制シャットダウン操作](../commands/_index.md#signals)がタイムアウトになりプロセスが終了するまでの秒数を示します。デフォルト値は`30`です。`0`以下に設定すると、デフォルト値が使用されます。 |
| `logLevel`         | 1.34     | ログレベルを定義します。オプションには、`debug`、`info`、`warn`、`error`、`fatal`、`panic`があります。 |
| `logFormat`        | 1.34     | ログ形式を指定します。オプションには、`runner`、`text`、`json`があります。デフォルト値は`runner`で、色分けのためのANSIエスケープコードが含まれています。 |
| `listenAddr`       | 1.34     | Prometheusメトリクス用HTTPサーバーがリッスンするアドレス（`<host>:<port>`）を定義します。設定の詳細については、[GitLab Runner Operatorの監視](../monitoring/_index.md#monitor-operator-managed-gitlab-runners)を参照してください。 |
| `sentryDsn`        | 1.34     | Sentryへのすべてのシステムレベルのエラーの追跡を有効にします。 |
| `connectionMaxAge` | 1.34     | GitLabサーバーへのTLSキープアライブ接続を再接続するまでの最大時間を指定します。デフォルト値は`15m`（15分）です。`0`以下に設定すると、接続は可能な限り持続します。 |
| `podSpec`          | 1.23     | GitLab Runnerポッド（テンプレート）に適用するパッチのリスト。詳細については、[Runnerポッドテンプレートのパッチ](#patching-the-runner-pod-template)を参照してください。 |
| `deploymentSpec`   | 1.40     | GitLab Runnerデプロイに適用するパッチのリスト。詳細については、[Runnerデプロイテンプレートのパッチ](#patching-the-runner-deployment-template)を参照してください。 |

## キャッシュプロパティ {#cache-properties}

### S3キャッシュ {#s3-cache}

| 設定       | オペレーター | 説明 |
|---------------|----------|-------------|
| `server`      | すべて      | S3サーバーアドレス。 |
| `credentials` | すべて      | `accesskey`プロパティと`secretkey`プロパティを含む、オブジェクトストレージへのアクセスに使用される`Secret`の名前。 |
| `bucket`      | すべて      | キャッシュが保存されているバケットの名前。 |
| `location`    | すべて      | キャッシュが保存されているS3リージョンの名前。 |
| `insecure`    | すべて      | インセキュアな接続または`HTTP`を使用します。 |

### `gcs` キャッシュ {#gcs-cache}

| 設定           | オペレーター | 説明 |
|-------------------|----------|-------------|
| `credentials`     | すべて      | `access-id`プロパティと`private-key`プロパティを含む、オブジェクトストレージへのアクセスに使用される`Secret`の名前。 |
| `bucket`          | すべて      | キャッシュが保存されているバケットの名前。 |
| `credentialsFile` | すべて      | `gcs`認証情報ファイル`keys.json`を取得します。 |

### Azureキャッシュ {#azure-cache}

| 設定         | オペレーター | 説明 |
|-----------------|----------|-------------|
| `credentials`   | すべて      | `accountName`プロパティと`privateKey`プロパティを含む、オブジェクトストレージへのアクセスに使用される`Secret`の名前。 |
| `container`     | すべて      | キャッシュが保存されているAzureコンテナの名前。 |
| `storageDomain` | すべて      | Azure blobストレージのドメイン名。 |

## プロキシ環境の設定 {#configure-a-proxy-environment}

プロキシ環境を作成するには:

1. `custom-env.yaml`ファイルを編集します。次に例を示します: 

   ```yaml
   apiVersion: v1
   data:
     HTTP_PROXY: example.com
   kind: ConfigMap
   metadata:
     name: custom-env
   ```

1. OpenShiftを更新して変更を適用します。

   ```shell
   oc apply -f custom-env.yaml
   ```

1. [`gitlab-runner.yml`](../install/operator.md#install-gitlab-runner)ファイルを更新してください。

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     token: gitlab-runner-secret # Name of the secret containing the Runner token
     env: custom-env
   ```

プロキシがKubernetes APIにアクセスできない場合は、CI/CDジョブでエラーが発生する可能性があります:

```shell
ERROR: Job failed (system failure): prepare environment: setting up credentials: Post https://172.21.0.1:443/api/v1/namespaces/<KUBERNETES_NAMESPACE>/secrets: net/http: TLS handshake timeout. Check https://docs.gitlab.com/runner/shells/#shell-profile-loading for more information
```

このエラーを解決するには、Kubernetes APIのIPアドレスを`custom-env.yaml`ファイルの`NO_PROXY`設定に追加します:

```yaml
   apiVersion: v1
   data:
     NO_PROXY: 172.21.0.1
     HTTP_PROXY: example.com
   kind: ConfigMap
   metadata:
     name: custom-env
```

Kubernetes APIのIPアドレスは、次を実行して確認できます:

```shell
oc get services --namespace default --field-selector='metadata.name=kubernetes' | grep -v NAME | awk '{print $3}'
```

## `config.toml`を設定テンプレートでカスタマイズする {#customize-configtoml-with-a-configuration-template}

[設定テンプレート](../register/_index.md#register-with-a-configuration-template)を使用して、Runnerの`config.toml`ファイルをカスタマイズできます。

1. カスタム設定テンプレートファイルを作成します。たとえば、Runnerに`EmptyDir`ボリュームをマウントし、`cpu_limit`を設定するように指示します。`custom-config.toml`ファイルを作成します:

   ```toml
   [[runners]]
     [runners.kubernetes]
       cpu_limit = "500m"
       [runners.kubernetes.volumes]
         [[runners.kubernetes.volumes.empty_dir]]
           name = "empty-dir"
           mount_path = "/path/to/empty_dir"
           medium = "Memory"
   ```

1. `custom-config.toml`ファイルから`custom-config-toml`という名前の`ConfigMap`を作成します:

   ```shell
    oc create configmap custom-config-toml --from-file config.toml=custom-config.toml
   ```

1. `Runner`の`config`プロパティを設定します:

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     token: gitlab-runner-secret
     config: custom-config-toml
   ```

[既知の問題](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/-/issues/229)のため、次の設定を変更するには、設定テンプレートの代わりに環境変数を使用する必要があります:

| 設定                          | 環境変数         | デフォルト値 |
|----------------------------------|------------------------------|---------------|
| `runners.request_concurrency`    | `RUNNER_REQUEST_CONCURRENCY` | `1`           |
| `runners.output_limit`           | `RUNNER_OUTPUT_LIMIT`        | `4096`        |
| `kubernetes.runner.poll_timeout` | `KUBERNETES_POLL_TIMEOUT`    | `180`         |

## カスタムTLS証明書の設定 {#configure-a-custom-tls-cert}

1. カスタムTLS証明書を設定するには、キー`tls.crt`を持つシークレットを作成します。この例では、ファイルの名前は`custom-tls-ca-secret.yaml`です:

   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
       name: custom-tls-ca
   type: Opaque
   stringData:
       tls.crt: |
           -----BEGIN CERTIFICATE-----
           MIIEczCCA1ugAwIBAgIBADANBgkqhkiG9w0BAQQFAD..AkGA1UEBhMCR0Ix
           .....
           7vQMfXdGsRrXNGRGnX+vWDZ3/zWI0joDtCkNnqEpVn..HoX
           -----END CERTIFICATE-----
   ```

1. シークレットを作成します:

   ```shell
   oc apply -f custom-tls-ca-secret.yaml
   ```

1. `runner.yaml`の`ca`キーを、シークレットの名前と同じ名前に設定します:

   ```yaml
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: dev
   spec:
     gitlabUrl: https://gitlab.example.com
     token: gitlab-runner-secret
     ca: custom-tls-ca
   ```

## RunnerポッドのCPUおよびメモリサイズの設定 {#configure-the-cpu-and-memory-size-of-runner-pods}

カスタム`config.toml`ファイルで[CPU制限](../executors/kubernetes/_index.md#cpu-requests-and-limits)と[メモリ制限](../executors/kubernetes/_index.md#memory-requests-and-limits)を設定するには、[このトピック](#customize-configtoml-with-a-configuration-template)の手順に従ってください。

## クラスターリソースに基づいて、Runnerごとのジョブの並行処理を設定します {#configure-job-concurrency-per-runner-based-on-cluster-resources}

`Runner`リソースの`concurrent`プロパティを設定します:

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  concurrent: 2
```

ジョブの並行処理は、プロジェクトの要件によって決まります。

1. まず、CIジョブを実行するために必要なコンピューティングリソースとメモリリソースを特定します。
1. クラスター内のリソースを考慮して、そのジョブを何回実行できるかを計算します。

高い並行処理値を設定すると、Kubernetesエグゼキューターは可能な限りすぐにジョブを処理します。ただし、ジョブがスケジュールされるタイミングは、Kubernetesクラスターのスケジューラ容量によって決まります。

## GitLab Runnerマネージャーのサービスアカウント {#service-account-for-the-gitlab-runner-manager}

新規インストールの場合は、これらのRBACロールバインディングリソースが存在しない場合、GitLab Runnerはrunnerマネージャーポッド用に`gitlab-runner-app-sa`という名前のKubernetes `ServiceAccount`を作成します:

- `gitlab-runner-app-rolebinding`
- `gitlab-runner-rolebinding`

ロールバインディングのいずれかが存在する場合、GitLabは、ロールバインディングで定義されている`subjects`と`roleRef`からロールとサービスアカウントを解決します。

両方のロールバインディングが存在する場合、`gitlab-runner-app-rolebinding`は`gitlab-runner-rolebinding`よりも優先されます。

## トラブルシューティング {#troubleshooting}

### ルートと非ルート {#root-vs-non-root}

GitLab Runner OperatorとGitLab Runnerポッドは、非ルートユーザーとして実行されます。そのため、ジョブで使用されるビルドイメージは、正常に完了できるように、非ルートユーザーとして実行する必要があります。これにより、ジョブは最小限の権限で正常に実行されることが保証されます。

これを機能させるには、CI/CDジョブに使用されるビルドイメージが以下であることを確認してください:

- 非ルートとして実行
- 制限されたファイルシステムに書き込まない

OpenShiftクラスター上のほとんどのコンテナファイルシステムは読み取り専用ですが、次の例外があります:

- マウントされたボリューム
- `/var/tmp`
- `/tmp`
- `tmpfs`としてルートファイルシステムにマウントされたその他のボリューム

#### `HOME`環境変数のオーバーライド {#overriding-the-home-environment-variable}

カスタムビルドイメージを作成するか、[環境変数をオーバーライドする](#configure-a-proxy-environment)場合は、`HOME`環境変数が`/`に設定されていないことを確認してください。これは読み取り専用になります。特に、ジョブがホームディレクトリにファイルを書き込む必要がある場合。たとえば、`/home`の下にディレクトリ（`/home/ci`など）を作成し、`Dockerfile`で`ENV HOME=/home/ci`を設定できます。

Runnerポッドの場合、[`HOME`が`/home/gitlab-runner`に設定されることが予想されます](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/-/blob/e265820a00a6a1b9a271dc132de2618ced43cf92/runner/Dockerfile.OCP#L14)。この変数が変更された場合、新しい場所には[適切な権限](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/-/blob/e265820a00a6a1b9a271dc132de2618ced43cf92/runner/Dockerfile.OCP#L38)が必要です。これらのガイドラインは、[Red Hatコンテナプラットフォームのドキュメント](https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/images/creating-images#images-create-guide-openshift_create-images)にも記載されています。

### `locked`変数のオーバーライド {#overriding-locked-variable}

Runnerトークンを登録するときに、`locked`変数を`true`に設定すると、エラー`Runner configuration other than name, description, and exector is reserved and cannot be specified`が表示されます。

```yaml
  locked: true # REQUIRED
  tags: ""
  runUntagged: false
  protected: false
  maximumTimeout: 0
```

詳細については、[イシュー472](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/472#note_1483346437)を参照してください。

#### セキュリティコンテキスト制約に注意してください {#watch-out-for-security-context-constraints}

デフォルトでは、新しいOpenShiftプロジェクトにインストールすると、GitLab Runner Operatorは非ルートとして実行されます。`default`プロジェクトなどの一部のプロジェクトは、すべてのサービスアカウントが`anyuid`アクセス権を持っている例外です。その場合、イメージのユーザーは`root`です。これは、ジョブなど、コンテナShell内で`whoami`を実行することで確認できます。[Red Hatコンテナプラットフォームのドキュメント](https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/authentication_and_authorization/managing-pod-security-policies)のセキュリティコンテキスト制約の詳細をご覧ください。

#### `anyuid`セキュリティコンテキストの制約として実行 {#run-as-anyuid-security-context-constraints}

{{< alert type="warning" >}}

ルートとしてジョブを実行したり、ルートファイルシステムに書き込んだりすると、システムがセキュリティリスクにさらされる可能性があります。

{{< /alert >}}

CI/CDジョブをルートユーザーとして実行したり、ルートファイルシステムに書き込んだりするには、`gitlab-runner-app-sa`サービスアカウントに`anyuid`セキュリティコンテキスト制約を設定します。GitLab Runnerコンテナは、このサービスアカウントを使用します。

OpenShift 4.3.8以前:

```shell
oc adm policy add-scc-to-user anyuid -z gitlab-runner-app-sa -n <runner_namespace>

# Check that the anyiud SCC is set:
oc get scc anyuid -o yaml
```

OpenShift 4.3.8以降:

```shell
oc create -f - <<EOF
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: scc-anyuid
  namespace: <runner_namespace>
rules:
- apiGroups:
  - security.openshift.io
  resourceNames:
  - anyuid
  resources:
  - securitycontextconstraints
  verbs:
  - use
EOF

oc create -f - <<EOF
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: sa-to-scc-anyuid
  namespace: <runner_namespace>
subjects:
  - kind: ServiceAccount
    name: gitlab-runner-app-sa
roleRef:
  kind: Role
  name: scc-anyuid
  apiGroup: rbac.authorization.k8s.io
EOF
```

#### ヘルパーコンテナとビルドコンテナのユーザーIDとグループIDのマッチング {#matching-helper-container-and-build-container-user-id-and-group-id}

GitLab Runner Operatorデプロイでは、`registry.gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/gitlab-runner-helper-ocp`がデフォルトのヘルパーイメージとして使用されます。このイメージは、セキュリティコンテキストによって明示的に変更されない限り、ユーザーIDとグループID `1001:1001`で実行されます。

ビルドコンテナのユーザーIDがヘルパーイメージのユーザーIDと異なる場合、ビルド中に権限関連のエラーが発生する可能性があります。一般的なエラーメッセージを次に示します:

```shell
fatal: detected dubious ownership in repository at '/builds/gitlab-org/gitlab-runner'
```

このエラーは、リポジトリがユーザーID `1001`（ヘルパーコンテナ）によってクローンされたことを示していますが、ビルドコンテナ内の別のユーザーIDがそれにアクセスしようとしています。

**解決策**:

ヘルパーコンテナのユーザーIDとグループIDに合わせて、ビルドコンテナのセキュリティコンテキストを設定します:

```toml
[runners.kubernetes.build_container_security_context]
run_as_user = 1001
run_as_group = 1001
```

**Additional notes**（追加の注意）*

- これらの設定により、リポジトリをクローンするコンテナと、それをビルドするコンテナの間で、一貫したファイルの所有権が保証されます。
- 異なるユーザーIDまたはグループIDでヘルパーイメージをカスタマイズした場合は、これらの値をそれに応じて調整します。
- OpenShiftデプロイの場合は、これらのセキュリティコンテキスト設定がクラスターのセキュリティコンテキスト制約（SCCS）に準拠していることを確認してください。

#### SETFCAPの設定 {#configure-setfcap}

Red Hat OpenShiftコンテナプラットフォーム（RHOCP）4.11以降を使用している場合は、次のエラーメッセージが表示されることがあります:

```shell
error reading allowed ID mappings:error reading subuid mappings for user
```

一部のジョブ（`buildah`など）では、正しく実行するために`SETFCAP`機能が付与されている必要があります。このイシューを解決するには、次の手順に従います:

1. GitLab Runnerが使用しているセキュリティコンテキスト制約にSETFCAP機能を追加します（GitLab Runnerポッドに割り当てられているセキュリティコンテキスト制約を`gitlab-scc`に置き換えます）:

   ```shell
   oc patch scc gitlab-scc --type merge -p '{"allowedCapabilities":["SETFCAP"]}'
   ```

1. `config.toml`を更新し、`kubernetes`セクションの下に`SETFCAP`機能を追加します:

   ```yaml
   [[runners]]
     [runners.kubernetes]
     [runners.kubernetes.pod_security_context]
       [runners.kubernetes.build_container_security_context]
       [runners.kubernetes.build_container_security_context.capabilities]
         add = ["SETFCAP"]
   ```

1. GitLab Runnerがデプロイされているネームスペースで、この`config.toml`を使用して`ConfigMap`を作成します:

   ```shell
   oc create configmap custom-config-toml --from-file config.toml=config.toml
   ```

1. 修正するRunnerを修正し、最近作成した`ConfigMap`を指すように`config:`パラメータを追加します（my-runnerを正しいRunnerポッド名に置き換えます）。

   ```shell
   oc patch runner my-runner --type merge -p '{"spec": {"config": "custom-config-toml"}}'
   ```

詳細については、[Red Hatのドキュメント](https://access.redhat.com/solutions/7016013)を参照してください。

### FIPS準拠のGitLab Runnerを使用する {#using-fips-compliant-gitlab-runner}

{{< alert type="note" >}}

Operatorの場合、変更できるのはヘルパーイメージのみです。GitLab Runnerイメージはまだ変更できません。[イシュー28814](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28814)は、この機能を追跡します。

{{< /alert >}}

[FIPS準拠のGitLab Runnerヘルパー](../install/_index.md#fips-compliant-gitlab-runner)を使用するには、次のようにヘルパーイメージを変更します:

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  helperImage: gitlab/gitlab-runner-helper:ubi-fips
  concurrent: 2
```

#### 自己署名証明書を使用したGitLab Runnerの登録 {#register-gitlab-runner-by-using-a-self-signed-certificate}

自己署名証明書をGitLab Self-Managedで使用するには、秘密証明書の署名に使用したCA証明書を含むシークレットを作成します。

シークレットの名前は、Runner仕様セクションでCAとして指定されます:

```yaml
KIND:     Runner
VERSION:  apps.gitlab.com/v1beta2

FIELD:    ca <string>

DESCRIPTION:
     Name of tls secret containing the custom certificate authority (CA)
     certificates
```

シークレットは、次のコマンドを使用して作成できます:

```shell
oc create secret generic mySecret --from-file=tls.crt=myCert.pem -o yaml
```

#### IPアドレスを指す外部URLでGitLab Runnerを登録します {#register-gitlab-runner-with-an-external-url-that-points-to-an-ip-address}

Runnerが自己署名証明書とホスト名を一致させることができない場合、エラーメッセージが表示される場合があります。この問題は、ホスト名の代わりにIPアドレス（###.##.##.##など）を使用するようにGitLab Self-Managedを設定した場合に発生します:

```shell
[31;1mERROR: Registering runner... failed               [0;m  [31;1mrunner[0;m=A5abcdEF [31;1mstatus[0;m=couldn't execute POST against https://###.##.##.##/api/v4/runners:
Post https://###.##.##.##/api/v4/runners: x509: cannot validate certificate for ###.##.##.## because it doesn't contain any IP SANs
[31;1mPANIC: Failed to register the runner. You may be having network problems.[0;m
```

このイシューを解決するには、次の手順に従います:

1. GitLab Self-Managedサーバーで、`subjectAltName`パラメータにIPアドレスを追加するように`openssl`を変更します:

   ```shell
   # vim /etc/pki/tls/openssl.cnf

   [ v3_ca ]
   subjectAltName=IP:169.57.64.36 <---- Add this line. 169.57.64.36 is your GitLab server IP.
    ```

1. 次に、次のコマンドを使用して自己署名CAを再生成します:

   ```shell
   # cd /etc/gitlab/ssl
   # openssl req -x509 -nodes -days 3650 -newkey rsa:4096 -keyout /etc/gitlab/ssl/169.57.64.36.key -out /etc/gitlab/ssl/169.57.64.36.crt
   # openssl dhparam -out /etc/gitlab/ssl/dhparam.pem 4096
   # gitlab-ctl restart
   ```

1. この新しい証明書を使用して、新しいシークレットを生成します。

## パッチの構造 {#patch-structure}

各仕様パッチは、次のプロパティで構成されています:

| 設定     | 説明                                                                                                                                     |
|-------------|-------------------------------------------------------------------------------------------------------------------------------------------------|
| `name`      | カスタム仕様パッチの名前。                                                                                                     |
| `patchFile` | 最終的な仕様の生成前に、このオブジェクトに適用する変更を定義するファイルのパス。このファイルはJSONまたはYAMLファイルである必要があります。 |
| `patch`     | 最終的な仕様に適用する変更を記述したJSONまたはYAML形式の文字列（生成前）。                         |
| `patchType` | 指定された変更を仕様に適用するために使用される戦略。使用できる値は、`merge`、`json`、`strategic`（デフォルト）です。  |

同じ仕様の設定で、`patchFile`と`patch`の両方を設定することはできません。

## Runnerポッドテンプレートのパッチ {#patching-the-runner-pod-template}

[ポッド仕様](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/pod-template-v1/#PodTemplateSpec)のパッチを使用すると、オペレーターが生成したKubernetesデプロイにパッチを適用することで、GitLab Runnerのデプロイ方法をカスタマイズできます。パッチは、ポッドテンプレートの仕様（`deployment.spec.template.spec`）に適用されます。

次のようなポッドレベルの設定を制御できます:

- リソースのリクエストと制限
- セキュリティコンテキスト
- ボリュームのマウントとボリューム
- 環境変数
- ノードセレクターとアフィニティルール
- Tolerations（トレランス）
- ホスト名とDNS設定

## Runnerデプロイテンプレートのパッチ {#patching-the-runner-deployment-template}

[デプロイメント仕様](https://kubernetes.io/docs/reference/kubernetes-api/workload-resources/deployment-v1/#Deployment)のパッチを使用すると、オペレーターが生成したKubernetesデプロイにパッチを適用することで、GitLab Runnerのデプロイ方法をカスタマイズできます。パッチは、デプロイ仕様（`deployment.spec`）に適用されます。

次のようなデプロイレベルの設定を制御できます:

- レプリカ数
- デプロイメント戦略（RollingUpdate、Recreate）
- リビジョン履歴制限
- 進捗期限秒数
- ラベルと注釈

## パッチの順序 {#patch-order}

デプロイメント仕様のパッチは、ポッド仕様のパッチの前に適用されます。つまり、デプロイメントとポッドの仕様が同じフィールドを変更した場合、ポッドの仕様が優先されます。

## 例 {#examples}

### ポッド仕様のパッチの例 {#pod-specification-patching-example}

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  podSpec:
    - name: "set-hostname"
      patch: |
        hostname: "custom-hostname"
      patchType: "merge"
    - name: "add-resource-requests"
      patch: |
        containers:
        - name: build
          resources:
            requests:
              cpu: "500m"
              memory: "256Mi"
      patchType: "strategic"
```

### デプロイメント仕様のパッチの例 {#deployment-specification-patching-example}

```yaml
apiVersion: apps.gitlab.com/v1beta2
kind: Runner
metadata:
  name: dev
spec:
  gitlabUrl: https://gitlab.example.com
  token: gitlab-runner-secret
  deploymentSpec:
    - name: "set-replicas"
      patch: |
        replicas: 3
      patchType: "strategic"
    - name: "configure-strategy"
      patch: |
        strategy:
          type: RollingUpdate
          rollingUpdate:
            maxUnavailable: 25%
            maxSurge: 50%
      patchType: "strategic"
    - name: "set-revision-history"
      patch: |
        [{"op": "add", "path": "/revisionHistoryLimit", "value": 10}]
      patchType: "json"
```

## ベストプラクティス {#best-practices}

- 本番環境へのデプロイに適用する前に、非本番環境でパッチをテストします。
- 個々のポッド設定ではなく、デプロイの動作に影響する設定には、デプロイレベルのパッチを使用します。
- ポッド仕様のパッチは、競合するフィールドのデプロイメント仕様のパッチをオーバーライドすることに注意してください。
