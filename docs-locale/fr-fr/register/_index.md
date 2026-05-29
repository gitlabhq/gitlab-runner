---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Enregistrement des runners
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

{{< history >}}

- [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3414) dans GitLab Runner 15.0, une modification du format de la demande d'enregistrement empêche GitLab Runner de communiquer avec les versions antérieures de GitLab. Vous devez utiliser une version de GitLab Runner adaptée à la version de GitLab, ou mettre à niveau l'application GitLab.

{{< /history >}}

L'enregistrement d'un runner est le processus qui lie le runner à une ou plusieurs instances GitLab. Vous devez enregistrer le runner afin qu'il puisse récupérer des jobs depuis l'instance GitLab.

## Prérequis {#requirements}

Avant d'enregistrer un runner :

- Installez [GitLab Runner](../install/_index.md) sur un serveur distinct de celui où GitLab est installé.
- Pour l'enregistrement d'un runner avec Docker, installez [GitLab Runner dans un conteneur Docker](../install/docker.md).

## Enregistrement avec un token d'authentification de runner {#register-with-a-runner-authentication-token}

{{< history >}}

- [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29613) dans GitLab 15.10.

{{< /history >}}

Prérequis :

- Obtenez un token d'authentification de runner. Vous pouvez soit :
  - Créez un runner d'instance, de groupe ou de projet. Pour obtenir des instructions, voir [gérer les runners](https://docs.gitlab.com/ci/runners/runners_scope).
  - Localisez le token d'authentification du runner dans le fichier `config.toml`. Les tokens d'authentification des runners ont le préfixe `glrt-`.

Une fois le runner enregistré, la configuration est sauvegardée dans `config.toml`.

Pour enregistrer le runner avec un [token d'authentification de runner](https://docs.gitlab.com/security/tokens/#runner-authentication-tokens) :

1. Exécutez la commande d'enregistrement :

   {{< tabs >}}

   {{< tab title="Linux" >}}

   ```shell
   sudo gitlab-runner register
   ```

   Si vous êtes derrière un proxy, ajoutez une variable d'environnement, puis exécutez la commande d'enregistrement :

   ```shell
   export HTTP_PROXY=http://yourproxyurl:3128
   export HTTPS_PROXY=http://yourproxyurl:3128

   sudo -E gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="macOS" >}}

   ```shell
   gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="Windows" >}}

   ```shell
   .\gitlab-runner.exe register
   ```

   {{< /tab >}}

   {{< tab title="FreeBSD" >}}

   ```shell
   sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="Docker" >}}

   Pour vous enregistrer avec un conteneur, vous pouvez soit :

   - Utilisez un conteneur `gitlab-runner` éphémère avec le montage de volume de configuration approprié :

     - Pour les montages de volumes système locaux :

       ```shell
       docker run --rm -it -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register
       ```

       Si vous avez utilisé un volume de configuration autre que `/srv/gitlab-runner/config` lors de l'installation, mettez à jour la commande avec le volume approprié.

     - Pour les montages de volumes Docker :

       ```shell
       docker run --rm -it -v gitlab-runner-config:/etc/gitlab-runner gitlab/gitlab-runner:latest register
       ```

   - Utilisez l'exécutable dans un conteneur de runner actif :

     ```shell
     docker exec -it gitlab-runner gitlab-runner register
     ```

   {{< /tab >}}

   {{< /tabs >}}

1. Saisissez votre URL GitLab :
   - Pour les runners sur GitLab Self-Managed, utilisez l'URL de votre instance GitLab. Par exemple, si votre projet est hébergé sur `gitlab.example.com/yourname/yourproject`, l'URL de votre instance GitLab est `https://gitlab.example.com`.
   - Pour les runners sur GitLab.com, l'URL de l'instance GitLab est `https://gitlab.com`.
1. Saisissez le token d'authentification du runner.
1. Saisissez une description pour le runner.
1. Saisissez les tags de job, séparés par des virgules.
1. Saisissez une note de maintenance facultative pour le runner.
1. Saisissez le type d'[exécuteur](../executors/_index.md).

