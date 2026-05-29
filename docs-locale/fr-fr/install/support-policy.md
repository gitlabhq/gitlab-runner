---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Politique de support de GitLab Runner
---

La politique de support de GitLab Runner est déterminée par la politique de cycle de vie du système d'exploitation.

## Support des images de conteneur {#container-images-support}

Nous suivons le cycle de vie de support des distributions de base (Ubuntu, Alpine, Red Hat Universal Base Image) utilisées pour créer les images de conteneur GitLab Runner.

Les dates de fin de publication des distributions de base ne s'aligneront pas nécessairement sur le cycle de version majeure de GitLab. Cela signifie que nous cesserons de publier une version de l'image de conteneur GitLab Runner dans une version mineure. Cela garantit que nous ne publions pas d'images que la distribution en amont ne met plus à jour.

### Images de conteneur et date de fin de publication {#container-images-and-end-of-publishing-date}

| Conteneur de base                 | Version du conteneur de base | Date de fin de vie (EOL) du fournisseur | Date de fin de vie (EOL) GitLab |
|--------------------------------|------------------------|-----------------|-----------------|
| Ubuntu                         | 24.04                  | 2027-04-30      | 2027-05-20      |
| Ubuntu                         | 20.04                  | 2025-05-31      | 2025-06-19      |
| Alpine                         | 3.12                   | 2022-05-01      | 2023-05-22      |
| Alpine                         | 3.13                   | 2022-11-01      | 2023-05-22      |
| Alpine                         | 3.14                   | 2023-05-01      | 2023-05-22      |
| Alpine                         | 3.15                   | 2023-11-01      | 2024-01-18      |
| Alpine                         | 3.16                   | 2024-05-23      | 2024-06-22      |
| Alpine                         | 3.17                   | 2024‑11‑22      | 2024-12-22      |
| Alpine                         | 3.18                   | 2025‑05‑09      | 2025-05-22      |
| Alpine                         | 3.19                   | 2025‑11‑01      | 2025-11-22      |
| Alpine                         | 3.21                   | 2026‑11‑01      | 2026-11-22      |
| Alpine                         | dernière version                 |                 |                 |
| Red Hat Universal Base Image 9 | 9.5                    | 2025-04-31      | 2025-05-22      |

Les versions 17.7 et ultérieures de GitLab Runner ne prennent en charge qu'une seule version Alpine (`latest`) au lieu de versions spécifiques. La version Alpine 3.21 sera prise en charge jusqu'à la date de fin de vie (EOL) indiquée. En revanche, Ubuntu 24.04 sera pris en charge jusqu'à sa date de fin de vie (EOL), à laquelle nous passerons à la version LTS la plus récente.

## Support des versions de Windows {#windows-version-support}

GitLab prend officiellement en charge les versions LTS des systèmes d'exploitation Microsoft Windows et suit donc la politique de cycle de vie des [Servicing Channels](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#servicing-channels) de Microsoft.

Cela signifie que nous prenons en charge :

- Les versions du [Long-Term Servicing Channel](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#long-term-servicing-channel) pendant cinq ans après leur date de sortie.

  Après cinq ans, Microsoft propose un support étendu pour cinq années supplémentaires. Durant cette période étendue, nous offrons un support aussi longtemps que cela est possible. Nous pouvons mettre fin à ce support, avec annonce préalable, lors d'une version majeure de GitLab.
- Les versions du [Semi-Annual Channel](https://learn.microsoft.com/en-us/windows/deployment/update/waas-overview#semi-annual-channel) pendant 18 mois après leur date de sortie. Nous ne prenons pas en charge ces versions après la fin du support standard.

Cette politique de support s'applique aux [binaires Windows](windows.md#installation) que nous distribuons et à l'[exécuteur Docker](../executors/docker.md#supported-windows-versions).

> [!note]
> L'exécuteur Docker pour les conteneurs Windows a des exigences strictes en matière de version, car les conteneurs doivent correspondre à la version du système d'exploitation hôte. Consultez la [liste des conteneurs Windows pris en charge](../executors/docker.md#supported-windows-versions) pour plus d'informations.

Comme source unique de vérité, nous utilisons <https://learn.microsoft.com/en-us/lifecycle/products/>, qui spécifie les dates de sortie, de support standard et de support étendu.

Voici une liste des versions couramment utilisées et leur date de fin de vie :

| Système d'exploitation           | Date de fin de support standard | Date de fin de support étendu |
|----------------------------|-----------------------------|---------------------------|
| Windows Server 2019 (1809) | Janvier 2024                | Janvier 2029              |
| Windows Server 2022 (21H2) | Octobre 2026                | Octobre 2031              |
| Windows Server 2025 (24H2) | Octobre 2029                | Octobre 2034              |

### Versions futures {#future-releases}

Microsoft publie de nouveaux produits Windows Server dans le [Semi-Annual Channel](https://learn.microsoft.com/en-us/windows-server/get-started/servicing-channels-comparison#semi-annual-channel) deux fois par an, et toutes les 2 à 3 ans, une nouvelle version majeure de Windows Server est publiée dans le [Long-Term Servicing Channel (LTSC)](https://learn.microsoft.com/en-us/windows-server/get-started/servicing-channels-comparison#long-term-servicing-channel-ltsc).

GitLab vise à tester et à publier de nouvelles images d'assistance GitLab Runner incluant la dernière version de Windows Server (Semi-Annual Channel) dans le mois suivant la date de sortie officielle de Microsoft sur Google Cloud Platform. Consultez la [liste des versions actuelles de Windows Server par option de maintenance](https://learn.microsoft.com/en-us/windows/release-health/windows-server-release-info#windows-server-current-versions-by-servicing-option) pour les dates de disponibilité.
