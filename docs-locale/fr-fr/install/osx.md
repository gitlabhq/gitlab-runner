---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: "Téléchargez, installez et configurez GitLab Runner en tant que service en mode utilisateur sur les systèmes Apple Silicon et Intel x86-64."
title: Installer GitLab Runner sur macOS
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Installez GitLab Runner sur macOS sur des systèmes Apple Silicon ou Intel x86-64. GitLab lui-même s'exécute généralement sur un conteneur ou une machine virtuelle, localement ou à distance.

## Modes de service macOS {#macos-service-modes}

Sur macOS, GitLab Runner s'exécute en tant que `LaunchAgent` en mode utilisateur, et non en tant que `LaunchDaemon` au niveau système. Il s'agit du seul mode pris en charge.

En mode utilisateur, le runner :

- S'exécute en tant qu'utilisateur actuellement authentifié, et non en tant que root.
- Démarre lorsque cet utilisateur se connecte, et s'arrête lorsqu'il se déconnecte.
- A accès au trousseau et à la session d'interface utilisateur de l'utilisateur, ce qui est requis pour exécuter le simulateur iOS et effectuer la signature de code.
- Stocke sa configuration dans `~/.gitlab-runner/config.toml`.

Un `LaunchDaemon` au niveau système démarre au démarrage, s'exécute en tant que root et n'a pas accès à une session utilisateur. GitLab Runner ne prend pas en charge l'exécution en tant que `LaunchDaemon`.

Pour que le runner reste disponible après un redémarrage, activez la connexion automatique sur la machine macOS.

## Installer GitLab Runner {#install-gitlab-runner}

Installez GitLab Runner sur macOS pour exécuter des jobs CI/CD sur des systèmes Apple Silicon ou Intel x86-64.

Prérequis :

- Vous devez être connecté à la machine macOS avec le compte utilisateur qui exécute les jobs. N'utilisez pas de session SSH pour cette procédure. Utilisez un terminal GUI local.

Pour installer GitLab Runner :

