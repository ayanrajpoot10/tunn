# Tunn - Versatile SSH Tunneling Tool

Tunn is a powerful and flexible SSH tunneling tool written in Go that creates secure connections through various strategies including HTTP proxy tunneling, SNI fronting, and direct connections with domain spoofing. It establishes SSH tunnels over WebSocket connections and provides a local SOCKS proxy for routing traffic.

## Features

- **Multiple Tunnel Strategies**: Support for proxy, SNI fronting, and direct connection modes
- **WebSocket-based SSH Tunnels**: Establishes SSH connections over WebSocket for better bypass capabilities
- **Dual Proxy Support**: Built-in SOCKS5 and HTTP proxy server for routing local traffic through the tunnel
- **Domain Spoofing**: Front domain support for Host header manipulation to bypass restrictions
- **Configurable Payloads**: Custom HTTP payload templates for different environments
- **Cross-platform**: Windows, Linux, and macOS support
- **Timeout Management**: Configurable connection timeouts and keepalive settings

## Installation

### Download Pre-built Binaries

Download the latest release from the [releases page](../../releases) for your platform:

#### Windows
- `tunn-windows-amd64.zip` - Windows 64-bit (Intel/AMD)
- `tunn-windows-386.zip` - Windows 32-bit  
- `tunn-windows-arm64.zip` - Windows ARM64

#### Linux
- `tunn-linux-amd64.tar.gz` - Linux 64-bit (Intel/AMD)
- `tunn-linux-386.tar.gz` - Linux 32-bit
- `tunn-linux-arm64.tar.gz` - Linux ARM64
- `tunn-linux-arm.tar.gz` - Linux ARM

#### macOS
- `tunn-darwin-amd64.tar.gz` - macOS Intel
- `tunn-darwin-arm64.tar.gz` - macOS Apple Silicon (M1/M2)

#### FreeBSD
- `tunn-freebsd-amd64.tar.gz` - FreeBSD 64-bit

**Installation Steps:**
1. Download the appropriate binary for your platform
2. Extract the archive
3. Move the binary to a directory in your PATH
4. Make it executable (Linux/macOS): `chmod +x tunn`

**Verify your download** using the provided `checksums.txt` file.

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

Tunn supports multiple tunneling strategies, and all modes support both SOCKS5 and HTTP local proxy types via the global `--proxy-type` flag:

**Global Proxy Type Control:**
- `--proxy-type socks5` (default): Universal compatibility, works with any TCP-based protocol
- `--proxy-type http`: Optimized for web traffic, works with HTTP/HTTPS applications

### 1. Proxy Mode

Routes traffic through an HTTP proxy server first, then establishes a WebSocket tunnel to the target host.

```bash
# Basic proxy mode with SOCKS5 local proxy (default)
tunn proxy --proxy-host proxy.example.com --target-host ssh-server.com --ssh-username user --ssh-password pass

# Proxy mode with HTTP local proxy
tunn --proxy-type http proxy --proxy-host proxy.example.com --target-host ssh-server.com --ssh-username user --ssh-password pass

# With custom proxy port and front domain
tunn --proxy-type socks5 proxy \
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
# Basic SNI fronting with SOCKS5 proxy (default)
tunn sni --front-domain google.com --proxy-host proxy.example.com --ssh-username user --ssh-password pass

# SNI fronting with HTTP proxy
tunn --proxy-type http sni \
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
# Basic direct connection with SOCKS5 proxy (default)
tunn direct --target-host ssh-server.com --ssh-username user --ssh-password pass

# Direct connection with HTTP proxy and front domain spoofing
tunn --proxy-type http direct \
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

- `--proxy-type`: Local proxy type - `socks5` (default) or `http` (global flag)
- `--local-port` / `-l`: Local proxy port (default: 1080 for SOCKS5, 8080 for HTTP)
- `--ssh-port`: SSH port on target server (default: 22)
- `--timeout` / `-t`: Connection timeout in seconds (0 = no timeout)
- `--payload`: Custom HTTP payload template
- `--verbose` / `-v`: Enable verbose output

## Configuration

Tunn supports two approaches for configuration and usage:

### Quick Start with Configuration Files

**The simplified way (recommended)**: Use a configuration file with profiles - the mode is automatically determined from the profile:

```bash
# Generate a sample configuration file
tunn config generate --output myconfig.json

# Edit the file to match your setup, then run:
tunn --config myconfig.json --profile default

# List available profiles
tunn config list --config myconfig.json

