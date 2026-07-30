package main

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"WailsTest/limbonia"
)

// fakePresence records the hide/restore transitions without touching the
// notification area or a real window.
type fakePresence struct {
	mu sync.Mutex

	trayShown  int
	trayHidden int
	hides      int
	shows      int

	// trayFails simulates a notification area that won't take the icon, which is
	// the case where the launcher must refuse to hide.
	trayFails bool
}

func (f *fakePresence) build() *presence {
	return &presence{
		trayShow: func() bool {
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.trayFails {
				return false
			}
			f.trayShown++
			return true
		},
		trayHide:   func() { f.mu.Lock(); f.trayHidden++; f.mu.Unlock() },
		windowHide: func() { f.mu.Lock(); f.hides++; f.mu.Unlock() },
		windowShow: func() { f.mu.Lock(); f.shows++; f.mu.Unlock() },
	}
}

func (f *fakePresence) counts() (trayShown, trayHidden, hides, shows int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.trayShown, f.trayHidden, f.hides, f.shows
}

// ── Requirement: an already-running companion means Play only injects ─────────

// testApp builds an App with every side effect replaced, so the launch decision
// can be exercised without a game, an injector or a UAC prompt.
func testApp(mephiRunning bool) (*App, *int, *int) {
	injected, opened := 0, 0

	app := &App{
		mephiPoll:      time.Millisecond,
		mephiAppear:    10 * time.Millisecond,
		mephiRunningFn: func() bool { return mephiRunning },
		injectFn:       func() error { injected++; return nil },
		openMephiFn:    func() error { opened++; return nil },
		quitFn:         func() {},
	}
	return app, &injected, &opened
}

// refusedBy builds the error a launch path actually returns when the user
// dismisses the UAC prompt — the sentinel as it arrives, wrapped.
func refusedBy(what string) error {
	return fmt.Errorf("%s couldn't start — %w. Accept the Windows permission prompt and try again", what, limbonia.ErrElevationCancelled)
}

// declineApp wires an App whose given launch step comes back refused, and
// returns a counter for the quits it performs.
func declineApp(injectErr, mephiErr error) (*App, *fakePresence, *int) {
	quits := 0
	fake := &fakePresence{}

	app := &App{
		presence:       fake.build(),
		mephiPoll:      time.Millisecond,
		mephiAppear:    10 * time.Millisecond,
		mephiRunningFn: func() bool { return false },
		injectFn:       func() error { return injectErr },
		openMephiFn:    func() error { return mephiErr },
		quitFn:         func() { quits++ },
	}
	return app, fake, &quits
}

func TestPlayStartsCompanionWhenNoneIsRunning(t *testing.T) {
	app, injected, opened := testApp(false)

	if err := app.InjectLimbonia(); err != nil {
		t.Fatalf("Play should succeed: %v", err)
	}

	if *injected != 1 {
		t.Errorf("the game should have been patched once, got %d", *injected)
	}
	if *opened != 1 {
		t.Errorf("the companion should have been started once, got %d", *opened)
	}
}

// Two companions would fight over the same control pipe and the same settings
// file. With one already up, Play is the injection and nothing else — the running
// instance picks the patched game up by itself.
func TestPlayOnlyInjectsWhenCompanionIsAlreadyRunning(t *testing.T) {
	app, injected, opened := testApp(true)

	if err := app.InjectLimbonia(); err != nil {
		t.Fatalf("Play should succeed: %v", err)
	}

	if *injected != 1 {
		t.Errorf("the game should still have been patched once, got %d", *injected)
	}
	if *opened != 0 {
		t.Errorf("a second companion must not be started, got %d launches", *opened)
	}
}

// The check has to happen before the injection, not after: injecting is what
// makes the game interesting to a companion, and a Mephi that reacted to it in
// between would be double-counted.
func TestPlayDecidesBeforeInjecting(t *testing.T) {
	var order []string

	app := &App{
		mephiRunningFn: func() bool { order = append(order, "check"); return false },
		injectFn:       func() error { order = append(order, "inject"); return nil },
		openMephiFn:    func() error { order = append(order, "open"); return nil },
	}

	if err := app.InjectLimbonia(); err != nil {
		t.Fatalf("Play should succeed: %v", err)
	}

	want := []string{"check", "inject", "open"}
	if len(order) != len(want) {
		t.Fatalf("got %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("got %v, want %v", order, want)
		}
	}
}

