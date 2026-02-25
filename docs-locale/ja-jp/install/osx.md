---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
description: macOSにGitLab Runnerをインストールします。
title: macOSにGitLab Runnerをインストールする
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

このページでは、macOS（Apple SiliconおよびIntel x86-64）にGitLab Runnerをインストールする方法を説明します。

{{< alert type="note" >}}

GitLab RunnerをインストールするmacOSユーザーは、通常、ローカルまたはリモートで実行されるコンテナまたは仮想マシンに[GitLabをインストール](https://docs.gitlab.com/install/install_methods/)します。

{{< /alert >}}

1. ご使用のシステムに対応するバイナリをダウンロードします。

   - Intelベースのシステムの場合は次のようにします。

     ```shell
     sudo curl --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-amd64"
     ```

   - Apple Siliconベースのシステムの場合は次のようにします。

     ```shell
     sudo curl --output /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-arm64"
     ```

   [Bleeding Edge - その他のタグ付きリリースをダウンロードする](bleeding-edge.md#download-any-other-tagged-release)の説明に従って、利用可能なすべてのバージョンのバイナリをダウンロードできます。

1. 実行のための権限を付与します。

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. GitLab Runnerアプリケーションを実行するユーザーアカウントで、次の手順に従います。

   1. [Runner設定を登録](../register/_index.md)します。登録プロセスで[Shell executor](../executors/shell.md)を選択します。macOSでiOSアプリケーションまたはmacOSアプリケーションをビルドする場合、ジョブはホスト上で直接実行され、認証済みユーザーのIDを使用します。ジョブはコンテナ内で実行されません。このため、コンテナexecutorを使用する場合よりも安全性が低くなります。詳細については、[セキュリティ](../security/_index.md#usage-of-shell-executor)に関する考慮事項のドキュメントを参照してください。

   1. ターミナルを開き、現在のユーザーに切り替えます。

      ```shell
      su - <username>
      ```

   1. GitLab Runnerをサービスとしてインストールして開始します。

      ```shell
      cd ~
      gitlab-runner install
      gitlab-runner start
      ```

   これらのコマンドの実行時に発生する可能性のあるエラーの解決方法について詳しくは、[トラブルシューティングのセクション](#macos-troubleshooting)を参照してください。

1. システムを再起動します。

上記の手順に従った場合、GitLab Runnerの設定ファイル（`config.toml`）は`/Users/<username>/.gitlab-runner/`にあります。[Runner](../configuration/advanced-configuration.md)の設定の詳細について参照してください。

詳細については、[用語集](../_index.md#glossary)を参照してください。

## 既知の問題 {#known-issues}

{{< alert type="note" >}}

サービスは、現在のユーザーとしてログインしているターミナルウィンドウからインストールする必要があります。このようにインストールした場合にのみ、サービスを管理できます。

{{< /alert >}}

現在のユーザーとしてサインインするには、ターミナルでコマンド`su - <username>`を実行します。ユーザー名を取得するには、コマンド`ls /users`を実行します。

macOSでサービスを動作させるための唯一の実証済みの方法は、ユーザーモードでサービスを実行することです。

サービスはユーザーがログインしている場合にのみ実行されるため、macOSマシンで自動ログインを有効にする必要があります。

サービスは`LaunchAgent`として起動されます。`LaunchAgents`を使用することでビルドはUIインタラクションを実行でき、iOSシミュレーターで実行およびテストできるようになります。

macOSには`LaunchDaemons`（バックグラウンドで完全に実行されるサービス）もあることに注意してください。`LaunchDaemons`はシステムの起動時に実行されますが、`LaunchAgents`と同じUIインタラクションへのアクセス権限はありません。Runnerのサービスを`LaunchDaemon`として実行することもできますが、この動作モードはサポートされていません。

`install`コマンドの実行後に`~/Library/LaunchAgents/gitlab-runner.plist`ファイルを検証することで、GitLab Runnerがサービス設定ファイルを作成したことを確認できます。

Homebrewを使用して`git`をインストールした場合、以下を含む`/usr/local/etc/gitconfig`ファイルが追加されている可能性があります。

```ini
[credential]
  helper = osxkeychain
```

これは、ユーザー認証情報をキーチェーンにキャッシュするようにGitに指示しますが、これが必要な動作ではない可能性があります。また、これが原因でフェッチがハングする可能性があります。次のコマンドを使用して、システムの`gitconfig`からこの行を削除できます。

```shell
git config --system --unset credential.helper
```

または、GitLabユーザーの`credential.helper`を無効にすることもできます。

```shell
git config --global --add credential.helper ''
```

次のコマンドを使用して、`credential.helper`の状態を確認できます。

```shell
git config credential.helper
```

## GitLab Runnerをアップグレードする {#upgrade-gitlab-runner}

1. サービスを停止します。

   ```shell
   gitlab-runner stop
   ```

1. バイナリをダウンロードして、GitLab Runner実行可能ファイルを置き換えます。

   - Intelベースのシステムの場合は次のようにします。

     ```shell
     sudo curl -o /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-amd64"
     ```

   - Apple Siliconベースのシステムの場合は次のようにします。

     ```shell
     sudo curl -o /usr/local/bin/gitlab-runner "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-arm64"
     ```

   [Bleeding Edge - その他のタグ付きリリースをダウンロードする](bleeding-edge.md#download-any-other-tagged-release)の説明に従って、利用可能なすべてのバージョンのバイナリをダウンロードできます。

1. 実行のための権限を付与します。

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. サービスを開始します。

   ```shell
   gitlab-runner start
   ```

## サービスファイルをアップグレードする {#upgrade-the-service-file}

`LaunchAgent`設定をアップグレードするには、サービスをアンインストールしてからインストールする必要があります。

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

## `codesign`をGitLab Runnerサービスで使用する {#using-codesign-with-the-gitlab-runner-service}

Homebrewを使用してmacOSに`gitlab-runner`をインストールしており、ビルドが`codesign`を呼び出すときに、ユーザーキーチェーンにアクセスできるように`<key>SessionCreate</key><true/>`を設定する必要がある場合があります。GitLabはHomebrewのformulaを保持しないため、公式バイナリを使用してGitLab Runnerをインストールする必要があります。

次の例では、`gitlab`ユーザーとしてビルドを実行し、コード署名のためにそのユーザーがインストールした署名証明書へのアクセスを必要とします。

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>SessionCreate</key><true/>
    <key>KeepAlive</key>
    <dict>
      <key>SuccessfulExit</key>
      <false/>
    </dict>
    <key>RunAtLoad</key><true/>
    <key>Disabled</key><false/>
    <key>Label</key>
    <string>com.gitlab.gitlab-runner</string>
    <key>UserName</key>
    <string>gitlab</string>
    <key>GroupName</key>
    <string>staff</string>
    <key>ProgramArguments</key>
    <array>
      <string>/usr/local/opt/gitlab-runner/bin/gitlab-runner</string>
      <string>run</string>
      <string>--working-directory</string>
      <string>/Users/gitlab/gitlab-runner</string>
      <string>--config</string>
      <string>/Users/gitlab/gitlab-runner/config.toml</string>
      <string>--service</string>
      <string>gitlab-runner</string>
      <string>--syslog</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
      <key>PATH</key>
      <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
  </dict>
</plist>
```

## macOSのトラブルシューティング {#macos-troubleshooting}

以下のエラーは、macOSでのトラブルシューティングに関連しています。一般的なトラブルシューティングについては、[GitLab Runnerのトラブルシューティング](../faq/_index.md)を参照してください。

### `killed: 9` {#killed-9}

Apple Siliconベースのシステムでは、`gitlab-runner install`、`gitlab-runner start`、または`gitlab-runner register`コマンドを実行するときにこのエラーが発生する可能性があります。

このエラーを解決するには、`~/Library/LaunchAgents/gitlab-runner.plist`の`StandardOutPath`と`StandardErrorPath`の値で指定されたディレクトリが書き込み可能であることを確認します。

次の例では、`/Users/USERNAME/Library/LaunchAgents/gitlab-runner.plist`ファイルが編集されており、ログファイル用に新しい書き込み可能なディレクトリ`gitlab-runner-log`が含まれています。

```xml
 <key>StandardErrorPath</key>
  <string>/Users/USERNAME/gitlab-runner-log/gitlab-runner.err.log</string>
 <key>StandardOutPath</key>
  <string>/Users/USERNAME/gitlab-runner-log/gitlab-runner.out.log</string>
</dict>

```

### エラー: `"launchctl" failed: exit status 112, Could not find domain for` {#error-launchctl-failed-exit-status-112-could-not-find-domain-for}

このメッセージは、macOSにGitLab Runnerをインストールしようとしたときに表示される場合があります。SSH接続ではなく、GUIターミナルアプリケーションからGitLab Runnerサービスを管理していることを確認してください。

### メッセージ: `Failed to authorize rights (0x1) with status: -60007.` {#message-failed-to-authorize-rights-0x1-with-status--60007}

macOSを使用しているときにGitLab Runnerが上記のメッセージでブロックされた場合、この状況が発生する原因は2つあります。

1. ユーザーがUIインタラクションを実行できることを確認します。

   ```shell
   DevToolsSecurity -enable
   sudo security authorizationdb remove system.privilege.taskport is-developer
   ```

   1番目のコマンドは、ユーザーのデベロッパーツールへのアクセスを有効にします。2番目のコマンドは、デベロッパーグループのメンバーであるユーザーがUIインタラクションを実行できるようにします（iOSシミュレーターの実行など）。

1. GitLab Runnerサービスが`SessionCreate = true`を使用していないことを確認します。以前は、GitLab Runnerをサービスとして実行するときに`SessionCreate`を使用して`LaunchAgents`を作成していました。その時点（**Mavericks**）では、これがコード署名を機能させるための唯一の解決策でした。これは最近、**OS X El Capitan**で変更されました。OS X El Capitanでは、この動作を変更する多くの新しいセキュリティ機能が導入されました。

   `SessionCreate`。ただしアップグレードの場合は、`LaunchAgent`スクリプトを手動で再インストールする必要があります。

   ```shell
   gitlab-runner uninstall
   gitlab-runner install
   gitlab-runner start
   ```

   これで、`~/Library/LaunchAgents/gitlab-runner.plist`で`SessionCreate`が`false`に設定されていることを検証できます。

### ジョブエラー: `Failed to connect to path port 3000: Operation timed out` {#job-error-failed-to-connect-to-path-port-3000-operation-timed-out}

ジョブの1つがこのエラーで失敗した場合は、RunnerがGitLabインスタンスに接続できることを確認してください。接続は、次のような原因によってブロックされる可能性があります。

- ファイアウォール
- プロキシ
- 権限
- ルーティング設定

### エラー: `gitlab-runner start`コマンドで`FATAL: Failed to start gitlab-runner: exit status 134` {#error-fatal-failed-to-start-gitlab-runner-exit-status-134-on-gitlab-runner-start-command}

このエラーは、GitLab Runnerサービスが正しくインストールされていないことを示しています。このエラーを解決するには、次のコマンドを実行します。

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

エラーが解決しない場合は、グラフィカルログインを実行します。グラフィカルログインは、サービスの起動に必要な`LaunchAgent`をブートストラップします。詳細については、[既知の問題](osx.md#known-issues)を参照してください。

AWSでホストされているmacOSインスタンスは、インスタンスのGUIに接続するために[追加の手順](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/connect-to-mac-instance.html)を実行する必要があります。`ssh -L`オプションを使用してSSHポート転送を有効にし、`vnc`などのリモートデスクトップクライアントがリモートインスタンスに接続できるようにします。また、AWSでホストされているmacOSインスタンスの`/private/etc/ssh/sshd_config`で`AllowTcpForwarding yes`を設定する必要があります。インスタンスを再起動して、`sshd`設定への変更を適用します。エラーを解決するため、GUIにサインインした後、GUIのターミナルからGitLab Runnerのトラブルシューティングの手順を繰り返し行います。

### エラー: `"launchctl" failed with stderr: Load failed: 5: Input/output error` {#error-launchctl-failed-with-stderr-load-failed-5-inputoutput-error}

`gitlab-runner start`コマンドの実行時にこのエラーが発生した場合は、まず、Runnerがすでに実行中かどうかを確認してください:

```shell
gitlab-runner status
```

Runnerがすでに実行中の場合は、再度開始する必要はありません。実行されておらず、それでもこのエラーが発生する場合は、`~/Library/LaunchAgents/gitlab-runner.plist`の値`StandardOutPath`と`StandardErrorPath`で指定されたディレクトリが存在することを確認してください:

```xml
<key>StandardOutPath</key>
<string>/usr/local/var/log/gitlab-runner.out.log</string>
<key>StandardErrorPath</key>
<string>/usr/local/var/log/gitlab-runner.err.log</string>
```

ディレクトリが存在しない場合はディレクトリを作成し、それらに対する読み取りおよび書き込みを行うための適切な権限がRunnerサービスユーザーにあることを確認します。次に、Runnerを起動します:

```shell
gitlab-runner start
```

### エラー: `Error on fetching TLS Data from API response... error  error=couldn't build CA Chain` {#error-error-on-fetching-tls-data-from-api-response-error--errorcouldnt-build-ca-chain}

GitLab Runner v15.5.0以降にアップグレードすると、次のエラーが発生することがあります。

```plaintext
Certificate doesn't provide parent URL: exiting the loop  Issuer=Baltimore CyberTrust Root IssuerCertURL=[] Serial=33554617 Subject=Baltimore CyberTrust Root context=certificate-chain-build
Verifying last certificate to find the final root certificate  Issuer=Baltimore CyberTrust Root IssuerCertURL=[] Serial=33554617 Subject=Baltimore CyberTrust Root context=certificate-chain-build
ERROR: Error on fetching TLS Data from API response... error  error=couldn't build CA Chain: error while fetching certificates from TLS ConnectionState: error while fetching certificates into the CA Chain: couldn't resolve certificates chain from the leaf certificate: error while resolving certificates chain with verification: error while verifying last certificate from the chain: x509: “Baltimore CyberTrust Root” certificate is not permitted for this usage runner=x7kDEc9Q
```

このエラーが発生した場合は、次の操作を行う必要があります。

1. GitLab Runner v15.5.1以降にアップグレードします。
1. [`[runners.feature_flags]`設定](../configuration/feature-flags.md#enable-feature-flag-in-runner-configuration)で`FF_RESOLVE_FULL_TLS_CHAIN`を`false`に設定します。下記は例です: 

```toml
[[runners]]
  name = "example-runner"
  url = "https://gitlab.com/"
  token = "TOKEN"
  executor = "docker"
  [runners.feature_flags]
    FF_RESOLVE_FULL_TLS_CHAIN = false
```

この機能フラグを無効にすると、SHA-1署名またはその他の非推奨のルート証明書署名を使用するHTTPSエンドポイントのTLS接続の問題を修正できる場合があります。
