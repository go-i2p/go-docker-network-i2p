// Package plugin provides IP address allocation for I2P Docker networks.
//
// This file implements IP address management (IPAM) for containers on I2P networks,
// ensuring proper allocation and cleanup of IP addresses within network subnets.
package plugin

import (
	"fmt"
	"net"
	"sync"
)

// IPAllocator manages IP address allocation within a network subnet.
//
// The allocator tracks allocated IP addresses and provides allocation/deallocation
// operations for container endpoints. It ensures no IP conflicts within a network.
type IPAllocator struct {
	// subnet defines the IP address range for allocation
	subnet *net.IPNet

	// gateway is the gateway IP address (reserved, not allocatable)
	gateway net.IP

	// allocated tracks which IP addresses are currently in use
	allocated map[string]bool

	// nextIP tracks the next IP to try for allocation (optimization)
	nextIP net.IP

	// mutex protects concurrent access to allocation state
	mutex sync.Mutex
}

// NewIPAllocator creates a new IP allocator for the given subnet.
//
// The allocator will manage IP allocation within the subnet, reserving the
// gateway address and tracking allocated addresses.
func NewIPAllocator(subnet *net.IPNet, gateway net.IP) *IPAllocator {
	allocator := &IPAllocator{
		subnet:    subnet,
		gateway:   gateway,
		allocated: make(map[string]bool),
		nextIP:    make(net.IP, len(subnet.IP)),
	}

	// Start allocation from the second usable IP (first is typically gateway)
	copy(allocator.nextIP, subnet.IP)
	allocator.nextIP = allocator.nextIP.Mask(subnet.Mask)

	// Increment to first usable IP
	allocator.incrementIP(allocator.nextIP)
	// If that's the gateway, increment again
	if allocator.nextIP.Equal(gateway) {
		allocator.incrementIP(allocator.nextIP)
	}

	// Mark gateway as allocated (reserved)
	allocator.allocated[gateway.String()] = true

	return allocator
}

// AllocateIP allocates an available IP address from the subnet.
//
// Returns an allocated IP address or an error if no addresses are available.
// The allocated IP is marked as in-use until released.
func (a *IPAllocator) AllocateIP() (net.IP, error) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	startIP := make(net.IP, len(a.nextIP))
	copy(startIP, a.nextIP)

	// Try to find an available IP starting from nextIP
	for {
		ipStr := a.nextIP.String()

		// Check if this IP is available
		if !a.allocated[ipStr] && a.subnet.Contains(a.nextIP) {
			// Found available IP
			allocatedIP := make(net.IP, len(a.nextIP))
			copy(allocatedIP, a.nextIP)

			// Mark as allocated
			a.allocated[ipStr] = true

			// Advance nextIP for next allocation
			a.incrementIP(a.nextIP)

			return allocatedIP, nil
		}

		// Move to next IP
		a.incrementIP(a.nextIP)

		// Check if we've wrapped around to start (subnet exhausted)
		if a.nextIP.Equal(startIP) {
			return nil, fmt.Errorf("no available IP addresses in subnet %s", a.subnet)
		}

		// Check if we've gone outside the subnet (shouldn't happen with proper increment)
		if !a.subnet.Contains(a.nextIP) {
			// Wrap to beginning of subnet
			copy(a.nextIP, a.subnet.IP)
			a.nextIP = a.nextIP.Mask(a.subnet.Mask)
			a.incrementIP(a.nextIP)
		}
	}
}

// AllocateSpecificIP allocates a specific IP address if available.
//
// Returns an error if the IP is already allocated or outside the subnet.
// This is useful when Docker requests a specific IP address.
func (a *IPAllocator) AllocateSpecificIP(ip net.IP) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Check if IP is within our subnet
	if !a.subnet.Contains(ip) {
		return fmt.Errorf("IP %s is outside subnet %s", ip, a.subnet)
	}

	ipStr := ip.String()

	// Check if IP is already allocated
	if a.allocated[ipStr] {
		return fmt.Errorf("IP %s is already allocated", ip)
	}

	// Allocate the IP
	a.allocated[ipStr] = true

	return nil
}

// ReleaseIP releases a previously allocated IP address.
//
// The IP address becomes available for future allocation. It's safe to call
// this method with an IP that wasn't allocated or is already released.
func (a *IPAllocator) ReleaseIP(ip net.IP) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	ipStr := ip.String()

	// Don't release the gateway IP
	if ip.Equal(a.gateway) {
		return
	}

	// Release the IP
	delete(a.allocated, ipStr)
}

// IsAllocated checks if an IP address is currently allocated.
//
// Returns true if the IP is allocated, false otherwise.
func (a *IPAllocator) IsAllocated(ip net.IP) bool {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	return a.allocated[ip.String()]
}

// GetAllocatedIPs returns a slice of all currently allocated IP addresses.
//
// This is useful for debugging and monitoring IP allocation state.
func (a *IPAllocator) GetAllocatedIPs() []net.IP {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	var ips []net.IP
	for ipStr := range a.allocated {
		if ip := net.ParseIP(ipStr); ip != nil {
			ips = append(ips, ip)
		}
	}

	return ips
}

// GetAvailableCount returns the number of available IP addresses.
//
// This provides insight into subnet utilization for monitoring and planning.
func (a *IPAllocator) GetAvailableCount() int {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// Calculate total IPs in subnet
	ones, bits := a.subnet.Mask.Size()
	totalIPs := 1 << (bits - ones)

	// Subtract network and broadcast addresses for IPv4
	if len(a.subnet.IP) == 4 { // IPv4
		totalIPs -= 2 // network and broadcast
	}

	// Subtract allocated IPs
	allocated := len(a.allocated)

	available := totalIPs - allocated
	if available < 0 {
		available = 0
	}

	return available
}

// incrementIP increments an IP address by 1.
//
// This handles both IPv4 and IPv6 addresses, modifying the IP in-place.
// The increment wraps at the maximum value for each byte.
func (a *IPAllocator) incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}
