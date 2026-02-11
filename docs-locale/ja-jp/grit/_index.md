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

[GitLab Runner Infrastructure Toolkit (GRIT)](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit)は、パブリッククラウドプロバイダー上で多くの一般的なランナーの設定を作成および管理するために使用できる、Terraformモジュールのライブラリです。

{{< alert type="note" >}}

これは[実験的機能](https://docs.gitlab.com/policy/development_stages_support/#experiment)です。GRIT開発の状況について詳しくは、[エピック1](https://gitlab.com/groups/gitlab-org/ci-cd/runner-tools/-/epics/1)をご覧ください。この機能に関するフィードバックを提供するには、[イシュー84](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/issues/84)にコメントを残してください。

{{< /alert >}}

## GRITでランナーを作成する {#create-a-runner-with-grit}

GRITを使用して、AWSでオートスケールLinux Dockerをデプロイするには、次の手順を実行します:

1. GitLabおよびAWSへのアクセスを提供するには、次の変数を設定します:

   - `GITLAB_TOKEN`
   - `AWS_REGION`
   - `AWS_SECRET_ACCESS_KEY`
   - `AWS_ACCESS_KEY_ID`

1. 最新の[GRITリリース](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/releases)をダウンロードし、`.local/grit`に展開します。
1. `main.tf`Terraformモジュールを作成します:

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

これらの手順では、GitLabプロジェクトに新しいランナーを作成します。ランナーマネージャーは、`docker-autoscaler` executorを使用して、`aws`および`linux`としてタグ付けされたジョブを実行します。ランナーは、ワークロードに基づいて、新しいオートスケールグループ（ASG）を介して1 ～ 5個のVMをプロビジョニングします。ASGは、ランナーチームが所有するパブリックAMIを使用します。ランナーマネージャーとASGはどちらも、新しいVPCで動作します。すべてのリソースは、指定された値（`grit-runner`）に基づいて命名されます。これにより、単一のAWSプロジェクト内で、異なる名前を持つこのモジュールの複数のインスタンスを作成できます。

## サポートレベルと`min_support`パラメータ {#support-levels-and-the-min_support-parameter}

すべてのGRITモジュールに`min_support`値を指定する必要があります。このパラメータは、オペレーターがデプロイに必要な最小サポートレベルを指定します。GRITモジュールは、`none`、`experimental`、`beta`、または`GA`のサポート指定に関連付けられています。目標は、すべてのモジュールが`GA`ステータスに到達することです。

`none`は特殊なケースです。主にテストおよび開発を目的とした、サポート保証のないモジュール。

`experimental`、`beta`、および`ga`のモジュールは、[GitLabの開発ステージの定義](https://docs.gitlab.com/policy/development_stages_support/)に準拠しています。

### 責任共有モデル {#shared-responsibility-model}

GRITは、作成者（モジュールの開発者）とオペレーター（GRITでデプロイするユーザー）間の責任共有モデルに基づいて動作します。各ロールの具体的な責任とサポートレベルの決定方法について詳しくは、GORPドキュメントの[「責任の共有」セクション](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/GORP.md#shared-responsibility)をご覧ください。

## ランナーの状態を管理する {#manage-runner-state}

ランナーを維持するには、次の手順を実行します:

1. GitLabプロジェクトにモジュールをチェックインします。
1. Terraformの状態をGitLab Terraformの`backend.tf`に保存します:

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

### ランナーを削除する {#delete-a-runner}

ランナーとそのインフラストラクチャを削除するには、次の手順を実行します:

```plaintext
terraform destroy
```

## サポートされている設定 {#supported-configurations}

| プロバイダー     | サービス | アーキテクチャ   | OS    | executor         | 機能サポート |
|--------------|---------|--------|-------|-------------------|-----------------|
| AWS          | EC2     | x86-64 | Linux | Docker Autoscaler | 実験的    |
| AWS          | EC2     | Arm64  | Linux | Docker Autoscaler | 実験的    |
| Google Cloud | GCE     | x86-64 | Linux | Docker Autoscaler | 実験的    |
| Google Cloud | GKE     | x86-64 | Linux | Kubernetes        | 実験的    |

## 高度な設定 {#advanced-configuration}

### トップレベルモジュール {#top-level-modules}

プロバイダーのトップレベルモジュールは、高度に分離されているか、ランナーのオプションの設定の側面を表します。たとえば、`fleeting`と`runner`は、アクセス認証情報とインスタンスグループ名のみを共有するため、別個のモジュールです。`vpc`は、一部のユーザーが独自のVPCを提供するため、別個のモジュールです。既存のVPCを持つユーザーは、他のGRITモジュールと接続するために、一致する入力構造を作成するだけで済みます。

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

GRITは、コミュニティからのコントリビューションを歓迎します。コントリビュートする前に、次のリソースを確認してください:

### デベロッパーCertificate of Originおよびライセンス {#developer-certificate-of-origin-and-license}

GRITへのすべてのコントリビューションは、[デベロッパーCertificate of Originおよびライセンス](https://docs.gitlab.com/legal/developer_certificate_of_origin/)に従うものとします。コントリビュートすることにより、現在および将来のGitLab, Inc. に提出されたコントリビューションに対するこれらの利用規約に同意したものとみなされます。

### 行動規範 {#code-of-conduct}

GRITは、[コントリビューター規約](https://www.contributor-covenant.org)から採用されたGitLabの行動規範に従います。このプロジェクトは、バックグラウンドやアイデンティティに関係なく、誰もがハラスメントのない体験ができるようにすることに取り組んでいます。

### コントリビューションのガイドライン {#contribution-guidelines}

GRITにコントリビュートする場合は、次のガイドラインに従ってください:

- 全体的なアーキテクチャ設計については、[GORPガイドライン](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/GORP.md)を確認してください。
- [Terraformを使用するためのGoogleのベストプラクティス](https://docs.cloud.google.com/docs/terraform/best-practices/general-style-structure)に従ってください。
- 複雑さと反復を軽減するために、再利用可能なモジュールアプローチに従ってください。
- コントリビューションに適切なGoテストを含めます。

### テストとLint {#testing-and-linting}

GRITは、品質を確保するために、いくつかのテストツールとLintツールを使用しています:

- 統合テスト: Terraformプランを検証するために、[Terratest](https://terratest.gruntwork.io/)を使用します。
- エンドツーエンドテスト: [e2eディレクトリ](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/e2e/README.md)で利用できます。
- Terraform Lint: `tflint`、`terraform fmt`、および`terraform validate`を使用します。
- Go Lint: Goコード（主にテスト）には、[golangci-lint](https://golangci-lint.run/)を使用します。
- ドキュメント: [GitLabドキュメントのスタイルガイドライン](https://docs.gitlab.com/development/documentation/styleguide/)に従い、`vale`と`markdownlint`を使用します。

開発環境のセットアップ、テストの実行、Lintの詳細な手順については、[CONTRIBUTING.md](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/CONTRIBUTING.md)を参照してください。

## GRITのユーザー {#who-uses-grit}

GRITは、GitLabエコシステム内のさまざまなチームやサービスで採用されています:

- **[GitLab Dedicated](https://about.gitlab.com/dedicated/)**: [GitLab Dedicatedのホストされたランナー](https://docs.gitlab.com/administration/dedicated/hosted_runners/)は、GRITを使用してランナーインフラストラクチャをプロビジョニングおよび管理します。

- **GitLab Self-Managed**: GRITは、多くのGitLab Self-Managedのお客様から非常に要望されています。一部の組織では、標準化された方法でランナーのデプロイを管理するために、GRITの採用を開始しています。

組織でGRITを使用していて、このセクションで紹介したい場合は、マージリクエストを開いてください。
