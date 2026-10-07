package mcpstdio

import (
	"context"
	"errors"

	"github.com/hughhan1/mcp-bridge/proxy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Run exposes a remote MCP server through a local transport and closes both
// connections when forwarding ends. A local EOF or closed local output completes
// normally. Remote disconnects, cancellation, and other transport failures return
// errors. Both transports must be non-nil.
func Run(ctx context.Context, remote, local mcp.Transport) error {
	if remote == nil || local == nil {
		return errors.New("remote and local transports are required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	downstream, err := local.Connect(ctx)
	if err != nil {
		return err
	}
	upstream, err := connectRemote(ctx, remote)
	if err != nil {
		downstream.Close()
		return err
	}
	return proxy.Relay(ctx, upstream, downstream)
}
