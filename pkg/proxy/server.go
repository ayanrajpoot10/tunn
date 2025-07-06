package proxy

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

// SSHClient interface defines what we need from an SSH client
type SSHClient interface {
	Dial(network, address string) (net.Conn, error)
}

// Server manages generic proxy functionality
type Server struct {
	ssh SSHClient
}

// NewServer creates a new proxy server instance
func NewServer(ssh SSHClient) *Server {
	return &Server{ssh: ssh}
}

// StartProxy starts a generic proxy server
func (p *Server) StartProxy(proxyType string, localPort int, handler func(net.Conn)) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return fmt.Errorf("failed to start %s proxy: %v", proxyType, err)
	}

	fmt.Printf("→ %s proxy listening on 127.0.0.1:%d\n", proxyType, localPort)

	go func() {
		defer listener.Close()
		for {
			clientConn, err := listener.Accept()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && !netErr.Temporary() {
					fmt.Printf("→ %s proxy listener closed\n", proxyType)
					return
				}
				fmt.Printf("✗ Error accepting connection: %v\n", err)
				time.Sleep(100 * time.Millisecond)
				continue
			}

			go handler(clientConn)
		}
	}()

	fmt.Printf("✓ %s proxy started.\n", proxyType)
	return nil
}

// HandleClientWithTimeout provides common client handling with timeout and panic recovery
func (p *Server) HandleClientWithTimeout(clientConn net.Conn, clientType string, timeout time.Duration, handler func()) {
	defer func() {
		clientConn.Close()
		if r := recover(); r != nil {
			fmt.Printf("✗ Panic in %s handler: %v\n", clientType, r)
		}
	}()

	// Set initial timeout
	clientConn.SetReadDeadline(time.Now().Add(timeout))
	clientConn.SetWriteDeadline(time.Now().Add(timeout))

	handler()
}

// OpenSSHChannel opens an SSH channel and forwards data
func (p *Server) OpenSSHChannel(clientConn net.Conn, host string, port int) {
	fmt.Printf("→ Opening SSH channel to %s:%d\n", host, port)

	// Open SSH channel with retry logic
	sshConn, err := p.dialWithRetry(host, port)
	if err != nil {
		fmt.Printf("✗ Failed to open SSH channel: %v\n", err)
		return
	}
	defer sshConn.Close()

	fmt.Printf("✓ SSH channel established to %s:%d\n", host, port)

	// Forward data bidirectionally
	p.forwardData(clientConn, sshConn)
	fmt.Printf("→ SSH channel to %s:%d closed\n", host, port)
}

// dialWithRetry attempts to dial with retry logic
func (p *Server) dialWithRetry(host string, port int) (net.Conn, error) {
	var conn net.Conn
	var err error

	// Format the address properly for IPv6
	address := net.JoinHostPort(host, strconv.Itoa(port))

	for retries := 0; retries < 3; retries++ {
		conn, err = p.ssh.Dial("tcp", address)
		if err == nil {
			return conn, nil
		}

		fmt.Printf("✗ SSH channel attempt %d failed: %v\n", retries+1, err)
		if retries < 2 {
			time.Sleep(time.Duration(retries+1) * time.Second)
		}
	}

	return nil, fmt.Errorf("failed after 3 attempts: %w", err)
}

// forwardData forwards data bidirectionally between two connections
func (p *Server) forwardData(conn1, conn2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Forward conn1 -> conn2
	go func() {
		defer wg.Done()
		p.copyData(conn1, conn2, "client", "SSH")
	}()

	// Forward conn2 -> conn1
	go func() {
		defer wg.Done()
		p.copyData(conn2, conn1, "SSH", "client")
	}()

	wg.Wait()
}

// copyData copies data from src to dst with proper error handling
func (p *Server) copyData(src, dst net.Conn, srcName, dstName string) {
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
				fmt.Printf("✗ Error reading from %s: %v\n", srcName, err)
			}
			return
		}

		if n > 0 {
			// Set write timeout
			dst.SetWriteDeadline(time.Now().Add(30 * time.Second))
			_, err := dst.Write(buffer[:n])
			if err != nil {
				if err != io.EOF {
					fmt.Printf("✗ Error writing to %s: %v\n", dstName, err)
				}
				return
			}
		}
	}
}
