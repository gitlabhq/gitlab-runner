#!/usr/bin/env bash

case $1 in
  get)
    cat <<EOF
{
  "username": "<token>",
  "secret": "script_token_1"
}
EOF
    ;;
  *)
    echo "Unknown option '$1'" >/dev/stderr
esac
