---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Types de shells pris en charge par GitLab Runner
---

{{< details >}}

- Niveau :  Free, Premium, Ultimate
- Offre :  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner implémente des générateurs de scripts shell qui permettent d'exécuter des builds sur différents systèmes.

Les scripts shell contiennent des commandes pour exécuter toutes les étapes du build :

1. `git clone`
1. Restaurer le cache du build
1. Commandes de build
1. Mettre à jour le cache du build
1. Générer et téléverser les artefacts du build

Les shells n'ont aucune option de configuration. Les étapes du build sont reçues des commandes définies dans la [`script` directive dans `.gitlab-ci.yml`](https://docs.gitlab.com/ci/yaml/#script).

Les shells pris en charge sont :

| Shell        | Statut          | Description |
|--------------|-----------------|-------------|
| `bash`       | Entièrement pris en charge | Bash (Bourne Again Shell). Toutes les commandes sont exécutées dans le contexte Bash (par défaut pour tous les systèmes Unix). |
| `sh`         | Entièrement pris en charge | Sh (Bourne shell). Toutes les commandes sont exécutées dans le contexte Sh (alternative à `bash` pour tous les systèmes Unix). |
| `powershell` | Entièrement pris en charge | Script PowerShell. Toutes les commandes sont exécutées dans le contexte PowerShell Desktop. Shell par défaut pour les jobs sur Windows avec les exécuteurs `kubernetes` et `docker-windows`. |
| `pwsh`       | Entièrement pris en charge | Script PowerShell. Toutes les commandes sont exécutées dans le contexte PowerShell Core. Shell par défaut pour les nouveaux enregistrements de runner sur Windows, et pour les jobs avec l'exécuteur `shell`. |

