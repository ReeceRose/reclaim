//go:build unix

package ffmpeg

import (
	"os/exec"
	"syscall"
)

// setProcessGroup puts ffmpeg in its own process group so the whole tree can be
// signalled at once. Without this, ffmpeg's child processes can be orphaned when
// the parent is killed.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends SIGKILL to the entire group (negative pid). Setpgid
// above makes the group id equal the child's pid.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
