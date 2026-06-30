---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: セルフマネージドRunnerのセキュリティ
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab CI/CDパイプラインは、シンプルまたは複雑なDevOps自動化タスクに使用されるワークフロー自動化エンジンです。これらのパイプラインはリモートコード実行サービスを可能にするため、セキュリティリスクを軽減するために以下のプロセスを実装する必要があります:

- 技術スタック全体のセキュリティを設定するための体系的なアプローチ。
- プラットフォームの設定と使用に関する継続的かつ厳格なレビュー。

セルフマネージドRunnerでGitLab CI/CDジョブを実行する予定がある場合、コンピューティングインフラとネットワークにセキュリティリスクが存在します。

RunnerはCI/CDジョブで定義されたコードを実行します。プロジェクトのリポジトリに対してデベロッパーロールを持つユーザーは、意図的であるか否かにかかわらず、Runnerをホストする環境のセキュリティを危険にさらす可能性があります。

セルフマネージドRunnerが非一時的なもので、複数のプロジェクトで使用されている場合、このリスクはさらに深刻になります。

- 悪意のあるコードが埋め込まれたリポジトリからのジョブは、非一時的なRunnerによってサービスされる他のリポジトリのセキュリティを危険にさらす可能性があります。
- executorによっては、ジョブがRunnerがホストされている仮想マシンに悪意のあるコードをインストールする可能性があります。
- 侵害された環境で実行されているジョブに公開されたシークレット変数は、`CI_JOB_TOKEN`などが盗まれる可能性があります。
- デベロッパーロールを持つユーザーは、サブモジュールのアップストリームプロジェクトへのアクセス権がなくても、プロジェクトに関連付けられたサブモジュールにアクセスできます。

## 異なるexecutorに対するセキュリティリスク {#security-risks-for-different-executors}

使用しているexecutorに応じて、異なるセキュリティリスクに直面する可能性があります。

### Shellexecutorの使用 {#usage-of-shell-executor}

**`shell` executorでビルドを実行すると、Runnerホストとネットワークに高いセキュリティリスクが存在します**。ジョブはGitLab Runnerのユーザー権限で実行され、このサーバーで実行される他のプロジェクトからコードを盗む可能性があります。信頼されたビルドの実行のみに使用してください。

### Dockerexecutorの使用 {#usage-of-docker-executor}

**Docker can be considered safe when running in non-privileged mode**。このような設定をより安全にするには、`sudo`を無効にするか、`SETUID`および`SETGID`の機能を削除したDockerコンテナで非ルートユーザーとしてジョブを実行します。

非特権モードでは、`cap_add`/`cap_drop`設定を介して、より詳細な権限を設定できます。

> [!warning]
> Dockerの特権コンテナは、ホストVMのすべてのルート権限を持っています。詳細については、Dockerの公式ドキュメント「[Runtime privilege and Linux capabilities](https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities)」を確認してください。同様に、コンテナをホストPIDネームスペースで実行すると、コンテナの分離が破られ、安全ではありません。

特権モード、または`--pid=host`フラグを使用してコンテナを実行することは**推奨されません**。

特権モードが有効になっている場合、CI/CDジョブを実行しているユーザーは、Runnerのホストシステムへの完全なルートアクセスレベル、ボリュームのマウントとデタッチの権限、およびネストされたコンテナを実行する権限を取得する可能性があります。

特権モードを有効にすると、コンテナのすべてのセキュリティメカニズムが実質的に無効になり、ホストが特権昇格にさらされ、コンテナのブレイクアウトにつながる可能性があります。

Docker Machine Executorを使用する場合は、`MaxBuilds = 1`設定を使用することを強くお勧めします。これにより、特権モードによって導入されたセキュリティの弱点のために潜在的に侵害された単一のオートスケールされたVMが、1つのジョブのみを処理するために使用されることが保証されます。

### `if-not-present`プルポリシーでのプライベートDockerイメージの使用 {#usage-of-private-docker-images-with-if-not-present-pull-policy}