func TestPlayReportsInjectionFailureAndSkipsCompanion(t *testing.T) {
	boom := errors.New("the game isn't running")
	opened := 0

	app := &App{
		mephiRunningFn: func() bool { return false },
		injectFn:       func() error { return boom },
		openMephiFn:    func() error { opened++; return nil },
	}

	if err := app.InjectLimbonia(); !errors.Is(err, boom) {
		t.Errorf("an injection failure should be reported, got %v", err)
	}
	if opened != 0 {
		t.Error("the companion should not be started when the injection failed")
	}
}

// ── Requirement: Open is refused while the game is already running ───────────

// The injector is what starts Limbus Company, so with a copy already open there
// is nothing here that can reach it — it would fail or start a second one, and
// the user would carry on playing unpatched.
func TestPlayRefusedWhileGameIsRunning(t *testing.T) {
	injected, opened := 0, 0
	app := &App{
		limbusRunningFn: func() bool { return true },
		mephiRunningFn:  func() bool { return false },
		injectFn:        func() error { injected++; return nil },
		openMephiFn:     func() error { opened++; return nil },
		quitFn:          func() {},
	}

	err := app.InjectLimbonia()

	if !errors.Is(err, limbonia.ErrLimbusRunning) {
		t.Fatalf("Open should be refused while the game is running, got %v", err)
	}
	if injected != 0 {
		t.Error("nothing should be injected while the game is already running")
	}
	if opened != 0 {
		t.Error("the companion should not be started either")
	}
}

// The check has to come first: injecting or launching anything before it would
// be doing exactly the thing the refusal exists to prevent.
func TestGameCheckHappensBeforeAnythingElse(t *testing.T) {
	var order []string
	app := &App{
		limbusRunningFn: func() bool { order = append(order, "game"); return false },
		mephiRunningFn:  func() bool { order = append(order, "mephi"); return false },
		injectFn:        func() error { order = append(order, "inject"); return nil },
		openMephiFn:     func() error { order = append(order, "open"); return nil },
		quitFn:          func() {},
	}

	if err := app.InjectLimbonia(); err != nil {
		t.Fatalf("Play should succeed: %v", err)
	}

	if len(order) == 0 || order[0] != "game" {
		t.Errorf("the game check must come first, got %v", order)
	}
}

// Closing the game must free the button up again, with no restart in between.
func TestPlayWorksOnceTheGameIsClosed(t *testing.T) {
	running := true
	injected := 0
	app := &App{
		limbusRunningFn: func() bool { return running },
		mephiRunningFn:  func() bool { return false },
		injectFn:        func() error { injected++; return nil },
		openMephiFn:     func() error { return nil },
		quitFn:          func() {},
	}

	if err := app.InjectLimbonia(); !errors.Is(err, limbonia.ErrLimbusRunning) {
		t.Fatalf("the first attempt should be refused, got %v", err)
	}

	running = false

	if err := app.InjectLimbonia(); err != nil {
		t.Fatalf("the second attempt should succeed, got %v", err)
	}
	if injected != 1 {
		t.Errorf("the game should have been patched once, got %d", injected)
	}
}

// Being refused is not a fault, and must not close the launcher — the user is
// meant to go and close the game and come back to this window.
func TestGameRunningDoesNotQuitTheLauncher(t *testing.T) {
	quits := 0
	app := &App{
		limbusRunningFn: func() bool { return true },
		mephiRunningFn:  func() bool { return false },
		injectFn:        func() error { return nil },
		openMephiFn:     func() error { return nil },
		quitFn:          func() { quits++ },
	}

	app.InjectLimbonia()

	if quits != 0 {
		t.Errorf("a running game must not close the launcher, got %d quits", quits)
	}
}

// IsGameRunning is what greys the button out. An unwired launcher must report
// "not running" rather than block Open for a reason that isn't true.
func TestIsGameRunningReflectsTheLookup(t *testing.T) {
	app := &App{limbusRunningFn: func() bool { return true }}
	if !app.IsGameRunning() {
		t.Error("IsGameRunning should follow the lookup")
	}

	if (&App{}).IsGameRunning() {
		t.Error("an unwired launcher must not claim the game is running")
	}
}

