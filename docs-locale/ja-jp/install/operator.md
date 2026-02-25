---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
description: Kubernetes用GitLab Operatorを使用してGitLab Runnerをインストールします。
title: GitLab Runner Operatorをインストールする
---

## Red Hat OpenShiftにインストールする {#install-on-red-hat-openshift}

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

OpenShiftのウェブコンソールでOperatorHubのstableチャネルから[GitLab Runner Operator](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator)を使用して、Red Hat OpenShift v4以降にGitLab Runnerをインストールします。インストールが完了すると、新しくデプロイされたGitLab Runnerインスタンスを使用して、GitLab CI/CDジョブを実行できます。各CI/CDジョブは、個別のポッドで実行されます。

### 前提条件 {#prerequisites}

- 管理者権限を持つOpenShift 4.xクラスター
- GitLab Runner登録トークン

### OpenShift Operatorをインストールする {#install-the-openshift-operator}

まず、OpenShift Operatorをインストールする必要があります。

1. OpenShift UIを開き、管理者権限を持つユーザーとしてサインインします。
1. 左側のペインで、**Operators**、**OperatorHub**の順に選択します。
1. メインペインの**All Items**の下で、キーワード`GitLab Runner`を検索します。

   ![GitLab Operator](img/openshift_allitems_v13_3.png)

1. インストールするには、GitLab Runner Operatorを選択します。
1. GitLab Runner Operatorの概要ページで、**Install**を選択します。
1. Install Operatorページで、以下を実行します:
   1. **Update Channel**で、**stable**を選択します。
   1. **Installed Namespace**で、目的のネームスペースを選択し、**インストール**を選択します。

   ![GitLab OperatorのInstallページ](img/openshift_installoperator_v13_3.png)

Installed Operatorsページで、GitLab Operatorの準備ができると、ステータスが**Succeeded**に変わります。

![GitLab Operator Install Status](img/openshift_success_v13_3.png)

## Kubernetesにインストールする {#install-on-kubernetes}

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