- Pour enregistrer plusieurs runners sur la même machine hôte, chacun avec une configuration différente, répétez la commande `register`.
- Pour enregistrer la même configuration sur plusieurs machines hôtes, utilisez le même token d'authentification de runner pour chaque enregistrement de runner. Pour plus d'informations, voir [Réutiliser une configuration de runner](../fleet_scaling/_index.md#reusing-a-runner-configuration).

Vous pouvez également utiliser le [mode non-interactif](../commands/_index.md#non-interactive-registration) pour utiliser des arguments supplémentaires lors de l'enregistrement du runner :

{{< tabs >}}

{{< tab title="Linux" >}}

```shell
sudo gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

{{< /tab >}}

{{< tab title="macOS" >}}

```shell
gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

{{< /tab >}}

{{< tab title="Windows" >}}

```shell
.\gitlab-runner.exe register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker-windows" \
  --docker-image mcr.microsoft.com/windows/servercore:1809_amd64 \
  --description "docker-runner"
```

{{< /tab >}}

{{< tab title="FreeBSD" >}}

```shell
sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

{{< /tab >}}

{{< tab title="Docker" >}}

```shell
docker run --rm -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --token "$RUNNER_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner"
```

{{< /tab >}}

{{< /tabs >}}

## Enregistrement avec un token d'enregistrement de runner (déprécié) {#register-with-a-runner-registration-token-deprecated}

> [!warning]
> Les tokens d'enregistrement de runners et plusieurs arguments de configuration de runners ont été [dépréciés](https://gitlab.com/gitlab-org/gitlab/-/issues/380872). Leur suppression est prévue dans GitLab 20.0. Utilisez plutôt des tokens d'authentification de runners. Pour plus d'informations, voir [Migration vers le nouveau workflow d'enregistrement de runners](https://docs.gitlab.com/ci/runners/new_creation_workflow/).

Prérequis :

- Les tokens d'enregistrement de runners doivent être [activés](https://docs.gitlab.com/administration/settings/continuous_integration/#control-runner-registration) dans la zone d'administration.
- Obtenez un token d'enregistrement de runner pour l'instance, le groupe ou le projet souhaité. Pour obtenir des instructions, voir [gérer les runners](https://docs.gitlab.com/ci/runners/runners_scope).

Une fois le runner enregistré, la configuration est sauvegardée dans `config.toml`.

Pour enregistrer le runner avec un [token d'enregistrement de runner](https://docs.gitlab.com/security/tokens/#runner-registration-tokens-legacy) :

1. Exécutez la commande d'enregistrement :

   {{< tabs >}}

   {{< tab title="Linux" >}}

   ```shell
   sudo gitlab-runner register
   ```

   Si vous êtes derrière un proxy, ajoutez une variable d'environnement, puis exécutez la commande d'enregistrement :

   ```shell
   export HTTP_PROXY=http://yourproxyurl:3128
   export HTTPS_PROXY=http://yourproxyurl:3128

   sudo -E gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="macOS" >}}

   ```shell
   gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="Windows" >}}

   ```shell
   .\gitlab-runner.exe register
   ```

   {{< /tab >}}

   {{< tab title="FreeBSD" >}}

   ```shell
   sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
   ```

   {{< /tab >}}

   {{< tab title="Docker" >}}

   Pour lancer un conteneur `gitlab-runner` éphémère afin d'enregistrer le conteneur que vous avez créé lors de l'installation :

   - Pour les montages de volumes système locaux :

     ```shell
     docker run --rm -it -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register
     ```

     Si vous avez utilisé un volume de configuration autre que `/srv/gitlab-runner/config` lors de l'installation, mettez à jour la commande avec le volume approprié.

   - Pour les montages de volumes Docker :

     ```shell
     docker run --rm -it -v gitlab-runner-config:/etc/gitlab-runner gitlab/gitlab-runner:latest register
     ```

   {{< /tab >}}

   {{< /tabs >}}

1. Saisissez votre URL GitLab :
   - Pour les runners sur GitLab Self-Managed, utilisez l'URL de votre instance GitLab. Par exemple, si votre projet est hébergé sur `gitlab.example.com/yourname/yourproject`, l'URL de votre instance GitLab est `https://gitlab.example.com`.
   - Pour GitLab.com, l'URL de l'instance GitLab est `https://gitlab.com`.
