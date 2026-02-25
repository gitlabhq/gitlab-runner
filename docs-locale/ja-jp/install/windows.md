---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
description: WindowsシステムにGitLab Runnerをインストールします。
title: WindowsにGitLab Runnerをインストールする
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

WindowsにGitLab Runnerをインストールして実行するには、以下が必要です。

- Git（[公式ウェブサイト](https://git-scm.com/download/win)からインストールできます）
- ユーザーアカウントのパスワード（組み込みのシステムアカウントではなく、ユーザーアカウントで実行する場合）。
- 文字エンコードの問題を回避するために、システムロケールが英語（米国）に設定されていること。詳細については、[イシュー38702](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38702)を参照してください。

## インストール {#installation}

1. システム内の任意の場所（`C:\GitLab-Runner`など）にフォルダーを作成します。
1. [64ビット](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-amd64.exe)または[32ビット](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-386.exe)のバイナリをダウンロードし、作成したフォルダーに配置します。以降の説明では、バイナリの名前を`gitlab-runner.exe`に変更したこと（オプション）を前提としています。[Bleeding Edge - その他のタグ付きリリースをダウンロードする](bleeding-edge.md#download-any-other-tagged-release)の説明に従って、利用可能なすべてのバージョンのバイナリをダウンロードできます。
1. GitLab Runnerのディレクトリと実行可能ファイルに対する`Write`権限を制限してください。これらの権限を設定しないと、一般ユーザーが実行可能ファイルを独自のファイルに置き換え、管理者権限で任意のコードを実行してしまう可能性があります。
1. [管理者権限でのコマンドプロンプト](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7.4#with-administrative-privileges-run-as-administrator)を実行します。
1. [Runnerを登録します](../register/_index.md)。
1. GitLab Runnerをサービスとしてインストールして開始します。組み込みのシステムアカウント（推奨）またはユーザーアカウントを使用してサービスを実行できます。

   **組み込みのシステムアカウントを使用してサービスを実行する**（ステップ1で作成したサンプルディレクトリ`C:\GitLab-Runner`内）

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe install
   .\gitlab-runner.exe start
   ```

   **ユーザーアカウントを使用してサービスを実行する**（ステップ1で作成したサンプルディレクトリ`C:\GitLab-Runner`内）

   現在のユーザーアカウントの有効なパスワードを入力する必要があります。これは、Windowsでサービスを開始するために必要であるためです。

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe install --user ENTER-YOUR-USERNAME --password ENTER-YOUR-PASSWORD
   .\gitlab-runner.exe start
   ```

   GitLab Runnerのインストール中にエラーが発生した場合は、[トラブルシューティングのセクション](#windows-troubleshooting)を参照してください。

1. （オプション）[高度な設定の詳細](../configuration/advanced-configuration.md)で詳しく説明されているようにして、複数の同時ジョブを許可するため、`C:\GitLab-Runner\config.toml`でRunnerの`concurrent`の値を更新します。また、高度な設定の詳細を使用して、BatchではなくBashまたはPowerShellを使用するようにShell executorを更新できます。

これで、Runnerがインストールされ、実行され、システムを再起動するたびに再起動されるようになります。ログはWindowsイベントログに保存されます。

## アップグレード {#upgrade}

1. サービスを停止します（以前と同様に[管理者権限でのコマンドプロンプト](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7.4#with-administrative-privileges-run-as-administrator)が必要です）。

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe stop
   ```

1. [64ビット](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-amd64.exe)または[32ビット](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-386.exe)のバイナリをダウンロードし、Runnerの実行可能ファイルを置き換えます。[Bleeding Edge - その他のタグ付きリリースをダウンロードする](bleeding-edge.md#download-any-other-tagged-release)の説明に従って、利用可能なすべてのバージョンのバイナリをダウンロードできます。

1. サービスを開始します。

   ```powershell
   .\gitlab-runner.exe start
   ```

## アンインストール {#uninstall}

[管理者権限でのコマンドプロンプト](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7.4#with-administrative-privileges-run-as-administrator)から次のようにします。

```powershell
cd C:\GitLab-Runner
.\gitlab-runner.exe stop
.\gitlab-runner.exe uninstall
cd ..
rmdir /s GitLab-Runner
```

## Windowsのトラブルシューティング {#windows-troubleshooting}

[FAQ](../faq/_index.md)セクションを参照してください。このセクションでは、GitLab Runnerに関する最も一般的な問題について説明しています。

_アカウント名が無効です_のようなエラーが発生した場合は、以下を試してください。

```powershell
# Add \. before the username
.\gitlab-runner.exe install --user ".\ENTER-YOUR-USERNAME" --password "ENTER-YOUR-PASSWORD"
```

サービスの開始中に`The service did not start due to a logon failure`エラーが発生した場合は、[FAQセクション](#error-the-service-did-not-start-due-to-a-logon-failure)を参照して、問題を解決する方法を確認してください。

Windowsパスワードがない場合は、GitLab Runnerサービスを開始できませんが、組み込みのシステムアカウントを使用できます。

組み込みのシステムアカウントの問題については、Microsoftのサポートウェブサイトの[Configure the Service to Start Up with the Built-in System Account](https://learn.microsoft.com/en-us/troubleshoot/windows-server/system-management-components/service-startup-permissions#resolution-3-configure-the-service-to-start-up-with-the-built-in-system-account)を参照してください。

### Runnerのログを取得する {#get-runner-logs}

`.\gitlab-runner.exe install`を実行すると、`gitlab-runner`がWindowsサービスとしてインストールされます。イベントビューアーで、プロバイダー名`gitlab-runner`でログを見つけることができます。

GUIにアクセスできない場合は、PowerShellで[`Get-WinEvent`](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.diagnostics/get-winevent?view=powershell-7.4)を実行できます。

```shell
PS C:\> Get-WinEvent -ProviderName gitlab-runner

   ProviderName: gitlab-runner

TimeCreated                     Id LevelDisplayName Message
-----------                     -- ---------------- -------
2/4/2025 6:20:14 AM              1 Information      [session_server].listen_address not defined, session endpoints disabled  builds=0...
2/4/2025 6:20:14 AM              1 Information      listen_address not defined, metrics & debug endpoints disabled  builds=0...
2/4/2025 6:20:14 AM              1 Information      Configuration loaded                                builds=0...
2/4/2025 6:20:14 AM              1 Information      Starting multi-runner from C:\config.toml...        builds=0...
```

### Windowsでのビルド中に`PathTooLongException`が発生する {#i-get-a-pathtoolongexception-during-my-builds-on-windows}

このエラーは、`npm`などのツールが、長さが260文字を超えるパスを含むディレクトリ構造を生成することがあるために発生します。この問題を解決するには、次のいずれかの解決策を採用します。

- `core.longpaths`が有効になっているGitを使用します。

  Gitを使用してディレクトリ構造をクリーンアップすることで、問題を回避できます。

  1. コマンドラインから`git config --system core.longpaths true`を実行します。
  1. GitLab CIプロジェクト設定ページで、`git fetch`を使用するようにプロジェクトを設定します。

- PowerShell用のNTFSSecurityツールを使用します。

  [NTFSSecurity](https://github.com/raandree/NTFSSecurity) PowerShellモジュールは、長いパスをサポートする`Remove-Item2`メソッドを提供します。このモジュールが利用可能な場合は、GitLab Runnerによってそれが検出され、自動的にそれが利用されます。

> GitLab Runner 16.9.1で導入されたリグレッションは、GitLab Runner 17.10.0で修正されています。リグレッションのあるGitLab Runnerバージョンを使用する場合は、次のいずれかの回避策を使用してください。
>
> - `pre_get_sources_script`を使用することにより、Gitシステムレベルの設定を再度有効にします（`Git_CONFIG_NOSYSTEM`を設定解除します）。このアクションにより、Windowsで`core.longpaths`がデフォルトで有効になります。
>
>   ```yaml
>   build:
>     hooks:
>       pre_get_sources_script:
>         - $env:GIT_CONFIG_NOSYSTEM=''
>   ```
>
> - カスタム`GitLab-runner-helper`イメージをビルドします。
>
>   ```dockerfile
>   FROM registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-v17.8.3-servercore21H2
>   ENV GIT_CONFIG_NOSYSTEM=
>   ```

### Windowsバッチスクリプトのエラー: `The system cannot find the batch label specified - buildscript` {#error-with-windows-batch-scripts-the-system-cannot-find-the-batch-label-specified---buildscript}

`.gitlab-ci.yml`のBatchファイル行の先頭に`call`を追加して、`call C:\path\to\test.bat`のように記述する必要があります。下記は例です: 

```yaml
before_script:
  - call C:\path\to\test.bat
```

詳細については、[イシュー1025](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1025)を参照してください。

### Webターミナルで色付きの出力を得るにはどうすればよいですか？ {#how-can-i-get-colored-output-on-the-web-terminal}

**簡単な説明**: 

プログラムの出力にANSIカラーコードが含まれていることを確認してください。テキストの書式設定という点から、UNIX ANSIターミナルエミュレーターで実行しているとします（これはウェブインターフェースの出力であるため）。

**詳しい説明**: 

GitLab CIのウェブインターフェースは、UNIX ANSIターミナルをエミュレートします（少なくとも部分的に）。`gitlab-runner`は、ビルドからの出力をウェブインターフェースに直接パイプします。つまり、存在するANSIカラーコードはすべて有効になります。

古いバージョンのWindowsのコマンドプロンプトターミナル（Windows 10、バージョン1511より前）は、ANSIカラーコードをサポートしていません。代わりにwin32（[`ANSI.SYS`](https://en.wikipedia.org/wiki/ANSI.SYS)）呼び出しを使用しますが、この呼び出しは、表示される文字列に**存在していません**。クロスプラットフォームプログラムを作成する場合、デベロッパーは、通常、デフォルトでANSIカラーコードを使用します。このコードは、Windowsシステムで実行する場合（[Colorama](https://pypi.org/project/colorama/)など）、win32呼び出しに変換されます。

ご使用のプログラムが上記の処理を実行している場合は、ANSIコードが文字列に残るように、CIビルドの変換を無効にする必要があります。

詳細については、[GitLab CI YAMLドキュメント](https://docs.gitlab.com/ci/yaml/#coloring-script-output)でPowerShellを使用する例を参照し、[イシュー332](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/332)を参照してください。

### エラー: `The service did not start due to a logon failure` {#error-the-service-did-not-start-due-to-a-logon-failure}

WindowsにGitLab Runnerサービスをインストールして開始するときに、このエラーが発生する可能性があります。

```shell
gitlab-runner install --password WINDOWS_MACHINE_PASSWORD
gitlab-runner start
FATA[0000] Failed to start GitLab Runner: The service did not start due to a logon failure.
```

このエラーは、サービスの実行に使用されるユーザーが`SeServiceLogonRight`権限を持っていない場合に発生する可能性があります。この場合、選択したユーザーにこの権限を追加してから、サービスを再度開始する必要があります。

1. **Control Panel > System and Security > Administrative Tools**に移動します。
1. **Local Security Policy**ツールを開きます。
1. 左側のリストで**Security Settings > Local Policies > User Rights Assignment**を選択します。
1. 右側のリストで**Log on as a service**を開きます。
1. **Add User or Group...**を選択します。
1. （「手動」で、または**Advanced...**を使用して）ユーザーを追加し、設定を適用します。

[Microsoftドキュメント](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2012-R2-and-2012/dn221981(v=ws.11))によると、これは次のWindowsバージョンで機能します。

- Windows Vista
- Windows Server 2008
- Windows 7
- Windows 8.1
- Windows Server 2008 R2
- Windows Server 2012 R2
- Windows Server 2012
- Windows 8

Local Security Policyツールは、一部のWindowsバージョン（各バージョンの「Home Edition」バリアントなど）では使用できない場合があります。

サービス設定で使用されているユーザーに`SeServiceLogonRight`を追加すると、コマンド`gitlab-runner start`が失敗せずに終了し、サービスが正常に開始されます。

### ジョブが誤って成功または失敗としてマークされる {#job-marked-as-success-or-failed-incorrectly}

ほとんどのWindowsプログラムは、成功した場合には`exit code 0`を出力します。ただし、一部のプログラムは終了コードを返さないか、成功時の値が異なることがあります。例として、Windowsツール`robocopy`があります。次の`.gitlab-ci.yml`は成功するはずですが、`robocopy`によって出力された終了コードが原因で失敗します。

```yaml
test:
  stage: test
  script:
    - New-Item -type Directory -Path ./source
    - New-Item -type Directory -Path ./dest
    - Write-Output "Hello World!" > ./source/file.txt
    - robocopy ./source ./dest
  tags:
    - windows
```

上記のケースでは、`script:`に終了コードチェックを手動で追加する必要があります。たとえば、PowerShellスクリプトを作成できます。

```powershell
$exitCodes = 0,1

robocopy ./source ./dest

if ( $exitCodes.Contains($LastExitCode) ) {
    exit 0
} else {
    exit 1
}
```

`.gitlab-ci.yml`ファイルを次のように変更します。

```yaml
test:
  stage: test
  script:
    - New-Item -type Directory -Path ./source
    - New-Item -type Directory -Path ./dest
    - Write-Output "Hello World!" > ./source/file.txt
    - ./robocopyCommand.ps1
  tags:
    - windows
```

また、PowerShell関数を使用する場合は、`return`と`exit`の違いに注意してください。`exit 1`はジョブを失敗としてマークしますが、`return 1`はそのようにマークしません。

### Kubernetes executorを使用しているときにジョブが成功としてマークされ、途中で終了した {#job-marked-as-success-and-terminated-midway-using-kubernetes-executor}

詳細については、[ジョブの実行](../executors/kubernetes/_index.md#job-execution)を参照してください。

### Docker executor: `unsupported Windows Version` {#docker-executor-unsupported-windows-version}

GitLab Runnerは、サポートされていることを確認するためにWindows Serverのバージョンを確認します。

このために`docker info`を実行します。

GitLab Runnerが起動に失敗し、Windows Serverバージョンを指定せずにエラーを表示する場合、Dockerバージョンが古い可能性があります。

```plaintext
Preparation failed: detecting base image: unsupported Windows Version: Windows Server Datacenter
```

このエラーには、Windows Serverバージョンに関する詳細情報が含まれている必要があります。この情報が、GitLab Runnerがサポートするバージョンと比較されます。

```plaintext
unsupported Windows Version: Windows Server Datacenter Version (OS Build 18363.720)
```

Windows Server上のDocker 17.06.2は、`docker info`の出力で以下を返します。

```plaintext
Operating System: Windows Server Datacenter
```

このケースでの修正策は、Windows Serverリリースと同程度の古いDockerバージョンを、それよりも新しいDockerバージョンをアップグレードすることです。

### Kubernetes executor: `unsupported Windows Version` {#kubernetes-executor-unsupported-windows-version}

Windows上のKubernetes executorは、次のエラーで失敗することがあります。

```plaintext
Using Kubernetes namespace: gitlab-runner
ERROR: Preparation failed: prepare helper image: detecting base image: unsupported Windows Version:
Will be retried in 3s ...
ERROR: Job failed (system failure): prepare helper image: detecting base image: unsupported Windows Version:
```

この問題を修正するには、GitLab Runner設定ファイルの`[runners.kubernetes.node_selector]`セクションに`node.kubernetes.io/windows-build`ノードセレクターを追加します。次に例を示します。

```toml
   [runners.kubernetes.node_selector]
     "kubernetes.io/arch" = "amd64"
     "kubernetes.io/os" = "windows"
     "node.kubernetes.io/windows-build" = "10.0.17763"
```

### マップされたネットワークドライブを使用しているが、ビルドが正しいパスを検出できない {#im-using-a-mapped-network-drive-and-my-build-cannot-find-the-correct-path}

管理者アカウントではなく標準ユーザーアカウントで実行されているGitLab Runnerは、マップされたネットワークドライブにアクセスできません。マップされたネットワークドライブを使用しようとすると、`The system cannot find the path specified.`エラーが発生します。このエラーは、サービスログオンセッションではリソースにアクセスする際に[セキュリティ制限](https://learn.microsoft.com/en-us/windows/win32/services/services-and-redirected-drives)があるために発生します。代わりに、ドライブの[UNCパス](https://learn.microsoft.com/en-us/dotnet/standard/io/file-path-formats#unc-paths)を使用します。

### ビルドコンテナがサービスコンテナに接続できない {#the-build-container-is-unable-to-connect-to-service-containers}

Windowsコンテナでサービスを使用するには、次のようにします。

- [ジョブごとにネットワークを作成する](../executors/docker.md#create-a-network-for-each-job)ネットワーキングモードを使用します。
- `FF_NETWORK_PER_BUILD`機能フラグが有効になっていることを確認します。

### ジョブがビルドディレクトリを作成できず、エラーで失敗する {#the-job-cannot-create-a-build-directory-and-fails-with-an-error}

`Docker-Windows` executorで`GitLab-Runner`を使用すると、ジョブが次のようなエラーで失敗することがあります。

```shell
fatal: cannot chdir to c:/builds/gitlab/test: Permission denied`
```

このエラーが発生した場合は、Dockerエンジンの実行ユーザーに、`C:\Program Data\Docker`に対する完全な権限があることを確認してください。Dockerエンジンは、特定のアクションでこのディレクトリに書き込むことができる必要がありますが、正しい権限がないと失敗します。

[WindowsでのDocker Engineの設定の詳細を参照してください](https://learn.microsoft.com/en-us/virtualization/windowscontainers/manage-docker/configure-docker-daemon)。

### ジョブログのWindows Subsystem for Linux（WSL）STDOUT出力の空白行 {#blank-lines-for-windows-subsystem-for-linux-wsl-stdout-output-in-job-logs}

デフォルトでは、Windows Subsystem for Linux（WSL）のSTDOUT出力はUTF8でエンコードされておらず、ジョブログに空白行として表示されます。STDOUT出力を表示するには、`WSL_UTF8`環境変数を設定して、WSLのエンコードを強制的にUTF8にすることができます。

```yaml
job:
  variables:
    WSL_UTF8: "1"
```
