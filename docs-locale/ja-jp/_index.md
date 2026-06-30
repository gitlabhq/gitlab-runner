---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Runner
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerは、GitLab CI/CDと連携してパイプラインでジョブを実行するアプリケーションです。

開発者がGitLabにコードをプッシュすると、`.gitlab-ci.yml`ファイルに自動化されたタスクを定義できます。これらのタスクには、テストの実行、アプリケーションのビルド、またはコードのデプロイが含まれる場合があります。GitLab Runnerは、これらのタスクをコンピューティングインフラストラクチャで実行するアプリケーションです。

管理者として、これらのCI/CDジョブが実行されるインフラストラクチャの提供と管理を担当します。これには、GitLab Runnerアプリケーションのインストール、設定、および組織のCI/CDワークロードを処理するのに十分な容量を確保することが含まれます。

## GitLab Runnerの機能 {#what-gitlab-runner-does}

GitLab Runnerは、GitLabインスタンスに接続し、CI/CDジョブを待機します。パイプラインが実行されると、GitLabは利用可能なRunnerにジョブを送信します。Runnerはジョブを実行し、結果をGitLabに報告します。

GitLab Runnerには次の機能があります。

- 複数のジョブを同時に実行する。
- 複数のサーバーで複数のトークンを使用する（プロジェクトごとにも可能）。
- トークンあたりの同時実行ジョブの数を制限する。
- ジョブを次のいずれかの方法で実行する。
  - ローカル環境での実行
  - Dockerコンテナを使用する
  - Dockerコンテナを使用し、SSH経由でジョブを実行する
  - 各種クラウドや仮想マシンハイパーバイザーでオートスケールとDockerコンテナを使用する
  - リモートSSHサーバーに接続する
- Go言語で記述され、他の要件のない単一バイナリとして配布される。
- Bash、PowerShell Core、およびWindows PowerShellをサポートする。
- GNU/Linux、macOS、およびWindows（Dockerを実行できる環境）で動作する。
- ジョブ実行環境のカスタマイズが可能。
- 再起動なしで設定を自動的に再読み込みする。
- Docker、Docker-SSH、Parallels、またはSSH実行環境をサポートするシームレスなセットアップ。
- Dockerコンテナのキャッシュを有効にする。
- GNU/Linux、macOS、およびWindows用のサービスとしてのシームレスなインストール。
- PrometheusメトリクスHTTPサーバーを搭載。
- Prometheusメトリクスやその他のジョブ固有のデータをモニタリングし、GitLabに送信するレフェリーワーカー機能。

## Runnerの実行フロー {#runner-execution-flow}

