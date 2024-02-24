


```

┌┬┐┌┬┐┌─┐┌─┐┬ ┬┌─┐┌┐
 │││││└─┐│ ┬│││├┤ ├┴┐
─┴┘┴ ┴└─┘└─┘└┴┘└─┘└─┘
DMSG resolving proxy & browser client - access websites over dmsg

Usage:
web

Available Commands:
completion   Generate the autocompletion script for the specified shell
gen-keys     generate public / secret keypair

Flags:
-d, --dmsg-disc string   dmsg discovery url default:
                         http://dmsgd.skywire.skycoin.com
-f, --filter string      domain suffix to filter (default ".dmsg")
-l, --loglvl string      [ debug | warn | error | fatal | panic | trace | info ]
-p, --port string        port to serve the web application (default "8080")
-r, --proxy string       configure additional socks5 proxy for dmsgweb (i.e. 127.0.0.1:1080)
-t, --resolve string     resolve the specified dmsg address:port on the local port & disable proxy
-e, --sess int           number of dmsg servers to connect to (default 1)
-s, --sk cipher.SecKey   a random key is generated if unspecified
(default 0000000000000000000000000000000000000000000000000000000000000000)
-q, --socks string       port to serve the socks5 proxy (default "4445")
-v, --version            version for web

```
