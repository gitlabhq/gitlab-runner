---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: 自己署名証明書またはカスタム認証局
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

GitLab Runnerには、TLSピアの検証に使用される証明書を設定するための2つのオプションがあります。

- **For connections to the GitLab server**: 証明書ファイルは、[GitLabサーバーを対象とした自己署名証明書のサポートされているオプション](#supported-options-for-self-signed-certificates-targeting-the-gitlab-server)セクションで詳しく説明されているように指定できます。

  これにより、`x509: certificate signed by unknown authority` Runner登録時の問題が解決されます。

  既存のRunnerの場合、ジョブを確認しようとするとRunnerログに同じエラーが示されることがあります。

  ```plaintext
  Couldn't execute POST against https://hostname.tld/api/v4/jobs/request:
  Post https://hostname.tld/api/v4/jobs/request: x509: certificate signed by unknown authority
  ```

- **Connecting to a cache server or an external Git LFS store**: より一般的なアプローチで、ユーザースクリプトなどの他のシナリオも対象としており、コンテナに証明書を指定してインストールすることができます。[DockerおよびKubernetes executorのTLS証明書の信頼](#trusting-tls-certificates-for-docker-and-kubernetes-executors)セクションで詳しく説明されています。

  証明書が欠落しているGit LFSオペレーションに関するジョブログのエラーの例

  ```plaintext
  LFS: Get https://object.hostname.tld/lfs-dev/c8/95/a34909dce385b85cee1a943788044859d685e66c002dbf7b28e10abeef20?X-Amz-Expires=600&X-Amz-Date=20201006T043010Z&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=svcgitlabstoragedev%2F20201006%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-SignedHeaders=host&X-Amz-Signature=012211eb0ff0e374086e8c2d37556f2d8ca4cc948763e90896f8f5774a100b55: x509: certificate signed by unknown authority
  ```

## GitLabサーバーを対象とする自己署名証明書のサポートされているオプション {#supported-options-for-self-signed-certificates-targeting-the-gitlab-server}

このセクションでは、GitLabサーバーのみがカスタム証明書を必要とする状況について説明します。他のホスト（[プロキシダウンロードが有効](https://docs.gitlab.com/administration/object_storage/#proxy-download)になっていないオブジェクトストレージサービスなど）もカスタム認証局（CA）を必要とする場合は、[次のセクション](#trusting-tls-certificates-for-docker-and-kubernetes-executors)を参照してください。

GitLab Runnerは次のオプションをサポートしています。

- **デフォルト - システム証明書を読み取る**: GitLab Runnerはシステム証明書ストアを読み取り、システムに保存されている公開認証局（CA）に照らしてGitLabサーバーを検証します。

- **カスタム証明書ファイルを指定する**: GitLab Runnerは、[登録時](../commands/_index.md#gitlab-runner-register)（`gitlab-runner register --tls-ca-file=/path`）および[`config.toml`](advanced-configuration.md)の`[[runners]]`セクションで`tls-ca-file`オプションを公開します。これにより、カスタム証明書ファイルを指定できるようになります。このファイルは、RunnerがGitLabサーバーへのアクセスを試行するたびに読み取られます。GitLab Runner Helmチャートを使用している場合は、[カスタム証明書を使用してGitLabにアクセスする](../install/kubernetes_helm_chart_configuration.md#access-gitlab-with-a-custom-certificate)の説明に従って証明書を設定する必要があります。

- **PEM証明書を読み取る**: GitLab Runnerは、定義済みのファイルからPEM証明書（**DER形式はサポートされていない**）を読み取ります。
  - GitLab Runnerが`root`として実行されている場合は、*nixシステムの`/etc/gitlab-runner/certs/gitlab.example.com.crt`。

    サーバーアドレスが`https://gitlab.example.com:8443/`の場合は、`/etc/gitlab-runner/certs/gitlab.example.com.crt`に証明書ファイルを作成します。

    `openssl`クライアントを使用して、GitLabインスタンスの証明書を`/etc/gitlab-runner/certs`にダウンロードできます。

    ```shell
    openssl s_client -showcerts -connect gitlab.example.com:443 -servername gitlab.example.com < /dev/null 2>/dev/null | openssl x509 -outform PEM > /etc/gitlab-runner/certs/gitlab.example.com.crt
    ```

    ファイルが正しくインストールされていることを検証するには、`openssl`などのツールを使用できます。下記は例です: 

    ```shell
    echo | openssl s_client -CAfile /etc/gitlab-runner/certs/gitlab.example.com.crt -connect gitlab.example.com:443 -servername gitlab.example.com
    ```

  - GitLab Runnerが非`root`として実行されている場合は、*nixシステムの`~/.gitlab-runner/certs/gitlab.example.com.crt`。
  - その他のシステムの`./certs/gitlab.example.com.crt`。GitLab RunnerをWindowsサービスとして実行している場合、これは機能しません。代わりに、カスタム証明書ファイルを指定してください。

ノート:

- GitLabサーバー証明書がCAによって署名されている場合は、GitLabサーバー署名証明書ではなくCA証明書を使用してください。場合によっては、中間証明書もチェーンに追加する必要があります。たとえば、プライマリ証明書、中間証明書、ルート証明書がある場合は、それらすべてを1つのファイルにまとめることができます。

  ```plaintext
  -----BEGIN CERTIFICATE-----
  (Your primary SSL certificate: your_domain_name.crt)
  -----END CERTIFICATE-----
  -----BEGIN CERTIFICATE-----
  (Your intermediate certificate)
  -----END CERTIFICATE-----
  -----BEGIN CERTIFICATE-----
  (Your root certificate)
  -----END CERTIFICATE-----
  ```

- 既存のRunnerの証明書を更新する場合は、[再起動](../commands/_index.md#gitlab-runner-restart)します。
- HTTPを介してすでにRunnerを設定している場合は、`config.toml`でインスタンスパスをGitLabインスタンスの新しいHTTPS URLに更新します。
- 一時的な安全性の低い回避策として、証明書の検証をスキップする方法があります。このためには、`.gitlab-ci.yml`ファイルの`variables:`セクションでCI変数`GIT_SSL_NO_VERIFY`を`true`に設定します。

### Gitのクローン {#git-cloning}

Runnerは、`CI_SERVER_TLS_CA_FILE`を使用してCAチェーンを構築するために不足している証明書を挿入します。これにより、公的に信頼されている証明書を使用しないサーバーで`git clone`とアーティファクトが機能するようになります。

このアプローチは安全ですが、Runnerが単一信頼点になります。

## Docker executorとKubernetes executorのTLS証明書を信頼する {#trusting-tls-certificates-for-docker-and-kubernetes-executors}

コンテナに証明書を登録する際には、次の情報を考慮してください。

- ユーザースクリプトの実行に使用される[**ユーザーイメージ**](https://docs.gitlab.com/ci/yaml/#image)。ユーザースクリプトの証明書を信頼するシナリオでは、証明書のインストール方法についてユーザーが責任を担う必要があります。証明書のインストール手順は、イメージによって異なることがあります。Runnerは、発生し得るすべてのシナリオにおいて証明書をインストールする方法を把握することはできません。
- Git、アーティファクト、およびキャッシュオペレーションの処理に使用される[**Runnerヘルパーイメージ**](advanced-configuration.md#helper-image)。他のCI/CDステージの証明書を信頼するシナリオでは、ユーザーが行う必要がある操作は、特定の場所（`/etc/gitlab-runner/certs/ca.crt`など）で証明書ファイルを使用できるようにすることだけです。Dockerコンテナがユーザーのために証明書ファイルを自動的にインストールします。

### ユーザースクリプトの証明書を信頼する {#trusting-the-certificate-for-user-scripts}

ビルドがTLSと自己署名証明書またはカスタム証明書を使用する場合は、ピア通信のためにビルドジョブに証明書をインストールします。デフォルトでは、ユーザースクリプトを実行しているDockerコンテナには証明書ファイルがインストールされていません。これは、カスタムキャッシュホストを使用するか、セカンダリ`git clone`を実行するか、`wget`のようなツールでファイルをフェッチするために必要になる場合があります。

証明書をインストールするには、次の手順に従います。

1. 必要なファイルをDockerボリュームとしてマップして、スクリプトを実行するDockerコンテナがこれらのファイルを認識できるようにします。このためには、たとえば`config.toml`ファイルの`[runners.docker]`内でそれぞれのキーの中にボリュームを追加します。

   - **Linux**:

     ```toml
     [[runners]]
       name = "docker"
       url = "https://example.com/"
       token = "TOKEN"
       executor = "docker"

       [runners.docker]
          image = "ubuntu:latest"

          # Add path to your ca.crt file in the volumes list
          volumes = ["/cache", "/path/to-ca-cert-dir/ca.crt:/etc/gitlab-runner/certs/ca.crt:ro"]
     ```

1. **Linuxのみ**: [`pre_build_script`](advanced-configuration.md#the-runners-section)で、次の操作を行うマップされたファイル（`ca.crt`など）を使用します。
   1. Dockerコンテナ内の`/usr/local/share/ca-certificates/ca.crt`にこのファイルをコピーします。
   1. `update-ca-certificates --fresh`を実行してインストールします。次に例を示します（コマンドは使用しているディストリビューションによって異なります）。

      - Ubuntu:

        ```toml
        [[runners]]
          name = "docker"
          url = "https://example.com/"
          token = "TOKEN"
          executor = "docker"

          # Copy and install CA certificate before each job
          pre_build_script = """
          apt-get update -y > /dev/null
          apt-get install -y ca-certificates > /dev/null

          cp /etc/gitlab-runner/certs/ca.crt /usr/local/share/ca-certificates/ca.crt
          update-ca-certificates --fresh > /dev/null
          """
        ```

      - Alpine:

        ```toml
        [[runners]]
          name = "docker"
          url = "https://example.com/"
          token = "TOKEN"
          executor = "docker"

          # Copy and install CA certificate before each job
          pre_build_script = """
          apk update >/dev/null
          apk add ca-certificates > /dev/null
          rm -rf /var/cache/apk/*

          cp /etc/gitlab-runner/certs/ca.crt /usr/local/share/ca-certificates/ca.crt
          update-ca-certificates --fresh > /dev/null
          """
        ```

使用可能なGitLabサーバーCA証明書のみが必要な場合は、`CI_SERVER_TLS_CA_FILE`変数に格納されているファイルから取得できます。

```shell
curl --cacert "${CI_SERVER_TLS_CA_FILE}"  ${URL} -o ${FILE}
```

### 他のCI/CDステージの証明書を信頼する {#trusting-the-certificate-for-the-other-cicd-stages}

Linuxでは`/etc/gitlab-runner/certs/ca.crt`に、Windowsでは`C:\GitLab-Runner\certs\ca.crt`に証明書ファイルをマップできます。Runnerヘルパーイメージは、起動時にこのユーザー定義の`ca.crt`ファイルをインストールし、クローンやアーティファクトのアップロードなどの操作を実行するときにこのファイルを使用します。

#### Docker {#docker}

- **Linux**:

  ```toml
  [[runners]]
    name = "docker"
    url = "https://example.com/"
    token = "TOKEN"
    executor = "docker"

    [runners.docker]
      image = "ubuntu:latest"

      # Add path to your ca.crt file in the volumes list
      volumes = ["/cache", "/path/to-ca-cert-dir/ca.crt:/etc/gitlab-runner/certs/ca.crt:ro"]
  ```

- **Windows**:

  ```toml
  [[runners]]
    name = "docker"
    url = "https://example.com/"
    token = "TOKEN"
    executor = "docker"

    [runners.docker]
      image = "mcr.microsoft.com/windows/servercore:21H2"

      # Add directory holding your ca.crt file in the volumes list
      volumes = ["c:\\cache", "c:\\path\\to-ca-cert-dir:C:\\GitLab-Runner\\certs:ro"]
  ```

#### Kubernetes {#kubernetes}

Kubernetesで実行されているジョブに証明書ファイルを提供するには、次の手順に従います。

1. ネームスペースに証明書をKubernetesシークレットとして保存します。

   ```shell
   kubectl create secret generic <SECRET_NAME> --namespace <NAMESPACE> --from-file=<CERT_FILE>
   ```

1. `<SECRET_NAME>`と`<LOCATION>`を適切な値に置き換えて、Runnerでシークレットをボリュームとしてマウントします。

   ```toml
   gitlab-runner:
     runners:
      config: |
        [[runners]]
          [runners.kubernetes]
            namespace = "{{.Release.Namespace}}"
            image = "ubuntu:latest"
          [[runners.kubernetes.volumes.secret]]
              name = "<SECRET_NAME>"
              mount_path = "<LOCATION>"
   ```

   `mount_path`は、証明書が保存されているコンテナ内のディレクトリです。`mount_path`として`/etc/gitlab-runner/certs/`を使用し、証明書ファイルとして`ca.crt`を使用した場合、証明書はコンテナ内の`/etc/gitlab-runner/certs/ca.crt`にあります。
1. ジョブの一部として、マップされた証明書ファイルをシステム証明書ストアにインストールします。たとえば、Ubuntuコンテナでは次のようになります。

   ```yaml
   script:
     - cp /etc/gitlab-runner/certs/ca.crt /usr/local/share/ca-certificates/
     - update-ca-certificates
   ```

  Kubernetes executorによるヘルパーイメージの`ENTRYPOINT`の処理には、[既知のイシュー](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28484)があります。証明書ファイルがマップされている場合、この証明書ファイルはシステム証明書ストアに自動的にインストールされません。

## トラブルシューティング {#troubleshooting}

一般的な[SSLトラブルシューティング](https://docs.gitlab.com/omnibus/settings/ssl/ssl_troubleshooting/)のドキュメントを参照してください。

また、[`tlsctl`](https://gitlab.com/gitlab-org/ci-cd/runner-tools/tlsctl)ツールを使用してRunner側からGitLab証明書をデバッグできます。

### エラー: `x509: certificate signed by unknown authority` {#error-x509-certificate-signed-by-unknown-authority}

このエラーは、executorイメージをプライベートレジストリからプルしようとしたときに、RunnerがexecutorをスケジュールするDockerホストまたはKubernetesノードが、プライベートレジストリの証明書を信頼していない場合に発生する可能性があります。

このエラーを修正するには、関連するルート認証局または証明書チェーンをシステムのトラストストアに追加し、コンテナサービスを再起動します。

UbuntuまたはAlpineを使用している場合は、次のコマンドを実行します。

```shell
cp ca.crt /usr/local/share/ca-certificates/ca.crt
update-ca-certificates
systemctl restart docker.service
```

UbuntuとAlpine以外のオペレーティングシステムの場合は、オペレーティングシステムのドキュメントを参照して、信頼できる証明書をインストールするための適切なコマンドを確認してください。

GitLab RunnerのバージョンとDockerホスト環境によっては、`FF_RESOLVE_FULL_TLS_CHAIN`機能フラグを無効にする必要もある場合があります。

### ジョブでの`apt-get: not found`エラー {#apt-get-not-found-errors-in-jobs}

[`pre_build_script`](advanced-configuration.md#the-runners-section)コマンドは、Runnerが実行するすべてのジョブよりも前に実行されます。`apk`または`apt-get`のようなディストリビューション固有のコマンドは、イシューを引き起こす可能性があります。ユーザースクリプトの証明書をインストールすると、これらのスクリプトが異なるディストリビューションに基づいた[イメージ](https://docs.gitlab.com/ci/yaml/#image)を使用している場合に、CIジョブが失敗する可能性があります。

たとえば、CIジョブがUbuntuイメージとAlpineイメージを実行する場合、AlpineではUbuntuコマンドは失敗します。`apt-get: not found`エラーは、Alpineベースイメージを使用するジョブで発生します。このイシューを解決するには、次のいずれかを実行します。

- ディストリビューションに依存しない`pre_build_script`を作成します。
- [タグ](https://docs.gitlab.com/ci/yaml/#tags)を使用して、Runnerが互換性のあるイメージを持つジョブのみをピックアップするようにします。

### エラー: `self-signed certificate in certificate chain` {#error-self-signed-certificate-in-certificate-chain}

CI/CDジョブが次のエラーで失敗します。

```plaintext
fatal: unable to access 'https://gitlab.example.com/group/project.git/': SSL certificate problem: self-signed certificate in certificate chain
```

ただし[OpenSSLデバッグコマンド](https://docs.gitlab.com/omnibus/settings/ssl/ssl_troubleshooting/#useful-openssl-debugging-commands)ではエラーが検出されません。

このエラーは、Gitが接続時に使用するプロキシが、`openssl s_client`トラブルシューティングコマンドではデフォルトで使用されないプロキシである場合に発生する可能性があります。Gitがプロキシを使用してリポジトリをフェッチするかどうかを検証するには、デバッグを有効にします。

```yaml
variables:
  GIT_CURL_VERBOSE: 1
```

Gitがプロキシを使用しないようにするには、`NO_PROXY`変数にGitLabホスト名が含まれているようにします。

```yaml
variables:
  NO_PROXY: gitlab.example.com
```
