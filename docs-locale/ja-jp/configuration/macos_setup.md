---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: macOS Runnerをセットアップする
---

macOS RunnerでCI/CDジョブを実行するには、次の手順を順番に実行します。

完了すると、GitLab RunnerがmacOSマシン上で実行され、個々のRunnerがジョブを処理できるようになります。

- システムShellをBashに変更します。
- Homebrew、rbenv、およびGitLab Runnerをインストールします。
- rbenvを設定し、Rubyをインストールします。
- Xcodeをインストールします。
- Runnerを登録します。
- CI/CDを設定します。

## 前提条件 {#prerequisites}

はじめる前:

- macOSの最新バージョンをインストールします。このガイドは11.4で開発されました。
- ターミナルまたはSSHでマシンにアクセスできることを確認します。

## システムShellをBashに変更する {#change-the-system-shell-to-bash}

新しいバージョンのmacOSでは、デフォルトのShellとしてZshが使用されます。ただし、RunnerのShell executorでは、Bash固有の構文と機能を使用するものが多いため、CI/CDスクリプトが正しく実行されるようにBashが必要です。

1. マシンに接続し、デフォルトのShellを確認します:

   ```shell
   echo $SHELL
   ```

1. 結果が`/bin/bash`でない場合は、次を実行してShellを変更します:

   ```shell
   chsh -s /bin/bash
   ```

1. パスワードを入力します。
1. ターミナルを再起動するか、SSHを使用して再接続します。
1. `echo $SHELL`をもう一度実行します。結果は`/bin/bash`になるはずです。

## Homebrew、rbenv、GitLab Runnerをインストールする {#install-homebrew-rbenv-and-gitlab-runner}

Runnerがマシンに接続してジョブを実行するには、特定の環境オプションが必要です。

1. [Homebrew](https://brew.sh/)パッケージマネージャーをインストールします:

   ```shell
   /bin/bash -c "$(curl "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh")"
   ```

1. Rubyバージョンマネージャーである[`rbenv`](https://github.com/rbenv/rbenv)とGitLab Runnerをセットアップします:

   ```shell
   brew install rbenv gitlab-runner
   brew services start gitlab-runner
   ```

## rbenvを設定してRubyをインストールする {#configure-rbenv-and-install-ruby}

rbenvを設定し、Rubyをインストールします。

1. rbenvをBash環境に追加します:

   ```shell
   echo 'if which rbenv > /dev/null; then eval "$(rbenv init -)"; fi' >> ~/.bash_profile
   source ~/.bash_profile
   ```

1. Ruby 3.3.xをインストールし、マシン全体のデフォルトとして設定します:

   ```shell
   rbenv install 3.3.4
   rbenv global 3.3.4
   ```

## Xcodeをインストールします {#install-xcode}

Xcodeをインストールして設定します。

1. 次のいずれかの場所に移動して、Xcodeをインストールします:

   - Apple App Store。
   - [Apple Developer Portal](https://developer.apple.com/)。
   - [`xcode-install`](https://github.com/xcpretty/xcode-install)。このプロジェクトは、コマンドラインからさまざまなAppleの依存関係を簡単にダウンロードできるようにすることを目的としています。

1. ライセンスに同意し、推奨される追加コンポーネントをインストールします。これを行うには、Xcodeを開いてプロンプトに従うか、ターミナルで次のコマンドを実行します:

   ```shell
   sudo xcodebuild -runFirstLaunch
   ```

1. Xcodeがビルド中に適切なコマンドラインツールを読み込むように、アクティブなデベロッパーディレクトリを更新します:

   ```shell
   sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
   ```

### プロジェクトRunnerを作成して登録する {#create-and-register-a-project-runner}

[プロジェクトRunnerを作成して登録](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-project-runner-with-a-runner-authentication-token)します。

Runnerを作成して登録するとき:

- GitLabで、タグ`macos`を追加して、macOSジョブがこのmacOSマシンで実行されるようにします。
- コマンドラインで、`shell`を[executor](../executors/_index.md)として選択します。

Runnerを登録すると、コマンドラインに成功メッセージが表示されます:

```shell
Runner registered successfully. Feel free to start it, but if it's running already the config should be automatically reloaded!
```

Runnerを表示するには:

1. 上部のバーで、**検索または移動先**を選択して、プロジェクトまたはグループを見つけます。
1. **設定 > CI/CD**を選択します。
1. **Runner**を展開します。

### CI/CDを設定する {#configure-cicd}

GitLabプロジェクトで、CI/CDを設定してビルドを開始します。このサンプルの`.gitlab-ci.yml`ファイルを使用できます。タグが、Runnerの登録に使用したタグと一致することを確認してください。

```yaml
stages:
  - build
  - test

variables:
  LANG: "en_US.UTF-8"

before_script:
  - gem install bundler
  - bundle install
  - gem install cocoapods
  - pod install

build:
  stage: build
  script:
    - bundle exec fastlane build
  tags:
    - macos

test:
  stage: test
  script:
    - bundle exec fastlane test
  tags:
    - macos
```

macOS Runnerは、プロジェクトをビルドする必要があります。
