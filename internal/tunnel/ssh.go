package tunnel

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHOverWebSocket wraps an SSH connection over a WebSocket-upgraded connection
type SSHOverWebSocket struct {
	conn        net.Conn
	sshClient   *ssh.Client
	sshUsername string
	sshPassword string
	sshPort     string
}

// NewSSHOverWebSocket creates a new SSH over WebSocket connection
func NewSSHOverWebSocket(conn net.Conn, sshUsername, sshPassword, sshPort string) *SSHOverWebSocket {
	return &SSHOverWebSocket{
		conn:        conn,
		sshUsername: sshUsername,
		sshPassword: sshPassword,
		sshPort:     sshPort,
	}
}

// StartSSHTransport initializes the SSH client over the WebSocket connection
func (s *SSHOverWebSocket) StartSSHTransport() error {
	fmt.Println("[*] Starting SSH transport over WebSocket connection...")

	// Set keepalive on the underlying WebSocket connection if it's TCP
	if tcpConn, ok := s.conn.(*net.TCPConn); ok {
		fmt.Println("[*] Setting keepalive on WebSocket connection...")
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	// Clear any previous deadlines
	s.conn.SetReadDeadline(time.Time{})
	s.conn.SetWriteDeadline(time.Time{})

	config := &ssh.ClientConfig{
		User: s.sshUsername,
		Auth: []ssh.AuthMethod{
			ssh.Password(s.sshPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // WARNING: This is insecure for production
		Timeout:         15 * time.Second,            // Reasonable timeout for SSH handshake
	}

	fmt.Printf("[*] Attempting SSH connection with user: %s\n", s.sshUsername)

	// Create SSH client using the WebSocket connection
	sshConn, chans, reqs, err := ssh.NewClientConn(s.conn, "tcp", config)
	if err != nil {
		return fmt.Errorf("failed to create SSH connection: %v", err)
	}

	s.sshClient = ssh.NewClient(sshConn, chans, reqs)
	fmt.Println("[*] SSH transport established and authenticated.")
	return nil
}

// Close closes the SSH connection
func (s *SSHOverWebSocket) Close() error {
	if s.sshClient != nil {
		return s.sshClient.Close()
	}
	return nil
}

// OpenSOCKSProxy starts a SOCKS proxy server on the specified local port
func (s *SSHOverWebSocket) OpenSOCKSProxy(localPort int) error {
	return s.startProxy("SOCKS", localPort, s.handleSOCKSClient)
}

// OpenHTTPProxy starts an HTTP proxy server on the specified local port
func (s *SSHOverWebSocket) OpenHTTPProxy(localPort int) error {
	return s.startProxy("HTTP", localPort, s.handleHTTPClient)
}

// startProxy starts a generic proxy server
func (s *SSHOverWebSocket) startProxy(proxyType string, localPort int, handler func(net.Conn)) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return fmt.Errorf("failed to start %s proxy: %v", proxyType, err)
	}

	fmt.Printf("[*] %s proxy listening on 127.0.0.1:%d\n", proxyType, localPort)

	go func() {
		defer listener.Close()
		for {
			clientConn, err := listener.Accept()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && !netErr.Temporary() {
					fmt.Printf("[*] %s proxy listener closed\n", proxyType)
					return
				}
				fmt.Printf("[!] Error accepting connection: %v\n", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			go handler(clientConn)
		}
	}()

	fmt.Printf("[*] %s proxy started.\n", proxyType)
	return nil
}

// handleSOCKSClient handles a SOCKS client connection
func (s *SSHOverWebSocket) handleSOCKSClient(clientConn net.Conn) {
	s.handleClientWithTimeout(clientConn, "SOCKS", 10*time.Second, func() {
		versionByte := make([]byte, 1)
		if _, err := clientConn.Read(versionByte); err != nil {
			fmt.Printf("[!] Error reading SOCKS version: %v\n", err)
			return
		}

		switch versionByte[0] {
		case 5:
			s.handleSOCKS5(clientConn, versionByte[0])
		default:
			fmt.Printf("[!] Unsupported SOCKS version: %d (only SOCKS5 supported)\n", versionByte[0])
		}
	})
}

// handleHTTPClient handles an HTTP proxy client connection
func (s *SSHOverWebSocket) handleHTTPClient(clientConn net.Conn) {
	s.handleClientWithTimeout(clientConn, "HTTP", 30*time.Second, func() {
		reader := bufio.NewReader(clientConn)
		req, err := http.ReadRequest(reader)
		if err != nil {
			fmt.Printf("[!] Error reading HTTP request: %v\n", err)
			s.sendHTTPError(clientConn, 400, "Bad Request")
			return
		}

		if req.Method == "CONNECT" {
			s.handleHTTPConnect(clientConn, req)
		} else {
			s.handleHTTPRequest(clientConn, req)
		}
	})
}

// handleClientWithTimeout provides common client handling with timeout and panic recovery
func (s *SSHOverWebSocket) handleClientWithTimeout(clientConn net.Conn, clientType string, timeout time.Duration, handler func()) {
	defer func() {
		clientConn.Close()
		if r := recover(); r != nil {
			fmt.Printf("[!] Panic in %s handler: %v\n", clientType, r)
		}
	}()

	// Set initial timeout
	clientConn.SetReadDeadline(time.Now().Add(timeout))
	clientConn.SetWriteDeadline(time.Now().Add(timeout))

	handler()
}

// handleSOCKS5 handles SOCKS5 protocol
func (s *SSHOverWebSocket) handleSOCKS5(clientConn net.Conn, version byte) {
	// Read number of methods
	nmethodsByte := make([]byte, 1)
	_, err := clientConn.Read(nmethodsByte)
	if err != nil {
		fmt.Printf("[!] Error reading SOCKS5 nmethods: %v\n", err)
		return
	}

	nmethods := int(nmethodsByte[0])
	methods := make([]byte, nmethods)
	_, err = io.ReadFull(clientConn, methods)
	if err != nil {
		fmt.Printf("[!] Error reading SOCKS5 methods: %v\n", err)
		return
	}

	// Send method selection (no auth)
	clientConn.Write([]byte{5, 0})

	// Read connection request
	requestHeader := make([]byte, 4) // ver, cmd, rsv, atyp
	_, err = io.ReadFull(clientConn, requestHeader)
	if err != nil {
		fmt.Printf("[!] Error reading SOCKS5 request header: %v\n", err)
		return
	}

	cmd := requestHeader[1]
	atyp := requestHeader[3]

	if cmd != 1 { // Only CONNECT supported
		s.sendSOCKS5Error(clientConn, 7) // Command not supported
		return
	}

	var host string
	var port int

	// Parse address based on type
	switch atyp {
	case 1: // IPv4
		addr := make([]byte, 4)
		_, err = io.ReadFull(clientConn, addr)
		if err != nil {
			s.sendSOCKS5Error(clientConn, 1)
			return
		}
		host = fmt.Sprintf("%d.%d.%d.%d", addr[0], addr[1], addr[2], addr[3])

	case 3: // Domain name
		lengthByte := make([]byte, 1)
		_, err = clientConn.Read(lengthByte)
		if err != nil {
			s.sendSOCKS5Error(clientConn, 1)
			return
		}

		length := int(lengthByte[0])
		domain := make([]byte, length)
		_, err = io.ReadFull(clientConn, domain)
		if err != nil {
			s.sendSOCKS5Error(clientConn, 1)
			return
		}
		host = string(domain)

	case 4: // IPv6
		addr := make([]byte, 16)
		_, err = io.ReadFull(clientConn, addr)
		if err != nil {
			s.sendSOCKS5Error(clientConn, 1)
			return
		}
		host = fmt.Sprintf("[%x:%x:%x:%x:%x:%x:%x:%x]",
			binary.BigEndian.Uint16(addr[0:2]),
			binary.BigEndian.Uint16(addr[2:4]),
			binary.BigEndian.Uint16(addr[4:6]),
			binary.BigEndian.Uint16(addr[6:8]),
			binary.BigEndian.Uint16(addr[8:10]),
			binary.BigEndian.Uint16(addr[10:12]),
			binary.BigEndian.Uint16(addr[12:14]),
			binary.BigEndian.Uint16(addr[14:16]))

	default:
		s.sendSOCKS5Error(clientConn, 8) // Address type not supported
		return
	}

	// Read port
	portBytes := make([]byte, 2)
	_, err = io.ReadFull(clientConn, portBytes)
	if err != nil {
		s.sendSOCKS5Error(clientConn, 1)
		return
	}
	port = int(binary.BigEndian.Uint16(portBytes))

	// Send success response
	s.sendSOCKS5Success(clientConn)

	// Open SSH channel
	s.openSSHChannel(clientConn, host, port)
}

// sendSOCKS5Error sends a SOCKS5 error response
func (s *SSHOverWebSocket) sendSOCKS5Error(clientConn net.Conn, errCode byte) {
	response := []byte{5, errCode, 0, 1, 0, 0, 0, 0, 0, 0}
	clientConn.Write(response)
}

// sendSOCKS5Success sends a SOCKS5 success response
func (s *SSHOverWebSocket) sendSOCKS5Success(clientConn net.Conn) {
	response := []byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}
	clientConn.Write(response)
}

// handleHTTPConnect handles HTTPS tunneling via CONNECT method
func (s *SSHOverWebSocket) handleHTTPConnect(clientConn net.Conn, req *http.Request) {
	host, portInt, err := s.parseHostPort(req.Host, 443)
	if err != nil {
		fmt.Printf("[!] Invalid host in CONNECT request: %v\n", err)
		s.sendHTTPError(clientConn, 400, "Bad Request")
		return
	}

	fmt.Printf("[*] HTTP CONNECT request to %s:%d\n", host, portInt)

	// Send success response
	response := "HTTP/1.1 200 Connection established\r\n\r\n"
	if _, err := clientConn.Write([]byte(response)); err != nil {
		fmt.Printf("[!] Error sending CONNECT response: %v\n", err)
		return
	}

	fmt.Printf("[+] HTTP CONNECT tunnel established to %s:%d\n", host, portInt)
	s.openSSHChannel(clientConn, host, portInt)
}

// parseHostPort parses host:port with default port fallback
func (s *SSHOverWebSocket) parseHostPort(hostPort string, defaultPort int) (string, int, error) {
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

// handleHTTPRequest handles regular HTTP requests (GET, POST, etc.)
func (s *SSHOverWebSocket) handleHTTPRequest(clientConn net.Conn, req *http.Request) {
	targetHost, targetPort, targetPath, err := s.parseHTTPTarget(req)
	if err != nil {
		fmt.Printf("[!] Error parsing HTTP target: %v\n", err)
		s.sendHTTPError(clientConn, 400, "Bad Request")
		return
	}

	fmt.Printf("[*] HTTP %s request to %s:%d%s\n", req.Method, targetHost, targetPort, targetPath)

	// Open SSH channel to target
	sshConn, err := s.sshClient.Dial("tcp", fmt.Sprintf("%s:%d", targetHost, targetPort))
	if err != nil {
		fmt.Printf("[!] Failed to open SSH channel for HTTP request: %v\n", err)
		s.sendHTTPError(clientConn, 502, "Bad Gateway")
		return
	}
	defer sshConn.Close()

	// Forward the HTTP request and response
	if err := s.forwardHTTPRequest(sshConn, req, targetPath); err != nil {
		fmt.Printf("[!] Error forwarding HTTP request: %v\n", err)
		s.sendHTTPError(clientConn, 502, "Bad Gateway")
		return
	}

	s.forwardHTTPResponse(clientConn, sshConn)
}

// parseHTTPTarget parses the target from HTTP request
func (s *SSHOverWebSocket) parseHTTPTarget(req *http.Request) (host string, port int, path string, err error) {
	if req.URL.IsAbs() {
		// Full URL provided (typical for proxy requests)
		parsedURL, err := url.Parse(req.URL.String())
		if err != nil {
			return "", 0, "", err
		}

		host = parsedURL.Hostname()
		if parsedURL.Port() != "" {
			port, err = strconv.Atoi(parsedURL.Port())
			if err != nil {
				return "", 0, "", fmt.Errorf("invalid port in URL: %s", parsedURL.Port())
			}
		} else {
			port = 80
			if parsedURL.Scheme == "https" {
				port = 443
			}
		}
		path = parsedURL.RequestURI()
	} else {
		// Relative URL - use Host header
		if req.Host == "" {
			return "", 0, "", fmt.Errorf("no Host header in HTTP request")
		}

		host, port, err = s.parseHostPort(req.Host, 80)
		if err != nil {
			return "", 0, "", fmt.Errorf("invalid Host header: %v", err)
		}
		path = req.URL.RequestURI()
	}

	return host, port, path, nil
}

// forwardHTTPRequest reconstructs and sends the HTTP request through SSH tunnel
func (s *SSHOverWebSocket) forwardHTTPRequest(sshConn net.Conn, req *http.Request, targetPath string) error {
	// Reconstruct the request
	var requestBuilder strings.Builder

	// Request line
	requestBuilder.WriteString(fmt.Sprintf("%s %s %s\r\n", req.Method, targetPath, req.Proto))

	// Headers (excluding proxy-specific headers)
	for name, values := range req.Header {
		// Skip proxy-specific headers
		if strings.ToLower(name) == "proxy-connection" {
			continue
		}
		for _, value := range values {
			requestBuilder.WriteString(fmt.Sprintf("%s: %s\r\n", name, value))
		}
	}

	// End of headers
	requestBuilder.WriteString("\r\n")

	// Send request headers
	_, err := sshConn.Write([]byte(requestBuilder.String()))
	if err != nil {
		return err
	}

	// Forward request body if present
	if req.Body != nil {
		_, err = io.Copy(sshConn, req.Body)
		req.Body.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// forwardHTTPResponse forwards the HTTP response from SSH tunnel back to client
func (s *SSHOverWebSocket) forwardHTTPResponse(clientConn net.Conn, sshConn net.Conn) {
	// Simply forward all data from SSH connection back to client
	_, err := io.Copy(clientConn, sshConn)
	if err != nil && err != io.EOF {
		fmt.Printf("[!] Error forwarding HTTP response: %v\n", err)
	}
}

// sendHTTPError sends an HTTP error response
func (s *SSHOverWebSocket) sendHTTPError(clientConn net.Conn, statusCode int, statusText string) {
	response := fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", statusCode, statusText)
	clientConn.Write([]byte(response))
}

// openSSHChannel opens an SSH channel and forwards data
func (s *SSHOverWebSocket) openSSHChannel(clientConn net.Conn, host string, port int) {
	fmt.Printf("[*] Opening SSH channel to %s:%d\n", host, port)

	// Open SSH channel with retry logic
	sshConn, err := s.dialWithRetry(host, port)
	if err != nil {
		fmt.Printf("[!] Failed to open SSH channel: %v\n", err)
		return
	}
	defer sshConn.Close()

	fmt.Printf("[+] SSH channel established to %s:%d\n", host, port)

	// Forward data bidirectionally
	s.forwardData(clientConn, sshConn)
	fmt.Printf("[*] SSH channel to %s:%d closed\n", host, port)
}

// dialWithRetry attempts to dial with retry logic
func (s *SSHOverWebSocket) dialWithRetry(host string, port int) (net.Conn, error) {
	var conn net.Conn
	var err error

	for retries := 0; retries < 3; retries++ {
		conn, err = s.sshClient.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
		if err == nil {
			return conn, nil
		}

		fmt.Printf("[!] SSH channel attempt %d failed: %v\n", retries+1, err)
		if retries < 2 {
			time.Sleep(time.Duration(retries+1) * time.Second)
		}
	}

	return nil, fmt.Errorf("failed after 3 attempts: %w", err)
}

// forwardData forwards data bidirectionally between two connections
func (s *SSHOverWebSocket) forwardData(conn1, conn2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Forward conn1 -> conn2
	go func() {
		defer wg.Done()
		s.copyData(conn1, conn2, "client", "SSH")
	}()

	// Forward conn2 -> conn1
	go func() {
		defer wg.Done()
		s.copyData(conn2, conn1, "SSH", "client")
	}()

	wg.Wait()
}

// copyData copies data from src to dst with proper error handling
func (s *SSHOverWebSocket) copyData(src, dst net.Conn, srcName, dstName string) {
	buffer := make([]byte, 32*1024) // 32KB buffer

	for {
		// Set read timeout
		src.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := src.Read(buffer)

		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue // Continue on timeout
			}
			if err != io.EOF {
				fmt.Printf("[!] Error reading from %s: %v\n", srcName, err)
			}
			return
		}

		if n > 0 {
			// Set write timeout
			dst.SetWriteDeadline(time.Now().Add(30 * time.Second))
			_, err := dst.Write(buffer[:n])
			if err != nil {
				if err != io.EOF {
					fmt.Printf("[!] Error writing to %s: %v\n", dstName, err)
				}
				return
			}
		}
	}
}

// ConnectViaWSAndStartSOCKS is a convenience function to start everything
func ConnectViaWSAndStartSOCKS(wsConn net.Conn, sshUser, sshPassword, sshPort string, localPort int) (*SSHOverWebSocket, error) {
	connector := NewSSHOverWebSocket(wsConn, sshUser, sshPassword, sshPort)

	err := connector.StartSSHTransport()
	if err != nil {
		return nil, err
	}

	err = connector.OpenSOCKSProxy(localPort)
	if err != nil {
		return nil, err
	}

	return connector, nil
}

// ConnectViaWSAndStartHTTP is a convenience function to start HTTP proxy
func ConnectViaWSAndStartHTTP(wsConn net.Conn, sshUser, sshPassword, sshPort string, localPort int) (*SSHOverWebSocket, error) {
	connector := NewSSHOverWebSocket(wsConn, sshUser, sshPassword, sshPort)

	err := connector.StartSSHTransport()
	if err != nil {
		return nil, err
	}

	err = connector.OpenHTTPProxy(localPort)
	if err != nil {
		return nil, err
	}

	return connector, nil
}
