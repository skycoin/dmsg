#!/usr/bin/env bash

# Ensure our aliases can be used in tmux windows.
SOURCE_SELF="source ./integration/env.sh"

# tmux session: db
DB=db
DB_REDIS=db_redis

# tmux session: dmsg
DMSG=dmsg
DMSG_DISC=dmsg_disc
DMSG_SRV1=dmsg_srv1
DMSG_SRV2=dmsg_srv2

# tmux session: dmsgpty
DMSGPTY=dmsgpty
DMSGPTY_H1=dmsgpty_h1
DMSGPTY_H2=dmsgpty_h2

# dmsgpty_h1.
export H1_CLIADDR="/tmp/dmsgpty1.sock"
export H1_DMSGDISC="http://127.0.0.1:9090"
export H1_PK="0392d691a628563fe48fd708a2c7fcafe0c6ef5057e7b4a4d82bf800da19d42e3f"
export H1_SK="eb92978d07e8d97d4de730d283823c50a79fbbc1c96c546f8390b4b297c08e4f"
alias dmsgpty1-host='./bin/dmsgpty-host --envprefix="H1"'
alias dmsgpty1-cli='./bin/dmsgpty-cli --cliaddr=${H1_CLIADDR}'

# dmsgpty_h2.
export H2_CLIADDR="/tmp/dmsgpty2.sock"
export H2_DMSGDISC="http://127.0.0.1:9090"
export H2_PK="025936da46d7e3691a1de824c60456d35c08994c23ec4cf6c15a16d9d162b677d3"
export H2_SK="bc87d93ca327d76a6825ee36ae307597bfc89394269fd0c961b1e2e8021ae738"
alias dmsgpty2-host='./bin/dmsgpty-host --envprefix="H2"'
alias dmsgpty2-cli='./bin/dmsgpty-cli --cliaddr=${H2_CLIADDR}'

function catch_ec() {
    if [[ $1 -ne 0 ]]; then
        echo "last command exited with non-zero exit code: $1"
        exit $1
    fi
}

function print_dmsgpty_help() {
  echo ""
  echo "# DMSGPTY LOCAL ENVIRONMENT HELP:"
  echo ""

  echo "## HOST 1 - ENVS:"
  printenv | grep "^H1_"
  echo ""

  echo "## HOST 1 - ALIASES:"
  alias | grep "dmsgpty1"
  echo ""

  echo "## HOST 2 - ENVS:"
  printenv | grep "^H2_"
  echo ""

  echo "## HOST 2 - ALIASES:"
  alias | grep "dmsgpty2"
  echo ""
}

# func_print prepends function name to echoed message.
function func_print() {
  echo "env.sh [${FUNCNAME[1]}] $*"
}