次の図は、Runnerが登録される仕組みと、ジョブがリクエストおよび処理される仕組みを示しています。また、[登録および認証トークン](https://docs.gitlab.com/api/runners/#registration-and-authentication-tokens)、および[ジョブトークン](https://docs.gitlab.com/ci/jobs/ci_job_token/)を使用するアクションも示します。

```mermaid
sequenceDiagram
    participant GitLab
    participant GitLabRunner
    participant Executor

    opt registration
      GitLabRunner ->>+ GitLab: POST /api/v4/runners with registration_token
      GitLab -->>- GitLabRunner: Registered with runner_token
    end

    loop job requesting and handling
      GitLabRunner ->>+ GitLab: POST /api/v4/jobs/request with runner_token
      GitLab -->>+ GitLabRunner: job payload with job_token
      GitLabRunner ->>+ Executor: Job payload
      Executor ->>+ GitLab: clone sources with job_token
      Executor ->>+ GitLab: download artifacts with job_token
      Executor -->>- GitLabRunner: return job output and status
      GitLabRunner -->>- GitLab: updating job output and status with job_token
    end
```

## Runnerのデプロイオプション {#runner-deployment-options}

### GitLabホストRunner {#gitlab-hosted-runners}

[GitLabホスト型Runner](https://docs.gitlab.com/ci/runners/)は、GitLabによって管理され、GitLab.comで利用できます。これらのRunnerをインストールまたは維持する必要はありません。GitLabはこれらをサービスとして提供します。ただし、実行環境に対する制御は限られており、インフラストラクチャをカスタマイズすることはできません。

### Self-Managed Runner {#self-managed-runners}

Self-Managed Runnerは、各自のインフラストラクチャでインストール、設定および管理するGitLab Runnerインスタンスです。すべてのGitLabインストールでSelf-Managed Runnerを[インストール](install/_index.md)して登録できます。管理者として、通常はセルフマネージドRunnerを扱います。

GitLabがホストおよび管理するGitLabホスト型Runnerとは異なり、セルフマネージドRunnerを完全に制御できます。

## GitLab Runnerのバージョン {#gitlab-runner-versions}

互換性の理由から、GitLab Runnerの[major.minor](https://en.wikipedia.org/wiki/Software_versioning)（メジャー.マイナー）バージョンは、GitLabのメジャーバージョンおよびマイナーバージョンと同期している必要があります。古いバージョンのRunnerが、新しいGitLabバージョンでも動作する可能性があります（またはその逆でも動作する可能性があります）。ただし、バージョンが異なる場合、一部の機能が利用できなかったり、正常に動作しなかったりする可能性があります。

マイナーバージョンの更新間では、下位互換性が保証されています。ただし、GitLabのマイナーバージョンアップデートで新機能が追加されると、その機能を利用するにはGitLab Runnerも同じマイナーバージョンにアップデートしなければならない場合もあります。

独自のRunnerをホストしながらGitLab.comでリポジトリをホストしている場合は、GitLab.comが[継続的に更新される](https://handbook.gitlab.com/handbook/engineering/deployments-and-releases/)ため、常にGitLab Runnerを最新バージョンに[更新](install/_index.md)してください。

## トラブルシューティング {#troubleshooting}

一般的な問題を[解決する](faq/_index.md)方法について説明します。

## 用語集 {#glossary}

- **GitLab Runner**: GitLabパイプラインからのCI/CDジョブを、ターゲットとなるコンピューティングプラットフォームで実行するアプリケーション。
- **Runner**: 設定されたGitLab Runnerのインスタンスで、ジョブを実行できるもの。executorのタイプに応じて、このマシンはRunnerマネージャーのローカル（`shell` executorまたは`docker` executor）であるか、またはオートスケーラーによって作成されたリモートマシン（`docker-autoscaler`または`kubernetes`）になります。
- **Runner設定**: UIに**Runner**として表示される`config.toml`の単一`[[runner]]`エントリ。
- **Runner manager**: `config.toml`ファイルを読み込み、すべてのRunnerの設定とジョブの実行を並行して行うプロセス。
- **Machine**: Runnerが動作する仮想マシン（VM）またはポッド。GitLab Runnerは、一意の永続的なマシンIDを自動的に生成します。このため、複数のマシンに同じRunner設定が指定されている場合でも、ジョブは個別にルーティングされますが、Runner設定はUIでグループ化されます。
- **Executor**: GitLab Runnerがジョブを実行する方法（Docker、Shell、Kubernetesなど）。
- **パイプライン**: コードがGitLabにプッシュされると自動的に実行されるジョブの集まり。
- **ジョブ**: パイプライン内の単一のタスクで、テストの実行やアプリケーションのビルドなどです。
- **Runner token**: RunnerがGitLabで認証することを可能にする固有識別子。
- **タグ**: Runnerに割り当てられるラベルで、実行できるジョブを決定します。
- **Concurrent jobs**: Runnerが同時に実行できるジョブの数。
- **Self-managed runner**: 独自のインフラストラクチャにインストールされ、管理されるRunner。
- **GitLab-hosted runner**: GitLabによって提供および管理されるRunner。

詳細については、公式の[GitLab用語リスト](https://docs.gitlab.com/development/documentation/styleguide/word_list/#gitlab-runner)およびGitLabアーキテクチャの[GitLab Runner](https://docs.gitlab.com/development/architecture/#gitlab-runner)のエントリを参照してください。

## コントリビュート {#contributing}

コントリビュートを歓迎します。詳細については、[`CONTRIBUTING.md`](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/CONTRIBUTING.md)と[開発ドキュメント](development/_index.md)を参照してください。

GitLab Runnerプロジェクトのレビュアーの方は、[GitLab Runnerのレビュー](development/reviewing-gitlab-runner.md)に関するドキュメントをお読みください。

[GitLab Runnerプロジェクトのリリースプロセス](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/PROCESS.md)を確認することもできます。

## 変更履歴 {#changelog}

最近の変更を確認するには、[CHANGELOG](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/CHANGELOG.md)（変更履歴）を参照してください。

## ライセンス {#license}

このコードは、MITライセンスに従って配布されます。[LICENSE](https://gitlab.com/gitlab-org/gitlab-runner/blob/main/LICENSE)（ライセンス）ファイルをご確認ください。
