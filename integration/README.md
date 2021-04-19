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
    $ ./bin/dmsg-server ./integration/configs/dmsgserver1.json
    ```
