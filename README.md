# Tunn - Versatile SSH Tunneling Tool

Tunn is a powerful and flexible SSH tunneling tool written in Go that creates secure connections through various strategies including HTTP proxy tunneling, SNI fronting, and direct connections with domain spoofing. It establishes SSH tunnels over WebSocket connections and provides a local SOCKS proxy for routing traffic.

## Features

- **Multiple Tunnel Strategies**: Support for proxy, SNI fronting, and direct connection modes
- **WebSocket-based SSH Tunnels**: Establishes SSH connections over WebSocket for better bypass capabilities
- **SOCKS Proxy**: Built-in SOCKS5 proxy server for routing local traffic through the tunnel
- **Domain Spoofing**: Front domain support for Host header manipulation to bypass restrictions
- **Configurable Payloads**: Custom HTTP payload templates for different environments
- **Cross-platform**: Windows, Linux, and macOS support
- **Timeout Management**: Configurable connection timeouts and keepalive settings

## Installation

### Download Pre-built Binaries

Download the latest release from the releases page for your platform.

### Build from Source

Requires Go 1.21 or later:

```bash
# Clone the repository
git clone <repository-url>
cd Tunn

# Install dependencies
go mod download

# Build for current platform
make build

# Or build for all platforms
make build-all
```

## Usage

Tunn supports three main tunneling strategies:

### 1. Proxy Mode

Routes traffic through an HTTP proxy server first, then establishes a WebSocket tunnel to the target host.

```bash
# Basic proxy mode
tunn proxy --proxy-host proxy.example.com --target-host ssh-server.com --ssh-username user --ssh-password pass

# With custom proxy port and front domain
tunn proxy \
  --proxy-host proxy.example.com \
  --proxy-port 8080 \
  --target-host ssh-server.com \
  --front-domain allowed-domain.com \
  --ssh-username user \
  --ssh-password pass
```

**Required flags for proxy mode:**
- `--proxy-host`: Proxy server to connect to first
- `--target-host`: Target SSH server to reach through proxy
- `--ssh-username`: SSH username for target server
- `--ssh-password`: SSH password for target server

### 2. SNI Fronting Mode

Uses SNI (Server Name Indication) fronting to establish connections through a proxy with forged SNI headers.

```bash
# Basic SNI fronting
tunn sni --front-domain google.com --proxy-host proxy.example.com --ssh-username user --ssh-password pass

# With custom configuration
tunn sni \
  --front-domain cloudflare.com \
  --proxy-host proxy.example.com \
  --proxy-port 443 \
  --target-host ssh-server.com \
  --ssh-username user \
  --ssh-password pass
```

**Required flags for SNI mode:**
- `--front-domain`: Domain for SNI fronting
- `--proxy-host`: Proxy server for the connection
- `--ssh-username`: SSH username
- `--ssh-password`: SSH password

### 3. Direct Mode

Establishes a direct connection to the target host with optional Host header spoofing.

```bash
# Basic direct connection
tunn direct --target-host ssh-server.com --ssh-username user --ssh-password pass

# With front domain spoofing
tunn direct \
  --front-domain google.com \
  --target-host ssh-server.com \
  --target-port 443 \
  --ssh-username user \
  --ssh-password pass
```

**Required flags for direct mode:**
- `--target-host`: Target SSH server
- `--ssh-username`: SSH username
- `--ssh-password`: SSH password

## Common Options

All modes support these additional options:

- `--local-port` / `-l`: Local SOCKS proxy port (default: 1080)
- `--ssh-port`: SSH port on target server (default: 22)
- `--timeout` / `-t`: Connection timeout in seconds (0 = no timeout)
- `--payload`: Custom HTTP payload template
- `--verbose` / `-v`: Enable verbose output

## Configuration

### Default Payload Template

```
GET / HTTP/1.1[crlf]Host: [host][crlf]Upgrade: websocket[crlf][crlf]
```

Placeholders:
- `[host]`: Replaced with target host or front domain
- `[crlf]`: Replaced with `\r\n`

### Custom Payloads

You can specify custom HTTP payloads for different environments:

```bash
tunn proxy \
  --proxy-host proxy.example.com \
  --target-host ssh-server.com \
  --payload "CONNECT [host] HTTP/1.1[crlf]Host: [host][crlf][crlf]" \
  --ssh-username user \
  --ssh-password pass
```

## Examples

### Example 1: Corporate Proxy Bypass

