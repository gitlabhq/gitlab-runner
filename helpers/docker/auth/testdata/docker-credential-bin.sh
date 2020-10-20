#!/usr/bin/env bash

case $1 in
  get)
    cat <<EOF
{
  "username": "script_user_1",
  "secret": "script_password_1",
  "serveraddress": "https://registry2.domain.tld:5005/v1/"
}
EOF
    ;;
  list)
    cat <<EOF
{
  "https://registry2.domain.tld:5005/v1/": "script_user_1"
}
EOF
    ;;
  *)
    echo "Unknown option '$1'" >/dev/stderr
esac
