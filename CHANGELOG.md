# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

updates may be generated with scripts/changelog.sh <PR#lowest> <PR#highest>

## 1.3.14

### Added
- add `dmsgweb` as new tools to release

### Changed
- change `dmsgget` to `dmsgcurl` with new functionalities

### Commits
-   update skywire-utilities  [#244](https://github.com/skycoin/dmsg/pull/244)
-   add ConnectedServersPK method  [#243](https://github.com/skycoin/dmsg/pull/243)
-   improve logic on save file dmsgcurl  [#242](https://github.com/skycoin/dmsg/pull/242)
-   dmsgcurl  [#238](https://github.com/skycoin/dmsg/pull/238)
-   dmsg client using socks5 proxy basic example  [#237](https://github.com/skycoin/dmsg/pull/237)
-   Bump Go images for Docker to 1.20-alpine  [#235](https://github.com/skycoin/dmsg/pull/235)
-   Export RootCmds  [#234](https://github.com/skycoin/dmsg/pull/234)
-   Dmsgweb  [#229](https://github.com/skycoin/dmsg/pull/229)


## 1.3.0

### Added
- add `start` command to `dmsg-server`, should used like `./dmsg-server start config.json`
- add `gen` command to generate config, with two flag `-o` for output file and `-t` for using test env values

### Changed
- switch from AppVeyor to Github Action in CI process
