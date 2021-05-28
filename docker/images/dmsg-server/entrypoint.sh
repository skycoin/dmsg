#!/usr/bin/env sh

if [ "$CONFIG_FILE_PATH" != "" ]; then
  dmsg-server "$CONFIG_FILE_PATH" "$@"
else
  dmsg-server "$@"
fi
