---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runnerのトラブルシューティング
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

このセクションは、GitLab Runnerの問題を解決する際に役立ちます。

## 一般的なトラブルシューティングのヒント {#general-troubleshooting-tips}

### ログを表示する {#view-the-logs}

GitLab Runnerサービスはログをsyslogに送信します。ログを表示するには、ディストリビューションのドキュメントを参照してください。ディストリビューションに`journalctl`コマンドが含まれている場合は、そのコマンドを使用してログを表示できます。

```shell
journalctl --unit=gitlab-runner.service -n 100 --no-pager
docker logs gitlab-runner-container # Docker
kubectl logs gitlab-runner-pod # Kubernetes
```

### サービスを再起動する {#restart-the-service}

```shell
systemctl restart gitlab-runner.service
```

### Docker Machineを表示する {#view-the-docker-machines}

```shell
sudo docker-machine ls
sudo su - && docker-machine ls
```

### すべてのDocker Machineを削除する {#delete-all-docker-machines}

```shell
docker-machine rm $(docker-machine ls -q)
```

### `config.toml`に変更を適用する {#apply-changes-to-configtoml}

```shell
systemctl restart gitlab-runner.service
docker-machine rm $(docker-machine ls -q) # Docker machine
journalctl --unit=gitlab-runner.service -f # Tail the logs to check for potential errors
```

## GitLabおよびGitLab Runnerのバージョンを確認する {#confirm-your-gitlab-and-gitlab-runner-versions}

