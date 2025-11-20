---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runner Helm Chartを設定する
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

オプションの設定をGitLab Runner Helmチャートに追加できます。

## 設定テンプレートでキャッシュを使用する {#use-the-cache-with-a-configuration-template}

設定テンプレートでキャッシュを使用するには、`values.yaml`で次の変数を設定します:

- `runners.cache.secretName`: オブジェクトストレージプロバイダーのシークレット名。オプションは、`s3access`、`gcsaccess`、`google-application-credentials`、または`azureaccess`です。
- `runners.config`: TOML形式の[キャッシュ](../configuration/advanced-configuration.md#the-runnerscache-section)に関するその他の設定。

### Amazon S3 {#amazon-s3}

[静的認証情報を使用するAmazon S3](https://aws.amazon.com/blogs/security/wheres-my-secret-access-key/)を設定するには、次の手順に従います:

1. 次の例を`values.yaml`に追加します。必要に応じて値を変更してください:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "s3"
           Path = "runner"
           Shared = true
           [runners.cache.s3]
             ServerAddress = "s3.amazonaws.com"
             BucketName = "my_bucket_name"
             BucketLocation = "eu-west-1"
             Insecure = false
             AuthenticationType = "access-key"

     cache:
         secretName: s3access
   ```

1. `accesskey`と`secretkey`を含むKubernetesのシークレット`s3access`を作成します:

   ```shell
   kubectl create secret generic s3access \
       --from-literal=accesskey="YourAccessKey" \
       --from-literal=secretkey="YourSecretKey"
   ```

### Google Cloud Storage（GCS） {#google-cloud-storage-gcs}

Google Cloud Storageは、静的な認証情報を使用して複数の方法で設定できます。

#### 直接設定された静的認証情報 {#static-credentials-directly-configured}

[アクセスIDとプライベートキーを含む](../configuration/advanced-configuration.md#the-runnerscache-section)認証情報を使用してGCSを設定するには、次の手順に従います:

1. 次の例を`values.yaml`に追加します。必要に応じて値を変更してください:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "gcs"
           Path = "runner"
           Shared = true
           [runners.cache.gcs]
             BucketName = "runners-cache"

     cache:
       secretName: gcsaccess
   ```

1. `gcs-access-id`と`gcs-private-key`を含むKubernetesのシークレット`gcsaccess`を作成します:

   ```shell
   kubectl create secret generic gcsaccess \
       --from-literal=gcs-access-id="YourAccessID" \
       --from-literal=gcs-private-key="YourPrivateKey"
   ```

#### GCPからダウンロードしたJSONファイル内の静的認証情報 {#static-credentials-in-a-json-file-downloaded-from-gcp}

Google Cloud Platformからダウンロードした[JSONファイル内の認証情報を使用してGCSを設定する](../configuration/advanced-configuration.md#the-runnerscache-section)には、次の手順に従います:

1. 次の例を`values.yaml`に追加します。必要に応じて値を変更してください:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "gcs"
           Path = "runner"
           Shared = true
           [runners.cache.gcs]
             BucketName = "runners-cache"

     cache:
         secretName: google-application-credentials

   secrets:
     - name: google-application-credentials
   ```

1. `google-application-credentials`という名前のKubernetesのシークレットを作成し、このシークレットを含むJSONファイルを読み込みます。必要に応じてパスを変更します:

   ```shell
   kubectl create secret generic google-application-credentials \
       --from-file=gcs-application-credentials-file=./PATH-TO-CREDENTIALS-FILE.json
   ```

### Azure {#azure}

[Azure Blob Storageを設定する](../configuration/advanced-configuration.md#the-runnerscacheazure-section)には、次の手順に従います:

1. 次の例を`values.yaml`に追加します。必要に応じて値を変更してください:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [runners.cache]
           Type = "azure"
           Path = "runner"
           Shared = true
           [runners.cache.azure]
             ContainerName = "CONTAINER_NAME"
             StorageDomain = "blob.core.windows.net"

     cache:
         secretName: azureaccess
   ```

1. `azure-account-name`と`azure-account-key`を含むKubernetesのシークレット`azureaccess`を作成します:

   ```shell
   kubectl create secret generic azureaccess \
       --from-literal=azure-account-name="YourAccountName" \
       --from-literal=azure-account-key="YourAccountKey"
   ```

Helmチャートのキャッシュの詳細については、[`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml)を参照してください。

### 永続ボリュームクレーム {#persistent-volume-claim}

どのオブジェクトストレージオプションも動作しない場合は、キャッシュに永続ボリュームクレーム（PVC）を使用できます。

PVCを使用するようにキャッシュを設定するには、次のようにします:

1. ジョブポッドが実行されるネームスペースで[PVCを作成](https://kubernetes.io/docs/concepts/storage/persistent-volumes/#persistentvolumeclaims)します。

   {{< alert type="note" >}}

   複数のジョブポッドが同じキャッシュPVCにアクセスできるようにする場合は、`ReadWriteMany`アクセスモードにする必要があります。

   {{< /alert >}}

1. PVCを`/cache`ディレクトリにマウントします:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           image = "ubuntu:22.04"
         [[runners.kubernetes.volumes.pvc]]
           name = "cache-pvc"
           mount_path = "/cache"
   ```

## RBACサポートを有効にする {#enable-rbac-support}

クラスターでRBAC（ロールベースのアクセス制御）が有効になっている場合、このチャートにより作成されるチャート独自サービスアカウントや[自分で作成するサービスアカウント](https://kubernetes.io/docs/reference/access-authn-authz/rbac/#service-account-permissions)を使用することができます。

- チャートにサービスアカウントを作成させるには、`rbac.create`をtrueに設定します:

  ```yaml
  rbac:
    create: true
  ```

- 既存のサービスアカウントを使用するには、`serviceAccount.name`を設定します:

  ```yaml
  rbac:
    create: false
  serviceAccount:
    create: false
    name: your-service-account
  ```

## Runnerの最大並行処理を制御する {#control-maximum-runner-concurrency}

Kubernetesにデプロイされた1つのRunnerは、追加のRunnerポッドを開始することで、複数のジョブを並列実行できます。一度に実行可能なポッドの最大数を変更するには、[`concurrent`設定](../configuration/advanced-configuration.md#the-global-section)を編集します。デフォルトは`10`です:

```yaml
## Configure the maximum number of concurrent jobs
## ref: https://docs.gitlab.com/runner/configuration/advanced-configuration/#the-global-section
##
concurrent: 10
```

この設定の詳細については、GitLab Runnerの高度な設定のドキュメントの[グローバルセクション](../configuration/advanced-configuration.md#the-global-section)を参照してください。

## GitLab RunnerでDocker-in-Dockerコンテナを実行する {#run-docker-in-docker-containers-with-gitlab-runner}

GitLab RunnerでDocker-in-Dockerコンテナを使用するには、次のようにします:

- 有効にするには、[Runnerに特権コンテナを使用する](#use-privileged-containers-for-the-runners)を参照してください。
- Docker-in-Dockerの実行方法については、[GitLab Runnerのドキュメント](../executors/kubernetes/_index.md#using-docker-in-builds)を参照してください。

## Runnerに特権コンテナを使用する {#use-privileged-containers-for-the-runners}

GitLab CI/CDジョブでDocker実行可能ファイルを使用するには、特権コンテナを使用するようにRunnerを設定します。

前提要件:

- リスクを理解していること。リスクについての説明は[GitLab CI/CD Runnerドキュメント](../executors/kubernetes/_index.md#using-docker-in-builds)に記載されています。
- GitLab RunnerインスタンスがGitLabの特定のプロジェクトに登録されており、そのCI/CDジョブを信頼していること。

`values.yaml`で特権モードを有効にするには、次の行を追加します:

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        # Run all containers with the privileged flag enabled.
        privileged = true
        ...
```

詳細については、[`[runners.kubernetes]`](../configuration/advanced-configuration.md#the-runnerskubernetes-section)セクションに関する高度な設定の情報を参照してください。

## プライベートレジストリのイメージを使用する {#use-an-image-from-a-private-registry}

プライベートレジストリのイメージを使用するには、`imagePullSecrets`を構成します。

1. CI/CDジョブに使用するKubernetesネームスペースに1つ以上のシークレットを作成します。このコマンドは、`image_pull_secrets`で機能するシークレットを作成します:

   ```shell
   kubectl create secret docker-registry <SECRET_NAME> \
     --namespace <NAMESPACE> \
     --docker-server="https://<REGISTRY_SERVER>" \
     --docker-username="<REGISTRY_USERNAME>" \
     --docker-password="<REGISTRY_PASSWORD>"
   ```

1. GitLab Runner Helm Chartバージョン0.53.x以降では、`config.toml`で`runners.config`に指定されているテンプレートからの`image_pull_secret`を設定します:

   ```yaml
   runners:
     config: |
       [[runners]]
         [runners.kubernetes]
           ## Specify one or more imagePullSecrets
           ## ref: https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/
           ##
           image_pull_secrets = [your-image-pull-secret]
   ```

   詳細については、Kubernetesドキュメントの[Pull an image from a private registry](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)を参照してください。

1. GitLab Runner Helmチャートバージョン0.52以前の場合は、`values.yaml`で`runners.imagePullSecrets`の値を設定します。この値を設定すると、コンテナは`--kubernetes-image-pull-secrets "<SECRET_NAME>"`をイメージエントリポイントスクリプトに追加します。これにより、Kubernetes executorの`config.toml`の設定で`image_pull_secrets`パラメータを設定する必要がなくなります。

   ```yaml
   runners:
     imagePullSecrets: [your-image-pull-secret]
   ```

{{< alert type="note" >}}

`imagePullSecrets`の値には、`name`タグがプレフィックスとして付加されていません。これはKubernetesリソースでの慣例です。1つのレジストリ認証情報のみを使用する場合でも、この値には1つ以上のシークレット名の配列が必要です。

{{< /alert >}}

`imagePullSecrets`の作成方法の詳細については、Kubernetesドキュメントの[Pull an Image from a Private Registry](https://kubernetes.io/docs/tasks/configure-pod-container/pull-image-private-registry/)を参照してください。

{{< alert type="note" >}}

ジョブポッドの作成時に、GitLab Runnerは自動的にイメージアクセスを次の2つのステップで処理します:

1. GitLab Runnerは、既存のDocker認証情報をKubernetes secretsに変換し、レジストリからイメージをプルできるようにします。手動で設定されたimagePullSecretsがクラスター内に実際に存在するかどうかも確認します。静的に定義された認証情報、認証情報ストア、または認証情報ヘルパーの詳細については、[プライベートコンテナイメージからのイメージへのアクセス](https://docs.gitlab.com/ci/docker/using_docker_images/#access-an-image-from-a-private-container-registry)を参照してください。
1. GitLab Runnerはジョブポッドを作成し、2種類の認証情報（`imagePullSecrets`と変換されたDocker認証情報）をその順にアタッチします。

Kubernetesがコンテナイメージをプルする必要がある場合、機能するものがみつかるまで、認証情報を1つずつ試します。

{{< /alert >}}

## カスタム証明書を使用してGitLabにアクセスする {#access-gitlab-with-a-custom-certificate}

カスタム証明書を使用するには、GitLab Runner Helmチャートに[Kubernetesシークレット](https://kubernetes.io/docs/concepts/configuration/secret/)を提供します。このシークレットは、コンテナの`/home/gitlab-runner/.gitlab-runner/certs`ディレクトリに追加されます:

1. [証明書を準備する](#prepare-your-certificate)
1. [Kubernetesのシークレットを作成する](#create-a-kubernetes-secret)
1. [チャートにシークレットを提供する](#provide-the-secret-to-the-chart)

### 証明書を準備する {#prepare-your-certificate}

Kubernetesシークレットの各キー名は、ディレクトリ内のファイル名として使用されます。ファイルの内容は、キーに関連付けられた値です:

- 使用するファイル名の形式は`<gitlab.hostname>.crt`である必要があります。たとえば`gitlab.your-domain.com.crt`などです。
- 中間証明書を同じファイル内のサーバー証明書に連結します。
- 使用するホスト名は、証明書が登録されているホスト名である必要があります。

### Kubernetesのシークレットを作成する {#create-a-kubernetes-secret}

[自動生成された自己署名ワイルドカード証明書](https://docs.gitlab.com/charts/installation/tls/#option-4-use-auto-generated-self-signed-wildcard-certificate)の手法を使用してGitLab Helmチャートをインストールした場合、シークレットが作成されています。

自動生成された自己署名ワイルドカード証明書を使用してGitLab Helmチャートをインストールしなかった場合は、シークレットを作成します。以下のコマンドは、証明書をシークレットとしてKubernetesに保存し、ファイルとしてGitLab Runnerコンテナに提示します。

- 証明書が現在のディレクトリに含まれており、`<gitlab.hostname.crt>`形式に従っている場合は、必要に応じてこのコマンドを変更します:

  ```shell
  kubectl create secret generic <SECRET_NAME> \
    --namespace <NAMESPACE> \
    --from-file=<CERTIFICATE_FILENAME>
  ```

  - `<NAMESPACE>`: GitLab RunnerをインストールするKubernetesネームスペース。
  - `<SECRET_NAME>`: Kubernetesシークレットリソース名（`gitlab-domain-cert`など）。
  - `<CERTIFICATE_FILENAME>`: 現在のディレクトリ内にある、シークレットにインポートする証明書のファイル名。

- 証明書が別のディレクトリにある場合、または`<gitlab.hostname.crt>`形式に従っていない場合は、ターゲットとして使用するファイル名を指定する必要があります:

  ```shell
  kubectl create secret generic <SECRET_NAME> \
    --namespace <NAMESPACE> \
    --from-file=<TARGET_FILENAME>=<CERTIFICATE_FILENAME>
  ```

  - `<TARGET_FILENAME>`は、Runnerコンテナに提示される証明書ファイルの名前です（`gitlab.hostname.crt`など）。
  - `<CERTIFICATE_FILENAME>`は、シークレットにインポートする証明書のファイル名です。これは、現在のディレクトリを基準とした相対的な名前です。例: `cert-directory/my-gitlab-certificate.crt`。

### チャートにシークレットを提供する {#provide-the-secret-to-the-chart}

`values.yaml`で、`certsSecretName`を同じネームスペース内のKubernetesシークレットオブジェクトのリソース名に設定します。これにより、GitLab Runnerが使用するカスタム証明書を渡すことができます。前述の例では、リソース名は`gitlab-domain-cert`でした:

```yaml
certsSecretName: <SECRET NAME>
```

詳細については、GitLabサーバーを対象とする[自己署名証明書のサポートされているオプション](../configuration/tls-self-signed.md#supported-options-for-self-signed-certificates-targeting-the-gitlab-server)を参照してください。

## ポッドラベルをCI環境変数キーに設定する {#set-pod-labels-to-ci-environment-variable-keys}

`values.yaml`ファイルでは、環境変数をポッドラベルとして使用できません。詳細については、[環境変数キーをポッドラベルとして設定できない](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/173)を参照してください。一時的な解決策として、[このイシューに記載されている回避策](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/issues/173#note_351057890)を使用してください。

## Ubuntuベースの`gitlab-runner` Dockerイメージに切り替える {#switch-to-the-ubuntu-based-gitlab-runner-docker-image}

デフォルトでは、GitLab Runner Helmチャートは、`musl libc`を使用する`gitlab/gitlab-runner`イメージのAlpineバージョンを使用します。`glibc`を使用するUbuntuベースのイメージに切り替える必要がある場合があります。

そのためには、`values.yaml`ファイルで次の値を使用してイメージを指定します:

```yaml
# Specify the Ubuntu image, and set the version. You can also use the `ubuntu` or `latest` tags.
image: gitlab/gitlab-runner:v17.3.0

# Update the security context values to the user ID in the Ubuntu image
securityContext:
  fsGroup: 999
  runAsUser: 999
```

## 非rootユーザーで実行する {#run-with-non-root-user}

デフォルトの場合、非rootユーザーではGitLab Runnerのイメージが動作しません。[GitLab Runner UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766421)イメージと[GitLab Runner Helper UBI](https://gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/container_registry/1766433)イメージは、このような状況に対応して設計されています。

これらのイメージを使用するには、`values.yaml`でGitLab RunnerイメージとGitLab Runner Helperイメージを変更します:

```yaml
image:
  registry: registry.gitlab.com
  image: gitlab-org/ci-cd/gitlab-runner-ubi-images/gitlab-runner-ocp
  tag: v16.11.0

securityContext:
    runAsNonRoot: true
    runAsUser: 999

runners:
    config: |
        [[runners]]
          [runners.kubernetes]
            helper_image = "registry.gitlab.com/gitlab-org/ci-cd/gitlab-runner-ubi-images/gitlab-runner-helper-ocp:x86_64-v16.11.0"
            [runners.kubernetes.pod_security_context]
              run_as_non_root = true
              run_as_user = 59417
```

`run_as_user`は`nonroot`ユーザーのユーザーID（59417）を参照していますが、イメージはどのユーザーIDでも機能します。このユーザーIDがルートグループの一部であることが重要です。ルートグループの一部であっても、特定の特権が付与されるわけではありません。

## FIPS準拠のGitLab Runnerを使用する {#use-a-fips-compliant-gitlab-runner}

[FIPS準拠のGitLab Runner](_index.md#fips-compliant-gitlab-runner)を使用するには、`values.yaml`でGitLab RunnerイメージとHelperイメージを変更します:

```yaml
image:
  registry: docker.io
  image: gitlab/gitlab-runner
  tag: ubi-fips

runners:
    config: |
        [[runners]]
          [runners.kubernetes]
            helper_image_flavor = "ubi-fips"
```

## 設定テンプレートを使用する {#use-a-configuration-template}

[KubernetesでGitLab Runnerビルドポッドの動作を設定する](../executors/kubernetes/_index.md#configuration-settings)には、[設定テンプレートファイル](../register/_index.md#register-with-a-configuration-template)を使用します。設定テンプレートでは、Helmチャートと特定のRunner設定オプションを共有せずに、Runnerの任意のフィールドを設定できます。たとえば、以下のデフォルト設定は`chart`リポジトリの[`values.yaml`ファイルにあります](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml):

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        image = "ubuntu:22.04"
```

`config.toml`が`values.yaml`に埋め込まれているため、`config:`セクションの値はTOMLを使用する必要があります（`<parameter> = <value>`ではなく`<parameter>: <value>`）。

executor固有の設定については、[`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml)ファイルを参照してください。
