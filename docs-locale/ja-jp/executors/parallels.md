---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Parallels
---

Parallels executorは、[Parallels Desktop](https://www.parallels.com/)仮想化ソフトウェアを使用して、macOS上の仮想マシン（VM）でCI/CDジョブを実行します。Parallels Desktopでは、macOSと並行してWindows、Linux、その他のオペレーティングシステムを実行できます。

Parallels executorは、VirtualBox executorと同様に動作します。仮想マシンを作成および管理し、GitLab CI/CDジョブを実行します。各ジョブはクリーンなVM環境で実行され、ビルド間の分離を実現します。設定情報については、[VirtualBox executor](virtualbox.md)を参照してください。

> [!note]
> Parallels executorsはローカルキャッシュをサポートしていません。[分散キャッシュ](../configuration/speed_up_job_execution.md)はサポートされています。
