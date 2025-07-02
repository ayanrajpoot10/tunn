package tunnel

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
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
