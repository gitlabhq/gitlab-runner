---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "Configurer la mise à l'échelle automatique Docker Machine du runner sur AWS EC2"
---

{{< details >}}

- Niveau : Free, Premium, Ultimate
- Offre : GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

L'un des plus grands avantages de GitLab Runner est sa capacité à lancer et arrêter automatiquement des machines virtuelles pour s'assurer que vos builds sont traités immédiatement. C'est une fonctionnalité formidable qui, utilisée correctement, peut s'avérer extrêmement utile dans les situations où vous n'utilisez pas vos runners 24h/24 et 7j/7 et où vous souhaitez disposer d'une solution rentable et évolutive.

## Introduction {#introduction}

Dans ce tutoriel, nous allons explorer comment configurer correctement GitLab Runner dans AWS. L'instance dans AWS servira de gestionnaire de runners qui crée de nouvelles instances Docker à la demande. Les runners sur ces instances sont créés automatiquement. Ils utilisent les paramètres présentés dans ce guide et ne nécessitent pas de configuration manuelle après leur création.

En outre, nous utiliserons les [instances Spot EC2 d'Amazon](https://aws.amazon.com/ec2/spot/) qui réduiront considérablement les coûts des instances GitLab Runner tout en utilisant des machines de mise à l'échelle automatique très puissantes.

## Prérequis {#prerequisites}

Une connaissance d'Amazon Web Services (AWS) est requise, car c'est là que la majeure partie de la configuration aura lieu.

Nous vous conseillons de lire rapidement la [documentation du pilote `amazonec2` de Docker Machine](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md) pour vous familiariser avec les paramètres que nous définirons plus loin dans cet article.

Votre GitLab Runner devra communiquer avec votre instance GitLab via le réseau, et c'est quelque chose à prendre en compte lors de la configuration des groupes de sécurité AWS ou lors de la configuration de votre DNS.

Par exemple, vous pouvez isoler les ressources EC2 du trafic public dans un VPC différent pour renforcer la sécurité de votre réseau. Votre environnement est probablement différent, alors réfléchissez à ce qui convient le mieux à votre situation.

### Groupes de sécurité AWS {#aws-security-groups}

Docker Machine tentera d'utiliser un [groupe de sécurité par défaut](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md/#security-group) avec des règles pour le port `2376` et SSH `22`, ce qui est requis pour la communication avec le démon Docker. Au lieu de vous fier à Docker, vous pouvez créer un groupe de sécurité avec les règles dont vous avez besoin et le fournir dans les options de GitLab Runner comme nous allons le [voir ci-dessous](#the-runnersmachine-section). De cette façon, vous pouvez le personnaliser à votre convenance à l'avance en fonction de votre environnement réseau. Vous devez vous assurer que les ports `2376` et `22` sont accessibles par l'[instance du gestionnaire de runners](#prepare-the-runner-manager-instance).

### Identifiants AWS {#aws-credentials}

Vous aurez besoin d'une [clé d'accès AWS](https://docs.aws.amazon.com/IAM/latest/UserGuide/security-creds.html) liée à un utilisateur ayant l'autorisation de mettre à l'échelle (EC2) et de mettre à jour le cache (via S3). Créez un nouvel utilisateur avec des [politiques](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/iam-policies-for-amazon-ec2.html) pour EC2 (AmazonEC2FullAccess) et S3. Pour plus d'informations sur les autorisations minimales requises pour S3, consultez [`runners.cache.s3`](../advanced-configuration.md#the-runnerscaches3-section). Pour plus de sécurité, vous pouvez désactiver la connexion à la console pour cet utilisateur. Laissez l'onglet ouvert ou copiez-collez les identifiants de sécurité dans un éditeur, car nous les utiliserons plus tard lors de la [configuration de GitLab Runner](#the-runnersmachine-section).

Vous pouvez également créer un [profil d'instance EC2](https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_use_switch-role-ec2_instance-profiles.html) avec les politiques `AmazonEC2FullAccess` et `AmazonS3FullAccess` requises.

Pour provisionner de nouvelles instances EC2 pour l'exécution des jobs, attachez ce profil d'instance à l'instance EC2 du gestionnaire de runners. Si la machine runner utilise un profil d'instance, incluez l'action `iam:PassRole` dans le profil d'instance du gestionnaire de runners.

Exemple :

```json
{
    "Statement": [
        {
            "Action": "iam:PassRole",
            "Effect": "Allow",
            "Resource": "arn:aws:iam:::role/instance-profile-of-runner-machine"
        }
    ],
    "Version": "2012-10-17"
}
```

## Préparer l'instance du gestionnaire de runners {#prepare-the-runner-manager-instance}

La première étape consiste à installer GitLab Runner dans une instance EC2 qui servira de gestionnaire de runners pour créer de nouvelles machines. Choisissez une distribution prise en charge par Docker et GitLab Runner, comme Ubuntu, Debian, CentOS ou RHEL.

Il n'est pas nécessaire que ce soit une machine puissante, car une instance de gestionnaire de runners n'exécute pas elle-même des jobs. Pour votre configuration initiale, vous pouvez commencer avec une instance plus petite. Cette machine est un hôte dédié car nous avons besoin qu'elle soit toujours opérationnelle. Par conséquent, c'est le seul hôte avec un coût de base continu.

Installez les prérequis :

1. Connectez-vous à votre serveur
1. [Installer GitLab Runner depuis le dépôt officiel GitLab](../../install/linux-repository.md)
1. [Installer Docker](https://docs.docker.com/engine/install/#server)
1. [Installer Docker Machine depuis le fork GitLab](https://gitlab.com/gitlab-org/ci-cd/docker-machine) (Docker a abandonné Docker Machine)

Maintenant que le runner est installé, il est temps de l'enregistrer.

## Enregistrement du GitLab Runner {#registering-the-gitlab-runner}

Avant de configurer le GitLab Runner, vous devez d'abord l'enregistrer afin qu'il se connecte à votre instance GitLab :

1. [Obtenir un jeton de runner](https://docs.gitlab.com/ci/runners/)
1. [Enregistrer le runner](../../register/_index.md)
1. Lorsqu'on vous demande le type d'exécuteur, entrez `docker+machine`

Vous pouvez maintenant passer à la partie la plus importante : la configuration du GitLab Runner.

> [!note]
> Si vous souhaitez que tous les utilisateurs de votre instance puissent utiliser les runners à mise à l'échelle automatique, enregistrez le runner comme runner partagé.

## Configuration du runner {#configuring-the-runner}

Maintenant que le runner est enregistré, vous devez modifier son fichier de configuration et ajouter les options requises pour le pilote de machine AWS.

Commençons par le décomposer en différentes parties.

### La section globale {#the-global-section}

Dans la section globale, vous pouvez définir la limite des jobs pouvant être exécutés simultanément sur tous les runners (`concurrent`). Cela dépend fortement de vos besoins, comme le nombre d'utilisateurs que GitLab Runner devra gérer, la durée de vos builds, etc. Vous pouvez commencer avec une valeur faible comme `10`, puis augmenter ou diminuer cette valeur au fur et à mesure.

L'option `check_interval` définit la fréquence à laquelle le runner doit vérifier GitLab pour de nouveaux jobs, en secondes.

Exemple :

```toml
concurrent = 10
check_interval = 0
```

[D'autres options](../advanced-configuration.md#the-global-section) sont également disponibles.

### La section `runners` {#the-runners-section}

Dans la section `[[runners]]`, la partie la plus importante est l'`executor` qui doit être défini sur `docker+machine`. La plupart de ces paramètres sont pris en charge lors de l'enregistrement initial du runner.

`limit` définit le nombre maximum de machines (en cours d'exécution et inactives) que ce runner créera. Pour plus d'informations, consultez la [relation entre `limit`, `concurrent` et `IdleCount`](../autoscale.md#how-concurrent-limit-and-idlecount-generate-the-upper-limit-of-running-machines).

Exemple :

```toml
[[runners]]
  name = "gitlab-aws-autoscaler"
  url = "<URL of your GitLab instance>"
  token = "<Runner's token>"
  executor = "docker+machine"
  limit = 20
```

[D'autres options](../advanced-configuration.md#the-runners-section) sous `[[runners]]` sont également disponibles.

### La section `runners.docker` {#the-runnersdocker-section}

Dans la section `[runners.docker]`, vous pouvez définir l'image Docker par défaut à utiliser par les runners enfants si elle n'est pas définie dans [`.gitlab-ci.yml`](https://docs.gitlab.com/ci/yaml/). En utilisant `privileged = true`, tous les runners pourront exécuter [Docker dans Docker](https://docs.gitlab.com/ci/docker/using_docker_build/#use-docker-in-docker), ce qui est utile si vous prévoyez de créer vos propres images Docker via GitLab CI/CD.

Ensuite, nous utilisons `disable_cache = true` pour désactiver le mécanisme de cache interne de l'exécuteur Docker, car nous utiliserons le mode de cache distribué décrit dans la section suivante.

Exemple :

```toml
  [runners.docker]
    image = "alpine"
    privileged = true
    disable_cache = true
```

[D'autres options](../advanced-configuration.md#the-runnersdocker-section) sous `[runners.docker]` sont également disponibles.

### La section `runners.cache` {#the-runnerscache-section}

Pour accélérer vos jobs, GitLab Runner fournit un mécanisme de cache où des répertoires et/ou fichiers sélectionnés sont sauvegardés et partagés entre les jobs suivants. Bien que non requis pour cette configuration, il est recommandé d'utiliser le mécanisme de cache distribué fourni par GitLab Runner. Étant donné que de nouvelles instances seront créées à la demande, il est essentiel de disposer d'un emplacement commun où le cache est stocké.

Dans l'exemple suivant, nous utilisons Amazon S3 :

```toml
  [runners.cache]
    Type = "s3"
    Shared = true
    [runners.cache.s3]
      ServerAddress = "s3.amazonaws.com"
      AccessKey = "<your AWS Access Key ID>"
      SecretKey = "<your AWS Secret Access Key>"
      BucketName = "<the bucket where your cache should be kept>"
      BucketLocation = "us-west-2"
```

Voici quelques informations supplémentaires pour explorer davantage le mécanisme de cache :

- [Référence pour `runners.cache`](../advanced-configuration.md#the-runnerscache-section)
- [Référence pour `runners.cache.s3`](../advanced-configuration.md#the-runnerscaches3-section)
- [Déploiement et utilisation d'un serveur de cache pour GitLab Runner](../autoscale.md#distributed-runners-caching)
- [Fonctionnement du cache](https://docs.gitlab.com/ci/yaml/#cache)

### La section `runners.machine` {#the-runnersmachine-section}

Il s'agit de la partie la plus importante de la configuration, qui indique à GitLab Runner comment et quand créer de nouvelles instances Docker Machine ou supprimer les anciennes.

Nous nous concentrerons sur les options de machine AWS ; pour le reste des paramètres, consultez :

- [Algorithme de mise à l'échelle automatique et paramètres sur lesquels il est basé](../autoscale.md#autoscaling-algorithm-and-parameters) \- dépend des besoins de votre organisation
- [Périodes de mise à l'échelle automatique](../autoscale.md#configure-autoscaling-periods) \- utile lorsqu'il existe des périodes régulières dans votre organisation où aucun travail n'est effectué, par exemple les week-ends

Voici un exemple de la section `runners.machine` :

```toml
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
      "amazonec2-tags=runner-manager-name,gitlab-aws-autoscaler,gitlab,true,gitlab-runner-autoscale,true",
      "amazonec2-security-group=xxxxx",
      "amazonec2-instance-type=m4.2xlarge",
    ]
    [[runners.machine.autoscaling]]
      Periods = ["* * 9-17 * * mon-fri *"]
      IdleCount = 50
      IdleTime = 3600
      Timezone = "UTC"
    [[runners.machine.autoscaling]]
      Periods = ["* * * * * sat,sun *"]
      IdleCount = 5
      IdleTime = 60
      Timezone = "UTC"
```

Le pilote Docker Machine est défini sur `amazonec2` et le nom de la machine a un préfixe standard suivi de `%s` (obligatoire) qui est remplacé par l'ID du runner enfant : `gitlab-docker-machine-%s`.

Maintenant, selon votre infrastructure AWS, il existe de nombreuses options que vous pouvez configurer sous `MachineOptions`. Vous pouvez voir ci-dessous les plus courantes.

| Option de machine                                                         | Description |
|------------------------------------------------------------------------|-------------|
| `amazonec2-access-key=XXXX`                                            | La clé d'accès AWS de l'utilisateur autorisé à créer des instances EC2, voir [Identifiants AWS](#aws-credentials). |
| `amazonec2-secret-key=XXXX`                                            | La clé secrète AWS de l'utilisateur autorisé à créer des instances EC2, voir [Identifiants AWS](#aws-credentials). |
| `amazonec2-region=eu-central-2`                                        | La région à utiliser lors du lancement de l'instance. Vous pouvez l'omettre entièrement et la valeur par défaut `us-east-1` sera utilisée. |
| `amazonec2-vpc-id=vpc-xxxxx`                                           | Votre [ID VPC](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#vpc-id) dans lequel lancer l'instance. |
| `amazonec2-subnet-id=subnet-xxxx`                                      | L'ID de sous-réseau VPC AWS. |
| `amazonec2-zone=x`                                                     | Si non spécifié, la [zone de disponibilité est `a`](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#environment-variables-and-default-values), elle doit être définie sur la même zone de disponibilité que le sous-réseau spécifié, par exemple lorsque la zone est `eu-west-1b`, elle doit être `amazonec2-zone=b` |
| `amazonec2-use-private-address=true`                                   | Utilisez l'adresse IP privée des machines Docker, tout en créant une adresse IP publique. Utile pour maintenir le trafic interne et éviter des coûts supplémentaires. |
| `amazonec2-tags=runner-manager-name,gitlab-aws-autoscaler,gitlab,true,gitlab-runner-autoscale,true` | Paires clé-valeur de balises supplémentaires AWS, utiles pour identifier les instances dans la console AWS. La [balise](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/Using_Tags.html) « Name » est définie sur le nom de la machine par défaut. Nous définissons « runner-manager-name » pour correspondre au nom du runner défini dans `[[runners]]`, afin de pouvoir filtrer toutes les instances EC2 créées par une configuration de gestionnaire spécifique. |
| `amazonec2-security-group=xxxx`                                        | Nom du groupe de sécurité VPC AWS, et non l'ID du groupe de sécurité. Voir [Groupes de sécurité AWS](#aws-security-groups). |
| `amazonec2-instance-type=m4.2xlarge`                                   | Le type d'instance sur lequel les runners enfants s'exécuteront. |
| `amazonec2-ssh-user=xxxx`                                              | L'utilisateur qui aura un accès SSH à l'instance. |
| `amazonec2-iam-instance-profile=xxxx_runner_machine_inst_profile_name` | Le profil d'instance IAM à utiliser pour la machine runner. |
| `amazonec2-ami=xxxx_runner_machine_ami_id`                             | L'ID AMI de GitLab Runner pour une image spécifique. |
| `amazonec2-request-spot-instance=true`                                 | Utiliser la capacité EC2 disponible à un prix inférieur au tarif à la demande. |
| `amazonec2-spot-price=xxxx_runner_machine_spot_price=x.xx`             | Prix de l'offre pour les instances Spot (en dollars américains). Requiert que `--amazonec2-request-spot-instance flag` soit défini sur `true`. Si vous omettez `amazonec2-spot-price`, Docker Machine fixe le prix maximum à une valeur par défaut de `$0.50` par heure. |
| `amazonec2-security-group-readonly=true`                               | Définir le groupe de sécurité en lecture seule. |
| `amazonec2-userdata=xxxx_runner_machine_userdata_path`                 | Spécifier le chemin `userdata` de la machine runner. |
| `amazonec2-root-size=XX`                                               | La taille du disque racine de l'instance (en Go). |

Remarques :

- Sous `MachineOptions`, vous pouvez ajouter tout ce que le [pilote AWS Docker Machine prend en charge](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#options). Nous vous encourageons vivement à lire la documentation de Docker, car la configuration de votre infrastructure peut nécessiter l'application d'options différentes.
- Les instances enfants utiliseront par défaut Ubuntu 16.04, sauf si vous choisissez un ID AMI différent en définissant `amazonec2-ami`. Définissez uniquement les [systèmes d'exploitation de base pris en charge pour Docker Machine](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/os-base).
- Si vous spécifiez `amazonec2-private-address-only=true` comme l'une des options de machine, votre instance EC2 ne se verra pas attribuer d'adresse IP publique. C'est correct si votre VPC est configuré correctement avec une passerelle Internet (IGW) et que le routage fonctionne bien, mais c'est quelque chose à prendre en compte si vous avez une configuration plus complexe. Pour en savoir plus, consultez la [documentation Docker sur la connectivité VPC](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#vpc-connectivity).

[D'autres options](../advanced-configuration.md#the-runnersmachine-section) sous `[runners.machine]` sont également disponibles.

### Tout assembler {#getting-it-all-together}

Voici l'exemple complet de `/etc/gitlab-runner/config.toml` :

```toml
concurrent = 10
check_interval = 0

[[runners]]
  name = "gitlab-aws-autoscaler"
  url = "<URL of your GitLab instance>"
  token = "<runner's token>"
  executor = "docker+machine"
  limit = 20
  [runners.docker]
    image = "alpine"
    privileged = true
    disable_cache = true
  [runners.cache]
    Type = "s3"
    Shared = true
    [runners.cache.s3]
      ServerAddress = "s3.amazonaws.com"
      AccessKey = "<your AWS Access Key ID>"
      SecretKey = "<your AWS Secret Access Key>"
      BucketName = "<the bucket where your cache should be kept>"
      BucketLocation = "us-west-2"
  [runners.machine]
    IdleCount = 1
    IdleTime = 1800
    MaxBuilds = 100
    MachineDriver = "amazonec2"
    MachineName = "gitlab-docker-machine-%s"
    MachineOptions = [
      "amazonec2-access-key=XXXX",
      "amazonec2-secret-key=XXXX",
      "amazonec2-region=eu-central-1",
      "amazonec2-vpc-id=vpc-xxxxx",
      "amazonec2-subnet-id=subnet-xxxxx",
      "amazonec2-use-private-address=true",
      "amazonec2-tags=runner-manager-name,gitlab-aws-autoscaler,gitlab,true,gitlab-runner-autoscale,true",
      "amazonec2-security-group=XXXX",
      "amazonec2-instance-type=m4.2xlarge",
    ]
    [[runners.machine.autoscaling]]
      Periods = ["* * 9-17 * * mon-fri *"]
      IdleCount = 50
      IdleTime = 3600
      Timezone = "UTC"
    [[runners.machine.autoscaling]]
      Periods = ["* * * * * sat,sun *"]
      IdleCount = 5
      IdleTime = 60
      Timezone = "UTC"
```

## Réduire les coûts avec les instances Spot Amazon EC2 {#cutting-down-costs-with-amazon-ec2-spot-instances}

Comme [décrit par](https://aws.amazon.com/ec2/spot/) Amazon :

>
Les instances Spot Amazon EC2 vous permettent d'enchérir sur la capacité de calcul Amazon EC2 disponible. Étant donné que les instances Spot sont souvent disponibles à un tarif réduit par rapport aux tarifs à la demande, vous pouvez réduire considérablement le coût d'exécution de vos applications, augmenter la capacité de calcul et le débit de votre application pour le même budget, et activer de nouveaux types d'applications de cloud computing.

En plus des options [`runners.machine`](#the-runnersmachine-section) sélectionnées ci-dessus, dans `/etc/gitlab-runner/config.toml` sous la section `MachineOptions`, ajoutez ce qui suit :

```toml
    MachineOptions = [
      "amazonec2-request-spot-instance=true",
      "amazonec2-spot-price=",
    ]
```

Dans cette configuration avec un `amazonec2-spot-price` vide, AWS définit votre prix d'enchère pour une instance Spot au prix à la demande par défaut de cette classe d'instance. Si vous omettez complètement `amazonec2-spot-price`, Docker Machine fixera le prix maximum à une [valeur par défaut de 0,50 $ par heure](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/aws.md#environment-variables-and-default-values).

Vous pouvez personnaliser davantage votre demande d'instance Spot :

```toml
    MachineOptions = [
      "amazonec2-request-spot-instance=true",
      "amazonec2-spot-price=0.03",
      "amazonec2-block-duration-minutes=60"
    ]
```

Avec cette configuration, les machines Docker sont créées à l'aide d'instances Spot avec un prix de demande Spot maximum de 0,03 $ par heure et la durée de l'instance Spot est limitée à 60 minutes. Le nombre `0.03` mentionné ci-dessus n'est qu'un exemple, veillez donc à vérifier le tarif actuel en fonction de la région que vous avez choisie.

Pour en savoir plus sur les instances Spot Amazon EC2, consultez les liens suivants :

- <https://aws.amazon.com/ec2/spot/>
- <https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/spot-requests.html>
- <https://aws.amazon.com/ec2/spot/getting-started/>

### Avertissements concernant les instances Spot {#caveats-of-spot-instances}

Bien que les instances Spot soient un excellent moyen d'utiliser les ressources inutilisées et de minimiser les coûts de votre infrastructure, vous devez être conscient des implications.

L'exécution de jobs CI sur des instances Spot peut augmenter les taux d'échec en raison du modèle de tarification des instances Spot. Si le prix Spot maximum que vous spécifiez dépasse le prix Spot actuel, vous n'obtiendrez pas la capacité demandée. La tarification Spot est révisée sur une base horaire. Toutes les instances Spot existantes dont le prix maximum est inférieur au prix Spot révisé seront arrêtées dans les deux minutes, et tous les jobs sur les hôtes Spot échoueront.

En conséquence, le runner à mise à l'échelle automatique ne parviendrait pas à créer de nouvelles machines, tout en continuant à demander de nouvelles instances. Cela finira par générer 60 requêtes, après quoi AWS n'en acceptera plus. Ensuite, une fois que le prix Spot redevient acceptable, vous êtes bloqué pendant un moment car la limite du nombre d'appels est dépassée.

Si vous rencontrez ce cas, vous pouvez utiliser la commande suivante dans la machine gestionnaire de runners pour voir l'état de Docker Machine :

```shell
docker-machine ls -q --filter state=Error --format "{{.NAME}}"
```

> [!note]
> Il existe des problèmes liés à la gestion correcte par GitLab Runner des changements de prix Spot, et des rapports font état de `docker-machine` qui tente continuellement de supprimer une machine Docker. GitLab a fourni des correctifs pour les deux cas dans le projet en amont. Pour plus d'informations, consultez le [ticket 2771](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2771) et le [ticket 2772](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2772).

Le fork GitLab ne prend pas en charge les flottes AWS EC2 et leur utilisation avec des instances Spot. Comme alternative, vous pouvez utiliser le [fork en aval du Continuous Kernel Integration Project](https://gitlab.com/cki-project/mirror/docker-machine).

## Conclusion {#conclusion}

Dans ce guide, nous avons appris à installer et configurer un GitLab Runner en mode de mise à l'échelle automatique sur AWS.

L'utilisation de la fonctionnalité de mise à l'échelle automatique de GitLab Runner peut vous faire gagner du temps et de l'argent. L'utilisation des instances Spot fournies par AWS peut vous faire économiser encore plus, mais vous devez être conscient des implications. Tant que votre offre est suffisamment élevée, il ne devrait pas y avoir de problème.

Vous pouvez lire les cas d'utilisation suivants, qui ont (fortement) influencé ce tutoriel :

- [HumanGeo est passé de Jenkins à GitLab](https://about.gitlab.com/blog/humangeo-switches-jenkins-gitlab-ci/)
- [Substrakt Health - Mise à l'échelle automatique des runners GitLab CI/CD et économies de 90 % sur les coûts EC2](https://about.gitlab.com/blog/autoscale-ci-runners/)
