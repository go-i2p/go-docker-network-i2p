# Known Issues

This document tracks known limitations and issues in the go-docker-network-i2p implementation.

## I2P SAM Library Limitations

### Multiple Server Tunnels per Container (go-sam-go limitation)

**Issue:** The current implementation cannot create multiple server tunnels for the same container due to a limitation in the `go-sam-go` library.

**Root Cause:** The `go-sam-go` library's primary session implementation only supports one active stream sub-session at a time. When attempting to create a second server tunnel (e.g., both HTTP on port 80 and HTTPS on port 443), the SAM bridge returns an error:

```text
SESSION STATUS RESULT=I2P_ERROR ID="tunnel-name-server-port443" MESSAGE="Duplicate protocol 6 and port 0"
```

**Technical Details:**

- This is NOT a limitation of the I2P SAM protocol itself
- This is specifically a limitation of the `go-sam-go` library implementation  
- The SAM protocol supports multiple sub-sessions per primary session
- The library constrains primary sessions to one stream sub-session listener

**Current Workaround:**

- Tests are limited to one server tunnel per container
- Service exposure manager continues on failure (logs warning and exposes available services)

**Potential Solutions:**

1. **Create separate primary sessions per server tunnel** (Recommended)
   - Modify `TunnelManager.GetOrCreateContainerSession()` to create unique sessions per tunnel
   - Track sessions by `containerID + tunnelName` instead of just `containerID`
   - Each server tunnel gets its own I2P destination and keys

2. **Switch to different I2P SAM library**
   - Evaluate alternative Go SAM libraries that don't have this limitation
   - Consider direct SAM protocol implementation

3. **Use different session types**
   - Investigate if non-stream session types support multiple concurrent instances
   - May require protocol-level changes

**Files Affected:**

- `pkg/i2p/tunnel.go` - Tunnel creation and session management
- `pkg/service/manager.go` - Service exposure logic  
- `pkg/service/manager_test.go` - Test limitations

**Impact:**

- **Medium Priority** - Limits container service exposure capabilities
- Workaround exists (single service exposure per container)
- Does not affect core network functionality

**Test Evidence:**

```bash
# Reproduction command:
go test -timeout 300s -v ./pkg/service -run TestExposeServices

# Expected behavior: 2 exposures created
# Actual behavior: 1 exposure created, 1 failed with duplicate protocol error
```

**Related GitHub Issues:**

- Consider filing issue with `go-sam-go` project regarding sub-session limitations
- Track upstream library development for resolution

---

## Network Interface Limitations

### iptables Dependency in Test Environment

**Issue:** Tests show warnings about missing `iptables` binary in development/CI environments.

**Root Cause:** The proxy manager attempts to configure iptables rules for traffic redirection but fails gracefully when iptables is not available.

**Current Behavior:**

- Logs warnings about iptables rule failures
- Does not break test execution
- Network functionality degrades gracefully

**Workaround:**

- Install iptables in test/development environments: `sudo apt-get install iptables`
- Tests continue to pass despite warnings

**Impact:**

- **Low Priority** - Only affects test environments
- Production Docker environments typically have iptables available
- Graceful degradation implemented

---

## Contributing

When encountering new issues:

1. **Document the issue** in this file with:
   - Clear description and root cause analysis
   - Reproduction steps and test evidence  
   - Impact assessment and workaround options
   - Potential solution approaches

2. **Update relevant tests** to reflect current limitations

3. **Add TODO comments** in code referencing this documentation

4. **Consider upstream fixes** where appropriate (library issues, dependencies)
