package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tunn/pkg/utils"
)

// HTTP handles HTTP proxy functionality
type HTTP struct {
	server *Server
}

// NewHTTP creates a new HTTP proxy instance
func NewHTTP(ssh SSHClient) *HTTP {
	return &HTTP{
		server: NewServer(ssh),
	}
}

// Start starts the HTTP proxy server on the specified local port
func (h *HTTP) Start(localPort int) error {
	return h.server.StartProxy("HTTP", localPort, h.handleClient)
}

// handleClient handles an HTTP proxy client connection
func (h *HTTP) handleClient(clientConn net.Conn) {
	h.server.HandleClientWithTimeout(clientConn, "HTTP", 30*time.Second, func() {
		reader := bufio.NewReader(clientConn)
		req, err := http.ReadRequest(reader)
		if err != nil {
			fmt.Printf("✗ Error reading HTTP request: %v\n", err)
			h.sendError(clientConn, 400, "Bad Request")
			return
		}

		if req.Method == "CONNECT" {
			h.handleConnect(clientConn, req)
		} else {
			h.handleRequest(clientConn, req)
		}
	})
}

// handleConnect handles HTTPS tunneling via CONNECT method
func (h *HTTP) handleConnect(clientConn net.Conn, req *http.Request) {
	host, portInt, err := utils.ParseHostPort(req.Host, 443)
	if err != nil {
		fmt.Printf("✗ Invalid host in CONNECT request: %v\n", err)
		h.sendError(clientConn, 400, "Bad Request")
		return
	}

	fmt.Printf("→ HTTP CONNECT request to %s:%d\n", host, portInt)

	// Send success response
	response := "HTTP/1.1 200 Connection established\r\n\r\n"
	if _, err := clientConn.Write([]byte(response)); err != nil {
		fmt.Printf("✗ Error sending CONNECT response: %v\n", err)
		return
	}

	fmt.Printf("✓ HTTP CONNECT tunnel established to %s:%d\n", host, portInt)
	h.server.OpenSSHChannel(clientConn, host, portInt)
}

// handleRequest handles regular HTTP requests (GET, POST, etc.)
func (h *HTTP) handleRequest(clientConn net.Conn, req *http.Request) {
	targetHost, targetPort, targetPath, err := h.parseTarget(req)
	if err != nil {
		fmt.Printf("✗ Error parsing HTTP target: %v\n", err)
		h.sendError(clientConn, 400, "Bad Request")
		return
	}

	fmt.Printf("→ HTTP %s request to %s:%d%s\n", req.Method, targetHost, targetPort, targetPath)

	// Open SSH channel to target
	address := net.JoinHostPort(targetHost, strconv.Itoa(targetPort))
	sshConn, err := h.server.ssh.Dial("tcp", address)
	if err != nil {
		fmt.Printf("✗ Failed to open SSH channel for HTTP request: %v\n", err)
		h.sendError(clientConn, 502, "Bad Gateway")
		return
	}
	defer sshConn.Close()

	// Forward the HTTP request and response
	if err := h.forwardRequest(sshConn, req, targetPath); err != nil {
		fmt.Printf("✗ Error forwarding HTTP request: %v\n", err)
		h.sendError(clientConn, 502, "Bad Gateway")
		return
	}

	h.forwardResponse(clientConn, sshConn)
}

// parseTarget parses the target from HTTP request
func (h *HTTP) parseTarget(req *http.Request) (host string, port int, path string, err error) {
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

		host, port, err = utils.ParseHostPort(req.Host, 80)
		if err != nil {
			return "", 0, "", fmt.Errorf("invalid Host header: %v", err)
		}
		path = req.URL.RequestURI()
	}

	return host, port, path, nil
}

// forwardRequest reconstructs and sends the HTTP request through SSH tunnel
func (h *HTTP) forwardRequest(sshConn net.Conn, req *http.Request, targetPath string) error {
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

// forwardResponse forwards the HTTP response from SSH tunnel back to client
func (h *HTTP) forwardResponse(clientConn net.Conn, sshConn net.Conn) {
	// Simply forward all data from SSH connection back to client
	_, err := io.Copy(clientConn, sshConn)
	if err != nil && err != io.EOF {
		fmt.Printf("✗ Error forwarding HTTP response: %v\n", err)
	}
}

// sendError sends an HTTP error response
func (h *HTTP) sendError(clientConn net.Conn, statusCode int, statusText string) {
	response := fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", statusCode, statusText)
	clientConn.Write([]byte(response))
}
