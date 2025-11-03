# Known Issues and Limitations# Known Issues and Limitations



This document tracks known limitations, edge cases, and areas requiring additional work in the I2P Docker network plugin.This document tracks known limitations, edge cases, and areas requiring additional work in the I2P Docker network plugin.



## Current Limitations## Current Limitations



### 1. Plugin Test Suite Requires iptables### 1. Plugin Test Suite Requires iptables



**Severity:** Environmental**Severity:** Environmental  

**Status:** Known Limitation**Status:** Known Limitation



**Description:****Description:**  

The plugin package tests (`pkg/plugin/*_test.go`) require `iptables` to be available in the test environment. Without it, tests fail during network creation with:The plugin package tests (`pkg/plugin/*_test.go`) require `iptables` to be available in the test environment. Without it, tests fail during network creation with:



```text```text

iptables not available (required for traffic filtering): iptables command not foundiptables not available (required for traffic filtering): iptables command not found

``````



**Impact:****Impact:**



- Plugin package test coverage reported as 28.1% in environments without iptables- Plugin package test coverage reported as 28.1% in environments without iptables

- Core functionality tests pass when iptables is available- Core functionality tests pass when iptables is available

- Does not affect runtime behavior (plugin requires iptables to function)- Does not affect runtime behavior (plugin requires iptables to function)



**Workaround:****Workaround:**



- Install `iptables` in test environment- Install `iptables` in test environment

- Tests can be skipped in CI environments without iptables using `go test -short`- Tests can be skipped in CI environments without iptables using `go test -short`



------



### 2. Service Address Inspection Timing### 2. Service Address Inspection Timing



**Severity:** Minor**Severity:** Minor  

