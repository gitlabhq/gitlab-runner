---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "Mise à l'échelle automatique de GitLab Runner"
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Vous pouvez utiliser la mise à l'échelle automatique de GitLab Runner pour mettre à l'échelle automatiquement le runner sur des instances de cloud public. Lorsque vous configurez un runner pour utiliser l'autoscaler, vous pouvez gérer les charges de job CI/CD accrues en exécutant plusieurs jobs simultanément sur votre infrastructure cloud.

En plus des options de mise à l'échelle automatique pour les instances de cloud public, vous pouvez utiliser les solutions d'orchestration de conteneurs suivantes pour héberger et mettre à l'échelle une flotte de runners.

- Clusters Kubernetes Red Hat OpenShift
- Clusters Kubernetes :  AWS EKS, Azure, sur site
- Clusters Amazon Elastic Container Services sur AWS Fargate

## Configurer le gestionnaire de runner {#configure-the-runner-manager}

Vous devez configurer le gestionnaire de runner pour utiliser la mise à l'échelle automatique de GitLab Runner, à la fois la solution Docker Machine Autoscaling et le GitLab Runner Autoscaler.

Le gestionnaire de runner est un type de runner qui crée plusieurs runners pour la mise à l'échelle automatique. Il interroge GitLab en continu pour les jobs et interagit avec l'infrastructure de cloud public pour créer une nouvelle instance afin d'exécuter des jobs. Le gestionnaire de runner doit s'exécuter sur une machine hôte sur laquelle GitLab Runner est installé. Choisissez une distribution prise en charge par Docker et GitLab Runner, comme Ubuntu, Debian, CentOS ou RHEL.

1. Créez une instance pour héberger le gestionnaire de runner. Il **ne doit pas** s'agir d'une instance spot (AWS) ou d'une machine virtuelle spot (GCP, Azure).
1. [Installez GitLab Runner](../install/linux-repository.md) sur l'instance.
1. Ajoutez les identifiants du fournisseur cloud à la machine hôte du gestionnaire de Runner.

> [!note]
> Vous pouvez héberger le gestionnaire de runner dans un conteneur. Pour les [runners hébergés par GitLab](https://docs.gitlab.com/ci/runners/), le gestionnaire de runner est hébergé sur une instance de machine virtuelle.

### Exemple de configuration des identifiants pour Docker Machine Autoscaling de GitLab Runner {#example-credentials-configuration-for-gitlab-runner-docker-machine-autoscaling}

Cet extrait se trouve dans la section `runners.machine` du fichier `config.toml`.

``` toml
  [runners.machine]
    IdleCount = 1
    IdleTime = 1800
    MaxBuilds = 10
    MachineDriver = "amazonec2"
    MachineName = "gitlab-docker-machine-%s"
    MachineOptions = [
      "amazonec2-access-key=XXXX",
      "amazonec2-secret-key=XXXX",
      "amazonec2-region=eu-central-1",
      "amazonec2-vpc-id=vpc-xxxxx",
      "amazonec2-subnet-id=subnet-xxxxx",
      "amazonec2-zone=x",
      "amazonec2-use-private-address=true",
      "amazonec2-security-group=xxxxx",
    ]
```

> [!note]
> Le fichier d'identifiants est facultatif. Vous pouvez utiliser un profil d'instance [AWS Identity and Access Management](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html) (IAM) pour le gestionnaire de runner dans l'environnement AWS. Si vous ne souhaitez pas héberger le gestionnaire de runner dans AWS, vous pouvez utiliser le fichier d'identifiants.

## Implémenter une conception tolérante aux pannes {#implement-a-fault-tolerant-design}

Commencez avec au moins deux gestionnaires de runner qui utilisent les mêmes tags de runner pour créer une conception tolérante aux pannes et prévenir les défaillances de l'hôte du gestionnaire de runner.

Par exemple, sur GitLab.com, plusieurs gestionnaires de runner sont configurés pour les [runners hébergés sur Linux](https://docs.gitlab.com/ci/runners/hosted_runners/linux/). Chaque gestionnaire de runner possède le tag `saas-linux-small-amd64`.

Utilisez l'observabilité et les métriques de la flotte de runners lorsque vous ajustez les paramètres de mise à l'échelle automatique pour équilibrer l'efficacité et les performances des charges de travail CI/CD de votre organisation.

## Configurer les exécuteurs de mise à l'échelle automatique du runner {#configure-runner-autoscaling-executors}

Après avoir configuré le gestionnaire de runner, configurez les exécuteurs spécifiques à la mise à l'échelle automatique :

- [Exécuteur d'instance](../executors/instance.md)
- [Exécuteur Docker Autoscaling](../executors/docker_autoscaler.md)
- [Exécuteur Docker Machine](../executors/docker_machine.md)

> [!note]
> Vous devriez utiliser les exécuteurs Instance et Docker Autoscaling, car ils constituent la technologie qui remplace l'autoscaler Docker Machine.
