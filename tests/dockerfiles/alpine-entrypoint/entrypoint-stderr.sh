#!/bin/sh -e

echo "entrypoint stdout message" >&2

exec "$@"