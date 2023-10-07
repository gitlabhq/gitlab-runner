#!/bin/sh

echo "this has been executed through a custom entrypoint overridden by the job response" >> /tmp/debug.log

su-exec nobody /bin/sh
