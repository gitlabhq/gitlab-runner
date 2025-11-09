---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Parallels
---

Parallels executorは、[Parallels Desktop](https://www.parallels.com/)の仮想化ソフトウェアを使用して、macOS上の仮想マシン（VM）でCI/CDジョブを実行します。Parallels Desktopは、macOSと並行してWindows、Linux、およびその他のオペレーティングシステムを実行できます。

Parallels executorは、VirtualBox executorと同様に動作します。これは、仮想マシンを作成および管理し、GitLab CI/CDジョブを実行します。各ジョブは、クリーンなVM環境で実行され、ビルド間の分離を提供します。設定情報については、[VirtualBox executor](virtualbox.md)を参照してください。

{{< alert type="note" >}}

Parallels executorは、キャッシュ機能をサポートしていません。

{{< /alert >}}
