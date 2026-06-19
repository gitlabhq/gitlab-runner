---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "Installer et enregistrer GitLab Runner pour la mise à l'échelle automatique avec Docker Machine"
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!warning]
> L'exécuteur Docker Machine a été déprécié dans GitLab 17.5 et est prévu pour suppression dans GitLab 20.0 (mai 2027). Bien que nous continuions à prendre en charge l'exécuteur Docker Machine jusqu'à GitLab 20.0, nous ne prévoyons pas d'ajouter de nouvelles fonctionnalités. Nous ne traiterons que les bogues critiques susceptibles d'empêcher l'exécution des jobs CI/CD ou d'affecter les coûts d'exécution. Si vous utilisez l'exécuteur Docker Machine sur Amazon Web Services (AWS) EC2, Microsoft Azure Compute ou Google Compute Engine (GCE), vous devriez migrer vers le [GitLab Runner Autoscaler](../runner_autoscale/_index.md).

L'exécuteur Docker Machine est une version spéciale de l'exécuteur Docker avec prise en charge de la mise à l'échelle automatique. Il fonctionne comme l'exécuteur Docker classique, mais les hôtes de build sont créés à la demande par Docker Machine. Cela le rend efficace dans les environnements cloud tels qu'AWS EC2, où il offre une bonne isolation et une bonne évolutivité pour les charges de travail variables.

Pour une vue d'ensemble de l'architecture de mise à l'échelle automatique, consultez la [documentation complète sur la mise à l'échelle automatique](../configuration/autoscale.md).

## Version dupliquée de Docker Machine {#forked-version-of-docker-machine}