# Validate configuration
tunn config validate --config myconfig.json
```

**Traditional method**: Use command-line flags with explicit mode specification:

```bash
# Still supported for direct CLI usage
tunn proxy --proxy-host proxy.example.com --target-host target.example.com --ssh-username user --ssh-password pass
```

### Configuration Files (Xray-like)

Tunn supports JSON and YAML configuration files for managing complex tunnel configurations with multiple profiles, routing rules, and environment variable substitution.

#### Configuration Structure

```json
{
  "log": {
    "level": "info",
    "access": "/var/log/tunn/access.log",
    "error": "/var/log/tunn/error.log"
  },
  "inbounds": [...],
  "outbounds": [...],
  "routing": {...},
  "dns": {...},
  "profiles": [...]
}
```

#### Profiles Section

The `profiles` section is the main configuration area for tunnel definitions:

```json
{
  "profiles": [
    {
      "name": "default",
      "mode": "proxy",
      "proxyHost": "proxy.example.com",
      "proxyPort": "80",
      "targetHost": "target.example.com",
      "targetPort": "22",
      "frontDomain": "google.com",
      "ssh": {
        "username": "user",
        "password": "password",
        "port": "22"
      },
      "localPort": 1080,
      "proxyType": "socks5",
      "payload": "GET / HTTP/1.1\\r\\nHost: $FRONT_DOMAIN\\r\\nUpgrade: websocket\\r\\n\\r\\n",
      "timeout": 30
    }
  ]
}
```

#### Configuration Management Commands

```bash
# Generate sample configuration
tunn config generate --output tunn-config.json --format json
tunn config generate --output tunn-config.yaml --format yaml

# Validate configuration
tunn config validate --config tunn-config.json

# List available profiles
tunn config list --config tunn-config.json

# Use configuration with profiles
tunn --config tunn-config.json --profile default proxy
tunn --config tunn-config.json --profile sni-mode sni

# Override configuration with CLI flags
tunn --config tunn-config.json --profile default --proxy-type http proxy
```

#### Environment Variables in Configuration

Configuration files support environment variable substitution using the `$VARIABLE` syntax:

```json
{
  "profiles": [
    {
      "name": "production",
      "targetHost": "$TARGET_HOST",
      "ssh": {
        "username": "$SSH_USERNAME",
        "password": "$SSH_PASSWORD"
      }
    }
  ]
}
```

Set environment variables before running:

```bash
export TARGET_HOST="prod.example.com"
export SSH_USERNAME="admin"
export SSH_PASSWORD="secret"

tunn --config config.json --profile production proxy
```

#### Advanced Configuration Features

**Routing Rules:**

```json
{
  "routing": {
    "domainStrategy": "IPIfNonMatch",
    "rules": [
      {
        "type": "field",
        "domain": ["geosite:cn"],
        "outboundTag": "freedom"
      },
      {
        "type": "field",
        "ip": ["geoip:private"],
        "outboundTag": "freedom"
      }
    ]
  }
}
```

**Inbound/Outbound Configuration:**

```json
{
  "inbounds": [
    {
      "tag": "socks-in",
      "port": 1080,
      "listen": "127.0.0.1",
      "protocol": "socks"
    }
  ],
  "outbounds": [
    {
      "tag": "tunnel-out", 
      "protocol": "tunnel",
      "streamSettings": {
        "network": "ws",
        "security": "tls"
      }
    }
  ]
}
```

**DNS Configuration:**

```json
{
  "dns": {
    "hosts": {
      "example.com": "127.0.0.1"
    },
    "servers": [
      "8.8.8.8",
      "1.1.1.1"
    ]
  }
}
```

### CLI Configuration (Traditional)

#### Default Payload Template

```
GET / HTTP/1.1[crlf]Host: [host][crlf]Upgrade: websocket[crlf][crlf]
```

Placeholders:
- `[host]`: Replaced with target host or front domain
- `[crlf]`: Replaced with `\r\n`

#### Custom Payloads

You can specify custom HTTP payloads for different environments:

```bash
tunn proxy \
  --proxy-host proxy.example.com \
  --target-host ssh-server.com \
  --payload "CONNECT [host] HTTP/1.1[crlf]Host: [host][crlf][crlf]" \
  --ssh-username user \
  --ssh-password pass
