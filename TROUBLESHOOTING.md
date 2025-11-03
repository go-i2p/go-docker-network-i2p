# I2P Docker Network Plugin Troubleshooting Guide

This guide helps you diagnose and resolve common issues with the I2P Docker Network Plugin.

## Table of Contents

- [Quick Diagnostics](#quick-diagnostics)
- [Common Issues](#common-issues)
- [Diagnostic Commands](#diagnostic-commands)
- [Debugging Techniques](#debugging-techniques)
- [Performance Issues](#performance-issues)
- [Security Issues](#security-issues)
- [Getting Help](#getting-help)

## Quick Diagnostics

Run these commands to quickly identify common issues:

```bash
# 1. Check if I2P router is running and SAM bridge is enabled
telnet localhost 7656

# 2. Check if plugin is responding
docker network ls | grep i2p

# 3. Check plugin logs
sudo journalctl -u i2p-network-plugin -n 50

# 4. Test plugin connectivity
docker network create --driver=i2p test-network
docker network rm test-network
```

## Common Issues

### Issue 1: Plugin Not Found

**Symptoms:**
```bash
$ docker network create --driver=i2p my-network
Error response from daemon: network plugin "i2p" not found
```

**Diagnosis:**
- Plugin is not installed or not running
- Plugin socket is not accessible to Docker daemon

**Solutions:**

1. **Check if plugin is running:**
   ```bash
   # Check plugin process
   ps aux | grep i2p-network-plugin
   
   # Check plugin socket
   ls -la /run/docker/plugins/i2p*.sock
   ```

2. **Start the plugin manually:**
   ```bash
   # Start with debug logging
   sudo ./bin/i2p-network-plugin -sock /run/docker/plugins/i2p-network.sock -debug
   ```

3. **Install as systemd service:**
   ```bash
   # Create systemd unit file
   sudo tee /etc/systemd/system/i2p-network-plugin.service > /dev/null <<EOF
   [Unit]
   Description=I2P Docker Network Plugin
   After=docker.service
   Requires=docker.service
   
   [Service]
   Type=simple
   ExecStart=/usr/local/bin/i2p-network-plugin -sock /run/docker/plugins/i2p-network.sock
   Restart=always
   User=root
   Group=root
   
   [Install]
   WantedBy=multi-user.target
   EOF
   
   # Enable and start service
   sudo systemctl daemon-reload
   sudo systemctl enable i2p-network-plugin
   sudo systemctl start i2p-network-plugin
   ```

### Issue 2: I2P Router Connection Failed

**Symptoms:**
```bash
$ docker network create --driver=i2p my-network
Error response from daemon: failed to create network: I2P connectivity check failed
```

**Diagnosis:**
- I2P router is not running
- SAM bridge is not enabled
- SAM bridge is on different host/port
- Firewall blocking connections

**Solutions:**

1. **Check I2P router status:**
   ```bash
   # Check if I2P router is running
   ps aux | grep i2p
   
   # Check I2P web console
   curl -s http://127.0.0.1:7657/ | grep -i "i2p router"
   ```

2. **Enable SAM bridge:**
   ```bash
   # Access I2P router console
   firefox http://127.0.0.1:7657/
   
   # Navigate to: Configure → Clients → SAM application bridge
   # Enable the SAM bridge and restart I2P router
   ```

3. **Test SAM connectivity:**
   ```bash
   # Test basic connectivity
   telnet localhost 7656
   
   # Test with timeout
   timeout 10 telnet localhost 7656
   ```

4. **Configure alternative SAM host:**
   ```bash
   # Set environment variable
   export I2P_SAM_HOST="192.168.1.100"
   export I2P_SAM_PORT="7656"
   
   # Or use network options
   docker network create --driver=i2p \
     --opt i2p.sam.host=192.168.1.100 \
     --opt i2p.sam.port=7656 \
     my-network
   ```

### Issue 3: Container Cannot Reach I2P Services

**Symptoms:**
- Container starts successfully but cannot connect to .i2p domains
- DNS resolution fails for I2P addresses
- Connection timeouts to I2P services

**Diagnosis:**
- Traffic filtering is blocking connections
- DNS proxy not working
- SOCKS proxy not configured properly

**Solutions:**

1. **Check traffic filtering:**
   ```bash
   # View recent traffic logs
   sudo journalctl -u i2p-network-plugin | grep "TRAFFIC" | tail -20
   
   # Look for BLOCK messages
   sudo journalctl -u i2p-network-plugin | grep "TRAFFIC BLOCK"
   ```

2. **Test DNS resolution:**
   ```bash
   # Inside container
   docker run --rm --network my-i2p-network alpine \
     nslookup example.i2p
   
   # Check if DNS proxy is running
   docker run --rm --network my-i2p-network alpine \
     netstat -tlnp | grep :53
   ```

3. **Test SOCKS proxy:**
   ```bash
   # Check SOCKS proxy is listening
   docker run --rm --network my-i2p-network alpine \
     netstat -tlnp | grep :1080
   
   # Test SOCKS connectivity
   docker run --rm --network my-i2p-network alpine \
     nc -zv 127.0.0.1 1080
   ```

4. **Disable traffic filtering temporarily:**
   ```bash
   # Create network without filtering
   docker network create --driver=i2p \
     --opt i2p.filter.mode=disabled \
     debug-network
   ```

### Issue 4: iptables Errors

**Symptoms:**
```bash
Warning: Failed to stop proxy manager: iptables command failed: 
exec: "iptables": executable file not found in $PATH
```

**Diagnosis:**
- iptables not installed
- Plugin running without sufficient privileges
- iptables rules conflicts

**Solutions:**

1. **Install iptables:**
   ```bash
   # Ubuntu/Debian
   sudo apt-get update
   sudo apt-get install iptables
   
   # RHEL/CentOS/Fedora
   sudo yum install iptables
   # or
   sudo dnf install iptables
   ```

2. **Run plugin with sufficient privileges:**
   ```bash
   # Run as root
   sudo ./bin/i2p-network-plugin
   
   # Add user to docker group
   sudo usermod -aG docker $USER
   ```

3. **Check iptables capabilities:**
   ```bash
   # Test iptables functionality
   sudo iptables -L -n
   
   # Check for existing rules
   sudo iptables -t nat -L -n
   sudo iptables -t filter -L -n
   ```

### Issue 5: Service Exposure Not Working

**Symptoms:**
- Container runs but no .b32.i2p address is generated
- Services not accessible from outside I2P network
- Missing service exposure logs

**Diagnosis:**
- Ports not properly exposed in container
- Service detection not working
- SAM session creation failed

**Solutions:**

1. **Check port exposure:**
   ```bash
   # Inspect container configuration
   docker inspect container-name | jq '.Config.ExposedPorts'
   
   # Check port mappings
   docker inspect container-name | jq '.NetworkSettings.Ports'
   ```

2. **Manual port exposure:**
   ```bash
   # Use explicit port exposure
   docker run -d --name web-service \
     --network i2p-network \
     --expose 80 \
     --expose 443 \
     nginx:alpine
   
   # Or use environment variables
   docker run -d --name api-service \
     --network i2p-network \
     -e PORT=8080 \
     -e API_PORT=8081 \
     my-api:latest
   ```

3. **Check service exposure logs:**
   ```bash
   # Look for service exposure messages
   sudo journalctl -u i2p-network-plugin | grep "Service exposed"
   
   # Check for tunnel creation
   sudo journalctl -u i2p-network-plugin | grep "Creating server tunnel"
   ```

4. **Test tunnel functionality:**
   ```bash
   # Check if tunnels are created
   docker exec container-name netstat -tlnp | grep 127.0.0.1
   
   # Look for server tunnel processes
   docker exec container-name ps aux | grep sam
   ```

## Diagnostic Commands

### Plugin Status

```bash
# Check plugin service status
sudo systemctl status i2p-network-plugin

# View plugin logs with timestamps
sudo journalctl -u i2p-network-plugin -f --since "1 hour ago"

# Check plugin socket permissions
ls -la /run/docker/plugins/i2p*.sock

# Test plugin binary
./bin/i2p-network-plugin -version
./bin/i2p-network-plugin -help
```

### Network Analysis

```bash
# List all Docker networks
docker network ls

# Inspect I2P network configuration
docker network inspect network-name

# Check network connectivity
docker run --rm --network network-name alpine \
  ping -c 3 gateway-ip

# View network interfaces
docker run --rm --network network-name alpine \
  ip addr show
```

### Container Diagnostics

```bash
# Check container network configuration
docker inspect container-name | jq '.NetworkSettings'

# Test container connectivity
docker exec container-name ping -c 3 127.0.0.1

# Check container processes
docker exec container-name ps aux

# View container logs
docker logs container-name --since 1h
```

### I2P Router Diagnostics

```bash
# Test SAM bridge connectivity
echo "HELLO VERSION MIN=3.0 MAX=3.3" | nc localhost 7656

# Check I2P router logs
tail -f ~/.i2p/logs/log-*.txt

# Monitor I2P tunnels
curl -s http://127.0.0.1:7657/tunnels | grep -E "(Tunnel|Status)"

# Check I2P router status
curl -s http://127.0.0.1:7657/index.jsp | grep -E "(Status|Uptime)"
```

## Debugging Techniques

### Enable Debug Logging

1. **Plugin Debug Mode:**
   ```bash
   # Start plugin with debug logging
   ./bin/i2p-network-plugin -debug
   
   # Or use environment variable
   export DEBUG=true
   ./bin/i2p-network-plugin
   ```

2. **Increase Log Verbosity:**
   ```bash
   # Monitor all plugin activity
   sudo journalctl -u i2p-network-plugin -f -o cat
   
   # Filter specific events
   sudo journalctl -u i2p-network-plugin | grep -E "(ERROR|WARN|FAIL)"
   ```

### Network Traffic Analysis

1. **Monitor Traffic Filtering:**
   ```bash
   # Watch traffic decisions in real-time
   sudo journalctl -u i2p-network-plugin -f | grep "TRAFFIC"
   
   # Analyze blocked connections
   sudo journalctl -u i2p-network-plugin | grep "TRAFFIC BLOCK" | \
     awk '{print $NF}' | sort | uniq -c | sort -nr
   ```

2. **Test Network Connectivity:**
   ```bash
   # Create test container
   docker run -it --rm --network i2p-network alpine sh
   
   # Inside container, test connectivity
   ping 127.0.0.1
   nslookup example.i2p
   nc -zv 127.0.0.1 1080  # SOCKS proxy
   nc -zv 127.0.0.1 53    # DNS proxy
   ```

### SAM Protocol Debugging

1. **Manual SAM Commands:**
   ```bash
   # Connect to SAM bridge
   telnet localhost 7656
   
   # Send SAM commands manually
   HELLO VERSION MIN=3.0 MAX=3.3
   SESSION CREATE STYLE=STREAM ID=test-session DESTINATION=TRANSIENT
   STREAM CONNECT ID=test-connect DESTINATION=example.b32.i2p PORT=80
   ```

2. **Monitor SAM Sessions:**
   ```bash
   # Check active sessions
   sudo journalctl -u i2p-network-plugin | grep "Successfully created primary session"
   
   # Look for session errors
   sudo journalctl -u i2p-network-plugin | grep -E "(session.*error|SESSION.*RESULT)"
   ```

## Performance Issues

### Slow Connection Establishment

**Symptoms:**
- Long delays when creating networks
- Containers take time to become network-ready
- Initial I2P connections are slow

**Solutions:**

1. **Optimize tunnel parameters:**
   ```bash
   # Reduce tunnel count for faster startup
   export I2P_INBOUND_TUNNELS=1
   export I2P_OUTBOUND_TUNNELS=1
   export I2P_INBOUND_LENGTH=2
   export I2P_OUTBOUND_LENGTH=2
   ```

2. **Use tunnel prewarming:**
   ```bash
   # Create persistent tunnel container
   docker run -d --name tunnel-warmer \
     --network i2p-network \
     --restart always \
     alpine sleep infinity
   ```

3. **Optimize I2P router:**
   ```bash
   # Increase I2P router bandwidth
   # Access http://127.0.0.1:7657/config
   # Set bandwidth limit higher
   # Enable "Participate in tunnels"
   ```

### High Memory Usage

**Symptoms:**
- Plugin consuming excessive memory
- Container OOM kills
- System becomes unresponsive

**Solutions:**

1. **Monitor memory usage:**
   ```bash
   # Check plugin memory usage
   ps aux | grep i2p-network-plugin
   
   # Monitor Docker memory usage
   docker stats --no-stream
   
   # Check system memory
   free -h
   ```

2. **Optimize configuration:**
   ```bash
   # Reduce tunnel counts
   export I2P_INBOUND_TUNNELS=2
   export I2P_OUTBOUND_TUNNELS=2
   
   # Enable idle cleanup
   export I2P_CLOSE_IDLE=true
   export I2P_CLOSE_IDLE_TIME=5
   ```

3. **Limit container resources:**
   ```bash
   # Run containers with memory limits
   docker run -d --name limited-container \
     --network i2p-network \
     --memory=512m \
     --memory-swap=1g \
     my-app:latest
   ```

## Security Issues

### Traffic Leakage

**Symptoms:**
- Non-I2P traffic is not blocked
- DNS queries bypass I2P
- Direct IP connections succeed

**Solutions:**

1. **Enable strict filtering:**
   ```bash
   # Create network with allowlist mode
   docker network create --driver=i2p \
     --opt i2p.filter.enabled=true \
     --opt i2p.filter.mode=allowlist \
     --opt i2p.filter.allowlist="trusted.i2p,*.example.i2p" \
     secure-network
   ```

2. **Test for leakage:**
   ```bash
   # Try to access clearnet from container
   docker run --rm --network i2p-network alpine \
     wget -T 10 -t 1 https://google.com
   
   # Should fail or be blocked
   ```

3. **Monitor for unauthorized traffic:**
   ```bash
   # Watch for blocked connections
   sudo journalctl -u i2p-network-plugin -f | grep "TRAFFIC BLOCK"
   
   # Check iptables rules
   sudo iptables -t nat -L I2P_REDIRECT -n -v
   sudo iptables -t filter -L I2P_FILTER -n -v
   ```

### Key Management Issues

**Symptoms:**
- Same I2P address across container restarts
- Key reuse between containers
- Weak cryptographic isolation

**Solutions:**

1. **Ensure key rotation:**
   ```bash
   # Remove container completely to generate new keys
   docker rm container-name
   docker run -d --name container-name \
     --network i2p-network \
     my-app:latest
   ```

2. **Verify key isolation:**
   ```bash
   # Check that different containers have different addresses
   docker logs container1 | grep "\.b32\.i2p"
   docker logs container2 | grep "\.b32\.i2p"
   # Addresses should be different
   ```

## Getting Help

### Information to Collect

When reporting issues, include:

1. **System Information:**
   ```bash
   # Docker version
   docker version
   
   # Operating system
   cat /etc/os-release
   
   # Plugin version
   ./bin/i2p-network-plugin -version
   ```

2. **Configuration:**
   ```bash
   # Environment variables
   env | grep -E "(I2P_|DEBUG|PLUGIN_)"
   
   # Network configuration
   docker network inspect network-name
   ```

3. **Logs:**
   ```bash
   # Plugin logs (last 100 lines)
   sudo journalctl -u i2p-network-plugin -n 100
   
   # Docker daemon logs
   sudo journalctl -u docker -n 50
   
   # I2P router logs (if accessible)
   tail -50 ~/.i2p/logs/log-*.txt
   ```

4. **Network State:**
   ```bash
   # Network interfaces
   ip addr show
   
   # iptables rules
   sudo iptables -t nat -L -n
   sudo iptables -t filter -L -n
   
   # Active connections
   netstat -tlnp | grep -E "(7656|1080|53)"
   ```

### Support Channels

- **GitHub Issues**: [Report bugs and feature requests](https://github.com/go-i2p/go-docker-network-i2p/issues)
- **Documentation**: Review [USAGE.md](USAGE.md) and [CONFIG.md](CONFIG.md)
- **I2P Community**: [I2P Community Forums](https://i2pforum.net/)
- **Docker Help**: [Docker Community Forums](https://forums.docker.com/)

### Before Reporting

1. **Search existing issues** for similar problems
2. **Try latest version** of the plugin
3. **Test with minimal configuration** to isolate the issue
4. **Verify I2P router** is working independently
5. **Include complete error messages** and logs

## See Also

- [USAGE.md](USAGE.md) - Usage examples and tutorials
- [CONFIG.md](CONFIG.md) - Configuration reference
- [DISTRIBUTION.md](DISTRIBUTION.md) - Distribution and packaging guide
- [Docker Network Troubleshooting](https://docs.docker.com/network/troubleshooting/)
- [I2P Troubleshooting Guide](https://geti2p.net/en/docs/how/troubleshoot)
