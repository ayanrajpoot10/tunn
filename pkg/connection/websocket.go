package connection

import (
	"bytes"
	"fmt"
	"net"
	"strings"
)

// ReplacePlaceholders performs template substitution in HTTP payload strings.
//
// This function replaces common placeholders in WebSocket upgrade payloads with
// actual values, enabling dynamic payload generation based on connection parameters.
//
// Supported placeholders:
//   - [host]: Replaced with the hostHeader value, or targetHost:targetPort if hostHeader is empty
//   - [crlf]: Replaced with HTTP line endings (\r\n)
//
// Parameters:
//   - payload: The template payload string containing placeholders
//   - targetHost: The target server hostname
//   - targetPort: The target server port
//   - hostHeader: Optional custom host header value (uses targetHost:targetPort if empty)
//
// Returns:
//   - []byte: The processed payload with placeholders replaced
//
// Example:
//
//	payload := "GET / HTTP/1.1[crlf]Host: [host][crlf]Upgrade: websocket[crlf][crlf]"
//	result := ReplacePlaceholders(payload, "example.com", "80", "")
//	// result contains: "GET / HTTP/1.1\r\nHost: example.com:80\r\nUpgrade: websocket\r\n\r\n"
func ReplacePlaceholders(payload, targetHost, targetPort, hostHeader string) []byte {
	hostValue := hostHeader
	if hostValue == "" {
		hostValue = net.JoinHostPort(targetHost, targetPort)
	}

	payload = strings.ReplaceAll(payload, "[host]", hostValue)
	payload = strings.ReplaceAll(payload, "[crlf]", "\r\n")
	return []byte(payload)
}

// ReadHeaders reads HTTP response headers from a connection until the header section ends.
//
// This function reads data byte-by-byte from the connection until it encounters
// the HTTP header terminator sequence (\r\n\r\n), which indicates the end of
// the header section in an HTTP response.
//
// The function is designed for reading WebSocket upgrade responses where you
// need to parse the HTTP headers to verify successful upgrade.
//
// Parameters:
//   - conn: The network connection to read from
//
// Returns:
//   - []byte: The complete header section including the terminating \r\n\r\n
//   - error: A network error if reading fails
//
// Note: This function reads one byte at a time and may be slow for large headers.
// It's optimized for the typical case of small WebSocket upgrade response headers.
func ReadHeaders(conn net.Conn) ([]byte, error) {
	var data []byte
	buffer := make([]byte, 1)

	for {
		n, err := conn.Read(buffer)
		if err != nil {
			return nil, err
		}
		if n > 0 {
			data = append(data, buffer[0])
			if len(data) >= 4 && bytes.HasSuffix(data, []byte("\r\n\r\n")) {
				break
			}
		}
	}
	return data, nil
}

// EstablishWSTunnel performs a WebSocket upgrade handshake over an existing connection.
//
// This function sends a WebSocket upgrade request using a custom HTTP payload and
// validates the server's response to ensure the upgrade was successful. It supports
// both HTTP/1.0 and HTTP/1.1 upgrade responses.
//
// The function is essential for bypassing network restrictions by tunneling SSH
// traffic over WebSocket connections, which appear as regular HTTP traffic to
// network monitoring systems.
//
// Parameters:
//   - conn: An established network connection to upgrade
//   - payload: HTTP payload template for the WebSocket upgrade request
//   - targetHost: Target server hostname for placeholder replacement
//   - targetPort: Target server port for placeholder replacement
//   - hostHeader: Optional custom host header (uses targetHost:targetPort if empty)
//
// Returns:
//   - net.Conn: The same connection, now upgraded to WebSocket
//   - error: An error if the upgrade request fails or server rejects the upgrade
//
// The function expects a successful WebSocket upgrade response (HTTP 101) from the server.
// If the server responds with any other status code, the upgrade is considered failed
// and an error is returned.
//
// Example payload:
//
//	payload := "GET / HTTP/1.1[crlf]Host: [host][crlf]Upgrade: websocket[crlf]Connection: Upgrade[crlf][crlf]"
func EstablishWSTunnel(conn net.Conn, payload, targetHost, targetPort, hostHeader string) (net.Conn, error) {
	if conn == nil {
		return nil, fmt.Errorf("connection must be established before WebSocket upgrade")
	}

	// Send WebSocket upgrade request
	if payload != "" {
		wsPayload := ReplacePlaceholders(payload, targetHost, targetPort, hostHeader)
		fmt.Printf("→ Sending WebSocket upgrade request\n")

		if _, err := conn.Write(wsPayload); err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to send WebSocket upgrade: %w", err)
		}

		// Read the response headers
		headers, err := ReadHeaders(conn)
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to read WebSocket response: %w", err)
		}

		// Print the response received from WebSocket request
		fmt.Printf("← WebSocket response received:\n")
		fmt.Printf("  %s\n", strings.SplitN(strings.TrimSpace(string(headers)), "\n", 2)[0])

		// Check if upgrade was successful
		headerStr := string(headers)
		if !strings.Contains(headerStr, "HTTP/1.1 101") &&
			!strings.Contains(headerStr, "HTTP/1.0 101") {
			conn.Close()
			return nil, fmt.Errorf("WebSocket upgrade failed: %s", headerStr)
		}

		fmt.Printf("✓ WebSocket tunnel established\n")
	}

	return conn, nil
}
