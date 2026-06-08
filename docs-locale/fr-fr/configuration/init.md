---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Les services système de GitLab Runner
---

{{< details >}}

- Niveau : Free, Premium, Ultimate
- Offre : GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner utilise la [bibliothèque Go `service`](https://github.com/kardianos/service) pour détecter le système d'exploitation sous-jacent et installer éventuellement le fichier de service en fonction du système init.

> [!note]
> Le package `service` installe, désinstalle, démarre, arrête et exécute un programme en tant que service (daemon). Windows XP+, Linux (systemd, Upstart et System V) et macOS (`launchd`) sont pris en charge.

Lorsque GitLab Runner [est installé](../install/_index.md), le fichier de service est créé automatiquement :

- **systemd** : `/etc/systemd/system/gitlab-runner.service`
- **Upstart** : `/etc/init/gitlab-runner`

## Définir des variables d'environnement personnalisées {#setting-custom-environment-variables}

Vous pouvez exécuter GitLab Runner avec des variables d'environnement personnalisées. Par exemple, vous souhaitez définir `GOOGLE_APPLICATION_CREDENTIALS` dans l'environnement du runner. Cette action est différente du [paramètre de configuration `environment`](advanced-configuration.md#the-runners-section), qui définit les variables automatiquement ajoutées à tous les jobs exécutés par un runner.

### Personnalisation de systemd {#customizing-systemd}

Pour les runners qui utilisent systemd, créez `/etc/systemd/system/gitlab-runner.service.d/env.conf` en utilisant une ligne `Environment=key=value` pour chaque variable à exporter.

Par exemple :

```toml
[Service]
Environment=GOOGLE_APPLICATION_CREDENTIALS=/etc/gitlab-runner/gce-credentials.json
```

Rechargez ensuite la configuration :

```shell
systemctl daemon-reload
systemctl restart gitlab-runner.service
```

### Personnalisation d'Upstart {#customizing-upstart}

Pour les runners qui utilisent Upstart, créez `/etc/init/gitlab-runner.override` et exportez les variables souhaitées.

Par exemple :

```shell
export GOOGLE_APPLICATION_CREDENTIALS="/etc/gitlab-runner/gce-credentials.json"
```

Redémarrez le runner pour que cette modification prenne effet.

## Remplacer le comportement d'arrêt par défaut {#overriding-default-stopping-behavior}

Dans certains cas, vous pouvez souhaiter remplacer le comportement par défaut du service.

Par exemple, lorsque vous mettez à niveau GitLab Runner, vous devez l'arrêter de manière progressive jusqu'à ce que tous les jobs en cours d'exécution soient terminés. Cependant, systemd, Upstart ou d'autres services peuvent redémarrer immédiatement le processus sans même le remarquer.

Ainsi, lorsque vous mettez à niveau GitLab Runner, le script d'installation arrête et redémarre le processus du runner qui gérait probablement de nouveaux jobs à ce moment-là.

### Remplacement de systemd {#overriding-systemd}

Pour les runners qui utilisent systemd, créez `/etc/systemd/system/gitlab-runner.service.d/kill.conf` avec le contenu suivant :

```toml
[Service]
TimeoutStopSec=7200
KillSignal=SIGQUIT
```

Après avoir ajouté ces deux paramètres à la configuration de l'unité systemd, vous pouvez arrêter le runner. Une fois le runner arrêté, systemd utilise `SIGQUIT` comme signal d'arrêt pour stopper le processus. De plus, un délai d'attente de deux heures est défini pour la commande d'arrêt. Si des jobs ne se terminent pas correctement avant ce délai, systemd arrête le processus en utilisant `SIGKILL`.

### Remplacement d'Upstart {#overriding-upstart}

Pour les runners qui utilisent Upstart, créez `/etc/init/gitlab-runner.override` avec le contenu suivant :

```shell
kill signal SIGQUIT
kill timeout 7200
```

Après avoir ajouté ces deux paramètres à la configuration de l'unité Upstart, vous pouvez arrêter le runner. Upstart fait la même chose que systemd ci-dessus.
