/*
Copyright (c) 2026.

MIT License - see LICENSE file for details.
*/

package api

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// ahpSessionMembers lists the live processes in a session, read from /proc.
func ahpSessionMembers(t *testing.T, sid int) []int {
	t.Helper()
	stats, err := filepath.Glob("/proc/[0-9]*/stat")
	require.NoError(t, err)
	var members []int
	for _, stat := range stats {
		raw, err := os.ReadFile(stat)
		if err != nil {
			continue
		}
		line := string(raw)
		fields := strings.Fields(line[strings.LastIndex(line, ")")+1:])
		if len(fields) < 4 || fields[0] == "Z" || fields[3] != strconv.Itoa(sid) {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(stat, "/proc/"), "/stat"))
		if err == nil {
			members = append(members, pid)
		}
	}
	return members
}

// A terminal's session is stopped with nothing but sh: the shell and what it
// started in the background are killed, its PID file goes, and a process
// outside the session is left alone.
func TestAHPStopSessionScript(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "terminal.pid")
	shell := exec.Command("/bin/sh", "-c", `echo $$ > "$1"; sleep 60 & sleep 60`, "sh", pidFile)
	shell.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	require.NoError(t, shell.Start())
	t.Cleanup(func() { _ = syscall.Kill(-shell.Process.Pid, syscall.SIGKILL) })
	outside := exec.Command("sleep", "60")
	require.NoError(t, outside.Start())
	t.Cleanup(func() { _ = outside.Process.Kill(); _ = outside.Wait() })
	require.Eventually(t, func() bool { return len(ahpSessionMembers(t, shell.Process.Pid)) >= 3 }, 5*time.Second, 20*time.Millisecond,
		"the shell and both sleeps run in its session")

	out, err := exec.Command("/bin/sh", "-c", ahpStopSessionScript, "sh", pidFile).CombinedOutput()
	require.NoError(t, err, string(out))
	exited := make(chan error, 1)
	go func() { exited <- shell.Wait() }()
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		t.Fatal("the shell outlived the script")
	}
	require.Eventually(t, func() bool { return len(ahpSessionMembers(t, shell.Process.Pid)) == 0 }, 5*time.Second, 20*time.Millisecond,
		"a process the shell started outlived the script")
	require.NoFileExists(t, pidFile)
	require.NoError(t, outside.Process.Signal(syscall.Signal(0)), "a process outside the session is left alone")
}
