// Package mcpstdio makes remote MCP servers available through a local transport,
// typically stdio. [Run] forwards messages until the local connection closes,
// the context is cancelled, or forwarding fails.
package mcpstdio
