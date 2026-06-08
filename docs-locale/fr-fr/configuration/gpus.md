---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Utilisation des unités de traitement graphique (GPU)
---

{{< details >}}

- Niveau : Free, Premium, Ultimate
- Offre : GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

{{< history >}}

- Introduit dans GitLab Runner 13.9.

{{< /history >}}

GitLab Runner prend en charge l'utilisation des unités de traitement graphique (GPU). La section suivante décrit la configuration requise pour activer les GPU pour différents exécuteurs.

## Exécuteur Shell {#shell-executor}

Aucune configuration de runner n'est nécessaire.

## Exécuteur Docker {#docker-executor}

> [!warning]
> Si vous utilisez Podman comme moteur d'exécution de conteneur, les GPU ne sont pas détectés. Pour plus d'informations, consultez [le ticket 39095](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/39095).

Prérequis :

- Installez [NVIDIA Driver](https://docs.nvidia.com/datacenter/tesla/driver-installation-guide/index.html).
- Installez [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html).

Utilisez l'option de configuration `gpus` ou `service_gpus` dans la [section `runners.docker`](advanced-configuration.md#the-runnersdocker-section) :

```toml
[runners.docker]
    gpus = "all"
    service_gpus = "all"
```

## Exécuteur Docker Machine {#docker-machine-executor}

Consultez la [documentation pour le fork GitLab de Docker Machine](../executors/docker_machine.md#using-gpus-on-google-compute-engine).

## Exécuteur Kubernetes {#kubernetes-executor}

Prérequis :

- Assurez-vous que [le sélecteur de nœud choisit un nœud avec prise en charge des GPU](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/).
- Activez le feature flag `FF_USE_ADVANCED_POD_SPEC_CONFIGURATION`.

Pour activer la prise en charge des GPU, configurez le runner afin qu'il demande des ressources GPU dans la spécification du pod. Par exemple :

```toml
[[runners.kubernetes.pod_spec]]
  name = "gpu"
  patch = '''
    containers:
    - name: build
      resources:
        requests:
          nvidia.com/gpu: 1
        limits:
          nvidia.com/gpu: 1
  '''
  patch_type = "strategic" # <--- `strategic` patch_type
```

Ajustez le nombre de GPU dans `requests` et `limits` en fonction des exigences de votre job.

GitLab Runner a été [testé sur Amazon Elastic Kubernetes Service](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4355) avec des [instances compatibles GPU](https://docs.aws.amazon.com/dlami/latest/devguide/gpu.html).

## Vérifier que les GPU sont activés {#validate-that-gpus-are-enabled}

Vous pouvez utiliser des runners avec des GPU NVIDIA. Pour les GPU NVIDIA, l'une des façons de s'assurer qu'un GPU est activé pour un job CI est d'exécuter `nvidia-smi` au début du script. Par exemple :

```yaml
train:
  script:
    - nvidia-smi
```

Si les GPU sont activés, la sortie de `nvidia-smi` affiche les appareils disponibles. Dans l'exemple suivant, un seul NVIDIA Tesla P4 est activé :

```shell
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

Si le matériel ne prend pas en charge un GPU, `nvidia-smi` devrait échouer, soit parce qu'il est manquant, soit parce qu'il ne peut pas communiquer avec le pilote :

```shell
modprobe: ERROR: could not insert 'nvidia': No such device
NVIDIA-SMI has failed because it couldn't communicate with the NVIDIA driver. Make sure that the latest NVIDIA driver is installed and running.
```
