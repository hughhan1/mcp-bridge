package mcpstdio

import (
	"context"

	"github.com/hughhan1/mcp-bridge/stdio/internal/bridge"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Run exposes a remote MCP server through a local transport and closes both
// connections when forwarding ends. A local EOF or closed local output completes
// normally. Remote disconnects, cancellation, and other transport failures return
// errors.
func Run(ctx context.Context, remote, local mcp.Transport) error {
	return bridge.Run(ctx, remote, local)
}