Docker a [déprécié Docker Machine](https://gitlab.com/gitlab-org/gitlab/-/issues/341856). Cependant, GitLab maintient une [duplication de Docker Machine](https://gitlab.com/gitlab-org/ci-cd/docker-machine) pour les utilisateurs de GitLab Runner qui dépendent de l'exécuteur Docker Machine. Cette duplication est basée sur la dernière branche `main` de `docker-machine` avec quelques correctifs supplémentaires pour les bogues suivants :

- [Rendre le pilote DigitalOcean compatible avec RateLimit](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/2)
- [Ajouter un backoff à la vérification des opérations du pilote Google](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/7)
- [Ajouter l'option `--google-min-cpu-platform` pour la création de machines](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/9)
- [Utiliser l'IP en cache pour le pilote Google](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/15)
- [Utiliser l'IP en cache pour le pilote AWS](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/14)
- [Ajouter la prise en charge des GPU dans Google Compute Engine](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/48)
- [Prendre en charge l'exécution d'instances AWS avec IMDSv2](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/merge_requests/49)

L'objectif de la [duplication de Docker Machine](https://gitlab.com/gitlab-org/ci-cd/docker-machine) est de corriger uniquement les problèmes critiques et les bogues qui affectent les coûts d'exécution. Nous ne prévoyons pas d'ajouter de nouvelles fonctionnalités.

## Préparer l'environnement {#preparing-the-environment}

Pour utiliser la fonctionnalité de mise à l'échelle automatique, Docker et GitLab Runner doivent être installés sur la même machine :

1. Connectez-vous à une nouvelle machine Linux qui peut fonctionner comme serveur bastion où Docker crée de nouvelles machines.
1. [Installez GitLab Runner](../install/_index.md).
1. Installez Docker Machine depuis la [duplication de Docker Machine](https://gitlab.com/gitlab-org/ci-cd/docker-machine).
1. En option mais recommandé, préparez un [registre de conteneurs proxy et un serveur de cache](../configuration/speed_up_job_execution.md) à utiliser avec les runners à mise à l'échelle automatique.

## Configurer GitLab Runner {#configuring-gitlab-runner}

1. Familiarisez-vous avec les concepts fondamentaux de l'utilisation de `docker-machine` avec `gitlab-runner` :
   - Lisez [GitLab Runner Autoscaling](../configuration/autoscale.md)
   - Lisez [GitLab Runner MachineOptions](../configuration/advanced-configuration.md#the-runnersmachine-section)
1. La **première fois** que vous utilisez Docker Machine, il est préférable d'exécuter manuellement la commande `docker-machine create ...` avec votre [pilote Docker Machine](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/tree/main/drivers). Exécutez cette commande avec les options que vous avez l'intention de configurer dans [MachineOptions](../configuration/advanced-configuration.md#the-runnersmachine-section) sous la section `[runners.machine]`. Cette approche configure correctement l'environnement Docker Machine et valide les options spécifiées. Après cela, vous pouvez détruire la machine avec `docker-machine rm [machine_name]` et démarrer le runner.

   > [!note]
   > Les requêtes concurrentes multiples vers `docker-machine create` effectuées **lors de la première utilisation** ne sont pas recommandées. Lorsque l'exécuteur `docker+machine` est utilisé, le runner peut lancer plusieurs commandes `docker-machine create` simultanées. Si Docker Machine est nouveau dans cet environnement, chaque processus tente de créer des clés SSH et des certificats SSL pour l'authentification de l'API Docker. Cette action entraîne des interférences entre les processus concurrents. Cela peut aboutir à un environnement non fonctionnel. C'est pourquoi il est important de créer une machine de test manuellement la toute première fois que vous configurez GitLab Runner avec Docker Machine.

   1. [Enregistrez un runner](../register/_index.md) et sélectionnez l'exécuteur `docker+machine` lorsque vous y êtes invité.
   1. Modifiez [`config.toml`](../commands/_index.md#configuration-file) et configurez le runner pour utiliser Docker Machine. Consultez la page dédiée contenant des informations détaillées sur la [mise à l'échelle automatique GitLab Runner](../configuration/autoscale.md).
   1. Vous pouvez maintenant essayer de démarrer un nouveau pipeline dans votre projet. En quelques secondes, si vous exécutez `docker-machine ls`, vous devriez voir une nouvelle machine en cours de création.

## Mettre à niveau GitLab Runner {#upgrading-gitlab-runner}

1. Vérifiez si votre système d'exploitation est configuré pour redémarrer automatiquement GitLab Runner (par exemple, en vérifiant son fichier de service) :
   - **Si oui**, assurez-vous que le gestionnaire de service est [configuré pour utiliser `SIGQUIT`](../configuration/init.md) et utilisez les outils du service pour arrêter le processus :

     ```shell
     # For systemd
     sudo systemctl stop gitlab-runner

     # For upstart
     sudo service gitlab-runner stop
     ```

   - **Si non**, vous pouvez arrêter le processus manuellement :

     ```shell
     sudo killall -SIGQUIT gitlab-runner
     ```

   L'envoi du [signal `SIGQUIT`](../commands/_index.md#signals) entraîne l'arrêt gracieux du processus. Le processus cesse d'accepter de nouveaux jobs et se termine dès que les jobs en cours sont terminés.

1. Attendez que GitLab Runner se termine. Vous pouvez vérifier son statut avec `gitlab-runner status` ou attendre un arrêt gracieux pendant jusqu'à 30 minutes avec :

   ```shell
   for i in `seq 1 180`; do # 1800 seconds = 30 minutes
       gitlab-runner status || break
       sleep 10
   done
   ```

1. Vous pouvez maintenant installer en toute sécurité la nouvelle version de GitLab Runner sans interrompre aucun job.

## Utilisation de la version dupliquée de Docker Machine {#using-the-forked-version-of-docker-machine}

### Installer {#install}

1. Téléchargez le [binaire `docker-machine` approprié](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/releases). Copiez le binaire vers un emplacement accessible au `PATH` et rendez-le exécutable. Par exemple, pour télécharger et installer `v0.16.2-gitlab.46` :

   ```shell
   curl -O "https://gitlab-docker-machine-downloads.s3.amazonaws.com/v0.16.2-gitlab.46/docker-machine-Linux-x86_64"
   cp docker-machine-Linux-x86_64 /usr/local/bin/docker-machine
   chmod +x /usr/local/bin/docker-machine
   ```

### Utilisation des GPU sur Google Compute Engine {#using-gpus-on-google-compute-engine}

> [!note]
> Les GPU sont [pris en charge sur chaque exécuteur](../configuration/gpus.md). Il n'est pas nécessaire d'utiliser Docker Machine uniquement pour la prise en charge des GPU. L'exécuteur Docker Machine fait évoluer les nœuds GPU à la hausse et à la baisse. Vous pouvez également utiliser l'[exécuteur Kubernetes](kubernetes/_index.md) à cette fin.

Vous pouvez utiliser la [duplication](#forked-version-of-docker-machine) de Docker Machine pour créer des [instances Google Compute Engine avec des unités de traitement graphique (GPU)](https://docs.cloud.google.com/compute/docs/gpus).

#### Options GPU de Docker Machine {#docker-machine-gpu-options}

Pour créer une instance avec des GPU, utilisez ces options Docker Machine :

| Option                        | Exemple                        | Description |
|-------------------------------|--------------------------------|-------------|
| `--google-accelerator`        | `type=nvidia-tesla-p4,count=1` | Spécifie le type et le nombre d'accélérateurs GPU à associer à l'instance (format `type=TYPE,count=N`) |
| `--google-maintenance-policy` | `TERMINATE`                    | Utilisez toujours `TERMINATE` car [Google Cloud ne permet pas la migration en direct des instances GPU](https://docs.cloud.google.com/compute/docs/instances/live-migration-process). |
| `--google-machine-image`      | `https://www.googleapis.com/compute/v1/projects/deeplearning-platform-release/global/images/family/tf2-ent-2-3-cu110` | L'URL d'un système d'exploitation compatible GPU. Consultez la [liste des images disponibles](https://docs.cloud.google.com/deep-learning-vm/docs/images). |
| `--google-metadata`           | `install-nvidia-driver=True`   | Cet indicateur indique à l'image d'installer le pilote GPU NVIDIA. |

Ces arguments correspondent aux [arguments de ligne de commande pour `gcloud compute`](https://docs.cloud.google.com/compute/docs/gcloud-compute). Consultez la [documentation Google sur la création de VM avec des GPU associés](https://docs.cloud.google.com/compute/docs/gpus/create-vm-with-gpus) pour plus de détails.

#### Vérification des options Docker Machine {#verifying-docker-machine-options}

Pour préparer votre système et tester que des GPU peuvent être créés avec Google Compute Engine :

1. [Configurez les identifiants du pilote Google Compute Engine](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/blob/main/docs/drivers/gce.md#credentials) pour Docker Machine. Vous devrez peut-être exporter des variables d'environnement vers le runner si votre VM ne dispose pas d'un compte de service par défaut. La façon de procéder dépend de la manière dont le runner est lancé. Par exemple, en utilisant :

   - `systemd` ou `upstart` :  Consultez la [documentation sur la définition de variables d'environnement personnalisées](../configuration/init.md#setting-custom-environment-variables).
   - Kubernetes avec le chart Helm :  Mettez à jour [l'entrée `values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/-/blob/5e7c5c0d6e1159647d65f04ff2cc1f45bb2d5efc/values.yaml#L431-438).
   - Docker :  Utilisez l'option `-e` (par exemple, `docker run -e GOOGLE_APPLICATION_CREDENTIALS=/path/to/credentials.json gitlab/gitlab-runner`).

1. Vérifiez que `docker-machine` peut créer une machine virtuelle avec les options souhaitées. Par exemple, pour créer une machine `n1-standard-1` avec un seul accélérateur NVIDIA Tesla P4, remplacez `test-gpu` par un nom et exécutez :

   ```shell
   docker-machine create --driver google --google-project your-google-project \
     --google-disk-size 50 \
     --google-machine-type n1-standard-1 \
     --google-accelerator type=nvidia-tesla-p4,count=1 \
     --google-maintenance-policy TERMINATE \
     --google-machine-image https://www.googleapis.com/compute/v1/projects/deeplearning-platform-release/global/images/family/tf2-ent-2-3-cu110 \
     --google-metadata "install-nvidia-driver=True" test-gpu
   ```

1. Pour vérifier que le GPU est actif, connectez-vous en SSH à la machine et exécutez `nvidia-smi` :

   ```shell
   $ docker-machine ssh test-gpu sudo nvidia-smi
   +-----------------------------------------------------------------------------+
   | NVIDIA-SMI 450.51.06    Driver Version: 450.51.06    CUDA Version: 11.0     |
   |-------------------------------+----------------------+----------------------+
   | GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
   | Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
   |                               |                      |               MIG M. |
   |===============================+======================+======================|
   |   0  Tesla P4            Off  | 00000000:00:04.0 Off |                    0 |
   | N/A   43C    P0    22W /  75W |      0MiB /  7611MiB |      3%      Default |
   |                               |                      |                  N/A |
   +-------------------------------+----------------------+----------------------+

   +-----------------------------------------------------------------------------+
   | Processes:                                                                  |
   |  GPU   GI   CI        PID   Type   Process name                  GPU Memory |
   |        ID   ID                                                   Usage      |
   |=============================================================================|
   |  No running processes found                                                 |
   +-----------------------------------------------------------------------------+
   ```

1. Supprimez cette instance de test pour économiser de l'argent :

   ```shell
   docker-machine rm test-gpu
   ```

#### Configurer GitLab Runner {#configuring-gitlab-runner-1}

1. Après avoir vérifié ces options, configurez l'exécuteur Docker pour utiliser tous les GPU disponibles dans la [configuration `runners.docker`](../configuration/advanced-configuration.md#the-runnersdocker-section). Ensuite, ajoutez les options Docker Machine à vos [paramètres `MachineOptions` dans la configuration `runners.machine` de GitLab Runner](../configuration/advanced-configuration.md#the-runnersmachine-section). Par exemple :

   ```toml
   [runners.docker]
     gpus = "all"
   [runners.machine]
     MachineOptions = [
       "google-project=your-google-project",
       "google-disk-size=50",
       "google-disk-type=pd-ssd",
       "google-machine-type=n1-standard-1",
       "google-accelerator=count=1,type=nvidia-tesla-p4",
       "google-maintenance-policy=TERMINATE",
       "google-machine-image=https://www.googleapis.com/compute/v1/projects/deeplearning-platform-release/global/images/family/tf2-ent-2-3-cu110",
       "google-metadata=install-nvidia-driver=True"
     ]
   ```

## Dépannage {#troubleshooting}

Lorsque vous utilisez l'exécuteur Docker Machine, vous pouvez rencontrer les problèmes suivants.

### Erreur :  Erreur lors de la création de la machine {#error-error-creating-machine}

Lors de l'installation de Docker Machine, vous pouvez rencontrer une erreur indiquant `ERROR: Error creating machine: Error running provisioning: error installing docker`.

Docker Machine tente d'installer Docker sur une machine virtuelle nouvellement provisionnée à l'aide de ce script :

```shell
if ! type docker; then curl -sSL "https://get.docker.com" | sh -; fi
```

Si la commande `docker` réussit, Docker Machine suppose que Docker est installé et continue.

Si elle ne réussit pas, Docker Machine tente de télécharger et d'exécuter le script à l'adresse `https://get.docker.com`. Si l'installation échoue, il est possible que le système d'exploitation ne soit plus pris en charge par Docker.

Pour résoudre ce problème, vous pouvez activer le débogage sur Docker Machine en définissant `MACHINE_DEBUG=true` dans l'environnement où GitLab Runner est installé.

### Erreur :  Impossible de se connecter au daemon Docker {#error-cannot-connect-to-the-docker-daemon}

Le job peut échouer pendant l'étape de préparation avec un message d'erreur :

```plaintext
Preparing environment
ERROR: Job failed (system failure): prepare environment: Cannot connect to the Docker daemon at tcp://10.200.142.223:2376. Is the docker daemon running? (docker.go:650:120s). Check https://docs.gitlab.com/runner/shells/#shell-profile-loading for more information
```

Cette erreur se produit lorsque le daemon Docker ne parvient pas à démarrer dans le délai prévu dans la VM créée par l'exécuteur Docker Machine. Pour résoudre ce problème, augmentez la valeur de `wait_for_services_timeout` dans la section [`[runners.docker]`](../configuration/advanced-configuration.md#the-runnersdocker-section).
