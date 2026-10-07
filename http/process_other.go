//go:build !unix

package mcphttp

import (
	"errors"
	"os/exec"
)

func prepareProcess(*exec.Cmd) error      { return errors.New("mcp-bridge requires a Unix process host") }
func signalProcess(cmd *exec.Cmd, _ bool) { _ = cmd.Process.Kill() }
