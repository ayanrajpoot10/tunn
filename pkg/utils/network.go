// Package utils provides utility functions for the Tunn SSH tunneling tool.
//
// This package contains common utility functions used across the Tunn application,
// focusing on network operations, string processing, and other shared functionality
// that doesn't belong to specific components.
//
// The utilities are designed to be simple, reliable, and reusable across different
// parts of the application, promoting code consistency and reducing duplication.
package utils

import (
	"fmt"
	"net"
	"strconv"
)

// ParseHostPort parses a host:port string with intelligent default port handling.
//
// This function extends the standard net.SplitHostPort functionality by providing
// default port fallback and support for common named ports. It handles various
// input formats gracefully and provides consistent host:port parsing across the
// application.
//
// Key features:
//   - Automatic default port assignment when port is omitted
//   - Support for named ports ("http" -> 80, "https" -> 443)
//   - Graceful handling of malformed input
//   - Consistent error reporting for invalid ports
//
// Parameters:
//   - hostPort: Host and port string in various formats:
//   - "hostname:port" - Standard format
//   - "hostname:http" - Named port
//   - "hostname" - Host only (uses default port)
//   - defaultPort: Port to use when not specified in hostPort
//
// Returns:
//   - string: The parsed hostname or IP address
//   - int: The parsed or default port number
//   - error: An error if port parsing fails (invalid numeric port)
//
// Examples:
//
//	host, port, err := ParseHostPort("example.com:8080", 80)
//	// Returns: "example.com", 8080, nil
//
//	host, port, err := ParseHostPort("example.com:https", 80)
//	// Returns: "example.com", 443, nil
//
//	host, port, err := ParseHostPort("example.com", 80)
//	// Returns: "example.com", 80, nil
func ParseHostPort(hostPort string, defaultPort int) (string, int, error) {
	host, portStr, err := net.SplitHostPort(hostPort)
	if err != nil {
		return hostPort, defaultPort, nil
	}

	// Handle named ports
	switch portStr {
	case "https":
		return host, 443, nil
	case "http":
		return host, 80, nil
	default:
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("invalid port: %s", portStr)
		}
		return host, port, nil
	}
}
