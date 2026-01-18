package utils

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ExpandEndpoints expands CIDRs to individual IPs, passes through URLs, domains, and IPs as-is
func ExpandEndpoints(endpoints []string) ([]string, error) {
	var result []string

	for _, endpoint := range endpoints {
		// Check if it's a URL (contains ://)
		if strings.Contains(endpoint, "://") {
			// It's a URL, pass through as-is
			result = append(result, endpoint)
			continue
		}

		// Check if it contains a slash (could be CIDR)
		if strings.Contains(endpoint, "/") {
			// Try to parse as CIDR
			ips, err := expandCIDR(endpoint)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %s: %w", endpoint, err)
			}
			result = append(result, ips...)
		} else {
			// Pass through as-is (IP or domain)
			result = append(result, endpoint)
		}
	}

	return result, nil
}

// expandCIDR expands a CIDR notation to individual IPs
func expandCIDR(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}

	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); incIP(ip) {
		ips = append(ips, ip.String())
	}

	// Remove network and broadcast addresses for IPv4
	if len(ips) > 2 {
		return ips[1 : len(ips)-1], nil
	}

	return ips, nil
}

// incIP increments an IP address
func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// ExpandPorts expands port ranges (e.g., "22-80") to individual ports
func ExpandPorts(ports []string) ([]string, error) {
	var result []string

	for _, portSpec := range ports {
		if strings.Contains(portSpec, "-") {
			// Port range
			expanded, err := expandPortRange(portSpec)
			if err != nil {
				return nil, fmt.Errorf("invalid port range %s: %w", portSpec, err)
			}
			result = append(result, expanded...)
		} else {
			// Single port
			result = append(result, portSpec)
		}
	}

	return result, nil
}

// expandPortRange expands a port range like "22-80" to ["22", "23", ..., "80"]
func expandPortRange(portRange string) ([]string, error) {
	parts := strings.Split(portRange, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid port range format: %s", portRange)
	}

	start, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, fmt.Errorf("invalid start port: %w", err)
	}

	end, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, fmt.Errorf("invalid end port: %w", err)
	}

	if start > end || start < 1 || end > 65535 {
		return nil, fmt.Errorf("invalid port range: %d-%d", start, end)
	}

	var ports []string
	for i := start; i <= end; i++ {
		ports = append(ports, strconv.Itoa(i))
	}

	return ports, nil
}

// IsURL checks if a string is a URL
func IsURL(s string) bool {
	_, err := url.Parse(s)
	return err == nil && strings.Contains(s, "://")
}
