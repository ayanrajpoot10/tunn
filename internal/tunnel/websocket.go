package tunnel

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
)

// ReplacePlaceholders replaces [host] and [crlf] placeholders in the payload
func ReplacePlaceholders(payload, targetHost, targetPort, frontDomain string) []byte {
	hostValue := frontDomain
	if hostValue == "" {
		hostValue = fmt.Sprintf("%s:%s", targetHost, targetPort)
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
			// Check if we have the complete header (ending with \r\n\r\n)
			if len(data) >= 4 && bytes.HasSuffix(data, []byte("\r\n\r\n")) {
				break
			}
		}
	}
	return data, nil
}

// EstablishWSTunnel performs the WebSocket upgrade handshake
func EstablishWSTunnel(proxyHost, proxyPort, targetHost, targetPort, payload, frontDomain string, useTLS bool, conn net.Conn) (net.Conn, error) {
	// Connect if no existing connection
	if conn == nil {
		address := net.JoinHostPort(targetHost, targetPort)
		if proxyHost != "" && proxyPort != "" {
			address = net.JoinHostPort(proxyHost, proxyPort)
		}

		var err error
		conn, err = net.Dial("tcp", address)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to %s: %v", address, err)
		}
	}

	// TLS upgrade if needed
	if useTLS {
		if _, isTLS := conn.(*tls.Conn); !isTLS {
			serverName := targetHost
			if frontDomain != "" {
				serverName = frontDomain
			} else if proxyHost != "" {
				serverName = proxyHost
			}

			conn = tls.Client(conn, &tls.Config{ServerName: serverName})
		}
	}

	// Process payload
	payloadBytes := ReplacePlaceholders(payload, targetHost, targetPort, frontDomain)
	blocks := bytes.Split(payloadBytes, []byte("\r\n\r\n"))

	// Send first block and read response
	if _, err := conn.Write(append(blocks[0], []byte("\r\n\r\n")...)); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send first block: %v", err)
	}

	response, err := ReadHeaders(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read response: %v", err)
	}

	// Check for WebSocket upgrade (101 response)
	if !strings.Contains(string(response), "101") {
		conn.Close()
		return nil, fmt.Errorf("WebSocket upgrade failed - expected 101 response, got: %s", string(response))
	}

	// Send remaining blocks
	for i := 1; i < len(blocks); i++ {
		if len(bytes.TrimSpace(blocks[i])) > 0 {
			if _, err := conn.Write(append(blocks[i], []byte("\r\n\r\n")...)); err != nil {
				conn.Close()
				return nil, fmt.Errorf("failed to send block %d: %v", i, err)
			}
		}
	}

	fmt.Println("[*] WebSocket handshake complete – tunnel ready.")
	return conn, nil
}
