---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Exécuteur Docker Autoscaler
---

{{< history >}}

- Introduit dans GitLab Runner 15.11.0 en tant qu'[expérience](https://docs.gitlab.com/policy/development_stages_support/#experiment).
- [Modifié](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29404) en [bêta](https://docs.gitlab.com/policy/development_stages_support/#beta) dans GitLab Runner 16.6.
- [Disponibilité générale](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29221) dans GitLab Runner 17.1.

{{< /history >}}

Avant d'utiliser l'exécuteur Docker Autoscaler, consultez le [ticket de retour d'information](https://gitlab.com/gitlab-org/gitlab/-/issues/408131) sur la mise à l'échelle automatique de GitLab Runner pour obtenir la liste des problèmes connus.

L'exécuteur Docker Autoscaler est un exécuteur Docker avec mise à l'échelle automatique activée, qui crée des instances à la demande pour traiter les jobs que le gestionnaire de runner traite. Il encapsule l'[exécuteur Docker](docker.md) de sorte que toutes les options et fonctionnalités de l'exécuteur Docker sont prises en charge.

Le Docker Autoscaler utilise les [plugins fleeting](https://gitlab.com/gitlab-org/fleeting/plugins) pour la mise à l'échelle automatique. Fleeting est une abstraction pour un groupe d'instances à mise à l'échelle automatique, qui utilise des plugins prenant en charge les fournisseurs cloud, tels que Google Cloud, AWS et Azure.

## Installer un plugin fleeting {#install-a-fleeting-plugin}

Pour installer un plugin pour votre plateforme cible, consultez [Installer le plugin fleeting](../fleet_scaling/fleeting.md#install-a-fleeting-plugin). Pour des détails de configuration spécifiques, consultez la [documentation du projet de plugin correspondant](https://gitlab.com/gitlab-org/fleeting/plugins).

## Configurer Docker Autoscaler {#configure-docker-autoscaler}

L'exécuteur Docker Autoscaler encapsule l'[exécuteur Docker](docker.md) de sorte que toutes les options et fonctionnalités de l'exécuteur Docker sont prises en charge.

Pour configurer le Docker Autoscaler, dans le `config.toml` :

- Dans la section [`[runners]`](../configuration/advanced-configuration.md#the-runners-section), spécifiez `executor` comme `docker-autoscaler`.
- Dans les sections suivantes, configurez le Docker Autoscaler en fonction de vos besoins :
  - [`[runners.docker]`](../configuration/advanced-configuration.md#the-runnersdocker-section)
  - [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section)

### Groupes de mise à l'échelle automatique dédiés pour chaque configuration de runner {#dedicated-autoscaling-groups-for-each-runner-configuration}

Chaque configuration de Docker Autoscaler doit disposer de sa propre ressource de mise à l'échelle automatique dédiée :

- Pour AWS, un groupe de mise à l'échelle automatique dédié
- Pour GCP, un groupe d'instances dédié
- Pour Azure, un groupe de machines virtuelles identiques dédié

Ne partagez pas ces ressources de mise à l'échelle automatique entre :

- Plusieurs gestionnaires de runner (installations GitLab Runner distinctes)
- Plusieurs entrées `[[runners]]` dans le `config.toml` du même gestionnaire de runner

Le Docker Autoscaler suit l'état des instances qui doit être synchronisé avec la ressource de mise à l'échelle automatique du fournisseur cloud. Lorsque plusieurs systèmes tentent de gérer la même ressource de mise à l'échelle automatique, ils peuvent émettre des commandes de mise à l'échelle conflictuelles, entraînant un comportement imprévisible, des échecs de jobs et potentiellement des coûts plus élevés.

### Exemple : Mise à l'échelle automatique AWS pour 1 job par instance {#example-aws-autoscaling-for-1-job-per-instance}

Prérequis :

- Une AMI avec [Docker Engine](https://docs.docker.com/engine/) installé. Pour permettre au Runner Manager d'accéder au socket Docker sur l'AMI, l'utilisateur doit faire partie du groupe `docker`.

  > [!note]
  > L'AMI ne nécessite pas l'installation de GitLab Runner. Les instances lancées à l'aide de l'AMI ne doivent pas s'enregistrer elles-mêmes en tant que runners dans GitLab.

- Un groupe de mise à l'échelle automatique AWS. Le runner gère directement tout le comportement de mise à l'échelle. Pour la politique de mise à l'échelle, utilisez `none` et activez la protection contre la réduction des instances. Si vous avez configuré plusieurs zones de disponibilité, désactivez le processus `AZRebalance`.
- Une politique IAM avec les [autorisations correctes](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy).

Cette configuration prend en charge :

- Une capacité par instance de 1
- Un nombre d'utilisations de 1
- Une échelle d'inactivité de 5
- Un délai d'inactivité de 20 minutes
- Un nombre maximal d'instances de 10

En définissant la capacité et le nombre d'utilisations à 1, chaque job reçoit une instance éphémère sécurisée qui ne peut pas être affectée par d'autres jobs. Dès que le job est terminé, l'instance sur laquelle il a été exécuté est immédiatement supprimée.

Avec une échelle d'inactivité de 5, le runner essaie de garder 5 instances entières (car la capacité par instance est de 1) disponibles pour la demande future. Ces instances restent disponibles pendant au moins 20 minutes.

Le champ `concurrent` du runner est défini sur 10 (nombre maximal d'instances × capacité par instance).

```toml
concurrent = 10

[[runners]]
  name = "docker autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"                                        # use powershell or pwsh for Windows AMIs

  # uncomment for Windows AMIs when the Runner manager is hosted on Linux
  # environment = ["FF_USE_POWERSHELL_PATH_RESOLVER=1"]

  executor = "docker-autoscaler"

  # Docker Executor config
  [runners.docker]
    image = "busybox:latest"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "aws" # in GitLab 16.11 and later, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # in GitLab 16.10 and earlier, manually install the plugin and use:
    # plugin = "fleeting-plugin-aws"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-docker-asg"               # AWS Autoscaling Group name
      profile          = "default"                     # optional, default is 'default'
      config_file      = "/home/user/.aws/config"      # optional, default is '~/.aws/config'
      credentials_file = "/home/user/.aws/credentials" # optional, default is '~/.aws/credentials'

    [runners.autoscaler.connector_config]
      username          = "ec2-user"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

### Exemple : Groupe d'instances Google Cloud pour 1 job par instance {#example-google-cloud-instance-group-for-1-job-per-instance}

Prérequis :

- Une image de VM avec [Docker Engine](https://docs.docker.com/engine/) installé, telle que [`COS`](https://docs.cloud.google.com/container-optimized-os/docs).

  > [!note]
  > L'image de VM ne nécessite pas l'installation de GitLab Runner. Les instances lancées à l'aide de l'image de VM ne doivent pas s'enregistrer elles-mêmes en tant que runners dans GitLab.

- Un groupe d'instances Google Cloud à zone unique. Pour **le mode de mise à l'échelle automatique**, sélectionnez **Ne pas utiliser la mise à l'échelle automatique**. Le runner gère la mise à l'échelle automatique, pas le groupe d'instances Google Cloud.

  > [!note]
  > Les groupes d'instances multi-zones ne sont actuellement pas pris en charge. Un [ticket](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud/-/issues/20) existe pour prendre en charge les groupes d'instances multi-zones à l'avenir.

- Une politique IAM avec les [autorisations correctes](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud#required-permissions). Si vous déployez votre runner dans un cluster GKE, vous pouvez ajouter une liaison IAM entre le compte de service Kubernetes et le compte de service GCP. Vous pouvez ajouter cette liaison avec le rôle `iam.workloadIdentityUser` pour vous authentifier auprès de GCP au lieu d'utiliser un fichier de clé avec `credentials_file`.

Cette configuration prend en charge :

- Une capacité par instance de 1
- Un nombre d'utilisations de 1
- Une échelle d'inactivité de 5
- Un délai d'inactivité de 20 minutes
- Un nombre maximal d'instances de 10

En définissant la capacité et le nombre d'utilisations à 1, chaque job reçoit une instance éphémère sécurisée qui ne peut pas être affectée par d'autres jobs. Dès que le job est terminé, l'instance sur laquelle il a été exécuté est immédiatement supprimée.

Avec une échelle d'inactivité de 5, le runner essaie de garder 5 instances entières (car la capacité par instance est de 1) disponibles pour la demande future. Ces instances restent disponibles pendant au moins 20 minutes.

Le champ `concurrent` du runner est défini sur 10 (nombre maximal d'instances × capacité par instance).

```toml
concurrent = 10

[[runners]]
  name = "docker autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"                                        # use powershell or pwsh for Windows Images

  # uncomment for Windows Images when the Runner manager is hosted on Linux
  # environment = ["FF_USE_POWERSHELL_PATH_RESOLVER=1"]

  executor = "docker-autoscaler"

  # Docker Executor config
  [runners.docker]
    image = "busybox:latest"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "googlecloud" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-googlecompute"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-docker-instance-group" # Google Cloud Instance Group name
      project          = "my-gcp-project"
      zone             = "europe-west1"
      credentials_file = "/home/user/.config/gcloud/application_default_credentials.json" # optional, default is '~/.config/gcloud/application_default_credentials.json'

    [runners.autoscaler.connector_config]
      username          = "runner"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

### Exemple : Groupe de machines virtuelles identiques Azure pour 1 job par instance {#example-azure-scale-set-for-1-job-per-instance}

Prérequis :

- Une image de VM Azure avec [Docker Engine](https://docs.docker.com/engine/) installé.

  > [!note]
  > L'image de VM ne nécessite pas l'installation de GitLab Runner. Les instances lancées à l'aide de l'image de VM ne doivent pas s'enregistrer elles-mêmes en tant que runners dans GitLab.

- Un groupe de machines virtuelles identiques Azure où la politique de mise à l'échelle automatique est définie sur `manual`. Le runner gère la mise à l'échelle.

Cette configuration prend en charge :

- Une capacité par instance de 1
- Un nombre d'utilisations de 1
- Une échelle d'inactivité de 5
- Un délai d'inactivité de 20 minutes
- Un nombre maximal d'instances de 10

Lorsque la capacité et le nombre d'utilisations sont tous deux définis sur `1`, chaque job reçoit une instance éphémère sécurisée qui ne peut pas être affectée par d'autres jobs. Lorsque le job est terminé, l'instance sur laquelle il a été exécuté est immédiatement supprimée.

Lorsque l'échelle d'inactivité est définie sur `5`, le runner conserve 5 instances disponibles pour la demande future (car la capacité par instance est de 1). Ces instances restent disponibles pendant au moins 20 minutes.

Le champ `concurrent` du runner est défini sur 10 (nombre maximal d'instances × capacité par instance).

```toml
concurrent = 10

[[runners]]
  name = "docker autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"                                        # use powershell or pwsh for Windows AMIs

  # uncomment for Windows AMIs when the Runner manager is hosted on Linux
  # environment = ["FF_USE_POWERSHELL_PATH_RESOLVER=1"]

  executor = "docker-autoscaler"

  # Docker Executor config
  [runners.docker]
    image = "busybox:latest"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "azure" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-azure"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name = "my-docker-scale-set"
      subscription_id = "9b3c4602-cde2-4089-bed8-889e5a3e7102"
      resource_group_name = "my-resource-group"

    [runners.autoscaler.connector_config]
      username = "azureuser"
      password = "my-scale-set-static-password"
      use_static_credentials = true
      timeout = "10m"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

## Prise en charge des cgroups par slot {#slot-based-cgroup-support}

L'exécuteur Docker Autoscaler prend en charge les cgroups par slot pour améliorer l'isolation des ressources entre les jobs simultanés. Les chemins de cgroup sont automatiquement appliqués aux conteneurs Docker à l'aide du flag `--cgroup-parent`.

Pour des informations détaillées sur les cgroups par slot, notamment les avantages, les prérequis et les instructions de configuration, consultez [la prise en charge des cgroups par slot](../configuration/slot_based_cgroups.md).

### Configuration spécifique à Docker {#docker-specific-configuration}

En plus de la configuration standard des cgroups par slot, vous pouvez spécifier un modèle de cgroup distinct pour les conteneurs de service :

```toml
[[runners]]
  executor = "docker+autoscaler"
  use_slot_cgroups = true
  slot_cgroup_template = "gitlab-runner/slot-${slot}"

  [runners.docker]
    service_slot_cgroup_template = "gitlab-runner/service-slot-${slot}"
```

Pour toutes les options disponibles, consultez la [documentation de configuration des cgroups par slot](../configuration/slot_based_cgroups.md#docker-specific-configuration).

## Dépannage {#troubleshooting}

### `ERROR: error during connect: ssh tunnel: EOF ()` {#error-error-during-connect-ssh-tunnel-eof-}

Lorsque des instances sont supprimées par une source externe (par exemple, un groupe de mise à l'échelle automatique ou un script automatisé), les jobs échouent avec l'erreur suivante :

```plaintext
ERROR: Job failed (system failure): error during connect: Post "http://internal.tunnel.invalid/v1.43/containers/xyz/wait?condition=not-running": ssh tunnel: EOF ()
```

Et les journaux de GitLab Runner affichent une erreur `instance unexpectedly removed` pour l'ID d'instance assigné au job :

```plaintext
ERROR: instance unexpectedly removed    instance=<instance_id> max-use-count=9999 runner=XYZ slots=map[] subsystem=taskscaler used=45
```

Pour résoudre cette erreur, vérifiez les événements liés à l'instance sur la plateforme de votre fournisseur cloud. Par exemple, sur AWS, vérifiez l'historique des événements CloudTrail pour la source d'événement `ec2.amazonaws.com`.

### `ERROR: Preparation failed: unable to acquire instance: context deadline exceeded` {#error-preparation-failed-unable-to-acquire-instance-context-deadline-exceeded}

Lorsque vous utilisez le [plugin AWS fleeting](https://gitlab.com/gitlab-org/fleeting/plugins/aws), les jobs peuvent échouer de manière intermittente avec l'erreur suivante :

```plaintext
ERROR: Preparation failed: unable to acquire instance: context deadline exceeded
```

Cela apparaît souvent dans les journaux AWS CloudWatch car le nombre d'instances `reserved` oscille à la hausse et à la baisse :

```plaintext
"2024-07-23T18:10:24Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:0,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
"2024-07-23T18:10:25Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:1,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
"2024-07-23T18:11:15Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:0,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
"2024-07-23T18:11:16Z","instance_count:1,max_instance_count:1000,acquired:0,unavailable_capacity:0,pending:0,reserved:1,idle_count:0,scale_factor:0,scale_factor_limit:0,capacity_per_instance:1","required scaling change",
```

Pour résoudre cette erreur, assurez-vous que le processus `AZRebalance` est désactivé pour votre groupe de mise à l'échelle automatique dans AWS.

### `Job failures when scaling from zero instances on Azure VMSS` {#job-failures-when-scaling-from-zero-instances-on-azure-vmss}

Les groupes de machines virtuelles identiques Microsoft Azure disposent d'une [fonctionnalité de surprovisionnement](https://learn.microsoft.com/en-us/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-design-overview#overprovisioning), qui peut provoquer des échecs de jobs. Lorsqu'Azure effectue une montée en charge, il crée des VM supplémentaires pour garantir la capacité, puis les résilie une fois la capacité demandée atteinte. Ce comportement entre en conflit avec le suivi des instances de GitLab Runner, ce qui amène l'autoscaler à assigner des jobs à des instances qu'Azure est sur le point de résilier.

Désactivez le surprovisionnement en définissant `overprovision` sur `false` dans votre configuration VMSS.
