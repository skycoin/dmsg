[![Go Report Card](https://goreportcard.com/badge/github.com/skycoin/dmsg)](https://goreportcard.com/report/github.com/skycoin/dmsg)
![Test](https://github.com/skycoin/dmsg/actions/workflows/test.yml/badge.svg)
![Deploy](https://github.com/skycoin/dmsg/actions/workflows/deploy.yml/badge.svg)
![Release](https://github.com/skycoin/dmsg/actions/workflows/release.yml/badge.svg)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/skycoin/dmsg/badge)](https://api.securityscorecards.dev/projects/github.com/skycoin/dmsg)
[![go.mod](https://img.shields.io/github/go-mod/go-version/skycoin/dmsg.svg)](https://github.com/skycoin/dmsg)
[![skywire](https://img.shields.io/aur/version/skywire?color=1793d1&label=skywire&logo=arch-linux)](https://aur.archlinux.org/packages/skywire/)
[![skywire-bin](https://img.shields.io/aur/version/skywire-bin?color=1793d1&label=skywire-bin&logo=arch-linux)](https://aur.archlinux.org/packages/skywire-bin/)

# dmsg

`dmsg` (read as *D-message*) is a distributed messaging system and encrypted transport layer used as the control plane for [Skywire](https://github.com/skycoin/skywire). It provides anonymous, public key-based routing between clients mediated by relay servers, with end-to-end encryption via the Noise Protocol (ChaCha20-Poly1305 / secp256k1).

## Architecture

The dmsg network is comprised of three types of services:

- **`dmsg.Discovery`** — acts like a DNS for the network, identifying servers and clients by their `secp256k1` public keys.
- **`dmsg.Server`** — relays encrypted streams between clients. Servers can be meshed with each other to enable cross-server client connectivity.
- **`dmsg.Client`** — connects to one or more servers to establish sessions and streams with other clients.

```
           [D]

     S(1) ←——→ S(2)
   //   \\      //   \\
  //     \\    //     \\
 C(A)    C(B) C(C)    C(D)
```

Legend:
- `[D]` — `dmsg.Discovery`
- `S(X)` — `dmsg.Server` (servers are meshed with each other)
- `C(X)` — `dmsg.Client`

Clients and servers are identified via `secp256k1` public keys and store records of themselves in the discovery. Client records include the public keys of servers they are delegated to.

## Key Concepts

- **Session** — the connection between a client and a server (noise-encrypted TCP + yamux/smux multiplexing).
- **Stream** — a connection between two clients, routed via one or more servers. Each stream has its own noise handshake for end-to-end encryption.
- **Server Mesh** — servers can peer with each other (via static config or auto-discovery) so that clients on different servers can communicate. The mesh uses a 1-hop maximum: a client's server forwards the request to the destination's server, which delivers it locally.

## Server-to-Server Mesh

By default, dmsg servers automatically discover and peer with all other servers registered in the same dmsg discovery. This means clients connected to different servers can reach each other transparently — the dial is forwarded through the server mesh.

Servers can also be configured to peer with specific servers via static config:

```json
{
  "peers": [
    {"public_key": "02abc...", "address": "1.2.3.4:8081"}
  ]
}
```

Static peers are useful for environments without discovery (e.g., direct clients) or for establishing guaranteed peering relationships.

## Dmsg Tools and Libraries

- [`dmsgcurl`](./docs/dmsgcurl.md) — simplified `curl` over `dmsg`.
- [`dmsgpty`](./docs/dmsgpty.md) — simplified `SSH` over `dmsg`.
- `dmsgweb` — HTTP and raw TCP port forwarding over `dmsg`, with a resolving SOCKS5 proxy for `.dmsg` domains.
- `dmsghttp` — HTTP file server over `dmsg`.
- `dmsg-socks5` — SOCKS5 proxy server and client over `dmsg`.

## Additional Resources

- [`dmsg` examples](./examples)
- [`dmsg.Discovery` documentation](./cmd/dmsg-discovery/README.md)
- [Starting a local `dmsg` environment](./integration/README.md)

## Dependency Graph

Made with [goda](https://github.com/loov/goda):

```
go run github.com/loov/goda@latest graph github.com/skycoin/dmsg/... | dot -Tsvg -o docs/dmsg-goda-graph.svg
```

![Dependency Graph](docs/dmsg-goda-graph.svg "github.com/skycoin/dmsg Dependency Graph")
