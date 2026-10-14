package app

import (
	"context"
	"os/exec"
	"strconv"
	"time"
)

func configureProcess(cmd *exec.Cmd) func() {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		kill := exec.CommandContext(ctx, "taskkill", "/T", "/F", "/PID", strconv.Itoa(cmd.Process.Pid))
		kill.WaitDelay = time.Second
		_ = kill.Run()
		return cmd.Process.Kill()
	}
	// shortcut: taskkill is best effort for descendants; use a job object before
	// promising process-tree containment on Windows.
	return func() { _ = cmd.Cancel() }
}
