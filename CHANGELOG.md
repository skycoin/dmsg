# Changelog
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/en/1.0.0/)
and this project adheres to [Semantic Versioning](http://semver.org/spec/v2.0.0.html).

## 1.3.14
- add `dmsgweb` as new tools to release
- change `dmsgget` to `dmsgcurl` with new functionalities

## 1.3.0

### Added
- add `start` command to `dmsg-server`, should used like `./dmsg-server start config.json`
- add `gen` command to generate config, with two flag `-o` for output file and `-t` for using test env values

### Changed
- switch from AppVeyor to Github Action in CI process
