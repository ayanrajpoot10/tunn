package connection

import (
	"bytes"
	"fmt"
	"net"
	"strings"
)

// ReplacePlaceholders replaces [host] and [crlf] placeholders in the payload
func ReplacePlaceholders(payload, targetHost, targetPort, hostHeader string) []byte {
	hostValue := hostHeader
	if hostValue == "" {
		hostValue = net.JoinHostPort(targetHost, targetPort)
	}

	payload = strings.ReplaceAll(payload, "[host]", hostValue)
	payload = strings.ReplaceAll(payload, "[crlf]", "\r\n")
	return []byte(payload)
}

// ReadHeaders reads from the connection until a blank line is reached
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

// EstablishWSTunnel performs the WebSocket upgrade handshake
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
		responseText := strings.TrimSpace(string(headers))
		fmt.Printf("  %s\n", strings.ReplaceAll(responseText, "\r\n", "\n  "))

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
