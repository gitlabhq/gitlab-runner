---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: VirtualBox
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

> [!note]
> L'exécuteur Parallels fonctionne de la même manière que l'exécuteur VirtualBox. Le cache local n'est pas pris en charge. Le [cache distribué](../configuration/speed_up_job_execution.md) est pris en charge.

VirtualBox vous permet d'utiliser la virtualisation de VirtualBox pour fournir un environnement de build propre pour chaque build. Cet exécuteur prend en charge tous les systèmes pouvant être exécutés sur VirtualBox. La seule exigence est que la machine virtuelle expose un serveur SSH et fournisse un shell compatible avec Bash ou PowerShell.

> [!note]
> Assurez-vous de satisfaire les [prérequis communs](_index.md#git-requirements-for-non-docker-executors) sur toute machine virtuelle où GitLab Runner utilise l'exécuteur VirtualBox.

## Présentation {#overview}

Le code source du projet est extrait vers : `~/builds/<namespace>/<project-name>`.

Où :

- `<namespace>` est l'espace de nommage dans lequel le projet est stocké sur GitLab
- `<project-name>` est le nom du projet tel qu'il est stocké sur GitLab

Pour remplacer le répertoire `~/builds`, spécifiez l'option `builds_dir` dans la section `[[runners]]` de [`config.toml`](../configuration/advanced-configuration.md).

Vous pouvez également définir des [répertoires de build personnalisés](https://docs.gitlab.com/ci/runners/configure_runners/#custom-build-directories) par job en utilisant `GIT_CLONE_PATH`.

## Créer une nouvelle machine virtuelle de base {#create-a-new-base-virtual-machine}

1. Installez [VirtualBox](https://www.virtualbox.org).
   - Si vous exécutez depuis Windows et que VirtualBox est installé à l'emplacement par défaut (par exemple `%PROGRAMFILES%\Oracle\VirtualBox`), GitLab Runner le détecte automatiquement. Sinon, vous devez ajouter le dossier d'installation à la variable d'environnement `PATH` du processus `gitlab-runner`.
1. Importez ou créez une nouvelle machine virtuelle dans VirtualBox
1. Configurez l'adaptateur réseau 1 en mode « NAT » (c'est actuellement le seul moyen pour GitLab Runner de se connecter via SSH à l'invité)
1. (facultatif) Configurez un autre adaptateur réseau en mode « Bridged networking » pour obtenir un accès à Internet depuis l'invité (par exemple)
1. Connectez-vous à la nouvelle machine virtuelle
1. S'il s'agit d'une VM Windows, consultez [Liste de contrôle pour les VM Windows](#checklist-for-windows-vms)
1. Installez le serveur OpenSSH
1. Installez toutes les autres dépendances requises par votre build
1. Si vous souhaitez télécharger ou charger des artefacts de job, installez `gitlab-runner` dans la VM
1. Déconnectez-vous et arrêtez la machine virtuelle

Il est tout à fait possible d'utiliser des outils d'automatisation comme Vagrant pour provisionner la machine virtuelle.

## Créer un nouveau runner {#create-a-new-runner}

1. Installez GitLab Runner sur l'hôte exécutant VirtualBox
1. Enregistrez un nouveau runner avec `gitlab-runner register`
1. Sélectionnez l'exécuteur `virtualbox`
1. Saisissez le nom de la machine virtuelle de base que vous avez créée précédemment (vous le trouverez dans les paramètres de la machine virtuelle **Général > Basic > Nom**)
1. Saisissez les paramètres SSH `user` et `password` ou le chemin vers `identity_file` de la machine virtuelle

## Fonctionnement {#how-it-works}

Lorsqu'un nouveau build est démarré :

1. Un nom unique est généré pour la machine virtuelle : `runner-<short-token>-concurrent-<id>`
1. La machine virtuelle est clonée si elle n'existe pas
1. Les règles de redirection de port sont créées pour accéder au serveur SSH
1. GitLab Runner démarre ou restaure le snapshot de la machine virtuelle
1. GitLab Runner attend que le serveur SSH soit accessible
1. GitLab Runner crée un snapshot de la machine virtuelle en cours d'exécution (ceci est fait pour accélérer les builds suivants)
1. GitLab Runner se connecte à la machine virtuelle et exécute un build
1. Si activé, le chargement des artefacts est effectué à l'aide du binaire `gitlab-runner` *à l'intérieur* de la machine virtuelle.
1. GitLab Runner arrête ou éteint la machine virtuelle

## Liste de contrôle pour les VM Windows {#checklist-for-windows-vms}

Pour utiliser VirtualBox avec Windows, vous pouvez installer Cygwin ou PowerShell.

### Utiliser Cygwin {#use-cygwin}

- Installez [Cygwin](https://cygwin.com/)
- Installez `sshd` et Git depuis Cygwin (n'utilisez pas *Git for Windows*, vous aurez de nombreux problèmes de chemins !)
- Installez Git LFS
- Configurez `sshd` et installez-le en tant que service (voir le [wiki Cygwin](https://cygwin.fandom.com/wiki/Sshd))
- Créez une règle dans le pare-feu Windows pour autoriser le trafic TCP entrant sur le port 22
- Ajoutez les serveurs GitLab à `~/.ssh/known_hosts`
- Pour convertir les chemins entre Cygwin et Windows, utilisez [l'utilitaire `cygpath`](https://cygwin.fandom.com/wiki/Cygpath_utility)

### Utiliser OpenSSH natif et PowerShell {#use-native-openssh-and-powershell}

- Installez [PowerShell](https://learn.microsoft.com/en-us/powershell/scripting/install/install-powershell-on-windows?view=powershell-7.4)
- Installez et configurez [OpenSSH](https://learn.microsoft.com/en-us/windows-server/administration/openssh/openssh_install_firstuse?tabs=powershell#install-openssh-for-windows)
- Installez [Git for Windows](https://git-scm.com/)
- Configurez le [shell par défaut en tant que `pwsh`](https://learn.microsoft.com/en-us/windows-server/administration/OpenSSH/openssh-server-configuration#configuring-the-default-shell-for-openssh-in-windows). Mettez à jour l'exemple avec le chemin complet correct :

  ```powershell
  New-ItemProperty -Path "HKLM:\SOFTWARE\OpenSSH" -Name DefaultShell -Value "$PSHOME\pwsh.exe" -PropertyType String -Force
  ```

- Ajoutez le shell `pwsh` à [`config.toml`](../configuration/advanced-configuration.md)
