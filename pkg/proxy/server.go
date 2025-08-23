// Package proxy provides local proxy server implementations for the Tunn SSH tunneling tool.
//
// This package implements both SOCKS5 and HTTP proxy servers that forward client
// connections through established SSH tunnels. The proxy servers act as local
// endpoints that applications can connect to, with all traffic being securely
// tunneled through SSH to the remote destination.
//
// Supported proxy protocols:
//   - SOCKS5: Full SOCKS5 protocol implementation with support for IPv4, IPv6, and domain names
//   - HTTP: HTTP proxy with support for both HTTP requests and HTTPS CONNECT tunneling
//
// All proxy implementations share common server functionality through the Server
// type, which provides connection handling, timeout management, and SSH channel
// establishment.
package proxy

import (
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

// SSHClient defines the interface for SSH client operations required by proxy servers.
//
// This interface abstracts the SSH client functionality needed for establishing
// tunneled connections, allowing different SSH client implementations to be used
// with the proxy servers.
type SSHClient interface {
	// Dial establishes a new connection through the SSH tunnel to the specified address.
	// The network parameter is typically "tcp" and address should be in "host:port" format.
	Dial(network, address string) (net.Conn, error)
}

// Server provides common functionality for all proxy server implementations.
//
// This type manages the core proxy server operations including listener management,
// connection handling with timeouts, panic recovery, and SSH channel establishment.
// It serves as the foundation for both SOCKS5 and HTTP proxy servers.
type Server struct {
	ssh SSHClient // SSH client for establishing tunneled connections
}

// NewServer creates a new proxy server instance with the specified SSH client.
//
// The server will use the provided SSH client to establish tunneled connections
// for incoming proxy requests. The SSH client must be properly initialized and
// ready to handle Dial requests.
//
// Parameters:
//   - ssh: An initialized SSH client for tunnel connections
//
// Returns:
//   - *Server: A new server instance ready for proxy operations
func NewServer(ssh SSHClient) *Server {
	return &Server{ssh: ssh}
}

// StartProxy starts a generic proxy server with the specified handler function.
//
// This method creates a TCP listener on the local interface and starts accepting
// client connections. Each client connection is handled in a separate goroutine
// using the provided handler function, enabling concurrent connection processing.
//
// The server binds to 127.0.0.1 (localhost) for security, preventing external
// access to the proxy server. Connection errors are logged but don't terminate
// the server unless they are permanent network errors.
//
// Parameters:
//   - proxyType: Description of the proxy type for logging (e.g., "SOCKS5", "HTTP")
//   - localPort: Local port number to listen on
//   - handler: Function to handle each client connection
//
// Returns:
//   - error: An error if the server fails to start listening
//
// The method returns immediately after starting the server goroutine, allowing
// the caller to continue with other operations.
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

// HandleClientWithTimeout provides standardized client connection handling with timeout and panic recovery.
//
// This method wraps client connection handling with essential safety and timeout features:
//   - Automatic connection cleanup on function exit
//   - Panic recovery to prevent server crashes from client handler errors
//   - Read and write timeout configuration for responsive operation
//
// The timeout values help ensure that slow or malicious clients cannot tie up
// server resources indefinitely. Panic recovery ensures that errors in protocol
// handling don't crash the entire proxy server.
//
// Parameters:
//   - clientConn: The client connection to handle
//   - clientType: Description for logging (e.g., "SOCKS5", "HTTP")
//   - timeout: Maximum time allowed for initial protocol negotiation
//   - handler: The actual protocol handling function to execute
//
// The handler function should perform the specific protocol operations (SOCKS5
// handshake, HTTP request processing, etc.) within the timeout period.
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

// OpenSSHChannel establishes an SSH tunnel connection to the specified destination.
//
// This method creates a new SSH channel through the tunnel to the target host and port,
// then manages bidirectional data forwarding between the client connection and the
// SSH channel until either side closes the connection.
//
// The method handles the complete lifecycle of the tunneled connection:
//  1. Establishes SSH channel to the target destination
//  2. Sets up bidirectional data forwarding
//  3. Manages connection cleanup when forwarding completes
//
// Data forwarding is performed concurrently in both directions using separate
// goroutines to ensure optimal performance and responsiveness.
//
// Parameters:
//   - clientConn: The local client connection to forward data from/to
//   - host: Target destination hostname or IP address
//   - port: Target destination port number
//
// This method blocks until the connection is closed by either the client or
// the remote server, making it suitable for use in connection handler goroutines.
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

// forwardData manages bidirectional data forwarding between two network connections.
//
// This method creates two goroutines to handle concurrent data forwarding in both
// directions between the connections. It uses a WaitGroup to ensure both forwarding
// operations complete before returning.
//
// The forwarding continues until one of the connections is closed or encounters
// an error, at which point both forwarding goroutines will terminate naturally
// due to the io.Copy operations completing.
//
// Parameters:
//   - conn1: First network connection
//   - conn2: Second network connection
//
// Data is copied from conn1 to conn2 and from conn2 to conn1 simultaneously,
// enabling full-duplex communication between the endpoints.
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
