---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Parallels
---

L'exécuteur Parallels utilise le logiciel de virtualisation [Parallels Desktop](https://www.parallels.com/) pour exécuter des jobs CI/CD dans des machines virtuelles (VM) sur macOS. Parallels Desktop peut exécuter Windows, Linux et d'autres systèmes d'exploitation en parallèle avec macOS.

L'exécuteur Parallels fonctionne de manière similaire à l'exécuteur VirtualBox. Il crée et gère des machines virtuelles et exécute vos jobs GitLab CI/CD. Chaque job s'exécute dans un environnement VM propre, assurant l'isolation entre les builds. Pour les informations de configuration, consultez [l'exécuteur VirtualBox](virtualbox.md).

> [!note]
> Les exécuteurs Parallels ne prennent pas en charge le cache local. Le [cache distribué](../configuration/speed_up_job_execution.md) est pris en charge.
