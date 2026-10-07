package proxy

import (
	"encoding/json"
	"slices"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	CurrentVersion  = "2026-07-28"
	MaxMessageBytes = 4 << 20
	MaxRequests     = 64
)

func IsLegacyVersion(version string) bool {
	return version < CurrentVersion && slices.Contains(mcp.SupportedProtocolVersions(), version)
}

func UnsupportedVersion(requested string, supported []string) *jsonrpc.Error {
	data, _ := json.Marshal(&mcp.UnsupportedProtocolVersionData{Requested: requested, Supported: supported})
	return &jsonrpc.Error{Code: mcp.CodeUnsupportedProtocolVersion, Message: "unsupported MCP protocol version: " + requested, Data: data}
}
