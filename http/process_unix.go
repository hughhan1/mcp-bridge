//go:build unix

package mcphttp

import (
	"errors"
	"os/exec"
	"syscall"
)

func prepareProcess(cmd *exec.Cmd) error {
	if cmd.SysProcAttr != nil && (cmd.SysProcAttr.Pgid != 0 || cmd.SysProcAttr.Setsid) {
		return errors.New("bridge requires its own process group")
	}
	attrs := syscall.SysProcAttr{}
	if cmd.SysProcAttr != nil {
		attrs = *cmd.SysProcAttr
	}
	attrs.Setpgid = true
	cmd.SysProcAttr = &attrs
	return nil
}

func signalProcess(cmd *exec.Cmd, force bool) {
	signal := syscall.SIGTERM
	if force {
		signal = syscall.SIGKILL
	}
	_ = syscall.Kill(-cmd.Process.Pid, signal)
}