// ── Requirement: a declined UAC prompt closes the launcher ───────────────────

// Refusing the prompt is a decision about the session, not about one button:
// neither the injector nor the companion works unelevated, so the launcher has
// nothing left to offer and closes.
func TestDeclinedElevationOnTheCompanionQuits(t *testing.T) {
	app, _, quits := declineApp(nil, refusedBy("Mephi"))

	err := app.InjectLimbonia()

	if !errors.Is(err, limbonia.ErrElevationCancelled) {
		t.Fatalf("a declined prompt should still reach the caller, got %v", err)
	}
	if *quits != 1 {
		t.Errorf("the launcher should close exactly once, got %d quits", *quits)
	}
	if !app.quitting.Load() {
		t.Error("the launcher should have marked itself as shutting down")
	}
}

// The injector is elevated too, and it is asked first — a refusal there must
// close the launcher just the same, and must not go on to start the companion.
func TestDeclinedElevationOnTheInjectorQuits(t *testing.T) {
	opened := 0
	app, _, quits := declineApp(refusedBy("The injector"), nil)
	app.openMephiFn = func() error { opened++; return nil }

	err := app.InjectLimbonia()

	if !errors.Is(err, limbonia.ErrElevationCancelled) {
		t.Fatalf("a declined prompt should still reach the caller, got %v", err)
	}
	if *quits != 1 {
		t.Errorf("the launcher should close exactly once, got %d quits", *quits)
	}
	if opened != 0 {
		t.Error("the companion must not be started after the injector was refused")
	}
}

// Both halves can come back refused from a single "No", and two quits would mean
// two shutdowns racing each other.
func TestRepeatedRefusalsQuitOnlyOnce(t *testing.T) {
	app, _, quits := declineApp(nil, refusedBy("Mephi"))

	app.InjectLimbonia()
	app.InjectLimbonia()
	app.declineShutdown()

	if *quits != 1 {
		t.Errorf("the launcher should close exactly once, got %d quits", *quits)
	}
}

// Quitting has to take the icon down on the way out. Leaving it behind is a ghost
// that sits in the notification area doing nothing until the user hovers over it.
func TestQuittingRestoresTheWindowAndRemovesTheIcon(t *testing.T) {
	app, fake, quits := declineApp(nil, refusedBy("Mephi"))

	// Pretend a previous launch had already hidden the launcher away.
	if !app.presence.hide() {
		t.Fatal("setup: the launcher should have hidden")
	}

	app.InjectLimbonia()

	_, trayHidden, _, shows := fake.counts()
	if shows != 1 {
		t.Errorf("the window should be visible before the app shuts down, got %d shows", shows)
	}
	if trayHidden != 1 {
		t.Errorf("the tray icon should be taken down, got %d removals", trayHidden)
	}
	if *quits != 1 {
		t.Errorf("the launcher should close exactly once, got %d quits", *quits)
	}
}

// A genuine fault is something the user may be able to fix from the window that
// is still open, so only the deliberate refusal is fatal.
func TestRealFailuresDoNotQuit(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"file missing", errors.New("Injector.exe isn't installed")},
		{"access denied", errors.New("couldn't start Injector.exe: Access is denied")},
		{"elevation required", errors.New("couldn't start Injector.exe: ERROR_ELEVATION_REQUIRED")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			app, _, quits := declineApp(c.err, nil)

			if err := app.InjectLimbonia(); err == nil {
				t.Error("a real failure should still be reported")
			}
			if *quits != 0 {
				t.Errorf("the launcher should stay open on a real failure, got %d quits", *quits)
			}
			if app.quitting.Load() {
				t.Error("a real failure must not mark the launcher as shutting down")
			}
		})
	}
}

// Anything else about the companion stays non-fatal AND non-fatal to the window:
// the injection is the part that had to work, and it did.
func TestPlaySwallowsOtherCompanionFailures(t *testing.T) {
	app, _, quits := declineApp(nil, errors.New("Mephi is not installed"))

	if err := app.InjectLimbonia(); err != nil {
		t.Errorf("a companion problem must not be reported as an injection failure, got %v", err)
	}
	if *quits != 0 {
		t.Errorf("a companion problem must not close the launcher, got %d quits", *quits)
	}
}

