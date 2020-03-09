# Integration Environment

## Start a local dmsg environment

1. Build `dmsg-discovery` and `dmsg-server` binaries.
    ```bash
    $ make build
    ```
2. Ensure `redis` is running and listening on port 6379.
    ```bash
    $ redis-server 
    ```
3. Start `dmsg-discovery`.
    ```bash
    $ ./bin/dmsg-discovery 
    ```
4. Start `dmsg-server`.
    ```bash
    $ ./bin/dmsg-server ./integration/configs/dmsgserver1.json
    ```
