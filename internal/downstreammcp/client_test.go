package downstreammcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type echoInput struct {
	Message string `json:"message"`
}

type echoOutput struct {
	Echo string `json:"echo"`
}

func TestStateLoadSaveRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp-servers.yaml")
	state := State{}
	state.Upsert(Server{Name: "linear", Transport: TransportCommand, Command: "npx", Args: []string{"-y", "mcp-remote", "https://mcp.linear.app/mcp"}})
	if err := Save(path, state); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	server, ok := loaded.Get("linear")
	if !ok {
		t.Fatal("linear server missing")
	}
	if server.Transport != TransportCommand || server.Command != "npx" {
		t.Fatalf("server = %#v", server)
	}
	if server.TimeoutMS != DefaultTimeoutMS || server.MaxBytes != DefaultMaxBytes {
		t.Fatalf("defaults not applied: %#v", server)
	}
}

func TestLoadMissingReturnsEmptyState(t *testing.T) {
	state, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Servers) != 0 {
		t.Fatalf("servers = %#v", state.Servers)
	}
}

func TestServerValidate(t *testing.T) {
	if err := (Server{Name: "x", Transport: TransportCommand, Command: "cmd"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (Server{Name: "x", Transport: TransportSSE, URL: "http://127.0.0.1/sse"}).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := (Server{Name: "x", Transport: TransportCommand}).Validate(); err == nil {
		t.Fatal("expected missing command error")
	}
}

func TestSaveUsesPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mcp-servers.yaml")
	if err := Save(path, State{}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestSDKListToolsAndCallToolWithInMemoryMCP(t *testing.T) {
	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()
	server := mcpsdk.NewServer(&mcpsdk.Implementation{Name: "test-server"}, nil)
	mcpsdk.AddTool(server, &mcpsdk.Tool{Name: "echo", Description: "Echo a message"},
		func(_ context.Context, _ *mcpsdk.CallToolRequest, in echoInput) (*mcpsdk.CallToolResult, echoOutput, error) {
			return nil, echoOutput{Echo: in.Message}, nil
		})
	errc := make(chan error, 1)
	go func() {
		errc <- server.Run(context.Background(), serverTransport)
	}()

	mcpClient := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client"}, nil)
	session, err := mcpClient.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = session.Close()
		_ = <-errc
	}()

	tools, err := session.ListTools(context.Background(), &mcpsdk.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "echo" {
		t.Fatalf("tools = %#v", tools.Tools)
	}
	res, err := session.CallTool(context.Background(), &mcpsdk.CallToolParams{
		Name:      "echo",
		Arguments: map[string]any{"message": "hello"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.StructuredContent == nil {
		t.Fatalf("structured content missing: %#v", res)
	}
}
