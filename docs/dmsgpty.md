# Dmsgpty
`dmsgpty` is an remote shell utility over `dmsg` (similar concept to SSH), to connect to the servers hosted over the `dmsg` network.

```
$ ./bin/dmsgpty-host --help
    runs a standalone dmsgpty-host instance

    Usage:
    dmsgpty-host [flags]
    dmsgpty-host [command]

    Available Commands:
    confgen     generates config file
    help        Help about any command

    Flags:
        --cliaddr string      address used for listening for cli connections (default "/tmp/dmsgpty.sock")
        --clinet string       network used for listening for cli connections (default "unix")
    -c, --confpath string     config path (default "./config.json")
        --confstdin           config will be read from stdin if set
        --dmsgdisc string     dmsg discovery address (default "http://dmsg.discovery.skywire.skycoin.com")
        --dmsgport uint16     dmsg port for listening for remote hosts (default 22)
        --dmsgsessions int    minimum number of dmsg sessions to ensure (default 1)
        --envprefix string    env prefix (default "DMSGPTY")
    -h, --help                help for dmsgpty-host
        --sk cipher.SecKey    secret key of the dmsgpty-host (default 0000000000000000000000000000000000000000000000000000000000000000)
        --wl cipher.PubKeys   whitelist of the dmsgpty-host (default public keys:
                                )

    Use "dmsgpty-host [command] --help" for more information about a command.
```

```
$ ./bin/dmsgpty-cli --help
    Run commands over dmsg

    Usage:
    dmsgpty-cli [flags]
    dmsgpty-cli [command]

    Available Commands:
    help             Help about any command
    whitelist        lists all whitelisted public keys
    whitelist-add    adds public key(s) to the whitelist
    whitelist-remove removes public key(s) from the whitelist

    Flags:
        --addr dmsg.Addr    remote dmsg address of format 'pk:port'. If unspecified, the pty will start locally (default 000000000000000000000000000000000000000000000000000000000000000000:~)
    -a, --args strings      command arguments
        --cliaddr string    address to use for dialing to dmsgpty-host (default "/tmp/dmsgpty.sock")
        --clinet string     network to use for dialing to dmsgpty-host (default "unix")
    -c, --cmd string        name of command to run (default "/bin/bash")
        --confpath string   config path (default "config.json")
    -h, --help              help for dmsgpty-cli

    Use "dmsgpty-cli [command] --help" for more information about a command.

```

## Example usage
In this example, we will use the `dmsg` network where the `dmsg.Discovery` address is `http://dmsg.discovery.skywire.skycoin.com`. However, any `dmsg.Discovery` would work.

### Share Files

First, lets generate a config file for the dmsgpty-host.

```shell script
// Generate config file 
$ ./bin/dmsgpty-host confgen
```
Config file will be generated.
```json
{
    "dmsgdisc": "http://dmsg.discovery.skywire.skycoin.com",
    "dmsgsessions": 1,
    "dmsgport": 22,
    "clinet": "unix",
    "cliaddr": "/tmp/dmsgpty.sock",
    "sk": "8770be1ae64aa22a6d442086dc5870339a4d402c10e30499fa8a53d34413d412",
    "pk": "03d3d3744f7d6a943b3d467fce8477ccc580b7568160346b8d8bbd95e343ad6be4",
    "wl": null
}
```
To start the `dmsgpty-host` simply run

```shell script
$ ./bin/dmsgpty-host
```
To interact with this host use `dmsgpty-cli`.<br>
`dmsgpty-cli` can be used to view, add or remove whitelist.
To view the whitelist run the following in a new terminal.
```shell script
$ ./bin/dmsgpty-cli whitelist
```

To add a whitelist use the following command with a Public key of a node you want to whitelist.
```shell script
$ ./bin/dmsgpty-cli whitelist-add 0278a4adc9071c695992d27123c5be7075abe369b1ef6cb4ee2716ac9151843d00
```

To remove a whitelist use the following command with a Public key of a node you want to remove.
```shell script
$ ./bin/dmsgpty-cli whitelist-remove 0278a4adc9071c695992d27123c5be7075abe369b1ef6cb4ee2716ac9151843d00
```

To start the `dmsgpty-ui` simply run

```shell script
$ ./bin/dmsgpty-ui
```

And open the browser at http://127.0.0.1:8080/

### Connect Two Local DMSG Hosts with DMSGPTY

First, lets generate a config file for the `dmsgpty-host 1`.
```shell script
// Generate config file 
$ ./bin/dmsgpty-host confgen
```
Config file will be generated for the `dmsgpty-host 1`..
```json
{
    "dmsgdisc": "http://dmsg.discovery.skywire.skycoin.com",
    "dmsgsessions": 1,
    "dmsgport": 22,
    "clinet": "unix",
    "cliaddr": "/tmp/dmsgpty.sock",
    "sk": "8770be1ae64aa22a6d442086dc5870339a4d402c10e30499fa8a53d34413d412",
    "pk": "03d3d3744f7d6a943b3d467fce8477ccc580b7568160346b8d8bbd95e343ad6be4",
    "wl": null
}
```

Now, lets generate a config file for the `dmsgpty-host 2`.<br>
We are changing the cliaddress since both the hosts are on the same machine and same cliaddr will clash.
```shell script
// Generate config file 
$ ./bin/dmsgpty-host confgen config2.json --cliaddr /tmp/dmsgpty2.sock
```
Config file will be generated for the `dmsgpty-host 2`..
```json
{
  "dmsgdisc": "http://dmsg.discovery.skywire.skycoin.com",
  "dmsgsessions": 1,
  "dmsgport": 22,
  "clinet": "unix",
  "cliaddr": "/tmp/dmsgpty2.sock",
  "sk": "76cc80ea9dcc8cbbb54d5463cea8797dd4ed27693daf176878a8d0929a4466d3",
  "pk": "024e804f8e8fc3c4fc8562a5e58c4897323e527dace63ec36badfb66b65d4606d7",
  "wl": null
}
```

To start the `dmsgpty-host 1` simply run
```shell script
$ ./bin/dmsgpty-host
```

To start the `dmsgpty-host 2` simply run the following in a new terminal 
```shell script
$ ./bin/dmsgpty-host -c ./config2.json
```

To interact with the hosts use `dmsgpty-cli`.<br>
`dmsgpty-cli` can be used to view, add or remove whitelist.

Now whitelist the Public key of `dmsgpty-host 1` in `dmsgpty-host 2`.<br>
So that `dmsgpty-host 2` will accept connection request from `dmsgpty-host 1`
```shell script
$ ./bin/dmsgpty-cli whitelist-add 03d3d3744f7d6a943b3d467fce8477ccc580b7568160346b8d8bbd95e343ad6be4 --cliaddr /tmp/dmsgpty2.sock

```
Now connect to the shell of `dmsgpty-host 2` from `dmsgpty-host 1` run
```shell script
$ ./bin/dmsgpty-cli --addr 024e804f8e8fc3c4fc8562a5e58c4897323e527dace63ec36badfb66b65d4606d7
```

To exit from the shell of `dmsgpty-host 2` run
```shell script
$ exit
```