```bash
# Connect through corporate proxy to external SSH server
tunn proxy \
  --proxy-host corporate-proxy.company.com \
  --proxy-port 8080 \
  --target-host remote-server.com \
  --front-domain microsoft.com \
  --ssh-username myuser \
  --ssh-password mypassword \
  --local-port 1080
```

### Example 2: SNI Fronting for CDN Bypass

```bash
# Use SNI fronting to bypass CDN restrictions
tunn sni \
  --front-domain cloudflare.com \
  --proxy-host edge-server.com \
  --target-host hidden-server.com \
  --ssh-username user \
  --ssh-password pass
```

### Example 3: Direct Connection with Domain Spoofing

```bash
# Direct connection with Host header spoofing
tunn direct \
  --target-host my-server.com \
  --front-domain google.com \
  --ssh-username admin \
  --ssh-password secret123 \
  --timeout 60
```

## How It Works

1. **Connection Establishment**: Tunn connects to the specified server (proxy, SNI, or direct)
2. **WebSocket Upgrade**: Sends an HTTP request with WebSocket upgrade headers
3. **SSH Tunnel**: Establishes an SSH connection over the WebSocket connection
4. **SOCKS Proxy**: Starts a local SOCKS5 proxy that forwards traffic through the SSH tunnel

### Architecture

```
[Local Application] → [SOCKS Proxy :1080] → [SSH over WebSocket] → [Target Server]
                                                    ↑
                                            [Proxy/SNI/Direct]
```

## Using the SOCKS Proxy

Once Tunn is running, configure your applications to use the SOCKS proxy:

- **Proxy Type**: SOCKS5
- **Host**: 127.0.0.1
- **Port**: 1080 (or your specified port)

### Browser Configuration

**Firefox**:
1. Go to Settings → Network Settings
2. Select "Manual proxy configuration"
3. Set SOCKS Host to `127.0.0.1` and Port to `1080`
4. Select "SOCKS v5"

**Chrome** (command line):
```bash
chrome --proxy-server="socks5://127.0.0.1:1080"
```

### Command Line Tools

```bash
# curl with SOCKS proxy
curl --socks5 127.0.0.1:1080 https://example.com

# ssh through the tunnel
ssh -o ProxyCommand="nc -X 5 -x 127.0.0.1:1080 %h %p" user@target-server.com
```

## Development

### Project Structure

```
tunn/
├── main.go              # Application entry point
├── go.mod               # Go module dependencies
├── Makefile            # Build automation
├── cmd/                # CLI commands
│   ├── root.go         # Root command and global flags
│   ├── proxy.go        # Proxy mode implementation
│   ├── sni.go          # SNI fronting mode implementation
│   └── direct.go       # Direct mode implementation
└── internal/
    └── tunnel/         # Core tunneling logic
        ├── config.go   # Configuration structures
        ├── ssh.go      # SSH over WebSocket implementation
        └── websocket.go # WebSocket connection handling
```

### Building

```bash
# Install dependencies
make deps

# Build for current platform
make build

# Build for all platforms
make build-all

# Clean build artifacts
make clean

# Run with default configuration
make run
```

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## Dependencies

- **[Cobra](https://github.com/spf13/cobra)**: CLI framework
- **[golang.org/x/crypto](https://golang.org/x/crypto)**: SSH client implementation

## Security Considerations

- **Password Security**: Avoid using passwords in command line arguments in production. Consider using SSH keys or environment variables.
- **Traffic Encryption**: All traffic is encrypted through SSH, but the initial WebSocket connection may be visible to network monitors.
- **Proxy Logs**: Be aware that proxy servers may log connection attempts.

## Troubleshooting

### Common Issues

**Connection Timeout**:
- Increase timeout with `--timeout` flag
- Check network connectivity to proxy/target hosts

**SSH Authentication Failed**:
- Verify SSH credentials
- Check if SSH service is running on target port
- Ensure target host allows password authentication

**WebSocket Upgrade Failed**:
- Try different payload templates
- Check if proxy supports WebSocket upgrades
- Verify front domain is not blocked

**SOCKS Proxy Not Working**:
- Ensure Tunn is running and showing "SOCKS proxy up" message
- Check local firewall settings
- Verify application SOCKS configuration

### Debug Mode

Enable verbose output for debugging:

```bash
tunn --verbose proxy --proxy-host example.com --target-host target.com --ssh-username user --ssh-password pass
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Version

Current version: 1.0.0

For the latest updates and releases, visit the project repository.
