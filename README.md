# Tunn - SSH Tunneling Tool

A cross-platform SSH tunneling tool that creates secure connections through HTTP proxies, SNI fronting, and direct connections over WebSocket.

## Features

- **Multiple tunnel modes**: Proxy, SNI fronting, and direct connection
- **WebSocket-based SSH tunnels** for better bypass capabilities
- **SOCKS5 and HTTP proxy support** 
- **Domain spoofing** capabilities
- **Cross-platform** (Windows, Linux, macOS)

## Installation

Download the latest binary from the [releases page](../../releases) or build from source:

```bash
git clone https://github.com/ayanrajpoot10/Tunn.git
cd Tunn
make build
```

## Quick Start

1. Generate a configuration file:
```bash
tunn config generate --mode proxy --output config.json
```

2. Edit the configuration with your details:
```json
{
  "connectionMode": "proxy",
  "proxyHost": "proxy.example.com",
  "proxyPort": "80",
  "serverHost": "ssh-server.com",
  "ssh": {
    "username": "user",
    "password": "password"
  },
  "listenPort": 1080,
  "proxyType": "socks5"
}
```

3. Run Tunn:
```bash
tunn --config config.json
```

4. Configure your applications to use the SOCKS proxy at `127.0.0.1:1080`

## Configuration

### Tunnel Modes

- **Proxy Mode**: Routes through HTTP proxy → WebSocket → SSH server
- **SNI Fronting**: Uses SNI header manipulation for bypassing restrictions
- **Direct Mode**: Direct connection with optional domain spoofing

### Required Fields
- `connectionMode`: "proxy", "sni", or "direct"
- `serverHost`: SSH server hostname
- `ssh.username` and `ssh.password`: SSH credentials

### Optional Fields
- `listenPort`: Local proxy port (default: 1080)
- `proxyType`: "socks5" or "http" (default: "socks5")
- `connectionTimeout`: Connection timeout in seconds (default: 30)

## Usage Examples

### Browser Configuration
Set your browser to use SOCKS5 proxy at `127.0.0.1:1080`

### System-Wide Proxy
Configure your system proxy settings to use `127.0.0.1:1080` (SOCKS5) or `127.0.0.1:8080` (HTTP) for system-wide tunneling.

## License

MIT License - see LICENSE file for details.
