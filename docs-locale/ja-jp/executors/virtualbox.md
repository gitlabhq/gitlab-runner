---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: VirtualBox
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

{{< alert type="note" >}}

Parallels executorは、VirtualBox executorと同じように動作します。ローカルキャッシュはサポートされていません。[分散キャッシュ](../configuration/speed_up_job_execution.md)がサポートされています。

{{< /alert >}}

VirtualBoxを使用すると、VirtualBoxの仮想化を使用して、すべてのビルドにクリーンなビルド環境を提供できます。このexecutorは、VirtualBoxで実行できるすべてのシステムをサポートします。唯一の要件は、仮想マシンがSSHサーバーを公開し、BashまたはPowerShellと互換性のあるシェルを提供することです。

{{< alert type="note" >}}

GitLab RunnerがVirtualBox executorを使用するすべての仮想マシンで、[一般的な前提条件](_index.md#prerequisites-for-non-docker-executors)を満たしていることを確認してください。

{{< /alert >}}

## 概要 {#overview}

プロジェクトのソースコードは、`~/builds/<namespace>/<project-name>`にチェックアウトされます。

各項目の説明: 

- `<namespace>`は、GitLabでプロジェクトが保存されているネームスペースです。
- `<project-name>`は、GitLabに保存されているプロジェクトの名前です。

`~/builds`ディレクトリをオーバーライドするには、[`config.toml`](../configuration/advanced-configuration.md)の`[[runners]]`セクションで`builds_dir`オプションを指定します。

`GIT_CLONE_PATH`を使用して、ジョブごとに[カスタムビルドディレクトリ](https://docs.gitlab.com/ci/runners/configure_runners/#custom-build-directories)を定義することもできます。

## 新しいベース仮想マシンを作成する {#create-a-new-base-virtual-machine}

1. [VirtualBox](https://www.virtualbox.org)をインストールします。
   - Windowsから実行していて、VirtualBoxがデフォルトの場所にインストールされている場合（たとえば、`%PROGRAMFILES%\Oracle\VirtualBox`）、GitLab Runnerはそれを自動的に検出します。そうでない場合は、`gitlab-runner`プロセスの`PATH`環境変数にインストールフォルダーを追加する必要があります。
1. VirtualBoxで新しい仮想マシンをインポートまたは作成します
1. ネットワークアダプター1を「NAT」として構成します（これは現在、GitLab RunnerがSSH経由でゲストに接続できる唯一の方法です）。
1. （オプション）別のネットワークアダプターを「ブリッジネットワーキング」として構成して、（たとえば）ゲストからインターネットにアクセスできるようにします
1. 新しい仮想マシンにログインします
1. Windows VMの場合は、[Windows VMのチェックリスト](#checklist-for-windows-vms)を参照してください
1. OpenSSHサーバーをインストールします
1. ビルドに必要な他のすべての依存関係をインストールします
1. ジョブアーティファクトをダウンロードまたはアップロードする場合は、VM内に`gitlab-runner`をインストールします
1. ログアウトして、仮想マシンをシャットダウンします

Vagrantのような自動化ツールを使用して、仮想マシンをプロビジョニングするのは完全に問題ありません。

## 新しいRunnerを作成する {#create-a-new-runner}

1. VirtualBoxを実行しているホストにGitLab Runnerをインストールします
1. `gitlab-runner register`で新しいRunnerを登録します
1. `virtualbox`executorを選択します
1. 以前に作成したベース仮想マシンの名前を入力します（仮想マシンの設定の**一般 > Basic > 名前**の下にあります）。
1. 仮想マシンのSSH `user`と`password`、または`identity_file`へのパスを入力します

## 仕組み {#how-it-works}

新しいビルドが開始されるとき:

1. 仮想マシンの一意の名前が生成されます：`runner-<short-token>-concurrent-<id>`
1. 仮想マシンが存在しない場合は、複製されます
1. SSHサーバーにアクセスするために、ポート転送ルールが作成されます
1. GitLab Runnerは、仮想マシンのスナップショットを開始または復元します
1. GitLab Runnerは、SSHサーバーがアクセス可能になるのを待ちます
1. GitLab Runnerは、実行中の仮想マシンのスナップショットを作成します（これは、次のビルドを高速化するために行われます）。
1. GitLab Runnerは仮想マシンに接続し、ビルドを実行します
1. 有効になっている場合、アーティファクトのアップロードは、仮想マシン*内*の`gitlab-runner`バイナリを使用して行われます。
1. GitLab Runnerは、仮想マシンを停止またはシャットダウンします

## Windows VMのチェックリスト {#checklist-for-windows-vms}

WindowsでVirtualBoxを使用するには、CygwinまたはPowerShellをインストールできます。

### Cygwinの使用 {#use-cygwin}

- [Cygwin](https://cygwin.com/)をインストールします
- `sshd`とGitをCygwinからインストールします（*Git for Windows*は使用しないでください。 パスの問題が発生します！）
- Git LFSをインストールします
- `sshd`を構成し、サービスとしてセットアップします（[Cygwin Wiki](https://cygwin.fandom.com/wiki/Sshd)を参照）。
- ポート22で受信TCPトラフィックを許可するように、Windowsファイアウォールのルールを作成します
- GitLabサーバーを`~/.ssh/known_hosts`に追加します
- CygwinとWindows間でパスを変換するには、[`cygpath`ユーティリティ](https://cygwin.fandom.com/wiki/Cygpath_utility)を使用します

### ネイティブOpenSSHとPowerShellの使用 {#use-native-openssh-and-powershell}

- [PowerShell](https://learn.microsoft.com/en-us/powershell/scripting/install/install-powershell-on-windows?view=powershell-7.4)をインストールします
- [OpenSSH](https://learn.microsoft.com/en-us/windows-server/administration/openssh/openssh_install_firstuse?tabs=powershell#install-openssh-for-windows)をインストールして構成します
- [Git for Windows](https://git-scm.com/)をインストールします
- [のデフォルトシェルを`pwsh`として設定](https://learn.microsoft.com/en-us/windows-server/administration/OpenSSH/openssh-server-configuration#configuring-the-default-shell-for-openssh-in-windows)します。正しいフルパスで例を更新します:

  ```powershell
  New-ItemProperty -Path "HKLM:\SOFTWARE\OpenSSH" -Name DefaultShell -Value "$PSHOME\pwsh.exe" -PropertyType String -Force
  ```

- [`config.toml`](../configuration/advanced-configuration.md)にシェル`pwsh`を追加します
