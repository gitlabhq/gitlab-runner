#!/usr/bin/env bash

case $1 in
  get)
    cat <<EOF
{
  "username": "pinned_user_1",
  "secret": "pinned_password_1"
}
EOF
    ;;
  *)
    echo "Unknown option '$1'" >/dev/stderr
esac
