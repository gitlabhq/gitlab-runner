#!/bin/sh

echo "this has been executed through a custom entrypoint" >> /tmp/debug.log

su-exec nobody /bin/sh
