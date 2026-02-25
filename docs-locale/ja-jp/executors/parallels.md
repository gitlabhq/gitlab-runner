---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Parallels
---

Parallels executorは、macOS上の仮想マシン（VM）でCI/CDジョブを実行するために、[Parallels Desktop](https://www.parallels.com/)仮想化ソフトウェアを使用します。Parallels Desktopは、macOSと並行してWindows、Linux、およびその他のオペレーティングシステムを実行できます。

Parallels executorは、VirtualBox executorと同様に動作します。仮想マシンを作成および管理し、GitLab CI/CDジョブを実行します。各ジョブはクリーンなVM環境で実行され、ビルド間の分離を提供します。設定情報については、[VirtualBox executor](virtualbox.md)を参照してください。

{{< alert type="note" >}}

Parallels executorはローカルキャッシュをサポートしていません。[分散キャッシュ](../configuration/speed_up_job_execution.md)がサポートされています。

{{< /alert >}}
