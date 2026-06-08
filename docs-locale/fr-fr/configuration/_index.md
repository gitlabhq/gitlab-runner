---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
description: "Configuration, certificats, mise à l'échelle automatique, configuration du proxy."
title: Configurer GitLab Runner
---

{{< details >}}

- Niveau : Free, Premium, Ultimate
- Offre : GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

Apprenez à configurer GitLab Runner.

- [Options de configuration avancées](advanced-configuration.md) : Utilisez le fichier de configuration [`config.toml`](https://github.com/toml-lang/toml) pour modifier les paramètres du runner.
- [Utiliser des certificats auto-signés](tls-self-signed.md) : Configurez des certificats qui vérifient les pairs TLS lors de la connexion au serveur GitLab.
- [Mise à l'échelle automatique avec Docker Machine](autoscale.md) : Exécutez des jobs sur des machines créées automatiquement par Docker Machine.
- [Mise à l'échelle automatique de GitLab Runner sur AWS EC2](runner_autoscale_aws/_index.md) : Exécutez des jobs sur des instances AWS EC2 à mise à l'échelle automatique.
- [Mise à l'échelle automatique de GitLab CI sur AWS Fargate](runner_autoscale_aws_fargate/_index.md) : Utilisez le pilote AWS Fargate avec l'exécuteur personnalisé GitLab pour exécuter des jobs dans AWS ECS.
- [Unités de traitement graphique](gpus.md) : Utilisez des GPU pour exécuter des jobs.
- [Le système init](init.md) : GitLab Runner installe ses fichiers de service init en fonction de votre système d'exploitation.
- [Shells pris en charge](../shells/_index.md) : Exécutez des builds sur différents systèmes en utilisant des générateurs de scripts shell.
- [Considérations de sécurité](../security/_index.md) : Soyez conscient des implications potentielles en matière de sécurité lors de l'exécution de vos jobs avec GitLab Runner.
- [Surveillance des runners](../monitoring/_index.md) : Surveillez le comportement de vos runners.
- [Nettoyer automatiquement le cache Docker](../executors/docker.md#clear-the-docker-cache) : Si vous manquez d'espace disque, utilisez un cron job pour nettoyer les anciens conteneurs et volumes.
- [Configurer GitLab Runner pour fonctionner derrière un proxy](proxy.md) : Configurez un proxy Linux et configurez GitLab Runner. Cette configuration fonctionne bien avec l'exécuteur Docker.
- [Configurer GitLab Runner pour Oracle Cloud Infrastructure (OCI)](oracle_cloud_performance.md) : Optimisez les performances de votre GitLab Runner dans OCI.
- [Gestion des requêtes soumises à une limite de débit](proxy.md#handling-rate-limited-requests).
- [Configurer GitLab Runner Operator](configuring_runner_operator.md).
