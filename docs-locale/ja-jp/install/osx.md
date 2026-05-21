---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: AppleシリコンおよびIntel x86-64システムにGitLab Runnerをユーザーモードサービスとしてダウンロード、インストール、設定します。
title: macOSにGitLab Runnerをインストールする
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

AppleシリコンまたはIntel x86-64システムにmacOS用GitLab Runnerをインストールします。GitLab自体は、通常、ローカルまたはリモートのコンテナまたは仮想マシンで実行されます。

## macOSサービスモード {#macos-service-modes}

macOSでは、GitLab Runnerはシステムレベルの`LaunchDaemon`ではなく、ユーザーモードの`LaunchAgent`として実行されます。これが唯一サポートされているモードです。

ユーザーモードでは、Runnerは次のようになります:

- 現在の認証済みユーザーとして実行され、rootとしては実行されません。
- そのユーザーがサインインしたときに開始し、サインアウトしたときに停止します。
- ユーザーのキーチェーンとUIセッションにアクセスできます。これは、iOSシミュレーターを実行し、コード署名を実行するために必要です。
- 設定を`~/.gitlab-runner/config.toml`に保存します。

システムレベルの`LaunchDaemon`は、起動時にrootとして実行され、ユーザーセッションにアクセスできません。GitLab Runnerは、`LaunchDaemon`としての実行をサポートしていません。

再起動後もRunnerを利用できるようにするには、macOSマシンで自動ログインをオンにしてください。

## GitLab Runnerをインストールする {#install-gitlab-runner}

Apple SiliconまたはIntel x86-64システムでCI/CDのジョブを実行するために、macOSにGitLab Runnerをインストールします。

前提条件: 

- ジョブを実行するユーザーアカウントとしてmacOSマシンにサインインしている必要があります。この手順ではSSHセッションを使用しないでください。ローカルGUIターミナルを使用してください。

GitLab Runnerをインストールするには、次の手順に従います:

