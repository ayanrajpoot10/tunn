// Package ssh provides SSH client functionality for the Tunn tunneling tool.
//
// This package implements SSH client operations over arbitrary network connections,
// enabling SSH tunneling through WebSocket connections and other transport layers.
// It handles SSH authentication, connection management, and channel creation for
// tunneled connections.
//
// The package supports:
//   - Password authentication
//   - SSH over custom network connections (including WebSocket)
//   - TCP keepalive for connection stability
//   - Banner message handling and HTML stripping
//   - Connection timeout management
//
// The SSH client can operate over any net.Conn implementation, making it suitable
// for tunneling SSH through WebSocket upgrades and proxy connections.
package ssh

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/html"
)

// Client defines the interface for SSH client operations required by tunnel components.
//
// This interface abstracts SSH client functionality to allow different implementations
// and facilitate testing. It provides the essential operations needed for tunneling:
// connection establishment and resource cleanup.
type Client interface {
	// Dial establishes a new connection through the SSH tunnel to the specified address.
	// The network parameter is typically "tcp" and address should be in "host:port" format.
	Dial(network, address string) (net.Conn, error)

	// Close closes the SSH client connection and releases associated resources.
	Close() error
}

// SSHClient provides SSH client functionality over any network connection.
//
// This implementation wraps an SSH connection that can operate over various
// transport layers including direct TCP, TLS, and WebSocket connections.
// It handles SSH authentication, keepalive, and connection management.
type SSHClient struct {
	conn      net.Conn    // The underlying network connection
	sshClient *ssh.Client // The SSH client instance
	username  string      // SSH username for authentication
	password  string      // SSH password for authentication
}

// NewSSHClient creates a new SSH client instance over the provided network connection.
//
// The SSH client will use the provided connection as the transport layer for SSH
// protocol operations. This allows SSH to operate over various connection types
// including WebSocket-upgraded connections.
//
// Parameters:
//   - conn: Network connection to use for SSH transport
//   - username: SSH username for authentication
//   - password: SSH password for authentication
//
// Returns:
//   - *SSHClient: A new SSH client instance ready for transport initialization
func NewSSHClient(conn net.Conn, username, password string) *SSHClient {
	return &SSHClient{
		conn:     conn,
		username: username,
		password: password,
	}
}

// stripHTMLTags removes HTML tags from a string and returns plain text.
//
// This function is used to clean SSH banner messages that may contain HTML
// content, making them more readable when displayed to users. It parses the
// HTML and extracts only the text content, removing all markup.
//
// The function is particularly useful for processing SSH server banners that
// may contain HTML formatting, ensuring clean console output.
//
// Parameters:
//   - htmlStr: String potentially containing HTML markup
//
// Returns:
//   - string: Plain text with HTML tags removed
func stripHTMLTags(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return htmlStr
	}
	var b strings.Builder
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return b.String()
}

// StartTransport initializes the SSH transport layer over the established connection.
//
// This method performs the complete SSH handshake including version negotiation,
// key exchange, and user authentication. It configures connection parameters for
// optimal performance and reliability including TCP keepalive and timeouts.
//
// The method performs several important operations:
//  1. Configures TCP keepalive if the underlying connection supports it
//  2. Sets handshake timeout to prevent hanging connections
//  3. Configures SSH client with password authentication and security settings
//  4. Handles server banners with HTML tag stripping
//  5. Establishes the SSH client connection with proper error handling
//
// Security considerations:
//   - Uses InsecureIgnoreHostKey for host key verification (suitable for tunneling)
//   - Implements connection timeouts to prevent resource exhaustion
//   - Handles authentication failures with descriptive error messages
//
// Returns:
//   - error: An error if SSH transport initialization fails
func (s *SSHClient) StartTransport() error {
	fmt.Println("→ Starting SSH transport over connection...")

	// Set keepalive on the underlying connection if it's TCP
	if tcpConn, ok := s.conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(30 * time.Second)
	}

	// Set a deadline for the SSH handshake to avoid hanging
	handshakeTimeout := 15 * time.Second
	s.conn.SetDeadline(time.Now().Add(handshakeTimeout))

	config := &ssh.ClientConfig{
		User: s.username,
		Auth: []ssh.AuthMethod{
			ssh.Password(s.password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         handshakeTimeout,
		BannerCallback: func(message string) error {
			plain := stripHTMLTags(message)
			fmt.Fprintln(os.Stderr, plain)
			return nil
		},
	}

	fmt.Printf("→ Attempting SSH connection with user: %s\n", s.username)

	// Create SSH client using the connection
	sshConn, chans, reqs, err := ssh.NewClientConn(s.conn, "tcp", config)
	if err != nil {
		if nErr, ok := err.(net.Error); ok && nErr.Timeout() {
			return fmt.Errorf("SSH handshake timed out after %v", handshakeTimeout)
		}
		return fmt.Errorf("failed to create SSH connection: %v", err)
	}

	// Clear deadline after handshake
	s.conn.SetDeadline(time.Time{})

	s.sshClient = ssh.NewClient(sshConn, chans, reqs)
	fmt.Println("✓ SSH transport established and authenticated.")
	return nil
}

// Dial establishes a new connection through the SSH tunnel to the specified destination.
//
// This method creates a new SSH channel to the target address, enabling tunneled
// connections to remote services. It's the primary method used by proxy servers
// to establish connections through the SSH tunnel.
//
// The method leverages the SSH client's built-in channel creation and port
// forwarding capabilities to create direct connections to remote addresses
// as if connecting from the SSH server's location.
//
// Parameters:
//   - network: Network type, typically "tcp"
//   - address: Target address in "host:port" format
//
// Returns:
//   - net.Conn: A connection to the target address through the SSH tunnel
//   - error: An error if channel creation fails
//
// Example:
//
//	conn, err := sshClient.Dial("tcp", "example.com:80")
//	if err != nil {
//	    return fmt.Errorf("tunnel connection failed: %w", err)
//	}
//	defer conn.Close()
func (s *SSHClient) Dial(network, address string) (net.Conn, error) {
	return s.sshClient.Dial(network, address)
}

// Close closes the SSH client connection and releases all associated resources.
//
// This method properly terminates the SSH client connection, ensuring all
// channels and resources are cleaned up. It should be called when the tunnel
// is no longer needed to prevent resource leaks.
//
// Returns:
//   - error: An error if connection closing fails, nil if successful or no connection exists
func (s *SSHClient) Close() error {
	if s.sshClient != nil {
		return s.sshClient.Close()
	}
	return nil
}