// The watcher's restore is deferred, so a companion that exits while the launcher
// is shutting down would otherwise put the window back up mid-teardown.
func TestWatcherDoesNotRestoreIntoAShutdown(t *testing.T) {
	fake := &fakePresence{}
	app := &App{
		presence:       fake.build(),
		mephiPoll:      time.Millisecond,
		mephiAppear:    10 * time.Millisecond,
		mephiRunningFn: func() bool { return false },
		quitFn:         func() {},
	}
	app.presence.hide()

	exited := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		app.watchUntilGone(true, func() { <-exited })
	}()

	// The refusal lands first and takes the window back itself.
	app.declineShutdown()
	beforeWatcher := func() int { _, _, _, shows := fake.counts(); return shows }()

	close(exited)
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the watcher never finished")
	}

	if got := func() int { _, _, _, shows := fake.counts(); return shows }(); got != beforeWatcher {
		t.Errorf("the watcher restored the window during shutdown (%d shows, was %d)", got, beforeWatcher)
	}
}

// Nor may a companion launched as the launcher is closing drag the window into a
// tray icon that the shutdown is about to delete.
func TestNothingHidesOnceQuitting(t *testing.T) {
	fake := &fakePresence{}
	app := &App{
		presence:       fake.build(),
		mephiPoll:      time.Millisecond,
		mephiAppear:    time.Millisecond,
		mephiRunningFn: func() bool { return false },
		quitFn:         func() {},
	}

	app.declineShutdown()
	app.mephiStarted(&limbonia.LaunchedProcess{PID: 4321})

	trayShown, _, hides, _ := fake.counts()
	if trayShown != 0 || hides != 0 {
		t.Errorf("a shutting-down launcher must not hide, got icon=%d hide=%d", trayShown, hides)
	}
	if app.mephiWatch.Load() {
		t.Error("no watcher should have been started during shutdown")
	}
}

// A zero App must refuse rather than panic — it is reachable from the bound
// frontend the moment anything constructs one without NewApp.
func TestPlayWithoutWiringFailsCleanly(t *testing.T) {
	app := &App{}

	if err := app.InjectLimbonia(); err == nil {
		t.Error("an unwired launcher should report a problem rather than silently do nothing")
	}
}

func TestNewAppIsFullyWired(t *testing.T) {
	app := NewApp()

	if app.injectFn == nil || app.openMephiFn == nil || app.mephiRunningFn == nil {
		t.Error("NewApp must wire the real launch behaviour")
	}
	// Without this a refused prompt would set the shutdown flag and then never
	// actually close, leaving a launcher whose buttons all silently do nothing.
	if app.quitFn == nil {
		t.Error("NewApp must wire the quit path")
	}
	if app.mephiPoll <= 0 || app.mephiAppear <= 0 {
		t.Error("NewApp must give the watcher usable intervals")
	}
}

// ── Requirement: hide to the tray, restore on exit ───────────────────────────

func TestHideRaisesIconBeforeHidingWindow(t *testing.T) {
	fake := &fakePresence{}
	p := fake.build()

	if !p.hide() {
		t.Fatal("hiding should succeed when the tray accepts the icon")
	}

	trayShown, _, hides, _ := fake.counts()
	if trayShown != 1 || hides != 1 {
		t.Errorf("want one icon and one hide, got icon=%d hide=%d", trayShown, hides)
	}
}

// Without an icon there is no way back to a hidden window, so the window must
// stay where the user can see it.
func TestHideRefusesWhenTrayIsUnavailable(t *testing.T) {
	fake := &fakePresence{trayFails: true}
	p := fake.build()

	if p.hide() {
		t.Error("hiding should fail when no tray icon could be created")
	}

	_, _, hides, _ := fake.counts()
	if hides != 0 {
		t.Error("the window must not be hidden when there is no way to restore it")
	}
}

func TestRestoreShowsWindowAndRemovesIcon(t *testing.T) {
	fake := &fakePresence{}
	p := fake.build()

	p.hide()
	p.restore()

	_, trayHidden, _, shows := fake.counts()
	if shows != 1 || trayHidden != 1 {
		t.Errorf("want one show and one icon removal, got show=%d iconRemoved=%d", shows, trayHidden)
	}
}