[OperatorHub.io](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator)のstableチャネルから[GitLab Runner Operator](https://operatorhub.io/operator/gitlab-runner-operator)を使用して、Kubernetes v1.21以降にGitLab Runnerをインストールします。インストールが完了すると、新しくデプロイされたGitLab Runnerインスタンスを使用して、GitLab CI/CDジョブを実行できます。各CI/CDジョブは、個別のポッドで実行されます。

### 前提条件 {#prerequisites-1}

- Kubernetes v1.21以降
- Cert manager v1.7.1

### Kubernetes Operatorをインストールする {#install-the-kubernetes-operator}

[OperatorHub.io](https://operatorhub.io/operator/gitlab-runner-operator)の手順に従ってください。

1. 前提条件をインストールします。
1. 右上にある**インストール**を選択し、指示に従って`olm`とOperatorをインストールします。

#### GitLab Runnerをインストールする {#install-gitlab-runner}

1. Runner認証トークンを取得します。次のいずれかの方法があります。
   - [インスタンス](https://docs.gitlab.com/ci/runners/runners_scope/#create-an-instance-runner-with-a-runner-authentication-token)、[グループ](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-group-runner-with-a-runner-authentication-token)、または[プロジェクト](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-project-runner-with-a-runner-authentication-token)のRunnerを作成する。
   - `config.toml`ファイルの中でRunner認証トークンを見つける。Runner認証トークンのプレフィックスは`glrt-`です。
1. GitLab Runnerトークンを使用して、シークレットファイルを作成します:

   ```shell
   cat > gitlab-runner-secret.yml << EOF
   apiVersion: v1
   kind: Secret
   metadata:
     name: gitlab-runner-secret
   type: Opaque
   # Only one of the following fields can be set. The Operator fails to register the runner if both are provided.
   # NOTE: runner-registration-token is deprecated and will be removed in GitLab 18.0. You should use runner-token instead.
   stringData:
     runner-token: REPLACE_ME # your project runner token
     # runner-registration-token: "" # your project runner secret
   EOF
   ```

1. 以下を実行して、クラスターに`secret`を作成します:

   ```shell
   kubectl apply -f gitlab-runner-secret.yml
   ```

1. カスタムリソース定義（CRD）ファイルを作成し、次の設定を含めます。

   ```shell
   cat > gitlab-runner.yml << EOF
   apiVersion: apps.gitlab.com/v1beta2
   kind: Runner
   metadata:
     name: gitlab-runner
   spec:
     gitlabUrl: https://gitlab.example.com
     buildImage: alpine
     token: gitlab-runner-secret
   EOF
   ```

1. 次に、コマンドを実行して`CRD`ファイルを適用します:

   ```shell
   kubectl apply -f gitlab-runner.yml
   ```

1. 以下を実行して、GitLab Runnerがインストールされていることを確認します:

   ```shell
   kubectl get runner
   NAME             AGE
   gitlab-runner    5m
   ```

1. Runnerポッドも表示されるはずです:

   ```shell
   kubectl get pods
   NAME                             READY   STATUS    RESTARTS   AGE
   gitlab-runner-bf9894bdb-wplxn    1/1     Running   0          5m
   ```

#### OpenShift用の他のバージョンのGitLab Runner Operatorをインストールする {#install-other-versions-of-gitlab-runner-operator-for-openshift}

Red Hat OperatorHubで使用可能なGitLab Runner Operatorのバージョンを使用しない場合は、別のバージョンをインストールできます。

公式に利用可能なOperatorのバージョンを確認するには、[`gitlab-runner-operator`リポジトリのタグを表示](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/-/tags)します。Operatorが実行しているGitLab Runnerのバージョンを確認するには、目的のコミットまたはタグの`APP_VERSION`ファイルの内容（たとえば、[https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/-/blob/1-17-stable/APP_VERSION](https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/-/blob/1-17-stable/APP_VERSION)）を表示します。

特定のバージョンをインストールするには、この`catalogsource.yaml`ファイルを作成し、`<VERSION>`をタグまたは特定のコミットに置き換えます:

{{< alert type="note" >}}

特定のコミットのイメージを使用する場合、タグの形式は`v0.0.1-<COMMIT>`です。例: `registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/gitlab-runner-operator-catalog-source:v0.0.1-f5a798af`。

{{< /alert >}}

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: CatalogSource
metadata:
  name: gitlab-runner-catalog
  namespace: openshift-marketplace
spec:
  sourceType: grpc
  image: registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/gitlab-runner-operator-catalog-source:<VERSION>
  displayName: GitLab Runner Operators
  publisher: GitLab Community
```

以下を使用して`CatalogSource`を作成します:

```shell
oc apply -f catalogsource.yaml
```

1分以内に、新しいRunnerがOpenShiftクラスターのOperatorHubセクションに表示されるはずです。

## オフライン環境のKubernetesクラスターにGitLab Runner Operatorをインストールする {#install-gitlab-runner-operator-on-kubernetes-clusters-in-offline-environments}

前提条件: 

- インストールプロセスに必要なイメージにアクセスできます。

インストール中にコンテナイメージをプルするために、GitLab Runner Operatorには、外部ネットワーク上のパブリックインターネットへの接続が必要です。オフライン環境にKubernetesクラスターがインストールされている場合は、ローカルイメージレジストリまたはパッケージレジストリを使用して、インストール中にイメージまたはパッケージをプルします。

ローカルリポジトリは、次のイメージを提供する必要があります:

| 画像                                                 | デフォルト値 |
|-------------------------------------------------------|---------------|
| **GitLab Runner Operator**イメージ                      | `registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/gitlab-runner-operator:vGITLAB_RUNNER_OPERATOR_VERSION` |
| **GitLab Runner**と**GitLab Runner Helper**のイメージ | これらのイメージは、GitLab Runner UBIイメージレジストリからダウンロードされ、Runnerカスタムリソースのインストール時に使用されます。使用するバージョンは、要件によって異なります。 |
| **RBAC Proxy**イメージ                                  | `registry.gitlab.com/gitlab-org/gl-openshift/gitlab-runner-operator/openshift4/ose-kube-rbac-proxy:v4.13.0` |

1. ダウンロードしたソフトウェアパッケージとコンテナイメージをホストするために、切断されたネットワーク環境でローカルリポジトリまたはレジストリをセットアップします。使用できるモデルは次のとおりです:

   - コンテナイメージ用のDockerレジストリ。
   - Kubernetesバイナリと依存関係のためのローカルパッケージレジストリ。

1. GitLab Runner Operator v1.23.2以降の場合は、`operator.k8s.yaml`ファイルの最新バージョンをダウンロードします:

   ```shell
   curl -O "https://gitlab.com/gitlab-org/gl-openshift/gitlab-runner-
   operator/-/releases/vGITLAB_RUNNER_OPERATOR_VERSION/downloads/operator.k8s.yaml"
   ```

1. `operator.k8s.yaml`ファイルで、次のURLを更新します:

   - `GitLab Runner Operator image`
   - `RBAC Proxy image`

1. 更新されたバージョンの`operator.k8s.yaml`ファイルをインストールします:

   ```shell
   kubectl apply -f PATH_TO_UPDATED_OPERATOR_K8S_YAML
   GITLAB_RUNNER_OPERATOR_VERSION = 1.23.2+
   ```

## Operatorをアンインストール {#uninstall-operator}

### Red Hat OpenShiftでアンインストールする {#uninstall-on-red-hat-openshift}

1. Runner `CRD`を削除します:

   ```shell
   kubectl delete -f gitlab-runner.yml
   ```

1. `secret`を削除します:

   ```shell
   kubectl delete -f gitlab-runner-secret.yml
   ```

1. [Webコンソールを使用してクラスターからOperatorを削除する](https://docs.redhat.com/en/documentation/openshift_container_platform/4.7/html/operators/administrator-tasks#olm-deleting-operators-from-a-cluster-using-web-console_olm-deleting-operators-from-a-cluster)については、Red Hatドキュメントの手順に従ってください。

### Kubernetesでアンインストールする {#uninstall-on-kubernetes}

1. Runner `CRD`を削除します:

   ```shell
   kubectl delete -f gitlab-runner.yml
   ```

1. `secret`を削除します:

   ```shell
   kubectl delete -f gitlab-runner-secret.yml
   ```

1. Operatorサブスクリプションを削除します:

   ```shell
   kubectl delete subscription my-gitlab-runner-operator -n operators
   ```

1. インストールされている`CSV`のバージョンを確認します:

   ```shell
   kubectl get clusterserviceversion -n operators
   NAME                            DISPLAY         VERSION   REPLACES   PHASE
   gitlab-runner-operator.v1.7.0   GitLab Runner   1.7.0                Succeeded
   ```

1. `CSV`を削除します:

   ```shell
   kubectl delete clusterserviceversion gitlab-runner-operator.v1.7.0 -n operators
   ```

#### 設定 {#configuration}

OpenShiftでGitLab Runnerを設定するには、[OpenShiftでのGitLab Runnerの設定](../configuration/configuring_runner_operator.md)ページを参照してください。

#### モニタリング {#monitoring}

GitLab Runner Operatorデプロイメントのモニタリングとメトリクス収集を有効にするには、[GitLab Runnerのモニタリング](../monitoring/_index.md#monitor-operator-managed-gitlab-runners)を参照してください。
