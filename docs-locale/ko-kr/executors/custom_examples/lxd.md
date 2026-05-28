---
stage: Verify
group: Runner Core
info: To determine the technical writer assigned to the Stage/Group associated with this page, see <https://handbook.gitlab.com/handbook/product/ux/technical-writing/#assignments>
title: Custom 실행기와 함께 LXD 사용하기
---

{{< details >}}

- 계층:  Free, Premium, Ultimate
- 제공:  GitLab.com, GitLab Self-Managed, GitLab Dedicated

{{< /details >}}

이 예시에서는 LXD를 사용하여 빌드당 하나의 컨테이너를 만들고 그 후에 정리합니다.

이 예시는 각 스테이지마다 bash 스크립트를 사용합니다. 자신의 이미지를 지정할 수 있으며, 이는 [CI_JOB_IMAGE](https://docs.gitlab.com/ci/variables/predefined_variables/)로 노출됩니다. 이 예시는 단순함을 위해 `ubuntu:22.04` 이미지를 사용합니다. 여러 이미지를 지원하려면 실행기를 수정해야 합니다.

이 스크립트들의 전제 조건은 다음과 같습니다:

- [LXD](https://ubuntu.com/lxd)
- [러너](../../install/linux-manually.md)

## 구성 {#configuration}

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

각 스테이지의 [prepare](#prepare) , [run](#run) , [cleanup](#cleanup)은 이 스크립트를 사용하여 스크립트 전체에서 사용되는 변수를 생성합니다.

이 스크립트가 다른 스크립트와 같은 디렉토리에 위치하는 것이 중요하며, 이 경우 `/opt/lxd-driver/`입니다.

```shell
#!/usr/bin/env bash

# /opt/lxd-driver/base.sh

CONTAINER_ID="runner-$CUSTOM_ENV_CI_RUNNER_ID-project-$CUSTOM_ENV_CI_PROJECT_ID-concurrent-$CUSTOM_ENV_CI_CONCURRENT_PROJECT_ID-$CUSTOM_ENV_CI_JOB_ID"
```

## 준비 {#prepare}

prepare 스크립트는 다음을 수행합니다:

- 같은 이름의 컨테이너가 실행 중이면 삭제합니다.
- 컨테이너를 시작하고 시작될 때까지 대기합니다.
- [필수 종속성](../custom.md#prerequisite-software-for-running-a-job)을 설치합니다.

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

## 실행 {#run}

이는 러너가 생성한 스크립트를 실행하며, `STDIN`을 통해 스크립트의 내용을 컨테이너로 보냅니다.

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

## 정리 {#cleanup}

빌드가 완료되었으므로 컨테이너를 삭제합니다.

```shell
#!/usr/bin/env bash

# /opt/lxd-driver/cleanup.sh

currentDir="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null 2>&1 && pwd )"
source ${currentDir}/base.sh # Get variables from base.

echo "Deleting container $CONTAINER_ID"

lxc delete -f "$CONTAINER_ID"
```
