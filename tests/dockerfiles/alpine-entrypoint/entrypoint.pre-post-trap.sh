#!/usr/bin/env bash

# set -e
set -u
set -o pipefail

LOOP_ITERATIONS="${LOOP_ITERATIONS:-30}"

pre() {
  counter="${LOOP_ITERATIONS}"
  while (( counter-- > 0 )) ; do
    echo "[entrypoint][pre][stdout][${counter}/${LOOP_ITERATIONS}] some pre message on stdout"
    echo "[entrypoint][pre][stderr][${counter}/${LOOP_ITERATIONS}] some pre message on stderr" >&2
    sleep 1
  done

  echo >&2 '----[ CMD ]---->'
}

post() {
  echo >&2 '----[ CMD ]----<'

  counter="${LOOP_ITERATIONS}"
  while (( counter-- > 0 )) ; do
    echo "[entrypoint][post][stdout][${counter}/${LOOP_ITERATIONS}] some post message on stdout"
    echo "[entrypoint][post][stderr][${counter}/${LOOP_ITERATIONS}] some post message on stderr" >&2
    sleep 1
  done
}

trap post EXIT

pre || true
"$@"