```

## SOCKS5 vs HTTP Proxy Comparison

Tunn now supports both SOCKS5 and HTTP proxy types for your local proxy server. Here's when to use each:

### SOCKS5 Proxy (Default)

**Best for:** Universal application compatibility
**Protocols:** Any TCP-based protocol (SSH, HTTP, HTTPS, FTP, SMTP, etc.)

**Advantages:**
- ✅ **Protocol Agnostic**: Works with any TCP application
- ✅ **Binary Data Support**: Handles any type of data
- ✅ **Low Overhead**: Minimal protocol overhead
- ✅ **Port Flexibility**: Can connect to any port
- ✅ **Transparent**: Preserves original destination information

**Use cases:**
- SSH tunneling through proxies
- Database connections
- File transfers (FTP, SFTP)
- Email clients (SMTP, IMAP)
- Any non-web application

**Example usage:**
```bash
# SOCKS5 proxy (default)
tunn proxy --proxy-host proxy.example.com --target-host ssh-server.com --ssh-username user --ssh-password pass

# Configure applications
curl --socks5 127.0.0.1:1080 https://httpbin.org/ip
ssh -o ProxyCommand="nc -X 5 -x 127.0.0.1:1080 %h %p" user@remote-server
```

### HTTP Proxy

**Best for:** Web browsing and HTTP-based applications
**Protocols:** HTTP and HTTPS

**Advantages:**
- ✅ **Web Optimized**: Excellent performance for web traffic
- ✅ **Browser Compatible**: Works seamlessly with web browsers
- ✅ **Header Processing**: Can modify HTTP headers
- ✅ **CONNECT Support**: Full HTTPS tunneling support

**Limitations:**
- ❌ **HTTP/HTTPS Only**: Cannot handle other protocols directly
- ❌ **Limited Scope**: Not suitable for non-web applications

**Use cases:**
- Web browsing through corporate proxies
- HTTP API access
- Web scraping
- Browser-based applications

**Example usage:**
```bash
# HTTP proxy mode
tunn --proxy-type http proxy --proxy-host proxy.example.com --target-host ssh-server.com --ssh-username user --ssh-password pass

# Configure applications
curl --proxy 127.0.0.1:8080 https://httpbin.org/ip
export http_proxy=http://127.0.0.1:8080
export https_proxy=http://127.0.0.1:8080
```

### Choosing the Right Proxy Type

| Use Case | Recommended Type | Reason |
|----------|------------------|---------|
| SSH tunneling | SOCKS5 | Universal protocol support |
| Web browsing only | HTTP | Optimized for web traffic |
| Database connections | SOCKS5 | Supports non-HTTP protocols |
| Mixed applications | SOCKS5 | Maximum compatibility |
| Corporate environments | HTTP | Better integration with existing HTTP proxy infrastructure |

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

### Example 2: HTTP Proxy Mode for Web Traffic

```bash
# Use HTTP proxy mode for optimized web browsing
tunn --proxy-type http proxy \
  --proxy-host corporate-proxy.company.com \
  --proxy-port 8080 \
  --target-host ssh-server.com \
  --ssh-username user \
  --ssh-password pass

# The local HTTP proxy will be available on 127.0.0.1:8080
# Configure your browser to use 127.0.0.1:8080 as HTTP proxy
# Or set environment variables:
# export http_proxy=http://127.0.0.1:8080
# export https_proxy=http://127.0.0.1:8080
```

### Example 3: SNI Fronting for CDN Bypass

```bash
# Use SNI fronting to bypass CDN restrictions
tunn sni \
  --front-domain cloudflare.com \
  --proxy-host edge-server.com \
  --target-host hidden-server.com \
  --ssh-username user \
  --ssh-password pass
```

### Example 4: Direct Connection with Domain Spoofing

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

## Releases

This project uses automated GitHub Actions workflows to create releases with cross-platform binaries.

### For Maintainers

To create a new release:

```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

The release process automatically:
- Builds binaries for all supported platforms
- Creates compressed archives  
- Generates SHA256 checksums
- Creates a GitHub release with detailed changelog
- Uploads all assets

## Dependencies

- **[Cobra](https://github.com/spf13/cobra)**: CLI framework
- **[golang.org/x/crypto](https://golang.org/x/crypto)**: SSH client implementation

## Security Considerations

- **Password Security**: Avoid using passwords in command line arguments in production. Consider using SSH keys or environment variables.
- **Traffic Encryption**: All traffic is encrypted through SSH, but the initial WebSocket connection may be visible to network monitors.
- **Proxy Logs**: Be aware that proxy servers may log connection attempts.

## Troubleshooting

### Configuration File Issues

**Invalid Configuration**: Use `tunn config validate` to check for syntax errors

**Missing Required Fields**: Ensure all required fields are present in profiles

**Environment Variables**: Make sure environment variables are set before running

**File Permissions**: Ensure the configuration file is readable

**Debug Mode**: Use verbose mode to see configuration loading details:

```bash
tunn --config config.json --profile myprofile --verbose proxy
```

This will show:
- Configuration file loading status
- Profile selection
- Environment variable substitution
- Validation results

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
