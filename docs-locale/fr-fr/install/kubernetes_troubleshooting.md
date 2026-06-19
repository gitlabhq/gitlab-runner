---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Dépannage du chart Helm GitLab Runner
---

## Erreur : `Job failed (system failure): secrets is forbidden` {#error-job-failed-system-failure-secrets-is-forbidden}

Si vous voyez l'erreur suivante, [activez la prise en charge du RBAC](kubernetes_helm_chart_configuration.md#enable-rbac-support) pour la corriger :

```plaintext
Using Kubernetes executor with image alpine ...
ERROR: Job failed (system failure): secrets is forbidden: User "system:serviceaccount:gitlab:default"
cannot create resource "secrets" in API group "" in the namespace "gitlab"
```

## Erreur : `Unable to mount volumes for pod` {#error-unable-to-mount-volumes-for-pod}

Si vous constatez des échecs de montage de volume pour un secret requis, assurez-vous d'avoir stocké les jetons d'enregistrement ou les jetons de runner dans des secrets.

## Lenteur des chargements d'artefacts vers Google Cloud Storage {#slow-artifact-uploads-to-google-cloud-storage}

Les chargements d'artefacts vers Google Cloud Storage peuvent connaître des performances réduites (un débit plus lent) en raison du pod helper du runner qui devient limité par le CPU. Pour atténuer ce problème, augmentez la limite CPU du pod Helper :

```yaml
runners:
  config: |
    [[runners]]
      [runners.kubernetes]
        helper_cpu_limit = "250m"
```

Pour plus d'informations, consultez [le ticket 28393](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28393#note_722733798).

## Erreur : `PANIC: creating directory: mkdir /nonexistent: permission denied` {#error-panic-creating-directory-mkdir-nonexistent-permission-denied}

Pour résoudre cette erreur, passez à l'[image Docker GitLab Runner basée sur Ubuntu](kubernetes_helm_chart_configuration.md#switch-to-the-ubuntu-based-gitlab-runner-docker-image).

## Erreur : `invalid header field for "Private-Token"` {#error-invalid-header-field-for-private-token}

Vous pourriez voir cette erreur si la valeur `runner-token` dans `gitlab-runner-secret` est encodée en base64 avec un caractère de nouvelle ligne (`\n`) à la fin :

```plaintext
couldn't execute POST against "https:/gitlab.example.com/api/v4/runners/verify":
net/http: invalid header field for "Private-Token"
```

Pour résoudre ce problème, assurez-vous qu'une nouvelle ligne (`\n`) n'est pas ajoutée à la fin de la valeur du jeton. Par exemple : `echo -n <gitlab-runner-token> | base64`.

## Erreur : `FATAL: Runner configuration is reserved` {#error-fatal-runner-configuration-is-reserved}

Vous pourriez obtenir l'erreur suivante dans les journaux du pod après l'installation du chart Helm GitLab Runner :

```plaintext
FATAL: Runner configuration other than name and executor configuration is reserved
(specifically --locked, --access-level, --run-untagged, --maximum-timeout, --paused, --tag-list, and --maintenance-note)
and cannot be specified when registering with a runner authentication token. This configuration is specified
on the GitLab server. Please try again without specifying any of those arguments
```

Cette erreur se produit lorsque vous utilisez un jeton d'authentification et fournissez un jeton via un secret. Pour y remédier, examinez votre fichier YAML de valeurs et assurez-vous que vous n'utilisez aucune valeur obsolète. Pour plus d'informations sur les valeurs obsolètes, consultez [Installation de GitLab Runner avec le chart Helm](https://docs.gitlab.com/ci/runners/new_creation_workflow/#installing-gitlab-runner-with-helm-chart).
