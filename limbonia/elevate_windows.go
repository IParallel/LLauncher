//go:build windows

package limbonia

import (
	"errors"
	"fmt"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32DLL          = windows.NewLazySystemDLL("shell32.dll")
	procShellExecuteExW = shell32DLL.NewProc("ShellExecuteExW")
)

// gameLaunchGuard is on here: the injector STARTS Limbus Company — injector.cfg
// names the executable for it to launch — so an already-open copy is one it can
// never patch. It would fail, or start a second game, and the user would carry on
// playing unpatched. See IsLimbusRunning.
const gameLaunchGuard = true

const (
	// SEE_MASK_NOCLOSEPROCESS is the whole reason ShellExecuteExW is used instead
	// of the simpler ShellExecuteW: it fills in hProcess, which is the only way to
	// learn when an elevated child exits. A "runas" launch crosses an integrity
	// boundary, so the process is not our child and os/exec can never see it.
	seeMaskNoCloseProcess = 0x00000040
	// SEE_MASK_NOASYNC keeps the call valid for a caller that may return before
	// the shell has finished with the arguments we passed it.
	seeMaskNoAsync = 0x00000100

	swShowNormal = 1
)

// shellExecuteInfo mirrors SHELLEXECUTEINFOW.
type shellExecuteInfo struct {
	cbSize         uint32
	fMask          uint32
	hwnd           windows.HWND
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       windows.Handle
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      windows.Handle
	dwHotKey       uint32
	hIconOrMonitor windows.Handle
	hProcess       windows.Handle
}

// launchElevated starts exePath with the "runas" verb, i.e. behind a UAC prompt.
//
// Both things the launcher spawns need this. The injector writes into another
// process's address space, which fails outright without administrator rights when
// the game is elevated; Mephi talks to the injected DLL and has to match.
func launchElevated(exePath, workDir string) (*LaunchedProcess, error) {
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return nil, err
	}
	file, err := windows.UTF16PtrFromString(exePath)
	if err != nil {
		return nil, err
	}
	var dir *uint16
	if workDir != "" {
		if dir, err = windows.UTF16PtrFromString(workDir); err != nil {
			return nil, err
		}
	}

	info := shellExecuteInfo{
		fMask:       seeMaskNoCloseProcess | seeMaskNoAsync,
		lpVerb:      verb,
		lpFile:      file,
		lpDirectory: dir,
		nShow:       swShowNormal,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))

	ret, _, callErr := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		// Only read on failure: Call returns the thread's last error
		// unconditionally, which reads as "The operation completed successfully"
		// after a launch that worked.
		return nil, elevationError(exePath, callErr)
	}

	if info.hProcess == 0 {
		// Documented as filled in when SEE_MASK_NOCLOSEPROCESS is set, but the
		// shell can hand an activation to an already-running instance instead of
		// starting one, and then there is no handle. Not an error — the caller
		// falls back to matching on the image name.
		return &LaunchedProcess{}, nil
	}

	handle := info.hProcess
	pid, err := windows.GetProcessId(handle)
	if err != nil {
		pid = 0
	}

	return &LaunchedProcess{
		PID: pid,
		// The handle is closed here rather than by the caller because Wait is the
		// only thing that uses it, and there is exactly one waiter.
		wait: func() {
			defer windows.CloseHandle(handle)
			_, _ = windows.WaitForSingleObject(handle, windows.INFINITE)
		},
	}, nil
}

// elevationError classifies a failed ShellExecuteExW.
//
// ERROR_CANCELLED is the whole point of doing this separately: it is not a fault
// but the user answering "no" to the prompt we asked for, and it has to come back
// as something callers can branch on rather than as one more opaque errno.
func elevationError(exePath string, callErr error) error {
	var errno syscall.Errno
	if errors.As(callErr, &errno) && errno == syscall.Errno(windows.ERROR_CANCELLED) {
		return ErrElevationCancelled
	}
	return fmt.Errorf("couldn't start %s: %v", filepath.Base(exePath), callErr)
}

// isProcessRunning reports whether any process is running under this image name.
func isProcessRunning(name string) bool {
	_, err := FindProcessID(name)
	return err == nil
}
