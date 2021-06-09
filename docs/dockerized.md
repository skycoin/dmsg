## Dockerized dmsg-server and dmsg-discovery

### Requirements

- Docker / Docker-CE
- bash or compatible shell
- redis (dockerized or not)

### How to

1. Clone this repository
2. Run this command to build `dmsg-server` and `dmsg-discovery` images
```bash
$ ./docker/scripts/docker-push.sh -t develop -b
```
3. Create a new docker network
```bash
$ docker network create -d bridge br-dmsg0
```
4. Run redis
```bash
$ docker run --network="br-dmsg0" --rm --name=redis -d -p 6379:6379 redis:alpine
```
5. Run `dmsg-discovery` and `dmsg-server`
```bash
$ docker run --rm --network="br-dmsg0" --name=dmsg-discovery skycoinpro/dmsg-discovery:test --redis redis://redis:6379
# Run dmsg-server with default config (default points to production server)
$ docker run --network="br-dmsg0" --rm --name=dmsg-server skycoinpro/dmsg-server:test
# or run it with your own config
$ docker run -v <YOUR_CONFIG_PATH>:/etc/dmsg --network="br-dmsg0" --rm --name=dmsg-server \
	skycoinpro/dmsg-server:test <YOUR_CONFIG_PATH>/<YOUR_CONFIG_FILE_NAME>
```
