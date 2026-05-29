---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: Installez GitLab Runner sur les systèmes Windows.
title: Installer GitLab Runner sur Windows
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Pour installer et exécuter GitLab Runner sur Windows, vous avez besoin :

- De Git, qui peut être installé depuis le [site officiel](https://git-scm.com/download/win)
- D'un mot de passe pour votre compte utilisateur, si vous souhaitez l'exécuter sous votre compte utilisateur plutôt que sous le compte système intégré (Built-in System Account).
- De définir les paramètres régionaux du système sur Anglais (États-Unis) pour éviter les problèmes d'encodage des caractères. Pour plus d'informations, consultez le [ticket 38702](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38702).

## Installation {#installation}

1. Créez un dossier quelque part sur votre système, par exemple `C:\GitLab-Runner`.
1. Téléchargez le binaire pour [x86 64 bits](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-amd64.exe), [ARM 64 bits](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-arm64.exe) ou [x86 32 bits](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-386.exe) et placez-le dans le dossier que vous avez créé. La procédure suivante suppose que vous avez renommé le binaire en `gitlab-runner.exe` (facultatif). Vous pouvez télécharger un binaire pour chaque version disponible comme décrit dans [Bleeding Edge - télécharger n'importe quelle autre version taguée](bleeding-edge.md#download-any-other-tagged-release).
1. Veillez à restreindre les permissions `Write` sur le répertoire et l'exécutable GitLab Runner. Si vous ne définissez pas ces permissions, les utilisateurs ordinaires peuvent remplacer l'exécutable par le leur et exécuter du code arbitraire avec des privilèges élevés.
1. Exécutez une [invite de commandes élevée](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7.4#with-administrative-privileges-run-as-administrator) :
1. [Enregistrez un runner](../register/_index.md).
1. Installez GitLab Runner en tant que service et démarrez-le. Vous pouvez exécuter le service en utilisant le compte système intégré (Built-in System Account) (recommandé) ou en utilisant un compte utilisateur.

   > [!note]
   > Les services Windows ne fournissent pas de sessions de bureau interactives. Pour exécuter des tests GUI ou d'automatisation du bureau, consultez [Tests GUI et sessions de bureau interactives](#gui-tests-and-interactive-desktop-sessions).

   **Exécutez le service en utilisant le compte système intégré** (dans le répertoire exemple créé à l'étape 1, `C:\GitLab-Runner`)

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe install
   .\gitlab-runner.exe start
   ```

   **Exécutez le service en utilisant le compte utilisateur** (dans le répertoire exemple créé à l'étape 1, `C:\GitLab-Runner`)

   Vous devez saisir un mot de passe valide pour le compte utilisateur actuel, car il est requis pour démarrer le service sous Windows :

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe install --user ENTER-YOUR-USERNAME --password ENTER-YOUR-PASSWORD
   .\gitlab-runner.exe start
   ```

   Consultez la [section de dépannage](#windows-troubleshooting) si vous rencontrez des erreurs lors de l'installation de GitLab Runner.

1. (Facultatif) Mettez à jour la valeur `concurrent` du runner dans `C:\GitLab-Runner\config.toml` pour autoriser plusieurs jobs simultanés, comme indiqué dans les [détails de configuration avancée](../configuration/advanced-configuration.md). De plus, vous pouvez utiliser les détails de configuration avancée pour mettre à jour votre exécuteur shell afin d'utiliser Bash ou PowerShell plutôt que Batch.

Voilà ! Le runner est installé, en cours d'exécution, et redémarre après chaque redémarrage du système. Les logs sont stockés dans le Journal des événements Windows (Windows Event Log).

## Mise à niveau {#upgrade}

1. Arrêtez le service (vous avez besoin d'une [invite de commandes élevée](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7.4#with-administrative-privileges-run-as-administrator) comme précédemment) :

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe stop
   ```

1. Téléchargez le binaire pour [x86 64 bits](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-amd64.exe), [ARM 64 bits](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-arm64.exe) ou [x86 32 bits](https://s3.dualstack.us-east-1.amazonaws.com/gitlab-runner-downloads/latest/binaries/gitlab-runner-windows-386.exe) et remplacez l'exécutable du runner. Vous pouvez télécharger un binaire pour chaque version disponible comme décrit dans [Bleeding Edge - télécharger n'importe quelle autre version taguée](bleeding-edge.md#download-any-other-tagged-release).

1. Démarrez le service :

   ```powershell
   .\gitlab-runner.exe start
   ```

## Désinstallation {#uninstall}

Depuis une [invite de commandes élevée](https://learn.microsoft.com/en-us/powershell/scripting/windows-powershell/starting-windows-powershell?view=powershell-7.4#with-administrative-privileges-run-as-administrator) :

```powershell
cd C:\GitLab-Runner
.\gitlab-runner.exe stop
.\gitlab-runner.exe uninstall
cd ..
rmdir /s GitLab-Runner
```

## Dépannage Windows {#windows-troubleshooting}

Assurez-vous de lire la section [FAQ](../faq/_index.md) qui décrit certains des problèmes les plus courants avec GitLab Runner.

Si vous rencontrez une erreur du type _The account name is invalid_, essayez :

```powershell
# Add \. before the username
.\gitlab-runner.exe install --user ".\ENTER-YOUR-USERNAME" --password "ENTER-YOUR-PASSWORD"
```

Si vous rencontrez une erreur `The service did not start due to a logon failure` lors du démarrage du service, consultez la [section FAQ](#error-the-service-did-not-start-due-to-a-logon-failure) pour savoir comment résoudre le problème.

Si vous n'avez pas de mot de passe Windows, vous ne pouvez pas démarrer le service GitLab Runner, mais vous pouvez utiliser le compte système intégré (Built-in System Account).

Pour les problèmes liés au compte système intégré, consultez [Configure the Service to Start Up with the Built-in System Account](https://learn.microsoft.com/en-us/troubleshoot/windows-server/system-management-components/service-startup-permissions#resolution-3-configure-the-service-to-start-up-with-the-built-in-system-account) sur le site de support Microsoft.

### Obtenir les logs du runner {#get-runner-logs}

Lorsque vous exécutez `.\gitlab-runner.exe install`, `gitlab-runner` est installé en tant que service Windows. Vous pouvez trouver les logs dans l'Observateur d'événements avec le nom de fournisseur `gitlab-runner`.

Si vous n'avez pas accès à l'interface graphique, dans PowerShell, vous pouvez exécuter [`Get-WinEvent`](https://learn.microsoft.com/en-us/powershell/module/microsoft.powershell.diagnostics/get-winevent?view=powershell-7.4).

```shell
PS C:\> Get-WinEvent -ProviderName gitlab-runner

   ProviderName: gitlab-runner

TimeCreated                     Id LevelDisplayName Message
-----------                     -- ---------------- -------
2/4/2025 6:20:14 AM              1 Information      [session_server].listen_address not defined, session endpoints disabled  builds=0...
2/4/2025 6:20:14 AM              1 Information      listen_address not defined, metrics & debug endpoints disabled  builds=0...
2/4/2025 6:20:14 AM              1 Information      Configuration loaded                                builds=0...
2/4/2025 6:20:14 AM              1 Information      Starting multi-runner from C:\config.toml...        builds=0...
```

### Tests GUI et sessions de bureau interactives {#gui-tests-and-interactive-desktop-sessions}

Les outils de test GUI Windows (comme Ranorex et les frameworks d'automatisation du bureau) nécessitent une session utilisateur interactive avec accès au bureau visible. Il s'agit d'une limitation connue de la plateforme. Pour plus de détails, consultez le [ticket 1046](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1046).

Lorsque GitLab Runner s'exécute uniquement en tant que service Windows :

- Les jobs s'exécutent dans une session non interactive.
- Les jobs ne peuvent pas accéder au bureau visible.
- Les tests GUI échouent ou se bloquent.

Pour exécuter des tests GUI ou d'automatisation du bureau :

1. Utilisez l'exécuteur `shell`.

   Les exécuteurs Docker et Kubernetes sur Windows ne fournissent pas de session de bureau interactive.

1. Connectez-vous à Windows avec le compte utilisateur pour la session interactive.
1. Démarrez GitLab Runner en tant que processus en avant-plan dans cette session au lieu d'utiliser le service :

   ```powershell
   cd C:\GitLab-Runner
   .\gitlab-runner.exe run
   ```

1. Maintenez la session utilisateur active aussi longtemps que les tests GUI s'exécutent.
1. Utilisez des tags dans votre fichier `.gitlab-ci.yml` pour envoyer les jobs de test GUI à ce runner :

   ```yaml
   gui_tests:
     stage: test
     tags:
       - windows-gui
     script:
       - .\run-gui-tests.ps1
   ```

Les runners Windows à mise à l'échelle automatique ou éphémères ne peuvent pas exécuter des tests GUI car ils ne prennent pas en charge les sessions de bureau interactives. Chaque job s'exécute sur une VM fraîchement provisionnée sans utilisateur connecté, donc il n'y a pas de bureau visible que l'automatisation GUI peut cibler.

### J'obtiens une `PathTooLongException` lors de mes builds sur Windows {#i-get-a-pathtoolongexception-during-my-builds-on-windows}

Cette erreur est causée par des outils comme `npm` qui génèrent parfois des structures de répertoires avec des chemins de plus de 260 caractères. Pour résoudre le problème, adoptez l'une des solutions suivantes.

- Utilisez Git avec `core.longpaths` activé :

  Vous pouvez éviter le problème en utilisant Git pour nettoyer votre structure de répertoires.

  1. Exécutez `git config --system core.longpaths true` depuis la ligne de commande.
  1. Configurez votre projet pour utiliser `git fetch` depuis la page des paramètres du projet GitLab CI.

- Utilisez les outils NTFSSecurity pour PowerShell :

  Le module PowerShell [NTFSSecurity](https://github.com/raandree/NTFSSecurity) fournit une méthode `Remove-Item2` qui prend en charge les chemins longs. GitLab Runner le détecte s'il est disponible et l'utilise automatiquement.

> Une régression introduite dans GitLab Runner 16.9.1 est corrigée dans GitLab Runner 17.10.0. Si vous avez l'intention d'utiliser les versions de GitLab Runner avec des régressions, utilisez l'une des solutions de contournement suivantes :
>
> - Utilisez `pre_get_sources_script` pour réactiver les paramètres Git au niveau du système (en désactivant `Git_CONFIG_NOSYSTEM`). Cette action active `core.longpaths` par défaut sur Windows.
>
>   ```yaml
>   build:
>     hooks:
>       pre_get_sources_script:
>         - $env:GIT_CONFIG_NOSYSTEM=''
>   ```
>
> - Créez une image `GitLab-runner-helper` personnalisée :
>
>   ```dockerfile
>   FROM registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper:x86_64-v17.8.3-servercore21H2
>   ENV GIT_CONFIG_NOSYSTEM=
>   ```

### Erreur avec les scripts batch Windows : `The system cannot find the batch label specified - buildscript` {#error-with-windows-batch-scripts-the-system-cannot-find-the-batch-label-specified---buildscript}

Vous devez faire précéder `call` de votre ligne de fichier Batch dans `.gitlab-ci.yml` afin qu'il ressemble à `call C:\path\to\test.bat`. Par exemple :

```yaml
before_script:
  - call C:\path\to\test.bat
```

Pour plus d'informations, consultez le [ticket 1025](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/1025).

### Comment obtenir une sortie en couleur sur le terminal web ? {#how-can-i-get-colored-output-on-the-web-terminal}

**Réponse courte** :

Assurez-vous d'avoir les codes couleur ANSI dans la sortie de votre programme. À des fins de formatage de texte, supposez que vous exécutez dans un émulateur de terminal UNIX ANSI (car c'est la sortie de l'interface web).

**Réponse longue** :

L'interface web de GitLab CI émule un terminal UNIX ANSI (au moins partiellement). `gitlab-runner` achemine toute sortie du build directement vers l'interface web. Cela signifie que tous les codes couleur ANSI présents sont respectés.

Les anciennes versions du terminal d'invite de commandes Windows (antérieures à Windows 10, version 1511) ne prennent pas en charge les codes couleur ANSI. Elles utilisent à la place des appels win32 ([`ANSI.SYS`](https://en.wikipedia.org/wiki/ANSI.SYS)) qui ne sont **pas** présents dans la chaîne à afficher. Lors de l'écriture de programmes multiplateformes, les développeurs utilisent généralement les codes couleur ANSI par défaut. Ces codes sont convertis en appels win32 lors de l'exécution sur un système Windows, par exemple [Colorama](https://pypi.org/project/colorama/).

Si votre programme effectue ce qui précède, vous devez désactiver cette conversion pour les builds CI afin que les codes ANSI restent dans la chaîne.

Pour plus d'informations, consultez la [documentation YAML de GitLab CI](https://docs.gitlab.com/ci/yaml/script/#add-color-codes-to-script-output) pour un exemple utilisant PowerShell et le [ticket 332](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/332).

### Erreur : `The service did not start due to a logon failure` {#error-the-service-did-not-start-due-to-a-logon-failure}

Lors de l'installation et du démarrage du service GitLab Runner sur Windows, vous pouvez rencontrer cette erreur :

```shell
gitlab-runner install --password WINDOWS_MACHINE_PASSWORD
gitlab-runner start
FATA[0000] Failed to start GitLab Runner: The service did not start due to a logon failure.
```

Cette erreur peut se produire lorsque l'utilisateur utilisé pour exécuter le service ne dispose pas de la permission `SeServiceLogonRight`. Dans ce cas, vous devez ajouter cette permission pour l'utilisateur choisi, puis essayer de redémarrer le service.

1. Accédez à **Panneau de configuration > Système et sécurité > Outils d’administration**.
1. Ouvrez l'outil **Stratégie de sécurité locale**.
1. Sélectionnez **Paramètres de sécurité > Stratégies locales > Attribution des droits aux utilisateurs** dans la liste à gauche.
1. Ouvrez **Ouvrir une session comme un service** dans la liste à droite.
1. Sélectionnez **Ajouter un utilisateur ou un groupe...**.
1. Ajoutez l'utilisateur (« manuellement » ou en utilisant **Avancé...**) et appliquez les paramètres.

Selon la [documentation Microsoft](https://learn.microsoft.com/en-us/previous-versions/windows/it-pro/windows-server-2012-R2-and-2012/dn221981(v=ws.11)), cela devrait fonctionner pour :

- Windows Vista
- Windows Server 2008
- Windows 7
- Windows 8.1
- Windows Server 2008 R2
- Windows Server 2012 R2
- Windows Server 2012
- Windows 8

L'outil Stratégie de sécurité locale peut ne pas être disponible dans certaines versions de Windows, par exemple dans la variante « Famille » de chaque version.

Après avoir ajouté `SeServiceLogonRight` pour l'utilisateur utilisé dans la configuration du service, la commande `gitlab-runner start` devrait se terminer sans échec et le service devrait démarrer correctement.

### Job marqué comme réussi ou échoué de manière incorrecte {#job-marked-as-success-or-failed-incorrectly}

La plupart des programmes Windows produisent `exit code 0` en cas de succès. Cependant, certains programmes ne renvoient pas de code de sortie ou ont une valeur différente pour indiquer le succès. L'outil Windows `robocopy` en est un exemple. Le fichier `.gitlab-ci.yml` suivant échoue, même s'il devrait réussir, en raison du code de sortie produit par `robocopy` :

```yaml
test:
  stage: test
  script:
    - New-Item -type Directory -Path ./source
    - New-Item -type Directory -Path ./dest
    - Write-Output "Hello World!" > ./source/file.txt
    - robocopy ./source ./dest
  tags:
    - windows
```

Dans le cas ci-dessus, vous devez ajouter manuellement une vérification du code de sortie dans `script:`. Par exemple, vous pouvez créer un script PowerShell :

```powershell
$exitCodes = 0,1

robocopy ./source ./dest

if ( $exitCodes.Contains($LastExitCode) ) {
    exit 0
} else {
    exit 1
}
```

Et modifiez le fichier `.gitlab-ci.yml` en :

```yaml
test:
  stage: test
  script:
    - New-Item -type Directory -Path ./source
    - New-Item -type Directory -Path ./dest
    - Write-Output "Hello World!" > ./source/file.txt
    - ./robocopyCommand.ps1
  tags:
    - windows
```

De plus, faites attention à la différence entre `return` et `exit` lors de l'utilisation de fonctions PowerShell. Alors que `exit 1` marque un job comme échoué, `return 1` ne le fait pas.

### Job marqué comme réussi et interrompu en cours d'exécution avec l'exécuteur Kubernetes {#job-marked-as-success-and-terminated-midway-using-kubernetes-executor}

Pour plus d'informations, consultez [Exécution des jobs](../executors/kubernetes/_index.md#job-execution).

### Exécuteur Docker : `unsupported Windows Version` {#docker-executor-unsupported-windows-version}

GitLab Runner vérifie la version de Windows Server pour s'assurer qu'elle est prise en charge.

Il effectue cette vérification en exécutant `docker info`.

Si GitLab Runner ne parvient pas à démarrer et affiche une erreur sans spécifier de version de Windows Server, la version de Docker est peut-être obsolète.

```plaintext
Preparation failed: detecting base image: unsupported Windows Version: Windows Server Datacenter
```

L'erreur doit contenir des informations détaillées sur la version de Windows Server, qui sont ensuite comparées aux versions prises en charge par GitLab Runner.

```plaintext
unsupported Windows Version: Windows Server Datacenter Version (OS Build 18363.720)
```

Docker 17.06.2 sur Windows Server renvoie ce qui suit dans la sortie de `docker info`.

```plaintext
Operating System: Windows Server Datacenter
```

Dans ce cas, la solution consiste à mettre à niveau la version de Docker vers une version de date similaire ou ultérieure à celle de la release de Windows Server.

### Exécuteur Kubernetes : `unsupported Windows Version` {#kubernetes-executor-unsupported-windows-version}

L'exécuteur Kubernetes sur Windows peut échouer avec l'erreur suivante :

```plaintext
Using Kubernetes namespace: gitlab-runner
ERROR: Preparation failed: prepare helper image: detecting base image: unsupported Windows Version:
Will be retried in 3s ...
ERROR: Job failed (system failure): prepare helper image: detecting base image: unsupported Windows Version:
```

Pour corriger cela, ajoutez le sélecteur de nœud `node.kubernetes.io/windows-build` dans la section `[runners.kubernetes.node_selector]` de votre fichier de configuration GitLab Runner, par exemple :

```toml
   [runners.kubernetes.node_selector]
     "kubernetes.io/arch" = "amd64"
     "kubernetes.io/os" = "windows"
     "node.kubernetes.io/windows-build" = "10.0.17763"
```

### J'utilise un lecteur réseau mappé et mon build ne trouve pas le chemin correct {#im-using-a-mapped-network-drive-and-my-build-cannot-find-the-correct-path}

Lorsque GitLab Runner s'exécute sous un compte utilisateur standard au lieu d'un compte administrateur, il ne peut pas accéder aux lecteurs réseau mappés. Lorsque vous essayez d'utiliser des lecteurs réseau mappés, vous obtenez l'erreur `The system cannot find the path specified.` Cette erreur se produit car les sessions d'ouverture de session de service ont des [limitations de sécurité](https://learn.microsoft.com/en-us/windows/win32/services/services-and-redirected-drives) lors de l'accès aux ressources. Utilisez plutôt le [chemin UNC](https://learn.microsoft.com/en-us/dotnet/standard/io/file-path-formats#unc-paths) de votre lecteur.

### Le conteneur de build ne peut pas se connecter aux conteneurs de service {#the-build-container-is-unable-to-connect-to-service-containers}

Pour utiliser des services avec des conteneurs Windows :

- Utilisez le mode réseau qui [crée un réseau pour chaque job](../executors/docker.md#create-a-network-for-each-job).
- Assurez-vous que le feature flag `FF_NETWORK_PER_BUILD` est activé.

### Le job ne peut pas créer de répertoire de build et échoue avec une erreur {#the-job-cannot-create-a-build-directory-and-fails-with-an-error}

Lorsque vous utilisez `GitLab-Runner` avec l'exécuteur `Docker-Windows`, un job peut échouer avec une erreur du type :

```shell
fatal: cannot chdir to c:/builds/gitlab/test: Permission denied`
```

Lorsque cette erreur se produit, assurez-vous que l'utilisateur sous lequel le moteur Docker s'exécute dispose des permissions complètes sur `C:\Program Data\Docker`. Le moteur Docker doit pouvoir écrire dans ce répertoire pour certaines actions, et sans les permissions correctes, il échoue.

[En savoir plus sur la configuration du moteur Docker sur Windows](https://learn.microsoft.com/en-us/virtualization/windowscontainers/manage-docker/configure-docker-daemon).

### Lignes vides pour la sortie STDOUT du Sous-système Windows pour Linux (WSL) dans les job logs {#blank-lines-for-windows-subsystem-for-linux-wsl-stdout-output-in-job-logs}

Par défaut, la sortie STDOUT du Sous-système Windows pour Linux (WSL) n'est pas encodée en UTF8 et s'affiche sous forme de lignes vides dans les logs de job. Pour afficher la sortie STDOUT, vous pouvez forcer l'encodage UTF8 pour WSL en définissant la variable d'environnement `WSL_UTF8`.

```yaml
job:
  variables:
    WSL_UTF8: "1"
```

### La résolution d'affichage est limitée à 1024x768 {#display-resolution-is-limited-to-1024x768}

Lorsque vous exécutez des jobs CI/CD sur Windows avec GitLab Runner en tant que service système, la résolution d'affichage est limitée à 1024x768. Ce problème est dû à l'isolation de la session 0 de Windows. Pour plus d'informations, consultez [Session 0 Isolation](https://learn.microsoft.com/en-us/previous-versions/bb756986(v=msdn.10)?redirectedfrom=MSDN).

Pour vérifier la session et la résolution d'affichage, exécutez le script PowerShell suivant dans un job :

```powershell
echo "Current session:"
[System.Diagnostics.Process]::GetCurrentProcess().SessionId

Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.Screen]::AllScreens
```

Voici la sortie du script lors de l'exécution dans la session isolée 0 :

```plaintext
Current session:
0
BitsPerPixel : 0
Bounds       : {X=0,Y=0,Width=1024,Height=768}
DeviceName   : WinDisc
Primary      : True
WorkingArea  : {X=0,Y=0,Width=1024,Height=768}
```
