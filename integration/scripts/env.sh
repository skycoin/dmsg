#!/usr/bin/env bash

# tmux session: db
export S_DB=db
export W_REDIS=redis

# func_print prepends function name to echoed message.
function func_print() {
  echo "${FUNCNAME[1]}(): $*"
}

# init_redis runs a redis instance in a tmux window if none is running.
function init_redis() {
  if [[ "$(redis-cli ping 2>&1)" != "PONG" ]]; then
    tmux new -d -s "${S_DB}"
    tmux new-window -a -t "${S_DB}" -n "${W_REDIS}"
    tmux send-keys -t "${W_REDIS}" 'redis-server' C-m
  else
    func_print "redis-server already running"
  fi
}

# stop_redis stops redis and it's associated tmux session/window.
function stop_redis() {
  if [[ "$(redis-cli ping 2>&1)" != "PONG" ]]; then
    func_print "redis-server is not running, nothing to be done."
  else
    killall redis-server
  fi
}

# TODO(evanlinjin): Finish this.
function init_dmsg() {
  init_redis

  if [[ $# -ne 0 ]]; then
    func_print "expected 0 args, got $#" 1>&2
    exit 1
  fi
}