---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "Autoscaler de groupe d'instances GitLab Runner"
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

L'autoscaler de groupe d'instances GitLab Runner est le successeur de la technologie de mise à l'échelle automatique basée sur Docker Machine. Les composants de la solution de mise à l'échelle automatique de groupe d'instances GitLab Runner sont :

- Taskscaler :  Gère la logique de mise à l'échelle automatique, la comptabilité, et crée des flottes pour les instances de runner qui utilisent des groupes de mise à l'échelle automatique d'instances du fournisseur cloud.
- [Fleeting](../fleet_scaling/fleeting.md) :  Une abstraction pour les machines virtuelles des fournisseurs cloud.
- Plugin de fournisseur cloud :  Gère les appels API vers la plateforme cloud cible et est implémenté à l'aide d'un framework de développement de plugins.

La mise à l'échelle automatique de groupe d'instances dans GitLab Runner fonctionne comme suit :

1. Le gestionnaire de runner interroge en permanence les jobs GitLab.
1. En réponse, GitLab envoie les charges utiles des jobs au gestionnaire de runner.
1. Le gestionnaire de runner interagit avec l'infrastructure cloud publique pour créer une nouvelle instance afin d'exécuter les jobs.
1. Le gestionnaire de runner distribue ces jobs aux runners disponibles dans le pool de mise à l'échelle automatique.

![Vue d'ensemble de la mise à l'échelle automatique du prochain Runner GitLab](img/next-runner-autoscaling-overview.png)

## Configurer le gestionnaire de runner {#configure-the-runner-manager}

Vous devez [configurer le gestionnaire de runner](_index.md#configure-the-runner-manager) pour utiliser l'autoscaler de groupe d'instances GitLab Runner.

1. Créez une instance pour héberger le gestionnaire de runner. Il **ne doit pas** s'agir d'une instance spot (AWS) ou d'une machine virtuelle spot (GCP ou Azure).
1. [Installez GitLab Runner](../install/linux-repository.md) sur l'instance.
1. Ajoutez les identifiants du fournisseur cloud à la machine hôte du gestionnaire de runner.

   > [!note]
   > Vous pouvez héberger le gestionnaire de runner dans un conteneur. Pour GitLab.com et GitLab Dedicated, les [runners hébergés](https://docs.gitlab.com/ci/runners/), le gestionnaire de runner est hébergé sur une instance de machine virtuelle.

### Exemple de configuration des identifiants pour l'autoscaler de groupe d'instances GitLab Runner {#example-credentials-configuration-for-gitlab-runner-instance-group-autoscaler}

Vous pouvez utiliser un profil d'instance [AWS Identity and Access Management](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html) (IAM) pour le gestionnaire de runner dans l'environnement AWS. Si vous ne souhaitez pas héberger le gestionnaire de runner dans AWS, vous pouvez utiliser un fichier d'identifiants.

Par exemple :

``` toml
## credentials_file

[default]
aws_access_key_id=__REDACTED__
aws_secret_access_key=__REDACTED__
```

Le fichier d'identifiants est facultatif.

## Instances de cloud public prises en charge {#supported-public-cloud-instances}

Les options de mise à l'échelle automatique suivantes sont prises en charge pour les instances de calcul cloud public :

- Instances Amazon Web Services EC2
- Google Compute Engine
- Microsoft Azure Virtual Machines

Ces instances cloud sont également prises en charge par l'autoscaler Docker Machine de GitLab Runner.

## Plateformes prises en charge {#supported-platforms}

| Exécuteur                   | Linux                                | macOS                                | Windows                              |
|----------------------------|--------------------------------------|--------------------------------------|--------------------------------------|
| Exécuteur d'instance          | {{< icon name="check-circle" >}} Oui | {{< icon name="check-circle" >}} Oui | {{< icon name="check-circle" >}} Oui |
| Exécuteur Docker Autoscaler | {{< icon name="check-circle" >}} Oui | {{< icon name="dotted-circle" >}} Non | {{< icon name="check-circle" >}} Oui |
