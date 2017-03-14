#!/bin/sh

set -e
docker version >/dev/null 2>/dev/null

echo Clearing docker cache...

CONTAINERS=$(docker ps -a -q \
             --filter=status=exited \
             --filter=status=dead \
             --filter=label=com.gitlab.gitlab-runner.type=cache)

if [ -n "${CONTAINERS}" ]; then
    docker rm -v ${CONTAINERS}
fi
