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

// Client represents an SSH client that can operate over any net.Conn
type Client interface {
	Dial(network, address string) (net.Conn, error)
	Close() error
}

// OverWebSocket wraps an SSH connection over a WebSocket-upgraded connection
type OverWebSocket struct {
	conn      net.Conn
	sshClient *ssh.Client
	username  string
	password  string
}

// NewOverWebSocket creates a new SSH over WebSocket connection
func NewOverWebSocket(conn net.Conn, username, password string) *OverWebSocket {
	return &OverWebSocket{
		conn:     conn,
		username: username,
		password: password,
	}
}

// stripHTMLTags removes HTML tags from a string and returns plain text.
func stripHTMLTags(htmlStr string) string {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return htmlStr // fallback to original if parsing fails
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

// StartTransport initializes the SSH client over the WebSocket connection
func (s *OverWebSocket) StartTransport() error {
	fmt.Println("→ Starting SSH transport over WebSocket connection...")

	// Set keepalive on the underlying WebSocket connection if it's TCP
	if tcpConn, ok := s.conn.(*net.TCPConn); ok {
		fmt.Println("→ Setting keepalive on WebSocket connection...")
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
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // WARNING: This is insecure for production
		Timeout:         handshakeTimeout,
		BannerCallback: func(message string) error {
			plain := stripHTMLTags(message)
			fmt.Fprintln(stderrOrStdout(), plain)
			return nil
		},
	}

	fmt.Printf("→ Attempting SSH connection with user: %s\n", s.username)

	// Create SSH client using the WebSocket connection
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

// Dial opens a new connection to the given address through the SSH tunnel
func (s *OverWebSocket) Dial(network, address string) (net.Conn, error) {
	return s.sshClient.Dial(network, address)
}

// Close closes the SSH connection
func (s *OverWebSocket) Close() error {
	if s.sshClient != nil {
		return s.sshClient.Close()
	}
	return nil
}

// stderrOrStdout returns stderr if available, otherwise stdout.
func stderrOrStdout() *os.File {
	// fallback to stdout if stderr is not available
	return os.Stderr
}
