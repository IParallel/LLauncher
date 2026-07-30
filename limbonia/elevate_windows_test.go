//go:build windows

package limbonia

import (
	"errors"
	"strings"
	"syscall"
	"testing"
)

// ERROR_CANCELLED is what ShellExecuteExW reports when the user dismisses the UAC
// prompt, and it is indistinguishable from a real failure unless it is
// specifically recognised. The prompt itself cannot be driven from a test, so
// this covers the half that is ours: the errno the shell hands back becoming the
// sentinel the rest of the launcher branches on.
func TestElevationErrorMapsCancelledToSentinel(t *testing.T) {
	err := elevationError(`C:\limbonia\Mephi.exe`, syscall.Errno(1223))

	if !errors.Is(err, ErrElevationCancelled) {
		t.Fatalf("ERROR_CANCELLED (1223) should map to ErrElevationCancelled, got %v", err)
	}
}

// Everything else has to stay a fault. Reporting a missing file or a blocked
// executable as "you declined the prompt" would send the user to retry a prompt
// that is never going to appear.
func TestElevationErrorKeepsRealFailures(t *testing.T) {
	cases := []struct {
		name  string
		errno syscall.Errno
	}{
		{"ERROR_FILE_NOT_FOUND", syscall.Errno(2)},
		{"ERROR_ACCESS_DENIED", syscall.Errno(5)},
		{"ERROR_ELEVATION_REQUIRED", syscall.Errno(740)},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := elevationError(`C:\limbonia\Injector.exe`, c.errno)

			if errors.Is(err, ErrElevationCancelled) {
				t.Fatalf("%s must not be reported as a declined prompt", c.name)
			}
			if err == nil {
				t.Fatal("a failed launch must produce an error")
			}
			// Named so the message says which of the two binaries went wrong.
			if !strings.Contains(err.Error(), "Injector.exe") {
				t.Errorf("the message should name the executable, got %q", err)
			}
			// The full path is machine-specific noise in a toast.
			if strings.Contains(err.Error(), `C:\limbonia`) {
				t.Errorf("the message should not carry the whole path, got %q", err)
			}
		})
	}
}

// The refusal has to survive the trip through the wrapper each caller applies,
// because that is the error the frontend actually receives.
func TestCancelledElevationReachesTheUserAsAdvice(t *testing.T) {
	err := elevationRefused("The injector", elevationError(`C:\limbonia\Injector.exe`, syscall.Errno(1223)))

	if !errors.Is(err, ErrElevationCancelled) {
		t.Fatalf("the sentinel must survive wrapping, got %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, "The injector") || !strings.Contains(strings.ToLower(msg), "permission prompt") {
		t.Errorf("the user-facing message should name the thing and the fix, got %q", msg)
	}
}
