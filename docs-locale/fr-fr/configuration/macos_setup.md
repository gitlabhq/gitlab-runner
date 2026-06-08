---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Configurer des runners macOS
---

Pour exécuter un job CI/CD sur un runner macOS, effectuez les étapes suivantes dans l'ordre.

Une fois que vous avez terminé, GitLab Runner s'exécutera sur une machine macOS et un runner individuel sera prêt à traiter des jobs.

- Changer le shell système en Bash.
- Installer Homebrew, rbenv et GitLab Runner.
- Configurer rbenv et installer Ruby.
- Installer Xcode.
- Enregistrer un runner.
- Configurer CI/CD.

## Prérequis {#prerequisites}

Avant de commencer :

- Installez une version récente de macOS. Ce guide a été élaboré sur la version 11.4.
- Assurez-vous d'avoir un accès terminal ou SSH à la machine.

## Changer le shell système en Bash {#change-the-system-shell-to-bash}

Les versions plus récentes de macOS utilisent Zsh comme shell par défaut. Cependant, l'exécuteur shell du runner nécessite Bash pour garantir que les scripts CI/CD s'exécutent correctement, car beaucoup utilisent une syntaxe et des fonctionnalités spécifiques à Bash.

1. Connectez-vous à votre machine et déterminez le shell par défaut :

   ```shell
   echo $SHELL
   ```

1. Si le résultat n'est pas `/bin/bash`, changez le shell en exécutant :

   ```shell
   chsh -s /bin/bash
   ```

1. Saisissez votre mot de passe.
1. Redémarrez votre terminal ou reconnectez-vous via SSH.
1. Exécutez à nouveau `echo $SHELL`. Le résultat devrait être `/bin/bash`.

## Installer Homebrew, rbenv et GitLab Runner {#install-homebrew-rbenv-and-gitlab-runner}

Le runner a besoin de certaines options d'environnement pour se connecter à la machine et exécuter un job.

1. Installez le [gestionnaire de paquets Homebrew](https://brew.sh/) :

   ```shell
   /bin/bash -c "$(curl "https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh")"
   ```

1. Configurez [`rbenv`](https://github.com/rbenv/rbenv), qui est un gestionnaire de versions Ruby, ainsi que GitLab Runner :

   ```shell
   brew install rbenv gitlab-runner
   brew services start gitlab-runner
   ```

## Configurer rbenv et installer Ruby {#configure-rbenv-and-install-ruby}

Maintenant, configurez rbenv et installez Ruby.

1. Ajoutez rbenv à l'environnement Bash :

   ```shell
   echo 'if which rbenv > /dev/null; then eval "$(rbenv init -)"; fi' >> ~/.bash_profile
   source ~/.bash_profile
   ```

1. Installez Ruby 3.3.x et définissez-le comme valeur par défaut globale de la machine :

   ```shell
   rbenv install 3.3.4
   rbenv global 3.3.4
   ```

## Installer Xcode {#install-xcode}

Maintenant, installez et configurez Xcode.

1. Accédez à l'un de ces emplacements et installez Xcode :

   - L'Apple App Store.
   - Le [portail développeur Apple](https://developer.apple.com/).
   - [`xcode-install`](https://github.com/xcpretty/xcode-install). Ce projet vise à faciliter le téléchargement de diverses dépendances Apple depuis la ligne de commande.

1. Acceptez la licence et installez les composants supplémentaires recommandés. Vous pouvez le faire en ouvrant Xcode et en suivant les invites, ou en exécutant la commande suivante dans le terminal :

   ```shell
   sudo xcodebuild -runFirstLaunch
   ```

1. Mettez à jour le répertoire développeur actif pour que Xcode charge les outils de ligne de commande appropriés lors de votre build :

   ```shell
   sudo xcode-select -s /Applications/Xcode.app/Contents/Developer
   ```

### Créer et enregistrer un runner de projet {#create-and-register-a-project-runner}

Maintenant, [créez et enregistrez](https://docs.gitlab.com/ci/runners/runners_scope/#create-a-project-runner-with-a-runner-authentication-token) un runner de projet.

Lorsque vous créez et enregistrez le runner :

- Dans GitLab, ajoutez le tag `macos` pour vous assurer que les jobs macOS s'exécutent sur cette machine macOS.
- Dans la ligne de commande, sélectionnez `shell` comme [exécuteur](../executors/_index.md).

Après avoir enregistré le runner, un message de succès s'affiche dans la ligne de commande :

```shell
Runner registered successfully. Feel free to start it, but if it's running already the config should be automatically reloaded!
```

Pour afficher le runner :

1. Dans la barre supérieure, sélectionnez **Rechercher ou aller à** et trouvez votre projet ou groupe.
1. Sélectionnez **Paramètres > CI/CD**.
1. Développez **Runners**.

### Configurer CI/CD {#configure-cicd}

Dans votre projet GitLab, configurez CI/CD et lancez un build. Vous pouvez utiliser cet exemple de fichier `.gitlab-ci.yml`. Notez que les tags correspondent aux tags que vous avez utilisés pour enregistrer le runner.

```yaml
stages:
  - build
  - test

variables:
  LANG: "en_US.UTF-8"

before_script:
  - gem install bundler
  - bundle install
  - gem install cocoapods
  - pod install

build:
  stage: build
  script:
    - bundle exec fastlane build
  tags:
    - macos

test:
  stage: test
  script:
    - bundle exec fastlane test
  tags:
    - macos
```

Le runner macOS devrait maintenant builder votre projet.
