#!/usr/bin/env bash

set -eo pipefail

script="$(cd "$(dirname "$0")" && pwd)/version"
tmp=$(mktemp -d)
trap 'rm -rf "${tmp}"' EXIT

export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null
export GIT_AUTHOR_NAME=t GIT_AUTHOR_EMAIL=t@example.com
export GIT_COMMITTER_NAME=t GIT_COMMITTER_EMAIL=t@example.com

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

commit() {
    git commit -q --allow-empty -m "$1"
}

new_repo() {
    rm -rf "${tmp}/repo"
    mkdir "${tmp}/repo"
    cd "${tmp}/repo"
    git init -q -b main .
    mkdir ci
    cp "${script}" ci/version
}

# Release tags sit on a stable branch that is not merged back into main.
new_repo
echo 19.5.0 > VERSION
git add . && commit "first"
echo 19.6.0 > VERSION
git commit -q -am "bump"
git checkout -q -b stable
commit "stable only"
git tag -a v19.5.0 -m v19.5.0
git checkout -q main
commit "main 1"
commit "main 2"
expect '^19\.6\.0~pre\.2\.g[0-9a-f]{8}$'

# An older tag that is an ancestor of main does not set the count.
git tag -a v19.4.0 -m v19.4.0 HEAD~3
expect '^19\.6\.0~pre\.2\.g[0-9a-f]{8}$'

# Release candidate tags are ignored.
git checkout -q stable
commit "rc"
git tag -a v19.6.0-rc1 -m v19.6.0-rc1
git checkout -q main
expect '^19\.6\.0~pre\.2\.g[0-9a-f]{8}$'

# Stable branch: the tag is an ancestor and the count starts there.
git checkout -q stable
commit "fix"
echo 19.5.0 > VERSION
git commit -q -am "version"
expect '^19\.5\.1~pre\.3\.g[0-9a-f]{8}$'

# New major: x.0.0 on main counts from the VERSION bump.
new_repo
echo 19.9.0 > VERSION
git add . && commit "first"
git checkout -q -b stable
commit "stable only"
git tag -a v19.9.0 -m v19.9.0
git checkout -q main
echo 20.0.0 > VERSION
git commit -q -am "bump"
commit "main 1"
expect '^20\.0\.0~pre\.1\.g[0-9a-f]{8}$'

# No tags at all.
new_repo
echo 19.6.0 > VERSION
git add . && commit "first"
commit "second"
expect '^19\.6\.0~pre\.1\.g[0-9a-f]{8}$'

# A tag of the same minor that is an ancestor sets the count.
git checkout -q -b stable
git tag -a v19.6.0 -m v19.6.0
git checkout -q main
commit "third"
expect '^19\.6\.1~pre\.1\.g[0-9a-f]{8}$'

# Two-digit minors sort after single-digit ones.
new_repo
echo 19.8.0 > VERSION
git add . && commit "first"
git checkout -q -b stable-8
commit "stable only"
git tag -a v19.8.0 -m v19.8.0
git checkout -q main
commit "main 1"
echo 19.9.0 > VERSION
git commit -q -am "bump 9"
git checkout -q -b stable-9
commit "stable only"
git tag -a v19.9.0 -m v19.9.0
git checkout -q main
echo 19.10.0 > VERSION
git commit -q -am "bump 10"
commit "main 2"
expect '^19\.10\.0~pre\.1\.g[0-9a-f]{8}$'

# A tag of VERSION's own minor on a stable branch does not set the base.
new_repo
echo 19.5.0 > VERSION
git add . && commit "first"
git checkout -q -b stable-5
commit "stable only"
git tag -a v19.5.0 -m v19.5.0
git checkout -q main
echo 19.6.0 > VERSION
git commit -q -am "bump 6"
git checkout -q -b stable-6
commit "stable only"
git tag -a v19.6.0 -m v19.6.0
git checkout -q main
commit "main 1"
expect '^19\.6\.0~pre\.1\.g[0-9a-f]{8}$'

# The count does not drop when the previous release is tagged after the bump.
new_repo
echo 19.5.0 > VERSION
git add . && commit "first"
git checkout -q -b stable
commit "stable only"
git checkout -q main
echo 19.6.0 > VERSION
git commit -q -am "bump"
commit "main 1"
commit "main 2"
expect '^19\.6\.0~pre\.2\.g[0-9a-f]{8}$'
git checkout -q stable
git tag -a v19.5.0 -m v19.5.0
git checkout -q main
expect '^19\.6\.0~pre\.2\.g[0-9a-f]{8}$'

# Shallow clone that does not reach the VERSION bump: fail.
new_repo
echo 19.5.0 > VERSION
git add . && commit "first"
echo 19.6.0 > VERSION
git commit -q -am "bump"
git checkout -q -b stable
commit "stable only"
git tag -a v19.5.0 -m v19.5.0
git checkout -q main
commit "main 1"
git clone -q --depth 1 "file://${tmp}/repo" "${tmp}/shallow"
cd "${tmp}/shallow"
if ./ci/version 2>/dev/null; then
    echo "FAIL: shallow clone without the VERSION bump printed a version"
    failures=$((failures + 1))
else
    echo "ok: shallow clone fails"
fi

exit "${failures}"
