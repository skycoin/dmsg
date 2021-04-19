# Dmsgpty
`dmsgpty` is a utility exec which uses SSH tp connect to servers hosted over the `dmsg` network (similar to a simplified `SSH` over `dmsg`).

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

### Example usage

In this example, we will use the `dmsg` network where the `dmsg.Discovery` address is `http://dmsg.discovery.skywire.skycoin.com`. However, any `dmsg.Discovery` would work.

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
    "sk": "25d06ea48daeb34191be5afb4bffac00932c65796586a0969066546345b1c627",
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