# DMSG E2E Testing

This document describes the e2e testing framework for DMSG client utilities.

## Overview

The e2e tests validate DMSG client utilities (`dmsg curl` and `dmsg web`) against a local DMSG deployment. This testing regime is similar to the one implemented for Skywire visor apps.

## What's Tested

### DMSG Client Utilities
- **dmsg curl**: Downloads content over DMSG protocol
- **dmsg web**: Runs SOCKS5 proxy and web interface for DMSG access
- **dmsg web srv**: Serves HTTP or TCP from local port over DMSG

### Critical Regression Tests
The e2e tests include specific regression tests that would have caught recent bugs:

1. **Version Field Test** (`TestVersionFieldPresent`)
   - Validates that all Entry structs include the required `version` field
   - Tests `dmsg curl -Z` (HTTP discovery mode) functionality
   - Would have caught the bug fixed in commit `caca20b5`
   - Before the fix: Failed with "entry validation error: entry has no version"
   - After the fix: Passes successfully

## Architecture

### Docker-Based Deployment
Similar to Skywire's e2e tests, the DMSG e2e framework uses Docker Compose to create an isolated test environment:

```
┌─────────────────────────────────────────────────┐
│  Docker Network: dmsg-e2e (172.20.0.0/16)       │
│                                                  │
│  ┌──────────┐  ┌─────────────┐  ┌────────────┐ │
│  │  Redis   │  │ DMSG        │  │ DMSG       │ │
│  │          │──│ Discovery   │──│ Server     │ │
│  └──────────┘  └─────────────┘  └────────────┘ │
│       │              │                │         │
│       │              └────────────────┼───┐     │
│       │                               │   │     │
│  ┌────────────────────────────────────┘   │     │
│  │  DMSG Client (Test Container)          │     │
│  │  - dmsg curl                            │     │
│  │  - dmsg web                             │     │
│  │  - dmsg web srv                         │     │
│  │  - HTTP test server                     │     │
│  └─────────────────────────────────────────┘     │
└─────────────────────────────────────────────────┘
```

### Services

1. **redis** (172.20.0.2)
   - Backend for DMSG discovery
   - Port 6379

2. **dmsg-discovery** (172.20.0.3)
   - DMSG discovery service in test mode (`-t`)
   - HTTP API on port 9090
   - Uses fixed secret key for reproducibility

3. **dmsg-server** (172.20.0.4)
   - DMSG server for routing traffic
   - Listens on port 8080
   - Uses fixed public key for testing

4. **dmsg-client** (172.20.0.5)
   - Test client with DMSG utilities installed
   - Runs test HTTP server
   - Executes dmsg curl and dmsg web commands

## Running Tests

### Prerequisites
- Docker and Docker Compose
- Go 1.25 or later
- Make (optional)

### Quick Start

```bash
# From dmsg root directory
make test-e2e
```

Or manually:

```bash
./scripts/run-e2e-tests.sh
```

### Manual Execution

```bash
# Build and start services
cd docker
docker-compose -f docker-compose.e2e.yml up -d

# Wait for services to initialize
sleep 15

# Run tests
go test -v -tags !no_ci ./internal/e2e/...

# View logs
docker-compose -f docker-compose.e2e.yml logs

# Cleanup
docker-compose -f docker-compose.e2e.yml down -v
```

## Test Cases

### TestDiscoveryIsRunning
**Purpose**: Verify DMSG discovery service is running
**Checks**: Container state

### TestDmsgServerIsRunning
**Purpose**: Verify DMSG server is running
**Checks**: Container state

### TestDmsgCurlBasic
**Purpose**: Test basic dmsg curl functionality
**Steps**:
1. Start HTTP test server on port 8086
2. Start `dmsg web srv` to proxy HTTP over DMSG
3. Use `dmsg curl` to fetch content
4. Validate response

**What it tests**:
- DMSG client can establish connections
- HTTP can be served over DMSG
- dmsg curl can retrieve content

### TestDmsgWebProxy
**Purpose**: Test dmsg web SOCKS5 proxy
**Steps**:
1. Start `dmsg web` with proxy and web interface
2. Verify services are listening on expected ports

**What it tests**:
- DMSG web proxy starts successfully
- Ports are correctly bound

### TestVersionFieldPresent (CRITICAL)
**Purpose**: Regression test for version field bug
**Background**: Commit `caca20b5` fixed a bug where Entry structs were missing the required `version` field, causing "entry validation error: entry has no version" when using HTTP discovery.

