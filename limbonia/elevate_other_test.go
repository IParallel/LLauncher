//go:build !windows

package limbonia

import (
	"os"
	"path/filepath"
	"testing"
)

// The /proc scan is the whole Linux process lookup, and it is the one part that
// cannot be checked with a fake: the point is whether it reads a real kernel
// correctly. So it is pointed at a process that is certainly running — this test
// binary — which also exercises the comm truncation, since Go's test binaries are
// named well past the 15-character limit.
func TestIsProcessRunningFindsThisProcess(t *testing.T) {
	if _, err := os.Stat("/proc/self/comm"); err != nil {
		t.Skip("no /proc on this system")
	}

	self := filepath.Base(os.Args[0])
	if self == "" {
		t.Skip("cannot determine this process's name")
	}

	if !isProcessRunning(self) {
		comm, _ := os.ReadFile("/proc/self/comm")
		t.Errorf("isProcessRunning(%q) = false, but this process is plainly running (comm=%q)", self, comm)
	}
}

func TestIsProcessRunningIgnoresAbsentProcesses(t *testing.T) {
	if _, err := os.Stat("/proc/self/comm"); err != nil {
		t.Skip("no /proc on this system")
	}

	if isProcessRunning("definitely-not-a-real-process-xyzzy") {
		t.Error("a process that does not exist must not be reported as running")
	}
}

// The rule that matters for the launcher: a companion that is not running must
// not suppress the launch. This used to be hardcoded false, which meant the
// answer was right by accident and wrong the moment Mephi WAS running.
func TestMephiNotRunningOnAnIdleSystem(t *testing.T) {
	if _, err := os.Stat("/proc/self/comm"); err != nil {
		t.Skip("no /proc on this system")
	}

	// Nothing in a test environment is called Mephi; if this ever fires it means
	// the scan is matching far too loosely.
	if IsMephiRunning() {
		t.Error("the scan matched something that is not the companion")
	}
}