1. Saisissez le token que vous avez obtenu pour enregistrer le runner.
1. Saisissez une description pour le runner.
1. Saisissez les tags de job, séparés par des virgules.
1. Saisissez une note de maintenance facultative pour le runner.
1. Saisissez le type d'[exécuteur](../executors/_index.md).

Pour enregistrer plusieurs runners sur la même machine hôte, chacun avec une configuration différente, répétez la commande `register`.

Vous pouvez également utiliser le [mode non-interactif](../commands/_index.md#non-interactive-registration) pour utiliser des arguments supplémentaires lors de l'enregistrement du runner :

{{< tabs >}}

{{< tab title="Linux" >}}

```shell
sudo gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< tab title="macOS" >}}

```shell
gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< tab title="Windows" >}}

```shell
.\gitlab-runner.exe register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker-windows" \
  --docker-image mcr.microsoft.com/windows/servercore:1809_amd64 \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< tab title="FreeBSD" >}}

```shell
sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< tab title="Docker" >}}

```shell
docker run --rm -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register \
  --non-interactive \
  --url "https://gitlab.com/" \
  --registration-token "$PROJECT_REGISTRATION_TOKEN" \
  --executor "docker" \
  --docker-image alpine:latest \
  --description "docker-runner" \
  --maintenance-note "Free-form maintainer notes about this runner" \
  --tag-list "docker,aws" \
  --run-untagged="true" \
  --locked="false" \
  --access-level="not_protected"
```

{{< /tab >}}

{{< /tabs >}}