**Steps**:
1. Execute `dmsg curl -Z` (HTTP discovery mode)
2. Verify it succeeds without validation errors

**What it catches**:
- Missing version field in Entry structs in:
  - `pkg/direct/entries.go` (`GetClientEntry`)
  - `internal/cli/cli.go` (synthetic entries)
- Any regression of this bug

**Why it's important**: This test would have caught the bug before it reached production.

### TestDmsgCurlToDiscovery
**Purpose**: Test querying discovery service
**Steps**:
1. Use `dmsg curl` to fetch available servers from discovery API
2. Verify test server is listed

**What it tests**:
- Discovery HTTP API is accessible
- Server registration is working
- dmsg curl can parse responses

## Configuration

### Fixed Keys for Testing
The e2e environment uses fixed keys for reproducibility:

- **Discovery SK**: `b3f6706cb72215d3873ef92cc0c6037a47fe651112b1685017d6347eed0fb714`
- **Server PK**: `03b88c1335c28264c5e40ffad67eee75c2f2c39bda27015d6e14a0e90eaa78a41c`
- **Test Client SK**: `a3e4a0c8f4e2f9a7b1d5c3e8f9a2b1c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0`

Configuration files are in `docker/e2e/`:
- `dmsg-server.json`: Server configuration

## Comparison with Skywire E2E Tests

### Similarities
- Docker-based isolated environment
- Tests client applications using the protocol
- Local deployment (not production services)
- CI-ready with `!no_ci` build tag

### Differences
- **Skywire**: Tests visor apps over Skywire transports
- **DMSG**: Tests client utilities (curl, web) over DMSG protocol
- **DMSG**: Simpler architecture (no need for multiple visors)
- **DMSG**: Focus on HTTP over DMSG use cases

## Troubleshooting

### Services Not Starting
```bash
# Check container status
docker-compose -f docker/docker-compose.e2e.yml ps

# View logs
docker-compose -f docker/docker-compose.e2e.yml logs [service-name]
```

### Port Conflicts
The e2e environment uses these ports:
- 6380: Redis
- 9090: DMSG Discovery
- 8080: DMSG Server

If you see bind errors, check if these ports are in use:
```bash
netstat -tuln | grep -E ':(6380|9090|8080)'
```

### Tests Timing Out
Increase wait time in `scripts/run-e2e-tests.sh`:
```bash
sleep 30  # Instead of 15
```

Or in individual tests by adjusting the `TestMain` sleep duration.

### Docker Build Failures
Ensure you're building from the dmsg root directory:
```bash
cd docker
docker-compose -f docker-compose.e2e.yml build --no-cache
```

## Adding New Tests

1. Add test function to `internal/e2e/e2e_test.go`
2. Use `TestEnv` helper methods
3. Focus on dmsg client utility functionality
4. Include clear assertions
5. Document what the test validates
6. Run locally before committing:
   ```bash
   make test-e2e
   ```

Example test structure:
```go
func TestNewFeature(t *testing.T) {
    env := NewEnv()
    
    // Setup
    // ...
    
    // Execute dmsg command
    output, err := env.ExecInContainer(containerClient, []string{
        "dmsg", "curl", "-Z", "-U", discoveryURL, "...",
    })
    
    // Validate
    require.NoError(t, err)
    require.Contains(t, output, "expected content")
}
```

## CI Integration

### GitHub Actions Example
```yaml
name: E2E Tests

on: [push, pull_request]

jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.25'
      
      - name: Run E2E Tests
        run: |
          cd dmsg
          make test-e2e
```

## Future Enhancements

Potential improvements for the e2e test framework:

1. **More dmsg utilities**
   - Add tests for `dmsg socks`, `dmsg http`
   - Test dmsgpty functionality

2. **Multi-client scenarios**
   - Multiple clients communicating via DMSG
   - Load testing scenarios

3. **Error scenarios**
   - Server unavailable
   - Discovery down
   - Network failures

4. **Performance tests**
   - Measure throughput
   - Connection establishment time
   - Resource usage

5. **Integration with Skywire tests**
   - Combined test suite
   - Shared infrastructure

## References

- [DMSG Integration README](../../integration/README.md) - Local tmux-based testing
- [Skywire E2E Tests](../../../skywire/internal/integration/) - Reference implementation
- Recent bug fix: [Add Version field to Entry structs](https://github.com/skycoin/dmsg/commit/caca20b5)
