---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: 自己管理Runnerのセキュリティ
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab CI/CDパイプラインは、単純または複雑なDevOps自動化タスクに使用されるワークフロー自動化エンジンです。これらのパイプラインはリモートコード実行サービスを有効にするため、セキュリティリスクを軽減するために、以下のプロセスを実装する必要があります:

- テクノロジースタック全体のセキュリティを設定するための体系的なアプローチ。
- プラットフォームの設定と使用に関する継続的かつ厳格なレビュー。

自己管理Runner上でGitLab CI/CDジョブを実行する場合、コンピューティングインフラストラクチャとネットワークにセキュリティリスクが存在します。

RunnerはCI/CDジョブで定義されたコードを実行します。プロジェクトのリポジトリのデベロッパーロールを持つすべてのユーザーは、意図的であるかどうかにかかわらず、Runnerをホストする環境のセキュリティを侵害する可能性があります。

自己管理Runnerが一時的でなく、複数のプロジェクトに使用されている場合、このリスクはさらに高まります。

- 悪意のあるコードが埋め込まれたリポジトリからのジョブは、一時的でないRunnerがサービスを提供する他のリポジトリのセキュリティを侵害する可能性があります。
- executorによっては、ジョブはRunnerがホストされている仮想マシンに悪意のあるコードをインストールする可能性があります。
- 侵害された環境で実行されているジョブに公開されたシークレット変数トークン（`CI_JOB_TOKEN`を含むが、これに限定されない）が盗まれる可能性があります。
- デベロッパーロールを持つユーザーは、サブモジュールのアップストリームプロジェクトへのアクセス権を持っていなくても、プロジェクトに関連付けられたサブモジュールにアクセスできます。

## さまざまなexecutorのセキュリティリスク {#security-risks-for-different-executors}

使用しているexecutorによっては、さまざまなセキュリティリスクに直面する可能性があります。

### Shell executorの使用 {#usage-of-shell-executor}

**`shell`executorでビルドを実行すると、Runnerホストとネットワークに高いセキュリティリスクが存在します**。ジョブはGitLab Runnerのユーザーの権限で実行され、このサーバーで実行されている他のプロジェクトからコードを盗む可能性があります。信頼できるビルドを実行する場合にのみ使用してください。

### Docker executorの使用 {#usage-of-docker-executor}

**特権のないモードで実行する場合、Dockerは安全であると見なすことができます**。このような設定をより安全にするには、`sudo`を無効にするか、`SETUID`および`SETGID`機能を削除して、ルート以外のユーザーとしてDockerコンテナ内でジョブを実行します。

よりきめ細かいアクセスレベルは、`cap_add`/`cap_drop`設定を介して、特権のないモードで設定できます。

{{< alert type="warning" >}}

