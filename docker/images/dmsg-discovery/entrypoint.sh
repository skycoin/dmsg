#!/usr/bin/env sh

if [ "$REDIS_DSN" != "" ]; then
  dmsg-discovery --redis redis://"${REDIS_DSN}" "$@"
else
  sh -c "$@"
fi
