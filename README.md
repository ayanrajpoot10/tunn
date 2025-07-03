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
git clone https://github.com/ayanrajpoot10/tunn.git
cd Tunn

# Install dependencies
go mod download

# Build for current platform
make build

# Or build for all platforms
make build-all
```

## Usage

Tunn supports multiple tunneling strategies and requires configuration via JSON/YAML files. All modes support both SOCKS5 and HTTP local proxy types configured in the config file.

### Configuration-Only Approach

Tunn now uses a **configuration-first approach** for better maintainability, version control, and sharing of tunnel configurations.

**Basic Usage:**
```bash
# Run with a configuration file
tunn --config myconfig.json

# Generate sample configurations
tunn config generate --mode proxy --output proxy-config.json
tunn config generate --mode sni --output sni-config.yaml --format yaml
tunn config generate --mode direct --output direct-config.json

# Validate configurations
tunn config validate --config myconfig.json
```

### Configuration File Structure

All tunnel configurations are defined in JSON or YAML files:

**Example config.json:**
```json
{
  "mode": "proxy",
  "targetHost": "target.example.com",
  "targetPort": "22",
  "proxyHost": "proxy.example.com", 
  "proxyPort": "80",
  "frontDomain": "google.com",
  "ssh": {
    "username": "user",
    "password": "password",
    "port": "22"
  },
  "localPort": 1080,
  "proxyType": "socks5",
  "timeout": 30
}
```

**Example config.yaml:**
```yaml
mode: direct
targetHost: ssh-server.com
targetPort: "22"
frontDomain: google.com
ssh:
  username: user
  password: password
  port: "22"
localPort: 1080
proxyType: http
timeout: 30
```

### Tunnel Modes

#### 1. Proxy Mode
Routes traffic through an HTTP proxy server first, then establishes a WebSocket tunnel to the target host.

**Required configuration:**
```json
{
  "mode": "proxy",
  "proxyHost": "proxy.example.com",
  "proxyPort": "80", 
  "targetHost": "ssh-server.com",
  "targetPort": "22",
  "ssh": {
    "username": "user",
    "password": "password"
  }
}
```

#### 2. SNI Fronting Mode

Uses SNI (Server Name Indication) fronting to establish connections through a proxy with forged SNI headers.

**Required configuration:**
```json
{
  "mode": "sni",
  "frontDomain": "cloudflare.com",
  "proxyHost": "proxy.example.com",
  "proxyPort": "443",
  "targetHost": "ssh-server.com", 
  "targetPort": "22",
  "ssh": {
    "username": "user",
    "password": "password"
  }
}
```

#### 3. Direct Mode

Establishes a direct connection to the target host with optional Host header spoofing.

**Required configuration:**
```json
{
  "mode": "direct",
  "targetHost": "ssh-server.com",
  "targetPort": "22",
  "frontDomain": "google.com",
  "ssh": {
    "username": "user", 
    "password": "password"
  }
}
```

### Configuration Options

All configuration files support these options:

**Required fields:**
- `mode`: Tunnel strategy - `proxy`, `sni`, or `direct`
- `targetHost`: Target SSH server hostname
- `ssh.username`: SSH username
- `ssh.password`: SSH password

**Optional fields:**
- `targetPort`: Target server port (default: "22")  
- `proxyHost`: Proxy server hostname (required for proxy/sni modes)
- `proxyPort`: Proxy server port (default: "80" for proxy, "443" for sni)
- `frontDomain`: Front domain for Host header spoofing
- `ssh.port`: SSH port on target server (default: "22")
- `localPort`: Local proxy port (default: 1080)
- `proxyType`: Local proxy type - `socks5` or `http` (default: "socks5")
- `payload`: Custom HTTP payload template
- `timeout`: Connection timeout in seconds (default: 30)

## Configuration Management

### Quick Configuration Setup

```bash
# Generate sample configurations for different modes
tunn config generate --mode proxy --output proxy-config.json
tunn config generate --mode sni --output sni-config.yaml --format yaml  
tunn config generate --mode direct --output direct-config.json

# Validate your configuration
tunn config validate --config myconfig.json

# Run with your configuration
tunn --config myconfig.json
```

### Environment Variables

Configuration files support environment variable substitution using `$VARIABLE` syntax:

```json
{
  "mode": "proxy",
  "targetHost": "$TARGET_HOST",
  "ssh": {
    "username": "$SSH_USERNAME",
    "password": "$SSH_PASSWORD"
  }
}
```

Set variables before running:
```bash
export TARGET_HOST="prod.example.com"
export SSH_USERNAME="admin"  
export SSH_PASSWORD="secret"
tunn --config config.json
```

## SOCKS5 vs HTTP Proxy Configuration

Tunn supports both SOCKS5 and HTTP proxy types configured via the `proxyType` field in your configuration file:

### SOCKS5 Proxy (Default)

**Best for:** Universal application compatibility
**Configuration:** `"proxyType": "socks5"`
**Default Port:** 1080

**Advantages:**
- ✅ **Protocol Agnostic**: Works with any TCP application
- ✅ **Binary Data Support**: Handles any type of data
- ✅ **Low Overhead**: Minimal protocol overhead
- ✅ **Port Flexibility**: Can connect to any port
- ✅ **Transparent**: Preserves original destination information

**Configuration example:**
```json
{
  "mode": "proxy",
  "proxyType": "socks5",
  "localPort": 1080,
  "targetHost": "ssh-server.com",
  "proxyHost": "proxy.example.com",
  "ssh": {"username": "user", "password": "pass"}
}
```

### HTTP Proxy

**Best for:** Web browsing and HTTP-based applications  
**Configuration:** `"proxyType": "http"`
**Default Port:** 8080

**Advantages:**
- ✅ **Web Optimized**: Excellent performance for web traffic
- ✅ **Browser Compatible**: Works seamlessly with web browsers
- ✅ **Header Processing**: Can modify HTTP headers
- ✅ **CONNECT Support**: Full HTTPS tunneling support

**Configuration example:**
```json
{
  "mode": "direct", 
  "proxyType": "http",
  "localPort": 8080,
  "targetHost": "ssh-server.com",
  "ssh": {"username": "user", "password": "pass"}
}
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

