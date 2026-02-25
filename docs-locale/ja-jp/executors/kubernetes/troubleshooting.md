---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Kubernetes executorのトラブルシューティング
---

Kubernetes executorの使用時に発生する一般的なエラーを以下に示します。

## `Job failed (system failure): timed out waiting for pod to start` {#job-failed-system-failure-timed-out-waiting-for-pod-to-start}

クラスターが`poll_timeout`で定義されたタイムアウトになる前にビルドポッドをスケジュールできない場合、ビルドポッドはエラーを返します。[Kubernetesスケジューラ](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-lifetime)は、それを削除できる必要があります。

このイシューを修正するには、`config.toml`ファイルの`poll_timeout`値を大きくします。

## `context deadline exceeded` {#context-deadline-exceeded}

ジョブログの`context deadline exceeded`エラーは通常、Kubernetes APIクライアントが特定のクラスターAPIリクエストでタイムアウトになったことを示しています。

兆候がないか、[`kube-apiserver`クラスターコンポーネントのメトリクス](https://kubernetes.io/docs/concepts/cluster-administration/system-metrics/)をチェックします:

- 応答レイテンシーの増加。
- ポッド、シークレット、ConfigMap、その他のコア（v1）リソースに対する一般的な作成または削除操作のエラー率。

`kube-apiserver`操作からのタイムアウト駆動型エラーのログは、次のように表示される場合があります:

```plaintext
Job failed (system failure): prepare environment: context deadline exceeded
Job failed (system failure): prepare environment: setting up build pod: context deadline exceeded
```

場合によっては、`kube-apiserver`エラー応答は、そのサブコンポーネントの障害（Kubernetesクラスターの`etcdserver`など）に関する追加の詳細を提供する場合があります:

```plaintext
Job failed (system failure): prepare environment: etcdserver: request timed out
Job failed (system failure): prepare environment: etcdserver: leader changed
Job failed (system failure): prepare environment: Internal error occurred: resource quota evaluates timeout
```

これらの`kube-apiserver`サービス障害は、ビルドポッドの作成中、および完了後のクリーンアップ試行中に発生する可能性があります:

```plaintext
Error cleaning up secrets: etcdserver: request timed out
Error cleaning up secrets: etcdserver: leader changed

Error cleaning up pod: etcdserver: request timed out, possibly due to previous leader failure
Error cleaning up pod: etcdserver: request timed out
Error cleaning up pod: context deadline exceeded
```

## `Dial tcp xxx.xx.x.x:xxx: i/o timeout` {#dial-tcp-xxxxxxxxxx-io-timeout}

これはKubernetesのエラーで、通常、RunnerマネージャーからKubernetes APIサーバーに到達できないことを示します。この問題を解決するには:

- ネットワークセキュリティポリシーを使用する場合は、通常、ポート443またはポート6443、あるいはその両方で、Kubernetes APIへのアクセスを許可してください。
- Kubernetes APIが実行されていることを確認してください。

## Kubernetes APIとの通信を試みるときに接続が拒否されました {#connection-refused-when-attempting-to-communicate-with-the-kubernetes-api}

GitLab RunnerがKubernetes APIにリクエストを送信して失敗した場合、[`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver)が過負荷状態で、APIリクエストを受け付けられない、または処理できないことが原因である可能性があります。

## `Error cleaning up pod`と`Job failed (system failure): prepare environment: waiting for pod running` {#error-cleaning-up-pod-and-job-failed-system-failure-prepare-environment-waiting-for-pod-running}

Kubernetesがジョブポッドをタイムリーにスケジュールできない場合、次のエラーが発生します。GitLab Runnerはポッドの準備ができるのを待ちますが、失敗するとポッドのクリーンアップを試みますが、これも失敗する可能性があります。

```plaintext
Error: Error cleaning up pod: Delete "https://xx.xx.xx.x:443/api/v1/namespaces/gitlab-runner/runner-0001": dial tcp xx.xx.xx.x:443 connect: connection refused

Error: Job failed (system failure): prepare environment: waiting for pod running: Get "https://xx.xx.xx.x:443/api/v1/namespaces/gitlab-runner/runner-0001": dial tcp xx.xx.xx.x:443 connect: connection refused
```

トラブルシューティングを行うには、Kubernetesのプライマリノードと、[`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver)インスタンスを実行するすべてのノードを確認してください。クラスター上でスケールアップしたいターゲットポッド数を管理するために必要なすべてのリソースがそれらに備わっていることを確認してください。

GitLab Runnerがポッドが`Ready`ステータスに達するまで待機する時間を変更するには、[`poll_timeout`](_index.md#other-configtoml-settings)設定を使用します。

ポッドがどのようにスケジュールされるか、または時間どおりにスケジュールされない理由をよりよく理解するには、[Kubernetesスケジューラについてお読みください](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/)。

## `request did not complete within requested timeout` {#request-did-not-complete-within-requested-timeout}

ビルドポッドの作成中に観測されたメッセージ`request did not complete within requested timeout`は、Kubernetesクラスターで構成された[アドミッションコントロールWebhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)がタイムアウトしていることを示します。

アドミッションコントロールWebhookは、スコープが設定されているすべてのAPIリクエストに対するクラスターレベルの管理制御インターセプトであり、時間内に実行されない場合、障害を引き起こす可能性があります。

アドミッションコントロールWebhookは、傍受するAPIリクエストとネームスペースネームスペースソースをきめ細かく制御できるフィルターをサポートしています。GitLab RunnerからのKubernetes API呼び出しがアドミッションコントロールWebhookを通過する必要がない場合は、GitLab Runnerネームスペースを無視するように[Webhookのセレクター/フィルターの構成](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-objectselector)を変更するか、[GitLab Runner Helmチャート`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/57e026d7f43f63adc32cdd2b21e6d450abcf0686/values.yaml#L490-500)で`podAnnotations`または`podLabels`を構成して、GitLab Runnerポッドに除外ラベル/注釈を適用できます。

たとえば、[DataDogアドミッションコントロールWebhook](https://docs.datadoghq.com/containers/cluster_agent/admission_controller/?tab=operator)がGitLab Runnerマネージャーポッドによって行われたAPIリクエストを傍受しないようにするには、次を追加できます:

```yaml
podLabels:
  admission.datadoghq.com/enabled: false
```

KubernetesクラスターのアドミッションコントロールWebhookを一覧表示するには、次を実行します:

```shell
kubectl get validatingwebhookconfiguration -o yaml
kubectl get mutatingwebhookconfiguration -o yaml
```

アドミッションコントロールWebhookがタイムアウトすると、次の形式のログが確認できます:

```plaintext
Job failed (system failure): prepare environment: Timeout: request did not complete within requested timeout
Job failed (system failure): prepare environment: setting up credentials: Timeout: request did not complete within requested timeout
```

アドミッションコントロールWebhookからの障害は、代わりに次のように表示される場合があります:

```plaintext
Job failed (system failure): prepare environment: setting up credentials: Internal error occurred: failed calling webhook "example.webhook.service"
```

## エラー`Could not resolve host: example.com` {#error-could-not-resolve-host-examplecom}

[ヘルパーイメージ](../../configuration/advanced-configuration.md#helper-image)の`alpine`フレーバーを使用している場合、Alpineの`musl`のDNSリゾルバーに関連する[DNSイシュー](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4129)が発生する可能性があります。エラーは次のように表示される場合があります:

- `fatal: unable to access 'https://gitlab-ci-token:token@example.com/repo/proj.git/': Could not resolve host: example.com`

このイシューを解決するには、`helper_image_flavor = "ubuntu"`オプションを使用します。

## `docker: Cannot connect to the Docker daemon at tcp://docker:2375. Is the docker daemon running?` {#docker-cannot-connect-to-the-docker-daemon-at-tcpdocker2375-is-the-docker-daemon-running}

このエラーは、[Docker-in-Docker](_index.md#using-dockerdind)を使用している場合に、DINDサービスが完全に起動する前にアクセスしようとすると発生する可能性があります。詳細については、[このイシュー](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27215)を参照してください。

## `curl: (35) OpenSSL SSL_connect: SSL_ERROR_SYSCALL in connection to github.com:443` {#curl-35-openssl-ssl_connect-ssl_error_syscall-in-connection-to-githubcom443}

このエラーは、[Docker-in-Docker](_index.md#using-dockerdind)を使用している場合に、DINDの最大転送ユニット（MTU）がKubernetesオーバーレイネットワークよりも大きい場合に発生する可能性があります。DINDはデフォルトのMTU 1500を使用しますが、これはデフォルトのオーバーレイネットワーク全体をルーティングするには大きすぎます。DIND MTUは、サービス定義内で変更できます:

```yaml
services:
  - name: docker:dind
    command: ["--mtu=1450"]
```

## `MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown is not supported by windows` {#mountvolumesetup-failed-for-volume-kube-api-access-xxxxx--chown-is-not-supported-by-windows}

CI/CDジョブを実行すると、次のようなエラーが発生する可能性があります:

```plaintext
MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown c:\var\lib\kubelet\pods\xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx\volumes\kubernetes.io~projected\kube-api-access-xxxxx\..2022_07_07_20_52_19.102630072\token: not supported by windows
```

このイシューは、[ノードセレクターを使用](_index.md#specify-the-node-to-execute-builds)して、異なるオペレーティングシステムとアーキテクチャを持つノードでビルドを実行する場合に発生します。

このイシューを修正するには、Runnerマネージャーポッドが常にLinuxノードでスケジュールされるように`nodeSelector`を構成します。たとえば、[`values.yaml`ファイル](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml)には、次のものが含まれている必要があります:

```yaml
nodeSelector:
  kubernetes.io/os: linux
```

## ビルドポッドにRunner IAMロールではなく、ワーカーノードのIAMロールが割り当てられています {#build-pods-are-assigned-the-worker-nodes-iam-role-instead-of-runner-iam-role}

このイシューは、ワーカーノードのIAMロールに正しいロールを引き受ける権限がない場合に発生します。これを修正するには、`sts:AssumeRole`権限をワーカーノードのIAMロールの信頼関係に追加します:

```json
{
    "Effect": "Allow",
    "Principal": {
        "AWS": "arn:aws:iam::<AWS_ACCOUNT_NUMBER>:role/<IAM_ROLE_NAME>"
    },
    "Action": "sts:AssumeRole"
}
```

## エラー: `pull_policy ([Always]) defined in GitLab pipeline config is not one of the allowed_pull_policies` {#error-pull_policy-always-defined-in-gitlab-pipeline-config-is-not-one-of-the-allowed_pull_policies}

このイシューは、`.gitlab-ci.yml`で`pull_policy`を指定したが、Runnerの構成ファイルに構成されたポリシーがない場合に発生します。エラーは次のように表示される場合があります:

- `Preparation failed: invalid pull policy for image 'image-name:latest': pull_policy ([Always]) defined in GitLab pipeline config is not one of the allowed_pull_policies ([])`

このイシューを修正するには、[Dockerプルポリシーの制限](_index.md#restrict-docker-pull-policies)に従って、構成に`allowed_pull_policies`を追加します。

## バックグラウンドプロセスによりジョブがハングアップし、タイムアウトになります {#background-processes-cause-jobs-to-hang-and-timeout}

ジョブの実行中に開始されたバックグラウンドプロセスは、[ビルドジョブが終了するのを防ぐ](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2880)ことができます。これを回避するには、次のことができます:

- プロセスをダブルフォークします。例: `command_to_run < /dev/null &> /dev/null &`。
- ジョブスクリプトを終了する前にプロセスを強制終了します。

## キャッシュ関連の`permission denied`エラー {#cache-related-permission-denied-errors}

ジョブで生成されるファイルとフォルダーには、特定のUNIX所有権と権限があります。ファイルとフォルダーがアーカイブまたは抽出されると、UNIXの詳細が保持されます。ただし、ファイルとフォルダーは、[ヘルパーイメージ](../../configuration/advanced-configuration.md#helper-image)の`USER`構成と一致しない場合があります。

`Creating cache ...`ステップで権限関連のエラーが発生した場合は、次のことができます:

- 解決策として、ソースデータが変更されているかどうかを調査します。たとえば、キャッシュされたファイルを作成するジョブスクリプトなどです。
- 回避策として、一致する[chown](https://linux.die.net/man/1/chown)コマンドと[chmod](https://linux.die.net/man/1/chmod)コマンドを追加します。 [(`before_`/`after_`)`script:`ディレクティブ](https://docs.gitlab.com/ci/yaml/#default)へ。

## 初期化システムを備えたビルドコンテナ内の明らかに冗長なシェルプロセス {#apparently-redundant-shell-process-in-build-container-with-init-system}

プロセスツリーには、次のいずれかの場合にシェルプロセスが含まれる場合があります:

- `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY`が`false`で、`FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR`が`true`の場合。
- ビルドイメージの`ENTRYPOINT`は、初期化システム（`tini-init`や`dumb-init`など）です。

```shell
UID    PID   PPID  C STIME TTY          TIME CMD
root     1      0  0 21:58 ?        00:00:00 /scripts-37474587-5556589047/dumb-init -- sh -c if [ -x /usr/local/bin/bash ]; then .exec /usr/local/bin/bash  elif [ -x /usr/bin/bash ]; then .exec /usr/bin/bash  elif [ -x /bin/bash ]; then .exec /bin/bash  elif [ -x /usr/local/bin/sh ]; then .exec /usr/local/bin/sh  elif [ -x /usr/bin/sh ]; then .exec /usr/bin/sh  elif [ -x /bin/sh ]; then .exec /bin/sh  elif [ -x /busybox/sh ]; then .exec /busybox/sh  else .echo shell not found .exit 1 fi
root     7      1  0 21:58 ?        00:00:00 /usr/bin/bash <---------------- WHAT IS THIS???
root    26      1  0 21:58 ?        00:00:00 sh -c (/scripts-37474587-5556589047/detect_shell_script /scripts-37474587-5556589047/step_script 2>&1 | tee -a /logs-37474587-5556589047/output.log) &
root    27     26  0 21:58 ?        00:00:00  \_ /usr/bin/bash /scripts-37474587-5556589047/step_script
root    32     27  0 21:58 ?        00:00:00  |   \_ /usr/bin/bash /scripts-37474587-5556589047/step_script
root    37     32  0 21:58 ?        00:00:00  |       \_ ps -ef --forest
root    28     26  0 21:58 ?        00:00:00  \_ tee -a /logs-37474587-5556589047/output.log
```

このシェルプロセスは、`sh`、`bash`、または`busybox`の可能性があり、`PPID`が1、`PID`が6または7の場合、初期化システムによって実行されるシェル検出スクリプトによって開始されるシェルです（上記の`PID` 1）。このプロセスは冗長ではなく、ビルドコンテナが初期化システムで実行されている場合の典型的な操作です。

## Runnerポッドは、登録が成功したにもかかわらず、ジョブの結果を実行できず、タイムアウトになります {#runner-pod-fails-to-run-job-results-and-times-out-despite-successful-registration}

RunnerポッドはGitLabに登録すると、ジョブの実行を試みますが、実行されず、最終的にジョブはタイムアウトになります。次のエラーが報告されます:

```plaintext
There has been a timeout failure or the job got stuck. Check your timeout limits or try again.

This job does not have a trace.
```

この場合、Runnerは次のエラーを受け取る可能性があります。

```plaintext
HTTP 204 No content response code when connecting to the `jobs/request` API.
```

このイシューのトラブルシューティングを行うには、APIにPOSTリクエストを手動で送信して、TCP接続がハングしているかどうかを検証します。TCP接続がハングしている場合、RunnerはCIジョブペイロードをリクエストできない可能性があります。

## `failed to reserve container name` (`gcs-fuse-csi-driver`が使用されている場合) {#failed-to-reserve-container-name-for-init-permissions-container-when-gcs-fuse-csi-driver-is-used}

`gcs-fuse-csi-driver` `csi`ドライバーは、[initコンテナのボリュームのマウントをサポートしていません](https://github.com/GoogleCloudPlatform/gcs-fuse-csi-driver/issues/38)。これにより、このドライバーを使用するときにinitコンテナの起動が失敗する可能性があります。[Kubernetes 1.28で導入された](https://kubernetes.io/blog/2023/08/25/native-sidecar-containers/)機能は、このバグを解決するために、ドライバーのプロジェクトでサポートされている必要があります。

## エラー: `only read-only root filesystem container is allowed` {#error-only-read-only-root-filesystem-container-is-allowed}

読み取り専用でマウントされたルートファイルシステム上でコンテナを実行するように強制するアドミッションコントロールポリシーを持つクラスターでは、このエラーは次の場合に表示される可能性があります:

- GitLab Runnerをインストールします。
- GitLab Runnerがビルドポッドをスケジュールしようとします。

これらのアドミッションコントロールポリシーは通常、[Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/)や[Kyverno](https://kyverno.io/)などのアドミッションコントロールコントローラーによって適用されます。たとえば、読み取り専用のルートファイルシステム上でコンテナを実行するように強制するポリシーは、[`readOnlyRootFilesystem`](https://open-policy-agent.github.io/gatekeeper-library/website/validation/read-only-root-filesystem/) Gatekeeperポリシーです。

この問題を解決するには:

- クラスターにデプロイされたすべてのポッドは、アドミッションコントロールコントローラーがポッドをブロックしないように、`securityContext.readOnlyRootFilesystem`をコンテナの`true`に設定して、アドミッションコントロールポリシーに準拠する必要があります。
- ルートファイルシステムが読み取り専用でマウントされていても、コンテナは正常に実行され、ファイルシステムに書き込むことができる必要があります。

### GitLab Runnerの場合 {#for-gitlab-runner}

[GitLab Runner Helmチャート](../../install/kubernetes.md)でGitLab Runnerがデプロイされている場合、次のものを持つようにGitLabチャートの構成を更新する必要があります:

- 適切な`securityContext`値:

  ```yaml
  <...>
  securityContext:
    readOnlyRootFilesystem: true
  <...>
  ```

- ポッドが書き込める場所にマウントされた書き込み可能なファイルシステム:

  ```yaml
  <...>
  volumeMounts:
  - name: tmp-dir
    mountPath: /tmp
  volumes:
  - name: tmp-dir
    emptyDir:
      medium: "Memory"
  <...>
  ```

### ビルドポッドの場合 {#for-the-build-pod}

ビルドポッドを読み取り専用のルートファイルシステムで実行するには、`config.toml`で異なるコンテナのセキュリティコンテキストを構成します。ビルドポッドに渡されるGitLabチャート変数`runners.config`を設定できます:

```yaml
runners:
  config: |
   <...>
   [[runners]]
     [runners.kubernetes.build_container_security_context]
       read_only_root_filesystem = true
     [runners.kubernetes.init_permissions_container_security_context]
       read_only_root_filesystem = true
     [runners.kubernetes.helper_container_security_context,omitempty]
       read_only_root_filesystem = true
     # This section is only needed if jobs with services are used
     [runners.kubernetes.service_container_security_context,omitempty]
       read_only_root_filesystem = true
   <...>
```

ビルドポッドとそのコンテナを読み取り専用ファイルシステム上で正常に実行するには、ビルドポッドが書き込める場所に書き込み可能なファイルシステムが必要です。少なくとも、これらの場所はビルドおよびホームディレクトリです。ビルドプロセスに、必要に応じて他の場所への書き込みアクセス権があることを確認してください。

一般に、ホームディレクトリは、プログラムが正常な実行に必要な構成やその他のデータを保存できるように、書き込み可能である必要があります。`git`バイナリは、ホームディレクトリに書き込むことができると予想されるプログラムの一例です。

異なるコンテナイメージでのパスに関係なく、ホームディレクトリを書き込み可能にするには:

1. （どのビルドイメージを使用しているかに関係なく）安定したパスにボリュームをマウントします。
1. すべてのビルドに対して、環境変数`$HOME`をグローバルに設定して、ホームディレクトリを変更します。

GitLabチャート変数`runners.config`の値を更新することにより、`config.toml`でビルドポッドとそのコンテナを構成できます。

```yaml
runners:
  config: |
   <...>
   [[runners]]
     environment = ["HOME=/build_home"]
     [[runners.kubernetes.volumes.empty_dir]]
       name = "repo"
       mount_path = "/builds"
     [[runners.kubernetes.volumes.empty_dir]]
       name = "build-home"
       mount_path = "/build_home"
   <...>
```

{{< alert type="note" >}}

`emptyDir`の代わりに、他の[サポートされているボリュームタイプ](_index.md#configure-volume-types)を使用できます。明示的に処理され、ビルド成果物として保存されないすべてのファイルは通常一時的であるため、ほとんどの場合`emptyDir`が機能します。

{{< /alert >}}

## AWS EKS: ポッドのクリーンアップエラー: 「Runner - \*\*」が見つからない、またはステータスが「失敗」 {#aws-eks-error-cleaning-up-pod-pods-runner--not-found-or-status-is-failed}

Amazon EKSゾーンのリバランシング機能は、オートスケールグループ内のAvailability Zoneのバランスを取ります。この機能は、あるAvailability Zoneのノードを停止し、別のAvailability Zoneで作成する可能性があります。

Runnerジョブを停止して別のノードに移動することはできません。このエラーを解決するには、Runnerジョブに対してこの機能を無効にします。

## Windowsコンテナではサポートされていないサービス {#services-not-supported-with-windows-containers}

Windowsノードで[サービス](https://docs.gitlab.com/ci/services/)を使用しようとすると、次のエラーで失敗する可能性があります:

- `ERROR: Job failed (system failure): prepare environment: admission webhook "windows.common-webhooks.networking.gke.io" denied the request: spec.hostAliases: Invalid value: []v1.HostAlias{v1.HostAlias{IP:"127.0.0.1", Hostnames:[]string{"<your windows image>"}}}: Windows does not support this field.`

Kubernetesランタイムによっては、エラーが報告されるか、黙って無視される可能性があります。たとえば、GKEはエラーを報告します。

サービスは、Kubernetes executorの`hostAlias`を使用して実装されます。これは、Windowsコンテナではサポートされていません。