// Restore lands here from two directions — the companion exiting and the tray's
// own Show item — so it has to be safe to call twice.
func TestRestoreIsIdempotent(t *testing.T) {
	fake := &fakePresence{}
	p := fake.build()

	p.hide()
	p.restore()
	p.restore()
	p.restore()

	_, _, _, shows := fake.counts()
	if shows != 1 {
		t.Errorf("the window should be restored exactly once, got %d", shows)
	}
}

// A user who clicked Show while the companion was still running must not have the
// window yanked to the front again when it finally closes.
func TestRestoreDoesNothingWhenNeverHidden(t *testing.T) {
	fake := &fakePresence{}
	p := fake.build()

	p.restore()

	_, trayHidden, _, shows := fake.counts()
	if shows != 0 || trayHidden != 0 {
		t.Errorf("restoring a visible window should do nothing, got show=%d iconRemoved=%d", shows, trayHidden)
	}
}

func TestHideIsIdempotent(t *testing.T) {
	fake := &fakePresence{}
	p := fake.build()

	p.hide()
	p.hide()

	trayShown, _, hides, _ := fake.counts()
	if trayShown != 1 || hides != 1 {
		t.Errorf("a second hide should be a no-op, got icon=%d hide=%d", trayShown, hides)
	}
}

func TestNilPresenceIsSafe(t *testing.T) {
	var p *presence

	if p.hide() {
		t.Error("a launcher with no tray must never report itself hidden")
	}
	p.restore()
}

// startup builds the presence out of method values on the tray, which is nil
// whenever the notification area is unavailable — every non-Windows build, and a
// Windows one whose window class failed to register. Binding a method to a nil
// receiver is legal; calling it has to be too, and the answer has to be "no",
// because the alternative is a hidden window nobody can bring back.
func TestPresenceOverANilTrayRefusesToHide(t *testing.T) {
	var tray *trayIcon
	hidden := false

	p := &presence{
		trayShow:   tray.show,
		trayHide:   tray.hide,
		windowHide: func() { hidden = true },
		windowShow: func() {},
	}

	if p.hide() {
		t.Error("there is no tray, so hiding must fail")
	}
	if hidden {
		t.Error("the window must stay visible when there is no icon to restore it")
	}

	// And the reverse direction must not blow up either, since the tray menu is
	// wired to it unconditionally.
	p.restore()
}

// ── Requirement: come back when the companion exits ──────────────────────────

// The handle path: the launcher stays hidden for exactly as long as the process
// it is waiting on lives, and comes back once — not on a timer, and not early.
func TestWatcherRestoresOnceWhenProcessExits(t *testing.T) {
	fake := &fakePresence{}
	app := &App{
		presence:    fake.build(),
		mephiPoll:   time.Millisecond,
		mephiAppear: 10 * time.Millisecond,
		// The handle is authoritative, so the process table is never consulted.
		mephiRunningFn: func() bool { return false },
	}
	app.presence.hide()

	exited := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		app.watchUntilGone(true, func() { <-exited })
	}()

	// Still alive: the launcher must stay away.
	time.Sleep(20 * time.Millisecond)
	if _, _, _, shows := fake.counts(); shows != 0 {
		t.Fatalf("the launcher came back while the companion was still running (%d shows)", shows)
	}

	close(exited)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the watcher never noticed the companion exiting")
	}

	_, trayHidden, _, shows := fake.counts()
	if shows != 1 {
		t.Errorf("the launcher should come back exactly once, got %d", shows)
	}
	if trayHidden != 1 {
		t.Errorf("the tray icon should be taken down once, got %d", trayHidden)
	}
}

// The no-handle path: with nothing to wait on, the launcher polls the process
// table and must not come back until the name stops answering.
func TestWatcherPollsWhenThereIsNoHandle(t *testing.T) {
	fake := &fakePresence{}

	var mu sync.Mutex
	running := true
	alive := func() bool {
		mu.Lock()
		defer mu.Unlock()
		return running
	}

	app := &App{
		presence:       fake.build(),
		mephiPoll:      time.Millisecond,
		mephiAppear:    time.Second,
		mephiRunningFn: alive,
	}
	app.presence.hide()

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.watchUntilGone(false, nil)
	}()

	time.Sleep(20 * time.Millisecond)
	if _, _, _, shows := fake.counts(); shows != 0 {
		t.Fatal("the launcher came back while the companion was still in the process table")
	}

	mu.Lock()
	running = false
	mu.Unlock()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the watcher never noticed the companion leaving the process table")
	}

	if _, _, _, shows := fake.counts(); shows != 1 {
		t.Errorf("the launcher should come back exactly once, got %d", shows)
	}
}