- `--access-level` crée un [runner protégé](https://docs.gitlab.com/ci/runners/configure_runners/#prevent-runners-from-revealing-sensitive-information).
  - Pour un runner protégé, utilisez le paramètre `--access-level="ref_protected"`.
  - Pour un runner non protégé, utilisez `--access-level="not_protected"` ou laissez la valeur non définie.
- `--maintenance-note` permet d'ajouter des informations qui peuvent être utiles pour la maintenance du runner. La longueur maximale est de 255 caractères.

### Processus d'enregistrement compatible avec l'ancienne version {#legacy-compatible-registration-process}

{{< history >}}

- [Introduit](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/4157) dans GitLab 16.2.

{{< /history >}}

Les tokens d'enregistrement de runners et plusieurs arguments de configuration de runners ont été [dépréciés](https://gitlab.com/gitlab-org/gitlab/-/issues/379743). Leur suppression est prévue dans GitLab 20.0. Pour garantir une perturbation minimale de votre workflow d'automatisation, le `legacy-compatible registration process` se déclenche si un token d'authentification de runner est spécifié dans le paramètre hérité `--registration-token`.

Le processus d'enregistrement compatible avec l'ancienne version ignore les paramètres de ligne de commande suivants. Ces paramètres ne peuvent être configurés que lors de la création d'un runner dans l'interface utilisateur ou via l'API.

- `--locked`
- `--access-level`
- `--run-untagged`
- `--maximum-timeout`
- `--paused`
- `--tag-list`
- `--maintenance-note`

## Enregistrement avec un modèle de configuration {#register-with-a-configuration-template}

Vous pouvez utiliser un modèle de configuration pour enregistrer un runner avec des paramètres non pris en charge par la commande `register`.

Prérequis :

- Le volume pour l'emplacement du fichier modèle doit être monté sur le conteneur GitLab Runner.
- Un token d'authentification ou d'enregistrement de runner :
  - Obtenez un token d'authentification de runner (recommandé). Vous pouvez soit :
    - Obtenez un token d'authentification de runner pour l'instance, le groupe ou le projet souhaité. Pour obtenir des instructions, voir [gérer les runners](https://docs.gitlab.com/ci/runners/runners_scope).
    - Localisez le token d'authentification du runner dans le fichier `config.toml`. Les tokens d'authentification des runners ont le préfixe `glrt-`.
  - Obtenez un token d'enregistrement de runner (déprécié) pour une instance, un groupe ou un projet. Pour obtenir des instructions, voir [gérer les runners](https://docs.gitlab.com/ci/runners/runners_scope).

Le modèle de configuration peut être utilisé pour les environnements automatisés qui ne prennent pas en charge certains arguments de la commande `register` en raison de :

- Limites de taille des variables d'environnement selon l'environnement.
- Options de ligne de commande non disponibles pour les volumes d'exécuteur pour Kubernetes.

> [!warning]
> Le modèle de configuration ne prend en charge qu'une seule section [`[[runners]]`](../configuration/advanced-configuration.md#the-runners-section) et ne prend pas en charge les options globales.

Pour enregistrer un runner :

1. Créez un fichier modèle de configuration au format `.toml` et ajoutez vos spécifications. Par exemple :

   ```toml
   [[runners]]
     [runners.kubernetes]
     [runners.kubernetes.volumes]
       [[runners.kubernetes.volumes.empty_dir]]
         name = "empty_dir"
         mount_path = "/path/to/empty_dir"
         medium = "Memory"
   ```

1. Ajoutez le chemin vers le fichier. Vous pouvez utiliser soit :
   - Le [mode non-interactif](../commands/_index.md#non-interactive-registration) en ligne de commande :

     ```shell
     $ sudo gitlab-runner register \
         --template-config /tmp/test-config.template.toml \
         --non-interactive \
         --url "https://gitlab.com" \
         --token <TOKEN> \ "# --registration-token if using the deprecated runner registration token"
         --name test-runner \
         --executor kubernetes
         --host = "http://localhost:9876/"
     ```

   - La variable d'environnement dans le fichier `.gitlab.yaml` :

     ```yaml
     variables:
       TEMPLATE_CONFIG_FILE = <file_path>
     ```

     Si vous mettez à jour la variable d'environnement, vous n'avez pas besoin d'ajouter le chemin du fichier dans la commande `register` à chaque enregistrement.

Une fois le runner enregistré, les paramètres du modèle de configuration sont fusionnés avec l'entrée `[[runners]]` créée dans `config.toml` :

```toml
concurrent = 1
check_interval = 0

[session_server]
  session_timeout = 1800

[[runners]]
  name = "test-runner"
  url = "https://gitlab.com"
  token = "glrt-<TOKEN>"
  executor = "kubernetes"
  [runners.kubernetes]
    host = "http://localhost:9876/"
    bearer_token_overwrite_allowed = false
    image = ""
    namespace = ""
    namespace_overwrite_allowed = ""
    privileged = false
    service_account_overwrite_allowed = ""
    pod_labels_overwrite_allowed = ""
    pod_annotations_overwrite_allowed = ""
    [runners.kubernetes.volumes]

      [[runners.kubernetes.volumes.empty_dir]]
        name = "empty_dir"
        mount_path = "/path/to/empty_dir"
        medium = "Memory"
```

Les paramètres du modèle sont fusionnés uniquement pour les options qui sont :

- Chaînes vides
- Entrées nulles ou inexistantes
- Zéros

Les arguments de ligne de commande ou les variables d'environnement ont la priorité sur les paramètres du modèle de configuration. Par exemple, si le modèle spécifie un exécuteur `docker`, mais que la ligne de commande spécifie `shell`, l'exécuteur configuré est `shell`.

## Enregistrement d'un runner pour les tests d'intégration de GitLab Community Edition {#register-a-runner-for-gitlab-community-edition-integration-tests}

Pour tester les intégrations de GitLab Community Edition, utilisez un modèle de configuration pour enregistrer un runner avec un exécuteur Docker confiné.

1. Créez un [runner de projet](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-project-runner-with-a-runner-authentication-token).
1. Créez un modèle avec la section `[[runners.docker.services]]` :

   ```shell
   $ cat > /tmp/test-config.template.toml << EOF
   [[runners]]
   [runners.docker]
   [[runners.docker.services]]
   name = "mysql:latest"
   [[runners.docker.services]]
   name = "redis:latest"

   EOF
   ```

1. Enregistrez le runner :

   {{< tabs >}}

   {{< tab title="Linux" >}}

   ```shell
   sudo gitlab-runner register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< tab title="macOS" >}}

   ```shell
   gitlab-runner register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< tab title="Windows" >}}

   ```shell
   .\gitlab-runner.exe register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< tab title="FreeBSD" >}}

   ```shell
   sudo -u gitlab-runner -H /usr/local/bin/gitlab-runner register
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< tab title="Docker" >}}

   ```shell
   docker run --rm -v /srv/gitlab-runner/config:/etc/gitlab-runner gitlab/gitlab-runner register \
     --non-interactive \
     --url "https://gitlab.com" \
     --token "$RUNNER_AUTHENTICATION_TOKEN" \
     --template-config /tmp/test-config.template.toml \
     --description "gitlab-ce-ruby-3.1" \
     --executor "docker" \
     --docker-image ruby:3.1
   ```

   {{< /tab >}}

   {{< /tabs >}}