**Status:** Addressed (Gap #4)**Status:** Addressed (Gap #4)



**Description:****Description:**  

Service addresses (`.b32.i2p` addresses) are only available via `docker network inspect <network> --verbose` after container join completes. The addresses are stored in the `EndpointOperInfo` response and are **not** available during the initial `docker inspect <container>` immediately after creation.Service addresses (`.b32.i2p` addresses) are only available via `docker network inspect <network> --verbose` after container join completes. The addresses are stored in the `EndpointOperInfo` response and are **not** available during the initial `docker inspect <container>` immediately after creation.



**Impact:****Impact:**

- Users must query network-level data, not container-level

- Users must query network-level data, not container-level- Documented `docker inspect test-web | grep "com.i2p.service.addresses"` command may not work as expected

- Documented `docker inspect test-web | grep "com.i2p.service.addresses"` command may not work as expected- Addresses are available via plugin API and network inspection

- Addresses are available via plugin API and network inspection

**Workaround:**

**Workaround:**```bash

# Correct way to retrieve service addresses:

```bashdocker network inspect i2p-net --verbose | jq '.Containers[].EndpointInfo.Value["com.i2p.service.addresses"]'

# Correct way to retrieve service addresses:

docker network inspect i2p-net --verbose | jq '.Containers[].EndpointInfo.Value["com.i2p.service.addresses"]'# Or parse plugin logs:

docker logs i2p-network-plugin 2>&1 | grep "Successfully exposed"

# Or parse plugin logs:```

docker logs i2p-network-plugin 2>&1 | grep "Successfully exposed"

```---



---### 3. Traffic Filter Limitations



### 3. Traffic Filter Limitations**Severity:** Informational  

**Status:** As Designed

**Severity:** Informational

**Status:** As Designed**Description:**  

The traffic filter operates at the network layer with the following constraints:

**Description:**

The traffic filter operates at the network layer with the following constraints:1. **Allowlist Mode Requires Explicit Entries:**

   - When filter mode is `allowlist`, destinations not in the list are blocked

1. **Allowlist Mode Requires Explicit Entries:**   - No automatic inclusion of "safe" or "default" I2P services

   - When filter mode is `allowlist`, destinations not in the list are blocked   - Users must manually populate allowlist with all intended destinations

   - No automatic inclusion of "safe" or "default" I2P services

   - Users must manually populate allowlist with all intended destinations2. **Wildcard Matching:**

   - Wildcards (`*`) match any substring within a label

2. **Wildcard Matching:**   - Pattern `*.example.i2p` matches `sub.example.i2p` but not `example.i2p`

   - Wildcards (`*`) match any substring within a label   - For exact+subdomain matching, add both: `example.i2p,*.example.i2p`

   - Pattern `*.example.i2p` matches `sub.example.i2p` but not `example.i2p`

   - For exact+subdomain matching, add both: `example.i2p,*.example.i2p`3. **Non-I2P Traffic:**

   - All non-I2P destinations (regular internet domains) are blocked by default

3. **Non-I2P Traffic:**   - Cannot be allowlisted (filter only allows I2P destinations)

   - All non-I2P destinations (regular internet domains) are blocked by default   - This is intentional for anonymity preservation

   - Cannot be allowlisted (filter only allows I2P destinations)

   - This is intentional for anonymity preservation**Impact:**

- Users must understand filter semantics to configure correctly

**Impact:**- Overly restrictive allowlists may break legitimate traffic



- Users must understand filter semantics to configure correctly**Documentation:**

- Overly restrictive allowlists may break legitimate trafficSee [USAGE.md](USAGE.md#traffic-filtering) for filter configuration examples.



**Documentation:**---

See [USAGE.md](USAGE.md#traffic-filtering) for filter configuration examples.

### 4. Concurrent Container Joins on Same Network

---

**Severity:** Low  

### 4. Concurrent Container Joins on Same Network**Status:** Needs Testing



**Severity:** Low**Description:**  

**Status:** Needs TestingWhile the plugin uses mutexes for network/endpoint state, extensive stress testing of concurrent container joins to the same network has not been performed.



**Description:****Potential Issues:**

While the plugin uses mutexes for network/endpoint state, extensive stress testing of concurrent container joins to the same network has not been performed.- Race conditions in I2P tunnel creation (each container gets SAM session)

- IPAM address allocation conflicts under high concurrency

**Potential Issues:**- ProxyManager state consistency during rapid endpoint creation



- Race conditions in I2P tunnel creation (each container gets SAM session)**Current Mitigations:**

- IPAM address allocation conflicts under high concurrency- Read/write locks on network state (`pkg/plugin/network.go`)

- ProxyManager state consistency during rapid endpoint creation- Mutex protection of service exposure manager (`pkg/service/manager.go`)

- Individual SAM connections per container (isolated state)

**Current Mitigations:**

**Recommended Actions:**

- Read/write locks on network state (`pkg/plugin/network.go`)- Test with Docker Compose scale operations (`docker-compose up --scale web=10`)

- Mutex protection of service exposure manager (`pkg/service/manager.go`)- Monitor for address allocation failures or tunnel creation errors

- Individual SAM connections per container (isolated state)- Report any race conditions observed in production



**Recommended Actions:**---



- Test with Docker Compose scale operations (`docker-compose up --scale web=10`)### 5. IP-Based Service Exposure Validation

- Monitor for address allocation failures or tunnel creation errors

- Report any race conditions observed in production**Severity:** Low  

**Status:** Recently Implemented (Gap #1)

---

**Description:**  

### 5. IP-Based Service Exposure ValidationIP-based service exposure (via `i2p.expose.80=ip`) was recently implemented and has limited production validation. Key considerations:



**Severity:** Low1. **Target IP Validation:**

**Status:** Recently Implemented (Gap #1)   - Plugin validates IP address format using `net.ParseIP()`

   - Does not verify that target IP is routable or listening

**Description:**   - Invalid IPs default to `127.0.0.1` with log warning

IP-based service exposure (via `i2p.expose.80=ip`) was recently implemented and has limited production validation. Key considerations:

2. **Port Conflicts:**

1. **Target IP Validation:**   - No detection of host port conflicts (OS will reject bind)

   - Plugin validates IP address format using `net.ParseIP()`   - Multiple containers can specify same IP:port (last one wins or fails)

   - Does not verify that target IP is routable or listening   - No integration with Docker's native port mapping system

   - Invalid IPs default to `127.0.0.1` with log warning

3. **Security Implications:**

2. **Port Conflicts:**   - Exposing on `0.0.0.0` makes service publicly accessible

   - No detection of host port conflicts (OS will reject bind)   - No automatic firewall rule creation beyond iptables traffic filtering

   - Multiple containers can specify same IP:port (last one wins or fails)   - Users responsible for securing exposed IP services

   - No integration with Docker's native port mapping system

**Best Practices:**

3. **Security Implications:**- Use `127.0.0.1` for local-only exposure (default)

   - Exposing on `0.0.0.0` makes service publicly accessible- Verify no port conflicts before container start

   - No automatic firewall rule creation beyond iptables traffic filtering- Apply host firewall rules independently of plugin

   - Users responsible for securing exposed IP services- Consider using I2P exposure (default) for better anonymity



**Best Practices:**---



- Use `127.0.0.1` for local-only exposure (default)## Edge Cases and Undefined Behavior

- Verify no port conflicts before container start

- Apply host firewall rules independently of plugin### 1. Network Deletion with Active Containers

- Consider using I2P exposure (default) for better anonymity

**Current Behavior:** Plugin prevents network deletion if endpoints exist.

---

**Undefined:**

## Edge Cases and Undefined Behavior- Forced deletion (`docker network rm -f`) behavior

- Cleanup of orphaned I2P tunnels if container crashes during delete

### 1. Network Deletion with Active Containers- SAM session lifecycle if Docker daemon restarts



**Current Behavior:** Plugin prevents network deletion if endpoints exist.**Recommendation:** Always stop containers before deleting networks.



**Undefined:**---



- Forced deletion (`docker network rm -f`) behavior### 2. I2P Router Unavailability

- Cleanup of orphaned I2P tunnels if container crashes during delete

- SAM session lifecycle if Docker daemon restarts**Current Behavior:** Plugin tests I2P connectivity at startup. Network creation fails if SAM bridge is unreachable.



**Recommendation:** Always stop containers before deleting networks.**Undefined:**

- Behavior if I2P router stops after network creation

---- Tunnel recovery after I2P router restart

- Long-term stability of idle SAM sessions

### 2. I2P Router Unavailability

**Recommendation:** Monitor I2P router health, restart plugin if SAM connectivity lost.

**Current Behavior:** Plugin tests I2P connectivity at startup. Network creation fails if SAM bridge is unreachable.

---

**Undefined:**

### 3. IPv6 Support

- Behavior if I2P router stops after network creation

- Tunnel recovery after I2P router restart**Current Behavior:** 

- Long-term stability of idle SAM sessions- IPv6 addresses accepted for IP-based service exposure

- No validation of IPv6 routing or connectivity

**Recommendation:** Monitor I2P router health, restart plugin if SAM connectivity lost.- Untested in production environments



---**Recommendation:** Validate IPv6 functionality before using in production.



### 3. IPv6 Support---



**Current Behavior:**## Reporting Issues



- IPv6 addresses accepted for IP-based service exposureFound a bug or limitation not listed here?

- No validation of IPv6 routing or connectivity

- Untested in production environments1. **Check Existing Issues:** [GitHub Issues](https://github.com/go-i2p/go-docker-network-i2p/issues)

2. **Review Documentation:** [USAGE.md](USAGE.md), [TROUBLESHOOTING.md](TROUBLESHOOTING.md)

**Recommendation:** Validate IPv6 functionality before using in production.3. **Enable Debug Logging:** Set `DEBUG=1` environment variable

4. **Open New Issue:** Include:

---   - Plugin version (`i2p-network-plugin --version`)

   - Docker version (`docker version`)

## Reporting Issues   - I2P router version and configuration

   - Steps to reproduce with minimal example

Found a bug or limitation not listed here?   - Relevant logs from plugin and containers



1. **Check Existing Issues:** [GitHub Issues](https://github.com/go-i2p/go-docker-network-i2p/issues)---

2. **Review Documentation:** [USAGE.md](USAGE.md), [TROUBLESHOOTING.md](TROUBLESHOOTING.md)

3. **Enable Debug Logging:** Set `DEBUG=1` environment variable## Changelog of Addressed Issues

4. **Open New Issue:** Include:

   - Plugin version (`i2p-network-plugin --version`)This section tracks previously documented gaps that have been resolved:

   - Docker version (`docker version`)

   - I2P router version and configuration### ✅ Fixed: IP-Based Port Exposure (Gap #1, v0.x.x)

   - Steps to reproduce with minimal example- **Issue:** `createIPServiceExposure()` returned "not yet implemented" error

   - Relevant logs from plugin and containers- **Resolution:** Full implementation with IP validation, defaults, IPv6 support

- **Files Changed:** `pkg/service/manager.go`, `pkg/service/manager_test.go`

---

### ✅ Fixed: Traffic Filter Mode Configuration (Gap #2, v0.x.x)

## Changelog of Addressed Issues- **Issue:** Filter mode hardcoded to default, network options ignored

- **Resolution:** Added `parseFilterConfig()` to parse `i2p.filter.mode` option

This section tracks previously documented gaps that have been resolved:- **Files Changed:** `pkg/plugin/network.go`, `pkg/proxy/filter.go`, `pkg/proxy/manager.go`



### ✅ Fixed: IP-Based Port Exposure (Gap #1, v0.x.x)### ✅ Fixed: Allowlist/Blocklist Population (Gap #3, v0.x.x)

- **Issue:** No mechanism to populate filter lists from network options

- **Issue:** `createIPServiceExposure()` returned "not yet implemented" error- **Resolution:** Added `parseFilterDestinations()` to parse comma-separated lists

- **Resolution:** Full implementation with IP validation, defaults, IPv6 support- **Files Changed:** `pkg/plugin/network.go`

- **Files Changed:** `pkg/service/manager.go`, `pkg/service/manager_test.go`

### ✅ Fixed: Service Address Inspection (Gap #4, v0.x.x)

### ✅ Fixed: Traffic Filter Mode Configuration (Gap #2, v0.x.x)- **Issue:** Service addresses only in ephemeral Join response

- **Resolution:** Return addresses via `EndpointOperInfo` for persistent storage

- **Issue:** Filter mode hardcoded to default, network options ignored- **Files Changed:** `pkg/plugin/handlers.go`

- **Resolution:** Added `parseFilterConfig()` to parse `i2p.filter.mode` option

- **Files Changed:** `pkg/plugin/network.go`, `pkg/proxy/filter.go`, `pkg/proxy/manager.go`### ✅ Fixed: Test Coverage Documentation (Gap #5, v0.x.x)

- **Issue:** README claimed ~64% coverage without verification

### ✅ Fixed: Allowlist/Blocklist Population (Gap #3, v0.x.x)- **Resolution:** Updated README with actual coverage: ~67% average

- **Files Changed:** `README.md`

- **Issue:** No mechanism to populate filter lists from network options

- **Resolution:** Added `parseFilterDestinations()` to parse comma-separated lists---

- **Files Changed:** `pkg/plugin/network.go`

## Version Information

### ✅ Fixed: Service Address Inspection (Gap #4, v0.x.x)

- **Last Updated:** 2025-11-02

- **Issue:** Service addresses only in ephemeral Join response- **Plugin Version:** Development (post-audit)

- **Resolution:** Return addresses via `EndpointOperInfo` for persistent storage- **Audit Reference:** [AUDIT.md](AUDIT.md)

- **Files Changed:** `pkg/plugin/handlers.go`

### ✅ Fixed: Test Coverage Documentation (Gap #5, v0.x.x)

- **Issue:** README claimed ~64% coverage without verification
- **Resolution:** Updated README with actual coverage: ~67% average
- **Files Changed:** `README.md`

---

## Version Information

- **Last Updated:** 2025-11-02
- **Plugin Version:** Development (post-audit)
- **Audit Reference:** [AUDIT.md](AUDIT.md)
