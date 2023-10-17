# Dmsgcurl

`dmsgcurl` is a utility exec which can download/upload from HTTP servers hosted over the `dmsg` network (similar to a simplified `curl` over `dmsg`).

```
$ dmsgcurl --help
              ┌┬┐┌┬┐┌─┐┌─┐┌─┐┬ ┬┬─┐┬  
               │││││└─┐│ ┬│  │ │├┬┘│  
              ─┴┘┴ ┴└─┘└─┘└─┘└─┘┴└─┴─┘

      Usage:
        dmsgcurl [OPTIONS] ... [URL] 

      Flags:
        -a, --agent AGENT        identify as AGENT (default "dmsgcurl/v1.2.0-184-gdb24d156")
        -d, --data string        dmsghttp POST data
        -c, --dmsg-disc string   dmsg discovery url default:
                                http://dmsgd.skywire.skycoin.com
        -l, --loglvl string      [ debug | warn | error | fatal | panic | trace | info ]
        -o, --out string         output filepath (default ".")
        -e, --sess int           number of dmsg servers to connect to (default 1)
        -s, --sk cipher.SecKey   a random key is generated if unspecified
        (default 0000000000000000000000000000000000000000000000000000000000000000)
        -n, --stdout             output to STDOUT
        -t, --try int            download attempts (0 unlimits) (default 1)
        -v, --version            version for dmsgcurl
        -w, --wait int           time to wait between fetches
```

### Example usage

In this example, we will use the `dmsg` network where the `dmsg.Discovery` address is `http://dmsgd.skywire.skycoin.com`. However, any `dmsg.Discovery` would work.

First, lets create a folder where we will host files to serve over `dmsg` and create a `hello.txt` file within.

```shell script
// Create serving folder.
$ mkdir /tmp/dmsghttp -p

// Create file.
$ echo 'Hello World!' > /tmp/dmsghttp/hello.txt
```

Next, let's serve this over `http` via `dmsg` as transport. We have an example exec for this located within `/example/dmsgget/dmsg-example-http-server`.

```shell script
# Generate public/private key pair
$ go run ./examples/dmsgget/gen-keys/gen-keys.go
#   PK: 038dde2d050803db59e2ad19e5a6db0f58f8419709fc65041c48b0cb209bb7a851
#   SK: e5740e093bd472c2730b0a58944a5dee220d415de62acf45d1c559f56eea2b2d

# Run dmsg http server.
#   (replace 'e5740e093bd472c2730b0a58944a5dee220d415de62acf45d1c559f56eea2b2d' with the SK returned from above command)
$ go run ./examples/dmsgget/dmsg-example-http-server/dmsg-example-http-server.go --dir /tmp/dmsghttp --sk e5740e093bd472c2730b0a58944a5dee220d415de62acf45d1c559f56eea2b2d
```

Now we can use `dmsgcurl` to download the hosted file. Open a new terminal and run the following.

```shell script
# Replace '038dde2d050803db59e2ad19e5a6db0f58f8419709fc65041c48b0cb209bb7a851' with the generated PK.
$ dmsgcurl dmsg://038dde2d050803db59e2ad19e5a6db0f58f8419709fc65041c48b0cb209bb7a851:80/hello.txt

# Check downloaded file.
$ cat hello.txt
#   Hello World!
```

Note: If you set `-d` or `--data` flag, then curl work as post method (upload), and if not then work as get method (download).
