package tunnel

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
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
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return fmt.Errorf("failed to start SOCKS proxy: %v", err)
	}

	fmt.Printf("[*] SOCKS proxy listening on 127.0.0.1:%d\n", localPort)

	go func() {
		defer listener.Close()
		for {
			clientConn, err := listener.Accept()
			if err != nil {
				// Check if this is because the listener was closed
				if netErr, ok := err.(net.Error); ok && !netErr.Temporary() {
					fmt.Printf("[*] SOCKS proxy listener closed\n")
					return
				}
				fmt.Printf("[!] Error accepting connection: %v\n", err)
				// Brief pause before retrying
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Handle each client in a separate goroutine
			go s.handleSOCKSClient(clientConn)
		}
	}()

	fmt.Println("[*] SOCKS proxy started.")
	return nil
}

// handleSOCKSClient handles a SOCKS client connection
func (s *SSHOverWebSocket) handleSOCKSClient(clientConn net.Conn) {
	defer func() {
		clientConn.Close()
		if r := recover(); r != nil {
			fmt.Printf("[!] Panic in SOCKS handler: %v\n", r)
		}
	}()

	// Set initial timeout for SOCKS handshake
	clientConn.SetReadDeadline(time.Now().Add(10 * time.Second))
	clientConn.SetWriteDeadline(time.Now().Add(10 * time.Second))

	// Read the first byte to determine SOCKS version
	versionByte := make([]byte, 1)
	_, err := clientConn.Read(versionByte)
	if err != nil {
		fmt.Printf("[!] Error reading SOCKS version: %v\n", err)
		return
	}

	switch versionByte[0] {
	case 4:
		s.handleSOCKS4(clientConn, versionByte[0])
	case 5:
		s.handleSOCKS5(clientConn, versionByte[0])
	default:
		fmt.Printf("[!] Unsupported SOCKS version: %d\n", versionByte[0])
	}
}

