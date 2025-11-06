---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: GitLab Runnerインフラストラクチャツールキット
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated
- ステータス: 実験的機能

{{< /details >}}

[GitLab Runner Infrastructure Toolkit（GRIT）](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit)は、パブリッククラウドプロバイダー上で一般的なrunner設定を多数作成および管理するために使用できる、Terraformモジュールのライブラリです。

{{< alert type="note" >}}

これは[実験的機能](https://docs.gitlab.com/policy/development_stages_support/#experiment)です。GRIT開発の状況について詳しくは、[エピック1](https://gitlab.com/groups/gitlab-org/ci-cd/runner-tools/-/epics/1)をご覧ください。この機能に関するフィードバックを提供するには、[issue 84](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/issues/84)にコメントを残してください。

{{< /alert >}}

## GRITでrunnerを作成する {#create-a-runner-with-grit}

GRITを使用して、Amazon Web ServicesでオートスケールLinux Dockerをデプロイするには、次の手順を実行します:

1. GitLabとAWSへのアクセスを提供するために、次の変数を設定します:

   - `GITLAB_TOKEN`
   - `AWS_REGION`
   - `AWS_SECRET_ACCESS_KEY`
   - `AWS_ACCESS_KEY_ID`

1. 最新の[GRITリリース](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/releases)をダウンロードし、`.local/grit`に展開します。
1. `main.tf` Terraformモジュールを作成します:

   ```hcl
   module "runner" {
     source = ".local/grit/scenarios/aws/linux/docker-autoscaler-default"

     name               = "grit-runner"
     gitlab_project_id  = "39258790" # gitlab.com/josephburnett/hello-runner
     runner_description = "Autoscaling Linux Docker runner on AWS deployed with GRIT. "
     runner_tags        = ["aws", "linux"]
     max_instances      = 5
     min_support        = "experimental"
   }
   ```

1. モジュールを初期化して適用します:

   ```plaintext
   terraform init
   terraform apply
   ```

これらの手順では、GitLabプロジェクトに新しいrunnerを作成します。Runnerマネージャーは、`docker-autoscaler` executorを使用して、`aws`および`linux`としてタグ付けされたジョブを実行します。runnerは、ワークロードに基づいて、新しいオートスケールグループ（ASG）を介して1〜5台のVMをプロビジョニングします。ASGは、runnerチームが所有するパブリックAMIを使用します。RunnerマネージャーとASGはどちらも、新しいVPCで動作します。すべてのリソースは、指定された値（`grit-runner`）に基づいて名前が付けられます。これにより、単一のAWSプロジェクトで、名前の異なるこのモジュールの複数のインスタンスを作成できます。

## サポートレベルと`min_support`パラメータ {#support-levels-and-the-min_support-parameter}

すべてのGRITモジュールに`min_support`値を指定する必要があります。このパラメータは、オペレーターがデプロイに必要な最小サポートレベルを指定します。GRITモジュールは、`none`、`experimental`、`beta`、または`GA`のサポート指定に関連付けられています。目標は、すべてのモジュールが`GA`ステータスに到達することです。

`none`は特別なケースです。主にテストと開発を目的とした、サポート保証のないモジュール。

`experimental`、`beta`、および`ga`モジュールは、[GitLabの開発ステージングの定義](https://docs.gitlab.com/policy/development_stages_support/)に準拠しています。

### 責任共有モデル {#shared-responsibility-model}

GRITは、作成者（モジュール開発者）とオペレーター（GRITでデプロイするユーザー）間の責任共有モデルに基づいて運用されます。各ロールの具体的な責任とサポートレベルの決定方法の詳細については、GORPドキュメントの[責任共有セクション](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/GORP.md#shared-responsibility)を参照してください。

## Runnerの状態を管理する {#manage-runner-state}

Runnerを維持するには、次の手順に従います:

1. モジュールをGitLabプロジェクトにチェックインします。
1. Terraformの状態をGitLab Terraform `backend.tf`に保存します:

   ```hcl
   terraform {
     backend "http" {}
   }
   ```

1. `.gitlab-ci.yml`を使用して変更を適用します:

   ```yaml
   terraform-apply:
     variables:
       TF_HTTP_LOCK_ADDRESS: "https://gitlab.com/api/v4/projects/${CI_PROJECT_ID}/terraform/state/${NAME}/lock"
       TF_HTTP_UNLOCK_ADDRESS: ${TF_HTTP_LOCK_ADDRESS}
       TF_HTTP_USERNAME: ${GITLAB_USER_LOGIN}
       TF_HTTP_PASSWORD: ${GITLAB_TOKEN}
       TF_HTTP_LOCK_METHOD: POST
       TF_HTTP_UNLOCK_METHOD: DELETE
     script:
       - terraform init
       - terraform apply -auto-approve
   ```

### Runnerを削除 {#delete-a-runner}

runnerとそのインフラストラクチャを削除するには:

```plaintext
terraform destroy
```

## サポートされている設定 {#supported-configurations}

| プロバイダー     | サービス | アーチ   | OS    | executor         | 機能のサポート |
|--------------|---------|--------|-------|-------------------|-----------------|
| AWS          | EC2     | x86-64 | Linux | Docker Autoscaler | 実験的    |
| AWS          | EC2     | Arm64  | Linux | Docker Autoscaler | 実験的    |
| Google Cloud | GCE     | x86-64 | Linux | Docker Autoscaler | 実験的    |
| Google Cloud | GKE     | x86-64 | Linux | Kubernetes        | 実験的    |

## 高度な設定 {#advanced-configuration}

### トップレベルモジュール {#top-level-modules}

プロバイダーのトップレベルモジュールは、高度に分離された、またはオプションのrunner設定の側面を表します。たとえば、`fleeting`と`runner`は、アクセス認証情報とインスタンスグループ名のみを共有するため、個別のモジュールです。`vpc`は、一部のユーザーが独自のVPCを提供するため、個別のモジュールです。既存のVPCを持つユーザーは、他のGRITモジュールに接続するために、一致する入力構造を作成するだけで済みます。

たとえば、トップレベルのVPCモジュールを使用して、VPCを必要とするモジュールのVPCを作成できます:

   ```hcl
   module "runner" {
      source = ".local/grit/modules/aws/runner"

      vpc = {
         id         = module.vpc.id
         subnet_ids = module.vpc.subnet_ids
      }

      # ...additional config omitted
   }

   module "vpc" {
      source   = ".local/grit/modules/aws/vpc"

      zone = "us-east-1b"

      cidr        = "10.0.0.0/16"
      subnet_cidr = "10.0.0.0/24"
   }
   ```

ユーザーは独自のVPCを提供でき、GRITのVPCモジュールを使用する必要はありません:

   ```hcl
   module "runner" {
      source = ".local/grit/modules/aws/runner"

      vpc = {
         id         = PREEXISTING_VPC_ID
         subnet_ids = [PREEXISTING_SUBNET_ID]
      }

      # ...additional config omitted
   }
   ```

## GRITへのコントリビュート {#contributing-to-grit}

GRITは、コミュニティからのコントリビュートを歓迎します。コントリビュートする前に、次のリソースを確認してください:

### 開発者のオリジン証明書とライセンス {#developer-certificate-of-origin-and-license}

GRITへのすべてのコントリビュートは、[開発者のオリジン証明書とライセンス](https://docs.gitlab.com/legal/developer_certificate_of_origin/)に従います。コントリビュートすることにより、GitLab Inc.に提出された現在および将来のコントリビュートについて、これらの条件に同意したことになります。

### 行動規範 {#code-of-conduct}

GRITはGitLabの行動規範に従います。これは、[コントリビューター規約](https://www.contributor-covenant.org)から採用されています。このプロジェクトは、バックグラウンドやアイデンティティに関係なく、誰にとってもハラスメントのない体験を保証することに取り組んでいます。

### コントリビューションのガイドライン {#contribution-guidelines}

GRITにコントリビュートする場合は、次のガイドラインに従ってください:

- 全体的なアーキテクチャ設計については、[GORPガイドライン](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/GORP.md)を確認してください。
- [Terraformを使用するためのGoogleのベストプラクティス](https://cloud.google.com/docs/terraform/best-practices/general-style-structure)を遵守してください。
- 複雑さと繰り返しを軽減するために、再利用可能なモジュールアプローチに従います。
- コントリビューションのために適切なGo言語テストを含めてください。

### テストとLint {#testing-and-linting}

GRITは、品質を確保するために、いくつかのテストツールとLintツールを使用しています:

- 統合テスト: [Terratest](https://terratest.gruntwork.io/)を使用して、Terraformプランを検証します。
- E2Eテスト: [e2eディレクトリ](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/e2e/README.md)で利用できます。
- Terraform Lint: `tflint`、`terraform fmt`、および`terraform validate`を使用します。
- Go言語Lint: Go言語コード（主にテスト）には、[golangci-Lint](https://golangci-lint.run/)を使用します。
- ドキュメント: [GitLabドキュメント](https://docs.gitlab.com/development/documentation/styleguide/)のスタイルガイドラインに従い、`vale`と`markdownlint`を使用します。

開発環境のセットアップ、テストの実行、およびLintの詳細な手順については、[CONTRIBUTING.md](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/CONTRIBUTING.md)を参照してください。

## GRITのユーザー {#who-uses-grit}

GRITは、GitLabエコシステム内のさまざまなチームやサービスで採用されています:

- **[GitLab Dedicated](https://about.gitlab.com/dedicated/)**: [GitLab Dedicatedのホストされたrunner](https://docs.gitlab.com/administration/dedicated/hosted_runners/)は、GRITを使用してrunnerインフラストラクチャをプロビジョニングおよび管理します。

- **GitLab Self-Managed**: GRITは、多くのGitLabセルフマネージドのお客様から非常に要望されています。一部の組織は、標準化された方法でrunnerのデプロイを管理するために、GRITの採用を開始しています。

組織でGRITを使用しており、このセクションで紹介されたい場合は、マージリクエストを開いてください。