Dockerの特権コンテナは、ホストVMのすべてのルート機能を備えています。詳細については、[ランタイム特権とLinux機能](https://docs.docker.com/engine/containers/run/#runtime-privilege-and-linux-capabilities)に関する公式Dockerドキュメントをご覧ください

{{< /alert >}}

**特権モードでコンテナを実行することはお勧めしません**。

特権モードが有効になっている場合、CI/CDジョブを実行しているユーザーは、Runnerのホストシステムへの完全なルートアクセス権を取得し、ボリュームをマウントおよびデタッチするアクセスレベルを取得し、ネストされたコンテナを実行できます。

特権モードを有効にすると、すべてのコンテナのセキュリティメカニズムが効果的に無効になり、ホストが特権エスカレーションにさらされ、コンテナブレイクアウトが発生する可能性があります。

Docker Machine Executorを使用する場合は、`MaxBuilds = 1`設定を使用することを強くお勧めします。これにより、（特権モードによって導入されたセキュリティの脆弱性により侵害される可能性のある）単一のオートスケールVMが1つのジョブのみを処理するために使用されます。

### `if-not-present`プルポリシーでの非公開Dockerイメージの使用 {#usage-of-private-docker-images-with-if-not-present-pull-policy}

[高度な設定：プライベートコンテナレジストリの使用](../configuration/advanced-configuration.md#use-a-private-container-registry)で説明されているプライベートDockerイメージのサポートを使用する場合は、`always`を`pull_policy`値として使用する必要があります。特に、DockerまたはKubernetes executorを使用してパブリックインスタンスRunnerをホストしている場合は、`always`プルポリシーを使用する必要があります。

プルポリシーが`if-not-present`に設定されている例を考えてみましょう:

1. ユーザーAは、`registry.example.com/image/name`にプライベートイメージを持っています。
1. ユーザーAは、インスタンスRunnerでビルドを開始します: ビルドは、レジストリの認可後にレジストリ認証情報を受け取り、イメージをプルします。
1. イメージは、インスタンスRunnerのホストに保存されます。
1. ユーザーBは、`registry.example.com/image/name`のプライベートイメージにアクセスできません。
1. ユーザーBは、ユーザーAと同じインスタンスRunnerでこのイメージを使用するビルドを開始します: Runnerはイメージのローカルバージョンを見つけ、**イメージが認証情報の欠落によりプルできなかった場合でも**、それを使用します。

したがって、（プライベートとパブリックのアクセスレベルが混在する）さまざまなユーザーやさまざまなプロジェクトで使用できるRunnerをホストする場合は、`if-not-present`をプルポリシー値として使用しないでください。代わりに、以下を使用します:

- `never` - ユーザーが事前にダウンロードしたイメージのみを使用するように制限する場合。
- `always` - ユーザーにあらゆるレジストリからイメージをダウンロードする可能性を与えたい場合。

`if-not-present`プルポリシーは、信頼できるビルドおよびユーザーが使用する特定のRunnerに**のみ**使用する必要があります。

詳細については、[プルポリシーのドキュメント](../executors/docker.md#configure-how-runners-pull-images)をお読みください。

### SSH executorの使用 {#usage-of-ssh-executor}

`StrictHostKeyChecking`オプションがないため、**SSH executorは、MITM攻撃対象領域（中間者攻撃対象領域）を受けやすい**。これは、将来のリリースのいずれかで修正されます。

### Parallels executorの使用 {#usage-of-parallels-executor}

**Parallels executorは、完全なシステム仮想マシンを使用し、分離された仮想マシンで実行するように設定されたVMマシンを使用するため、可能な限り最も安全なオプションです**。すべての周辺機器と共有フォルダーへのアクセスをブロックします。

## Runnerの複製 {#cloning-a-runner}

Runnerはトークンを使用してGitLabサーバーを識別します。Runnerを複製すると、複製されたRunnerがそのトークンに対して同じジョブを取得する可能性があります。これは、Runnerジョブを「盗む」ための可能な脅威ベクターです。

## 共有環境で`GIT_STRATEGY: fetch`を使用する場合のセキュリティリスク {#security-risks-when-using-git_strategy-fetch-on-shared-environments}

[`GIT_STRATEGY`](https://docs.gitlab.com/ci/runners/configure_runners/#git-strategy)を`fetch`に設定すると、RunnerはGitリポジトリのローカル実行コピーを再利用しようとします。

ローカルバージョンを使用すると、CI/CDジョブのパフォーマンスを向上させることができます。ただし、その再利用可能なコピーへのアクセス権を持つすべてのユーザーは、他のユーザーのパイプラインで実行されるコードを追加できます。

Gitは、サブモジュール（別のリポジトリに埋め込まれたリポジトリ）の内容を親リポジトリのGit参照ログに格納します。その結果、プロジェクトのサブモジュールが最初にクローンされた後、後続のジョブは、スクリプトで`git submodule update`を実行することにより、サブモジュールのコンテンツにアクセスできます。これは、サブモジュールが削除され、ジョブを開始したユーザーがサブモジュールプロジェクトへのアクセス権を持っていない場合でも適用されます。

共有環境へのアクセス権を持つすべてのユーザーを信頼できる場合にのみ`GIT_STRATEGY: fetch`を使用してください。

## セキュリティ強化オプション {#security-hardening-options}

### 特権付きコンテナを使用するセキュリティリスクを軽減する {#reduce-the-security-risk-of-using-privileged-containers}

Dockerの`--privileged`フラグの使用を必要とするCI/CDジョブを実行する必要がある場合は、以下の手順を実行して、セキュリティリスクを軽減できます:

- `--privileged`フラグが有効になっているDockerコンテナは、分離された一時的な仮想マシンでのみ実行してください。
- Dockerの`--privileged`フラグの使用を必要とするジョブを実行するための専用のRunnerを設定します。次に、これらのRunnerを保護ブランチでのみジョブを実行するように設定します。

### ネットワークセグメンテーション {#network-segmentation}

GitLab Runnerは、ユーザーが制御するスクリプトを実行するように設計されています。ジョブが悪意のあるものである場合にアタックサーフェスを削減するために、独自のネットワークセグメントで実行することを検討できます。これにより、他のインフラストラクチャおよびサービスからのネットワーク分離が提供されます。

すべてのニーズは固有ですが、クラウドプロバイダー環境の場合、これには以下が含まれる可能性があります:

- 独自のネットワークセグメントでのRunner仮想マシンの設定
- インターネットからRunner仮想マシンへのSSHアクセスをブロックする
- Runner仮想マシン間のトラフィックを制限する
- クラウドプロバイダーメタデータエンドポイントへのアクセスをフィルタリングする

{{< alert type="note" >}}

すべてのRunnerは、GitLab.comまたはGitLabインスタンスへの送信ネットワーク接続を必要とします。ほとんどのジョブは、依存関係のプルなどのために、インターネットへの送信ネットワーク接続も必要とします。

{{< /alert >}}

### Runnerホストを保護する {#secure-the-runner-host}

Runnerに静的ホスト（ベアメタルまたは仮想マシン）を使用している場合は、ホストオペレーティングシステムのセキュリティのベストプラクティスを実装する必要があります。

CIジョブのコンテキストで実行される悪意のあるコードはホストを侵害する可能性があるため、セキュリティプロトコルは影響を軽減するのに役立ちます。留意すべきその他のポイントとしては、攻撃者が環境内の他のエンドポイントにアクセスできるようにする可能性のあるSSHキーなどのファイルをホストシステムから保護または削除することが挙げられます。

### 各ビルド後に`.git`フォルダーをクリーンアップする {#clean-up-the-git-folder-after-each-build}

Runnerに静的ホストを使用する場合は、`FF_ENABLE_JOB_CLEANUP` [機能フラグ](../configuration/feature-flags.md)を有効にすることで、セキュリティのレイヤーを追加できます。

`FF_ENABLE_JOB_CLEANUP`を有効にすると、Runnerがホストで使用するビルドディレクトリが各ビルド後にクリーンアップされます。