Pour plus d'options de configuration, voir [Configuration avancée](../configuration/advanced-configuration.md).

## Enregistrement des runners avec Docker {#registering-runners-with-docker}

Après avoir enregistré le runner avec un conteneur Docker :

- La configuration est écrite dans votre volume de configuration. Par exemple, `/srv/gitlab-runner/config`.
- Le conteneur utilise le volume de configuration pour charger le runner.

> [!note]
> Si `gitlab-runner restart` s'exécute dans un conteneur Docker, GitLab Runner démarre un nouveau processus au lieu de redémarrer le processus existant. Pour appliquer les modifications de configuration, redémarrez plutôt le conteneur Docker.

## Dépannage {#troubleshooting}

### Erreur : `Check registration token` {#error-check-registration-token}

Le message d'erreur `check registration token` s'affiche lorsque l'instance GitLab ne reconnaît pas le token d'enregistrement de runner saisi lors de l'enregistrement. Ce problème peut se produire dans l'un ou l'autre des cas suivants :

- Le token d'enregistrement du runner d'instance, de groupe ou de projet a été modifié dans GitLab.
- Un token d'enregistrement de runner incorrect a été saisi.

Lorsque cette erreur se produit, vous pouvez demander à un administrateur GitLab de :

- Vérifier que le token d'enregistrement du runner est valide.
- Confirmer que l'enregistrement du runner dans le projet ou le groupe est [autorisé](https://docs.gitlab.com/administration/settings/continuous_integration/#restrict-runner-registration-for-a-specific-group).

### Erreur : `410 Gone - runner registration disallowed` {#error-410-gone---runner-registration-disallowed}

Le message d'erreur `410 Gone - runner registration disallowed` s'affiche lorsque l'enregistrement de runner via des tokens d'enregistrement a été désactivé.

Lorsque cette erreur se produit, vous pouvez demander à un administrateur GitLab de :

- Vérifier que le token d'enregistrement du runner est valide.
- Confirmer que l'enregistrement du runner dans l'instance est [autorisé](https://docs.gitlab.com/administration/settings/continuous_integration/#control-runner-registration).
- Dans le cas d'un token d'enregistrement de runner de groupe ou de projet, vérifier que l'enregistrement du runner dans le groupe et/ou le projet concerné [est autorisé](https://docs.gitlab.com/ci/runners/runners_scope/#enable-use-of-runner-registration-tokens-in-projects-and-groups).
