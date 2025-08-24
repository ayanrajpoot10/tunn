# Tunn - SSH Tunneling Tool

A cross-platform SSH tunneling tool that creates secure connections through direct connections over WebSocket and HTTP proxies.

## Features

- **Multiple tunnel modes**: Direct connection and Proxy
- **WebSocket-based SSH tunnels** for better bypass capabilities
- **SOCKS5 and HTTP proxy support** 
- **Domain spoofing** capabilities
- **Cross-platform** (Windows, Linux, macOS)

## Installation

Download the latest binary from the [releases page](../../releases) or build from source:

```bash
git clone https://github.com/ayanrajpoot10/Tunn.git
cd Tunn
go build
```

## Quick Start

1. Generate a configuration file:
```bash
tunn config generate --mode direct --output config.json
```

2. Edit the configuration with your details:
```json
{
  "mode": "direct",
  "ssh": {
    "host": "www.ayanrajpoot.net",
    "port": 80,
    "username": "abc",
    "password": "1234"
  },
  "listener": {
    "port": 1080,
    "proxyType": "http"
  },
  "httpPayload": "GET / HTTP/1.1[crlf]Host: api.ril.com[crlf]Upgrade: websocket[crlf][crlf]",
  "connectionTimeout": 30
}
```

3. Run Tunn:
```bash
tunn --config config.json
```

4. Configure your applications to use the proxy at `127.0.0.1:1080`

## Configuration

### Tunnel Modes

- **Direct Mode**: Direct connection with optional domain spoofing
- **Proxy Mode**: Routes through HTTP proxy → WebSocket → SSH server

### Required Fields
- `mode`: "direct" or "proxy"
- `ssh.host`: SSH server hostname
- `ssh.username` and `ssh.password`: SSH credentials

### Optional Fields
- `listener.port`: Local proxy port (default: 1080)
- `listener.proxyType`: "socks5" or "http" (default: "socks5")
- `connectionTimeout`: Connection timeout in seconds (default: 30)

## Usage Examples

### Browser Configuration
Set your browser to use HTTP/SOCKS5 proxy at `127.0.0.1:1080`

### System-Wide Proxy
Configure your system proxy settings to use `127.0.0.1:1080` (SOCKS5) or `127.0.0.1:1080` (HTTP) for system-wide tunneling.

## License

MIT License - see LICENSE file for details.
