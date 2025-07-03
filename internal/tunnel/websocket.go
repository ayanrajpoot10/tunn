package tunnel

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"strings"
)

// ReplacePlaceholders replaces [host] and [crlf] placeholders in the payload
// If frontDomain is provided, it will be used for the Host header instead of targetHost
func ReplacePlaceholders(payload, targetHost, targetPort, frontDomain string) []byte {
	var hostValue string
	if frontDomain != "" {
		// Use front domain for Host header spoofing
		hostValue = frontDomain
	} else {
		// Use target host and port as normal
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
			if err == io.EOF {
				break
			}
			return nil, err
		}
		if n > 0 {
			data = append(data, buffer[0])
			if bytes.Contains(data, []byte("\r\n\r\n")) {
				break
			}
		}
	}
	return data, nil
}

// EstablishWSTunnel performs the WebSocket upgrade handshake
func EstablishWSTunnel(proxyHost, proxyPort, targetHost, targetPort, payload, frontDomain string, useTLS bool, conn net.Conn) (net.Conn, error) {
	// 1. Connect or re-use an existing connection
	if conn == nil {
		var address string
		// If proxy host/port are provided, connect to proxy; otherwise connect directly to target
		if proxyHost != "" && proxyPort != "" {
			address = net.JoinHostPort(proxyHost, proxyPort)
		} else {
			// Direct mode - connect directly to target
			address = net.JoinHostPort(targetHost, targetPort)
		}

		var err error
		conn, err = net.Dial("tcp", address)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to %s: %v", address, err)
		}
	}

	// Optional TLS upgrade (skip if it's already TLS)
	if useTLS {
		if _, isTLS := conn.(*tls.Conn); !isTLS {
			var serverName string
			// Use front domain for TLS if specified, otherwise use the appropriate host
			if frontDomain != "" {
				serverName = frontDomain
			} else if proxyHost != "" {
				serverName = proxyHost
			} else {
				serverName = targetHost
			}

			tlsConfig := &tls.Config{
				ServerName: serverName,
			}
			conn = tls.Client(conn, tlsConfig)
		}
	}

	// 2. Build payload blocks
	payloadBytes := ReplacePlaceholders(payload, targetHost, targetPort, frontDomain)
	blocks := bytes.Split(payloadBytes, []byte("\r\n\r\n"))

	// 3. Send first block
	firstBlock := append(blocks[0], []byte("\r\n\r\n")...)
	_, err := conn.Write(firstBlock)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to send first block: %v", err)
	}

	// 4. Read first response
	firstResponse, err := ReadHeaders(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to read first response: %v", err)
	}

	fmt.Printf(">> Response:\n%s\n", string(firstResponse))

	// 5. Check for WebSocket upgrade (101 response)
	responseStr := string(firstResponse)
	if !strings.Contains(responseStr, "101") {
		conn.Close()
		return nil, fmt.Errorf("WebSocket upgrade failed - expected 101 response, got: %s", responseStr)
	}

	// WebSocket upgrade successful - send any remaining blocks
	fmt.Println("[*] WebSocket upgrade successful (101 response) - tunnel ready.")
	for i := 1; i < len(blocks); i++ {
		if len(bytes.TrimSpace(blocks[i])) > 0 {
			block := append(blocks[i], []byte("\r\n\r\n")...)
			_, err := conn.Write(block)
			if err != nil {
				conn.Close()
				return nil, fmt.Errorf("failed to send block %d: %v", i, err)
			}
		}
	}

	// 6. Tunnel is live
	fmt.Println("[*] WebSocket handshake complete – returning raw connection.")
	return conn, nil
}
