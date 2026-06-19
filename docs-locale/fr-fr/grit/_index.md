---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: GitLab Runner Infrastructure Toolkit
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated
- Statut :  Expérimentation

{{< /details >}}

Le [GitLab Runner Infrastructure Toolkit (GRIT)](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit) est une bibliothèque de modules Terraform que vous pouvez utiliser pour créer et gérer de nombreuses configurations de runner courantes sur des fournisseurs de cloud public.

> [!note]
> Cette fonctionnalité est une [expérimentation](https://docs.gitlab.com/policy/development_stages_support/#experiment). Pour plus d'informations sur l'état du développement de GRIT, consultez l'[epic 1](https://gitlab.com/groups/gitlab-org/ci-cd/runner-tools/-/epics/1). Pour faire part de vos commentaires sur cette fonctionnalité, laissez un commentaire sur le [ticket 84](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/issues/84).

## Créer un runner avec GRIT {#create-a-runner-with-grit}

Pour utiliser GRIT afin de déployer un Linux Docker à mise à l'échelle automatique dans AWS :

1. Définissez les variables suivantes pour fournir l'accès à GitLab et à AWS :

   - `GITLAB_TOKEN`
   - `AWS_REGION`
   - `AWS_SECRET_ACCESS_KEY`
   - `AWS_ACCESS_KEY_ID`

1. Téléchargez la dernière [release GRIT](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/releases) et extrayez-la dans `.local/grit`.
1. Créez un module Terraform `main.tf` :

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

1. Initialisez et appliquez le module :

   ```plaintext
   terraform init
   terraform apply
   ```

Ces étapes créent un nouveau runner dans un projet GitLab. Le gestionnaire de runner utilise l'exécuteur `docker-autoscaler` pour exécuter les jobs tagués `aws` et `linux`. Le runner provisionne entre 1 et 5 VMs via un nouveau groupe de mise à l'échelle automatique (ASG), en fonction de la charge de travail. L'ASG utilise une AMI publique appartenant à l'équipe runner. Le gestionnaire de runner et l'ASG fonctionnent tous deux dans un nouveau VPC. Toutes les ressources sont nommées d'après la valeur fournie (`grit-runner`), ce qui vous permet de créer plusieurs instances de ce module avec des noms différents dans un même projet AWS.

## Niveaux de support et paramètre `min_support` {#support-levels-and-the-min_support-parameter}

Vous devez fournir une valeur `min_support` pour tous les modules GRIT. Ce paramètre spécifie le niveau de support minimum requis par l'opérateur pour son déploiement. Les modules GRIT sont associés à une désignation de support `none`, `experimental`, `beta` ou `GA`. L'objectif est que tous les modules atteignent le statut `GA`.

`none` est un cas particulier. Modules sans garantie de support, principalement destinés aux tests et au développement.

Les modules `experimental`, `beta` et `ga` sont conformes aux [définitions GitLab des étapes de développement](https://docs.gitlab.com/policy/development_stages_support/).

### Modèle de responsabilité partagée {#shared-responsibility-model}

GRIT fonctionne selon un modèle de responsabilité partagée entre les auteurs (développeurs de modules) et les opérateurs (ceux qui déploient avec GRIT). Pour plus de détails sur les responsabilités spécifiques de chaque rôle et la façon dont les niveaux de support sont déterminés, consultez la [section Responsabilité partagée](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/GORP.md#shared-responsibility) dans la documentation GORP.

## Gérer l'état du runner {#manage-runner-state}

Pour maintenir les runners :

1. Enregistrez le module dans un projet GitLab.
1. Stockez l'état Terraform dans le fichier Terraform GitLab `backend.tf` :

   ```hcl
   terraform {
     backend "http" {}
   }
   ```

1. Appliquez les modifications en utilisant `.gitlab-ci.yml` :

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

### Supprimer un runner {#delete-a-runner}

Pour supprimer le runner et son infrastructure :

```plaintext
terraform destroy
```

## Configurations prises en charge {#supported-configurations}

| Fournisseur     | Service | Architecture   | SE    | Exécuteurs         | Support des fonctionnalités |
|--------------|---------|--------|-------|-------------------|-----------------|
| AWS          | EC2     | x86-64 | Linux | Docker Autoscaler | Expérimental    |
| AWS          | EC2     | Arm64  | Linux | Docker Autoscaler | Expérimental    |
| Google Cloud | GCE     | x86-64 | Linux | Docker Autoscaler | Expérimental    |
| Google Cloud | GKE     | x86-64 | Linux | Kubernetes        | Expérimental    |

## Configuration avancée {#advanced-configuration}

### Modules de niveau supérieur {#top-level-modules}

Les modules de niveau supérieur dans un fournisseur représentent des aspects de configuration hautement découplés ou facultatifs du runner. Par exemple, `fleeting` et `runner` sont des modules séparés car ils ne partagent que des identifiants d'accès et des noms de groupes d'instances. Le `vpc` est un module séparé car certains utilisateurs fournissent leur propre VPC. Les utilisateurs disposant de VPC existants doivent uniquement créer une structure d'entrée correspondante pour se connecter aux autres modules GRIT.

Par exemple, le module VPC de niveau supérieur peut être utilisé pour créer un VPC pour les modules qui en nécessitent un :

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

L'utilisateur peut fournir son propre VPC et ne pas utiliser le module VPC de GRIT :

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

## Contribuer à GRIT {#contributing-to-grit}

GRIT accueille favorablement les contributions de la communauté. Avant de contribuer, consultez les ressources suivantes :

### Certificat d'origine du développeur et licence {#developer-certificate-of-origin-and-license}

Toutes les contributions à GRIT sont soumises au [Certificat d'origine du développeur et à la licence](https://docs.gitlab.com/legal/developer_certificate_of_origin/). En contribuant, vous acceptez et convenez de ces termes et conditions pour vos contributions présentes et futures soumises à GitLab Inc.

### Code de conduite {#code-of-conduct}

GRIT suit le Code de conduite de GitLab, qui est adapté du [Contributor Covenant](https://www.contributor-covenant.org). Le projet s'engage à faire de la participation une expérience sans harcèlement pour tous, quelle que soit leur origine ou leur identité.

### Directives de contribution {#contribution-guidelines}

Lors de vos contributions à GRIT, suivez ces directives :

- Consultez les [directives GORP](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/GORP.md) pour la conception architecturale globale.
- Respectez les [bonnes pratiques de Google pour l'utilisation de Terraform](https://docs.cloud.google.com/docs/terraform/best-practices/general-style-structure).
- Suivez l'approche des modules composables pour réduire la complexité et la répétition.
- Incluez des tests Go appropriés pour vos contributions.

### Tests et linting {#testing-and-linting}

GRIT utilise plusieurs outils de test et de linting pour garantir la qualité :

- Tests d'intégration :  Utilise [Terratest](https://terratest.gruntwork.io/) pour valider les plans Terraform.
- Tests de bout en bout :  Disponible dans le [répertoire e2e](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/e2e/README.md).
- Linting Terraform :  Utilise `tflint`, `terraform fmt` et `terraform validate`.
- Linting Go :  Utilise [golangci-lint](https://golangci-lint.run/) pour le code Go (principalement les tests).
- Documentation :  Suit le [guide de style de la documentation GitLab](https://docs.gitlab.com/development/documentation/styleguide/) et utilise `vale` et `markdownlint`.

Pour des instructions détaillées sur la configuration de votre environnement de développement, l'exécution des tests et le linting, consultez [CONTRIBUTING.md](https://gitlab.com/gitlab-org/ci-cd/runner-tools/grit/-/blob/main/CONTRIBUTING.md).

## Qui utilise GRIT ? {#who-uses-grit}

GRIT a été adopté par diverses équipes et services au sein de l'écosystème GitLab :

- **[GitLab Dedicated](https://about.gitlab.com/dedicated/)** :  [Les runners hébergés pour GitLab Dedicated](https://docs.gitlab.com/administration/dedicated/hosted_runners/) utilisent GRIT pour provisionner et gérer l'infrastructure de runner.
- **GitLab Self-Managed** :  GRIT est très demandé parmi de nombreux clients GitLab Self-Managed. Certaines organisations ont commencé à adopter GRIT pour gérer leurs déploiements de runner de manière standardisée.

Si vous utilisez GRIT dans votre organisation et souhaitez être présenté dans cette section, ouvrez un merge request !
