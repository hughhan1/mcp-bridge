package proxy

import (
	"encoding/json"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// CurrentVersion is the current MCP protocol revision supported by the bridge.
	CurrentVersion = "2026-07-28"
	// MaxMessageBytes is the maximum accepted JSON-RPC message size in bytes.
	MaxMessageBytes = 4 << 20
	// MaxRequests is the maximum number of concurrent calls or legacy sessions.
	MaxRequests = 64
)

// IsLegacyVersion reports whether version is a supported MCP revision older
// than [CurrentVersion].
func IsLegacyVersion(version string) bool {
	return version < CurrentVersion && slices.Contains(mcp.SupportedProtocolVersions(), version)
}

// UnsupportedVersion returns an MCP error identifying the rejected version
// and the versions clients may use instead.
func UnsupportedVersion(requested string, supported []string) *jsonrpc.Error {
	data, _ := json.Marshal(&mcp.UnsupportedProtocolVersionData{Requested: requested, Supported: supported})
	return &jsonrpc.Error{Code: mcp.CodeUnsupportedProtocolVersion, Message: "unsupported MCP protocol version: " + requested, Data: data}
}
