package limbonia

import (
	"errors"
	"fmt"
	"strings"
)

// ErrElevationCancelled reports that the user dismissed the UAC prompt.
//
// Windows only — nothing off Windows elevates anything, see launchElevated there.
// The injector and Mephi are elevated per launch rather than the launcher marking
// itself requireAdministrator: an always-elevated launcher would prompt on every
// start, run its whole UI as administrator, and refuse to be started by the
// Windows startup folder. The cost of asking per launch is that "no" is a normal
// answer, so it has to read as a declined permission rather than as a crash or a
// button that did nothing.
var ErrElevationCancelled = errors.New("administrator permission was declined")

// ErrLimbusRunning reports that the game is already open.
//
// Windows only, and about the injector specifically: it STARTS Limbus Company —
// injector.cfg tells it which executable to launch — so pressing Open with the
// game already up does not patch the copy that is running. It either fails or
// starts a second one, and the user is left playing an unpatched game wondering
// why nothing happened. Refusing early, by name, is the only honest answer.
//
// The message is the whole error rather than a prefix, because it is shown to the
// user verbatim.
var ErrLimbusRunning = errors.New("Limbus Company is already running — close the game first, then press Open")

// limbusProcessName is the game's image name.
//
// Fixed rather than derived from the configured folder: OpenFileDialog only
// accepts a path ending in this name, so the two cannot disagree, and a launcher
// with no folder set yet must still be able to answer the question.
const limbusProcessName = "LimbusCompany.exe"

// IsLimbusRunning reports whether the game is already open.
//
// Windows only, and gated explicitly rather than left to fall out of a process
// lookup that happens to answer false. Off Windows there is no injector at all —
// Limbonia is loaded by a Wine DLL override and STEAM starts the game — so an
// already-running copy is Steam's business rather than ours. Blocking there would
// refuse a launch that would have worked fine, and would do it under Proton where
// the process is named LimbusCompany.exe just the same. See gameLaunchGuard.
func IsLimbusRunning() bool {
	if !gameLaunchGuard {
		return false
	}
	return processLookup(limbusProcessName)
}

// commNameLimit is the kernel's TASK_COMM_LEN minus its NUL: /proc/<pid>/comm
// holds at most this many characters.
const commNameLimit = 15

// matchesCommName reports whether a name read from /proc/<pid>/comm is the image
// name we are looking for.
//
// The truncation is the trap. "Mephi" survives comm intact, so a naive comparison
// looks like it works — and then "LimbusCompany.exe" (17 characters) arrives as
// "LimbusCompany.e" and can never match, silently, forever. Lives here rather
// than in the Linux-only file so it can be tested on any host.
func matchesCommName(got, want string) bool {
	if strings.EqualFold(got, want) {
		return true
	}
	if len(want) > commNameLimit {
		return strings.EqualFold(got, want[:commNameLimit])
	}
	return false
}

// isPIDDir reports whether a /proc entry name is a process id.
//
// /proc is full of non-numeric entries (self, sys, net, …); reading comm out of
// each of them would be a wasted syscall per scan.
func isPIDDir(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// LaunchedProcess is a handle on something the launcher started.
//
// ShellExecuteExW hands back a process handle rather than an *os.Process, so
// there is nothing os/exec can wait on. PID is kept as well because the handle is
// not guaranteed — see CanWait — and a caller that has lost it can still match on
// the image name.
type LaunchedProcess struct {
	PID uint32

	// wait blocks until the process exits. It is nil when the platform gave us
	// nothing to wait on, which is why callers must check CanWait rather than
	// assume Wait means anything.
	wait func()
}

// CanWait reports whether Wait will actually block on the process exiting.
func (p *LaunchedProcess) CanWait() bool {
	return p != nil && p.wait != nil
}

// Wait blocks until the process exits, returning at once when there is nothing
// to wait on.
func (p *LaunchedProcess) Wait() {
	if !p.CanWait() {
		return
	}
	p.wait()
}

// elevationRefused turns a declined UAC prompt into something aimed at the user.
//
// The sentinel on its own says what happened but not what to do about it, and
// "administrator permission was declined" shown as a bare toast reads like a
// failure of the launcher rather than the answer the user just gave. Anything
// that is not a refusal passes straight through untouched.
func elevationRefused(what string, err error) error {
	if !errors.Is(err, ErrElevationCancelled) {
		return err
	}
	return fmt.Errorf("%s couldn't start — %w. Accept the Windows permission prompt and try again", what, err)
}

// processLookup reports whether a process is running under the given image name.
//
// A variable rather than a direct call so tests can stand in a known process
// table: the real one is platform-specific and answers about whatever happens to
// be running on the machine, which is not something a test can arrange.
var processLookup = isProcessRunning

// IsMephiRunning reports whether a copy of the companion is already up.
//
// Matching is by image name, so it also sees an instance this launcher did not
// start — a Mephi left running from before the launcher was restarted, or one
// started by hand. That is the point: the decisions that hang off this (don't
// start a second one; stay hidden) are about the machine's state, not about
// whose child process it is.
func IsMephiRunning() bool {
	return processLookup(mephiProcessName())
}
