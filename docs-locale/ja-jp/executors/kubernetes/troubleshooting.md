---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Kubernetes executorのトラブルシューティング
---

Kubernetes executorの使用時に発生する一般的なエラーを以下に示します。

## `Job failed (system failure): timed out waiting for pod to start` {#job-failed-system-failure-timed-out-waiting-for-pod-to-start}

クラスターが`poll_timeout`で定義されたタイムアウトになる前にビルドポッドをスケジュールできない場合、ビルドポッドはエラーを返します。[Kubernetes Scheduler](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-lifetime)は、それを削除できる必要があります。

この問題を解決するには、`config.toml`ファイルの`poll_timeout`の値を大きくしてください。

## `context deadline exceeded` {#context-deadline-exceeded}

ジョブジョブログの`context deadline exceeded`エラーは通常、Kubernetes APIクライアントが特定のクラスターAPIリクエストのタイムアウトになったことを示します。

次の兆候がないか、[`kube-apiserver`クラスターコンポーネントのmetrics](https://kubernetes.io/docs/concepts/cluster-administration/system-metrics/)を確認します:

- 応答latencyの増加。
- ポッド、シークレット、ConfigMap、およびその他のコア（v1）リソースに対する一般的な作成または削除操作のエラー率。

`kube-apiserver`操作からのタイムアウト駆動型エラーのログは、次のように表示される場合があります:

```plaintext
Job failed (system failure): prepare environment: context deadline exceeded
Job failed (system failure): prepare environment: setting up build pod: context deadline exceeded
```

場合によっては、`kube-apiserver`エラー応答で、Kubernetesクラスターの`etcdserver`などのサブコンポーネントの失敗に関する追加の詳細が提供されることがあります:

```plaintext
Job failed (system failure): prepare environment: etcdserver: request timed out
Job failed (system failure): prepare environment: etcdserver: leader changed
Job failed (system failure): prepare environment: Internal error occurred: resource quota evaluates timeout
```

これらの`kube-apiserver`サービスの失敗は、ビルドポッドの作成中、および完了後のクリーンアップの試行中に発生する可能性があります:

```plaintext
Error cleaning up secrets: etcdserver: request timed out
Error cleaning up secrets: etcdserver: leader changed

Error cleaning up pod: etcdserver: request timed out, possibly due to previous leader failure
Error cleaning up pod: etcdserver: request timed out
Error cleaning up pod: context deadline exceeded
```

## `Dial tcp xxx.xx.x.x:xxx: i/o timeout` {#dial-tcp-xxxxxxxxxx-io-timeout}

これはKubernetesのエラーであり、通常、RunnerマネージャーがKubernetes APIサーバーにアクセスできないことを示します。この問題を解決するには、以下を実行します:

- ネットワークセキュリティポリシーを使用する場合は、通常はポート443またはポート6443のいずれかまたは両方で、Kubernetes APIへのアクセスを許可します。
- Kubernetes APIが実行されていることを確認します。

## Kubernetes APIとの通信を試みるときに接続が拒否されました {#connection-refused-when-attempting-to-communicate-with-the-kubernetes-api}

GitLab RunnerがKubernetes APIにリクエストを送信して失敗した場合、[`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver)がオーバーロードになり、APIリクエストを受信または処理できないことが原因である可能性があります。

## `Error cleaning up pod`と`Job failed (system failure): prepare environment: waiting for pod running` {#error-cleaning-up-pod-and-job-failed-system-failure-prepare-environment-waiting-for-pod-running}

Kubernetesがジョブポッドをタイムリーにスケジュールできない場合、次のエラーが発生します。GitLab Runnerはポッドの準備ができるのを待ちますが、失敗し、次にポッドのクリーンアップを試みますが、これも失敗する可能性があります。

```plaintext
Error: Error cleaning up pod: Delete "https://xx.xx.xx.x:443/api/v1/namespaces/gitlab-runner/runner-0001": dial tcp xx.xx.xx.x:443 connect: connection refused

Error: Job failed (system failure): prepare environment: waiting for pod running: Get "https://xx.xx.xx.x:443/api/v1/namespaces/gitlab-runner/runner-0001": dial tcp xx.xx.xx.x:443 connect: connection refused
```

トラブルシューティングを行うには、Kubernetesプライマリノードと、[`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver)インスタンスを実行するすべてのノードを確認してください。クラスターでスケールアップすることを希望するターゲットポッド数を管理するために必要なすべてのリソースがあることを確認してください。

GitLab Runnerがポッドが`Ready`ステータスになるまで待機する時間を変更するには、[`poll_timeout`](_index.md#other-configtoml-settings)設定を使用します。

ポッドのスケジュール方法、またはポッドが時間どおりにスケジュールされない理由をより深く理解するには、[Kubernetes Schedulerについてお読みください](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/)。

## `request did not complete within requested timeout` {#request-did-not-complete-within-requested-timeout}

ビルドポッドの作成中に確認されたメッセージ`request did not complete within requested timeout`は、Kubernetesクラスターで構成された[アドミッションコントロールWebhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/)がタイムアウトになっていることを示します。

アドミッションコントロールWebhookは、スコープが設定されているすべてのAPIリクエストに対するクラスターレベルの管理コントロールインターセプトであり、時間内に実行されない場合、失敗を引き起こす可能性があります。

アドミッションコントロールWebhookは、インターセプトするAPIリクエストおよびネームスペースソースを細かく制御できるフィルターをサポートします。GitLab RunnerからのKubernetes APIコールがアドミッションコントロールWebhookを通過する必要がない場合は、[Webhookのセレクター/フィルター構成](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-objectselector)を変更してGitLab Runnerネームスペースを無視するか、[GitLab Runner Helmチャート`values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/57e026d7f43f63adc32cdd2b21e6d450abcf0686/values.yaml#L490-500)で`podAnnotations`または`podLabels`を構成して、GitLab Runnerポッドに除外ラベル/注釈を適用できます。

たとえば、[DataDog Admission Controller Webhook](https://docs.datadoghq.com/containers/cluster_agent/admission_controller/?tab=operator)がGitLab Runnerマネージャーポッドによって行われたAPIリクエストをインターセプトしないようにするには、次を追加します:

```yaml
podLabels:
  admission.datadoghq.com/enabled: false
```

KubernetesクラスターのアドミッションコントロールWebhookを一覧表示するには、次を実行します:

```shell
kubectl get validatingwebhookconfiguration -o yaml
kubectl get mutatingwebhookconfiguration -o yaml
```

アドミッションコントロールWebhookがタイムアウトになると、次の形式のログが確認できます:

```plaintext
Job failed (system failure): prepare environment: Timeout: request did not complete within requested timeout
Job failed (system failure): prepare environment: setting up credentials: Timeout: request did not complete within requested timeout
```

アドミッションコントロールWebhookからの失敗は、代わりに次のように表示される場合があります:

```plaintext
Job failed (system failure): prepare environment: setting up credentials: Internal error occurred: failed calling webhook "example.webhook.service"
```

<!-- markdownlint-disable line-length -->

## `fatal: unable to access 'https://gitlab-ci-token:token@example.com/repo/proj.git/': Could not resolve host: example.com` {#fatal-unable-to-access-httpsgitlab-ci-tokentokenexamplecomrepoprojgit-could-not-resolve-host-examplecom}

[ヘルパーイメージ](../../configuration/advanced-configuration.md#helper-image)の`alpine`alpineフレーバーを使用している場合、Alpineの`musl`のDNSリゾルバーに関連する[DNSの問題](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4129)が発生する可能性があります。

`helper_image_flavor = "ubuntu"`オプションを使用すると、この問題を解決できるはずです。

<!-- markdownlint-enable line-length -->

## `docker: Cannot connect to the Docker daemon at tcp://docker:2375. Is the docker daemon running?` {#docker-cannot-connect-to-the-docker-daemon-at-tcpdocker2375-is-the-docker-daemon-running}

このエラーは、[Docker-in-Docker](_index.md#using-dockerdind)の使用時に、DINDサービスが完全に起動する前にアクセスしようとすると発生する可能性があります。詳細については、[このイシュー](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27215)を参照してください。

## `curl: (35) OpenSSL SSL_connect: SSL_ERROR_SYSCALL in connection to github.com:443` {#curl-35-openssl-ssl_connect-ssl_error_syscall-in-connection-to-githubcom443}

このエラーは、[Docker-in-Docker](_index.md#using-dockerdind)を使用している場合に、DINDの最大伝送ユニット（MTU）がKubernetesオーバーレイネットワークよりも大きい場合に発生する可能性があります。DINDはデフォルトのMTU 1500を使用しますが、これはデフォルトのオーバーレイネットワーク全体でルーティングするには大きすぎます。DIND MTUは、サービス定義内で変更できます:

```yaml
services:
  - name: docker:dind
    command: ["--mtu=1450"]
```

## `MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown is not supported by windows` {#mountvolumesetup-failed-for-volume-kube-api-access-xxxxx--chown-is-not-supported-by-windows}

CI/CDジョブを実行すると、次のようなエラーが発生する場合があります:

```plaintext
MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown c:\var\lib\kubelet\pods\xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx\volumes\kubernetes.io~projected\kube-api-access-xxxxx\..2022_07_07_20_52_19.102630072\token: not supported by windows
```

この問題は、[ノードセレクターを使用](_index.md#specify-the-node-to-execute-builds)して、異なるオペレーティングシステムとアーキテクチャを持つノードでビルドを実行すると発生します。

この問題を修正するには、`nodeSelector`を構成して、Runnerマネージャーポッドが常にLinuxノードでスケジュールされるようにします。たとえば、[`values.yaml`ファイル](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml)には、次のものが含まれている必要があります:

```yaml
nodeSelector:
  kubernetes.io/os: linux
```

## ビルドポッドに、Runner IAMロールの代わりにワーカーノードのIAMロールが割り当てられています {#build-pods-are-assigned-the-worker-nodes-iam-role-instead-of-runner-iam-role}

この問題は、ワーカーノードIAMロールに、正しいロールを引き受ける権限がない場合に発生します。これを修正するには、`sts:AssumeRole`権限をワーカーノードのIAMロールの信頼関係に追加します:

```json
{
    "Effect": "Allow",
    "Principal": {
        "AWS": "arn:aws:iam::<AWS_ACCOUNT_NUMBER>:role/<IAM_ROLE_NAME>"
    },
    "Action": "sts:AssumeRole"
}
```

<!-- markdownlint-disable line-length -->

## `Preparation failed: invalid pull policy for image 'image-name:latest': pull_policy ([Always]) defined in GitLab pipeline config is not one of the allowed_pull_policies ([])` {#preparation-failed-invalid-pull-policy-for-image-image-namelatest-pull_policy-always-defined-in-gitlab-pipeline-config-is-not-one-of-the-allowed_pull_policies-}

この問題は、`pull_policy`を`.gitlab-ci.yml`で指定したが、Runnerのconfig fileにポリシーが構成されていない場合に発生します。これを修正するには、[Dockerプルポリシーの制限](_index.md#restrict-docker-pull-policies)に従って、`allowed_pull_policies`を構成に追加します。

<!-- markdownlint-enable line-length -->

## バックグラウンドプロセスにより、ジョブがハングアップし、タイムアウトします {#background-processes-cause-jobs-to-hang-and-timeout}

ジョブの実行中に開始されたバックグラウンドプロセスは、[ビルドジョブが終了するのを防ぐ](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2880)可能性があります。これを回避するには、次の手順を実行します:

- プロセスをダブルフォークします。たとえば、`command_to_run < /dev/null &> /dev/null &`などです。
- ジョブスクリプトを終了する前にプロセスを強制終了します。

## キャッシュに関連する`permission denied`エラー {#cache-related-permission-denied-errors}

ジョブで生成されるファイルとフォルダーには、特定のUNIX所有権と権限があります。ファイルとフォルダーがアーカイブまたは抽出されると、UNIXの詳細が保持されます。ただし、ファイルとフォルダーは、[ヘルパーイメージ](../../configuration/advanced-configuration.md#helper-image)の`USER`構成と一致しない場合があります。

`Creating cache ...`ステップで権限関連のエラーが発生した場合は、次の手順を実行できます:

- 解決策として、ソースデータが変更されているかどうかを調査します（たとえば、キャッシュされたファイルを作成するジョブスクリプトなど）。
- 回避策として、一致する[chown](https://linux.die.net/man/1/chown)コマンドと[chmod](https://linux.die.net/man/1/chmod)コマンドを[（`before_`/`after_`）`script:`ディレクティブ](https://docs.gitlab.com/ci/yaml/#default)に追加します。

## 初期システムを備えたビルドコンテナ内の明らかに冗長なシェルプロセス {#apparently-redundant-shell-process-in-build-container-with-init-system}

プロセスツリーには、次のいずれかの場合にシェルプロセスが含まれる場合があります:

- `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY`が`false`で、`FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR`が`true`です。
- ビルドイメージの`ENTRYPOINT`は、（`tini-init`や`dumb-init`のような）初期システムです。

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

このシェルプロセスは、`sh`、`bash`、または`busybox`である可能性があり、`PPID`が1で、`PID`が6または7である場合、初期システム（上記の`PID`1）によって実行されるシェル検出スクリプトによって開始されるシェルです。このプロセスは冗長ではなく、ビルドコンテナが初期システムで実行される場合の一般的な操作です。

## Runnerポッドはジョブの結果を実行できず、登録が成功したにもかかわらずタイムアウトします {#runner-pod-fails-to-run-job-results-and-times-out-despite-successful-registration}

RunnerポッドがGitLabに登録された後、ジョブを実行しようとしますが、実行されず、最終的にジョブはタイムアウトします。次のエラーがレポートされます:

```plaintext
There has been a timeout failure or the job got stuck. Check your timeout limits or try again.

This job does not have a trace.
```

この場合、Runnerは次のエラーを受け取る可能性があります。

```plaintext
HTTP 204 No content response code when connecting to the `jobs/request` API.
```

この問題をトラブルシューティングするには、手動でPOSTリクエストをAPIに送信して、TCP接続がハングしているかどうかを検証します。TCP接続がハングしている場合、RunnerはCIジョブペイロードをリクエストできない可能性があります。

## `failed to reserve container name`（`gcs-fuse-csi-driver`を使用している場合）のinit-permissionsコンテナ {#failed-to-reserve-container-name-for-init-permissions-container-when-gcs-fuse-csi-driver-is-used}

`gcs-fuse-csi-driver` `csi`ドライバーは、[初期コンテナのボリュームのマウントをサポートしていません](https://github.com/GoogleCloudPlatform/gcs-fuse-csi-driver/issues/38)。これにより、このドライバーを使用するときに初期コンテナの起動が失敗する可能性があります。[Kubernetes 1.28で導入された](https://kubernetes.io/blog/2023/08/25/native-sidecar-containers/)機能は、このbugを解決するためにドライバーのプロジェクトでサポートされている必要があります。

## エラー: `only read-only root filesystem container is allowed` {#error-only-read-only-root-filesystem-container-is-allowed}

コンテナを読み取り専用でマウントされたルートfilesystemで実行するように強制するアドミッションポリシーを持つクラスターでは、このエラーは次の場合に表示されることがあります:

- GitLab Runnerをインストールするには、次の手順に従ってください。
- GitLab Runnerがビルドポッドをスケジュールしようとしています。

これらのアドミッションポリシーは通常、[Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/)や[Kyverno](https://kyverno.io/)などのアドミッションコントローラーによって適用されます。たとえば、コンテナを読み取り専用ルートfilesystemで実行するように強制するポリシーは、[`readOnlyRootFilesystem`](https://open-policy-agent.github.io/gatekeeper-library/website/validation/read-only-root-filesystem/) Gatekeeperポリシーです。

この問題を解決するには、以下を実行します:

- クラスターにデプロイされるすべてのポッドは、コンテナの`securityContext.readOnlyRootFilesystem`を`true`に設定することにより、アドミッションポリシーに準拠する必要があります。これにより、アドミッションコントローラーがポッドをブロックしません。
- ルートfilesystemが読み取り専用でマウントされている場合でも、コンテナは正常に実行され、filesystemに書き込むことができる必要があります。

### GitLab Runnerの場合 {#for-gitlab-runner}

GitLab Runnerが[GitLab Runner Helmチャート](../../install/kubernetes.md)でデプロイされている場合は、GitLabチャート構成を更新して、次のようにする必要があります:

- 適切な`securityContext`値:

  ```yaml
  <...>
  securityContext:
    readOnlyRootFilesystem: true
  <...>
  ```

- ポッドが書き込むことができる、書き込み可能なfilesystemがマウントされています:

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

ビルドポッドを読み取り専用ルートfilesystemで実行するには、`config.toml`で異なるコンテナのセキュリティコンテキストを構成します。GitLabチャートの変数`runners.config`を設定できます。これはビルドポッドに渡されます:

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

ビルドポッドとそのコンテナを読み取り専用filesystemで正常に実行するには、ビルドポッドが書き込むことができる場所に書き込み可能なfilesystemが必要です。少なくとも、これらの場所はビルドおよびホームディレクトリです。必要に応じて、ビルドプロセスに他の場所への書き込みアクセス権があることを確認してください。

一般に、ホームディレクトリは、プログラムが正常な実行に必要な構成やその他のデータを格納できるように、書き込み可能である必要があります。`git` binaryは、ホームディレクトリに書き込むことができると予想されるプログラムの一例です。

異なるコンテナイメージのパスに関係なく、ホームディレクトリを書き込み可能にするには:

1. （どのビルドイメージを使用する場合でも）安定したパスにボリュームをマウントします。
1. すべてのビルドに対してグローバルに環境変数`$HOME`を設定して、ホームディレクトリを変更します。

GitLabチャートの変数`runners.config`の値を更新することにより、`config.toml`でビルドポッドとそのコンテナを構成できます。

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

`emptyDir`の代わりに、他の[サポートされているボリュームタイプ](_index.md#configure-volume-types)を使用できます。明示的に処理され、ビルドアーティファクトとして保存されないすべてのファイルは通常一時的なため、ほとんどの場合、`emptyDir`が機能します。

{{< /alert >}}

## AWS EKS: ポッドのクリーンアップ中にエラー: ポッド「Runner-\*\*」が見つからないか、ステータスが「失敗」です {#aws-eks-error-cleaning-up-pod-pods-runner--not-found-or-status-is-failed}

Amazon EKSゾーン再分散機能は、autoscalingグループ内の可用性ゾーンのバランスを取ります。この機能は、1つの可用性ゾーンのノードを停止し、別の可用性ゾーンに作成する可能性があります。

Runnerジョブを停止して別のノードに移動することはできません。このエラーを解決するには、Runnerジョブに対してこの機能を無効にします。