Create a configuration file `corporate-proxy.json`:
```json
{
  "mode": "proxy",
  "proxyHost": "corporate-proxy.company.com",
  "proxyPort": "8080", 
  "targetHost": "remote-server.com",
  "frontDomain": "microsoft.com",
  "ssh": {
    "username": "myuser",
    "password": "mypassword"
  },
  "localPort": 1080,
  "proxyType": "socks5"
}
```

Run: `tunn --config corporate-proxy.json`

### Example 2: HTTP Proxy Mode for Web Traffic

Create `web-proxy.json`:
```json
{
  "mode": "proxy",
  "proxyHost": "corporate-proxy.company.com", 
  "proxyPort": "8080",
  "targetHost": "ssh-server.com",
  "ssh": {
    "username": "user",
    "password": "pass"
  },
  "localPort": 8080,
  "proxyType": "http"
}
```

Run: `tunn --config web-proxy.json`

Configure your browser to use HTTP proxy 127.0.0.1:8080

### Example 3: SNI Fronting for CDN Bypass

Create `sni-fronting.yaml`:
```yaml
mode: sni
frontDomain: cloudflare.com
proxyHost: edge-server.com
targetHost: hidden-server.com
ssh:
  username: user
  password: pass
localPort: 1080
proxyType: socks5
```

Run: `tunn --config sni-fronting.yaml`

### Example 4: Direct Connection with Domain Spoofing

Create `direct-tunnel.json`:
```json
{
  "mode": "direct",
  "targetHost": "my-server.com",
  "frontDomain": "google.com",
  "ssh": {
    "username": "admin", 
    "password": "secret123"
  },
  "timeout": 60,
  "proxyType": "socks5"
}
```

Run: `tunn --config direct-tunnel.json`

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

# curl with HTTP proxy  
curl --proxy 127.0.0.1:8080 https://example.com

# ssh through the tunnel
ssh -o ProxyCommand="nc -X 5 -x 127.0.0.1:1080 %h %p" user@target-server.com

# Set environment variables for HTTP proxy
export http_proxy=http://127.0.0.1:8080
export https_proxy=http://127.0.0.1:8080
```

## Development

### Project Structure

```
tunn/
├── main.go              # Application entry point
├── go.mod               # Go module dependencies
├── Makefile            # Build automation
├── cmd/                # CLI commands
│   ├── root.go         # Root command and config file handling
│   └── config.go       # Configuration management commands
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

## Troubleshooting

### Configuration File Issues

**Invalid Configuration**: Use `tunn config validate --config myconfig.json` to check for syntax errors

**Missing Required Fields**: Ensure all required fields are present:
- `mode`: Must be "proxy", "sni", or "direct"
- `targetHost`: Target SSH server hostname
- `ssh.username` and `ssh.password`: SSH credentials

**Environment Variables**: Make sure environment variables are set before running

**File Permissions**: Ensure the configuration file is readable

**Debug Mode**: Check configuration loading by examining the startup output

### Common Issues

**Configuration File Not Found**:
- Verify the path to your config file
- Use absolute paths if relative paths don't work
- Check file permissions

**Connection Timeout**:
- Increase timeout in config: `"timeout": 60`
- Check network connectivity to proxy/target hosts

**SSH Authentication Failed**:
- Verify SSH credentials in config file
- Check if SSH service is running on target port
- Ensure target host allows password authentication

**WebSocket Upgrade Failed**:
- Try different payload templates in config
- Check if proxy supports WebSocket upgrades
- Verify front domain is not blocked

**SOCKS/HTTP Proxy Not Working**:
- Ensure Tunn is running and showing "proxy up" message
- Check local firewall settings
- Verify application proxy configuration matches your config file settings

### Config File Examples

**Minimal proxy config:**
```json
{
  "mode": "proxy",
  "proxyHost": "proxy.example.com",
  "targetHost": "ssh-server.com", 
  "ssh": {"username": "user", "password": "pass"}
}
```

**Minimal direct config:**
```json
{
  "mode": "direct",
  "targetHost": "ssh-server.com",
  "ssh": {"username": "user", "password": "pass"}
}
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.