// A companion started without a usable handle needs a moment to appear before its
// absence is believed, or the launcher hides and immediately un-hides.
func TestWatcherWaitsForACompanionThatIsSlowToAppear(t *testing.T) {
	fake := &fakePresence{}

	var mu sync.Mutex
	appeared, gone := false, false
	app := &App{
		presence:    fake.build(),
		mephiPoll:   time.Millisecond,
		mephiAppear: 2 * time.Second,
		mephiRunningFn: func() bool {
			mu.Lock()
			defer mu.Unlock()
			return appeared && !gone
		},
	}
	app.presence.hide()

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.watchUntilGone(false, nil)
	}()

	// Not in the process table yet — the launcher must not read that as "exited".
	time.Sleep(30 * time.Millisecond)
	if _, _, _, shows := fake.counts(); shows != 0 {
		t.Fatal("the launcher came back before the companion had even started")
	}

	mu.Lock()
	appeared = true
	mu.Unlock()
	time.Sleep(20 * time.Millisecond)
	if _, _, _, shows := fake.counts(); shows != 0 {
		t.Fatal("the launcher came back while the companion was running")
	}

	mu.Lock()
	gone = true
	mu.Unlock()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the watcher never finished")
	}

	if _, _, _, shows := fake.counts(); shows != 1 {
		t.Errorf("the launcher should come back exactly once, got %d", shows)
	}
}

// Two Play presses must not leave two watchers racing to restore the window.
func TestOnlyOneWatcherRunsAtATime(t *testing.T) {
	fake := &fakePresence{}
	app := &App{
		presence:       fake.build(),
		mephiPoll:      time.Millisecond,
		mephiAppear:    10 * time.Millisecond,
		mephiRunningFn: func() bool { return false },
	}
	app.presence.hide()

	first := make(chan struct{})
	started := make(chan struct{})
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		app.watchUntilGone(true, func() { close(started); <-first })
	}()
	<-started

	// The second watcher should bow out immediately rather than restore the
	// window out from under the first.
	app.watchUntilGone(true, func() { t.Error("a second watcher should not wait on anything") })

	if _, _, _, shows := fake.counts(); shows != 0 {
		t.Fatalf("the second watcher restored the window early (%d shows)", shows)
	}

	close(first)
	<-firstDone
}

// mephiStarted is the seam the limbonia package calls into. It must not spawn a
// watcher when the window never went away, or the launcher would "restore" itself
// on top of whatever the user is doing.
func TestMephiStartedDoesNotWatchWhenItCouldNotHide(t *testing.T) {
	fake := &fakePresence{trayFails: true}
	app := &App{
		presence:       fake.build(),
		mephiPoll:      time.Millisecond,
		mephiAppear:    time.Millisecond,
		mephiRunningFn: func() bool { return false },
	}

	app.mephiStarted(&limbonia.LaunchedProcess{PID: 4321})

	time.Sleep(20 * time.Millisecond)
	if _, _, _, shows := fake.counts(); shows != 0 {
		t.Errorf("nothing should be restored when nothing was hidden, got %d shows", shows)
	}
	if app.mephiWatch.Load() {
		t.Error("no watcher should have been started")
	}
}

func TestMephiStartedHidesAndWatches(t *testing.T) {
	fake := &fakePresence{}
	app := &App{
		presence:       fake.build(),
		mephiPoll:      time.Millisecond,
		mephiAppear:    10 * time.Millisecond,
		mephiRunningFn: func() bool { return false },
	}

	// No handle and nothing in the process table: the watcher gives up after
	// mephiAppear and restores, which is the shape of the whole cycle.
	app.mephiStarted(&limbonia.LaunchedProcess{PID: 4321})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, _, shows := fake.counts(); shows == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}

	trayShown, trayHidden, hides, shows := fake.counts()
	if trayShown != 1 || hides != 1 {
		t.Errorf("the launcher should have hidden to the tray, got icon=%d hide=%d", trayShown, hides)
	}
	if shows != 1 || trayHidden != 1 {
		t.Errorf("the launcher should have come back, got show=%d iconRemoved=%d", shows, trayHidden)
	}
}
