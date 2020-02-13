#!/usr/bin/env bash

export S_DMSG=dmsg
export W_DB=redis
export W_DISC=dmsg_disc
export W_SRV1=dmsg_srv1
export W_SRV2=dmsg_srv2

tmux new -d -s ${S_DMSG}

# Only create redis instance if redis is not running.
if [[ "$(redis-cli ping 2>&1)" != "PONG" ]]; then
  tmux new-window -a -t ${S_DMSG} -n ${W_DB}
  tmux send-keys -t ${W_DB} 'redis-server' C-m
fi

tmux new-window -a -t ${S_DMSG} -n ${W_DISC}
tmux send-keys -t ${W_DISC} './bin/dmsg-discovery -t' C-m

tmux new-window -a -t ${S_DMSG} -n ${W_SRV1}
tmux send-keys -t ${W_SRV1} './bin/dmsg-server ./integration/configs/dmsgserver1.json' C-m

tmux new-window -a -t ${S_DMSG} -n ${W_SRV2}
tmux send-keys -t ${W_SRV2} './bin/dmsg-server ./integration/configs/dmsgserver2.json' C-m

tmux attach -t ${S_DMSG}
