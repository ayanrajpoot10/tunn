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
func (s *Server) StartProxy(proxyType string, localPort int, handler func(net.Conn)) error {
	listener, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		return fmt.Errorf("failed to start %s proxy: %v", proxyType, err)
	}

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
func (s *Server) HandleClientWithTimeout(clientConn net.Conn, clientType string, timeout time.Duration, handler func()) {
	defer func() {
		clientConn.Close()
		if r := recover(); r != nil {
			fmt.Printf("✗ Panic in %s handler: %v\n", clientType, r)
		}
	}()

	clientConn.SetReadDeadline(time.Now().Add(timeout))
	clientConn.SetWriteDeadline(time.Now().Add(timeout))

	handler()
}

// OpenSSHChannel opens an SSH channel and forwards data
func (s *Server) OpenSSHChannel(clientConn net.Conn, host string, port int) {
	fmt.Printf("→ Opening SSH channel to %s:%d\n", host, port)

	address := net.JoinHostPort(host, strconv.Itoa(port))
	sshConn, err := s.ssh.Dial("tcp", address)
	if err != nil {
		fmt.Printf("✗ Failed to open SSH channel: %v\n", err)
		return
	}
	defer sshConn.Close()

	fmt.Printf("✓ SSH channel established to %s:%d\n", host, port)

	// Forward data bidirectionally
	s.forwardData(clientConn, sshConn)
	fmt.Printf("→ SSH channel to %s:%d closed\n", host, port)
}

// forwardData forwards data bidirectionally between two connections
func (s *Server) forwardData(conn1, conn2 net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Forward conn1 -> conn2
	go func() {
		defer wg.Done()
		io.Copy(conn1, conn2)
	}()

	// Forward conn2 -> conn1
	go func() {
		defer wg.Done()
		io.Copy(conn2, conn1)
	}()

	wg.Wait()
}
