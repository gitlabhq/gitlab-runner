---
stage: Verify
group: Runner
info: To determine the technical writer assigned to the Stage/Group associated with this page, see https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments
title: Docker Machine Executorの自動スケール設定
---

{{< details >}}

- プラン:Free、Premium、Ultimate
- 製品:GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

{{< history >}}

- 自動スケール機能はGitLab Runner 1.1.0で導入されました。

{{< /history >}}

{{< alert type="note" >}}

Docker Machine ExecutorはGitLab 17.5で非推奨となりました。GitLab 20.0（2027年5月）で削除される予定です。GitLab 20.0まではDocker Machine Executorのサポートが継続されますが、新機能を追加する予定はありません。CI/CDジョブの実行を妨げる可能性のある重大なバグ、または実行コストに影響を与えるバグのみに対処します。Amazon Web Services（AWS）EC2、Microsoft Azure Compute、またはGoogle Compute Engine（GCE）でDocker Machine Executorを使用している場合は、[GitLab Runner Autoscaler](../runner_autoscale/_index.md)に移行してください。

{{< /alert >}}

自動スケール機能を使用すると、より柔軟かつ動的な方法でリソースを使用できます。

GitLab Runnerは自動スケールできるため、インフラストラクチャには、常に必要な数のビルドインスタンスのみが含まれます。自動スケールのみを使用するようにGitLab Runnerを設定すると、GitLab Runnerをホストするシステムは、作成するすべてのマシンの踏み台として機能します。このマシンは「Runnerマネージャー」と呼ばれます。

{{< alert type="note" >}}