[詳細設定: プライベートコンテナレジストリの使用](../configuration/advanced-configuration.md#use-a-private-container-registry)で説明されているプライベートDockerイメージのサポートを使用する場合、`pull_policy`の値として`always`を使用する必要があります。DockerまたはKubernetesexecutorを使用してパブリックなインスタンスRunnerをホストしている場合は、特に`always`プルポリシーを使用する必要があります。

プルポリシーが`if-not-present`に設定されている例を考えてみましょう:

1. ユーザーAは`registry.example.com/image/name`にプライベートイメージを持っています。
1. ユーザーAがインスタンスRunnerでビルドを開始します: ビルドはレジストリ認証情報を受け取り、レジストリでの認可後にイメージをプルします。
1. イメージはインスタンスRunnerのホストに保存されます。
1. ユーザーBは`registry.example.com/image/name`にあるプライベートイメージへのアクセスレベルを持っていません。
1. ユーザーBは、ユーザーAと同じインスタンスRunnerでこのイメージを使用するビルドを開始します: Runnerはイメージのローカルバージョンを見つけ、**even if the image could not be pulled because of missing credentials**それを使用します。

したがって、異なるユーザーや異なるプロジェクト（プライベートとパブリックのアクセスレベルが混在）が使用できるRunnerをホストしている場合、プルポリシーの値として`if-not-present`を使用せず、次を使用してください:

- `never` - ユーザーが事前にダウンロードしたイメージのみを使用するように制限したい場合。
- `always` - ユーザーにあらゆるレジストリから任意のイメージをダウンロードする可能性を与えたい場合。

The `if-not-present`プルポリシーは、信頼できるビルドとユーザーが使用する特定のRunnerに対して**only**使用する必要があります。

詳細については、[プルポリシードキュメント](../executors/docker.md#configure-how-runners-pull-images)を読んでください。

### SSHexecutorの使用 {#usage-of-ssh-executor}

**SSH executors are susceptible to MITM attack (man-in-the-middle)**のは、`StrictHostKeyChecking`オプションが不足しているためです。これは今後のリリースで修正される予定です。

### Parallels executorの使用 {#usage-of-parallels-executor}

**Parallels executor is the safest possible option**。なぜなら、完全なシステム仮想化を使用し、分離された仮想化で実行するように設定されたVMマシンと、分離モードで実行するように設定されたVMマシンを使用するからです。すべての周辺機器と共有フォルダーへのアクセスレベルをブロックします。

## Runnerのクローン {#cloning-a-runner}

RunnerはトークンをGitLabサーバーに識別するために使用します。Runnerをクローンすると、クローンされたRunnerがそのトークンに対して同じジョブを取得する可能性があります。これはRunnerジョブを「盗む」ための可能性のある脅威ベクターです。

## 共有環境で`GIT_STRATEGY: fetch`を使用する場合のセキュリティリスク {#security-risks-when-using-git_strategy-fetch-on-shared-environments}

[`GIT_STRATEGY`](https://docs.gitlab.com/ci/runners/configure_runners/#git-strategy)を`fetch`に設定すると、RunnerはGitリポジトリのローカルコピーを再利用しようとします。

ローカルコピーを使用すると、CI/CDジョブのパフォーマンスを向上させることができます。ただし、その再利用可能なコピーへのアクセスレベルを持つユーザーは、他のユーザーのパイプラインで実行されるコードを追加できます。

Gitはサブモジュールのコンテンツ（別のリポジトリ内に埋め込まれたリポジトリ）を、親リポジトリのGit参照ログに保存します。その結果、プロジェクトのサブモジュールが最初にクローンされた後、後続のジョブは、スクリプトで`git submodule update`を実行することにより、サブモジュールのコンテンツにアクセスレベルできます。これは、サブモジュールが削除され、ジョブを開始したユーザーがサブモジュールプロジェクトへのアクセスレベルを持っていなくても適用されます。

共有環境へのアクセスレベルを持つすべてのユーザーを信頼する場合にのみ`GIT_STRATEGY: fetch`を使用してください。

## セキュリティ強化オプション {#security-hardening-options}

### 特権コンテナを使用するセキュリティリスクを軽減する {#reduce-the-security-risk-of-using-privileged-containers}

Dockerの`--privileged`フラグの使用を必要とするCI/CDジョブを実行する必要がある場合は、セキュリティリスクを軽減するために次の手順を実行できます:

- `--privileged`フラグを有効にしたDockerコンテナは、分離された一時的な仮想マシンでのみ実行してください。
- Dockerの`--privileged`フラグの使用を必要とするジョブを実行するために設計された専用のRunnerを設定します。次に、これらのRunnerが保護ブランチでのみジョブを実行するように設定します。

### ネットワークセグメンテーション {#network-segmentation}

GitLab Runnerは、ユーザーが制御するスクリプトを実行するように設計されています。ジョブが悪意のあるものである場合、アタックサーフェスを軽減するために、独自のネットワークセグメントで実行することを検討できます。これにより、他のインフラストラクチャやサービスからネットワークを分離できます。

すべてのニーズは固有ですが、クラウドプロバイダー環境の場合、これには次のものが含まれます:

- Runner仮想マシンを独自のネットワークセグメントで設定する
- インターネットからRunner仮想マシンへのSSHアクセスレベルをブロックする
- Runner仮想マシン間のトラフィックを制限する
- クラウドプロバイダーメタデータエンドポイントへのアクセスレベルをフィルタリングする

> [!note]
> すべてのRunnerは、GitLab.comまたはGitLabインスタンスへの送信ネットワーク接続を必要とします。ほとんどのジョブは、インターネットへの送信ネットワーク接続も必要とします（依存プルなど）。

### Runnerホストを安全にする {#secure-the-runner-host}

Runnerに静的ホストを使用している場合、それがベアメタルであろうと仮想マシンであろうと、ホストのオペレーティングシステムに対するセキュリティのベストプラクティスを実装する必要があります。

CIジョブのコンテキストで実行される悪意のあるコードはホストを危険にさらす可能性があるため、セキュリティプロトコルはその影響を軽減するのに役立ちます。その他の留意点としては、SSHキーなどのファイルをホストシステムから保護または削除し、攻撃者が環境内の他のエンドポイントにアクセスレベルできるようになるのを防ぐことが挙げられます。

### 各ビルド後に`.git`フォルダーをクリーンアップする {#clean-up-the-git-folder-after-each-build}

Runnerに静的ホストを使用する場合、`FF_ENABLE_JOB_CLEANUP` [機能フラグ](../configuration/feature-flags.md)を有効にすることで、追加のレイヤーのセキュリティを実装できます。

`FF_ENABLE_JOB_CLEANUP`を有効にすると、ホストでRunnerが使用するビルドディレクトリは、各ビルド後にクリーンアップされます。