# has_session returns whether a tmux session of given name exists or not.
# Input 1: session name.
function has_session() {
  if [[ $# -ne 1 ]]; then
    func_print "expected 1 arg(s), got $#" 1>&2
    exit 1
  fi

  session_name=$1

  [[ $(tmux ls | grep "${session_name}") == "${session_name}:"* ]]
}

# send_to_all_windows sends a command to all windows of a given tmux session.
# Input 1: tmux session name.
function send_to_all_windows() {
  if [[ $# -ne 2 ]]; then
    func_print "expected 2 arg(s), got $#" 1>&2
    exit 1
  fi

  session_name=$1
  cmd_name=$2

  for W_NAME in $(tmux list-windows -F '#W' -t "${session_name}"); do
    tmux send-keys -t "${W_NAME}" "${cmd_name}" C-m
  done
}

# is_redis_running returns whether redis is running.
function is_redis_running() {
  [[ "$(redis-cli ping 2>&1)" == "PONG" ]]
}

# init_redis runs a redis instance in a tmux window if none is running.
function init_redis() {
  if is_redis_running; then
    func_print "redis-server already running, nothing to be done"
    return 0
  fi

  if has_session ${DB}; then
    func_print "tmux session ${DB} will be killed before restarting"
    tmux kill-session -t ${DB}
  fi

  tmux new -d -s ${DB}
  tmux new-window -a -t ${DB} -n ${DB_REDIS}
  tmux send-keys -t ${DB_REDIS} 'redis-server' C-m

  # Wait until redis is up and running
  for i in {1..5}; do
    sleep 0.5
    if is_redis_running; then
      func_print "attempt $i: redis-server started"
      tmux select-window -t bash
      return 0
    fi
    func_print "attempt $i: redis-server not started, checking again in 0.5s..."
  done

  func_print "failed to start redis-server"
  exit 1
}

# stop_redis stops redis and it's associated tmux session/window.
function stop_redis() {
  if [[ "$(redis-cli ping 2>&1)" != "PONG" ]]; then
    func_print "redis-server is not running, nothing to be done."
  elif tmux kill-session -t ${DB};
    then killall redis-server
  fi
}

function attach_redis() {
  tmux attach -t ${DB}
}

function init_dmsg() {
  if has_session ${DMSG}; then
    func_print "Session already running, nothing to be done here."
    return 0
  fi

  # dmsg session depends on redis.
  init_redis

  func_print "Creating ${DMSG} tmux session..."
  tmux new -d -s ${DMSG}
  tmux new-window -a -t ${DMSG} -n ${DMSG_DISC}
  tmux new-window -a -t ${DMSG} -n ${DMSG_SRV1}
  tmux new-window -a -t ${DMSG} -n ${DMSG_SRV2}
  send_to_all_windows ${DMSG} "${SOURCE_SELF}"

  func_print "Running ${DMSG_DISC}..."
  tmux send-keys -t ${DMSG_DISC} './bin/dmsg-discovery -t' C-m
  catch_ec $?

  func_print "Running ${DMSG_SRV1}..."
  tmux send-keys -t ${DMSG_SRV1} './bin/dmsg-server ./integration/configs/dmsgserver1.json' C-m
  catch_ec $?

  func_print "Running ${DMSG_SRV2}..."
  tmux send-keys -t ${DMSG_SRV2} './bin/dmsg-server ./integration/configs/dmsgserver2.json' C-m
  catch_ec $?
  sleep 1
  func_print "${DMSG} session started successfully."
  tmux select-window -t bash
}

function stop_dmsg() {
  tmux kill-session -t ${DMSG}
  return 0
}

function attach_dmsg() {
  tmux attach -t ${DMSG}
}

function init_dmsgpty() {
  if has_session ${DMSGPTY}; then
    func_print "Session already running, nothing to be done here."
    return 0
  fi

  # dmsgpty session depends on dmsg.
  init_dmsg

  func_print "Creating ${DMSGPTY} tmux session..."
  tmux new -d -s ${DMSGPTY}
  tmux new-window -a -t ${DMSGPTY} -n ${DMSGPTY_H1}
  tmux new-window -a -t ${DMSGPTY} -n ${DMSGPTY_H2}
  send_to_all_windows ${DMSGPTY} "${SOURCE_SELF}"

  func_print "Running ${DMSGPTY_H1}..."
  tmux send-keys -t ${DMSGPTY_H1} "${SOURCE_SELF} && dmsgpty1-host" C-m
  catch_ec $?

  func_print "Running ${DMSGPTY_H2}..."
  tmux send-keys -t ${DMSGPTY_H2} "${SOURCE_SELF} && dmsgpty2-host" C-m
  catch_ec $?

  sleep 1
  tmux send-keys -t bash "dmsgpty1-cli whitelist-add ${H2_PK} && dmsgpty2-cli whitelist-add ${H1_PK} && print_dmsgpty_help" C-m

  func_print "${DMSGPTY} session started successfully."
  tmux select-window -t bash
}

function stop_dmsgpty() {
    tmux kill-session -t ${DMSGPTY}
    return 0
}

function attach_dmsgpty() {
    tmux attach -t ${DMSGPTY}
}
