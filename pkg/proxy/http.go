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

// HTTP implements an HTTP proxy server that forwards connections through SSH tunnels.
//
// This implementation provides comprehensive HTTP proxy support including:
//   - HTTP CONNECT method for HTTPS tunneling
//   - Regular HTTP requests (GET, POST, etc.) with request/response forwarding
//   - Automatic host and port parsing from URLs and Host headers
//   - Proper error response generation for client and server errors
//
// The HTTP proxy handles both transparent HTTP requests and HTTPS tunneling
// via the CONNECT method, making it suitable for web browser proxy configuration.
type HTTP struct {
	server *Server // Embedded server for common proxy functionality
}

// NewHTTP creates a new HTTP proxy instance with the specified SSH client.
//
// The HTTP proxy will use the provided SSH client to establish tunneled
// connections for incoming HTTP requests and CONNECT tunnels.
//
// Parameters:
//   - ssh: An initialized SSH client for tunnel connections
//
// Returns:
//   - *HTTP: A new HTTP proxy server instance
func NewHTTP(ssh SSHClient) *HTTP {
	return &HTTP{
		server: NewServer(ssh),
	}
}

// Start starts the HTTP proxy server on the specified local port.
//
// This method begins listening for HTTP client connections on the local
// interface at the specified port. Each client connection is handled according
// to HTTP proxy standards, supporting both regular HTTP requests and CONNECT
// tunneling for HTTPS traffic.
//
// The server will continue running until the application is terminated or
// an unrecoverable error occurs.
//
// Parameters:
//   - localPort: Local port number to listen for HTTP proxy connections
//
// Returns:
//   - error: An error if the server fails to start listening
func (h *HTTP) Start(localPort int) error {
	return h.server.StartProxy("HTTP", localPort, h.handleClient)
}

// handleClient processes a single HTTP proxy client connection.
//
// This method manages the complete HTTP client session including timeout handling,
// panic recovery, and request type detection. It distinguishes between regular
// HTTP requests and HTTPS CONNECT tunnels, routing them to appropriate handlers.
//
// The method uses a 30-second timeout for initial request reading to prevent
// slow or malicious clients from consuming server resources.
//
// Supported HTTP methods:
//   - CONNECT: Creates an HTTPS tunnel through the SSH connection
//   - All other methods (GET, POST, etc.): Forwarded as regular HTTP requests
//
// Parameters:
//   - clientConn: The incoming HTTP client connection to handle
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

// handleConnect processes HTTP CONNECT requests for HTTPS tunneling.
//
// This method implements the HTTP CONNECT method as defined in RFC 7231,
// establishing a tunnel between the client and the target server through
// the SSH connection. It's primarily used for HTTPS traffic but can tunnel
// any TCP-based protocol.
//
// The process:
//  1. Parses the target host and port from the CONNECT request
//  2. Sends "200 Connection established" response to the client
//  3. Establishes SSH tunnel to the target destination
//  4. Begins transparent data forwarding in both directions
//
// Parameters:
//   - clientConn: The HTTP client connection requesting the tunnel
//   - req: The parsed HTTP CONNECT request containing target information
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

// handleRequest processes regular HTTP requests (GET, POST, etc.) through the proxy.
//
// This method handles standard HTTP requests by parsing the target destination,
// establishing an SSH tunnel connection, forwarding the reconstructed request,
// and relaying the response back to the client.
//
// The process:
//  1. Extracts target host, port, and path from the request URL or Host header
//  2. Opens an SSH channel to the target server
//  3. Reconstructs and forwards the HTTP request through the tunnel
//  4. Streams the response back to the original client
//
// This method supports both absolute URLs (typical in proxy requests) and
// relative URLs with Host headers (less common but still valid).
//
// Parameters:
//   - clientConn: The HTTP client connection making the request
//   - req: The parsed HTTP request to forward through the tunnel
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

// parseTarget extracts the target host, port, and path from an HTTP request.
//
// This method handles both absolute URLs (common in proxy requests) and relative
// URLs with Host headers. It properly determines default ports based on the
// URL scheme and validates the extracted information.
//
// URL processing logic:
//   - Absolute URLs: Extract all information from the URL
//   - Relative URLs: Use Host header for host/port, URL path for the path
//   - Default ports: 80 for HTTP, 443 for HTTPS
//
// Parameters:
//   - req: The HTTP request to parse target information from
//
// Returns:
//   - host: Target server hostname or IP address
//   - port: Target server port number
//   - path: Request path to forward to the target server
//   - error: An error if the target information cannot be determined
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

// forwardRequest reconstructs and sends the HTTP request through the SSH tunnel.
//
// This method rebuilds the original HTTP request with the correct path and
// forwards it through the SSH connection to the target server. It filters out
// proxy-specific headers that shouldn't be sent to the origin server.
//
// The reconstruction process:
//  1. Builds the HTTP request line with method, path, and protocol version
//  2. Copies headers (excluding proxy-specific ones like "Proxy-Connection")
//  3. Adds request body if present
//
// Headers filtered out:
//   - "Proxy-Connection": Proxy-specific header not relevant to origin servers
//
// Parameters:
//   - sshConn: The SSH tunnel connection to the target server
//   - req: The original HTTP request to reconstruct and forward
//   - targetPath: The path to use in the reconstructed request
//
// Returns:
//   - error: An error if request forwarding fails
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

// forwardResponse streams the HTTP response from the SSH tunnel back to the client.
//
// This method performs transparent forwarding of the complete HTTP response
// including headers and body from the target server through the SSH tunnel
// back to the original client. It continues until the connection is closed
// or an error occurs.
//
// The forwarding is done using io.Copy for optimal performance with large
// responses and streaming data.
//
// Parameters:
//   - clientConn: The original client connection to send the response to
//   - sshConn: The SSH tunnel connection receiving the response from target
func (h *HTTP) forwardResponse(clientConn net.Conn, sshConn net.Conn) {
	// Simply forward all data from SSH connection back to client
	_, err := io.Copy(clientConn, sshConn)
	if err != nil && err != io.EOF {
		fmt.Printf("✗ Error forwarding HTTP response: %v\n", err)
	}
}

// sendError sends an HTTP error response to the client.
//
// This method generates and sends a properly formatted HTTP error response
// with the specified status code and message. The response includes minimal
// headers and immediately closes the connection.
//
// The error response format follows HTTP/1.1 standards with:
//   - Proper status line with code and reason phrase
//   - Content-Length: 0 (no response body)
//   - Connection: close (to terminate the connection)
//
// Parameters:
//   - clientConn: The client connection to send the error response to
//   - statusCode: HTTP status code (e.g., 400, 502)
//   - statusText: HTTP reason phrase (e.g., "Bad Request", "Bad Gateway")
func (h *HTTP) sendError(clientConn net.Conn, statusCode int, statusText string) {
	response := fmt.Sprintf("HTTP/1.1 %d %s\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", statusCode, statusText)
	clientConn.Write([]byte(response))
}