Si vous souhaitez sélectionner un shell particulier autre que celui par défaut, vous devez [spécifier le shell](../executors/shell.md#selecting-your-shell) dans votre fichier `config.toml`.

## Shells Sh/Bash {#shbash-shells}

Sh/Bash est le shell par défaut utilisé sur tous les systèmes Unix. Le script bash utilisé dans `.gitlab-ci.yml` est exécuté en acheminant le script shell vers l'une des commandes suivantes :

```shell
# This command is used if the build should be executed in context
# of another user (the shell executor)
cat generated-bash-script | su --shell /bin/bash --login user

# This command is used if the build should be executed using
# the current user, but in a login environment
cat generated-bash-script | /bin/bash --login

# This command is used if the build should be executed in
# a Docker environment
cat generated-bash-script | /bin/bash
```

### Chargement du profil shell {#shell-profile-loading}

Pour certains exécuteurs, le runner passe le drapeau `--login` comme indiqué ci-dessus, ce qui charge également le profil shell. Tout ce qui se trouve dans votre `.bashrc`, `.bash_logout`, [ou tout autre dotfile](https://tldp.org/LDP/Bash-Beginners-Guide/html/sect_03_01.html#sect_03_01_02), est exécuté dans votre job.

Si un [job échoue à l'étape `Prepare environment`](../faq/_index.md#job-failed-system-failure-preparing-environment), il est probable que quelque chose dans le profil shell est à l'origine de l'échec. Un échec courant se produit lorsqu'il existe un fichier `.bash_logout` qui tente d'effacer la console.

Pour résoudre cette erreur, vérifiez `/home/gitlab-runner/.bash_logout`. Par exemple, si le fichier `.bash_logout` contient une section de script comme celle qui suit, commentez-la et redémarrez le pipeline :

```shell
if [ "$SHLVL" = 1 ]; then
    [ -x /usr/bin/clear_console ] && /usr/bin/clear_console -q
fi
```

Exécuteurs qui chargent les profils shell :

- [`shell`](../executors/shell.md)
- [`parallels`](../executors/parallels.md) (Le profil shell de la machine virtuelle cible est chargé)
- [`virtualbox`](../executors/virtualbox.md) (Le profil shell de la machine virtuelle cible est chargé)
- [`ssh`](../executors/ssh.md) (Le profil shell de la machine cible est chargé)

## PowerShell {#powershell}

PowerShell Core est le shell par défaut pour les nouveaux enregistrements de runner sur Windows. Cependant, cette valeur par défaut d'inscription s'applique uniquement lorsque vous définissez explicitement une valeur `shell` dans `config.toml`. Lorsqu'aucun `shell` n'est configuré :

- Les exécuteurs `docker-windows` et `kubernetes` utilisent PowerShell Desktop par défaut à l'exécution.
- L'exécuteur `shell` utilise PowerShell Core par défaut.

PowerShell ne prend pas en charge l'exécution du build dans le contexte d'un autre utilisateur.

Le script PowerShell généré est exécuté en enregistrant son contenu dans un fichier et en transmettant le nom du fichier à la commande suivante :

- Pour PowerShell Desktop Edition :

  ```batch
  powershell -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command generated-windows-powershell.ps1
  ```

- Pour PowerShell Core Edition :

  ```batch
  pwsh -NoProfile -NonInteractive -ExecutionPolicy Bypass -Command generated-windows-powershell.ps1
  ```

Voici un exemple de script PowerShell :

```powershell
$ErrorActionPreference = "Continue" # This will be set to 'Stop' when targetting PowerShell Core

echo "Running on $([Environment]::MachineName)..."

& {
  $CI="true"
  $env:CI=$CI
  $CI_COMMIT_SHA="db45ad9af9d7af5e61b829442fd893d96e31250c"
  $env:CI_COMMIT_SHA=$CI_COMMIT_SHA
  $CI_COMMIT_BEFORE_SHA="d63117656af6ff57d99e50cc270f854691f335ad"
  $env:CI_COMMIT_BEFORE_SHA=$CI_COMMIT_BEFORE_SHA
  $CI_COMMIT_REF_NAME="main"
  $env:CI_COMMIT_REF_NAME=$CI_COMMIT_REF_NAME
  $CI_JOB_ID="1"
  $env:CI_JOB_ID=$CI_JOB_ID
  $CI_REPOSITORY_URL="Z:\Gitlab\tests\test"
  $env:CI_REPOSITORY_URL=$CI_REPOSITORY_URL
  $CI_PROJECT_ID="1"
  $env:CI_PROJECT_ID=$CI_PROJECT_ID
  $CI_PROJECT_DIR="Z:\Gitlab\tests\test\builds\0\project-1"
  $env:CI_PROJECT_DIR=$CI_PROJECT_DIR
  $CI_SERVER="yes"
  $env:CI_SERVER=$CI_SERVER
  $CI_SERVER_NAME="GitLab CI"
  $env:CI_SERVER_NAME=$CI_SERVER_NAME
  $CI_SERVER_VERSION=""
  $env:CI_SERVER_VERSION=$CI_SERVER_VERSION
  $CI_SERVER_REVISION=""
  $env:CI_SERVER_REVISION=$CI_SERVER_REVISION
  $GITLAB_CI="true"
  $env:GITLAB_CI=$GITLAB_CI
  $GIT_SSL_CAINFO=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $GIT_SSL_CAINFO | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $GIT_SSL_CAINFO="C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $env:GIT_SSL_CAINFO=$GIT_SSL_CAINFO
  $CI_SERVER_TLS_CA_FILE=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $CI_SERVER_TLS_CA_FILE | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $CI_SERVER_TLS_CA_FILE="C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $env:CI_SERVER_TLS_CA_FILE=$CI_SERVER_TLS_CA_FILE
  echo "Cloning repository..."
  if( (Get-Command -Name Remove-Item2 -Module NTFSSecurity -ErrorAction SilentlyContinue) -and (Test-Path "C:\GitLab-Runner\builds\0\project-1" -PathType Container) ) {
    Remove-Item2 -Force -Recurse "C:\GitLab-Runner\builds\0\project-1"
  } elseif(Test-Path "C:\GitLab-Runner\builds\0\project-1") {
    Remove-Item -Force -Recurse "C:\GitLab-Runner\builds\0\project-1"
  }

  & "git" "clone" "https://gitlab.com/group/project.git" "Z:\Gitlab\tests\test\builds\0\project-1"
  if(!$?) { Exit $LASTEXITCODE }

  cd "C:\GitLab-Runner\builds\0\project-1"
  if(!$?) { Exit $LASTEXITCODE }

  echo "Checking out db45ad9a as main..."
  & "git" "checkout" "db45ad9af9d7af5e61b829442fd893d96e31250c"
  if(!$?) { Exit $LASTEXITCODE }

  if(Test-Path "..\..\..\cache\project-1\pages\main\cache.tgz" -PathType Leaf) {
    echo "Restoring cache..."
    & "gitlab-runner-windows-amd64.exe" "extract" "--file" "..\..\..\cache\project-1\pages\main\cache.tgz"
    if(!$?) { Exit $LASTEXITCODE }

  } else {
    if(Test-Path "..\..\..\cache\project-1\pages\main\cache.tgz" -PathType Leaf) {
      echo "Restoring cache..."
      & "gitlab-runner-windows-amd64.exe" "extract" "--file" "..\..\..\cache\project-1\pages\main\cache.tgz"
      if(!$?) { Exit $LASTEXITCODE }

    }
  }
}
if(!$?) { Exit $LASTEXITCODE }

& {
  $CI="true"
  $env:CI=$CI
  $CI_COMMIT_SHA="db45ad9af9d7af5e61b829442fd893d96e31250c"
  $env:CI_COMMIT_SHA=$CI_COMMIT_SHA
  $CI_COMMIT_BEFORE_SHA="d63117656af6ff57d99e50cc270f854691f335ad"
  $env:CI_COMMIT_BEFORE_SHA=$CI_COMMIT_BEFORE_SHA
  $CI_COMMIT_REF_NAME="main"
  $env:CI_COMMIT_REF_NAME=$CI_COMMIT_REF_NAME
  $CI_JOB_ID="1"
  $env:CI_JOB_ID=$CI_JOB_ID
  $CI_REPOSITORY_URL="Z:\Gitlab\tests\test"
  $env:CI_REPOSITORY_URL=$CI_REPOSITORY_URL
  $CI_PROJECT_ID="1"
  $env:CI_PROJECT_ID=$CI_PROJECT_ID
  $CI_PROJECT_DIR="Z:\Gitlab\tests\test\builds\0\project-1"
  $env:CI_PROJECT_DIR=$CI_PROJECT_DIR
  $CI_SERVER="yes"
  $env:CI_SERVER=$CI_SERVER
  $CI_SERVER_NAME="GitLab CI"
  $env:CI_SERVER_NAME=$CI_SERVER_NAME
  $CI_SERVER_VERSION=""
  $env:CI_SERVER_VERSION=$CI_SERVER_VERSION
  $CI_SERVER_REVISION=""
  $env:CI_SERVER_REVISION=$CI_SERVER_REVISION
  $GITLAB_CI="true"
  $env:GITLAB_CI=$GITLAB_CI
  $GIT_SSL_CAINFO=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $GIT_SSL_CAINFO | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $GIT_SSL_CAINFO="C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $env:GIT_SSL_CAINFO=$GIT_SSL_CAINFO
  $CI_SERVER_TLS_CA_FILE=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $CI_SERVER_TLS_CA_FILE | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $CI_SERVER_TLS_CA_FILE="C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $env:CI_SERVER_TLS_CA_FILE=$CI_SERVER_TLS_CA_FILE
  cd "C:\GitLab-Runner\builds\0\project-1"
  if(!$?) { Exit $LASTEXITCODE }

  echo "`$ echo true"
  echo true
}
if(!$?) { Exit $LASTEXITCODE }

& {
  $CI="true"
  $env:CI=$CI
  $CI_COMMIT_SHA="db45ad9af9d7af5e61b829442fd893d96e31250c"
  $env:CI_COMMIT_SHA=$CI_COMMIT_SHA
  $CI_COMMIT_BEFORE_SHA="d63117656af6ff57d99e50cc270f854691f335ad"
  $env:CI_COMMIT_BEFORE_SHA=$CI_COMMIT_BEFORE_SHA
  $CI_COMMIT_REF_NAME="main"
  $env:CI_COMMIT_REF_NAME=$CI_COMMIT_REF_NAME
  $CI_JOB_ID="1"
  $env:CI_JOB_ID=$CI_JOB_ID
  $CI_REPOSITORY_URL="Z:\Gitlab\tests\test"
  $env:CI_REPOSITORY_URL=$CI_REPOSITORY_URL
  $CI_PROJECT_ID="1"
  $env:CI_PROJECT_ID=$CI_PROJECT_ID
  $CI_PROJECT_DIR="Z:\Gitlab\tests\test\builds\0\project-1"
  $env:CI_PROJECT_DIR=$CI_PROJECT_DIR
  $CI_SERVER="yes"
  $env:CI_SERVER=$CI_SERVER
  $CI_SERVER_NAME="GitLab CI"
  $env:CI_SERVER_NAME=$CI_SERVER_NAME
  $CI_SERVER_VERSION=""
  $env:CI_SERVER_VERSION=$CI_SERVER_VERSION
  $CI_SERVER_REVISION=""
  $env:CI_SERVER_REVISION=$CI_SERVER_REVISION
  $GITLAB_CI="true"
  $env:GITLAB_CI=$GITLAB_CI
  $GIT_SSL_CAINFO=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $GIT_SSL_CAINFO | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $GIT_SSL_CAINFO="C:\GitLab-Runner\builds\0\project-1.tmp\GIT_SSL_CAINFO"
  $env:GIT_SSL_CAINFO=$GIT_SSL_CAINFO
  $CI_SERVER_TLS_CA_FILE=""
  New-Item -ItemType directory -Force -Path "C:\GitLab-Runner\builds\0\project-1.tmp" | out-null
  $CI_SERVER_TLS_CA_FILE | Out-File "C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $CI_SERVER_TLS_CA_FILE="C:\GitLab-Runner\builds\0\project-1.tmp\CI_SERVER_TLS_CA_FILE"
  $env:CI_SERVER_TLS_CA_FILE=$CI_SERVER_TLS_CA_FILE
  cd "C:\GitLab-Runner\builds\0\project-1"
  if(!$?) { Exit $LASTEXITCODE }

  echo "Archiving cache..."
  & "gitlab-runner-windows-amd64.exe" "archive" "--file" "..\..\..\cache\project-1\pages\main\cache.tgz" "--path" "vendor"
  if(!$?) { Exit $LASTEXITCODE }

}
if(!$?) { Exit $LASTEXITCODE }
```

### Exécution de Windows Batch {#running-windows-batch}

Vous pouvez exécuter des scripts Batch depuis PowerShell en utilisant `Start-Process
"cmd.exe" "/c C:\Path\file.bat"` pour les anciens scripts Batch non portés vers PowerShell.

### Accéder au shell `CMD` lorsque PowerShell est le shell par défaut {#access-cmd-shell-when-powershell-is-the-default}

Le projet [Call `CMD` From Default PowerShell in GitLab CI](https://gitlab.com/guided-explorations/microsoft/windows/call-cmd-from-powershell) montre comment accéder au shell `CMD`. Cette approche fonctionne lorsque PowerShell est le shell par défaut sur un runner.

### Vidéo de présentation d'exemples PowerShell fonctionnels {#video-walkthrough-of-working-powershell-examples}

La vidéo [Slicing and Dicing with PowerShell on GitLab CI](https://www.youtube.com/watch?v=UZvtAYwruFc) est une présentation du projet d'exploration guidée [PowerShell Pipelines on GitLab CI](https://gitlab.com/guided-explorations/microsoft/powershell/powershell-pipelines-on-gitlab-ci). Il a été testé sur :

- Windows PowerShell et PowerShell Core 7 sur les [runners hébergés sur Windows pour GitLab.com](https://docs.gitlab.com/ci/runners/hosted_runners/windows/).
- PowerShell Core 7 dans des conteneurs Linux avec le [runner Docker-Machine](../executors/docker_machine.md).

L'exemple peut être copié dans votre propre groupe ou instance à des fins de test. Plus de détails sur les autres patterns GitLab CI présentés sont disponibles sur la page du projet.