1. ご使用のシステムに対応するバイナリをダウンロードします。

   - Intel (x86-64)の場合:

     ```shell
     sudo curl --output /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-amd64"
     ```

   - Apple Siliconの場合:

     ```shell
     sudo curl --output /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-arm64"
     ```

   特定のタグ付きリリースのバイナリをダウンロードするには、[他のタグ付きリリースをダウンロード](bleeding-edge.md#download-any-other-tagged-release)を参照してください。

1. バイナリを実行可能にします:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. [Runner設定を登録](../register/_index.md)します。iOSおよびmacOSのビルドには、[Shell executor](../executors/shell.md)を使用してください。セキュリティの詳細については、[Shell executorのセキュリティ](../security/_index.md#usage-of-shell-executor)を参照してください。

1. GitLab Runnerサービスをインストールして開始します:

   ```shell
   cd ~
   gitlab-runner install
   gitlab-runner start
   ```

1. システムを再起動します。

`gitlab-runner install`コマンドは、`~/Library/LaunchAgents/gitlab-runner.plist`に`LaunchAgent` plistを作成し、`launchctl`に登録します。エラーが発生した場合は、[トラブルシューティング](#troubleshooting)を参照してください。

## 設定ファイルの場所 {#configuration-file-locations}

| ファイル                 | パス                                             |
|----------------------|--------------------------------------------------|
| 設定        | `~/.gitlab-runner/config.toml`                   |
| `LaunchAgent` plist  | `~/Library/LaunchAgents/gitlab-runner.plist`     |
| 標準出力ログ  | `~/Library/Logs/gitlab-runner.out.log`           |
| 標準エラーログ   | `~/Library/Logs/gitlab-runner.err.log`           |

設定オプションの詳細については、[高度な設定](../configuration/advanced-configuration.md)を参照してください。

## GitLab Runnerをアップグレードする {#upgrade-gitlab-runner}

GitLab Runnerを新しいバージョンにアップグレードするには:

1. サービスを停止します。

   ```shell
   gitlab-runner stop
   ```

1. バイナリをダウンロードして、GitLab Runner実行可能ファイルを置き換えます。

   - Intel (x86-64)の場合:

     ```shell
     sudo curl -o /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-amd64"
     ```

   - Apple Siliconの場合:

     ```shell
     sudo curl -o /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-arm64"
     ```

   特定のタグ付きリリースのバイナリをダウンロードするには、[他のタグ付きリリースをダウンロード](bleeding-edge.md#download-any-other-tagged-release)を参照してください。

1. バイナリを実行可能にします:

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. サービスを開始します:

   ```shell
   gitlab-runner start
   ```

## サービスファイルをアップグレードする {#upgrade-the-service-file}

`LaunchAgent`の設定をアップグレードするには、サービスをアンインストールして再インストールします:

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

## GitLab Runnerで`codesign`を使用する {#use-codesign-with-gitlab-runner}

HomebrewでGitLab Runnerをインストールし、ビルドが`codesign`を呼び出す場合、ユーザーキーチェーンにアクセスするために`<key>SessionCreate</key><true/>`を設定する必要があるかもしれません。

> [!note]
> GitLabはHomebrewのフォーミュラを保守していません。公式バイナリを使用してGitLab Runnerをインストールしてください。

以下の例では、Runnerは`gitlab`ユーザーとしてビルドを実行し、そのユーザーの署名証明書にアクセスする必要があります:

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

## トラブルシューティング {#troubleshooting}

macOSにGitLab Runnerをインストールする際に、以下のイシューに遭遇する可能性があります。

一般的なトラブルシューティングについては、[GitLab Runnerのトラブルシューティング](../faq/_index.md)を参照してください。

### エラー: `killed: 9` {#error-killed-9}

Apple Siliconでは、`gitlab-runner install`、`gitlab-runner start`、または`gitlab-runner register`コマンドを実行すると、このエラーが発生する可能性があります。

このエラーを解決するには、`~/Library/LaunchAgents/gitlab-runner.plist`にある`StandardOutPath`と`StandardErrorPath`のディレクトリが存在し、書き込み可能であることを確認してください。例: 

```xml
<key>StandardErrorPath</key>
<string>/Users/<username>/gitlab-runner-log/gitlab-runner.err.log</string>
<key>StandardOutPath</key>
<string>/Users/<username>/gitlab-runner-log/gitlab-runner.out.log</string>
```

### エラー: `"launchctl" failed: Could not find domain for` {#error-launchctl-failed-could-not-find-domain-for}

このエラーは、ローカルGUIターミナルの代わりにSSH経由でGitLab Runnerサービスを管理するときに発生します。

このエラーを解決するには、macOSマシンでターミナルアプリケーションを直接開き、そこから`install`と`start`コマンドを実行してください。

### エラー: `Failed to authorize rights (0x1) with status: -60007` {#error-failed-to-authorize-rights-0x1-with-status--60007}

このエラーには2つの原因が考えられます。

あなたのユーザーアカウントには開発者ツールへのアクセス権がありません。アクセスを許可するには:

```shell
DevToolsSecurity -enable
sudo security authorizationdb remove system.privilege.taskport is-developer
```

または、`LaunchAgent` plistの`SessionCreate`が`true`に設定されています。このイシューを修正するには、サービスを再インストールしてください:

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

`~/Library/LaunchAgents/gitlab-runner.plist`の`SessionCreate`が`false`に設定されていることを確認してください。

### エラー: `Failed to connect to path port 3000: Operation timed out` {#error-failed-to-connect-to-path-port-3000-operation-timed-out}

RunnerがあなたのGitLabインスタンスに到達できません。接続をブロックしている可能性のあるファイアウォール、プロキシ、ルーティング設定、または権限のイシューを確認してください。

### エラー: `FATAL: Failed to start gitlab-runner: exit status 134` {#error-fatal-failed-to-start-gitlab-runner-exit-status-134}

このエラーは、GitLab Runnerサービスが正しくインストールされていないことを示しています。

このエラーを解決するには、サービスを再インストールしてください:

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

エラーが解決しない場合は、SSHを使用する代わりにmacOS GUIデスクトップにサインインし、そこからターミナルでコマンドを実行してください。`LaunchAgent`は、ブートストラップのためにグラフィカルなログインセッションを必要とします。

AWS上のmacOSインスタンスの場合、GUIに接続するために[AWSのドキュメント](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/connect-to-mac-instance.html)に従い、そのセッションのターミナルから再試行してください。

### エラー: `launchctl failed: Load failed: 5: Input/output error` {#error-launchctl-failed-load-failed-5-inputoutput-error}

`gitlab-runner start`コマンドの実行時にこのエラーが発生した場合は、まず、Runnerがすでに実行中かどうかを確認してください:

```shell
gitlab-runner status
```

Runnerが実行されていない場合は、`~/Library/LaunchAgents/gitlab-runner.plist`にある`StandardOutPath`と`StandardErrorPath`のディレクトリが存在し、Runnerのユーザーアカウントがそれらへの読み取り/書き込みアクセス権を持っていることを確認してください。次に、Runnerを起動します:

```shell
gitlab-runner start
```

### エラー: `couldn't build CA Chain` {#error-couldnt-build-ca-chain}

このエラーは、GitLab Runner v15.5.0にアップグレードした後に発生する可能性があります。完全なエラーメッセージは次のとおりです:

```plaintext
ERROR: Error on fetching TLS Data from API response... error  error=couldn't build CA Chain:
error while fetching certificates from TLS ConnectionState: error while fetching certificates
into the CA Chain: couldn't resolve certificates chain from the leaf certificate: error while
resolving certificates chain with verification: error while verifying last certificate from
the chain: x509: "Baltimore CyberTrust Root" certificate is not permitted for this usage
runner=x7kDEc9Q
```

このエラーを解決するには:

1. GitLab Runner v15.5.1以降にアップグレードします。
1. アップグレードできない場合は、[`[runners.feature_flags]`の設定](../configuration/feature-flags.md#enable-feature-flag-in-runner-configuration)で`FF_RESOLVE_FULL_TLS_CHAIN`を`false`に設定します:

   ```toml
   [[runners]]
     name = "example-runner"
     url = "https://gitlab.com/"
     token = "TOKEN"
     executor = "docker"
     [runners.feature_flags]
       FF_RESOLVE_FULL_TLS_CHAIN = false
   ```

### Homebrew Git認証情報ヘルパーによりフェッチがハングする {#homebrew-git-credential-helper-causes-fetches-to-hang}

HomebrewがGitをインストールした場合、`/usr/local/etc/gitconfig`に`credential.helper = osxkeychain`エントリが追加されている可能性があります。これはmacOSキーチェーンに認証情報をキャッシュし、`git fetch`がハングする原因となることがあります。

認証情報ヘルパーをシステム全体から削除するには:

```shell
git config --system --unset credential.helper
```

GitLab Runnerユーザーのみに無効にするには:

```shell
git config --global --add credential.helper ''
```

現在の設定を確認するには:

```shell
git config credential.helper
```