1. Téléchargez le binaire pour votre système :

   - Pour Intel (x86-64) :

     ```shell
     sudo curl --output /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-amd64"
     ```

   - Pour Apple Silicon :

     ```shell
     sudo curl --output /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-arm64"
     ```

   Pour télécharger un binaire pour une release taguée spécifique, consultez [télécharger toute autre release taguée](bleeding-edge.md#download-any-other-tagged-release).

1. Rendez le binaire exécutable :

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. [Enregistrez un runner](../register/_index.md) configuration. Utilisez l'[exécuteur shell](../executors/shell.md) pour les builds iOS et macOS. Pour les détails de sécurité, consultez [sécurité pour l'exécuteur shell](../security/_index.md#usage-of-shell-executor).

1. Installez et démarrez le service GitLab Runner :

   ```shell
   cd ~
   gitlab-runner install
   gitlab-runner start
   ```

1. Redémarrez votre système.

La commande `gitlab-runner install` crée un plist `LaunchAgent` dans `~/Library/LaunchAgents/gitlab-runner.plist` et l'enregistre avec `launchctl`. Si vous rencontrez des erreurs, consultez [dépannage](#troubleshooting).

## Emplacements des fichiers de configuration {#configuration-file-locations}

| Fichier                 | Chemin                                             |
|----------------------|--------------------------------------------------|
| Configuration        | `~/.gitlab-runner/config.toml`                   |
| Plist `LaunchAgent`  | `~/Library/LaunchAgents/gitlab-runner.plist`     |
| Journal de sortie standard  | `~/Library/Logs/gitlab-runner.out.log`           |
| Journal d'erreur standard   | `~/Library/Logs/gitlab-runner.err.log`           |

Pour plus d'informations sur les options de configuration, consultez [configuration avancée](../configuration/advanced-configuration.md).

## Mettre à jour GitLab Runner {#upgrade-gitlab-runner}

Pour mettre à jour GitLab Runner vers une version plus récente :

1. Arrêtez le service :

   ```shell
   gitlab-runner stop
   ```

1. Téléchargez le binaire pour remplacer l'exécutable GitLab Runner :

   - Pour Intel (x86-64) :

     ```shell
     sudo curl -o /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-amd64"
     ```

   - Pour Apple Silicon :

     ```shell
     sudo curl -o /usr/local/bin/gitlab-runner \
       "https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-darwin-arm64"
     ```

   Pour télécharger un binaire pour une release taguée spécifique, consultez [télécharger toute autre release taguée](bleeding-edge.md#download-any-other-tagged-release).

1. Rendez le binaire exécutable :

   ```shell
   sudo chmod +x /usr/local/bin/gitlab-runner
   ```

1. Démarrez le service :

   ```shell
   gitlab-runner start
   ```

## Mettre à jour le fichier de service {#upgrade-the-service-file}

Pour mettre à jour la configuration `LaunchAgent`, désinstallez et réinstallez le service :

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

## Utiliser `codesign` avec GitLab Runner {#use-codesign-with-gitlab-runner}

Si vous avez installé GitLab Runner avec Homebrew et que votre build appelle `codesign`, vous devrez peut-être définir `<key>SessionCreate</key><true/>` pour accéder au trousseau utilisateur.

> [!note]
> GitLab ne maintient pas la formule Homebrew. Utilisez le binaire officiel pour installer GitLab Runner.

Dans l'exemple suivant, le runner exécute des builds en tant qu'utilisateur `gitlab` et a besoin d'accéder aux certificats de signature de cet utilisateur :

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>SessionCreate</key><true/>
    <key>KeepAlive</key>
    <dict>
      <key>SuccessfulExit</key>
      <false/>
    </dict>
    <key>RunAtLoad</key><true/>
    <key>Disabled</key><false/>
    <key>Label</key>
    <string>com.gitlab.gitlab-runner</string>
    <key>UserName</key>
    <string>gitlab</string>
    <key>GroupName</key>
    <string>staff</string>
    <key>ProgramArguments</key>
    <array>
      <string>/usr/local/opt/gitlab-runner/bin/gitlab-runner</string>
      <string>run</string>
      <string>--working-directory</string>
      <string>/Users/gitlab/gitlab-runner</string>
      <string>--config</string>
      <string>/Users/gitlab/gitlab-runner/config.toml</string>
      <string>--service</string>
      <string>gitlab-runner</string>
      <string>--syslog</string>
    </array>
    <key>EnvironmentVariables</key>
    <dict>
      <key>PATH</key>
      <string>/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
  </dict>
</plist>
```

## Dépannage {#troubleshooting}

Lors de l'installation de GitLab Runner sur macOS, vous pouvez rencontrer les problèmes suivants.

Pour le dépannage général, consultez [dépannage de GitLab Runner](../faq/_index.md).

### Erreur : `killed: 9` {#error-killed-9}

Sur Apple Silicon, vous pouvez obtenir cette erreur lorsque vous exécutez les commandes `gitlab-runner install`, `gitlab-runner start` ou `gitlab-runner register`.

Pour résoudre cette erreur, assurez-vous que les répertoires pour `StandardOutPath` et `StandardErrorPath` dans `~/Library/LaunchAgents/gitlab-runner.plist` existent et sont accessibles en écriture. Par exemple :

```xml
<key>StandardErrorPath</key>
<string>/Users/<username>/gitlab-runner-log/gitlab-runner.err.log</string>
<key>StandardOutPath</key>
<string>/Users/<username>/gitlab-runner-log/gitlab-runner.out.log</string>
```

### Erreur : `"launchctl" failed: Could not find domain for` {#error-launchctl-failed-could-not-find-domain-for}

Cette erreur se produit lorsque vous gérez le service GitLab Runner via SSH au lieu d'un terminal GUI local.

Pour résoudre cette erreur, ouvrez une application de terminal directement sur la machine macOS et exécutez les commandes `install` et `start` depuis celle-ci.

### Erreur : `Failed to authorize rights (0x1) with status: -60007` {#error-failed-to-authorize-rights-0x1-with-status--60007}

Cette erreur a deux causes possibles.

Votre compte utilisateur n'a pas accès aux outils de développement. Pour accorder l'accès :

```shell
DevToolsSecurity -enable
sudo security authorizationdb remove system.privilege.taskport is-developer
```

Ou bien, le plist `LaunchAgent` a `SessionCreate` défini sur `true`. Pour résoudre ce problème, réinstallez le service :

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

Vérifiez que `~/Library/LaunchAgents/gitlab-runner.plist` a maintenant `SessionCreate` défini sur `false`.

### Erreur : `Failed to connect to path port 3000: Operation timed out` {#error-failed-to-connect-to-path-port-3000-operation-timed-out}

Le runner ne peut pas atteindre votre instance GitLab. Vérifiez les pare-feux, les proxys, la configuration du routage ou les problèmes de permissions susceptibles de bloquer la connexion.

### Erreur : `FATAL: Failed to start gitlab-runner: exit status 134` {#error-fatal-failed-to-start-gitlab-runner-exit-status-134}

Cette erreur indique que le service GitLab Runner n'est pas installé correctement.

Pour résoudre cette erreur, réinstallez le service :

```shell
gitlab-runner uninstall
gitlab-runner install
gitlab-runner start
```

Si l'erreur persiste, connectez-vous au bureau GUI macOS au lieu d'utiliser SSH, et exécutez les commandes depuis un terminal sur place. Le `LaunchAgent` nécessite une session de connexion graphique pour démarrer.

Pour les instances macOS sur AWS, suivez la [documentation AWS](https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/connect-to-mac-instance.html) pour vous connecter à l'interface graphique, puis réessayez depuis un terminal dans cette session.

### Erreur : `launchctl failed: Load failed: 5: Input/output error` {#error-launchctl-failed-load-failed-5-inputoutput-error}

Si vous rencontrez cette erreur lorsque vous exécutez la commande `gitlab-runner start`, vérifiez d'abord si le runner est déjà en cours d'exécution :

```shell
gitlab-runner status
```

Si le runner n'est pas en cours d'exécution, assurez-vous que les répertoires pour `StandardOutPath` et `StandardErrorPath` dans `~/Library/LaunchAgents/gitlab-runner.plist` existent et que le compte utilisateur du runner dispose d'un accès en lecture et en écriture à ces répertoires. Puis démarrez le runner :

```shell
gitlab-runner start
```

### Erreur : `couldn't build CA Chain` {#error-couldnt-build-ca-chain}

Cette erreur peut se produire après une mise à jour vers GitLab Runner v15.5.0. Le message d'erreur complet est :

```plaintext
ERROR: Error on fetching TLS Data from API response... error  error=couldn't build CA Chain:
error while fetching certificates from TLS ConnectionState: error while fetching certificates
into the CA Chain: couldn't resolve certificates chain from the leaf certificate: error while
resolving certificates chain with verification: error while verifying last certificate from
the chain: x509: "Baltimore CyberTrust Root" certificate is not permitted for this usage
runner=x7kDEc9Q
```

Pour résoudre cette erreur :

1. Mettez à jour vers GitLab Runner v15.5.1 ou ultérieur.
1. Si vous ne pouvez pas effectuer la mise à jour, définissez `FF_RESOLVE_FULL_TLS_CHAIN` sur `false` dans la [configuration `[runners.feature_flags]`](../configuration/feature-flags.md#enable-feature-flag-in-runner-configuration) :

   ```toml
   [[runners]]
     name = "example-runner"
     url = "https://gitlab.com/"
     token = "TOKEN"
     executor = "docker"
     [runners.feature_flags]
       FF_RESOLVE_FULL_TLS_CHAIN = false
   ```

### L'assistant d'informations d'identification Git de Homebrew provoque le blocage des récupérations {#homebrew-git-credential-helper-causes-fetches-to-hang}

Si Homebrew a installé Git, il peut avoir ajouté une entrée `credential.helper = osxkeychain` dans `/usr/local/etc/gitconfig`. Cela met en cache les informations d'identification dans le trousseau macOS et peut provoquer le blocage de `git fetch`.

Pour supprimer l'assistant d'informations d'identification à l'échelle du système :

```shell
git config --system --unset credential.helper
```

Pour le désactiver uniquement pour l'utilisateur GitLab Runner :

```shell
git config --global --add credential.helper ''
```

Pour vérifier le paramètre actuel :

```shell
git config credential.helper
```
