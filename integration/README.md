# Integration Environment

## Start a local dmsg environment

1. Build `dmsg-discovery` and `dmsg-server` binaries.
    ```bash
    $ make build
    ```
2. Ensure `redis-server` is installed on the system. If not alredy installed install it. (e.g. for Linux)
    ```bash
    $ sudo apt install redis-server 
    ```
3. Ensure `redis` is running and listening on port 6379.
    ```bash
    $ redis-server
    ```
4. Start `dmsg-discovery` in testing mode.
    ```bash
    $ ./bin/dmsg-discovery -t
    ```
5. Start `dmsg-server`.
    ```bash
    $ ./bin/dmsg-server start ./integration/configs/dmsgserver1.json
    ```

## Put dmsg-server under load
You need [tmux](https://github.com/tmux/tmux) to continue this test
1. Start a local dmsg environment
2. Clone skywire, checkout to develop
    ```
    git clone https://github.com/skycoin/skywire.git underLoadDmsgServer
    cd underLoadDmsgServer
    git checkout develop
    ```
3. Add `time.Sleep(15 * time.Minute)` at the beginning of `initLauncher` at **pkg > visor > init.go**.
    ```
    ...
    func initLauncher(ctx context.Context, v *Visor, log *logging.Logger) error {
	  time.Sleep(15 * time.Minute)
	  conf := v.conf.Launcher
    ...
    ```
4. Build new binaries: `make build`
5. Copy `underlocal.sh` file to skywire clone directory
6. Run it by `bash underlocal.sh -n 200 -u localhost:9090`

For close all visors and delete generated configs, use these two commands:
```
pkill -9 -f 'skywire-visor -c ./config'
rm config*
```
