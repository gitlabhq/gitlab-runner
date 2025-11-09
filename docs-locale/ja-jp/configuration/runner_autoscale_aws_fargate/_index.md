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

Fargateドライバーは、コミュニティでサポートされています。GitLabサポートは、問題のデバッグを支援しますが、保証は提供しません。

{{< /alert >}}

[カスタムexecutor](../../executors/custom.md)のGitLabドライバーは、[AWS Fargate](https://gitlab.com/gitlab-org/ci-cd/custom-executor-drivers/fargate)のAmazon Elastic Container Service (ECS) 上でコンテナを自動的に起動し、各GitLab CIジョブを実行します。

このドキュメントのタスクを完了すると、executorはGitLabから開始されたジョブを実行できます。GitLabでコミットが行われるたびに、GitLabインスタンスは新しいジョブが利用可能であることをRunnerに通知します。Runnerは、AWS ECSで設定したタスク定義に基づいて、ターゲットECSクラスターで新しいタスクを開始します。任意のDockerイメージを使用するようにAWS ECSタスク定義を設定できます。このアプローチでは、AWS Fargateで実行できるビルドのタイプを完全に柔軟に設定できます。

![GitLab Runner Fargateドライバーアーキテクチャ](../img/runner_fargate_driver_ssh.png)

このドキュメントでは、実装の最初の理解を深めることを目的とした例を示します。これは本番環境での使用を目的としていません。AWSでは追加のセキュリティが必要です。

たとえば、2つのAWSセキュリティグループが必要になる場合があります:

- GitLab Runnerをホストし、制限された外部IP範囲（管理アクセス用）からのSSH接続のみを受け入れるEC2インスタンスによって使用されるもの。
- Fargateタスクに適用され、EC2インスタンスからのSSHトラフィックのみを許可するもの。

パブリックでないコンテナレジストリの場合、ECSタスクには、[（AWS ECRのみの）IAM権限](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html)または非ECRプライベートレジストリの[タスクのプライベートレジストリ認証](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/private-auth.html)のいずれかが必要です。

CloudFormationまたはTerraformを使用して、AWSインフラストラクチャのプロビジョニングとセットアップを自動化できます。

{{< alert type="warning" >}}

CI/CDジョブは、`.gitlab-ci.yml`ファイルの`image:`キーワードの値ではなく、ECSタスクで定義されたイメージを使用します。ECSでは、ECSタスクに使用されるイメージをオーバーライドできません。

この制限を回避するには、次の操作を実行します:

- Runnerが使用されるすべてのプロジェクトのすべてのビルド依存関係を含むECSタスク定義でイメージを作成して使用します。
- 異なるイメージを持つ複数のECSタスク定義を作成し、`FARGATE_TASK_DEFINITION`CI/CD変数にARNを指定します。
- 公式の[AWS EKS Blueprints](https://aws-ia.github.io/terraform-aws-eks-blueprints/)に従って、EKSクラスターの作成を検討してください。

詳細については、[GitLab EKS Fargate Runnerを1時間とゼロコードで開始する方法](https://about.gitlab.com/blog/2023/05/24/eks-fargate-runner/)を参照してください。

{{< /alert >}}

{{< alert type="warning" >}}

Fargateはコンテナホストを抽象化するため、コンテナホストのプロパティの設定が制限されます。これは、ディスクまたはネットワークへの高いIOを必要とするRunnerワークロードに影響します。これらのプロパティは、Fargateでの設定が制限されているか、または設定できないためです。FargateでGitLab Runnerを使用する前に、CPU、メモリ、ディスクIO、またはネットワークIOで高いコンピューティング特性を持つRunnerワークロードがFargateに適していることを確認してください。

{{< /alert >}}

## 前提要件 {#prerequisites}

始める前に、以下が必要です:

- EC2、ECS、およびECRリソースを作成および設定するための権限を持つAWS IAMユーザー。
- AWS VPCとサブネット。
- 1つ以上のAWSセキュリティグループ。

## ステップ1: AWS Fargateタスク用のコンテナイメージを準備する {#step-1-prepare-a-container-image-for-the-aws-fargate-task}

コンテナイメージを準備します。このイメージをレジストリにアップロードできます。これは、GitLabジョブの実行時にコンテナの作成に使用できます。

1. イメージに、CIジョブのビルドに必要なツールがあることを確認します。たとえば、Javaプロジェクトには`Java JDK`や、MavenやGradleなどのビルドツールが必要です。Node.jsプロジェクトには、`node`と`npm`が必要です。
1. イメージに、アーティファクトとキャッシュを処理するGitLab Runnerがあることを確認してください。詳細については、カスタムexecutorドキュメントの[実行](../../executors/custom.md#run)ステージセクションを参照してください。
1. コンテナイメージが公開キー認証を介してSSH接続を受け入れることができることを確認します。Runnerは、この接続を使用して、`.gitlab-ci.yml`ファイルで定義されたビルドコマンドをAWS Fargate上のコンテナに送信します。SSHキーは、Fargateドライバーによって自動的に管理されます。コンテナは、`SSH_PUBLIC_KEY`環境変数からのキーを受け入れることができる必要があります。

GitLab RunnerとSSH設定を含む[Debianの例](https://gitlab.com/tmaczukin-test-projects/fargate-driver-debian)をご覧ください。[Node.jsの例](https://gitlab.com/aws-fargate-driver-demo/docker-nodejs-gitlab-ci-fargate)をご覧ください。

## ステップ2: コンテナイメージをレジストリにプッシュする {#step-2-push-the-container-image-to-a-registry}

イメージを作成したら、ECSタスク定義で使用するために、イメージをコンテナレジストリに公開します。

- リポジトリを作成してイメージをECRにプッシュするには、[Amazon ECRリポジトリ](https://docs.aws.amazon.com/AmazonECR/latest/userguide/Repositories.html)ドキュメントに従ってください。
- AWS CLIを使用してイメージをECRにプッシュするには、[AWS CLIを使用したAmazon ECRの開始方法](https://docs.aws.amazon.com/AmazonECR/latest/userguide/getting-started-cli.html)ドキュメントに従ってください。
- [GitLabコンテナレジストリ](https://docs.gitlab.com/user/packages/container_registry/)を使用するには、[Debian](https://gitlab.com/tmaczukin-test-projects/fargate-driver-debian)または[NodeJS](https://gitlab.com/aws-fargate-driver-demo/docker-nodejs-gitlab-ci-fargate)の例を使用できます。Debianイメージは、`registry.gitlab.com/tmaczukin-test-projects/fargate-driver-debian:latest`に公開されています。NodeJSのイメージ例は、`registry.gitlab.com/aws-fargate-driver-demo/docker-nodejs-gitlab-ci-fargate:latest`に公開されています。

## ステップ3: GitLab Runner用のEC2インスタンスを作成する {#step-3-create-an-ec2-instance-for-gitlab-runner}

次に、AWS EC2インスタンスを作成します。次のステップでは、GitLab Runnerをインストールします。

1. [https://console.aws.amazon.com/ec2/v2/home#LaunchInstanceWizard](https://console.aws.amazon.com/ec2/v2/home#LaunchInstanceWizard)にアクセスします。
1. インスタンスの場合は、Ubuntu Server 18.04 LTS AMIを選択します。選択したAWSリージョンによっては、名前が異なる場合があります。
1. インスタンスタイプの場合は、t2.microを選択します。**次へ: インスタンスの詳細**を選択します。
1. **Number of instances**（インスタンス数）のデフォルトのままにします。
1. **ネットワーク**の場合は、VPCを選択します。
1. **Auto-assign Public IP**（パブリックIPの自動割り当て）を**有効**に設定します。
1. **IAM role**（IAMロール）で、**Create new IAM role**（新しいIAMロールを作成）を選択します。このロールはテスト目的のみのものであり、安全ではありません。
   1. **ロールを作成する**を選択します。
   1. **AWS service**（AWSサービス）を選択し、**Common use cases**（一般的なユースケース）で**EC2**（EC2）を選択します。次に、**次へ: 権限**を選択します。
   1. **AmazonECS_FullAccess**（AmazonECS_FullAccess）ポリシーのチェックボックスをオンにします。**次へ: タグ**を選択します。
   1. **次へ: レビュー**を選択します。
   1. IAMロールの名前（例：`fargate-test-instance`）を入力し、**ロールを作成する**を選択します。
1. インスタンスを作成しているブラウザータブに戻ります。
1. **Create new IAM role**（新しいIAMロールを作成）の左側にある更新ボタンを選択します。`fargate-test-instance`ロールを選択します。**次へ: ストレージの追加**を選択します。
1. **次へ: タグの追加**を選択します。
1. **次へ: セキュリティグループ**を選択します。
1. **Create a new security group**（新しいセキュリティグループを作成）を選択し、`fargate-test`という名前を付け、SSHのルールが定義されていることを確認します（`Type: SSH, Protocol: TCP, Port Range: 22`）。インバウンドルールとアウトバウンドルールのIP範囲を指定する必要があります。
1. **Review and Launch**（レビューと起動）を選択します。
1. **Launch**（起動）を選択します。
1. （オプション）**Create a new key pair**（新しいキーペアを作成）を選択し、`fargate-runner-manager`という名前を付けて、**Download Key Pair**（キーペアをダウンロード）を選択します。SSHのプライベートキーがコンピューターにダウンロードされます（ブラウザーで設定されているディレクトリを確認してください）。
1. **Launch Instances**（インスタンスを起動）を選択します。
1. **View Instances**（インスタンスを表示）を選択します。
1. インスタンスが起動するまで待ちます。`IPv4 Public IP`アドレスを書き留めます。

## ステップ4: EC2インスタンスにGitLab Runnerをインストールして設定する {#step-4-install-and-configure-gitlab-runner-on-the-ec2-instance}

UbuntuインスタンスにGitLab Runnerをインストールします。

1. GitLabプロジェクトの**設定 > CI/CD**に移動し、Runnerセクションを展開します。**Set up a specific Runner manually**（特定Runnerを手動でセットアップ）で、登録トークンを書き留めます。
1. `chmod 400 path/to/downloaded/key/file`を実行して、キーファイルに適切な権限があることを確認します。
1. 次を使用して、作成したEC2インスタンスにSSHで接続します:

   ```shell
   ssh ubuntu@[ip_address] -i path/to/downloaded/key/file
   ```

1. 正常に接続されたら、次のコマンドを実行します:

   ```shell
   sudo mkdir -p /opt/gitlab-runner/{metadata,builds,cache}
   curl -s "https://packages.gitlab.com/install/repositories/runner/gitlab-runner/script.deb.sh" | sudo bash
   sudo apt install gitlab-runner
   ```

1. ステップ1でメモしたGitLab URLと登録トークンを使用して、このコマンドを実行します。

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

   [証明書の信頼の詳細](../tls-self-signed.md#trusting-the-certificate-for-the-other-cicd-stages)。

   以下に示す`config.toml`ファイルのセクションは、登録コマンドによって作成されます。変更しないでください。

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

   - `Cluster`の値と`TaskDefinition`タスク定義の名前を書き留めます。この例は、リビジョン番号として`:1`を持つ`test-task`を示しています。リビジョン番号が指定されていない場合は、最新の**active**（アクティブ）なリビジョンが使用されます。
   - リージョンを選択します。Runnerマネージャーインスタンスから`Subnet`の値を取得します。
   - セキュリティグループIDを見つけるには:

     1. AWSで、インスタンスのリストで、作成したEC2インスタンスを選択します。詳細が表示されます。
     1. **Security groups**（セキュリティグループ）で、作成したグループの名前を選択します。
     1. **Security group ID**（セキュリティグループID） をコピーします。

     本番環境では、セキュリティグループのセットアップと使用に関する[AWSガイドライン](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-security-groups.html)に従ってください。

   - `EnablePublicIP`がtrueに設定されている場合、SSH接続を実行するためにタスクコンテナのパブリックIPが収集されます。
   - `EnablePublicIP`がfalseに設定されている場合:
     - Fargateドライバーは、タスクコンテナのプライベートIPを使用します。`false`に設定されているときに接続をセットアップするには、ソースがVPC CIDRである場合、VPCセキュリティグループにポート22（SSH）のインバウンドルールが必要です。
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

1. [`https://console.aws.amazon.com/ecs/home#/clusters`](https://console.aws.amazon.com/ecs/home#/clusters)に移動します。
1. **Create Cluster**（クラスターの作成）を選択します。
1. **Networking only**（ネットワークのみ）タイプを選択します。**次のステップ**を選択します。
1. `fargate.toml`の場合と同じ`test-cluster`という名前を付けます。
1. **作成**を選択します。
1. **View cluster**（クラスターの表示）を選択します。`Cluster ARN`の値から、リージョンとアカウントIDの部分を書き留めます。
1. **Update Cluster**（クラスターの更新）を選択します。
1. `Default capacity provider strategy`の横にある**Add another provider**（別のプロバイダーを追加）を選択し、`FARGATE`を選択します。**更新**を選択します。

ECS Fargateでのクラスターのセットアップと操作の詳細については、AWS[ドキュメント](https://docs.aws.amazon.com/AmazonECS/latest/userguide/create_cluster.html)を参照してください。

## ステップ6: ECSタスク定義を作成する {#step-6-create-an-ecs-task-definition}

このステップでは、タイプ`Fargate`のタスク定義を作成し、CIビルドに使用できるコンテナイメージを参照します。

1. [`https://console.aws.amazon.com/ecs/home#/taskDefinitions`](https://console.aws.amazon.com/ecs/home#/taskDefinitions)に移動します。
1. **Create new Task Definition**（新しいタスク定義を作成）を選択します。
1. **FARGATE**（FARGATE）を選択し、**次のステップ**を選択します。
1. 名前を`test-task`にします。注: 名前は`fargate.toml`ファイルで定義されているのと同じ値ですが、`:1`はありません）。
1. **Task memory (GB)**（タスク）メモリ（GB）および**Task CPU (vCPU)**（タスクCPU（vCPU））の値を選択します。
1. **Add container**（コンテナの追加）を選択します。次に:
   1. `ci-coordinator`という名前を付けます。これにより、Fargateドライバーが`SSH_PUBLIC_KEY`環境変数を挿入できます。
   1. イメージを定義します（例：`registry.gitlab.com/tmaczukin-test-projects/fargate-driver-debian:latest`）。
   1. 22/TCPのポートマッピングを定義します。
   1. **追加**を選択します。
1. **作成**を選択します。
1. **View task definition**（タスク定義を表示）を選択します。

{{< alert type="warning" >}}

1つのFargateタスクで、複数のコンテナを起動できます。Fargateドライバーは、`ci-coordinator`名を持つコンテナにのみ`SSH_PUBLIC_KEY`環境変数を挿入します。Fargateドライバーで使用されるすべてのタスク定義に、この名前のコンテナが必要です。この名前を持つコンテナは、上記のように、SSHサーバーとすべてのGitLab Runner要件がインストールされているものである必要があります。

{{< /alert >}}

タスク定義のセットアップと操作の詳細については、AWS[ドキュメント](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/create-task-definition.html)を参照してください。

AWS ECRからイメージを起動するために必要なECSサービス権限については、[Amazon ECSタスク実行IAMロール](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html)を参照してください。

GitLabインスタンスでホストされているものを含む、プライベートレジストリへのECS認証については、[タスクのプライベートレジストリ認証](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/private-auth.html)を参照してください。

この時点で、RunnerマネージャーとFargateドライバーが設定され、AWS Fargateでジョブの実行を開始する準備ができました。

## ステップ7: 設定をテストします {#step-7-test-the-configuration}

構成が使用できる状態になりました。

1. GitLabプロジェクトで、`.gitlab-ci.yml`ファイルを作成します:

   ```yaml
   test:
     script:
       - echo "It works!"
       - for i in $(seq 1 30); do echo "."; sleep 1; done
   ```

1. プロジェクトの**CI/CD > パイプライン**に移動します。
1. **Run Pipeline**（パイプラインの実行）を選択します。
1. ブランチと変数を更新し、**Run Pipeline**（パイプラインの実行）を選択します。

{{< alert type="note" >}}

`.gitlab-ci.yml`ファイルの`image`および`service`キーワードは無視されます。Runnerは、タスク定義で指定された値のみを使用します。

{{< /alert >}}

## クリーンアップ {#clean-up}

AWS Fargateでカスタムexecutorをテストした後でクリーンアップを実行する場合は、次のオブジェクトを削除します:

- [ステップ3](#step-3-create-an-ec2-instance-for-gitlab-runner)で作成されたEC2インスタンス、キーペア、IAMロール、およびセキュリティグループ。
- [ステップ5](#step-5-create-an-ecs-fargate-cluster)で作成されたECS Fargateクラスター。
- [ステップ6](#step-6-create-an-ecs-task-definition)で作成されたECSタスク定義。

## プライベートAWS Fargateタスクを構成する {#configure-a-private-aws-fargate-task}

高度なセキュリティを確保するために、[プライベートAWS Fargateタスク](https://repost.aws/knowledge-center/ecs-fargate-tasks-private-subnet)を構成します。この構成では、executorは内部AWS IPアドレスのみを使用します。CI/CDジョブがプライベートAWS Fargateインスタンスで実行されるように、AWSからの送信トラフィックのみを許可します。

プライベートAWS Fargateタスクを構成するには、次の手順に従ってAWSを構成し、プライベートサブネットでAWS Fargateタスクを実行します:

1. 既存のパブリックサブネットが、VPCアドレス範囲内のすべてのIPアドレスを予約していないことを確認します。VPCとサブネットの`cird`アドレス範囲を調べます。サブネット`cird`アドレス範囲がVPC `cird`アドレス範囲のサブセットである場合は、手順2と4をスキップします。それ以外の場合、VPCに使用可能なアドレス範囲がないため、VPCとパブリックサブネットを削除して再作成する必要があります:
   1. 既存のサブネットとVPCを削除します。
   1. 削除したVPCと同じ構成で[VPCを作成](https://docs.aws.amazon.com/vpc/latest/privatelink/create-interface-endpoint.html#create-interface-endpoint)し、`cird`アドレス（例：`10.0.0.0/23`）を更新します。
   1. 削除したサブネットと同じ構成で[パブリックサブネットを作成](https://docs.aws.amazon.com/vpc/latest/privatelink/interface-endpoints.html)します。VPCアドレス範囲のサブセットである`cird`アドレス（例：`10.0.0.0/24`）を使用します。
1. パブリックサブネットと同じ構成で[プライベートサブネットを作成](https://docs.aws.amazon.com/vpc/latest/userguide/working-with-subnets.html#create-subnets)します。パブリックサブネット範囲とオーバーラップしない`cird`アドレス範囲（例：`10.0.1.0/24`）を使用します。
1. [NATゲートウェイを作成](https://docs.aws.amazon.com/vpc/latest/userguide/vpc-nat-gateway.html)し、パブリックサブネット内に配置します。
1. 宛先`0.0.0.0/0`がNATゲートウェイを指すように、プライベートサブルートテーブルを変更します。
1. `farget.toml`の設定をタグ付けします:

   ```toml
   Subnet = "private-subnet-id"
   EnablePublicIP = false
   UsePublicIP = false
   ```

1. 次のインラインポリシーを、Fargateタスクに関連付けられたIAMロールに追加します（Fargateタスクに関連付けられたIAMロールは通常`ecsTaskExecutionRole`という名前で、既に存在しているはずです）。

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

1. セキュリティグループの「受信ルール」を変更して、セキュリティグループ自体を参照するようにします。AWS構成ダイアログで:
   - `Type`を`ssh`に設定します。
   - `Source`を`Custom`に設定します。
   - セキュリティグループを選択します。
   - すべてのホストからのSSHアクセスを許可する既存の受信ルールを削除します。

{{< alert type="warning" >}}

既存の受信ルールを削除すると、SSHを使用してAmazon Elastic Compute Cloudインスタンスに接続できなくなります。

{{< /alert >}}

詳細については、次のAWSドキュメントを参照してください:

- [Amazon ECSタスク実行IAMロール](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/task_execution_IAM_role.html)
- [Amazon ECRインターフェースVPCエンドポイント（AWS PrivateLink）](https://docs.aws.amazon.com/AmazonECR/latest/userguide/vpc-endpoints.html)
- [Amazon ECSインターフェースVPCエンドポイント](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/vpc-endpoints.html)
- [パブリックサブネットとプライベートサブネットを備えたVPC](https://docs.aws.amazon.com/vpc/latest/userguide/VPC_Scenario2.html)

## トラブルシューティング {#troubleshooting}

### 構成のテスト中に`No Container Instances were found in your cluster`エラーが発生しました {#no-container-instances-were-found-in-your-cluster-error-when-testing-the-configuration}

`error="starting new Fargate task: running new task on Fargate: error starting AWS Fargate Task: InvalidParameterException: No Container Instances were found in your cluster."`

AWS Fargateドライバーでは、ECSクラスターが[デフォルトのキャパシティープロバイダー戦略](#step-5-create-an-ecs-fargate-cluster)で構成されている必要があります。

さらに詳しく:

- デフォルトの[キャパシティープロバイダー戦略](https://docs.aws.amazon.com/AmazonECS/latest/developerguide/cluster-capacity-providers.html)は、各Amazon ECSクラスターに関連付けられています。他のキャパシティープロバイダー戦略または起動タイプが指定されていない場合、タスクの実行時またはサービスの作成時に、クラスターはこの戦略を使用します。
- [`capacityProviderStrategy`](https://docs.aws.amazon.com/AmazonECS/latest/APIReference/API_RunTask.html#ECS-RunTask-request-capacityProviderStrategy)が指定されている場合、`launchType`パラメータは省略する必要があります。`capacityProviderStrategy`または`launchType`が指定されていない場合は、クラスターの`defaultCapacityProviderStrategy`が使用されます。

### ジョブの実行中にメタデータ`file does not exist`エラーが発生しました {#metadata-file-does-not-exist-error-when-running-jobs}

`Application execution failed PID=xxxxx error="obtaining information about the running task: trying to access file \"/opt/gitlab-runner/metadata/<runner_token>-xxxxx.json\": file does not exist" cleanup_std=err job=xxxxx project=xx runner=<runner_token>`

IAMロールポリシーが正しく構成されており、`/opt/gitlab-runner/metadata/`にメタデータJSONファイルを作成する書き込み操作を実行できることを確認してください。本番環境以外の環境でテストするには、AmazonECS_FullAccessポリシーを使用します。組織のセキュリティ要件に従って、IAMロールポリシーをレビューしてください。

### ジョブの実行中に`connection timed out`が発生しました {#connection-timed-out-when-running-jobs}

`Application execution failed PID=xxxx error="executing the script on the remote host: executing script on container with IP \"172.x.x.x\": connecting to server: connecting to server \"172.x.x.x:22\" as user \"root\": dial tcp 172.x.x.x:22: connect: connection timed out"`

`EnablePublicIP`がfalseに構成されている場合は、VPCセキュリティグループに、SSH接続を許可する受信ルールがあることを確認してください。AWS Fargateタスクコンテナは、GitLab Runner EC2インスタンスからのSSHトラフィックを受け入れる必要があります。

### ジョブの実行中に`connection refused`が発生しました {#connection-refused-when-running-jobs}

`Application execution failed PID=xxxx error="executing the script on the remote host: executing script on container with IP \"10.x.x.x\": connecting to server: connecting to server \"10.x.x.x:22\" as user \"root\": dial tcp 10.x.x.x:22: connect: connection refused"`

タスクコンテナの22番ポートが公開されており、[ステップ6の指示に基づいてポートマッピングが構成されていることを確認してください: ECSタスク定義を作成します](#step-6-create-an-ecs-task-definition)。ポートが公開され、コンテナが構成されている場合:

1. **Amazon ECS > Clusters > Choose your task definition > Tasks**（Amazon ECS > クラスター > タスク定義を選択 > タスク）で、コンテナのエラーがあるかどうかを確認します。
1. `Stopped`のステータスのタスクを表示し、失敗した最新のタスクを確認します。**logs**（ログ）タブには、コンテナに障害が発生した場合の詳細が表示されます。

または、ローカルでDockerコンテナを実行できることを確認してください。

<!-- markdownlint-disable line-length -->

### ジョブの実行中に`ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain`が発生しました {#ssh-handshake-failed-ssh-unable-to-authenticate-attempted-methods-none-publickey-no-supported-methods-remain-when-running-jobs}

サポートされていないキータイプが、古いバージョンのAWS Fargateドライバーが原因で使用されている場合、次のエラーが発生します。

`Application execution failed PID=xxxx error="executing the script on the remote host: executing script on container with IP \"172.x.x.x\": connecting to server: connecting to server \"172.x.x.x:22\" as user \"root\": ssh: handshake failed: ssh: unable to authenticate, attempted methods [none publickey], no supported methods remain"`

この問題を解決するには、最新のAWS FargateドライバーをGitLab Runner EC2インスタンスにインストールします:

```shell
sudo curl -Lo /opt/gitlab-runner/fargate "https://gitlab-runner-custom-fargate-downloads.s3.amazonaws.com/latest/fargate-linux-amd64"
sudo chmod +x /opt/gitlab-runner/fargate
```

<!-- markdownlint-enable line-length -->
