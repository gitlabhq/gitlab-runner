---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Exécution de GitLab Runner derrière un proxy
---

{{< details >}}

- Niveau : Free, Premium, Ultimate
- Offre : GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Ce guide vise spécifiquement à faire fonctionner GitLab Runner avec l'exécuteur Docker derrière un proxy.

Avant de continuer, assurez-vous d'avoir déjà [installé Docker](https://docs.docker.com/get-started/get-docker/) et [GitLab Runner](../install/_index.md) sur la même machine.

## Configuration de `cntlm` {#configuring-cntlm}

> [!note]
> Si vous utilisez déjà un proxy sans authentification, cette section est optionnelle et vous pouvez passer directement à la [configuration de Docker](#configuring-docker-for-downloading-images). La configuration de `cntlm` n'est nécessaire que si vous vous trouvez derrière un proxy avec authentification, mais il est recommandé de l'utiliser dans tous les cas.

[`cntlm`](https://github.com/versat/cntlm) est un proxy Linux qui peut être utilisé comme proxy local et présente 2 avantages majeurs par rapport à l'ajout manuel des détails du proxy partout :

- Une seule source où vous devez modifier vos identifiants
- Les identifiants ne sont pas accessibles depuis les runners Docker

En supposant que vous [ayez installé `cntlm`](https://www.howtoforge.com/linux-ntlm-authentication-proxy-isa-server-with-cntlm), vous devez d'abord le configurer.

### Faire écouter `cntlm` sur l'interface `docker0` {#make-cntlm-listen-to-the-docker0-interface}

Pour une sécurité accrue et une protection contre Internet, liez `cntlm` pour écouter sur l'interface `docker0`, qui dispose d'une adresse IP accessible par les conteneurs. Si vous indiquez à `cntlm` sur l'hôte Docker de se lier uniquement à cette adresse, les conteneurs Docker pourront l'atteindre, mais le monde extérieur ne le pourra pas.

1. Trouvez l'adresse IP utilisée par Docker :

   ```shell
   ip -4 -oneline addr show dev docker0
   ```

   L'adresse IP est généralement `172.17.0.1`, appelons-la `docker0_interface_ip`.

1. Ouvrez le fichier de configuration de `cntlm` (`/etc/cntlm.conf`). Saisissez votre nom d'utilisateur, votre mot de passe, votre domaine et vos hôtes proxy, et configurez l'adresse IP `Listen` que vous avez trouvée à l'étape précédente. Le résultat devrait ressembler à ceci :

   ```plaintext
   Username     testuser
   Domain       corp-uk
   Password     password
   Proxy        10.0.0.41:8080
   Proxy        10.0.0.42:8080
   Listen       172.17.0.1:3128 # Change to your docker0 interface IP
   ```

1. Enregistrez les modifications et redémarrez son service :

   ```shell
   sudo systemctl restart cntlm
   ```

## Configuration de Docker pour le téléchargement d'images {#configuring-docker-for-downloading-images}

> [!note]
> Les éléments suivants s'appliquent aux systèmes d'exploitation prenant en charge systemd.

Pour plus d'informations sur l'utilisation d'un proxy, consultez la [documentation Docker](https://docs.docker.com/engine/daemon/proxy/).

Le fichier de service devrait ressembler à ceci :

```ini
[Service]
Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
```

## Ajout de variables Proxy à la configuration de GitLab Runner {#adding-proxy-variables-to-the-gitlab-runner-configuration}

Les variables proxy doivent également être ajoutées à la configuration de GitLab Runner, afin qu'il puisse se connecter à GitLab.com depuis derrière le proxy.

Cette action est identique à l'ajout du proxy au service Docker ci-dessus :

1. Créez un répertoire drop-in systemd pour le service `gitlab-runner` :

   ```shell
   mkdir /etc/systemd/system/gitlab-runner.service.d
   ```

1. Créez un fichier appelé `/etc/systemd/system/gitlab-runner.service.d/http-proxy.conf` qui ajoute les variables d'environnement `HTTP_PROXY` :

   ```ini
   [Service]
   Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
   Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
   ```

   Pour connecter GitLab Runner à des URL internes, comme une instance GitLab Self-Managed, définissez une valeur pour la variable d'environnement `NO_PROXY`.

   ```ini
   [Service]
   Environment="HTTP_PROXY=http://docker0_interface_ip:3128/"
   Environment="HTTPS_PROXY=http://docker0_interface_ip:3128/"
   Environment="NO_PROXY=gitlab.example.com"
   ```

1. Enregistrez le fichier et appliquez les modifications :

   ```shell
   systemctl daemon-reload
   ```

1. Redémarrez GitLab Runner :

   ```shell
   sudo systemctl restart gitlab-runner
   ```

1. Vérifiez que la configuration a bien été chargée :

   ```shell
   systemctl show --property=Environment gitlab-runner
   ```

   Vous devriez voir :

   ```ini
   Environment=HTTP_PROXY=http://docker0_interface_ip:3128/ HTTPS_PROXY=http://docker0_interface_ip:3128/
   ```

## Ajout du proxy aux conteneurs Docker {#adding-the-proxy-to-the-docker-containers}

Après avoir [enregistré votre runner](../register/_index.md), vous pouvez souhaiter propager vos paramètres proxy aux conteneurs Docker (par exemple, pour `git clone`).

Pour ce faire, vous devez modifier `/etc/gitlab-runner/config.toml` et ajouter les éléments suivants à la section `[[runners]]` :

```toml
pre_get_sources_script = "git config --global http.proxy $HTTP_PROXY; git config --global https.proxy $HTTPS_PROXY"
environment = ["https_proxy=http://docker0_interface_ip:3128", "http_proxy=http://docker0_interface_ip:3128", "HTTPS_PROXY=docker0_interface_ip:3128", "HTTP_PROXY=docker0_interface_ip:3128"]
```

Où `docker0_interface_ip` est l'adresse IP de l'interface `docker0`.

> [!note]
> Dans nos exemples, nous définissons à la fois des variables en minuscules et en majuscules, car certains programmes attendent `HTTP_PROXY` et d'autres `http_proxy`. Malheureusement, il n'existe pas de [standard](https://unix.stackexchange.com/questions/212894/whats-the-right-format-for-the-http-proxy-environment-variable-caps-or-no-ca#212972) pour ces types de variables d'environnement.

## Paramètres proxy lors de l'utilisation du service `dind` {#proxy-settings-when-using-dind-service}

Lors de l'utilisation de l'[exécuteur Docker-in-Docker](https://docs.gitlab.com/ci/docker/using_docker_build/#use-docker-in-docker) (`dind`), il peut être nécessaire de spécifier `docker:2375,docker:2376` dans la variable d'environnement `NO_PROXY`. Les ports sont requis, sinon `docker push` est bloqué.

La communication entre `dockerd` de `dind` et le client `docker` local (comme décrit ici : <https://hub.docker.com/_/docker/>) utilise les variables proxy contenues dans la configuration Docker de root.

Pour configurer cela, vous devez modifier `/root/.docker/config.json` afin d'inclure votre configuration proxy complète, par exemple :

```json
{
    "proxies": {
        "default": {
            "httpProxy": "http://proxy:8080",
            "httpsProxy": "http://proxy:8080",
            "noProxy": "docker:2375,docker:2376"
        }
    }
}
```

Pour transmettre les paramètres au conteneur de l'exécuteur Docker, un `$HOME/.docker/config.json` doit également être créé à l'intérieur du conteneur. Cela peut être scripté en tant que `before_script` dans le `.gitlab-ci.yml`, par exemple :

```yaml
before_script:
  - mkdir -p $HOME/.docker/
  - 'echo "{ \"proxies\": { \"default\": { \"httpProxy\": \"$HTTP_PROXY\", \"httpsProxy\": \"$HTTPS_PROXY\", \"noProxy\": \"$NO_PROXY\" } } }" > $HOME/.docker/config.json'
```

Ou alternativement, dans la configuration du `gitlab-runner` (`/etc/gitlab-runner/config.toml`) concerné :

```toml
[[runners]]
  pre_build_script = "mkdir -p $HOME/.docker/ && echo \"{ \\\"proxies\\\": { \\\"default\\\": { \\\"httpProxy\\\": \\\"$HTTP_PROXY\\\", \\\"httpsProxy\\\": \\\"$HTTPS_PROXY\\\", \\\"noProxy\\\": \\\"$NO_PROXY\\\" } } }\" > $HOME/.docker/config.json"
```

> [!note]
> Un niveau supplémentaire d'échappement de `"` est requis car cela crée un fichier JSON avec un shell spécifié sous la forme d'une chaîne unique à l'intérieur d'un fichier TOML. Comme il ne s'agit pas de YAML, n'échappez pas le caractère `:`.

Si la liste `NO_PROXY` doit être étendue, les caractères génériques `*` ne fonctionnent que pour les suffixes, mais pas pour les préfixes ou la notation CIDR. Pour plus d'informations, consultez <https://github.com/moby/moby/issues/9145> et <https://unix.stackexchange.com/questions/23452/set-a-network-range-in-the-no-proxy-environment-variable>.

## Gestion des requêtes soumises à une limite de débit {#handling-rate-limited-requests}

Une instance GitLab peut se trouver derrière un proxy inverse qui applique une limite de débit sur les requêtes API afin de prévenir les abus. GitLab Runner envoie plusieurs requêtes à l'API et peut dépasser ces limites de débit.

Par conséquent, GitLab Runner gère les scénarios de limite de débit en utilisant la [logique de nouvelle tentative](#retry-logic) suivante :

### Logique de nouvelle tentative {#retry-logic}

Lorsque GitLab Runner reçoit une réponse `429 Too Many Requests`, il suit cette séquence de nouvelles tentatives :

1. Le runner vérifie dans les en-têtes de réponse la présence d'un en-tête `RateLimit-ResetTime`.
   - L'en-tête `RateLimit-ResetTime` doit avoir une valeur qui est une date HTTP valide (RFC1123), comme `Wed, 21 Oct 2015 07:28:00 GMT`.
   - Si l'en-tête est présent et a une valeur valide, le runner attend jusqu'au moment spécifié et émet une autre requête.
1. Si l'en-tête `RateLimit-ResetTime` est invalide ou absent, le runner vérifie dans les en-têtes de réponse la présence d'un en-tête `Retry-After`.
   - L'en-tête `Retry-After` doit avoir une valeur en secondes, comme `Retry-After: 30`.
   - Si le format de l'en-tête est présent et a une valeur valide, le runner attend jusqu'au moment spécifié et émet une autre requête.
1. Si les deux en-têtes sont absents ou invalides, le runner attend l'intervalle par défaut et émet une autre requête.

Le runner effectue jusqu'à 5 nouvelles tentatives pour les requêtes échouées. Si toutes les tentatives échouent, le runner consigne l'erreur provenant de la dernière réponse.

### Formats d'en-têtes pris en charge {#supported-header-formats}

| En-tête                | Format              | Exemple                         |
|-----------------------|---------------------|---------------------------------|
| `RateLimit-ResetTime` | Date HTTP (RFC1123) | `Wed, 21 Oct 2015 07:28:00 GMT` |
| `Retry-After`         | Secondes             | `30`                            |

> [!note]
> L'en-tête `RateLimit-ResetTime` est insensible à la casse car toutes les clés d'en-tête sont traitées par la fonction [`http.CanonicalHeaderKey`](https://pkg.go.dev/net/http#CanonicalHeaderKey).
