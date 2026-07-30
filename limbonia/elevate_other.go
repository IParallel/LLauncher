//go:build !windows

package limbonia

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// gameLaunchGuard is off here: pressing Open with the game already running is not
// an error on this platform.
//
// On Windows the injector STARTS Limbus Company, so a copy that is already open
// is one it can never patch. There is no injector here at all — nothing is
// injected into anything, and Injector.exe is not even installed (it lives under
// windows/ in the bundle, which this platform's extractor skips; see
// updater.expectedFiles). Limbonia is loaded by a WINE DLL OVERRIDE instead:
// InjectLimbo copies winhttp.dll and Limbonia.dll into the game folder, sets
// Steam's launch options to WINEDLLOVERRIDES="winhttp=n,b", and hands
// steam://rungameid/… to xdg-open. Wine then loads the bundled winhttp.dll when
// the game starts, and that pulls Limbonia in.
//
// So Steam owns the launch and already handles "it's running". Refusing would
// block something that works, and would do it under Proton where the process is
// called LimbusCompany.exe just the same.
const gameLaunchGuard = false

// launchElevated starts the process with the user's own rights.
//
// Elevation is a Windows notion and nothing here wants it. On this platform the
// only caller is OpenMephi: the companion is the single thing the launcher
// spawns, because there is no injector here to spawn.
//
// Root is not merely unnecessary, it is harmful. Everything the launch path
// touches already belongs to the user — the Steam library folder the DLLs are
// copied into, the localconfig.vdf under ~/.local/share/Steam, and the launcher's
// own directory that Mephi runs from. Running Steam or Wine as root re-owns files
// inside the user's prefix and breaks the next unprivileged launch. Hence no
// pkexec and no sudo, deliberately.
func launchElevated(exePath, workDir string) (*LaunchedProcess, error) {
	cmd := exec.Command(exePath)
	cmd.Dir = workDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	return &LaunchedProcess{
		PID: uint32(cmd.Process.Pid),
		// Wait also reaps the child; without it every launch would leave a zombie
		// behind for the lifetime of the launcher.
		wait: func() { _ = cmd.Wait() },
	}, nil
}

// isProcessRunning reports whether any process is running under this image name.
//
// Reads /proc directly rather than shelling out to pgrep or pidof: neither is
// guaranteed to be installed, and the answer is wanted every couple of seconds,
// so spawning a process to get it would be absurd.
//
// This used to return a flat false, which quietly disabled the "don't start a
// second companion" rule on Linux — pressing Open twice launched two Mephis, both
// fighting over the same settings file.
func isProcessRunning(name string) bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		// No /proc at all (macOS, or a stripped container). False is the safe
		// answer: nothing gets blocked, and nothing waits on a process we have no
		// way to see.
		return false
	}

	for _, e := range entries {
		if !isPIDDir(e.Name()) {
			continue
		}
		comm, err := os.ReadFile(filepath.Join("/proc", e.Name(), "comm"))
		if err != nil {
			// Processes come and go while this loop runs. One that vanished
			// mid-scan is not an error, it is simply not a match.
			continue
		}
		if matchesCommName(strings.TrimSpace(string(comm)), name) {
			return true
		}
	}
	return false
}