GitLabは[下位互換性を保証](../_index.md#gitlab-runner-versions)することを目標としています。ただし、最初のトラブルシューティング手順として、GitLab RunnerのバージョンがGitLabのバージョンと同じであることを確認する必要があります。

## `coordinator`について {#what-does-coordinator-mean}

`coordinator`は、ジョブのリクエスト元であるGitLabインストールのことです。

つまりRunnerは、`coordinator`（GitLab APIを介したGitLabインストール）からジョブをリクエストする、分離されたエージェントです。

## Windowsでサービスとして実行する場合にログはどこに保存されますか？ {#where-are-logs-stored-when-run-as-a-service-on-windows}

- GitLab RunnerがWindowsでサービスとして実行されている場合、システムイベントログが作成されます。これらを表示するには、イベントビューアーを開きます（「ファイル名を指定して実行」メニューで`eventvwr.msc`と入力するか、「イベントビューアー」を検索します）。次に、**Windows Logs > Application**に移動します。Runnerログの**ソース**は`gitlab-runner`です。Windows Server Coreを使用している場合は、PowerShellコマンド`get-eventlog Application -Source gitlab-runner -Newest 20 | format-table -wrap -auto`を実行して、最後の20件のログエントリを取得します。

## デバッグログ生成モードを有効にする {#enable-debug-logging-mode}

{{< alert type="warning" >}}

デバッグログ生成は、重大なセキュリティリスクとなる可能性があります。出力には、ジョブで使用可能なすべての変数およびその他のシークレットの内容が含まれます。サードパーティにシークレットを送信する可能性のあるログ集計はすべて無効にする必要があります。マスクされた変数を使用すると、ジョブログ出力ではシークレットを保護できますが、コンテナログでは保護できません。

{{< /alert >}}

### コマンドライン {#in-the-command-line}

rootとしてログインしたターミナルから、以下を実行します。

{{< alert type="warning" >}}

このコマンドは`systemd`サービスを再定義し、すべてのジョブをrootとして実行するため、[Shell executor](../executors/shell.md)を使用するRunnerでは実行しないでください。これはセキュリティ上のリスクをもたらし、特権なしのアカウントに戻すことが困難になるファイル所有権の変更につながります。

{{< /alert >}}

```shell
gitlab-runner stop
gitlab-runner --debug run
```

### GitLab Runner `config.toml`内 {#in-the-gitlab-runner-configtoml}

デバッグログ生成を有効にするには、[`config.toml`のグローバルセクション](../configuration/advanced-configuration.md#the-global-section)で`log_level`を`debug`に設定します。`config.toml`の最上部で、concurrent行の前または後に次の行を追加します。

```toml
log_level = "debug"
```

### Helmチャート内 {#in-the-helm-chart}

[GitLab Runner Helmチャート](../install/kubernetes.md)を使用してKubernetesクラスターにGitLab Runnerがインストールされている場合、デバッグログ生成を有効にするには、[`values.yaml`のカスタマイズ](../install/kubernetes.md#configure-gitlab-runner-with-the-helm-chart)で`logLevel`オプションを設定します。

```yaml
## Configure the GitLab Runner logging level. Available values are: debug, info, warn, error, fatal, panic
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration/#the-global-section
##
logLevel: debug
```

## Docker executor RunnerのDNSを設定する {#configure-dns-for-a-docker-executor-runner}

Docker executorでGitLab Runnerを設定すると、ホストRunnerデーモンがGitLabにアクセスできてもDockerコンテナがアクセスできない場合があります。これは、ホストでDNSが設定されていても、その設定がコンテナに渡されない場合に発生する可能性があります。

**例**: 

GitLabサービスとGitLab Runnerが、2種類の方法（インターネット経由とVPN経由など）でブリッジされる2つの異なるネットワークに存在しています。Runnerのルーティングメカニズムでは、VPN経由のDNSサービスではなく、デフォルトのインターネットサービスを介してDNSをクエリする可能性があります。この設定を使用すると、次のメッセージが表示されます。

```shell
Created fresh repository.
++ echo 'Created fresh repository.'
++ git -c 'http.userAgent=gitlab-runner 16.5.0 linux/amd64' fetch origin +da39a3ee5e6b4b0d3255bfef95601890afd80709:refs/pipelines/435345 +refs/heads/master:refs/remotes/origin/master --depth 50 --prune --quiet
fatal: Authentication failed for 'https://gitlab.example.com/group/example-project.git/'
```

この場合の認証の失敗の原因は、インターネットとGitLabサービスの間にあるサービスにあります。このサービスは個別の認証情報を使用しており、RunnerがVPN経由でDNSサービスを使用した場合は、Runnerがそれらの認証情報を回避している可能性があります。

使用するDNSサーバーをDockerに指示するには、[Runnerの`config.toml`ファイル](../configuration/advanced-configuration.md#the-runnersdocker-section)の`[runners.docker]`セクションで`dns`設定を使用します。

```toml
dns = ["192.168.xxx.xxx","192.168.xxx.xxx"]
```

## `x509: certificate signed by unknown authority`が表示される {#im-seeing-x509-certificate-signed-by-unknown-authority}

詳細については、[自己署名証明書](../configuration/tls-self-signed.md)を参照してください。

## `/var/run/docker.sock`へアクセスするときに`Permission Denied`が表示される {#i-get-permission-denied-when-accessing-the-varrundockersock}

Docker executorを使用する場合に、サーバーにインストールされているDocker Engineに接続しているとします。この場合には`Permission Denied`エラーが表示されることがあります。最も可能性が高い原因は、システムがSELinuxを使用していることです（CentOS、Fedora、RHELではデフォルトで有効になっています）。システムでSELinuxポリシーを調べて、拒否がないか確認してください。

## Docker-machineエラー: `Unable to query docker version: Cannot connect to the docker engine endpoint.` {#docker-machine-error-unable-to-query-docker-version-cannot-connect-to-the-docker-engine-endpoint}

このエラーはマシンのプロビジョニングに関連しており、次の原因が考えられます。

- TLSエラーが発生している。`docker-machine`がインストールされている場合、一部の証明書が無効になっている可能性があります。このイシューを解決するには、証明書を削除してRunnerを再起動します。

  ```shell
  sudo su -
  rm -r /root/.docker/machine/certs/*
  service gitlab-runner restart
  ```

  再起動したRunnerは、証明書が空であると認識し、証明書を再作成します。

- ホスト名が、プロビジョニングされたマシンでサポートされている長さを超えている。たとえば、Ubuntuマシンでの`HOST_NAME_MAX`の文字数制限は64文字です。ホスト名は`docker-machine ls`によって報告されます。Runner設定で`MachineName`を確認し、必要に応じてホスト名を短くします。

{{< alert type="note" >}}

このエラーは、Dockerがマシンにインストールされる前に発生していた可能性があります。

{{< /alert >}}

## `dialing environment connection: ssh: rejected: connect failed (open failed)` {#dialing-environment-connection-ssh-rejected-connect-failed-open-failed}

このエラーは、SSH経由で接続をトンネルしているときに、Docker autoscalerがターゲットシステムのDockerデーモンに到達できない場合に発生します。ターゲットシステムにSSHで接続し、`docker info`などのDockerコマンドを正常に実行できることを確認します。

## オートスケールされたRunnerにAWSインスタンスプロファイルを追加する {#adding-an-aws-instance-profile-to-your-autoscaled-runners}

AWS IAMロールを作成した後、IAMコンソールではそのロールに**ロールARN**と**インスタンスプロファイルARN**があります。**ロール名****ではなく****インスタンスプロファイル**名を使用する必要があります。

`[runners.machine]`セクションに値`"amazonec2-iam-instance-profile=<instance-profile-name>",`を追加します。

## Javaプロジェクトのビルド時にDocker executorがタイムアウトになる {#the-docker-executor-gets-timeout-when-building-java-project}

最も可能性が高い原因は、破損した`aufs`ストレージドライバーです。[Javaプロセスがコンテナ内でハングアップ](https://github.com/moby/moby/issues/18502)します。最適な解決策は、[ストレージドライバー](https://docs.docker.com/engine/storage/drivers/select-storage-driver/)をOverlayFS（高速）またはDeviceMapper（低速）のいずれかに変更することです。

[Dockerの設定と実行に関する記事](https://docs.docker.com/engine/daemon/)、または[systemdによる制御と設定に関する記事](https://docs.docker.com/engine/daemon/proxy/#systemd-unit-file)を確認してください。

## アーティファクトのアップロード時に411が表示される {#i-get-411-when-uploading-artifacts}

GitLab Runnerが`Transfer-Encoding: chunked`を使用していることが原因で発生します。これは、以前のバージョンのNGINXで破損しています（<https://serverfault.com/questions/164220/is-there-a-way-to-avoid-nginx-411-content-length-required-errors>）。

NGINXを新しいバージョンにアップグレードしてください。詳細については、イシュー<https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1031>を参照してください。

## 他のアーティファクトのアップロードエラーが発生しています。このエラーを詳しくデバッグするにはどうすればよいですか？ {#i-am-seeing-other-artifact-upload-errors-how-can-i-further-debug-this}

アーティファクトは、GitLab Runnerプロセスを回避して、ビルド環境からGitLabインスタンスに直接アップロードされます。次に例を示します。

- Docker executorの場合、アップロードはDockerコンテナから行われます
- Kubernetes executorの場合、アップロードはビルドポッドのビルドコンテナから行われます

ビルド環境からGitLabインスタンスへのネットワークルートは、GitLab RunnerからGitLabインスタンスへのルートとは異なる場合があります。

アーティファクトのアップロードを有効にするには、アップロードパス内のすべてのコンポーネントが、ビルド環境からGitLabインスタンスへのPOSTリクエストを許可していることを確認します。

デフォルトでは、アーティファクトアップローダーはアップロードURLとアップロード応答のHTTPステータスコードをログに記録します。この情報だけでは、どのシステムがエラーを引き起こしたか、またはアーティファクトのアップロードをブロックしたかを理解するには不十分です。アーティファクトのアップロードの問題を解決するには、アップロード応答のヘッダーと本文を確認するために、アップロード試行で[デバッグログ生成を有効にします](https://docs.gitlab.com/ci/variables/#enable-debug-logging)。

{{< alert type="note" >}}

アーティファクトのアップロードのデバッグログの応答本文の長さは、512バイトに制限されています。機密データがログに公開される可能性があるため、ログ生成はデバッグ目的でのみ有効にしてください。

{{< /alert >}}

アップロードがGitLabに到達してもエラー状態コードで失敗する場合（たとえば、成功以外の応答ステータスコードが生成される場合）は、GitLabインスタンス自体を調べます。一般的なアーティファクトのアップロードのイシューについては、[GitLabドキュメント](https://docs.gitlab.com/administration/cicd/job_artifacts_troubleshooting/#job-artifact-upload-fails-with-error-500)を参照してください。

## `No URL provided, cache will not be download`/`uploaded` {#no-url-provided-cache-will-not-be-downloaduploaded}

このエラーは、GitLab Runnerヘルパーが無効なURLを受信するか、リモートキャッシュにアクセスするための事前署名付きURLがない場合に発生します。[`config.toml`のキャッシュ関連のエントリ](../configuration/advanced-configuration.md#the-runnerscache-section)と、プロバイダー固有のキーと値を確認します。URL構文の要件に従っていないアイテムから無効なURLが作成される可能性があります。

また、ヘルパー`image`と`helper_image_flavor`が一致し、最新であることを確認してください。

認証情報の設定に問題がある場合は、診断エラーメッセージがGitLab Runnerプロセスログに追加されます。

## エラー: `warning: You appear to have cloned an empty repository.` {#error-warning-you-appear-to-have-cloned-an-empty-repository}（ヘルプページドキュメントのべースURLがブロックされています: 実行が期限切れです）

HTTP(S)を使用して`git clone`を実行すると（GitLab Runnerを使用するか、テスト用に手動で実行）、次の出力が表示されます。

```shell
$ git clone https://git.example.com/user/repo.git

Cloning into 'repo'...
warning: You appear to have cloned an empty repository.
```

GitLabサーバーのインストールでHTTPプロキシ設定が正しく行われていることを確認してください。独自の設定でHTTPプロキシを使用する場合は、リクエストが**GitLab Workhorseソケット**ではなく**GitLab Unicornソケット**にプロキシされることを確認してください。

HTTP(S)を介したGitプロトコルはGitLab Workhorseによって解決されるため、これはGitLabの**メインエントリポイント**です。

Linuxパッケージのインストールを使用しているが、バンドルされているNGINXサーバーを使用したくない場合は、[バンドルされていないWebサーバーを使用する](https://docs.gitlab.com/omnibus/settings/nginx/#use-a-non-bundled-web-server)を参照してください。

GitLabレシピリポジトリには、ApacheとNGINXの[Webサーバー設定の例](https://gitlab.com/gitlab-org/gitlab-recipes/tree/master/web-server)があります。

ソースからインストールされたGitLabを使用している場合は、上記のドキュメントと例を参照してください。すべてのHTTP(S)トラフィックが**GitLab Workhorse**を経由していることを確認してください。

[ユーザーイシューの例](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1105)を参照してください。

## エラー: `Timezone`または`OffPeakTimezone`の使用時に`zoneinfo.zip: no such file or directory`エラーが発生する {#error-zoneinfozip-no-such-file-or-directory-error-when-using-timezone-or-offpeaktimezone}

`[[docker.machine.autoscaling]]`の期間が記述されているタイムゾーンを設定できます。この機能は、ほとんどのUnixシステムですぐに動作するはずです。ただし、一部のUnixシステムとほとんどの非Unixシステム（GitLab Runnerバイナリが利用可能なWindowsなど）では、Runnerが起動時に次のエラーでクラッシュする可能性があります。

```plaintext
Failed to load config Invalid OffPeakPeriods value: open /usr/local/go/lib/time/zoneinfo.zip: no such file or directory
```

このエラーは、Goの`time`パッケージが原因で発生します。GoはIANA Time Zoneデータベースを使用して、指定されたタイムゾーンの設定を読み込みます。ほとんどのUnixシステムでは、このデータベースは、既知のパス（`/usr/share/zoneinfo`、`/usr/share/lib/zoneinfo`、`/usr/lib/locale/TZ/`）のいずれかにすでに存在しています。Goの`time`パッケージは、これら3つのパスすべてでTime Zoneデータベースを検索します。いずれも見つからないが、マシンに設定済みのGo開発環境がある場合は、`$GOROOT/lib/time/zoneinfo.zip`ファイルにフォールバックします。

これらのパスがいずれも存在しない場合（本番環境のWindowsホスト上など）は、上記のエラーがスローされます。

システムがIANA Time Zoneデータベースをサポートしているが、デフォルトでは利用できない場合は、このデータベースをインストールしてみることができます。Linuxシステムでは、次のような方法でこのインストールを実行できます。

```shell
# on Debian/Ubuntu based systems
sudo apt-get install tzdata

# on RPM based systems
sudo yum install tzdata

# on Linux Alpine
sudo apk add -U tzdata
```

システムがこのデータベースを_ネイティブ_な方法で提供していない場合は、次の手順に従って`OffPeakTimezone`を動作させることができます。

1. [`zoneinfo.zip`](https://gitlab-runner-downloads.s3.amazonaws.com/latest/zoneinfo.zip)をダウンロードします。バージョンv9.1.0以降では、タグ付けされたパスからファイルをダウンロードできます。この場合は、`zoneinfo.zip`ダウンロードURLで`latest`をタグ名（`v9.1.0`など）に置き換える必要があります。

1. このファイルを既知のディレクトリに保存します。`config.toml`ファイルが存在するディレクトリを使用することをお勧めします。たとえば、WindowsマシンでRunnerをホスティングしていて、設定ファイルが`C:\gitlab-runner\config.toml`に保存されている場合は、`zoneinfo.zip`を`C:\gitlab-runner\zoneinfo.zip`に保存します。

1. `zoneinfo.zip`ファイルのフルパスを含む`ZONEINFO`環境変数を設定します。`run`コマンドを使用してRunnerを起動する場合は、次のようにします。

   ```shell
   ZONEINFO=/etc/gitlab-runner/zoneinfo.zip gitlab-runner run <other options ...>
   ```

   Windowsを使用している場合は次のようにします。

   ```powershell
   C:\gitlab-runner> set ZONEINFO=C:\gitlab-runner\zoneinfo.zip
   C:\gitlab-runner> gitlab-runner run <other options ...>
   ```

   GitLab Runnerをシステムサービスとして起動する場合は、サービス設定を更新または上書きする必要があります。

   - Unixシステムでは、サービスマネージャーソフトウェアで設定を変更します。
   - Windowsでは、システム設定でGitLab Runnerユーザーが利用できる環境変数のリストに`ZONEINFO`変数を追加します。

## 複数のGitLab Runnerインスタンスを実行できないのはなぜですか？ {#why-cant-i-run-more-than-one-instance-of-gitlab-runner}

同じ`config.toml`ファイルを共有していなければ実行できます。

同じ設定ファイルを使用する複数のGitLab Runnerインスタンスを実行すると、デバッグが難しい予期しない動作が発生する可能性があります。一度に1つのGitLab Runnerインスタンスのみが特定の`config.toml`ファイルを使用できます。

## `Job failed (system failure): preparing environment:` {#job-failed-system-failure-preparing-environment}

このエラーは多くの場合、Shellによる[プロファイルの読み込み](../shells/_index.md#shell-profile-loading)が原因で発生します。スクリプトの1つが失敗の原因となっています。

失敗の原因となることが判明している`dotfiles`の例:

- `.bash_logout`
- `.condarc`
- `.rvmrc`

SELinuxもこのエラーの原因となる可能性があります。これは、SELinux監査ログを調べることで確認できます。

```shell
sealert -a /var/log/audit/audit.log
```

## Runnerが`Cleaning up`ステージの後に突然終了する {#runner-abruptly-terminates-after-cleaning-up-stage}

「コンテナドリフト検出」設定が有効になっている場合に、ジョブの`Cleaning up files`ステージの後でCrowdStrike Falcon Sensorがポッドを強制終了することが報告されています。ジョブを確実に完了できるようにするには、この設定を無効にする必要があります。

## ジョブが`remote error: tls: bad certificate (exec.go:71:0s)`で失敗する {#job-fails-with-remote-error-tls-bad-certificate-execgo710s}

このエラーは、アーティファクトを作成するジョブの実行中にシステム時刻が大幅に変更された場合に発生する可能性があります。システム時刻が変更されたため、SSL証明書の有効期限が切れ、Runnerがアーティファクトをアップロードしようとするとエラーが発生します。

アーティファクトのアップロード中にSSL検証が確実に成功するようにするには、ジョブの終わりにシステム時刻を有効な日付と時刻に変更します。アーティファクトファイルの作成時刻も変更されているため、アーティファクトファイルは自動的にアーカイブされます。

## Helmチャート: `ERROR .. Unauthorized` {#helm-chart-error--unauthorized}

HelmでデプロイされたRunnerをアンインストールまたはアップグレードする前に、GitLabでRunnerを一時停止し、ジョブが完了するまで待ちます。

ジョブの実行中に`helm uninstall`または`helm upgrade`を使用してRunnerポッドを削除すると、ジョブが完了したときに、次のような`Unauthorized`エラーが発生する可能性があります。

```plaintext
ERROR: Error cleaning up pod: Unauthorized
ERROR: Error cleaning up secrets: Unauthorized
ERROR: Job failed (system failure): Unauthorized
```

これはおそらく、Runnerが削除されるとロールバインドが削除されることが原因で発生します。Runnerポッドはジョブが完了するまで継続し、その後、RunnerがRunnerポッドを削除しようとします。ロールバインドがないと、Runnerポッドはアクセスできなくなります。

詳細については、[このイシュー](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/225)を参照してください。

<!-- markdownlint-disable line-length -->

## Elasticsearchサービスコンテナの起動エラー `max virtual memory areas vm.max_map_count [65530] is too low, increase to at least [262144]` {#elasticsearch-service-container-startup-error-max-virtual-memory-areas-vmmax_map_count-65530-is-too-low-increase-to-at-least-262144}

Elasticsearchには、Elasticsearchが実行されるインスタンスで設定する必要がある`vm.max_map_count`要求事項があります。

プラットフォームに応じてこの値を正しく設定する方法については、[Elasticsearchドキュメント](https://www.elastic.co/docs/deploy-manage/deploy/self-managed/install-elasticsearch-docker-prod)を参照してください。

## `Preparing the "docker+machine" executor ERROR: Preparation failed: exit status 1 Will be retried in 3s` {#preparing-the-dockermachine-executor-error-preparation-failed-exit-status-1-will-be-retried-in-3s}

このエラーは、Docker Machineがexecutor仮想マシンを正常に作成できない場合に発生する可能性があります。このエラーに関する詳細情報を取得するには、`config.toml`で定義した`MachineOptions`を使用して、仮想マシンを手動で作成します。

例: `docker-machine create --driver=google --google-project=GOOGLE-PROJECT-ID --google-zone=GOOGLE-ZONE ...`。

<!-- markdownlint-enable line-length -->

## `No unique index found for name` {#no-unique-index-found-for-name}

このエラーは、Runnerを作成または更新するときに、データベースに`tags`テーブルの一意のインデックスがない場合に発生する可能性があります。GitLab UIで`Response not successful: Received status code 500`エラーが発生する場合があります。

このイシューは、長期間にわたって複数のメジャーアップグレードが行われたインスタンスに影響を与える可能性があります。このイシューを解決するには、[`gitlab:db:deduplicate_tags` Rakeタスク](https://docs.gitlab.com/administration/raketasks/maintenance/#check-the-database-for-deduplicate-cicd-tags)を使用して、テーブル内の重複するタグを統合します。詳細については、[Rakeタスク](https://docs.gitlab.com/raketasks/)を参照してください。
