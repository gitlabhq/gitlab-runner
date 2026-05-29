---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: "Définir des variables d'environnement dans le chart Helm de GitLab Runner"
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Les variables d'environnement sont des paires clé-valeur contenant des informations que les applications peuvent utiliser pour adapter leur comportement au moment de l'exécution. Ces variables sont injectées dans l'environnement du conteneur. Vous pouvez utiliser ces variables pour transmettre des données de configuration, des secrets ou toute autre information dynamique requise par l'application.

Vous pouvez définir des variables d'environnement dans le chart Helm de GitLab Runner en utilisant :

- [la propriété `runners.config`](#use-the-runnersconfig-property)
- [les propriétés dans `values.yaml`](#use-valuesyaml-properties)

## Utiliser la propriété `runners.config` {#use-the-runnersconfig-property}

Vous pouvez configurer des variables d'environnement via la propriété `runners.config`, de manière similaire à ce que vous feriez dans le fichier `config.toml` :

```yaml
runners:
  config: |
    [[runners]]
      shell = "bash"
      [runners.kubernetes]
        host = ""
        environment = ["FF_USE_ADVANCED_POD_SPEC_CONFIGURATION=true"]
```

Les variables définies de cette manière sont appliquées à la fois au pod de job et au conteneur GitLab Runner Manager. Dans l'exemple ci-dessus, le feature flag `FF_USE_ADVANCED_POD_SPEC_CONFIGURATION` est défini comme variable d'environnement, que le GitLab Runner Manager utilise pour modifier son comportement.

## Utiliser les propriétés `values.yaml` {#use-valuesyaml-properties}

Vous pouvez également définir des variables d'environnement en utilisant les propriétés suivantes dans `values.yaml`. Ces variables n'affectent que le conteneur GitLab Runner Manager.

- `envVars`

  ```yaml
  envVars:
    - name: RUNNER_EXECUTOR
      value: kubernetes
  ```

- `extraEnv`

  ```yaml
  extraEnv:
    CACHE_S3_SERVER_ADDRESS: s3.amazonaws.com
    CACHE_S3_BUCKET_NAME: runners-cache
    CACHE_S3_BUCKET_LOCATION: us-east-1
    CACHE_SHARED: true
  ```

- `extraEnvFrom`

  ```yaml
  extraEnvFrom:
    CACHE_S3_ACCESS_KEY:
      secretKeyRef:
        name: s3access
        key: accesskey
    CACHE_S3_SECRET_KEY:
      secretKeyRef:
        name: s3access
        key: secretkey
  ```

  Pour plus d'informations sur `extraEnvFrom`, consultez :

  - [`Distribute Credentials Securely Using Secrets`](https://kubernetes.io/docs/tasks/inject-data-application/distribute-credentials-secure/)
  - [`Use container fields as values for environment variables`](https://kubernetes.io/docs/tasks/inject-data-application/environment-variable-expose-pod-information/#use-container-fields-as-values-for-environment-variables)
