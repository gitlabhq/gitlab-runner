---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "Dépannage de l'exécuteur Kubernetes"
---

Les erreurs suivantes sont fréquemment rencontrées lors de l'utilisation de l'exécuteur Kubernetes.

## `Job failed (system failure): timed out waiting for pod to start` {#job-failed-system-failure-timed-out-waiting-for-pod-to-start}

Si le cluster ne peut pas planifier le pod de build avant le délai d'expiration défini par `poll_timeout`, le pod de build renvoie une erreur. Le [Kubernetes Scheduler](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-lifetime) devrait pouvoir le supprimer.

Pour résoudre ce problème, augmentez la valeur de `poll_timeout` dans votre fichier `config.toml`.

## `context deadline exceeded` {#context-deadline-exceeded}

Les erreurs `context deadline exceeded` dans les job logs indiquent généralement que le client de l'API Kubernetes a dépassé le délai d'attente pour une requête donnée à l'API du cluster.

Vérifiez les [métriques du composant de cluster `kube-apiserver`](https://kubernetes.io/docs/concepts/cluster-administration/system-metrics/) pour détecter tout signe de :

- Augmentation des latences de réponse.
- Taux d'erreurs pour les opérations courantes de création ou de suppression sur les pods, les secrets, les ConfigMaps et d'autres ressources principales (v1).

Les logs d'erreurs liées aux délais d'expiration provenant des opérations `kube-apiserver` peuvent apparaître comme suit :

```plaintext
Job failed (system failure): prepare environment: context deadline exceeded
Job failed (system failure): prepare environment: setting up build pod: context deadline exceeded
```

Dans certains cas, la réponse d'erreur de `kube-apiserver` peut fournir des détails supplémentaires sur les défaillances de ses sous-composants (comme le `etcdserver` du cluster Kubernetes) :

```plaintext
Job failed (system failure): prepare environment: etcdserver: request timed out
Job failed (system failure): prepare environment: etcdserver: leader changed
Job failed (system failure): prepare environment: Internal error occurred: resource quota evaluates timeout
```

Ces défaillances du service `kube-apiserver` peuvent survenir lors de la création du pod de build, mais aussi lors des tentatives de nettoyage après la complétion :

```plaintext
Error cleaning up secrets: etcdserver: request timed out
Error cleaning up secrets: etcdserver: leader changed

Error cleaning up pod: etcdserver: request timed out, possibly due to previous leader failure
Error cleaning up pod: etcdserver: request timed out
Error cleaning up pod: context deadline exceeded
```

## `Dial tcp xxx.xx.x.x:xxx: i/o timeout` {#dial-tcp-xxxxxxxxxx-io-timeout}

Il s'agit d'une erreur Kubernetes qui indique généralement que le serveur d'API Kubernetes n'est pas accessible par le gestionnaire de runner. Pour résoudre ce problème :

- Si vous utilisez des politiques de sécurité réseau, accordez l'accès à l'API Kubernetes, généralement sur le port 443 ou le port 6443, ou les deux.
- Assurez-vous que l'API Kubernetes est en cours d'exécution.

## Connexion refusée lors d'une tentative de communication avec l'API Kubernetes {#connection-refused-when-attempting-to-communicate-with-the-kubernetes-api}

Lorsque GitLab Runner effectue une requête à l'API Kubernetes et qu'elle échoue, cela est probablement dû au fait que [`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver) est surchargé et ne peut pas accepter ou traiter les requêtes API.

## `Error cleaning up pod` et `Job failed (system failure): prepare environment: waiting for pod running` {#error-cleaning-up-pod-and-job-failed-system-failure-prepare-environment-waiting-for-pod-running}

Les erreurs suivantes se produisent lorsque Kubernetes ne parvient pas à planifier le pod de job dans les délais impartis. GitLab Runner attend que le pod soit prêt, mais celui-ci échoue, puis tente de nettoyer le pod, ce qui peut également échouer.

```plaintext
Error: Error cleaning up pod: Delete "https://xx.xx.xx.x:443/api/v1/namespaces/gitlab-runner/runner-0001": dial tcp xx.xx.xx.x:443 connect: connection refused

Error: Job failed (system failure): prepare environment: waiting for pod running: Get "https://xx.xx.xx.x:443/api/v1/namespaces/gitlab-runner/runner-0001": dial tcp xx.xx.xx.x:443 connect: connection refused
```

Pour résoudre le problème, vérifiez le nœud principal Kubernetes et tous les nœuds qui exécutent une instance [`kube-apiserver`](https://kubernetes.io/docs/concepts/overview/components/#kube-apiserver). Assurez-vous qu'ils disposent de toutes les ressources nécessaires pour gérer le nombre cible de pods que vous espérez mettre à l'échelle sur le cluster.

Pour modifier le délai d'attente de GitLab Runner avant qu'un pod n'atteigne son statut `Ready`, utilisez le paramètre [`poll_timeout`](_index.md#other-configtoml-settings).

Pour limiter la durée totale d'exécution de l'étape de préparation (y compris la planification des pods), utilisez le paramètre [`prepare_timeout`](../../configuration/advanced-configuration.md#prepare-stage-timeout).

Pour mieux comprendre comment les pods sont planifiés ou pourquoi ils pourraient ne pas être planifiés dans les délais, [consultez la documentation sur le Kubernetes Scheduler](https://kubernetes.io/docs/concepts/scheduling-eviction/kube-scheduler/).

## `request did not complete within requested timeout` {#request-did-not-complete-within-requested-timeout}

Le message `request did not complete within requested timeout` observé lors de la création du pod de build indique qu'un [webhook de contrôle d'admission](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) configuré sur le cluster Kubernetes est en train d'expirer.

Les webhooks de contrôle d'admission constituent un mécanisme d'interception de contrôle administratif au niveau du cluster pour toutes les requêtes API dans leur portée, et peuvent provoquer des défaillances s'ils ne s'exécutent pas dans les délais.

Les webhooks de contrôle d'admission prennent en charge des filtres permettant de contrôler précisément les requêtes API et les sources de namespace qu'ils interceptent. Si les appels API Kubernetes de GitLab Runner n'ont pas besoin de passer par un webhook de contrôle d'admission, vous pouvez modifier la [configuration du sélecteur/filtre du webhook](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/#matching-requests-objectselector) pour ignorer le namespace GitLab Runner, ou appliquer des labels/annotations d'exclusion sur le pod GitLab Runner en configurant `podAnnotations` ou `podLabels` dans le [GitLab Runner Helm Chart `values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/57e026d7f43f63adc32cdd2b21e6d450abcf0686/values.yaml#L490-500).

Par exemple, pour éviter que le [webhook DataDog Admission Controller](https://docs.datadoghq.com/containers/cluster_agent/admission_controller/?tab=operator) n'intercepte les requêtes API effectuées par le pod gestionnaire GitLab Runner, vous pouvez ajouter ce qui suit :

```yaml
podLabels:
  admission.datadoghq.com/enabled: false
```

Pour lister les webhooks de contrôle d'admission d'un cluster Kubernetes, exécutez :

```shell
kubectl get validatingwebhookconfiguration -o yaml
kubectl get mutatingwebhookconfiguration -o yaml
```

Les formes de logs suivantes peuvent être observées lorsqu'un webhook de contrôle d'admission expire :

```plaintext
Job failed (system failure): prepare environment: Timeout: request did not complete within requested timeout
Job failed (system failure): prepare environment: setting up credentials: Timeout: request did not complete within requested timeout
```

Un échec d'un webhook de contrôle d'admission peut également apparaître comme suit :

```plaintext
Job failed (system failure): prepare environment: setting up credentials: Internal error occurred: failed calling webhook "example.webhook.service"
```

## Erreur `Could not resolve host: example.com` {#error-could-not-resolve-host-examplecom}

Si vous utilisez la variante `alpine` de l'[image helper](../../configuration/advanced-configuration.md#helper-image) , il peut y avoir des [problèmes DNS](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/4129) liés au résolveur DNS de `musl` d'Alpine. L'erreur peut ressembler à :

- `fatal: unable to access 'https://gitlab-ci-token:token@example.com/repo/proj.git/': Could not resolve host: example.com`

Utilisez l'option `helper_image_flavor = "ubuntu"` pour résoudre ce problème.

## `docker: Cannot connect to the Docker daemon at tcp://docker:2375. Is the docker daemon running?` {#docker-cannot-connect-to-the-docker-daemon-at-tcpdocker2375-is-the-docker-daemon-running}

Cette erreur peut se produire lors de l'[utilisation de Docker-in-Docker](_index.md#using-dockerdind) si des tentatives d'accès au service DIND sont effectuées avant qu'il n'ait eu le temps de démarrer complètement. Pour une explication plus détaillée, consultez [ce ticket](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27215).

## `curl: (35) OpenSSL SSL_connect: SSL_ERROR_SYSCALL in connection to github.com:443` {#curl-35-openssl-ssl_connect-ssl_error_syscall-in-connection-to-githubcom443}

Cette erreur peut se produire lors de l'[utilisation de Docker-in-Docker](_index.md#using-dockerdind) si le Maximum Transmission Unit (MTU) de DIND est supérieur à celui du réseau overlay Kubernetes. DIND utilise un MTU par défaut de 1500, ce qui est trop grand pour être routé sur le réseau overlay par défaut. Le MTU de DIND peut être modifié dans la définition du service :

```yaml
services:
  - name: docker:dind
    command: ["--mtu=1450"]
```

## `MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown is not supported by windows` {#mountvolumesetup-failed-for-volume-kube-api-access-xxxxx--chown-is-not-supported-by-windows}

Lorsque vous exécutez votre job CI/CD, vous pourriez recevoir une erreur similaire à la suivante :

```plaintext
MountVolume.SetUp failed for volume "kube-api-access-xxxxx" : chown c:\var\lib\kubelet\pods\xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx\volumes\kubernetes.io~projected\kube-api-access-xxxxx\..2022_07_07_20_52_19.102630072\token: not supported by windows
```

Ce problème se produit lorsque vous [utilisez des sélecteurs de nœuds](_index.md#specify-the-node-to-execute-builds) pour exécuter des builds sur des nœuds avec différents systèmes d'exploitation et architectures.

Pour résoudre le problème, configurez `nodeSelector` afin que le pod gestionnaire de runner soit toujours planifié sur un nœud Linux. Par exemple, votre [fichier `values.yaml`](https://gitlab.com/gitlab-org/charts/gitlab-runner/blob/main/values.yaml) doit contenir ce qui suit :

```yaml
nodeSelector:
  kubernetes.io/os: linux
```

## Les pods de build se voient attribuer le rôle IAM du nœud worker au lieu du rôle IAM du runner {#build-pods-are-assigned-the-worker-nodes-iam-role-instead-of-runner-iam-role}

Ce problème se produit lorsque le rôle IAM du nœud worker n'a pas la permission d'assumer le rôle correct. Pour résoudre ce problème, ajoutez la permission `sts:AssumeRole` à la relation de confiance du rôle IAM du nœud worker :

```json
{
    "Effect": "Allow",
    "Principal": {
        "AWS": "arn:aws:iam::<AWS_ACCOUNT_NUMBER>:role/<IAM_ROLE_NAME>"
    },
    "Action": "sts:AssumeRole"
}
```

## Erreur : `pull_policy ([Always]) defined in GitLab pipeline config is not one of the allowed_pull_policies` {#error-pull_policy-always-defined-in-gitlab-pipeline-config-is-not-one-of-the-allowed_pull_policies}

Ce problème se produit si vous avez spécifié une `pull_policy` dans votre `.gitlab-ci.yml` mais qu'aucune politique n'est configurée dans le fichier de configuration du runner. L'erreur peut ressembler à :

- `Preparation failed: invalid pull policy for image 'image-name:latest': pull_policy ([Always]) defined in GitLab pipeline config is not one of the allowed_pull_policies ([])`

Pour résoudre ce problème, ajoutez `allowed_pull_policies` à votre configuration conformément à la section [restreindre les politiques Docker pull](_index.md#restrict-docker-pull-policies).

## Les processus en arrière-plan provoquent le blocage et l'expiration des jobs {#background-processes-cause-jobs-to-hang-and-timeout}

Les processus en arrière-plan démarrés lors de l'exécution du job peuvent [empêcher le job de build de se terminer](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2880). Pour éviter cela, vous pouvez :

- Effectuer un double fork du processus. Par exemple, `command_to_run < /dev/null &> /dev/null &`.
- Arrêter le processus avant de quitter le script du job.

## Erreurs `permission denied` liées au cache {#cache-related-permission-denied-errors}

Les fichiers et dossiers générés dans votre job ont certaines propriétés et permissions UNIX. Lorsque vos fichiers et dossiers sont archivés ou extraits, les détails UNIX sont conservés. Cependant, les fichiers et dossiers peuvent ne pas correspondre aux configurations `USER` des [images helper](../../configuration/advanced-configuration.md#helper-image).

Si vous rencontrez des erreurs liées aux permissions à l'étape `Creating cache ...`, vous pouvez :

- En guise de solution, vérifiez si les données source sont modifiées, par exemple dans le script du job qui crée les fichiers mis en cache.
- En guise de contournement, ajoutez des commandes [chown](https://linux.die.net/man/1/chown) et [chmod](https://linux.die.net/man/1/chmod) correspondantes à vos [directives (`before_`/`after_`)`script:`](https://docs.gitlab.com/ci/yaml/#default).

## Processus shell apparemment redondant dans le conteneur de build avec système init {#apparently-redundant-shell-process-in-build-container-with-init-system}

L'arbre de processus peut inclure un processus shell dans les cas suivants :

- `FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY` est `false` et `FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR` est `true`.
- Le `ENTRYPOINT` de l'image de build est un système init (comme `tini-init` ou `dumb-init`).

```shell
UID    PID   PPID  C STIME TTY          TIME CMD
root     1      0  0 21:58 ?        00:00:00 /scripts-37474587-5556589047/dumb-init -- sh -c if [ -x /usr/local/bin/bash ]; then .exec /usr/local/bin/bash  elif [ -x /usr/bin/bash ]; then .exec /usr/bin/bash  elif [ -x /bin/bash ]; then .exec /bin/bash  elif [ -x /usr/local/bin/sh ]; then .exec /usr/local/bin/sh  elif [ -x /usr/bin/sh ]; then .exec /usr/bin/sh  elif [ -x /bin/sh ]; then .exec /bin/sh  elif [ -x /busybox/sh ]; then .exec /busybox/sh  else .echo shell not found .exit 1 fi
root     7      1  0 21:58 ?        00:00:00 /usr/bin/bash <---------------- WHAT IS THIS???
root    26      1  0 21:58 ?        00:00:00 sh -c (/scripts-37474587-5556589047/detect_shell_script /scripts-37474587-5556589047/step_script 2>&1 | tee -a /logs-37474587-5556589047/output.log) &
root    27     26  0 21:58 ?        00:00:00  \_ /usr/bin/bash /scripts-37474587-5556589047/step_script
root    32     27  0 21:58 ?        00:00:00  |   \_ /usr/bin/bash /scripts-37474587-5556589047/step_script
root    37     32  0 21:58 ?        00:00:00  |       \_ ps -ef --forest
root    28     26  0 21:58 ?        00:00:00  \_ tee -a /logs-37474587-5556589047/output.log
```

Ce processus shell, qui peut être `sh`, `bash` ou `busybox`, avec un `PPID` de 1 et un `PID` de 6 ou 7, est le shell démarré par le script de détection de shell exécuté par le système init (`PID` 1 ci-dessus). Le processus n'est pas redondant et correspond au fonctionnement normal lorsque le conteneur de build s'exécute avec un système init.

## Le pod runner échoue à exécuter les résultats du job et expire malgré une inscription réussie {#runner-pod-fails-to-run-job-results-and-times-out-despite-successful-registration}

Après que le pod runner s'est inscrit auprès de GitLab, il tente d'exécuter un job mais n'y parvient pas et le job finit par expirer. Les erreurs suivantes sont signalées :

```plaintext
There has been a timeout failure or the job got stuck. Check your timeout limits or try again.

This job does not have a trace.
```

Dans ce cas, le runner pourrait recevoir l'erreur suivante :

```plaintext
HTTP 204 No content response code when connecting to the `jobs/request` API.
```

Pour résoudre ce problème, envoyez manuellement une requête POST à l'API pour vérifier si la connexion TCP est bloquée. Si la connexion TCP est bloquée, le runner pourrait ne pas être en mesure de demander les charges utiles des jobs CI.

## `failed to reserve container name` pour le conteneur init-permissions lorsque `gcs-fuse-csi-driver` est utilisé {#failed-to-reserve-container-name-for-init-permissions-container-when-gcs-fuse-csi-driver-is-used}

Le pilote `gcs-fuse-csi-driver` `csi` [ne prend pas en charge le montage de volumes pour le conteneur init](https://github.com/GoogleCloudPlatform/gcs-fuse-csi-driver/issues/38). Cela peut provoquer des échecs lors du démarrage du conteneur init lors de l'utilisation de ce pilote. Les fonctionnalités [introduites dans Kubernetes 1.28](https://kubernetes.io/blog/2023/08/25/native-sidecar-containers/) doivent être prises en charge dans le projet du pilote pour résoudre ce bug.

## Erreur : `only read-only root filesystem container is allowed` {#error-only-read-only-root-filesystem-container-is-allowed}

Dans les clusters avec des politiques d'admission qui forcent les conteneurs à s'exécuter sur des systèmes de fichiers racine montés en lecture seule, cette erreur peut apparaître lorsque :

- Vous installez GitLab Runner.
- GitLab Runner tente de planifier un pod de build.

Ces politiques d'admission sont généralement appliquées par un contrôleur d'admission comme [Gatekeeper](https://open-policy-agent.github.io/gatekeeper/website/) ou [Kyverno](https://kyverno.io/). Par exemple, une politique forçant les conteneurs à s'exécuter sur des systèmes de fichiers racine en lecture seule est la politique Gatekeeper [`readOnlyRootFilesystem`](https://open-policy-agent.github.io/gatekeeper-library/website/validation/read-only-root-filesystem/).

Pour résoudre ce problème :

- Tous les pods déployés sur le cluster doivent respecter les politiques d'admission en définissant `securityContext.readOnlyRootFilesystem` sur `true` pour leurs conteneurs afin que le contrôleur d'admission ne bloque pas le pod.
- Les conteneurs doivent s'exécuter correctement et être en mesure d'écrire dans le système de fichiers même si le système de fichiers racine est monté en lecture seule.

### Pour GitLab Runner {#for-gitlab-runner}

Si GitLab Runner est déployé avec le [chart Helm GitLab Runner](../../install/kubernetes.md), vous devez mettre à jour la configuration du chart GitLab pour avoir :

- Une valeur `securityContext` appropriée :

  ```yaml
  <...>
  securityContext:
    readOnlyRootFilesystem: true
  <...>
  ```

- Un système de fichiers accessible en écriture monté là où le pod peut écrire :

  ```yaml
  <...>
  volumeMounts:
  - name: tmp-dir
    mountPath: /tmp
  volumes:
  - name: tmp-dir
    emptyDir:
      medium: "Memory"
  <...>
  ```

### Pour le pod de build {#for-the-build-pod}

Pour faire en sorte que le pod de build s'exécute sur un système de fichiers racine en lecture seule, configurez les contextes de sécurité des différents conteneurs dans `config.toml`. Vous pouvez définir la variable du chart GitLab `runners.config`, qui est transmise au pod de build :

```yaml
runners:
  config: |
   <...>
   [[runners]]
     [runners.kubernetes.build_container_security_context]
       read_only_root_filesystem = true
     [runners.kubernetes.init_permissions_container_security_context]
       read_only_root_filesystem = true
     [runners.kubernetes.helper_container_security_context,omitempty]
       read_only_root_filesystem = true
     # This section is only needed if jobs with services are used
     [runners.kubernetes.service_container_security_context,omitempty]
       read_only_root_filesystem = true
   <...>
```

Pour que le pod de build et ses conteneurs s'exécutent correctement sur un système de fichiers en lecture seule, vous devez disposer de systèmes de fichiers accessibles en écriture aux emplacements où le pod de build peut écrire. Au minimum, ces emplacements sont les répertoires de build et le répertoire personnel. Assurez-vous que le processus de build dispose d'un accès en écriture aux autres emplacements si nécessaire.

Le répertoire personnel doit généralement être accessible en écriture afin que les programmes puissent y stocker leur configuration et les autres données nécessaires à leur exécution. Le binaire `git` est un exemple de programme qui s'attend à pouvoir écrire dans le répertoire personnel.

Pour rendre le répertoire personnel accessible en écriture quel que soit son chemin dans les différentes images de conteneurs :

1. Montez un volume sur un chemin stable (quel que soit l'image de build utilisée).
1. Modifiez le répertoire personnel en définissant la variable d'environnement `$HOME` globalement pour tous les builds.

Vous pouvez configurer le pod de build et ses conteneurs dans `config.toml` en mettant à jour la valeur de la variable du chart GitLab `runners.config`.

```yaml
runners:
  config: |
   <...>
   [[runners]]
     environment = ["HOME=/build_home"]
     [[runners.kubernetes.volumes.empty_dir]]
       name = "repo"
       mount_path = "/builds"
     [[runners.kubernetes.volumes.empty_dir]]
       name = "build-home"
       mount_path = "/build_home"
   <...>
```

> [!note]
> À la place de `emptyDir`, vous pouvez utiliser n'importe quel autre [type de volume pris en charge](_index.md#configure-volume-types). Étant donné que tous les fichiers qui ne sont pas explicitement gérés et stockés en tant qu'artefacts de build sont généralement éphémères, `emptyDir` convient à la plupart des cas.

## AWS EKS :  Erreur lors du nettoyage du pod : pods "runner-\*\*" introuvables ou le statut est "Failed" {#aws-eks-error-cleaning-up-pod-pods-runner--not-found-or-status-is-failed}

La fonctionnalité de rééquilibrage de zones Amazon EKS équilibre les zones de disponibilité dans un groupe de mise à l'échelle automatique. Cette fonctionnalité peut arrêter un nœud dans une zone de disponibilité et en créer un dans une autre.

Les jobs de runner ne peuvent pas être arrêtés et déplacés vers un autre nœud. Désactivez cette fonctionnalité pour les jobs de runner afin de résoudre cette erreur.

## Services non pris en charge avec les conteneurs Windows {#services-not-supported-with-windows-containers}

Lors d'une tentative d'utilisation des [services](https://docs.gitlab.com/ci/services/) sur des nœuds Windows, ils peuvent échouer avec l'erreur suivante :

- `ERROR: Job failed (system failure): prepare environment: admission webhook "windows.common-webhooks.networking.gke.io" denied the request: spec.hostAliases: Invalid value: []v1.HostAlias{v1.HostAlias{IP:"127.0.0.1", Hostnames:[]string{"<your windows image>"}}}: Windows does not support this field.`

Selon le runtime Kubernetes, l'erreur peut être signalée ou ignorée silencieusement. Par exemple, GKE signale bien l'erreur.

Les services sont implémentés en utilisant `hostAlias` dans l'exécuteur Kubernetes, ce qui n'est pas pris en charge dans les conteneurs Windows.