DockerではDocker Machineが非推奨になりました。Docker Machineは、パブリッククラウド仮想マシンでRunnerを自動スケールするために使用される基盤技術です。詳細については、[Docker Machineの非推奨に対応するための戦略について説明するイシュー](https://gitlab.com/gitlab-org/gitlab/-/issues/341856)をお読みください。

{{< /alert >}}

Docker Machine autoscalerは、`limit`と`concurrent`の設定に関係なく、VMごとに1つのコンテナを作成します。

この機能が有効であり、適切に設定されている場合、ジョブは_オンデマンド_で作成されたマシン上で実行されます。これらのマシンは、ジョブの完了後に次のジョブを実行するために待機するか、設定された`IdleTime`の経過後に削除できます。多くのクラウドプロバイダーでは、この方法は既存のインスタンスを使用することでコストを削減します。

以下に、[GitLab Community Edition（CE）](https://gitlab.com/gitlab-org/gitlab-foss)プロジェクトのGitLab.comでテストされたGitLab Runner自動スケール機能の実例を示します:

![自動スケールの実例](img/autoscale-example.png)

チャートに示されている各マシンは独立したクラウドインスタンスであり、Dockerコンテナ内でジョブを実行します。

## システム要件

自動スケールを設定する前に、次のことを行う必要があります。

- [独自の環境を準備します](../executors/docker_machine.md#preparing-the-environment)。
- （オプション）GitLabが提供するDocker Machineの[フォークバージョン](../executors/docker_machine.md#forked-version-of-docker-machine)を使用します。これにはいくつかの追加修正が含まれています。

## サポートされているクラウドプロバイダー

自動スケールメカニズムは[Docker Machine](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/tree/main/)に基づいています。サポートされているすべての仮想化およびクラウドプロバイダーのパラメーターは、GitLabが管理する[Docker Machine](https://gitlab.com/gitlab-org/ci-cd/docker-machine/-/tree/main/)のフォークで利用できます。

## Runnerの設定

このセクションでは、重要な自動スケールパラメーターについて説明します。設定の詳細については、[高度な設定](advanced-configuration.md)を参照してください。

### Runnerのグローバルオプション

| パラメーター    | 値   | 説明 |
|--------------|---------|-------------|
| `concurrent` | 整数 | グローバルで同時に実行できるジョブの数を制限します。このパラメーターは、ローカルと自動スケールの両方で、_すべて_の定義済みRunnerを使用できるジョブの最大数を設定します。`limit`（[`[[runners]]`セクション](#runners-options)）および`IdleCount`（[`[runners.machine]`セクション](advanced-configuration.md#the-runnersmachine-section)）とともに、作成されるマシンの数の上限に影響します。 |

### `[[runners]]`のオプション

| パラメーター  | 値            | 説明 |
|------------|------------------|-------------|
| `executor` | 文字列           | 自動スケール機能を使用するには、`executor`を`docker+machine`に設定する必要があります。 |
| `limit`    | 整数          | この特定のトークンで同時に処理できるジョブの数を制限します。`0`は制限がないことを意味します。自動スケールの場合、これはこのプロバイダーによって作成されるマシンの数の上限です（`concurrent`および`IdleCount`との組み合わせ）。 |

### `[runners.machine]`のオプション

設定パラメーターの詳細については、[GitLab Runner - 高度な構成 - `[runners.machine]` セクション](advanced-configuration.md#the-runnersmachine-section)を参照してください。

### `[runners.cache]`のオプション

設定パラメーターの詳細については、[GitLab Runner - 高度な構成 - `[runners.cache]` セクション](advanced-configuration.md#the-runnerscache-section)を参照してください。

### その他の設定情報

`IdleCount = 0`を設定する場合には特別なモードもあります。このモードでは、（アイドル状態のマシンがない場合は）各ジョブの前にマシンが**常に****オンデマンド**で作成されます。ジョブが完了すると、自動スケールアルゴリズムは[以下の説明と同様に](#autoscaling-algorithm-and-parameters)動作します。マシンが次のジョブを待機しているが実行するジョブがない場合、`IdleTime`期間の経過後にマシンは削除されます。ジョブがない場合、アイドル状態のマシンはありません。

`IdleCount`が`0`より大きな値に設定されている場合、アイドル状態のVMがバックグラウンドで作成されます。Runnerは新しいジョブを要求する前に、既存のアイドル状態のVMを取得します。

- ジョブがRunnerに割り当てられている場合、そのジョブは以前に取得したVMに送信されます。
- ジョブがRunnerに割り当てられていない場合、アイドル状態のVMのロックが解除され、VMはプールに戻されます。

## Docker Machine Executorによって作成されるVMの数を制限する

Docker Machine Executorによって作成される仮想マシン（VM）の数を制限するには、`config.toml`ファイルの`[[runners]]`セクションの`limit`パラメーターを使用します。

`concurrent`パラメーターではVMの数は**制限されません**。

複数のRunnerワーカーを管理するように１つのプロセスを設定できます。詳細については、[基本設定: 1つのRunnerマネージャー、1つのRunner](../fleet_scaling/_index.md#basic-configuration-one-runner-manager-one-runner)を参照してください。

次の例は、1つのRunnerプロセスに対して`config.toml`ファイルで設定された値を示しています。

```toml
concurrent = 100

[[runners]]
name = "first"
executor = "shell"
limit = 40
(...)

[[runners]]
name = "second"
executor = "docker+machine"
limit = 30
(...)

[[runners]]
name = "third"
executor = "ssh"
limit = 10

[[runners]]
name = "fourth"
executor = "virtualbox"
limit = 20
(...)

```

この設定では次のようになります。

- 1つのRunnerプロセスで、異なる実行環境を使用する4つの異なるRunnerワーカーを作成できます。
- `concurrent`の値が100に設定されているため、この1つのRunnerは、最大100個のGitLab CI/CDジョブを同時実行します。
- `second` RunnerワーカーのみがDocker Machine Executorを使用するように設定されているため、このワーカーがVMを自動的に作成できます。
- `limit`が`30`に設定されているため、`second` Runnerワーカーは常に、自動スケールされたVMで最大30個のCI/CDジョブを実行できます。
- `concurrent`は複数の`[[runners]]`ワーカー全体のグローバルな同時実行制限を定義しますが、`limit`は1つの`[[runners]]`ワーカーの最大同時実行数を定義します。

この例では、Runnerプロセスは次のように処理します。

- すべての`[[runners]]`ワーカー全体で最大100個の同時ジョブ。
- `first`ワーカーの場合、40個以下のジョブ。これらのジョブは`shell` executorを使用して実行されます。
- `second`ワーカーの場合、30個以下のジョブ。これらのジョブは`docker+machine` executorを使用して実行されます。さらに、Runnerは`[runners.machine]`の自動スケール設定に基づいてVMを維持しますが、維持するVMの数は、すべての状態（アイドル状態、使用中、作成中、削除中）で30個以下です。
- `third`ワーカーの場合、10個以下のジョブ。これらのジョブは`ssh` executorで実行されます。
- `fourth`ワーカーの場合、20個以下のジョブ。これらのジョブは`virtualbox` executorで実行されます。

次の2番目の例では、`docker+machine` executorを使用するように設定された2つの`[[runners]]`ワーカーがあります。この設定では、各Runnerワーカーは、`limit`パラメーターの値によって制約される個別のVMプールを管理します。

```toml
concurrent = 100

[[runners]]
name = "first"
executor = "docker+machine"
limit = 80
(...)

[[runners]]
name = "second"
executor = "docker+machine"
limit = 50
(...)

```

この例では次のようになります。

- Runnerプロセスが処理するジョブは最大100個です（`concurrent`の値）。
- Runnerプロセスは、2つの`[[runners]]`ワーカーでジョブを実行します。各ワーカーは`docker+machine` executorを使用します。
- `first` Runnerは最大80個のVMを作成できます。したがって、このRunnerはいつでも最大80個のジョブを実行できます。
- `second` Runnerは最大50個のVMを作成できます。したがって、このRunnerはいつでも最大50個のジョブを実行できます。

{{< alert type="note" >}}

制限値の合計は`130`（`80 + 50`）ですが、グローバルな`concurrent`の設定が100であるため、Runnerプロセスが同時実行するジョブの最大数は100個です。

{{< /alert >}}

## 自動スケールアルゴリズムとパラメーター

自動スケールアルゴリズムは次のパラメーターに基づいています。

- `IdleCount`
- `IdleCountMin`
- `IdleScaleFactor`
- `IdleTime`
- `MaxGrowthRate`
- `limit`

ジョブを実行していない各マシンはアイドル状態になっています。自動スケールモードのGitLab Runnerはすべてのマシンをモニタリングし、アイドル状態のマシンの数が常に`IdleCount`であるようにします。

{{< alert type="note" >}}

GitLab Runner 14.5で、`IdleScaleFactor`設定と`IdleCountMin`設定が追加され、この動作が多少変更されました。詳細については、[専用セクション](#the-idlescalefactor-strategy)を参照してください。

{{< /alert >}}

アイドル状態のマシンの数が不十分な場合、GitLab Runnerは`MaxGrowthRate`制限に従って新しいマシンのプロビジョニングを開始します。`MaxGrowthRate`値を超える数のマシンに対するリクエストは、作成されているマシンの数が`MaxGrowthRate`を下回るまで保留されます。

同時に、GitLab Runnerは各マシンのアイドル状態の期間を確認します。この時間が`IdleTime`の値を超えている場合と、マシンは自動的に削除されます。

---

**例:**次の自動スケールパラメーターを使用してGitLab Runnerを設定したとします。

```toml
[[runners]]
  limit = 10
  # (...)
  executor = "docker+machine"
  [runners.machine]
    MaxGrowthRate = 1
    IdleCount = 2
    IdleTime = 1800
    # (...)
```

最初に、ジョブがキューに入れられていない場合、GitLab Runnerは2台のマシン（`IdleCount = 2`）を起動し、それらをアイドル状態に設定します。また、`IdleTime`を30分（`IdleTime = 1800`）に設定していることにも注意してください。

次に、5個のジョブがGitLab CIでキューに入れられているとします。最初の2個のジョブが、2台あるアイドル状態のマシンに送信されます。GitLab Runnerは、アイドル状態のマシンの数が`IdleCount`よりも少ないこと（`0 < 2`）を認識したため、新しいマシンを起動します。これらのマシンは、`MaxGrowthRate`を超えないように順次プロビジョニングされます。

残りの3個のジョブは、準備ができた最初のマシンに割り当てられます。最適化として、これは以前にビジー状態だったがジョブを完了したマシンか、新しくプロビジョニングされたマシンにできます。この例では、プロビジョニングが高速で、以前のジョブが完了する前に新しいマシンが準備できていると仮定します。

現在、1台のアイドル状態のマシンがあるため、GitLab Runnerは`IdleCount`を満たすためにもう1台の新しいマシンを起動します。キューに新しいジョブがないため、この2台のマシンはアイドル状態になり、GitLab Runnerは満足します。

---

**発生した状況:**新しいジョブを待機しているアイドル状態のマシンが2台ありました。5個のジョブがキューに入れられた後で新しいマシンが作成されたため、マシンの合計数は7台になりました。それらのうち5台のマシンはジョブを実行しており、2台のマシンはアイドル状態で次のジョブを待機していました。

アルゴリズムは引き続き同様に機能しており、GitLab Runnerは、`IdleCount`が満たされるまで、ジョブの実行に使用されるマシンとして新しいアイドル状態のマシンを作成します。これらのマシンは、`limit`パラメーターで定義された数まで作成されます。GitLab Runnerは、作成されたマシンの総数が`limit`の数であることを認識すると、自動スケールを停止します。新しいジョブは、マシンがアイドル状態に戻るまで、ジョブキューで待機する必要があります。

上記の例では、常に2台のアイドル状態のマシンがあります。`IdleTime`は、`IdleCount`を超えている場合にのみ適用されます。次に、マシンの数を`IdleCount`に削減することを試みます。

---

**スケールダウン:**ジョブが完了すると、マシンはアイドル状態に設定され、次のジョブが実行されるまで待機します。キューに新しいジョブがないとします。`IdleTime`で指定された時間が経過すると、アイドル状態のマシンは削除されます。この例では、30分後にすべてのマシンが削除されます（最後のジョブの実行が終了してから30分後に各マシンが削除されます）。GitLab Runnerは、この例の最初の部分と同じように、`IdleCount`の数のアイドル状態のマシンを実行し続けます。

---

まとめ:

1. GitLab Runnerを起動します
1. GitLab Runnerが2台のアイドル状態のマシンを作成します
1. GitLab Runnerが1つのジョブを選択します
1. GitLab Runnerは、常に2台のアイドル状態のマシンを維持するという厳格な要件を満たすために、もう1台のマシンを作成します
1. ジョブが完了し、アイドル状態のマシンが3台になります
1. 3台のアイドル状態のマシンの1つが、最後にジョブを選択した時点からの`IdleTime`を超えると、このマシンが削除されます
1. GitLab Runnerには、常に少なくとも2台のアイドル状態のマシンがあり、ジョブが迅速に取得されることを待機しています

以下に、ジョブの状態とマシンの状態の経時的な比較チャートを示します。

![自動スケール状態のチャート](img/autoscale-state-chart.png)

## `concurrent`、`limit`、`IdleCount`によって実行マシン数の上限が生成される仕組み

`limit`または`concurrent`に設定すべき値を示す魔法のような方程式は存在しません。各自のニーズに応じて設定してください。`IdleCount`の数のアイドル状態のマシンを維持することで、処理がスピードアップします。インスタンスが作成されるまで、10秒/20秒/30秒にわたって待つ必要はありません。ただしユーザーとしては、（料金を支払う必要のある）すべてのマシンにジョブを実行させ、アイドル状態にしないようにしたいと考えます。したがって`concurrent`と`limit`は、料金を支払う最大数のマシンを実行する値に設定する必要があります。`IdleCount`は、ジョブキューが空の場合に維持する_未使用_のマシンの最小数を示す値に設定する必要があります。

次の例を考えてみましょう。

```toml
concurrent=20

[[runners]]
  limit = 40
  [runners.machine]
    IdleCount = 10
```

上記のシナリオでは、作成するマシンの総数は30です。マシン（ビルド中およびアイドル状態）の総数の`limit`を40に設定できます。10台のアイドル状態のマシンを維持できますが、`concurrent`ジョブは20個です。したがって、20台の同時実行マシンがジョブを実行し、10台のマシンがアイドル状態であるため、総数は30になります。

しかし`limit`が、作成される可能性があるマシンの総数よりも少ない場合はどうなるでしょうか？以下の例で、このケースについて説明します。

```toml
concurrent=20

[[runners]]
  limit = 25
  [runners.machine]
    IdleCount = 10
```

この例では、最大20個の同時実行ジョブと25台のマシンを持つことができます。`limit`が25であるため、最悪の場合はアイドル状態のマシンの数は10ではなく5になります。

## `IdleScaleFactor`戦略

{{< history >}}

- [GitLab Runner 14.6](https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/3179)で実験的な機能として導入されました。

{{< /history >}}

`IdleCount`パラメーターは、Runnerが維持する必要があるアイドル状態のマシンの静的な数を定義します。割り当てる値はユースケースによって異なります。

まず、アイドル状態のマシンの数としてある程度少ない数を割り当てます。次に、現在の使用状況に応じて自動的にこの数を大きな数に調整します。このために実験的な`IdleScaleFactor`設定を使用します。

{{< alert type="warning" >}}

`IdleScaleFactor`は内部的に`float64`値であり、浮動小数点数形式を使用する必要があります（`0.0`、`1.0`、`1.5`など）。整数形式（`IdleScaleFactor = 1`など）を使用すると、Runnerのプロセスはエラー`FATAL: Service run failed   error=toml: cannot load TOML value of type int64 into a Go float`で失敗します。

{{< /alert >}}

この設定を使用すると、GitLab Runnerは定義された数のアイドル状態のマシンを維持しようとします。ただしこの数はもはや静的ではありません。GitLab Runnerは`IdleCount`を使用する代わりに、使用中のマシンをカウントし、必要なアイドル状態のマシンの数をその数の係数として定義します。

使用中のマシンがない場合、`IdleScaleFactor`は維持するアイドル状態のマシンがないと評価されます。`IdleCount`が`0`よりも大きい場合（かつ`IdleScaleFactor`が適用可能な場合のみ）、ジョブを処理できるアイドル状態のマシンがないと、Runnerはジョブを要求しません。新しいジョブがない場合、使用中のマシンの数は増加しないため、`IdleScaleFactor`は常に`0`と評価されます。これにより、Runnerは使用不可能な状態でブロックされます。

このことから、2番目の設定`IdleCountMin`が導入されました。これは、`IdleScaleFactor`の評価結果に関係なく維持する必要があるアイドル状態のマシンの最小数を定義します。**`IdleScaleFactor`を使用する場合、この設定は1未満に設定できません。Runnerは自動的に`IdleCountMin`を1に設定します。**

`IdleCountMin`を使用して、常に利用可能である必要があるアイドル状態のマシンの最小数を定義することもできます。これにより、キューに入れられる新しいジョブをすばやく開始できます。`IdleCount`と同様に、割り当てる値はユースケースによって異なります。

次に例を示します。

```toml
concurrent=200

[[runners]]
  limit = 200
  [runners.machine]
    IdleCount = 100
    IdleCountMin = 10
    IdleScaleFactor = 1.1
```

この場合、Runnerは決定ポイントに近づくと、使用中のマシンの数を確認します。たとえば、5台のアイドル状態のマシンと10台の使用中のマシンがあるとします。Runnerはこの数に`IdleScaleFactor`を乗算して、11台のアイドル状態のマシンが必要であると判断します。そのため、さらに6台のマシンが作成されます。

アイドル状態のマシンが90台、使用中のマシンが100台ある場合、GitLab Runnerは`IdleScaleFactor`に基づいて、`100 * 1.1 = 110`台のアイドル状態のマシンが必要であると認識します。そのため、再び新しいマシンの作成を開始します。ただし、アイドル状態のマシンの数が`100`に達すると、これは`IdleCount`で定義された上限であるため、アイドル状態のマシンの作成が停止します。

使用中のアイドル状態のマシンが100台から20台に減った場合、必要なアイドル状態のマシン数は`20 * 1.1 = 22`になります。GitLab Runnerはマシンの停止を開始します。前述したように、GitLab Runnerは`IdleTime`の間に使用されていないマシンを削除します。したがって、過剰な数のアイドル状態のVMの削除が積極的に行われます。

アイドル状態のマシンの数が0になった場合、必要なアイドル状態のマシン数は`0 * 1.1 = 0`です。ただし、これは定義されている`IdleCountMin`設定よりも少ないため、Runnerは残りのVMの数が10台になるまで、アイドル状態のVMを削除します。VMの数が10台になった時点でスケールダウンが停止し、Runnerは10台のマシンをアイドル状態で維持します。

## 自動スケールの期間を設定する

自動スケールは、期間に応じて異なる値を持つように設定できます。組織によっては、実行されるジョブの数が急増する定期的な時間帯と、ジョブがほとんどまたはまったくない時間帯がある場合があります。たとえば、ほとんどの民間企業は月曜日から金曜日の午前10時から午後6時までのような固定時間で稼働しています。週の夜間と週末には、パイプラインは開始されません。

これらの期間は`[[runners.machine.autoscaling]]`セクションを使用して設定できます。各期間では、一連の`Periods`に基づいて`IdleCount`と`IdleTime`を設定することがサポートされています。

**自動スケールの期間の仕組み**

`[runners.machine]`設定に複数の`[[runners.machine.autoscaling]]`セクションを追加できます。各セクションには、独自の`IdleCount`、`IdleTime`、`Periods`、および`Timezone`プロパティがあります。最も一般的なシナリオから最も具体的なシナリオの順に、設定ごとにセクションを定義する必要があります。

すべてのセクションが解析されます。現在の時刻に一致する最後のセクションがアクティブになります。一致するものがない場合、`[runners.machine]`のルートの値が使用されます。

次に例を示します。

```toml
[runners.machine]
  MachineName = "auto-scale-%s"
  MachineDriver = "google"
  IdleCount = 10
  IdleTime = 1800
  [[runners.machine.autoscaling]]
    Periods = ["* * 9-17 * * mon-fri *"]
    IdleCount = 50
    IdleTime = 3600
    Timezone = "UTC"
  [[runners.machine.autoscaling]]
    Periods = ["* * * * * sat,sun *"]
    IdleCount = 5
    IdleTime = 60
    Timezone = "UTC"
```

この設定では、すべての平日の9時から16時59分（UTC）までの期間は、稼働時間中の大量のトラフィックを処理するためにマシンがオーバープロビジョニングされます。週末には、トラフィックの減少を考慮して`IdleCount`が5に減っています。それ以外の期間には、値はルートのデフォルト（`IdleCount = 10`と`IdleTime = 1800`）から取得されます。

{{< alert type="note" >}}

指定した期間の最後の分の59秒目は、その期間の一部と*みなされません*。詳細については、[イシュー#2170](https://gitlab.com/gitlab-org/gitlab-runner/-/issues/2170)を参照してください。

{{< /alert >}}

期間の`Timezone`を指定できます（`"Australia/Sydney"`など）。指定しない場合、すべてのRunnerのホストマシンのシステム設定が使用されます。このデフォルトは、`Timezone = "Local"`として明示的に記述できます。

`[[runner.machine.autoscaling]]`セクションの構文の詳細については、[GitLab Runner - 詳細設定 - `[runners.machine]`セクション](advanced-configuration.md#the-runnersmachine-section)を参照してください。

## 分散Runnerキャッシュ

{{< alert type="note" >}}

[分散キャッシュの使用方法](../configuration/speed_up_job_execution.md#use-a-distributed-cache)を参照してください。

{{< /alert >}}

ジョブの処理をスピードアップするために、GitLab Runnerは、選択されたディレクトリやファイルを保存し、後続のジョブ間で共有する[キャッシュメカニズム](https://docs.gitlab.com/ci/yaml/#cache)を提供します。

このメカニズムは、ジョブが同じホストで実行される場合には正常に機能します。ただし、GitLab Runner自動スケール機能を使用し始めると、ほとんどのジョブは新しい（またはほぼ新しい）ホストで実行されます。この新しいホストは、新しいDockerコンテナで各ジョブを実行します。その場合、キャッシュ機能を利用することはできません。

この問題に対処するために、自動スケール機能とともに分散Runnerキャッシュ機能が導入されました。

この機能は設定済みのオブジェクトストレージサーバーを使用して、使用中のDockerホスト間でキャッシュを共有します。GitLab Runnerはサーバーをクエリし、アーカイブをダウンロードしてキャッシュを復元するか、アップロードしてキャッシュをアーカイブします。

分散キャッシュを有効にするには、`config.toml`で[`[runners.cache]`ディレクティブ](advanced-configuration.md#the-runnerscache-section)を使用して定義する必要があります。

```toml
[[runners]]
  limit = 10
  executor = "docker+machine"
  [runners.cache]
    Type = "s3"
    Path = "path/to/prefix"
    Shared = false
    [runners.cache.s3]
      ServerAddress = "s3.example.com"
      AccessKey = "access-key"
      SecretKey = "secret-key"
      BucketName = "runner"
      Insecure = false
```

上記の例では、S3 URLは`http(s)://<ServerAddress>/<BucketName>/<Path>/runner/<runner-id>/project/<id>/<cache-key>`構造に従っています。

2つ以上のRunnerの間でキャッシュを共有するには、`Shared`フラグをtrueに設定します。このフラグにより、URLからRunnerトークン（`runner/<runner-id>`）が削除され、設定されているすべてのRunnerが同じキャッシュを共有するようになります。キャッシュ共有が有効になっている場合にRunner間でキャッシュを分離するために、`Path`を設定することもできます。

## 分散コンテナレジストリミラーリング

Dockerコンテナ内で実行されるジョブを高速化するには、[Dockerレジストリミラーリングサービス](https://docs.docker.com/retired/#registry-now-cncf-distribution)を使用できます。このサービスは、Docker Machineと使用されているすべてのレジストリの間のプロキシを提供します。イメージはレジストリミラーによって1回ダウンロードされます。新しい各ホスト、またはイメージが利用できない既存のホストで、設定されたレジストリミラーからイメージがダウンロードされます。

ミラーがDocker MachineのLANに存在する場合、各ホストでのイメージのダウンロードステップははるかに高速になります。

Dockerレジストリミラーリングを設定するには、`config.toml`で設定に`MachineOptions`を追加する必要があります。

```toml
[[runners]]
  limit = 10
  executor = "docker+machine"
  [runners.machine]
    (...)
    MachineOptions = [
      (...)
      "engine-registry-mirror=http://10.11.12.13:12345"
    ]
```

ここで`10.11.12.13:12345`は、レジストリミラーがDockerサービスからの接続をリッスンしているIPアドレスとポートです。Docker Machineによって作成された各ホストからアクセスできる必要があります。

[コンテナのプロキシの使用方法](../configuration/speed_up_job_execution.md#use-a-proxy-for-containers)の詳細を参照してください。

## 完全な`config.toml`の例

以下に示す`config.toml`では、[`google` Docker Machineドライバー](https://github.com/docker/docs/blob/173d3c65f8e7df2a8c0323594419c18086fc3a30/machine/drivers/gce.md)が使用されています。

```toml
concurrent = 50   # All registered runners can run up to 50 concurrent jobs

[[runners]]
  url = "https://gitlab.com"
  token = "RUNNER_TOKEN"             # Note this is different from the registration token used by `gitlab-runner register`
  name = "autoscale-runner"
  executor = "docker+machine"        # This runner is using the 'docker+machine' executor
  limit = 10                         # This runner can execute up to 10 jobs (created machines)
  [runners.docker]
    image = "ruby:2.7"               # The default image used for jobs is 'ruby:2.7'
  [runners.machine]
    IdleCount = 5                    # There must be 5 machines in Idle state - when Off Peak time mode is off
    IdleTime = 600                   # Each machine can be in Idle state up to 600 seconds (after this it will be removed) - when Off Peak time mode is off
    MaxBuilds = 100                  # Each machine can handle up to 100 jobs in a row (after this it will be removed)
    MachineName = "auto-scale-%s"    # Each machine will have a unique name ('%s' is required)
    MachineDriver = "google" # Refer to Docker Machine docs on how to authenticate: https://docs.docker.com/machine/drivers/gce/#credentials
    MachineOptions = [
      "google-project=GOOGLE-PROJECT-ID",
      "google-zone=GOOGLE-ZONE", # e.g. 'us-central-1'
      "google-machine-type=GOOGLE-MACHINE-TYPE", # e.g. 'n1-standard-8'
      "google-machine-image=ubuntu-os-cloud/global/images/family/ubuntu-1804-lts",
      "google-username=root",
      "google-use-internal-ip",
      "engine-registry-mirror=https://mirror.gcr.io"
    ]
    [[runners.machine.autoscaling]]  # Define periods with different settings
      Periods = ["* * 9-17 * * mon-fri *"] # Every workday between 9 and 17 UTC
      IdleCount = 50
      IdleCountMin = 5
      IdleScaleFactor = 1.5 # Means that current number of Idle machines will be 1.5*in-use machines,
                            # no more than 50 (the value of IdleCount) and no less than 5 (the value of IdleCountMin)
      IdleTime = 3600
      Timezone = "UTC"
    [[runners.machine.autoscaling]]
      Periods = ["* * * * * sat,sun *"] # During the weekends
      IdleCount = 5
      IdleTime = 60
      Timezone = "UTC"
  [runners.cache]
    Type = "s3"
    [runners.cache.s3]
      ServerAddress = "s3.eu-west-1.amazonaws.com"
      AccessKey = "AMAZON_S3_ACCESS_KEY"
      SecretKey = "AMAZON_S3_SECRET_KEY"
      BucketName = "runner"
      Insecure = false
```

`MachineOptions`パラメーターには、Docker MachineがGoogle Compute Engineでマシンを作成するために使用する`google`ドライバーのオプションと、Docker Machine自体のオプション（`engine-registry-mirror`）の両方が含まれています。
