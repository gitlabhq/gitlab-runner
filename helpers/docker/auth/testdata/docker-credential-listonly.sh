#!/usr/bin/env bash

case $1 in
  list)
    cat <<EOF
{
  "https://registry2.domain.tld:5005/v1/": "script_user_1"
}
EOF
    ;;
  *)
    echo "get is not supported by this helper" >/dev/stderr
    exit 1
esac
