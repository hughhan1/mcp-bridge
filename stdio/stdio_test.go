package mcpstdio_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	mcpbridge "github.com/hughhan1/mcp-bridge/stdio"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }
func (failingWriter) Close() error                { return nil }

func TestStdioLifecycle(t *testing.T) {
	for _, mode := range []string{"streamable-http", "sse"} {
		t.Run("cancel/"+mode, func(t *testing.T) {
			started, cancelled := make(chan struct{}), make(chan struct{})
			s := mcp.NewServer(&mcp.Implementation{Name: "cancel", Version: "1"}, nil)
			mcp.AddTool(s, &mcp.Tool{Name: "slow"}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
				close(started)
				<-ctx.Done()
				close(cancelled)
				return nil, nil, ctx.Err()
			})
			var handler http.Handler = mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return s }, nil)
			if mode == "sse" {
				handler = mcp.NewSSEHandler(func(*http.Request) *mcp.Server { return s }, nil)
			}
			server := httptest.NewServer(handler)
			defer server.Close()
			var remote mcp.Transport = &mcp.StreamableClientTransport{Endpoint: server.URL}
			if mode == "sse" {
				remote = &mcp.SSEClientTransport{Endpoint: server.URL}
			}
			local, clientTransport := mcp.NewInMemoryTransports()
			ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
			defer cancel()
			finished := make(chan error, 1)
			go func() { finished <- mcpbridge.Run(ctx, remote, local) }()
			client := mcp.NewClient(&mcp.Implementation{Name: "cancel-test", Version: "1"}, nil)
			session, err := client.Connect(ctx, clientTransport, &mcp.ClientSessionOptions{ProtocolVersion: "2025-11-25"})
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			go func() {
				_, _ = session.CallTool(t.Context(), &mcp.CallToolParams{Name: "slow", Arguments: map[string]any{}})
			}()
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("remote call did not start")
			}
			cancel()
			select {
			case err := <-finished:
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("proxy cancellation blocked")
			}
			select {
			case <-cancelled:
			case <-time.After(5 * time.Second):
				t.Fatal("remote call survived proxy cancellation")
			}
		})
	}
	for _, failure := range []struct {
		name    string
		local   bool
		readEOF bool
		err     error
		wantErr bool
	}{
		{name: "remote read EOF", readEOF: true, wantErr: true},
		{name: "remote write EOF", err: io.EOF, wantErr: true},
		{name: "remote write failure", err: io.ErrClosedPipe, wantErr: true},
		{name: "remote connection closed", err: mcp.ErrConnectionClosed, wantErr: true},
		{name: "local write EOF", local: true, err: io.EOF},
		{name: "local write closed pipe", local: true, err: io.ErrClosedPipe},
		{name: "local connection closed", local: true, err: mcp.ErrConnectionClosed},
		{name: "local write failure", local: true, err: errors.New("local output failed"), wantErr: true},
	} {
		t.Run(failure.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			reader, writer := io.Pipe()
			defer writer.Close()
			if failure.readEOF {
				writer.Close()
			}
			memory, client := mcp.NewInMemoryTransports()
			var remote, local mcp.Transport = &mcp.IOTransport{Reader: reader, Writer: failingWriter{failure.err}}, memory
			if failure.local {
				remote, local = local, remote
			}
			finished := make(chan error, 1)
			go func() { finished <- mcpbridge.Run(ctx, remote, local) }()
			conn, err := client.Connect(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			if !failure.readEOF {
				if err := conn.Write(ctx, &jsonrpc.Request{Method: "notifications/initialized", Params: json.RawMessage(`{}`)}); err != nil {
					t.Fatal(err)
				}
			}
			select {
			case err := <-finished:
				if ctx.Err() != nil || (err != nil) != failure.wantErr {
					t.Fatalf("shutdown: %v, want error: %t", err, failure.wantErr)
				}
			case <-ctx.Done():
				t.Fatal("proxy did not stop after connection failure")
			}
		})
	}
}

func TestStdioRequestIDs(t *testing.T) {
	for _, value := range []any{float64(1), "client-request"} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			upstream := mcp.NewServer(&mcp.Implementation{Name: "fixture", Version: "1"}, nil)
			remote, upstreamTransport := mcp.NewInMemoryTransports()
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			session, err := upstream.Connect(ctx, upstreamTransport, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer session.Close()
			local, client := mcp.NewInMemoryTransports()
			finished := make(chan error, 1)
			go func() {
				finished <- mcpbridge.Run(ctx, remote, local)
			}()
			conn, err := client.Connect(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			id, err := jsonrpc.MakeID(value)
			if err != nil {
				t.Fatal(err)
			}
			for _, request := range []*jsonrpc.Request{
				{ID: id, Method: "initialize", Params: json.RawMessage(`{"protocolVersion":"2025-11-25","clientInfo":{"name":"ids","version":"1"},"capabilities":{}}`)},
				{Method: "notifications/initialized", Params: json.RawMessage(`{}`)},
				{ID: id, Method: "tools/list", Params: json.RawMessage(`{}`)},
			} {
				if err := conn.Write(ctx, request); err != nil {
					t.Fatal(err)
				}
				if request.IsCall() {
					message, err := conn.Read(ctx)
					if err != nil {
						t.Fatal(err)
					}
					if response, ok := message.(*jsonrpc.Response); !ok || response.Error != nil || response.ID != id {
						t.Fatalf("%s: %v", request.Method, message)
					}
				}
			}
			conn.Close()
			select {
			case err := <-finished:
				if err != nil {
					t.Fatal(err)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
		})
	}
}
