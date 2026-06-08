---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Certificats auto-signés ou autorités de certification personnalisées
---

{{< details >}}

- Niveau : Free, Premium, Ultimate
- Offre : GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

GitLab Runner propose deux options pour configurer les certificats à utiliser afin de vérifier les pairs TLS :

- **Pour les connexions au serveur GitLab** : Le fichier de certificat peut être spécifié comme décrit dans la section [Options prises en charge pour les certificats auto-signés ciblant le serveur GitLab](#supported-options-for-self-signed-certificates-targeting-the-gitlab-server).

  Cela résout le problème `x509: certificate signed by unknown authority` lors de l'enregistrement d'un runner.

  Pour les runners existants, la même erreur peut apparaître dans les logs du runner lorsqu'il tente de vérifier les jobs :

  ```plaintext
  Couldn't execute POST against https://hostname.tld/api/v4/jobs/request:
  Post https://hostname.tld/api/v4/jobs/request: x509: certificate signed by unknown authority
  ```

- **Connexion à un serveur de cache ou à un magasin Git LFS externe** : Une approche plus générique qui couvre également d'autres scénarios tels que les scripts utilisateur : un certificat peut être spécifié et installé sur le conteneur comme décrit dans la section [Approuver les certificats TLS pour les exécuteurs Docker et Kubernetes](#trusting-tls-certificates-for-docker-and-kubernetes-executors).

  Exemple d'erreur de job log concernant une opération Git LFS pour laquelle un certificat est manquant :

  ```plaintext
  LFS: Get https://object.hostname.tld/lfs-dev/c8/95/a34909dce385b85cee1a943788044859d685e66c002dbf7b28e10abeef20?X-Amz-Expires=600&X-Amz-Date=20201006T043010Z&X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=svcgitlabstoragedev%2F20201006%2Fus-east-1%2Fs3%2Faws4_request&X-Amz-SignedHeaders=host&X-Amz-Signature=012211eb0ff0e374086e8c2d37556f2d8ca4cc948763e90896f8f5774a100b55: x509: certificate signed by unknown authority
  ```

## Options prises en charge pour les certificats auto-signés ciblant le serveur GitLab {#supported-options-for-self-signed-certificates-targeting-the-gitlab-server}

Cette section traite du cas où seul le serveur GitLab nécessite un certificat personnalisé. Si d'autres hôtes (par exemple, un service de stockage d'objets sans [téléchargement par proxy activé](https://docs.gitlab.com/administration/object_storage/#proxy-download) ) nécessitent également une autorité de certification (CA) personnalisée, consultez la [section suivante](#trusting-tls-certificates-for-docker-and-kubernetes-executors).

GitLab Runner prend en charge les options suivantes :

- **Par défaut : lire le certificat système** : GitLab Runner lit le magasin de certificats système et vérifie le serveur GitLab par rapport aux autorités de certification (CA) stockées dans le système.

- **Spécifier un fichier de certificat personnalisé** : GitLab Runner expose l'option `tls-ca-file` lors de l'[enregistrement](../commands/_index.md#gitlab-runner-register) (`gitlab-runner register --tls-ca-file=/path`), et dans [`config.toml`](advanced-configuration.md) sous la section `[[runners]]`. Cela vous permet de spécifier un fichier de certificat personnalisé. Ce fichier est lu à chaque fois que le runner tente d'accéder au serveur GitLab. Si vous utilisez le chart Helm de GitLab Runner, vous devez configurer les certificats comme décrit dans [Accéder à GitLab avec un certificat personnalisé](../install/kubernetes_helm_chart_configuration.md#access-gitlab-with-a-custom-certificate).

- **Lire un certificat PEM** : GitLab Runner lit le certificat PEM (**Le format DER n'est pas pris en charge**) à partir d'un fichier prédéfini :
  - `/etc/gitlab-runner/certs/gitlab.example.com.crt` sur les systèmes *nix lorsque GitLab Runner est exécuté en tant que `root`.

    Si l'adresse de votre serveur est `https://gitlab.example.com:8443/`, créez le fichier de certificat à l'emplacement suivant : `/etc/gitlab-runner/certs/gitlab.example.com.crt`.

    Vous pouvez utiliser le client `openssl` pour télécharger le certificat de l'instance GitLab vers `/etc/gitlab-runner/certs` :

    ```shell
    openssl s_client -showcerts -connect gitlab.example.com:443 -servername gitlab.example.com < /dev/null 2>/dev/null | openssl x509 -outform PEM > /etc/gitlab-runner/certs/gitlab.example.com.crt
    ```

    Pour vérifier que le fichier est correctement installé, vous pouvez utiliser un outil tel que `openssl`. Par exemple :

    ```shell
    echo | openssl s_client -CAfile /etc/gitlab-runner/certs/gitlab.example.com.crt -connect gitlab.example.com:443 -servername gitlab.example.com
    ```

  - `~/.gitlab-runner/certs/gitlab.example.com.crt` sur les systèmes *nix lorsque GitLab Runner est exécuté en tant que non-`root`.
  - `./certs/gitlab.example.com.crt` sur les autres systèmes. Si GitLab Runner est exécuté en tant que service Windows, cela ne fonctionne pas. Spécifiez plutôt un fichier de certificat personnalisé.

Remarques :

- Si le certificat de votre serveur GitLab est signé par votre CA, utilisez votre certificat CA (et non le certificat signé de votre serveur GitLab). Il peut également être nécessaire d'ajouter les certificats intermédiaires à la chaîne. Par exemple, si vous disposez d'un certificat primaire, intermédiaire et racine, vous pouvez tous les regrouper dans un seul fichier :

  ```plaintext
  -----BEGIN CERTIFICATE-----
  (Your primary SSL certificate: your_domain_name.crt)
  -----END CERTIFICATE-----
  -----BEGIN CERTIFICATE-----
  (Your intermediate certificate)
  -----END CERTIFICATE-----
  -----BEGIN CERTIFICATE-----
  (Your root certificate)
  -----END CERTIFICATE-----
  ```

- Si vous mettez à jour le certificat d'un runner existant, [redémarrez-le](../commands/_index.md#gitlab-runner-restart).
- Si vous avez déjà un runner configuré via HTTP, mettez à jour le chemin de votre instance vers la nouvelle URL HTTPS de votre instance GitLab dans votre `config.toml`.
- En tant que solution de contournement temporaire et non sécurisée, pour ignorer la vérification des certificats, dans la section `variables:` de votre fichier `.gitlab-ci.yml`, définissez la variable CI `GIT_SSL_NO_VERIFY` sur `true`.

### Clonage Git {#git-cloning}

Le runner injecte les certificats manquants pour construire la chaîne CA en utilisant `CI_SERVER_TLS_CA_FILE`. Cela permet à `git clone` et aux artefacts de fonctionner avec des serveurs qui n'utilisent pas de certificats approuvés publiquement.

Cette approche est sécurisée, mais fait du runner un point de confiance unique.

## Approuver les certificats TLS pour les exécuteurs Docker et Kubernetes {#trusting-tls-certificates-for-docker-and-kubernetes-executors}

Tenez compte des informations suivantes lorsque vous enregistrez un certificat sur un conteneur :

- L'[**image utilisateur**](https://docs.gitlab.com/ci/yaml/#image), qui est utilisée pour exécuter le script utilisateur. Pour les scénarios impliquant l'approbation du certificat pour les scripts utilisateur, l'utilisateur doit prendre en charge la procédure d'installation d'un certificat. Les procédures d'installation des certificats peuvent varier selon l'image. Le runner n'a aucun moyen de savoir comment installer un certificat dans chaque scénario possible.
- L'[**image helper du runner**](advanced-configuration.md#helper-image), qui est utilisée pour gérer les opérations Git, les artefacts et le cache. Pour les scénarios impliquant l'approbation du certificat pour d'autres étapes CI/CD, l'utilisateur doit uniquement rendre un fichier de certificat disponible à un emplacement spécifique (par exemple, `/etc/gitlab-runner/certs/ca.crt`), et le conteneur Docker l'installera automatiquement pour l'utilisateur.

### Approbation du certificat pour les scripts utilisateur {#trusting-the-certificate-for-user-scripts}

Si votre build utilise TLS avec un certificat auto-signé ou un certificat personnalisé, installez le certificat dans votre job de build pour la communication entre pairs. Le conteneur Docker exécutant les scripts utilisateur ne possède pas les fichiers de certificat installés par défaut. Cela peut être nécessaire pour utiliser un hôte de cache personnalisé, effectuer un `git clone` secondaire ou récupérer un fichier via un outil comme `wget`.

Pour installer le certificat :

1. Mappez les fichiers nécessaires en tant que volume Docker afin que le conteneur Docker qui exécute les scripts puisse les voir. Pour ce faire, ajoutez un volume dans la clé correspondante à l'intérieur de `[runners.docker]` dans le fichier `config.toml`, par exemple :

   - **Linux** :

     ```toml
     [[runners]]
       name = "docker"
       url = "https://example.com/"
       token = "TOKEN"
       executor = "docker"

       [runners.docker]
          image = "ubuntu:latest"

          # Add path to your ca.crt file in the volumes list
          volumes = ["/cache", "/path/to-ca-cert-dir/ca.crt:/etc/gitlab-runner/certs/ca.crt:ro"]
     ```

1. **Uniquement Linux** : Utilisez le fichier mappé (par exemple, `ca.crt`) dans un [`pre_build_script`](advanced-configuration.md#the-runners-section) qui :
   1. Le copie vers `/usr/local/share/ca-certificates/ca.crt` à l'intérieur du conteneur Docker.
   1. L'installe en exécutant `update-ca-certificates --fresh`. Par exemple (les commandes varient selon la distribution que vous utilisez) :

      - Sur Ubuntu :

        ```toml
        [[runners]]
          name = "docker"
          url = "https://example.com/"
          token = "TOKEN"
          executor = "docker"

          # Copy and install CA certificate before each job
          pre_build_script = """
          apt-get update -y > /dev/null
          apt-get install -y ca-certificates > /dev/null

          cp /etc/gitlab-runner/certs/ca.crt /usr/local/share/ca-certificates/ca.crt
          update-ca-certificates --fresh > /dev/null
          """
        ```

      - Sur Alpine :

        ```toml
        [[runners]]
          name = "docker"
          url = "https://example.com/"
          token = "TOKEN"
          executor = "docker"

          # Copy and install CA certificate before each job
          pre_build_script = """
          apk update >/dev/null
          apk add ca-certificates > /dev/null
          rm -rf /var/cache/apk/*

          cp /etc/gitlab-runner/certs/ca.crt /usr/local/share/ca-certificates/ca.crt
          update-ca-certificates --fresh > /dev/null
          """
        ```

Si vous avez seulement besoin du certificat CA du serveur GitLab pouvant être utilisé, vous pouvez le récupérer à partir du fichier stocké dans la variable `CI_SERVER_TLS_CA_FILE` :

```shell
curl --cacert "${CI_SERVER_TLS_CA_FILE}"  ${URL} -o ${FILE}
```

### Approbation du certificat pour les autres étapes CI/CD {#trusting-the-certificate-for-the-other-cicd-stages}

Vous pouvez mapper un fichier de certificat vers `/etc/gitlab-runner/certs/ca.crt` sur Linux, ou `C:\GitLab-Runner\certs\ca.crt` sur Windows. L'image helper du runner installe ce fichier `ca.crt` défini par l'utilisateur au démarrage et l'utilise lors d'opérations telles que le clonage et le chargement d'artefacts, par exemple.

#### Docker {#docker}

- **Linux** :

  ```toml
  [[runners]]
    name = "docker"
    url = "https://example.com/"
    token = "TOKEN"
    executor = "docker"

    [runners.docker]
      image = "ubuntu:latest"

      # Add path to your ca.crt file in the volumes list
      volumes = ["/cache", "/path/to-ca-cert-dir/ca.crt:/etc/gitlab-runner/certs/ca.crt:ro"]
  ```

- **Windows** :

  ```toml
  [[runners]]
    name = "docker"
    url = "https://example.com/"
    token = "TOKEN"
    executor = "docker"

    [runners.docker]
      image = "mcr.microsoft.com/windows/servercore:21H2"

      # Add directory holding your ca.crt file in the volumes list
      volumes = ["c:\\cache", "c:\\path\\to-ca-cert-dir:C:\\GitLab-Runner\\certs:ro"]
  ```

#### Kubernetes {#kubernetes}

Pour fournir un fichier de certificat aux jobs s'exécutant dans Kubernetes :

1. Stockez le certificat en tant que secret Kubernetes dans votre espace de nommage :

   ```shell
   kubectl create secret generic <SECRET_NAME> --namespace <NAMESPACE> --from-file=<CERT_FILE>
   ```

1. Montez le secret en tant que volume dans votre runner, en remplaçant `<SECRET_NAME>` et `<LOCATION>` par des valeurs appropriées :

   ```toml
   gitlab-runner:
     runners:
      config: |
        [[runners]]
          [runners.kubernetes]
            namespace = "{{.Release.Namespace}}"
            image = "ubuntu:latest"
          [[runners.kubernetes.volumes.secret]]
              name = "<SECRET_NAME>"
              mount_path = "<LOCATION>"
   ```

   Le `mount_path` est le répertoire dans le conteneur où le certificat est stocké. Si vous avez utilisé `/etc/gitlab-runner/certs/` comme `mount_path` et `ca.crt` comme fichier de certificat, votre certificat est disponible à l'adresse `/etc/gitlab-runner/certs/ca.crt` dans votre conteneur.
1. Dans le cadre du job, installez le fichier de certificat mappé dans le magasin de certificats système. Par exemple, dans un conteneur Ubuntu :

   ```yaml
   script:
     - cp /etc/gitlab-runner/certs/ca.crt /usr/local/share/ca-certificates/
     - update-ca-certificates
   ```

   La gestion par l'exécuteur Kubernetes du `ENTRYPOINT` de l'image helper présente un [problème connu](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28484). Lorsqu'un fichier de certificat est mappé, il n'est pas automatiquement installé dans le magasin de certificats système.

## Dépannage {#troubleshooting}

Consultez la documentation générale sur le [dépannage SSL](https://docs.gitlab.com/omnibus/settings/ssl/ssl_troubleshooting/).

De plus, vous pouvez utiliser l'outil [`tlsctl`](https://gitlab.com/gitlab-org/ci-cd/runner-tools/tlsctl) pour déboguer les certificats GitLab depuis le côté du runner.

### Erreur : `x509: certificate signed by unknown authority` {#error-x509-certificate-signed-by-unknown-authority}

Cette erreur peut se produire lors d'une tentative d'extraction d'images d'exécuteur depuis un registre privé lorsque l'hôte Docker ou le nœud Kubernetes sur lequel le runner planifie les exécuteurs ne fait pas confiance au certificat du registre privé.

Pour corriger l'erreur, ajoutez l'autorité de certification racine ou la chaîne de certificats appropriée au magasin de confiance du système et redémarrez le service de conteneur.

Si vous utilisez Ubuntu ou Alpine, exécutez les commandes suivantes :

```shell
cp ca.crt /usr/local/share/ca-certificates/ca.crt
update-ca-certificates
systemctl restart docker.service
```

Pour les systèmes d'exploitation autres qu'Ubuntu ou Alpine, consultez la documentation de votre système d'exploitation pour trouver les commandes appropriées permettant d'installer le certificat approuvé.

En fonction de votre version de GitLab Runner et de l'environnement hôte Docker, vous devrez peut-être également désactiver le feature flag `FF_RESOLVE_FULL_TLS_CHAIN`.

### Erreurs `apt-get: not found` dans les jobs {#apt-get-not-found-errors-in-jobs}

Les commandes [`pre_build_script`](advanced-configuration.md#the-runners-section) sont exécutées avant chaque job qu'un runner exécute. Les commandes spécifiques à une distribution telles que `apk` ou `apt-get` peuvent provoquer des problèmes. Lorsque vous installez un certificat pour des scripts utilisateur, vos jobs CI peuvent échouer s'ils utilisent des [images](https://docs.gitlab.com/ci/yaml/#image) basées sur des distributions différentes.

Par exemple, si vos jobs CI exécutent des images Ubuntu et Alpine, les commandes Ubuntu échouent sur Alpine. L'erreur `apt-get: not found` se produit dans les jobs utilisant des images basées sur Alpine. Pour résoudre ce problème, effectuez l'une des opérations suivantes :

- Rédigez votre `pre_build_script` de façon à ce qu'il soit indépendant de la distribution.
- Utilisez des [tags](https://docs.gitlab.com/ci/yaml/#tags) pour vous assurer que les runners ne prennent en charge que les jobs avec des images compatibles.

### Erreur : `self-signed certificate in certificate chain` {#error-self-signed-certificate-in-certificate-chain}

Les jobs CI/CD échouent avec l'erreur suivante :

```plaintext
fatal: unable to access 'https://gitlab.example.com/group/project.git/': SSL certificate problem: self-signed certificate in certificate chain
```

Cependant, les [commandes de débogage OpenSSL](https://docs.gitlab.com/omnibus/settings/ssl/ssl_troubleshooting/#useful-openssl-debugging-commands) ne détectent aucune erreur.

Cette erreur peut se produire lorsque Git se connecte via un proxy que les commandes de dépannage `openssl s_client` n'utilisent pas par défaut. Pour vérifier si Git utilise un proxy pour récupérer le dépôt, activez le débogage :

```yaml
variables:
  GIT_CURL_VERBOSE: 1
```

Pour empêcher Git d'utiliser le proxy, définissez la variable `NO_PROXY` afin d'inclure le nom d'hôte de votre instance GitLab :

```yaml
variables:
  NO_PROXY: gitlab.example.com
```
