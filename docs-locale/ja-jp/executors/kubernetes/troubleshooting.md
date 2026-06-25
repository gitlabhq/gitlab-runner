---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Kubernetes executorのトラブルシューティング
---

Kubernetes executorを使用する際、一般的に次のエラーが発生します。

## `Job failed (system failure): timed out waiting for pod to start` {#job-failed-system-failure-timed-out-waiting-for-pod-to-start}

`poll_timeout`で定義されたタイムアウトまでにクラスターがビルドポッドをスケジュールできない場合、ビルドポッドはエラーを返します。[Kubernetesスケジューラー](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-lifetime)で、このポッドを削除できるはずです。

この問題を修正するには、`config.toml`ファイルの`poll_timeout`値を増やします。

## `context deadline exceeded` {#context-deadline-exceeded}

ジョブログにおける`context deadline exceeded`エラーは通常、特定のクラスターAPIリクエストに対してKubernetes APIクライアントがタイムアウトに達したことを示しています。

[`kube-apiserver`クラスターコンポーネントのメトリクス](https://kubernetes.io/docs/concepts/cluster-administration/system-metrics/)を確認し、次の兆候がないか調べてください:

- レスポンスレイテンシーの増加。
- ポッド、シークレット、ConfigMapなどのコア（v1）リソースに対する一般的な作成または削除操作のエラー率。

`kube-apiserver`の操作によるタイムアウト起因のエラーログは、次のように表示されることがあります:

```plaintext
Job failed (system failure): prepare environment: context deadline exceeded
Job failed (system failure): prepare environment: setting up build pod: context deadline exceeded
```

場合によっては、`kube-apiserver`のエラーレスポンスは、そのサブコンポーネント（Kubernetesクラスターの`etcdserver`など）の失敗に関する追加の詳細が含まれることがあります:

```plaintext
Job failed (system failure): prepare environment: etcdserver: request timed out
Job failed (system failure): prepare environment: etcdserver: leader changed
Job failed (system failure): prepare environment: Internal error occurred: resource quota evaluates timeout
```

これらの`kube-apiserver`サービスの失敗は、ビルドポッドの作成中だけでなく、完了後のクリーンアップ試行中にも発生する可能性があります:

```plaintext
Error cleaning up secrets: etcdserver: request timed out
Error cleaning up secrets: etcdserver: leader changed

Error cleaning up pod: etcdserver: request timed out, possibly due to previous leader failure
Error cleaning up pod: etcdserver: request timed out
Error cleaning up pod: context deadline exceeded
```

## `Dial tcp xxx.xx.x.x:xxx: i/o timeout` {#dial-tcp-xxxxxxxxxx-io-timeout}

これはKubernetesのエラーで、一般にRunnerマネージャーからKubernetes APIサーバーに到達できないことを示します。この問題を解決するには:

- ネットワークセキュリティポリシーを使用している場合、通常はポート443またはポート6443、あるいはその両方でKubernetes APIへのアクセスを許可します。
- Kubernetes APIが稼働していることを確認します。

## Kubernetes APIとの通信試行時に接続が拒否された {#connection-refused-when-attempting-to-communicate-with-the-kubernetes-api}

GitLab RunnerがKubernetes APIにリクエストを送信して失敗した場合、[`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver)が過負荷状態で、APIリクエストを受け付けられない、または処理できないことが原因である可能性が高いです。

## `Error cleaning up pod`と`Job failed (system failure): prepare environment: waiting for pod running` {#error-cleaning-up-pod-and-job-failed-system-failure-prepare-environment-waiting-for-pod-running}

Kubernetesがジョブポッドをタイムリーにスケジュールできない場合、次のエラーが発生します。GitLab Runnerはポッドの準備完了になるのを待機しますが失敗し、その後ポッドのクリーンアップを試みます。しかしそのクリーンアップも失敗することがあります。

```plaintext
Error: Error cleaning up pod: Delete "https://xx.xx.xx.x:443/api/v1/namespaces/gitlab-runner/runner-0001": dial tcp xx.xx.xx.x:443 connect: connection refused

Error: Job failed (system failure): prepare environment: waiting for pod running: Get "https://xx.xx.xx.x:443/api/v1/namespaces/gitlab-runner/runner-0001": dial tcp xx.xx.xx.x:443 connect: connection refused
```

トラブルシューティングを行うには、Kubernetesのプライマリノードと、[`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver)インスタンスを実行しているすべてのノードを確認してください。それらのノードに、クラスターでスケールして到達したいポッド数を管理するために必要なリソースがすべて備わっていることを確認してください。

ポッドが`Ready`ステータスに到達するまでGitLab Runnerが待機する時間を変更するには、[`poll_timeout`](_index.md#other-configtoml-settings)設定を使用します。

ポッドのスケジューリングを含め、準備段階が合計で実行できる期間を制限するには、[`prepare_timeout`](../../configuration/advanced-configuration.md#prepare-stage-timeout)設定を使用します。

ポッドがどのようにスケジュールされるのか、またはなぜ時間どおりにスケジュールされないのかを深く理解するには、[Kubernetesスケジューラーに関するドキュメントを参照してください](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/)。

## `request did not complete within requested timeout` {#request-did-not-complete-within-requested-timeout}

ビルドポッドの作成中に観測されるメッセージ`request did not complete within requested timeout`は、Kubernetesクラスターで構成されている[アドミッションコントロールWebhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)がタイムアウトしていることを示します。

アドミッションコントロールWebhookは、スコープ内のすべてのAPIリクエストをインターセプトするクラスターレベルの管理制御機能であり、所定の時間内に実行されない場合は失敗の原因となる可能性があります。

アドミッションコントロールWebhookは、インターセプトするAPIリクエストとネームスペースソースをきめ細かく制御できるフィルターをサポートしています。GitLab RunnerからのKubernetes APIコールがアドミッションコントロールWebhookを通過する必要がない場合は、[Webhookのセレクター/フィルター設定](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-objectselector)を変更してGitLab Runnerネームスペースを無視するか、[GitLab Runner Helmチャートの`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/57e026d7f43f63adc32cdd2b21e6d450abcf0686/values.yaml#L490-500)で`podAnnotations`または`podLabels`を設定して、GitLab Runnerポッドに除外用のラベル/アノテーションを適用できます。

たとえば、[DataDogアドミッションコントローラーWebhook](https://docs.datadoghq.com/containers/cluster_agent/admission_controller/?tab=operator)がGitLab RunnerマネージャーポッドによるAPIリクエストをインターセプトしないようにするには、次の内容を追加します:

```yaml
podLabels:
  admission.datadoghq.com/enabled: false
```

KubernetesクラスターのアドミッションコントロールWebhookを一覧表示するには、次のコマンドを実行します:

```shell
kubectl get validatingwebhookconfiguration -o yaml
kubectl get mutatingwebhookconfiguration -o yaml
```

アドミッションコントロールWebhookがタイムアウトした場合、次のような形式のログが確認されることがあります:

```plaintext
Job failed (system failure): prepare environment: Timeout: request did not complete within requested timeout
Job failed (system failure): prepare environment: setting up credentials: Timeout: request did not complete within requested timeout
```

アドミッションコントロールWebhookに起因する失敗は、代わりに次のように表示される場合もあります:

```plaintext
Job failed (system failure): prepare environment: setting up credentials: Internal error occurred: failed calling webhook "example.webhook.service"
```

## エラー`Could not resolve host: example.com` {#error-could-not-resolve-host-examplecom}

[ヘルパーイメージ](../../configuration/advanced-configuration.md#helper-image)の`alpine`フレーバーを使用している場合、Alpineの`musl`のDNSリゾルバーに起因する[DNSの問題](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4129)が発生する可能性があります。エラーは次のように表示される場合があります:

- `fatal: unable to access 'https://gitlab-ci-token:token@example.com/repo/proj.git/': Could not resolve host: example.com`

この問題を解決するには、`helper_image_flavor = "ubuntu"`オプションを使用します。

## `docker: Cannot connect to the Docker daemon at tcp://docker:2375. Is the docker daemon running?` {#docker-cannot-connect-to-the-docker-daemon-at-tcpdocker2375-is-the-docker-daemon-running}

このエラーは、[Docker-in-Docker](_index.md#using-dockerdind)を使用している場合に、DINDサービスが完全に起動する前にアクセスしようとすると発生することがあります。詳細については、[このイシュー](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27215)を参照してください。

## `curl: (35) OpenSSL SSL_connect: SSL_ERROR_SYSCALL in connection to github.com:443` {#curl-35-openssl-ssl_connect-ssl_error_syscall-in-connection-to-githubcom443}

このエラーは、[Docker-in-Docker](_index.md#using-dockerdind)を使用している場合に、DINDの最大転送ユニット（MTU）がKubernetesオーバーレイネットワークよりも大きい場合に発生することがあります。DINDはデフォルトで1500のMTUを使用しますが、これはデフォルトのオーバーレイネットワークを経由してルーティングするには大きすぎます。DINDのMTUはサービス定義内で変更できます:

```yaml
services:
  - name: docker:dind
    command: ["--mtu=1450"]
```

## `MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown is not supported by windows` {#mountvolumesetup-failed-for-volume-kube-api-access-xxxxx--chown-is-not-supported-by-windows}

CI/CDジョブを実行すると、次のようなエラーが発生することがあります:

```plaintext
MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown c:\var\lib\kubelet\pods\xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx\volumes\kubernetes.io~projected\kube-api-access-xxxxx\..2022_07_07_20_52_19.102630072\token: not supported by windows
```

この問題は、[ノードセレクターを使用](_index.md#specify-the-node-to-execute-builds)して、異なるオペレーティングシステムおよびアーキテクチャのノードでビルドを実行する場合に発生します。

この問題を修正するには、Runnerマネージャーポッドが常にLinuxノードでスケジュールされるように`nodeSelector`を設定します。たとえば、[`values.yaml`ファイル](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml)で次のように設定する必要があります:

```yaml
nodeSelector:
  kubernetes.io/os: linux
```

## ビルドポッドにRunner IAMロールではなくワーカーノードのIAMロールが割り当てられる {#build-pods-are-assigned-the-worker-nodes-iam-role-instead-of-runner-iam-role}

この問題は、ワーカーノードのIAMロールに、正しいロールを引き受ける権限がない場合に発生します。この問題を修正するには、ワーカーノードのIAMロールの信頼関係に`sts:AssumeRole`権限を追加します:

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

この問題は、`.gitlab-ci.yml`で`pull_policy`を指定したものの、Runnerの設定ファイルでポリシーが設定されていない場合に発生します。エラーは次のように表示される場合があります:

- `Preparation failed: invalid pull policy for image 'image-name:latest': pull_policy ([Always]) defined in GitLab pipeline config is not one of the allowed_pull_policies ([])`

この問題を修正するには、[Dockerプルポリシーを制限する](_index.md#restrict-docker-pull-policies)に従って、設定に`allowed_pull_policies`を追加します。

## バックグラウンドプロセスによりジョブがハングしてタイムアウトする {#background-processes-cause-jobs-to-hang-and-timeout}

ジョブの実行中に開始されたバックグラウンドプロセスにより、[ビルドジョブが終了できなくなる](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2880)ことができます。これを回避するには、次のようにします:

- プロセスをダブルフォークします。例: `command_to_run < /dev/null &> /dev/null &`。
- ジョブスクリプトを終了する前にプロセスを強制終了します。

## キャッシュ関連の`permission denied`エラー {#cache-related-permission-denied-errors}

ジョブで生成されるファイルとフォルダーには、特定のUNIX所有権と権限が付与されます。ファイルとフォルダーをアーカイブまたは抽出する際に、UNIXの詳細は保持されます。ただし、ファイルとフォルダーの情報が[ヘルパーイメージ](../../configuration/advanced-configuration.md#helper-image)の`USER`設定と一致しない場合があります。

`Creating cache ...`ステップで権限関連のエラーが発生した場合は、次のように対応できます:

- 解決策として、たとえばキャッシュ対象ファイルを作成するジョブスクリプトなどで、ソースデータが変更されていないかを調べます。
- 回避策として、[(`before_`/`after_`)`script:`ディレクティブ](https://docs.gitlab.com/ci/yaml/#default)に、一致する[chown](https://linux.die.net/man/1/chown)コマンドと[chmod](https://linux.die.net/man/1/chmod)コマンドを追加します。

## initシステムを使用するビルドコンテナ内の一見冗長なShellプロセス {#apparently-redundant-shell-process-in-build-container-with-init-system}

次のいずれかの場合、プロセスツリーにShellプロセスが含まれることがあります:

- `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY`が`false`で、かつ`FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR`が`true`である。
- ビルドイメージの`ENTRYPOINT`がinitシステム（`tini-init`や`dumb-init`など）である。

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

`PPID`が1、`PID`が6または7となっているこのShellプロセス（`sh`、`bash`、または`busybox`である可能性がある）は、（上記の`PID` 1の）initシステムによって実行されるShell検出スクリプトが起動したShellです。このプロセスは冗長なものではなく、ビルドコンテナをinitシステムとともに実行する場合の一般的な動作です。

## 登録成功後もRunnerポッドがジョブリクエストに対する結果を実行できずタイムアウトする {#runner-pod-fails-to-run-job-results-and-times-out-despite-successful-registration}

RunnerポッドがGitLabに登録後、ジョブの実行を試みますが実行されず、最終的にジョブがタイムアウトします。次のエラーが報告されます:

```plaintext
There has been a timeout failure or the job got stuck. Check your timeout limits or try again.

This job does not have a trace.
```

この場合、Runnerは次のエラーを受け取ることがあります。

```plaintext
HTTP 204 No content response code when connecting to the `jobs/request` API.
```

この問題のトラブルシューティングを行うには、APIにPOSTリクエストを手動で送信して、TCP接続がハングしているかどうかを検証します。TCP接続がハングしている場合、RunnerはCIジョブペイロードをリクエストできない可能性があります。

## `gcs-fuse-csi-driver`を使用している場合に`failed to reserve container name`コンテナ用のコンテナ名を予約できない {#failed-to-reserve-container-name-for-init-permissions-container-when-gcs-fuse-csi-driver-is-used}

`gcs-fuse-csi-driver` `csi`ドライバーは、[initコンテナ向けのボリュームマウントをサポートしていません](https://github.com/GoogleCloudPlatform/gcs-fuse-csi-driver/issues/38)。これにより、このドライバーを使用している場合、initコンテナの起動が失敗することがあります。このバグを解決するには、[Kubernetes 1.28で導入](https://kubernetes.io/blog/2023/08/25/native-sidecar-containers/)された機能がドライバーのプロジェクトでサポートされる必要があります。

## エラー: `only read-only root filesystem container is allowed` {#error-only-read-only-root-filesystem-container-is-allowed}

コンテナに対して読み取り専用のルートファイルシステムでの実行を強制するアドミッションポリシーがあるクラスターでは、次の場合にこのエラーが表示されることがあります:

- GitLab Runnerをインストールする。
- GitLab Runnerがビルドポッドをスケジュールしようとする。

これらのアドミッションポリシーは通常、[Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/)や[Kyverno](https://kyverno.io/)などのアドミッションコントローラーによって強制されます。たとえば、読み取り専用のルートファイルシステムでの実行をコンテナに強制するポリシーとして、Gatekeeperの[`readOnlyRootFilesystem`](https://open-policy-agent.github.io/gatekeeper-library/website/validation/read-only-root-filesystem/)ポリシーがあります。

この問題を解決するには:

- クラスターにデプロイされるすべてのポッドは、アドミッションコントローラーがポッドをブロックしないように、コンテナに対して`securityContext.readOnlyRootFilesystem`を`true`に設定し、アドミッションポリシーに準拠する必要があります。
- ルートファイルシステムが読み取り専用でマウントされていても、コンテナが正常に動作し、ファイルシステムに書き込める必要があります。

### GitLab Runnerの場合 {#for-gitlab-runner}

GitLab Runnerが[GitLab Runner Helmチャート](../../install/kubernetes.md)でデプロイされている場合、GitLabチャートの設定を更新し、次を設定する必要があります:

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

ビルドポッドを読み取り専用のルートファイルシステム上で実行するには、`config.toml`で各コンテナのセキュリティコンテキストを設定します。ビルドポッドに渡されるGitLabチャート変数`runners.config`を設定できます:

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

ビルドポッドおよびそのコンテナを読み取り専用のファイルシステム上で正常に実行するには、ビルドポッドが書き込める場所に書き込み可能なファイルシステムを用意する必要があります。最低限、ビルドディレクトリとホームディレクトリがこれに該当します。必要に応じて、ビルドプロセスが他の場所にも書き込み可能であることを確認してください。

一般に、正常に実行するために必要な設定やその他のデータをプログラムが保存できるように、ホームディレクトリは書き込み可能である必要があります。たとえば、`git`バイナリは、ホームディレクトリへの書き込みを必要とするプログラムの1つです。

異なるコンテナイメージで、ホームディレクトリのパスに関係なくホームディレクトリを書き込み可能にするには:

1. （使用するビルドイメージに関係なく）安定したパスにボリュームをマウントします。
1. すべてのビルドに対して環境変数`$HOME`をグローバルに設定して、ホームディレクトリを変更します。

GitLabチャート変数`runners.config`の値を更新することで、`config.toml`でビルドポッドとそのコンテナを設定できます。

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

> [!note]
> `emptyDir`の代わりに、他の[サポートされているボリュームタイプ](_index.md#configure-volume-types)を使用できます。明示的に処理されビルドアーティファクトとして保存されないすべてのファイルは通常、一時的であるため、`emptyDir`はほとんどのケースで機能します。

## AWS EKS: ポッドのクリーンアップエラー: ポッド「Runner - \*\*」が見つからない、またはステータスが「Failed」 {#aws-eks-error-cleaning-up-pod-pods-runner--not-found-or-status-is-failed}

Amazon EKSのゾーンリバランシング機能は、オートスケールグループ内のアベイラビリティーゾーンのバランスを取ります。この機能により、あるアベイラビリティーゾーンのノードが停止され、別のアベイラビリティーゾーンでノードが作成されることがあります。

Runnerジョブは、停止して別のノードに移動させることはできません。このエラーを解決するには、Runnerジョブに対してこの機能を無効にします。

## Windowsコンテナではservicesがサポートされない {#services-not-supported-with-windows-containers}

Windowsノードで[services](https://docs.gitlab.com/ci/services/)を使用しようとすると、次のエラーで失敗する可能性があります:

- `ERROR: Job failed (system failure): prepare environment: admission webhook "windows.common-webhooks.networking.gke.io" denied the request: spec.hostAliases: Invalid value: []v1.HostAlias{v1.HostAlias{IP:"127.0.0.1", Hostnames:[]string{"<your windows image>"}}}: Windows does not support this field.`

Kubernetesランタイムによっては、このエラーが報告される場合と、黙って無視される場合があります。たとえば、GKEではエラーが報告されます。

Kubernetes executorにおけるservicesは`hostAlias`を使用して実装されていますが、Windowsコンテナではサポートされません。
