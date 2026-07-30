package limbonia

import (
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// withProcessLookup stands a known process table in for the real one, so the
// "is the companion already up?" question has an answer a test can arrange.
func withProcessLookup(t *testing.T, running ...string) *[]string {
	t.Helper()

	asked := []string{}
	original := processLookup
	processLookup = func(name string) bool {
		asked = append(asked, name)
		for _, r := range running {
			if strings.EqualFold(r, name) {
				return true
			}
		}
		return false
	}
	t.Cleanup(func() { processLookup = original })

	return &asked
}

// The image name is what the whole detection hangs off: get it wrong and
// IsMephiRunning answers "no" forever, which silently turns every launch into a
// second companion.
func TestMephiProcessName(t *testing.T) {
	want := "Mephi"
	if runtime.GOOS == "windows" {
		want = "Mephi.exe"
	}
	if got := mephiProcessName(); got != want {
		t.Errorf("mephiProcessName() = %q, want %q", got, want)
	}
	if got := mephiProcessName(); got != filepath.Base(mephiPath()) {
		t.Errorf("the name being matched (%q) must be the file being launched (%q)", got, mephiPath())
	}
}

func TestIsMephiRunningDetectsCompanion(t *testing.T) {
	asked := withProcessLookup(t, mephiProcessName())

	if !IsMephiRunning() {
		t.Error("a running companion should be detected")
	}
	if len(*asked) != 1 || (*asked)[0] != mephiProcessName() {
		t.Errorf("looked up %v, want a single lookup of %q", *asked, mephiProcessName())
	}
}

func TestIsMephiRunningIgnoresOtherProcesses(t *testing.T) {
	// Notably including the injector and the game: neither means the companion is
	// up, and treating them as if it were would suppress the launch entirely.
	withProcessLookup(t, "Injector.exe", "LimbusCompany.exe", "explorer.exe")

	if IsMephiRunning() {
		t.Error("unrelated processes must not be mistaken for the companion")
	}
}

// The injector STARTS the game, so an already-open copy is not something it can
// patch. Detection is by the same fixed image name the file dialog enforces.
func TestIsLimbusRunningDetectsTheGame(t *testing.T) {
	if !gameLaunchGuard {
		t.Skip("the already-running guard is Windows-only by design — Steam owns the launch elsewhere")
	}
	asked := withProcessLookup(t, "LimbusCompany.exe")

	if !IsLimbusRunning() {
		t.Error("a running game should be detected")
	}
	if len(*asked) != 1 || (*asked)[0] != "LimbusCompany.exe" {
		t.Errorf("looked up %v, want a single lookup of LimbusCompany.exe", *asked)
	}
}

// Case is not something a user or the process table guarantees.
func TestIsLimbusRunningIgnoresCase(t *testing.T) {
	if !gameLaunchGuard {
		t.Skip("the already-running guard is Windows-only by design")
	}
	withProcessLookup(t, "limbuscompany.exe")

	if !IsLimbusRunning() {
		t.Error("the game should be detected regardless of case")
	}
}

// Off Windows the guard must not merely happen to be false — it must be off by
// construction, because there the game IS legitimately started while running and
// Steam handles it.
func TestLimbusGuardIsWindowsOnly(t *testing.T) {
	if gameLaunchGuard {
		t.Skip("the guard is on, which is the Windows expectation")
	}
	// Even with the game plainly in the process table, Open must not be blocked.
	withProcessLookup(t, "LimbusCompany.exe", "LimbusCompany.e")

	if IsLimbusRunning() {
		t.Error("Steam owns the launch here, so a running game must not block Open")
	}
}

func TestIsLimbusRunningIgnoresOtherProcesses(t *testing.T) {
	// The companion and the injector are both expected to be around at various
	// points and neither means the game is up.
	withProcessLookup(t, "Mephi.exe", "Injector.exe", "steam.exe")

	if IsLimbusRunning() {
		t.Error("unrelated processes must not be mistaken for the game")
	}
}

// The refusal is shown to the user verbatim, so it has to name the game and say
// what to do rather than describe an internal state.
func TestLimbusRunningErrorIsActionable(t *testing.T) {
	msg := ErrLimbusRunning.Error()

	if !strings.Contains(msg, "Limbus Company") {
		t.Errorf("the message should name the game, got %q", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "close") {
		t.Errorf("the message should tell the user to close it, got %q", msg)
	}
}

// ── /proc name matching (drives the Linux process lookup) ────────────────────

// /proc/<pid>/comm truncates at 15 characters. "Mephi" survives it intact, so a
// naive comparison looks like it works — and then "LimbusCompany.exe" arrives as
// "LimbusCompany.e" and can never match, silently, forever.
func TestMatchesCommNameHandlesTruncation(t *testing.T) {
	cases := []struct {
		got, want string
		match     bool
	}{
		// Short enough to survive comm intact.
		{"Mephi", "Mephi", true},
		{"mephi", "Mephi", true},
		// 17 characters, so the kernel only kept 15 of them.
		{"LimbusCompany.e", "LimbusCompany.exe", true},
		{"LimbusCompany.exe", "LimbusCompany.exe", true},
		// Near misses must not match.
		{"Mephisto", "Mephi", false},
		{"LimbusCompany", "LimbusCompany.exe", false},
		{"", "Mephi", false},
		{"steam", "Mephi", false},
	}

	for _, c := range cases {
		if got := matchesCommName(c.got, c.want); got != c.match {
			t.Errorf("matchesCommName(%q, %q) = %v, want %v", c.got, c.want, got, c.match)
		}
	}
}

// A short name must NOT be matched by prefix, or "Mephi" would match anything
// starting with it.
func TestMatchesCommNameDoesNotPrefixMatchShortNames(t *testing.T) {
	if matchesCommName("MephiOther", "Mephi") {
		t.Error("a short name must be compared whole, not as a prefix")
	}
}

// /proc is full of non-numeric entries; reading comm out of each would be a
// wasted syscall per scan, every couple of seconds.
func TestIsPIDDir(t *testing.T) {
	for _, ok := range []string{"1", "42", "999999"} {
		if !isPIDDir(ok) {
			t.Errorf("isPIDDir(%q) should be true", ok)
		}
	}
	for _, no := range []string{"", "self", "sys", "net", "1a", "a1", "cpuinfo", "-1"} {
		if isPIDDir(no) {
			t.Errorf("isPIDDir(%q) should be false", no)
		}
	}
}

// A declined UAC prompt has to arrive as something callers can branch on. If it
// degrades into a generic error, InjectLimbonia stops being able to tell the user
// why Play did nothing.
func TestElevationRefusedWrapsSentinel(t *testing.T) {
	err := elevationRefused("Mephi", ErrElevationCancelled)

	if !errors.Is(err, ErrElevationCancelled) {
		t.Fatalf("wrapping must preserve the sentinel, got %v", err)
	}

	msg := err.Error()
	if !strings.Contains(msg, "Mephi") {
		t.Errorf("the message should name what failed to start, got %q", msg)
	}
	// The user can only act on this if they're told what the action is.
	if !strings.Contains(strings.ToLower(msg), "prompt") {
		t.Errorf("the message should point at the permission prompt, got %q", msg)
	}
}

func TestElevationRefusedPassesOtherErrorsThrough(t *testing.T) {
	other := errors.New("the file is missing")

	got := elevationRefused("The injector", other)

	if !errors.Is(got, other) {
		t.Errorf("an unrelated failure should pass through unchanged, got %v", got)
	}
	if errors.Is(got, ErrElevationCancelled) {
		t.Error("an unrelated failure must not be reported as a declined prompt")
	}
}

func TestElevationRefusedToleratesNil(t *testing.T) {
	if err := elevationRefused("Mephi", nil); err != nil {
		t.Errorf("no failure should stay no failure, got %v", err)
	}
}

// A refusal nested inside another error still has to be recognisable, because
// that is how it reaches the app layer — wrapped once on the way out of
// OpenMephi and checked again in InjectLimbonia.
func TestElevationRefusedSurvivesNesting(t *testing.T) {
	nested := fmt.Errorf("starting the companion: %w", elevationRefused("Mephi", ErrElevationCancelled))

	if !errors.Is(nested, ErrElevationCancelled) {
		t.Errorf("a wrapped refusal must stay identifiable, got %v", nested)
	}
}

// CanWait is what decides between waiting on a handle and polling the process
// table. A LaunchedProcess with no handle claiming otherwise would park the
// watcher forever and leave the launcher hidden for good.
func TestLaunchedProcessWithoutHandleCannotWait(t *testing.T) {
	p := &LaunchedProcess{PID: 1234}

	if p.CanWait() {
		t.Error("a process with no handle must not claim it can be waited on")
	}
	p.Wait() // must return rather than block or panic
}

func TestNilLaunchedProcessIsSafe(t *testing.T) {
	var p *LaunchedProcess

	if p.CanWait() {
		t.Error("a nil process cannot be waited on")
	}
	p.Wait()
}

func TestLaunchedProcessWithHandleWaits(t *testing.T) {
	waited := false
	p := &LaunchedProcess{PID: 99, wait: func() { waited = true }}

	if !p.CanWait() {
		t.Fatal("a process with a wait function should report it can wait")
	}
	p.Wait()

	if !waited {
		t.Error("Wait should have blocked on the process")
	}
}
