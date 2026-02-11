---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: AWS FargateでGitLab CIをオートスケールする
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

{{< alert type="warning" >}}

Fargateドライバーは、コミュニティでサポートされています。GitLabサポートは問題のデバッグを支援しますが、保証は提供しません。

{{< /alert >}}

GitLabの[custom executor](../../executors/custom.md)ドライバー（[AWS Fargate](https://gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate)用）は、Amazon Elastic Container Service (ECS) 上のコンテナを自動的に起動して、各GitLab CIジョブを実行します。

このドキュメントのタスクを完了すると、executorはGitLabから開始されたジョブを実行できます。GitLabでコミットが行われるたびに、GitLabインスタンスは新しいジョブが利用可能になったことをRunnerに通知します。次に、Runnerは、AWS ECSで設定したタスク定義に基づいて、ターゲットECSクラスターで新しいタスクを開始します。任意のDockerイメージを使用するようにAWS ECSタスク定義を設定できます。このアプローチを使用すると、AWS Fargateで実行できるビルドのタイプを完全に柔軟に設定できます。

![GitLab Runner Fargateドライバーのアーキテクチャ](../img/runner_fargate_driver_ssh.png)

このドキュメントでは、実装の最初の理解を深めるための例を示します。本番環境での使用を目的としたものではありません。AWSでは追加のセキュリティが必要です。

たとえば、2つのAWSセキュリティグループが必要になる場合があります:

- GitLab RunnerをホストするEC2インスタンスで使用され、制限された外部IP範囲（管理アクセス用）からのSSH接続のみを受け入れるもの。
- Fargateタスクに適用され、EC2インスタンスからのSSHトラフィックのみを許可するもの。

非公開のコンテナレジストリの場合、ECSタスクには、[IAM権限（AWS ECRのみ）](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html)または非ECRプライベートレジストリの[タスクのプライベートレジストリ認証](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/private-auth.html)が必要です。

CloudFormationまたはTerraformを使用して、AWSインフラストラクチャのプロビジョニングとセットアップを自動化できます。

{{< alert type="warning" >}}

CI/CDジョブは、`image:`ファイルの`.gitlab-ci.yml`キーワードの値ではなく、ECSタスクで定義されたイメージを使用します。ECSでは、ECSタスクに使用されるイメージをオーバーライドすることはできません。

この制限を回避するには、次の操作を実行できます:

- Runnerが使用するすべてのプロジェクトのすべてのビルド依存関係を含むイメージをECSタスク定義に作成して使用します。
- 異なるイメージを持つ複数のECSタスク定義を作成し、`FARGATE_TASK_DEFINITION` CI/CD変数でARNを指定します。
- 公式の[AWS EKSブループリント](https://aws-ia.github.io/terraform-aws-eks-blueprints/)に従って、EKSクラスターの作成を検討してください。

詳細については、[GitLab EKS Fargate Runnerを1時間で開始し、コードをゼロにする](https://about.gitlab.com/blog/eks-fargate-runner/)を参照してください。

{{< /alert >}}

{{< alert type="warning" >}}

Fargateはコンテナホストを抽象化するため、コンテナホストのプロパティの設定可能性が制限されます。これは、ディスクまたはネットワークへの高いIOを必要とするRunnerワークロードに影響します。これらのプロパティは、Fargateでは設定可能性が限られているか、設定できないためです。FargateでGitLab Runnerを使用する前に、CPU、メモリ、ディスクI/O、またはネットワークI/Oに関するコンピューティング特性の高いRunnerワークロードがFargateに適していることを確認してください。

{{< /alert >}}

## 前提条件 {#prerequisites}

始める前に、以下が必要です:

- EC2、ECS、ECRリソースを作成および構成する権限を持つAWS IAMユーザー。
- AWS VPCとサブネット。
- 1つ以上のAWSセキュリティグループ。

## ステップ1: AWS Fargateタスクのコンテナイメージを準備する {#step-1-prepare-a-container-image-for-the-aws-fargate-task}

コンテナイメージを準備します。このイメージをレジストリにアップロードできます。このレジストリは、GitLabジョブの実行時にコンテナを作成するために使用できます。

1. イメージにCIジョブのビルドに必要なツールがあることを確認します。たとえば、Javaプロジェクトには、`Java JDK`やMavenやGradleなどのビルドツールが必要です。Node.jsプロジェクトには、`node`と`npm`が必要です。
1. イメージにアーティファクトとキャッシュを処理するGitLab Runnerがあることを確認します。詳細については、カスタムexecutorドキュメントの[実行](../../executors/custom.md#run)ステージセクションを参照してください。
1. コンテナイメージが公開キー認証を介してSSH接続を受け入れることができることを確認します。Runnerは、この接続を使用して、`.gitlab-ci.yml`ファイルで定義されたビルドコマンドをAWS Fargate上のコンテナに送信します。SSHキーは、Fargateドライバーによって自動的に管理されます。コンテナは、`SSH_PUBLIC_KEY`環境変数からのキーを受け入れることができる必要があります。

GitLab RunnerとSSH構成を含む[Debianの例](https://gitlab.com/tmaczukin-test-projects/fargate-driver-debian)をご覧ください。[Node.jsの例](https://gitlab.com/aws-fargate-driver-demo/docker-nodejs-gitlab-ci-fargate)をご覧ください。

## ステップ2: コンテナイメージをレジストリにプッシュする {#step-2-push-the-container-image-to-a-registry}

イメージを作成したら、ECSタスク定義で使用するために、イメージをコンテナレジストリに公開します。

- リポジトリを作成してイメージをECRにプッシュするには、[Amazon ECRリポジトリ](https://docs.aws.amazon.com/AmazonECR/latest/userguide/Repositories.html)のドキュメントに従ってください。
- AWS CLIを使用してイメージをECRにプッシュするには、[AWS CLIを使用したAmazon ECRの概要](https://docs.aws.amazon.com/AmazonECR/latest/userguide/getting-started-cli.html)ドキュメントに従ってください。
- [GitLabコンテナレジストリ](https://docs.gitlab.com/user/packages/container_registry/)を使用するには、[Debian](https://gitlab.com/tmaczukin-test-projects/fargate-driver-debian)または[NodeJS](https://gitlab.com/aws-fargate-driver-demo/docker-nodejs-gitlab-ci-fargate)の例を使用できます。Debianイメージは`registry.gitlab.com/tmaczukin-test-projects/fargate-driver-debian:latest`に公開されています。NodeJSのサンプルイメージは`registry.gitlab.com/aws-fargate-driver-demo/docker-nodejs-gitlab-ci-fargate:latest`に公開されています。

## ステップ3: GitLab RunnerのEC2インスタンスを作成する {#step-3-create-an-ec2-instance-for-gitlab-runner}

次に、AWS EC2インスタンスを作成します。次の手順では、GitLab Runnerをインストールします。

1. [https://console.aws.amazon.com/ec2/v2/home#LaunchInstanceWizard](https://console.aws.amazon.com/ec2/v2/home#LaunchInstanceWizard)にアクセスします。
1. インスタンスの場合は、Ubuntu Server 18.04 LTS AMIを選択します。名前は、選択したAWSリージョンによって異なる場合があります。
1. インスタンスタイプの場合は、t2.microを選択します。**次へ: インスタンスの詳細を設定**。
1. **Number of instances**はデフォルトのままにします。
1. **ネットワーク**はネットワーク、VPCを選択します。
1. **Auto-assign Public IP**を**有効**に設定します。
1. **IAM role**で、**Create new IAM role**を選択します。このロールはテストのみを目的としており、安全ではありません。
   1. **Create role**を選択します。
   1. **AWS service**を選択し、**Common use cases**で、**EC2**を選択します。次に、**次へ：を選択します: 権限**。
   1. **AmazonECS_FullAccess**ポリシーのチェックボックスをオンにします。**次へ: タグ**。
   1. **次へ: レビュー**。
   1. IAMロールの名前（`fargate-test-instance`など）を入力し、**ロールを作成する**を選択します。
1. インスタンスを作成しているブラウザータブに戻ります。
1. **Create new IAM role**の左側にある更新ボタンを選択します。`fargate-test-instance`ロールを選択します。**次へ: ストレージを追加**。
1. **次へ: タグの追加**。
1. **次へ: セキュリティグループを設定**。
1. **Create a new security group**を選択し、`fargate-test`という名前を付けて、SSHのルールが定義されていることを確認します（`Type: SSH, Protocol: TCP, Port Range: 22`）。インバウンドルールとアウトバウンドルールのIP範囲を指定する必要があります。
1. **Review and Launch**を選択します。
1. **Launch**を選択します。
1. オプション。オプション。**Create a new key pair**を選択し、`fargate-runner-manager`という名前を付けて、**Download Key Pair**を選択します。SSHのプライベートキーがコンピューターにダウンロードされます（ブラウザーで構成されたディレクトリを確認してください）。
1. **Launch Instances**を選択します。
1. **View Instances**を選択します。
1. インスタンスが起動するまで待ちます。`IPv4 Public IP`アドレスを書き留めます。

## ステップ4: EC2インスタンスにGitLab Runnerをインストールして構成する {#step-4-install-and-configure-gitlab-runner-on-the-ec2-instance}

次に、UbuntuインスタンスにGitLab Runnerをインストールします。

1. GitLabプロジェクトの**設定 > CI/CD**に移動し、Runnerセクションを展開します。**Set up a specific Runner manually**で、登録トークンを書き留めます。
1. キーファイルに適切な権限があることを確認するために、`chmod 400 path/to/downloaded/key/file`を実行します。
1. 次のコマンドを使用して、作成したEC2インスタンスにSSHで接続します:

   ```shell
   ssh ubuntu@[ip_address] -i path/to/downloaded/key/file
   ```

1. 正常に接続されたら、次のコマンドを実行します:

   ```shell
   sudo mkdir -p /opt/gitlab-runner/{metadata,builds,cache}
   curl -s "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" | sudo bash
   sudo apt install gitlab-runner
   ```

1. 手順1でメモしたGitLab URLと登録トークンを使用して、このコマンドを実行します。

   ```shell
   sudo gitlab-runner register --url "https://gitlab.com/" --registration-token TOKEN_HERE --name fargate-test-runner --run-untagged --executor custom -n
   ```

1. `sudo vim /etc/gitlab-runner/config.toml`を実行し、次のコンテンツを追加します:

   ```toml
   concurrent = 1
   check_interval = 0

   [session_server]
     session_timeout = 1800

   [[runners]]
     name = "fargate-test"
     url = "https://gitlab.com/"
     token = "__REDACTED__"
     executor = "custom"
     builds_dir = "/opt/gitlab-runner/builds"
     cache_dir = "/opt/gitlab-runner/cache"
     [runners.custom]
       volumes = ["/cache", "/path/to-ca-cert-dir/ca.crt:/etc/gitlab-runner/certs/ca.crt:ro"]
       config_exec = "/opt/gitlab-runner/fargate"
       config_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "config"]
       prepare_exec = "/opt/gitlab-runner/fargate"
       prepare_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "prepare"]
       run_exec = "/opt/gitlab-runner/fargate"
       run_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "run"]
       cleanup_exec = "/opt/gitlab-runner/fargate"
       cleanup_args = ["--config", "/etc/gitlab-runner/fargate.toml", "custom", "cleanup"]
   ```

1. プライベートCAを持つGitLab Self-Managedインスタンスがある場合は、次の行を追加します:

   ```toml
          volumes = ["/cache", "/path/to-ca-cert-dir/ca.crt:/etc/gitlab-runner/certs/ca.crt:ro"]
   ```

   [証明書を信頼する方法の詳細](../tls-self-signed.md#trusting-the-certificate-for-the-other-cicd-stages)。

   以下に示す`config.toml`のセクションは、登録コマンドによって作成されます。変更しないでください。

   ```toml
   concurrent = 1
   check_interval = 0

   [session_server]
     session_timeout = 1800

   name = "fargate-test"
   url = "https://gitlab.com/"
   token = "__REDACTED__"
   executor = "custom"
   ```

1. `sudo vim /etc/gitlab-runner/fargate.toml`を実行し、次のコンテンツを追加します:

   ```toml
   LogLevel = "info"
   LogFormat = "text"

   [Fargate]
     Cluster = "test-cluster"
     Region = "us-east-2"
     Subnet = "subnet-xxxxxx"
     SecurityGroup = "sg-xxxxxxxxxxxxx"
     TaskDefinition = "test-task:1"
     EnablePublicIP = true

   [TaskMetadata]
     Directory = "/opt/gitlab-runner/metadata"

   [SSH]
     Username = "root"
     Port = 22
   ```

   - `Cluster`の値と`TaskDefinition`の名前を書き留めます。この例では、`test-task`がリビジョン番号として`:1`と表示されています。リビジョン番号が指定されていない場合は、最新の**active**なリビジョンが使用されます。
   - リージョンを選択します。Runnerマネージャーインスタンスから`Subnet`の値を取得します。
   - セキュリティグループIDを見つける方法:

     1. AWSのインスタンスのリストで、作成したEC2インスタンスを選択します。詳細が表示されます。
     1. **Security groups**で、作成したグループの名前を選択します。
     1. **Security group ID**をコピーします。

     本番環境では、セキュリティグループの設定と使用に関する[AWSガイドライン](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-security-groups.html)に従ってください。

   - `EnablePublicIP`がtrueに設定されている場合、タスクコンテナのパブリックIPが収集され、SSH接続が実行されます。
   - `EnablePublicIP`がfalseに設定されている場合:
     - Fargateドライバーは、タスクコンテナのプライベートIPを使用します。`false`に設定されている場合に接続をセットアップするには、VPCセキュリティグループにポート22（SSH）のインバウンドルールが必要です。ソースはVPC CIDRです。
     - 外部依存関係をフェッチするには、プロビジョニングされたAWS Fargateコンテナがパブリックインターネットにアクセスできる必要があります。AWS Fargateコンテナにパブリックインターネットアクセスを提供するには、VPCでNATゲートウェイを使用できます。

   - SSHサーバーのポート番号はオプションです。省略した場合、デフォルトのSSHポート（22）が使用されます。
   - セクション設定の詳細については、[Fargateドライバードキュメント](https://gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate/-/tree/master/docs#configuration)を参照してください。

1. Fargateドライバーをインストールします:

   ```shell
   sudo curl -Lo /opt/gitlab-runner/fargate "https://gitlab-runner-custom-fargate-downloads.s3.amazonaws.com/latest/fargate-linux-amd64"
   sudo chmod +x /opt/gitlab-runner/fargate
   ```

## ステップ5: ECS Fargateクラスターを作成する {#step-5-create-an-ecs-fargate-cluster}

Amazon ECSクラスターは、ECSコンテナインスタンスのグループです。

1. [`https://console.aws.amazon.com/ecs/home#/clusters`](https://console.aws.amazon.com/ecs/home#/clusters)にアクセスします。
1. **Create Cluster**を選択します。
1. **Networking only**タイプを選択します。**次のステップ**を選択します。
1. 名前を`test-cluster`（`fargate.toml`と同じ）にします。
1. **Create**を選択します。
1. **View cluster**を選択します。`Cluster ARN`の値からリージョンとアカウントIDの部分を書き留めます。
1. **Update Cluster**を選択します。
1. `Default capacity provider strategy`の横にある**Add another provider**を選択し、`FARGATE`を選択します。**更新**を選択します。

ECS Fargateでのクラスターの設定と操作の詳細な手順については、[AWSドキュメント](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/Welcome.html)を参照してください。

## ステップ6: ECSタスク定義を作成する {#step-6-create-an-ecs-task-definition}

この手順では、タイプ`Fargate`のタスク定義を作成し、CIビルドに使用するコンテナイメージを参照します。

1. [`https://console.aws.amazon.com/ecs/home#/taskDefinitions`](https://console.aws.amazon.com/ecs/home#/taskDefinitions)にアクセスします。
1. **Create new Task Definition**を選択します。
1. **FARGATE**を選択し、**次のステップ**を選択します。
1. 名前を`test-task`にします。（注: 名前は`fargate.toml`ファイルで定義されているのと同じ値ですが、`:1`はありません）。
1. **Task memory (GB)**と**Task CPU (vCPU)**の値を選択します。
1. **Add container**を選択します。次に:
   1. `ci-coordinator`という名前を付けて、Fargateドライバーが`SSH_PUBLIC_KEY`環境変数を挿入できるようにします。
   1. イメージを定義します（例：`registry.gitlab.com/tmaczukin-test-projects/fargate-driver-debian:latest`）。
   1. 22/TCPのポートマッピングを定義します。
   1. **追加**を選択します。
1. **Create**を選択します。
1. **View task definition**を選択します。

{{< alert type="warning" >}}

単一のFargateタスクで、1つまたは複数のコンテナを起動できます。Fargateドライバーは、`ci-coordinator`という名前のコンテナにのみ、`SSH_PUBLIC_KEY`環境変数を挿入します。Fargateドライバーで使用されるすべてのタスク定義に、この名前のコンテナが必要です。この名前の付いたコンテナは、上記のように、SSHサーバーとすべてのGitLab Runnerの要件がインストールされているものである必要があります。

{{< /alert >}}

タスク定義の設定と操作の詳細な手順については、AWSの[ドキュメント](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/create-task-definition.html)を参照してください。

AWS ECRからイメージを起動するために必要なECSサービス許可については、[Amazon ECSタスク実行IAMロール](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html)を参照してください。

GitLabインスタンスでホストされているものを含む、プライベートレジストリへのECS認証については、[タスクのプライベートレジストリ認証](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/private-auth.html)を参照してください。

この時点で、RunnerマネージャーとFargateドライバーが構成され、AWS AWS Fargateでジョブの実行を開始する準備が完了します。

## ステップ7: 設定のテスト {#step-7-test-the-configuration}

これで設定を使用する準備ができました。

1. GitLabプロジェクトで、`.gitlab-ci.yml`ファイルを作成します:

   ```yaml
   test:
     script:
       - echo "It works!"
       - for i in $(seq 1 30); do echo "."; sleep 1; done
   ```

1. プロジェクトの**CI/CD > パイプライン**に移動します。
1. **Run Pipeline**を選択します。
1. ブランチとすべての変数を更新し、**Run Pipeline**を選択します。

{{< alert type="note" >}}

`.gitlab-ci.yml`ファイル内の`image`および`service`キーワードは無視されます。Runnerは、タスク定義で指定された値のみを使用します。

{{< /alert >}}

## クリーンアップ {#clean-up}

AWS AWS Fargateでカスタムexecutorをテストした後でクリーンアップを実行する場合は、次のオブジェクトを削除します:

- [手順3](#step-3-create-an-ec2-instance-for-gitlab-runner)で作成されたEC2インスタンス、キーペア、IAMロール、およびセキュリティグループ。
- [手順5](#step-5-create-an-ecs-fargate-cluster)で作成されたECS AWS Fargateクラスター。
- [手順6](#step-6-create-an-ecs-task-definition)で作成されたECSタスク定義。

## プライベートAWS AWS Fargateタスクの設定 {#configure-a-private-aws-fargate-task}

高度なセキュリティを確保するには、[プライベートAWS AWS Fargateタスク](https://repost.aws/knowledge-center/ecs-fargate-tasks-private-subnet)を設定します。この設定では、executorは内部AWS IPアドレスのみを使用します。CI/CDジョブがプライベートAWS AWS Fargateインスタンスで実行されるように、AWSからの送信トラフィックのみを許可します。

プライベートAWS AWS Fargateタスクを設定するには、次の手順を完了して、AWSを設定し、プライベートサブネットでAWS AWS Fargateタスクを実行します:

1. 既存のパブリックサブネットが、VPCアドレス範囲内のすべてのIPアドレスを予約していないことを確認します。VPCとサブネットの`cird`アドレス範囲を調べます。サブネット`cird`アドレス範囲がVPC `cird`アドレス範囲のサブセットである場合は、手順2と4をスキップします。それ以外の場合、VPCに使用可能なアドレス範囲がないため、VPCとパブリックサブネットを削除して再作成する必要があります:
   1. 既存のサブネットとVPCを削除します。
   1. 削除したVPCと同じ設定で[VPCを作成する](https://docs.aws.amazon.com/vpc/latest/privatelink/create-interface-endpoint.html#create-interface-endpoint)し、`cird`アドレス（例：`10.0.0.0/23`）を更新します。
   1. 削除したサブネットと同じ設定で[パブリックサブネットを作成する](https://docs.aws.amazon.com/vpc/latest/privatelink/interface-endpoints.html)。`cird`アドレス範囲（例：`10.0.0.0/24`）であるVPCアドレス範囲のサブセットであるアドレスを使用します。
1. パブリックサブネットと同じ設定で[プライベートサブネットを作成する](https://docs.aws.amazon.com/vpc/latest/userguide/create-subnet.html#create-subnets)。`cird`アドレス範囲（例：`10.0.1.0/24`）であるパブリックサブネット範囲と重複しないアドレス範囲を使用します。
1. [NATゲートウェイを作成する](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-nat-gateway.html)し、パブリックサブネット内に配置します。
1. 宛先`0.0.0.0/0`がNATゲートウェイを指すように、プライベートサブネットルーティングテーブルを変更します。
1. `farget.toml`設定を更新します:

   ```toml
   Subnet = "private-subnet-id"
   EnablePublicIP = false
   UsePublicIP = false
   ```

1. Fargateタスクに関連付けられているIAMロールに次のインラインポリシーを追加します（Fargateタスクに関連付けられているIAMロールは通常、`ecsTaskExecutionRole`という名前で、既に存在しているはずです）。

   ```json
   {
       "Statement": [
           {
               "Sid": "VisualEditor0",
               "Effect": "Allow",
               "Action": [
                   "secretsmanager:GetSecretValue",
                   "kms:Decrypt",
                   "ssm:GetParameters"
               ],
               "Resource": [
                   "arn:aws:secretsmanager:*:<account-id>:secret:*",
                   "arn:aws:kms:*:<account-id>:key/*"
               ]
           }
       ]
   }
   ```

1. セキュリティグループ自体の参照するように、セキュリティグループの「受信ルール」を変更します。AWS設定ダイアログで、以下を実行します:
   - `Type`を`ssh`に設定します。
   - `Source`を`Custom`に設定します。
   - セキュリティグループを選択します。
   - 任意のホストからのSSHアクセスを許可する既存の受信ルールを削除します。

{{< alert type="warning" >}}

既存の受信ルールを削除すると、SSHを使用してAmazon Elastic Compute Cloudインスタンスに接続できなくなります。

{{< /alert >}}

詳細については、次のAWSドキュメントを参照してください:

- [Amazon ECSタスク実行IAMロール](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html)
- [Amazon ECRインターフェースVPCエンドポイント（AWS PrivateLink）](https://docs.aws.amazon.com/AmazonECR/latest/userguide/vpc-endpoints.html)
- [Amazon ECSインターフェースVPCエンドポイント](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/vpc-endpoints.html)
- [パブリックサブネットとプライベートサブネットを持つVPC](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-example-private-subnets-nat.html)

## トラブルシューティング {#troubleshooting}

### 設定をテストする際のエラー`No Container Instances were found in your cluster` {#no-container-instances-were-found-in-your-cluster-error-when-testing-the-configuration}

`error="starting new Fargate task: running new task on Fargate: error starting AWS Fargate Task: InvalidParameterException: No Container Instances were found in your cluster."`

AWS AWS Fargateドライバーでは、[デフォルトのキャパシティプロバイダー戦略](#step-5-create-an-ecs-fargate-cluster)でECSクラスターが設定されている必要があります。

詳細情報:

- デフォルトの[キャパシティプロバイダー戦略](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/cluster-capacity-providers.html)は、各Amazon ECSクラスターに関連付けられています。他のキャパシティプロバイダー戦略または起動タイプが指定されていない場合、タスクの実行またはサービスの作成時に、クラスターはこの戦略を使用します。
- [`capacityProviderStrategy`](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_RunTask.html#ECS-RunTask-request-capacityProviderStrategy)が指定されている場合、`launchType`パラメータは省略する必要があります。`capacityProviderStrategy`または`launchType`が指定されていない場合、クラスターの`defaultCapacityProviderStrategy`が使用されます。

### ジョブの実行時のメタデータ`file does not exist`エラー {#metadata-file-does-not-exist-error-when-running-jobs}

`Application execution failed PID=xxxxx error="obtaining information about the running task: trying to access file \"/opt/gitlab-runner/metadata/<runner_token>-xxxxx.json\": file does not exist" cleanup_std=err job=xxxxx project=xx runner=<runner_token>`

IAMロールポリシーが正しく設定され、`/opt/gitlab-runner/metadata/`にメタデータJSONファイルを作成するための書き込み操作を実行できることを確認してください。非本番環境でテストするには、AmazonECS_FullAccessポリシーを使用します。組織のセキュリティ要件に従ってIAMロールポリシーを確認します。

### ジョブの実行時の`connection timed out` {#connection-timed-out-when-running-jobs}

`Application execution failed PID=xxxx error="executing the script on the remote host: executing script on container with IP \"172.x.x.x\": connecting to server: connecting to server \"172.x.x.x:22\" as user \"root\": dial tcp 172.x.x.x:22: connect: connection timed out"`

`EnablePublicIP`がfalseに設定されている場合は、VPCセキュリティグループに、SSH接続を許可する受信ルールがあることを確認してください。AWS AWS Fargateタスクコンテナは、GitLab Runner EC2インスタンスからのSSHトラフィックを受け入れる必要があります。

### ジョブの実行時の`connection refused` {#connection-refused-when-running-jobs}

`Application execution failed PID=xxxx error="executing the script on the remote host: executing script on container with IP \"10.x.x.x\": connecting to server: connecting to server \"10.x.x.x:22\" as user \"root\": dial tcp 10.x.x.x:22: connect: connection refused"`

タスクコンテナのポート22が公開されており、[手順6の指示に基づいてポートマッピングが設定されていることを確認します: ECSタスク定義を作成します](#step-6-create-an-ecs-task-definition)。ポートが公開されていて、コンテナが設定されている場合:

1. **Amazon ECS > Clusters > Choose your task definition > Tasks**で、コンテナのエラーがないか確認します。
1. `Stopped`ステータスのタスクを表示し、失敗した最新のタスクを確認します。コンテナに失敗がある場合、**logs**タブには詳細が表示されます。

または、Dockerコンテナをローカルで実行できることを確認します。

### エラー: `ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain` {#error-ssh-unable-to-authenticate-attempted-methods-none-publickey-no-supported-methods-remain}

AWS AWS Fargateドライバーの古いバージョンが原因で、サポートされていないキータイプが使用されている場合、次のエラーが発生します。

`Application execution failed PID=xxxx error="executing the script on the remote host: executing script on container with IP \"172.x.x.x\": connecting to server: connecting to server \"172.x.x.x:22\" as user \"root\": ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain"`

この問題を解決するには、最新のAWS AWS FargateドライバーをGitLab Runner EC2インスタンスにインストールします:

```shell
sudo curl -Lo /opt/gitlab-runner/fargate "https://gitlab-runner-custom-fargate-downloads.s3.amazonaws.com/latest/fargate-linux-amd64"
sudo chmod +x /opt/gitlab-runner/fargate
```
