// Package main provides the entry point for the Tunn SSH tunneling tool.
//
// Tunn is a cross-platform SSH tunneling application that creates secure
// connections through direct connections over WebSocket and HTTP proxies.
// It supports both SOCKS5 and HTTP proxy modes for flexible tunneling solutions.
//
// Usage:
//
//	tunn --config config.json
//	tunn config generate --mode direct
//	tunn config validate --config myconfig.json
//
// For more information, see the README.md file or run:
//
//	tunn --help
package main

import "tunn/cmd"

// main is the application entry point that delegates execution to the CLI command handler.
func main() {
	cmd.Execute()
}
