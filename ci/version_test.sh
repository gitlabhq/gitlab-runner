#!/usr/bin/env bash

set -eo pipefail

script="$(cd "$(dirname "$0")" && pwd)/version"
tmp=$(mktemp -d)
trap 'rm -rf "${tmp}"' EXIT

export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com
export GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com

cd "${tmp}"
git init -q .
git checkout -q -b stable
mkdir ci
cp "${script}" ci/version
echo 19.2.0 > VERSION
git add . && git commit -q -m "cut"
git tag -a v19.2.0 -m v19.2.0

failures=0
expect() {
    local want_re=$1 got
    got=$(./ci/version)
    if [[ "${got}" =~ ${want_re} ]]; then
        echo "ok: ${got}"
    else
        echo "FAIL: got ${got}, want match ${want_re}"
        failures=$((failures + 1))
    fi
}

git commit -q --allow-empty -m "fix"
expect '^19\.2\.1~pre\.1\.g[0-9a-f]{8}$'

git tag -a v19.2.1 -m v19.2.1
expect '^19\.2\.1$'

git commit -q --allow-empty -m "fix 2"
expect '^19\.2\.2~pre\.1\.g[0-9a-f]{8}$'

git tag -a v19.2.2-rc1 -m v19.2.2-rc1
git commit -q --allow-empty -m "fix 3"
expect '^19\.2\.2~pre\.2\.g[0-9a-f]{8}$'

# Development branch: VERSION is the next minor, ahead of the last tag.
echo 19.3.0 > VERSION
git commit -q -am "next minor"
expect '^19\.3\.0~pre\.3\.g[0-9a-f]{8}$'

exit "${failures}"
