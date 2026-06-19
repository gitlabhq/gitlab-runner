---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "Exécuteur d'instance"
---

{{< history >}}

- Introduit dans GitLab Runner 15.11.0 en tant qu'[expérience](https://docs.gitlab.com/policy/development_stages_support/#experiment).
- [Modifié](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29404) en [bêta](https://docs.gitlab.com/policy/development_stages_support/#beta) dans GitLab Runner 16.6.
- [Disponibilité générale](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29221) dans GitLab Runner 17.1.

{{< /history >}}

L'exécuteur d'instance est un exécuteur compatible avec la mise à l'échelle automatique qui crée des instances à la demande pour accueillir le volume de jobs attendu que le gestionnaire de runner traite.

Vous pouvez utiliser l'exécuteur d'instance lorsque les jobs nécessitent un accès complet à l'instance hôte, au système d'exploitation et aux périphériques connectés. L'exécuteur d'instance peut également être configuré pour prendre en charge les jobs à locataire unique et multi-locataires avec différents niveaux d'isolation et de sécurité.

## Virtualisation imbriquée {#nested-virtualization}

L'exécuteur d'instance prend en charge la virtualisation imbriquée avec le [démon nesting](https://gitlab.com/gitlab-org/fleeting/nesting) développé par GitLab. Le démon nesting permet la création et la suppression de machines virtuelles préconfigurées sur des systèmes hôtes utilisés pour des charges de travail isolées et de courte durée, comme les jobs. Nesting n'est pris en charge que sur les instances Apple Silicon.

## Préparer l'environnement pour la mise à l'échelle automatique {#prepare-the-environment-for-autoscaling}

Pour préparer l'environnement à la mise à l'échelle automatique :

1. [Installez un plugin fleeting](../fleet_scaling/fleeting.md#install-a-fleeting-plugin) pour votre plateforme cible où le gestionnaire de runner est installé et configuré.
1. Créez une image de VM pour la plateforme que vous utilisez. L'image doit inclure :
   - Git
   - Binaire GitLab Runner

     > [!note]
     > Pour traiter les artefacts de job et le cache, installez le binaire GitLab Runner sur la machine virtuelle et conservez l'exécutable du runner dans le chemin par défaut. L'image de VM ne nécessite pas l'exécution de GitLab Runner. Les instances lancées à l'aide de l'image de VM ne doivent pas s'enregistrer elles-mêmes en tant que runners dans GitLab.

   - Dépendances requises par les jobs que vous prévoyez d'exécuter

## Configurer l'exécuteur pour la mise à l'échelle automatique {#configure-the-executor-to-autoscale}

Prérequis :

- Vous devez être administrateur.

Pour configurer l'exécuteur d'instance pour la mise à l'échelle automatique, mettez à jour les sections suivantes dans le `config.toml` :

- [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section)
- [`[runners.instance]`](../configuration/advanced-configuration.md#the-runnersinstance-section)

## Mode préemptif {#preemptive-mode}

Avec fleeting et taskscaler :

- Lorsqu'il est activé, le gestionnaire de runner ne demande pas de nouveaux jobs CI/CD tant que des instances inactives ne sont pas disponibles. Dans ce mode, les jobs CI/CD s'exécutent presque immédiatement.
- Si le mode préemptif est désactivé, le gestionnaire de runner demande de nouveaux jobs CI/CD, qu'il y ait ou non des instances inactives disponibles pour les exécuter. Le nombre de jobs est basé sur `max_instances` et `capacity_per_instance`. Dans ce mode, les temps de démarrage des jobs CI/CD sont plus lents. Il est possible que vous ne puissiez pas provisionner de nouvelles instances et que les jobs CI/CD ne s'exécutent pas.

## Exemples de configuration d'un groupe de mise à l'échelle automatique AWS {#aws-autoscaling-group-configuration-examples}

### Un job par instance {#one-job-per-instance}

Prérequis :

- Une AMI avec au moins `git` et GitLab Runner installés.
- Un groupe de mise à l'échelle automatique AWS. Pour la politique de mise à l'échelle, utilisez `none`. Le runner gère la mise à l'échelle.
- Une politique IAM avec les [autorisations correctes](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy).

Cette configuration prend en charge :

- Une capacité de `1` pour chaque instance.
- Un nombre d'utilisations de `1`.
- Une échelle d'inactivité de `5`.
- Un temps d'inactivité de 20 minutes.
- Un nombre maximum d'instances de `10`.

Lorsque la capacité et le nombre d'utilisations sont définis sur `1`, chaque job reçoit une instance éphémère sécurisée qui ne peut pas être affectée par d'autres jobs. Lorsque le job est terminé, l'instance sur laquelle il a été exécuté est immédiatement supprimée.

Lorsque la capacité pour chaque instance est `1` et que l'échelle d'inactivité est `5`, le runner maintient 5 instances entières disponibles pour les besoins futurs. Ces instances restent disponibles pendant au moins 20 minutes.

Le champ `concurrent` du runner est défini sur 10 (nombre maximum d'instances × capacité par instance).

```toml
concurrent = 10

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "aws" # in GitLab 16.11 and later, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # in GitLab 16.10 and earlier, manually install the plugin and use:
    # plugin = "fleeting-plugin-aws"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-linux-asg"                # AWS Autoscaling Group name
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

### Cinq jobs par instance avec des utilisations illimitées {#five-jobs-per-instance-with-unlimited-uses}

Prérequis :

- Une AMI avec au moins `git` et GitLab Runner installés.
- Un groupe de mise à l'échelle automatique AWS avec la politique de mise à l'échelle définie sur `none`. Le runner gère la mise à l'échelle.
- Une politique IAM avec les [autorisations correctes](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy).

Cette configuration prend en charge :

- Une capacité de `5` pour chaque instance.
- Un nombre d'utilisations illimité.
- Une échelle d'inactivité de `5`.
- Un temps d'inactivité de 20 minutes.
- Un nombre maximum d'instances de `10`.

Lorsque vous définissez la capacité par instance sur `5` avec un nombre d'utilisations illimité, chaque instance exécute simultanément cinq jobs pendant toute la durée de vie de l'instance.

Lorsque l'échelle d'inactivité est `5` et que la capacité d'inactivité de l'instance est `5`, une instance inactive est créée chaque fois que la capacité en cours d'utilisation tombe en dessous de cinq. Les instances inactives restent disponibles pendant au moins 20 minutes.

Les jobs exécutés dans ces environnements doivent être **trusted** car il existe peu d'isolation entre eux et chaque job peut affecter les performances d'un autre.

Le champ `concurrent` du runner est défini sur 50 (nombre maximum d'instances × capacité par instance).

```toml
concurrent = 50

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "aws" # in GitLab 16.11 and later, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # in GitLab 16.10 and earlier, manually install the plugin and use:
    # plugin = "fleeting-plugin-aws"

    capacity_per_instance = 5
    max_use_count = 0
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-windows-asg"              # AWS Autoscaling Group name
      profile          = "default"                     # optional, default is 'default'
      config_file      = "/home/user/.aws/config"      # optional, default is '~/.aws/config'
      credentials_file = "/home/user/.aws/credentials" # optional, default is '~/.aws/credentials'

    [runners.autoscaler.connector_config]
      username          = "Administrator"
      timeout           = "5m0s"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

### Deux jobs par instance, utilisations illimitées, virtualisation imbriquée sur des instances EC2 Mac {#two-jobs-per-instance-unlimited-uses-nested-virtualization-on-ec2-mac-instances}

Prérequis :

- Une AMI Apple Silicon avec [nesting](https://gitlab.com/gitlab-org/fleeting/nesting) et [Tart](https://github.com/cirruslabs/tart) installés.
- Les images de VM Tart que le runner utilise. Les images de VM sont spécifiées par le mot-clé `image` du job. Les images de VM doivent avoir au moins `git` et GitLab Runner installés.
- Un groupe de mise à l'échelle automatique AWS. Pour la politique de mise à l'échelle, utilisez `none`, car le runner gère la mise à l'échelle. Pour plus d'informations sur la configuration d'un ASG pour MacOS, consultez [Implementing autoscaling for EC2 Mac instances](https://aws.amazon.com/blogs/compute/implementing-autoscaling-for-ec2-mac-instances/).
- Une politique IAM avec les [autorisations correctes](https://gitlab.com/gitlab-org/fleeting/plugins/aws#recommended-iam-policy).

Cette configuration prend en charge :

- Une capacité de `2` pour chaque instance.
- Un nombre d'utilisations illimité.
- La virtualisation imbriquée pour prendre en charge les jobs isolés. La virtualisation imbriquée n'est disponible que pour les instances Apple Silicon avec [nesting](https://gitlab.com/gitlab-org/fleeting/nesting) installé.
- Une échelle d'inactivité de `5`.
- Un temps d'inactivité de 20 minutes.
- Un nombre maximum d'instances de `10`.

Lorsque la capacité pour chaque instance est `2` et que le nombre d'utilisations est illimité, chaque instance exécute simultanément 2 jobs pendant toute la durée de vie de l'instance.

Lorsque l'échelle d'inactivité est `2`, une instance inactive est créée chaque fois que la capacité en cours d'utilisation tombe en dessous de `2`. Les instances inactives restent disponibles pendant au moins 24 heures. Cette durée est due à la période d'allocation minimale de 24 heures des hôtes d'instances AWS MacOS.

Les jobs exécutés dans cet environnement n'ont pas besoin d'être approuvés car [nesting](https://gitlab.com/gitlab-org/fleeting/nesting) est utilisé pour la virtualisation imbriquée de chaque job. Cela ne fonctionne que sur les instances Apple Silicon.

Le champ `concurrent` du runner est défini sur 8 (nombre maximum d'instances × capacité par instance).

```toml
concurrent = 8

[[runners]]
  name = "macos applesilicon autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  executor = "instance"

  [runners.instance]
    allowed_images = ["*"] # allow any nesting image

  [runners.autoscaler]
    capacity_per_instance = 2 # AppleSilicon can only support 2 VMs per host
    max_use_count = 0
    max_instances = 4

    plugin = "aws" # in GitLab 16.11 and later, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # in GitLab 16.10 and earlier, manually install the plugin and use:
    # plugin = "fleeting-plugin-aws"

    [[runners.autoscaler.policy]]
      idle_count = 2
      idle_time  = "24h" # AWS's MacOS instances

    [runners.autoscaler.connector_config]
      username = "ec2-user"
      key_path = "macos-key.pem"
      timeout  = "1h" # connecting to a MacOS instance can take some time, as they can be slow to provision

    [runners.autoscaler.plugin_config]
      name = "mac2metal"
      region = "us-west-2"

    [runners.autoscaler.vm_isolation]
      enabled = true
      nesting_host = "unix:///Users/ec2-user/Library/Application Support/nesting.sock"

    [runners.autoscaler.vm_isolation.connector_config]
      username = "nested-vm-username"
      password = "nested-vm-password"
      timeout  = "20m"
```

## Exemples de configuration d'un groupe d'instances Google Cloud {#google-cloud-instance-group-configuration-examples}

### Un job par instance utilisant un groupe d'instances Google Cloud {#one-job-per-instance-using-a-google-cloud-instance-group}

Prérequis :

- Une image personnalisée avec au moins `git` et GitLab Runner installés.
- Un groupe d'instances Google Cloud où le mode de mise à l'échelle automatique est défini sur `do not autoscale`. Le runner gère la mise à l'échelle.
- Une politique IAM avec les [autorisations correctes](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud#required-permissions). Si vous déployez votre runner dans un cluster GKE, vous pouvez ajouter une liaison IAM entre le compte de service Kubernetes et le compte de service GCP. Vous pouvez ajouter cette liaison avec le rôle `iam.workloadIdentityUser` pour vous authentifier auprès de GCP au lieu d'utiliser un fichier de clé avec `credentials_file`.

Cette configuration prend en charge :

- Une capacité par instance de 1
- Un nombre d'utilisations de 1
- Une échelle d'inactivité de 5
- Un temps d'inactivité de 20 minutes
- Un nombre maximum d'instances de 10

Lorsque la capacité et le nombre d'utilisations sont tous deux définis sur `1`, chaque job reçoit une instance éphémère sécurisée qui ne peut pas être affectée par d'autres jobs. Lorsque le job est terminé, l'instance sur laquelle il a été exécuté est immédiatement supprimée.

Lorsque l'échelle d'inactivité est définie sur `5`, le runner maintient 5 instances disponibles pour les besoins futurs (car la capacité par instance est 1). Ces instances restent disponibles pendant au moins 20 minutes.

Le champ `concurrent` du runner est défini sur 10 (nombre maximum d'instances × capacité par instance).

```toml
concurrent = 10

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "googlecloud" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-googlecompute"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-linux-instance-group" # Google Cloud Instance Group name
      project          = "my-gcp-project"
      zone             = "europe-west1-c"
      credentials_file = "/home/user/.config/gcloud/application_default_credentials.json" # optional, default is '~/.config/gcloud/application_default_credentials.json'

    [runners.autoscaler.connector_config]
      username          = "runner"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

### Cinq jobs par instance, utilisations illimitées, avec un groupe d'instances Google Cloud {#five-jobs-per-instance-unlimited-uses-using-google-cloud-instance-group}

Prérequis :

- Une image personnalisée avec au moins `git` et GitLab Runner installés.
- Un groupe d'instances. Pour le « Mode de mise à l'échelle automatique », sélectionnez « do not autoscale », car le runner gère la mise à l'échelle.
- Une politique IAM avec les [autorisations correctes](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud#required-permissions).

Cette configuration prend en charge :

- Une capacité par instance de 5
- Un nombre d'utilisations illimité
- Une échelle d'inactivité de 5
- Un temps d'inactivité de 20 minutes
- Un nombre maximum d'instances de 10

Lorsque la capacité est définie sur `5` et que le nombre d'utilisations est illimité, chaque instance exécute simultanément 5 jobs pendant toute la durée de vie de l'instance.

Les jobs exécutés dans ces environnements doivent être **trusted** car il existe peu d'isolation entre eux et chaque job peut affecter les performances d'un autre.

Lorsque l'échelle d'inactivité est `5`, une instance inactive est créée chaque fois que la capacité en cours d'utilisation tombe en dessous de `5`. Les instances inactives restent disponibles pendant au moins 20 minutes.

Le champ `concurrent` du runner est défini sur 50 (nombre maximum d'instances × capacité par instance).

```toml
concurrent = 50

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "googlecloud" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-googlecompute"

    capacity_per_instance = 5
    max_use_count = 0
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name             = "my-windows-instance-group" # Google Cloud Instance Group name
      project          = "my-gcp-project"
      zone             = "europe-west1-c"
      credentials_file = "/home/user/.config/gcloud/application_default_credentials.json" # optional, default is '~/.config/gcloud/application_default_credentials.json'

    [runners.autoscaler.connector_config]
      username          = "Administrator"
      timeout           = "5m0s"
      use_external_addr = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

## Exemples de configuration d'un groupe de machines virtuelles Azure {#azure-scale-set-configuration-examples}

### Un job par instance utilisant un groupe de machines virtuelles Azure {#one-job-per-instance-using-an-azure-scale-set}

Prérequis :

- Une image personnalisée avec au moins `git` et GitLab Runner installés.
- Un groupe de machines virtuelles Azure où le mode de mise à l'échelle automatique est défini sur `manual` et le sur-approvisionnement est désactivé. Le runner gère la mise à l'échelle.

Cette configuration prend en charge :

- Une capacité par instance de 1
- Un nombre d'utilisations de 1
- Une échelle d'inactivité de 5
- Un temps d'inactivité de 20 minutes
- Un nombre maximum d'instances de 10

Lorsque la capacité et le nombre d'utilisations sont tous deux définis sur `1`, chaque job reçoit une instance éphémère sécurisée qui ne peut pas être affectée par d'autres jobs. Lorsque le job est terminé, l'instance sur laquelle il a été exécuté est immédiatement supprimée.

Lorsque l'échelle d'inactivité est définie sur `5`, le runner maintient 5 instances disponibles pour les besoins futurs (car la capacité par instance est 1). Ces instances restent disponibles pendant au moins 20 minutes.

Le champ `concurrent` du runner est défini sur 10 (nombre maximum d'instances × capacité par instance).

```toml
concurrent = 10

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "azure" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-azure"

    capacity_per_instance = 1
    max_use_count = 1
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name                = "my-linux-scale-set" # Azure scale set name
      subscription_id     = "9b3c4602-cde2-4089-bed8-889e5a3e7102"
      resource_group_name = "my-resource-group"

    [runners.autoscaler.connector_config]
      username               = "runner"
      password               = "my-scale-set-static-password"
      use_static_credentials = true
      timeout                = "10m"
      use_external_addr      = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time  = "20m0s"
```

### Cinq jobs par instance, utilisations illimitées, avec un groupe de machines virtuelles Azure {#five-jobs-per-instance-unlimited-uses-using-an-azure-scale-set}

Prérequis :

- Une image personnalisée avec au moins `git` et GitLab Runner installés.
- Un groupe de machines virtuelles Azure où le mode de mise à l'échelle automatique est défini sur `manual` et le sur-approvisionnement est désactivé. Le runner gère la mise à l'échelle.

Cette configuration prend en charge :

- Une capacité par instance de 5
- Un nombre d'utilisations illimité
- Une échelle d'inactivité de 5
- Un temps d'inactivité de 20 minutes
- Un nombre maximum d'instances de 10

Lorsque la capacité est définie sur `5` et que le nombre d'utilisations est illimité, chaque instance exécute simultanément 5 jobs pendant toute la durée de vie de l'instance.

Les jobs exécutés dans ces environnements doivent être **trusted** car il existe peu d'isolation entre eux et chaque job peut affecter les performances d'un autre.

Lorsque l'échelle d'inactivité est `2`, une instance inactive est créée chaque fois que la capacité en cours d'utilisation tombe en dessous de `5`. Les instances inactives restent disponibles pendant au moins 20 minutes.

Le champ `concurrent` du runner est défini sur 50 (nombre maximum d'instances × capacité par instance).

```toml
concurrent = 50

[[runners]]
  name = "instance autoscaler example"
  url = "https://gitlab.com"
  token = "<token>"
  shell = "sh"

  executor = "instance"

  # Autoscaler config
  [runners.autoscaler]
    plugin = "azure" # for >= 16.11, ensure you run `gitlab-runner fleeting install` to automatically install the plugin

    # for versions < 17.0, manually install the plugin and use:
    # plugin = "fleeting-plugin-azure"

    capacity_per_instance = 5
    max_use_count = 0
    max_instances = 10

    [runners.autoscaler.plugin_config] # plugin specific configuration (see plugin documentation)
      name                = "my-windows-scale-set" # Azure scale set name
      subscription_id     = "9b3c4602-cde2-4089-bed8-889e5a3e7102"
      resource_group_name = "my-resource-group"

    [runners.autoscaler.connector_config]
      username               = "Administrator"
      password               = "my-scale-set-static-password"
      use_static_credentials = true
      timeout                = "10m"
      use_external_addr      = true

    [[runners.autoscaler.policy]]
      idle_count = 5
      idle_time = "20m0s"
```

## Prise en charge des cgroups par emplacement {#slot-based-cgroup-support}

L'exécuteur d'instance prend en charge les cgroups par emplacement pour une meilleure isolation des ressources entre les jobs simultanés. Lorsqu'elle est activée, la variable d'environnement `GITLAB_RUNNER_SLOT_CGROUP` est automatiquement fournie aux jobs, ce qui vous permet d'exécuter des processus sous des cgroups spécifiques à l'emplacement.

Pour des informations détaillées sur les cgroups par emplacement, notamment les avantages, les prérequis, la configuration et les instructions d'installation, consultez [Prise en charge des cgroups par emplacement](../configuration/slot_based_cgroups.md).

### Utilisation de la variable d'environnement de cgroup d'emplacement de GitLab Runner {#using-the-gitlab-runner-slot-cgroup-environment-variable}

L'exécuteur d'instance fournit la variable d'environnement `GITLAB_RUNNER_SLOT_CGROUP` à vos jobs. Utilisez cette variable avec des outils tels que `systemd-run` ou `cgexec` pour exécuter des processus sous le cgroup spécifique à l'emplacement.

Pour des exemples d'utilisation et le dépannage, consultez la [section Exécuteur d'instance](../configuration/slot_based_cgroups.md#instance-executor) dans la documentation sur les cgroups par emplacement.

## Dépannage {#troubleshooting}

Lorsque vous travaillez avec l'exécuteur d'instance, vous pouvez rencontrer les problèmes suivants :

### `sh: 1: eval: Running on ip-x.x.x.x via runner-host...n: not found` {#sh-1-eval-running-on-ip-xxxx-via-runner-hostn-not-found}

Cette erreur se produit généralement lorsque la commande `eval` dans l'étape de préparation échoue. Pour résoudre cette erreur, passez au shell `bash` et activez le feature flag [feature flag](../configuration/feature-flags.md) `FF_USE_NEW_BASH_EVAL_STRATEGY`.
