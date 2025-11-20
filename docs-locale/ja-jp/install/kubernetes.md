---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runner Helmチャート
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runner Helmチャートは、GitLab RunnerインスタンスをKubernetesクラスターにデプロイするための公式の手法です。このチャートにより、GitLab Runnerが次のように設定されます:

- GitLab Runnerの[Kubernetes executor](../executors/kubernetes/_index.md)を使用して実行する。
- 新しいCI/CDジョブごとに、指定されたネームスペースで新しいポッドをプロビジョニングする。

## HelmチャートでGitLab Runnerを設定する {#configure-gitlab-runner-with-the-helm-chart}

GitLab Runnerの設定の変更を`values.yaml`に保存します。このファイルの設定については、以下を参照してください:

- チャートリポジトリ内のデフォルトの[`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml)設定。
- [値ファイル](https://helm.sh/docs/chart_template_guide/values_files/)に関するHelmドキュメント。値ファイルによってデフォルト値がオーバーライドされる仕組みが説明されています。

GitLab Runnerを適切に実行するには、設定ファイルで次の値を設定する必要があります:

- `gitlabUrl`: Runnerの登録先のGitLabサーバーの完全なURL（`https://gitlab.example.com`など）。
- `rbac: { create: true }`: GitLab Runnerがジョブを実行するポッドを作成するためのRBAC（ロールベースのアクセス制御）ルールを作成します。
  - 既存の`serviceAccount`を使用する場合は、`rbac`にサービスアカウント名を追加してください:

    ```yaml
    rbac:
      create: false
    serviceAccount:
      create: false
      name: your-service-account
    ```

  - `serviceAccount`に必要な最小限の権限については、[Runner APIの権限を設定する](../executors/kubernetes/_index.md#configure-runner-api-permissions)を参照してください。
- `runnerToken`: [GitLab UIでRunnerを作成する](https://docs.gitlab.com/ci/runners/runners_scope/#create-an-instance-runner-with-a-runner-authentication-token)ときに取得した認証トークン。
  - このトークンを直接設定するか、シークレットに保存します。

その他の[オプションの設定](kubernetes_helm_chart_configuration.md)も使用できます。

これで、[GitLab Runnerをインストール](#install-gitlab-runner-with-the-helm-chart)する準備ができました。

## Helmチャートを使用してGitLab Runnerをインストールする {#install-gitlab-runner-with-the-helm-chart}

前提要件:

- GitLabサーバーのAPIにクラスターからアクセスできること。
- ベータAPIが有効になっているKubernetes 1.4以降。
- `kubectl` CLIがローカルにインストールされ、クラスターに対して認証されていること。
- [Helmクライアント](https://helm.sh/docs/using_helm/#installing-the-helm-client)がマシンにローカルにインストールされていること。
- [`values.yaml`で必要な値](#configure-gitlab-runner-with-the-helm-chart)をすべて設定していること。

HelmチャートからGitLab Runnerをインストールするには、次の手順に従います:

1. GitLab Helmリポジトリを追加します。

   ```shell
   helm repo add gitlab https://charts.gitlab.io
   ```

1. Helm 2を使用している場合は、`helm init`でHelmを初期化します。
1. アクセスできるGitLab Runnerのバージョンを確認します:

   ```shell
   helm search repo -l gitlab/gitlab-runner
   ```

1. GitLab Runnerの最新バージョンにアクセスできない場合は、次のコマンドでチャートを更新します:

   ```shell
   helm repo update gitlab
   ```

1. `values.yaml`ファイルでGitLab Runnerを[設定](#configure-gitlab-runner-with-the-helm-chart)したら、必要に応じてパラメータを変更して、次のコマンドを実行します:

   ```shell
   # For Helm 2
   helm install --namespace <NAMESPACE> --name gitlab-runner -f <CONFIG_VALUES_FILE> gitlab/gitlab-runner

   # For Helm 3
   helm install --namespace <NAMESPACE> gitlab-runner -f <CONFIG_VALUES_FILE> gitlab/gitlab-runner
   ```

   - `<NAMESPACE>`: GitLab RunnerをインストールするKubernetesネームスペース。
   - `<CONFIG_VALUES_FILE>`: カスタム設定を含む値ファイルのパス。作成するには、[HelmチャートでGitLab Runnerを設定する](#configure-gitlab-runner-with-the-helm-chart)を参照してください。
   - GitLab Runner Helmチャートの特定バージョンをインストールするには、`helm install`コマンドに`--version <RUNNER_HELM_CHART_VERSION>`を追加します。任意のバージョンのチャートをインストールできますが、新しい`values.yml`には古いバージョンのチャートとの互換性がない場合があります。

### 使用可能なGitLab Runner Helmチャートのバージョンを確認する {#check-available-gitlab-runner-helm-chart-versions}

HelmチャートとGitLab Runnerのバージョニング方法は異なります。この2つの間のバージョンマッピングを確認するには、ご使用のHelmのバージョンに対応するコマンドを実行します:

```shell
# For Helm 2
helm search -l gitlab/gitlab-runner

# For Helm 3
helm search repo -l gitlab/gitlab-runner
```

出力の例は次のとおりです:

```plaintext
NAME                  CHART VERSION APP VERSION DESCRIPTION
gitlab/gitlab-runner  0.64.0        16.11.0     GitLab Runner
gitlab/gitlab-runner  0.63.0        16.10.0     GitLab Runner
gitlab/gitlab-runner  0.62.1        16.9.1      GitLab Runner
gitlab/gitlab-runner  0.62.0        16.9.0      GitLab Runner
gitlab/gitlab-runner  0.61.3        16.8.1      GitLab Runner
gitlab/gitlab-runner  0.61.2        16.8.0      GitLab Runner
...
```

## Helmチャートを使用してGitLab Runnerをアップグレードする {#upgrade-gitlab-runner-with-the-helm-chart}

前提要件:

- GitLab Runnerチャートをインストールしていること。
- GitLabでRunnerを一時停止していること。これにより、[完了時の認証エラー](../faq/_index.md#helm-chart-error--unauthorized)など、ジョブで発生する問題を回避できます。
- すべてのジョブが完了していることを確認していること。

設定を変更するか、チャートを更新するには、必要に応じてパラメータを変更して`helm upgrade`を使用します:

```shell
helm upgrade --namespace <NAMESPACE> -f <CONFIG_VALUES_FILE> <RELEASE-NAME> gitlab/gitlab-runner
```

- `<NAMESPACE>`: GitLab RunnerをインストールしたKubernetesネームスペース。
- `<CONFIG_VALUES_FILE>`: カスタム設定を含む値ファイルのパス。作成するには、[HelmチャートでGitLab Runnerを設定する](#configure-gitlab-runner-with-the-helm-chart)を参照してください。
- `<RELEASE-NAME>`: チャートをインストールしたときに付けた名前。インストールセクションの例では`gitlab-runner`という名前が付けられています。
- GitLab Runner Helmチャートの最新バージョンではなく特定バージョンに更新するには、`helm upgrade`コマンドに`--version <RUNNER_HELM_CHART_VERSION>`を追加します。

## Helmチャートを使用してGitLab Runnerをアンインストールする {#uninstall-gitlab-runner-with-the-helm-chart}

GitLab Runnerをアンインストールするには、次の手順に従います:

1. GitLabでRunnerを一時停止し、すべてのジョブが完了していることを確認します。これにより、[完了時の認証エラー](../faq/_index.md#helm-chart-error--unauthorized)など、ジョブに関連する問題を回避できます。
1. このコマンドを実行します（必要に応じて変更します）:

   ```shell
   helm delete --namespace <NAMESPACE> <RELEASE-NAME>
   ```

   - `<NAMESPACE>`は、GitLab RunnerをインストールしたKubernetesネームスペースです。
   - `<RELEASE-NAME>`は、チャートをインストールしたときに付けた名前です。このページの[インストールセクション](#install-gitlab-runner-with-the-helm-chart)では、これは`gitlab-runner`でした。
