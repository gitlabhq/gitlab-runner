---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Custom executorでLXDを使用する
---

{{< details >}}

- プラン: Free、Premium、Ultimate
- 提供形態: GitLab.com、GitLab Self-Managed、GitLab Dedicated

{{< /details >}}

この例では、LXDを使用して、ビルドごとにコンテナを作成し、後でクリーンアップします。

この例では、各ステージにbashスクリプトを使用します。独自のイメージを指定でき、これは[CI_JOB_IMAGE](https://docs.gitlab.com/ci/variables/predefined_variables/)として公開されます。この例では、簡潔にするために`ubuntu:22.04`イメージを使用しています。複数のイメージをサポートする場合は、executorを変更する必要があります。

これらのスクリプトには、次の前提条件があります:

- [LXD](https://canonical.com/lxd)
- [GitLab Runner](../../install/linux-manually.md)

## 設定 {#configuration}

```toml
[[runners]]
  name = "lxd-driver"
  url = "https://gitlab.example.com"
  token = "xxxxxxxxxxx"
  executor = "custom"
  builds_dir = "/builds"
  cache_dir = "/cache"
  [runners.custom]
    prepare_exec = "/opt/lxd-driver/prepare.sh" # Path to a bash script to create lxd container and download dependencies.
    run_exec = "/opt/lxd-driver/run.sh" # Path to a bash script to run script inside the container.
    cleanup_exec = "/opt/lxd-driver/cleanup.sh" # Path to bash script to delete container.
```

## Base {#base}

各ステージ ([prepare](#prepare)、[run](#run)、[cleanup](#cleanup)) は、このスクリプトを使用して、スクリプト全体で使用される変数を生成します。

このスクリプトが他のスクリプトと同じディレクトリにあることが重要です。この場合、`/opt/lxd-driver/`です。

```shell
#!/usr/bin/env bash

# /opt/lxd-driver/base.sh

CONTAINER_ID="runner-$CUSTOM_ENV_CI_RUNNER_ID-project-$CUSTOM_ENV_CI_PROJECT_ID-concurrent-$CUSTOM_ENV_CI_CONCURRENT_PROJECT_ID-$CUSTOM_ENV_CI_JOB_ID"
```

## Prepare {#prepare}

準備スクリプトは次のことを行います:

- 同じ名前のコンテナが実行中の場合は、そのコンテナを削除します。
- コンテナを起動し、起動するまで待ちます。
- [前提条件となる依存関係](../custom.md#prerequisite-software-for-running-a-job)をインストールします。

```shell
#!/usr/bin/env bash

# /opt/lxd-driver/prepare.sh

currentDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${currentDir}/base.sh # Get variables from base.

set -eo pipefail

# trap any error, and mark it as a system failure.
trap "exit $SYSTEM_FAILURE_EXIT_CODE" ERR

start_container () {
    if lxc info "$CONTAINER_ID" >/dev/null 2>/dev/null ; then
        echo 'Found old container, deleting'
        lxc delete -f "$CONTAINER_ID"
    fi

    # The container image is hardcoded, but you can use
    # the `CI_JOB_IMAGE` predefined variable
    # https://docs.gitlab.com/ci/variables/predefined_variables/
    # which is available under `CUSTOM_ENV_CI_JOB_IMAGE` to allow the
    # user to specify the image. The rest of the script assumes that
    # you are running on an ubuntu image so modifications might be
    # required.
    lxc launch ubuntu:22.04 "$CONTAINER_ID"

    # Wait for container to start, we are using systemd to check this,
    # for the sake of brevity.
    for i in $(seq 1 10); do
        if lxc exec "$CONTAINER_ID" -- sh -c "systemctl isolate multi-user.target" >/dev/null 2>/dev/null; then
            break
        fi

        if [ "$i" == "10" ]; then
            echo 'Waited for 10 seconds to start container, exiting..'
            # Inform GitLab Runner that this is a system failure, so it
            # should be retried.
            exit "$SYSTEM_FAILURE_EXIT_CODE"
        fi

        sleep 1s
    done
}

install_dependencies () {
    # Install Git LFS, git comes pre installed with ubuntu image.
    lxc exec "$CONTAINER_ID" -- sh -c 'curl -s "https://packagecloud.io/install/repositories/github/git-lfs/script.deb.sh" | sudo bash'
    lxc exec "$CONTAINER_ID" -- sh -c "apt-get install -y git-lfs"

    # Install gitlab-runner binary since we need for cache/artifacts.
    lxc exec "$CONTAINER_ID" -- sh -c 'curl -L --output /usr/local/bin/gitlab-runner "https://gitlab-runner-downloads.s3.amazonaws.com/latest/binaries/gitlab-runner-linux-amd64"'
    lxc exec "$CONTAINER_ID" -- sh -c "chmod +x /usr/local/bin/gitlab-runner"
}

echo "Running in $CONTAINER_ID"

start_container

install_dependencies
```

## Run {#run}

これは、スクリプトの内容を`STDIN`経由でコンテナに送信することにより、GitLab Runnerによって生成されたスクリプトを実行します。

```shell
#!/usr/bin/env bash

# /opt/lxd-driver/run.sh

currentDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${currentDir}/base.sh # Get variables from base.

lxc exec "$CONTAINER_ID" /bin/bash < "${1}"
if [ $? -ne 0 ]; then
    # Exit using the variable, to make the build as failure in GitLab
    # CI.
    exit $BUILD_FAILURE_EXIT_CODE
fi
```

## Cleanup {#cleanup}

ビルドが完了したため、コンテナを削除します。

```shell
#!/usr/bin/env bash

# /opt/lxd-driver/cleanup.sh

currentDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${currentDir}/base.sh # Get variables from base.

echo "Deleting container $CONTAINER_ID"

lxc delete -f "$CONTAINER_ID"
```
