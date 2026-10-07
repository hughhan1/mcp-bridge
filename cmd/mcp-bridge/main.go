package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/spf13/cobra"
)

// Set by GoReleaser from the release tag.
var version string

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil && !errors.Is(err, context.Canceled) {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, input io.ReadCloser, output io.WriteCloser, diagnostics io.Writer) error {
	commandVersion := version
	if commandVersion == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			commandVersion = info.Main.Version
		}
	}
	root := &cobra.Command{
		Use:               "mcp-bridge",
		Version:           commandVersion,
		Short:             "Connect MCP clients and servers across stdio and HTTP",
		SilenceErrors:     true,
		SilenceUsage:      true,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
		RunE: func(*cobra.Command, []string) error {
			return errors.New("a subcommand is required; use 'mcp-bridge --help' for usage")
		},
	}
	root.SetArgs(append([]string{}, args...))
	root.SetOut(diagnostics)
	root.SetErr(diagnostics)
	root.AddCommand(newServeCommand(), newConnectCommand(input, output))
	return root.ExecuteContext(ctx)
}
