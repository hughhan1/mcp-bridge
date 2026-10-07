package mcpstdio

import (
	"context"
	"errors"

	"github.com/hughhan1/mcp-bridge/proxy"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Run exposes remote through local for the lifetime of ctx. It owns both
// connections and closes them on EOF, cancellation, or failure. The protocol
// handshake and messages are forwarded without translating protocol eras.
// Local EOF or closed output completes normally; upstream disconnects and other
// transport failures return errors.
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