// handleSOCKS4 handles SOCKS4 protocol
func (s *SSHOverWebSocket) handleSOCKS4(clientConn net.Conn, version byte) {
	// Read the rest of the SOCKS4 request
	header := make([]byte, 7) // cmd(1) + port(2) + ip(4)
	_, err := io.ReadFull(clientConn, header)
	if err != nil {
		fmt.Printf("[!] Error reading SOCKS4 header: %v\n", err)
		return
	}

	cmd := header[0]
	port := binary.BigEndian.Uint16(header[1:3])
	ip := header[3:7]

	for {
		b := make([]byte, 1)
		_, err := clientConn.Read(b)
		if err != nil || b[0] == 0 {
			break
		}
	}

	// Determine host
	host := fmt.Sprintf("%d.%d.%d.%d", ip[0], ip[1], ip[2], ip[3])

	// Check for SOCKS4a (domain name)
	if ip[0] == 0 && ip[1] == 0 && ip[2] == 0 && ip[3] != 0 {
		var domain []byte
		for {
			b := make([]byte, 1)
			_, err := clientConn.Read(b)
			if err != nil || b[0] == 0 {
				break
			}
			domain = append(domain, b[0])
		}
		host = string(domain)
	}

	if cmd != 1 { // Only CONNECT is supported
		// Send error response
		response := []byte{0, 0x5B, 0, 0, 0, 0, 0, 0}
		clientConn.Write(response)
		return
	}

	// Send success response
	response := []byte{0, 0x5A}
	response = append(response, header[1:3]...) // port
	response = append(response, header[3:7]...) // ip
	clientConn.Write(response)

	// Open SSH channel
	s.openSSHChannel(clientConn, host, int(port))
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

// OpenHTTPProxy starts an HTTP proxy server on the specified local port
func (s *SSHOverWebSocket) OpenHTTPProxy(localPort int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return fmt.Errorf("failed to start HTTP proxy: %v", err)
	}

	fmt.Printf("[*] HTTP proxy listening on 127.0.0.1:%d\n", localPort)

	go func() {
		defer listener.Close()
		for {
			clientConn, err := listener.Accept()
			if err != nil {
				// Check if this is because the listener was closed
				if netErr, ok := err.(net.Error); ok && !netErr.Temporary() {
					fmt.Printf("[*] HTTP proxy listener closed\n")
					return
				}
				fmt.Printf("[!] Error accepting connection: %v\n", err)
				// Brief pause before retrying
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Handle each client in a separate goroutine
			go s.handleHTTPClient(clientConn)
		}
	}()

	fmt.Println("[*] HTTP proxy started.")
	return nil
}

// handleHTTPClient handles an HTTP proxy client connection
func (s *SSHOverWebSocket) handleHTTPClient(clientConn net.Conn) {
	defer func() {
		clientConn.Close()
		if r := recover(); r != nil {
			fmt.Printf("[!] Panic in HTTP proxy handler: %v\n", r)
		}
	}()

	// Set initial timeout for HTTP request reading
	clientConn.SetReadDeadline(time.Now().Add(30 * time.Second))
	clientConn.SetWriteDeadline(time.Now().Add(30 * time.Second))

	// Read the HTTP request
	reader := bufio.NewReader(clientConn)
	req, err := http.ReadRequest(reader)
	if err != nil {
		fmt.Printf("[!] Error reading HTTP request: %v\n", err)
		s.sendHTTPError(clientConn, 400, "Bad Request")
		return
	}

	// Handle CONNECT method for HTTPS tunneling
	if req.Method == "CONNECT" {
		s.handleHTTPConnect(clientConn, req)
		return
	}

	// Handle regular HTTP requests (GET, POST, etc.)
	s.handleHTTPRequest(clientConn, req)
}

// handleHTTPConnect handles HTTPS tunneling via CONNECT method
func (s *SSHOverWebSocket) handleHTTPConnect(clientConn net.Conn, req *http.Request) {
	// Parse host and port from the request
	host, port, err := net.SplitHostPort(req.Host)
	if err != nil {
		// If no port specified, default to 443 for HTTPS
		host = req.Host
		port = "443"
	}

	// Convert port to integer
	var portInt int
	if port == "https" || port == "443" {
		portInt = 443
	} else if port == "http" || port == "80" {
		portInt = 80
	} else {
		_, err := fmt.Sscanf(port, "%d", &portInt)
		if err != nil {
			fmt.Printf("[!] Invalid port in CONNECT request: %s\n", port)
			s.sendHTTPError(clientConn, 400, "Bad Request")
			return
		}
	}

	fmt.Printf("[*] HTTP CONNECT request to %s:%d\n", host, portInt)

	// Send 200 Connection Established response
	response := "HTTP/1.1 200 Connection established\r\n\r\n"
	_, err = clientConn.Write([]byte(response))
	if err != nil {
		fmt.Printf("[!] Error sending CONNECT response: %v\n", err)
		return
	}

	fmt.Printf("[+] HTTP CONNECT tunnel established to %s:%d\n", host, portInt)

	// Open SSH channel and forward traffic
	s.openSSHChannel(clientConn, host, portInt)
}

// handleHTTPRequest handles regular HTTP requests (GET, POST, etc.)
func (s *SSHOverWebSocket) handleHTTPRequest(clientConn net.Conn, req *http.Request) {
	// Parse the target URL
	var targetHost string
	var targetPort int
	var targetPath string

	if req.URL.IsAbs() {
		// Full URL provided (typical for proxy requests)
		parsedURL, err := url.Parse(req.URL.String())
		if err != nil {
			fmt.Printf("[!] Error parsing URL: %v\n", err)
			s.sendHTTPError(clientConn, 400, "Bad Request")
			return
		}

		targetHost = parsedURL.Hostname()
		if parsedURL.Port() != "" {
			_, err := fmt.Sscanf(parsedURL.Port(), "%d", &targetPort)
			if err != nil {
				fmt.Printf("[!] Invalid port in URL: %s\n", parsedURL.Port())
				s.sendHTTPError(clientConn, 400, "Bad Request")
				return
			}
		} else {
			// Default ports
			if parsedURL.Scheme == "https" {
				targetPort = 443
			} else {
				targetPort = 80
			}
		}
		targetPath = parsedURL.RequestURI()
	} else {
		// Relative URL - use Host header
		if req.Host == "" {
			fmt.Printf("[!] No Host header in HTTP request\n")
			s.sendHTTPError(clientConn, 400, "Bad Request")
			return
		}

		host, port, err := net.SplitHostPort(req.Host)
		if err != nil {
			targetHost = req.Host
			targetPort = 80 // Default HTTP port
		} else {
			targetHost = host
			_, err := fmt.Sscanf(port, "%d", &targetPort)
			if err != nil {
				fmt.Printf("[!] Invalid port in Host header: %s\n", port)
				s.sendHTTPError(clientConn, 400, "Bad Request")
				return
			}
		}
		targetPath = req.URL.RequestURI()
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

	// Reconstruct and forward the HTTP request
	err = s.forwardHTTPRequest(sshConn, req, targetPath)
	if err != nil {
		fmt.Printf("[!] Error forwarding HTTP request: %v\n", err)
		s.sendHTTPError(clientConn, 502, "Bad Gateway")
		return
	}

	// Forward the response back to client
	s.forwardHTTPResponse(clientConn, sshConn)
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

	// Set reasonable timeouts - longer for established connections
	clientConn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	clientConn.SetWriteDeadline(time.Now().Add(5 * time.Minute))

	// Open SSH channel with retry logic
	var sshConn net.Conn
	var err error
	for retries := 0; retries < 3; retries++ {
		sshConn, err = s.sshClient.Dial("tcp", fmt.Sprintf("%s:%d", host, port))
		if err == nil {
			break
		}
		fmt.Printf("[!] SSH channel attempt %d failed: %v\n", retries+1, err)
		if retries < 2 {
			time.Sleep(time.Duration(retries+1) * time.Second)
		}
	}

	if err != nil {
		fmt.Printf("[!] Failed to open SSH channel after 3 attempts: %v\n", err)
		return
	}
	defer sshConn.Close()

	fmt.Printf("[+] SSH channel established to %s:%d\n", host, port)

	// Set longer timeouts for SSH connection
	sshConn.SetReadDeadline(time.Now().Add(5 * time.Minute))
	sshConn.SetWriteDeadline(time.Now().Add(5 * time.Minute))

	// Start bidirectional forwarding with better error handling
	var wg sync.WaitGroup
	wg.Add(2)

	// Channel for coordinating shutdown
	done := make(chan struct{})

	// Client to SSH
	go func() {
		defer wg.Done()
		defer func() {
			select {
			case <-done:
			default:
				close(done)
			}
		}()

		buffer := make([]byte, 32*1024) // 32KB buffer for better performance
		for {
			select {
			case <-done:
				return
			default:
			}

			// Set short read timeout to allow checking for done signal
			clientConn.SetReadDeadline(time.Now().Add(30 * time.Second))
			n, err := clientConn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout - continue to check for done signal
					continue
				}
				// Real error or EOF
				if err != io.EOF {
					fmt.Printf("[!] Error reading from client: %v\n", err)
				}
				return
			}

			if n > 0 {
				// Reset deadline for write
				sshConn.SetWriteDeadline(time.Now().Add(30 * time.Second))
				_, writeErr := sshConn.Write(buffer[:n])
				if writeErr != nil {
					if writeErr != io.EOF {
						fmt.Printf("[!] Error writing to SSH: %v\n", writeErr)
					}
					return
				}
			}
		}
	}()

	// SSH to client
	go func() {
		defer wg.Done()
		defer func() {
			select {
			case <-done:
			default:
				close(done)
			}
		}()

		buffer := make([]byte, 32*1024) // 32KB buffer for better performance
		for {
			select {
			case <-done:
				return
			default:
			}

			// Set short read timeout to allow checking for done signal
			sshConn.SetReadDeadline(time.Now().Add(30 * time.Second))
			n, err := sshConn.Read(buffer)
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					// Timeout - continue to check for done signal
					continue
				}
				// Real error or EOF
				if err != io.EOF {
					fmt.Printf("[!] Error reading from SSH: %v\n", err)
				}
				return
			}

			if n > 0 {
				// Reset deadline for write
				clientConn.SetWriteDeadline(time.Now().Add(30 * time.Second))
				_, writeErr := clientConn.Write(buffer[:n])
				if writeErr != nil {
					if writeErr != io.EOF {
						fmt.Printf("[!] Error writing to client: %v\n", writeErr)
					}
					return
				}
			}
		}
	}()

	wg.Wait()
	fmt.Printf("[*] SSH channel to %s:%d closed\n", host, port)
}

// ConnectViaWSAndStartSOCKS is a convenience function to start everything
func ConnectViaWSAndStartSOCKS(wsConn net.Conn, sshUser, sshPassword, sshPort string, localSOCKSPort int) (*SSHOverWebSocket, error) {
	connector := NewSSHOverWebSocket(wsConn, sshUser, sshPassword, sshPort)

	err := connector.StartSSHTransport()
	if err != nil {
		return nil, err
	}

	err = connector.OpenSOCKSProxy(localSOCKSPort)
	if err != nil {
		return nil, err
	}

	return connector, nil
}

// ConnectViaWSAndStartHTTP is a convenience function to start HTTP proxy
func ConnectViaWSAndStartHTTP(wsConn net.Conn, sshUser, sshPassword, sshPort string, localHTTPPort int) (*SSHOverWebSocket, error) {
	connector := NewSSHOverWebSocket(wsConn, sshUser, sshPassword, sshPort)

	err := connector.StartSSHTransport()
	if err != nil {
		return nil, err
	}

	err = connector.OpenHTTPProxy(localHTTPPort)
	if err != nil {
		return nil, err
	}

	return connector, nil
}
