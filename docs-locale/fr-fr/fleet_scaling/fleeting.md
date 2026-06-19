---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Fleeting
---

[Fleeting](https://gitlab.com/gitlab-org/fleeting/fleeting) est une bibliothèque que GitLab Runner utilise pour fournir une abstraction basée sur des plugins pour les groupes d'instances d'un fournisseur cloud.

Les exécuteurs suivants utilisent fleeting pour mettre à l'échelle les runners :

- [Docker Autoscaler](../executors/docker_autoscaler.md)
- [Instance](../executors/instance.md)

## Trouver un plugin fleeting {#find-a-fleeting-plugin}

GitLab maintient ces plugins officiels :

| Fournisseur cloud                                                             | Remarques |
|----------------------------------------------------------------------------|-------|
| [Google Cloud](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud) | Utilise les [groupes d'instances Google Cloud](https://docs.cloud.google.com/compute/docs/instance-groups) |
| [AWS](https://gitlab.com/gitlab-org/fleeting/plugins/aws)                  | Utilise les [groupes AWS Auto Scaling](https://docs.aws.amazon.com/autoscaling/ec2/userguide/auto-scaling-groups.html) |
| [Azure](https://gitlab.com/gitlab-org/fleeting/plugins/azure)              | Utilise les [Virtual Machine Scale Sets](https://learn.microsoft.com/en-us/azure/virtual-machine-scale-sets/overview) Azure. Seul le mode [Uniform orchestration](https://learn.microsoft.com/en-us/azure/virtual-machine-scale-sets/virtual-machine-scale-sets-orchestration-modes#scale-sets-with-uniform-orchestration) est pris en charge. |

Les plugins suivants sont maintenus par la communauté :

| Fournisseur cloud | Référence OCI | Remarques |
|----------------|---------------|-------|
| [VMware vSphere](https://gitlab.com/santhanuv/fleeting-plugin-vmware-vsphere) | `registry.gitlab.com/santhanuv/fleeting-plugin-vmware-vsphere:latest` | Utilise VMware vSphere pour créer et gérer des machines virtuelles en les clonant à partir d'un modèle existant. Testé avec le simulateur [`govmomi vcsim`](https://github.com/vmware/govmomi/tree/main/vcsim) et validé par des membres de la communauté pour des cas d'utilisation de base. Il peut avoir des limitations avec des permissions vSphere restreintes. Vous pouvez créer des tickets liés dans le [projet Fleeting Plugin VMware vSphere](https://gitlab.com/santhanuv/fleeting-plugin-vmware-vsphere/-/issues).|

Les plugins maintenus par la communauté sont détenus, construits, hébergés et maintenus par des contributeurs extérieurs à GitLab (la communauté). GitLab détient et maintient la bibliothèque Fleeting et l'API pour fournir une revue de code statique. GitLab ne peut pas tester les plugins de la communauté car nous n'avons pas accès à tous les environnements informatiques nécessaires. Les membres de la communauté doivent créer, tester et publier des plugins dans un dépôt OCI et fournir la référence sur cette page via des merge requests. La référence OCI doit être accompagnée de remarques sur l'endroit où signaler les problèmes, le niveau de support et de stabilité du plugin, et où trouver la documentation.

## Configurer un plugin fleeting {#configure-a-fleeting-plugin}

Pour configurer fleeting, dans le fichier `config.toml`, utilisez la section de configuration [`[runners.autoscaler]`](../configuration/advanced-configuration.md#the-runnersautoscaler-section).

> [!note]
> Le fichier README.md de chaque plugin contient des informations importantes concernant l'installation et la configuration.

## Installer un plugin fleeting {#install-a-fleeting-plugin}

Pour installer un plugin fleeting, utilisez l'une des méthodes suivantes :

- Distribution via le registre OCI (recommandée)
- Installation manuelle du binaire

## Installer via la distribution du registre OCI {#install-with-the-oci-registry-distribution}

{{< history >}}

- Distribution du registre OCI [introduite](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4690) dans GitLab Runner 16.11

{{< /history >}}

Les plugins sont installés dans `~/.config/fleeting/plugins` sur les systèmes UNIX, et dans `%APPDATA%/fleeting/plugins` sur Windows. Pour remplacer l'emplacement d'installation des plugins, mettez à jour la variable d'environnement `FLEETING_PLUGIN_PATH`.

Pour installer le plugin fleeting :

1. Dans le fichier `config.toml`, dans la section `[runners.autoscaler]`, ajoutez le plugin fleeting :

   {{< tabs >}}

   {{< tab title="AWS" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "aws:latest"
   ```

   {{< /tab >}}

   {{< tab title="Google Cloud" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "googlecloud:latest"
   ```

   {{< /tab >}}

   {{< tab title="Azure" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "azure:latest"
   ```

   {{< /tab >}}

   {{< /tabs >}}

1. Exécutez `gitlab-runner fleeting install`.

### Formats de `plugin` {#plugin-formats}

Le paramètre `plugin` prend en charge les formats suivants :

- `<name>`
- `<name>:<version constraint>`
- `<repository>/<name>`
- `<repository>/<name>:<version constraint>`
- `<registry>/<repository>/<name>`
- `<registry>/<repository>/<name>:<version constraint>`

Où :

- `registry.gitlab.com` est le registre par défaut.
- `gitlab-org/fleeting/plugins` est le dépôt par défaut.
- `latest` est la version par défaut.

### Formats de contrainte de version {#version-constraint-formats}

La commande `gitlab-runner fleeting install` utilise la contrainte de version pour trouver la dernière version correspondante dans le dépôt distant.

Lorsque GitLab Runner s'exécute, il utilise la contrainte de version pour trouver la dernière version correspondante installée localement.

Utilisez les formats de contrainte de version suivants :

| Format                    | Description |
|---------------------------|-------------|
| `latest`                  | Dernière version. |
| `<MAJOR>`                 | Sélectionne la version majeure. Par exemple, `1` sélectionne la version correspondant à `1.*.*`. |
| `<MAJOR>.<MINOR>`         | Sélectionne la version majeure et mineure. Par exemple, `1.5` sélectionne la dernière version correspondant à `1.5.*`. |
| `<MAJOR>.<MINOR>.<PATCH>` | Sélectionne la version majeure, mineure et le correctif. Par exemple, `1.5.1` sélectionne la version `1.5.1`. |

## Installer le binaire manuellement {#install-binary-manually}

Pour installer manuellement un plugin fleeting :

1. Téléchargez le binaire du plugin fleeting pour votre système :
   - [AWS](https://gitlab.com/gitlab-org/fleeting/plugins/aws/-/releases)
   - [Google Cloud](https://gitlab.com/gitlab-org/fleeting/plugins/googlecloud/-/releases)
   - [Azure](https://gitlab.com/gitlab-org/fleeting/plugins/azure/-/releases)
1. Assurez-vous que le binaire porte un nom au format `fleeting-plugin-<name>`. Par exemple, `fleeting-plugin-aws`.
1. Assurez-vous que le binaire est accessible depuis `$PATH`. Par exemple, déplacez-le vers `/usr/local/bin`.
1. Dans le fichier `config.toml`, dans la section `[runners.autoscaler]`, ajoutez le plugin fleeting. Par exemple :

   {{< tabs >}}

   {{< tab title="AWS" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "fleeting-plugin-aws"
   ```

   {{< /tab >}}

   {{< tab title="Google Cloud" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "fleeting-plugin-googlecloud"
   ```

   {{< /tab >}}

   {{< tab title="Azure" >}}

   ```toml
   [[runners]]
     name = "my runner"
     url = "https://gitlab.com"
     token = "<token>"
     shell = "sh"

   executor = "instance"

   [runners.autoscaler]
     plugin = "fleeting-plugin-azure"
   ```

   {{< /tab >}}

   {{< /tabs >}}

## Gestion des plugins fleeting {#fleeting-plugin-management}

Utilisez les sous-commandes `fleeting` suivantes pour gérer les plugins fleeting :

| Commande                          | Description |
|----------------------------------|-------------|
| `gitlab-runner fleeting install` | Installe le plugin fleeting depuis la distribution du registre OCI. |
| `gitlab-runner fleeting list`    | Liste les plugins référencés et la version utilisée. |
| `gitlab-runner fleeting login`   | Se connecter aux registres privés. |
