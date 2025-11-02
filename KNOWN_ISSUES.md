# Known Issues

This document tracks known limitations and issues in the go-docker-network-i2p implementation.

## I2P SAM Library Limitations

### Multiple Server Tunnels per Container (RESOLVED)

**Issue:** ~~The current implementation cannot create multiple server tunnels for the same container due to a limitation in the `go-sam-go` library.~~ **RESOLVED**

**Resolution:** Updated to use local dev build of `go-sam-go` with `NewStreamSubSessionWithPort()` function that supports port specification.

**Previous Root Cause:** The original `go-sam-go` library's primary session implementation only supported one active stream sub-session at a time. When attempting to create a second server tunnel (e.g., both HTTP on port 80 and HTTPS on port 443), the SAM bridge returned an error:

```text
SESSION STATUS RESULT=I2P_ERROR ID="tunnel-name-server-port443" MESSAGE="Duplicate protocol 6 and port 0"
```

**Solution Implemented:**

- Using `NewStreamSubSessionWithPort(id, options, fromPort, toPort)` instead of `NewStreamSubSession(id, options)`
- Each server tunnel now specifies its actual port numbers to the SAM bridge
- Multiple server tunnels per container now supported

**Technical Details:**

- ✅ **FIXED** - Now using port-specific sub-session creation
- Local dev build of `go-sam-go` includes the required `NewStreamSubSessionWithPort` function
- Each tunnel gets unique port identification at the SAM protocol level

**Current Status:**

- ✅ **RESOLVED** - Multiple server tunnels per container now supported
- Tests validate both HTTP (port 80) and HTTPS (port 443) tunnel creation
- Service exposure manager can expose multiple services per container

**Files Modified:**

- `pkg/i2p/tunnel.go` - Updated to use `NewStreamSubSessionWithPort()`
- `pkg/service/manager.go` - Updated comments to reflect resolution  
- `pkg/service/manager_test.go` - Restored multi-tunnel testing

**Impact:**

- ✅ **RESOLVED** - No longer limits container service exposure capabilities
- Multiple services per container now fully supported
- Core network functionality enhanced

**Test Evidence:**

```bash
# Test command:
go test -timeout 300s -v ./pkg/service -run TestExposeServices

# Expected behavior: 2 exposures created ✅
# Actual behavior: 2 exposures created successfully ✅
```

**Related Development:**

- Using local dev build of `go-sam-go` with enhanced port support
- Consider contributing `NewStreamSubSessionWithPort` back to upstream `go-sam-go`
- Monitor upstream library development for official release

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
