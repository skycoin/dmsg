#!/usr/bin/env bash

function helpFunction() {
    echo ""
    echo "Use: $0 -n (max 212) -u addr"
    echo "use -n for number of clients"
    echo "use -u for dsmg discovery URL (optional)"
    exit 1 # Exit script after printing help
}

function makeConfigs() {
    n=$1
    u=$2

    if [ -z "$n" ]
    then
        echo "-n cannot be empty";
        helpFunction
    fi

    if [ -z "$u" ]
    then
        u="localhost:9090"
    fi

    while [ $n -gt 0 ]
    do 
        # Generate config
        ./skywire-cli config gen -two config$n.json --disableapps skysocks,skysocks-client,skychat,vpn-server,vpn-client
	    sed -i 's/dmsgd.skywire.dev/$u/gI' config$n.json
        # increment the value
        n=`expr $n - 1`
    done
}

function startClients() {
    n=$1
    while [ $n -gt 0 ]
    do 
        # Start pty in xterm
        xterm -hold -title "App $n" -e "./skywire-visor -c ./config$n.json"&
        n=`expr $n - 1`
    done
}

while getopts "n:u:" opt
do
    case "$opt" in
        n ) numb="$OPTARG";;
        u ) url="$OPTARG"
            makeConfigs $numb $url
            startClients $numb;;
        ? ) helpFunction ;; # Print helpFunction in case parameter is non-existent
    esac
done

if [[ $# -ne 3 ]]; then
  helpFunction
  exit 0
fi
