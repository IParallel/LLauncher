package main

import (
	"WailsTest/client"
	"WailsTest/config"
	"WailsTest/limbonia"
	"WailsTest/updater"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	// mephiPollInterval is how often the launcher re-checks for a companion it
	// has no process handle for. Slow on purpose: it walks the process table, and
	// the only thing waiting on the answer is a window coming back.
	mephiPollInterval = 2 * time.Second

	// mephiAppearWindow is how long a companion started without a usable handle
	// is given to show up in the process table before its absence is believed.
	mephiAppearWindow = 15 * time.Second
)

// App struct
type App struct {
	ctx         context.Context
	LimboniaApp *limbonia.LimboniaApp

	updateMu    sync.RWMutex
	updateState LauncherUpdate
	// restartPath is the launcher's own path once a downloaded update has been
	// swapped into place — i.e. what to execute to come back up on the new build.
	restartPath string

	// tray is nil when the notification area is unavailable (any non-Windows
	// build, or a failed registration). Nil means "never hide the window".
	tray *trayIcon
	// presence is the hide-to-tray state machine, built in startup() once the
	// Wails context and the tray both exist.
	presence *presence
	// mephiWatch guards against two Play presses leaving two watchers racing to
	// restore the window.
	mephiWatch atomic.Bool
	// quitting is set once the launcher has decided to close. It exists so a
	// watcher that finishes mid-shutdown can't put the window back up.
	quitting atomic.Bool

	mephiPoll   time.Duration
	mephiAppear time.Duration

	// The things Play actually does, held as indirections so the launch decision
	// and the shutdown-on-refusal can be exercised without a game, an injector, a
	// UAC prompt or a live Wails runtime. NewApp wires the real ones.
	injectFn        func() error
	openMephiFn     func() error
	mephiRunningFn  func() bool
	limbusRunningFn func() bool
	quitFn          func()
}

// presence is the launcher's hide-to-tray state machine.
//
// Kept apart from the Win32 and Wails calls it drives, because the ordering below
// is the part that can actually be got wrong, and it is not observable in a build
// that needs a notification area and a live window to run at all.
type presence struct {
	mu     sync.Mutex
	hidden bool

	trayShow   func() bool
	trayHide   func()
	windowHide func()
	windowShow func()
}

// hide puts the icon up and the window away, reporting whether it went.
//
// The order is load-bearing: the icon goes up FIRST and the window only follows
// if that worked. A hidden window with no icon to bring it back is a process the
// user can only reach through Task Manager, so a tray that won't appear means the
// launcher stays visible instead.
func (p *presence) hide() bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.hidden {
		return true
	}
	if !p.trayShow() {
		return false
	}
	p.windowHide()
	p.hidden = true
	return true
}

// restore brings the window back and takes the icon down.
//
// Idempotent, and deliberately a no-op when the window was never hidden: the
// companion exiting and the tray's own Show item both land here, and a user who
// already asked for the window back must not have it yanked to the front a second
// time when Mephi later closes.
func (p *presence) restore() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.hidden {
		return
	}
	p.windowShow()
	p.trayHide()
	p.hidden = false
}

// LauncherUpdate is the self-update state shown in the status bar.
//
// This replaces a blocking MessageDialog that used to appear the moment a
// download finished and was immediately followed by os.Exit — the user got a
// modal they had to dismiss, and then the app vanished. Reporting through the UI
// instead means the update never interrupts what they were doing, and restarting
// stays their decision.
type LauncherUpdate struct {
	// State is one of: "" (nothing happening), "downloading", "ready", "error".
	State   string `json:"state"`
	Version string `json:"version"`
	Percent int    `json:"percent"`
	// Message carries a user-facing reason when State is "error".
	Message string `json:"message"`
}

// emitLauncherUpdate records the state and pushes it to the frontend.
func (a *App) emitLauncherUpdate(s LauncherUpdate) {
	a.updateMu.Lock()
	a.updateState = s
	a.updateMu.Unlock()

	// ctx is nil until Wails calls startup(); dropping the event is fine because
	// the frontend pulls the current state on mount anyway.
	if a.ctx != nil {
		wailsruntime.EventsEmit(a.ctx, "launcher:update", s)
	}
}

// LauncherVersion is the running build's version, for the title bar.
//
// Read from the ldflags-stamped constant rather than the manifest: this is our
// OWN version, always known locally, and it must not disappear from the title
// bar just because the network is down.
func (a *App) LauncherVersion() string {
	return updater.CURRENT_LAUNCHER_VERSION
}

// LauncherUpdateState lets the frontend read the current state on mount, so a
// download that finished before the UI was listening still shows up.
func (a *App) LauncherUpdateState() LauncherUpdate {
	a.updateMu.RLock()
	defer a.updateMu.RUnlock()
	return a.updateState
}

// RestartLauncher relaunches into the freshly installed build.
//
// Only meaningful after DownloadLauncher has swapped the new binary into place;
// restartPath is captured there because os.Executable() is unreliable to
// interpret once the running image has been renamed out from under it.
func (a *App) RestartLauncher() error {
	a.updateMu.RLock()
	path := a.restartPath
	a.updateMu.RUnlock()

	if path == "" {
		return fmt.Errorf("no update is waiting to be applied")
	}

	cmd := exec.Command(path)
	if err := cmd.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}

// NewApp creates a new App application struct
func NewApp() *App {
	limboniaApp := limbonia.NewApp()
	a := &App{
		LimboniaApp:     limboniaApp,
		mephiPoll:       mephiPollInterval,
		mephiAppear:     mephiAppearWindow,
		injectFn:        limbonia.InjectLimbo,
		openMephiFn:     limboniaApp.OpenMephiIfInstalled,
		mephiRunningFn:  limbonia.IsMephiRunning,
		limbusRunningFn: limbonia.IsLimbusRunning,
	}
	// Wired after construction because it is one of the App's own methods.
	a.quitFn = a.quit
	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if exe, err := os.Executable(); err == nil {
		os.Remove(exe + ".old")
	}

	// Built up front, with no icon shown yet, so that whether hiding is even
	// possible is known before anything tries it. The icon only appears while the
	// launcher is actually away.
	a.tray = newTray("LLauncher", a.showWindow, a.quit)
	a.presence = &presence{
		// Method values on a nil *trayIcon are fine — the methods guard for it —
		// so a build with no notification area simply never manages to hide.
		trayShow:   a.tray.show,
		trayHide:   a.tray.hide,
		windowHide: func() { wailsruntime.WindowHide(a.ctx) },
		windowShow: func() {
			wailsruntime.WindowShow(a.ctx)
			// Unminimise too: a window minimised before it was hidden comes back
			// still minimised, which looks exactly like nothing happening.
			wailsruntime.WindowUnminimise(a.ctx)
		},
	}

	// Mephi is spawned down in the limbonia package; this is how the launcher
	// hears about it so it can get out of the way.
	a.LimboniaApp.OnMephiStarted = a.mephiStarted
}

// shutdown removes the tray icon before the process goes away.
func (a *App) shutdown(ctx context.Context) {
	a.tray.destroy()
}

// mephiStarted hides the launcher for as long as the companion is up.
//
// The launcher's whole job is over once Mephi is running — leaving a second
// window in the taskbar is just clutter — but it has to still be there when Mephi
// closes, so it goes to the tray rather than exiting.
func (a *App) mephiStarted(p *limbonia.LaunchedProcess) {
	// Nothing gets hidden on the way out — a window that vanishes into a tray icon
	// the shutdown is about to delete is a window nobody can get back.
	if a.quitting.Load() || !a.presence.hide() {
		return
	}
	go a.watchMephi(p)
}

// showWindow is the tray menu's Show, and what the watcher calls when the
// companion is gone.
func (a *App) showWindow() {
	// A watcher finishing while the launcher is on its way out must not put the
	// window back up mid-shutdown.
	if a.quitting.Load() {
		return
	}
	a.presence.restore()
}

// quit is the tray menu's Exit.
func (a *App) quit() {
	if a.ctx == nil {
		return
	}
	wailsruntime.Quit(a.ctx)
}

// declineShutdown closes the launcher after the user refused an elevation prompt.
//
// Refusing the prompt is a decision about the whole session, not about one
// button: neither the injector nor the companion can do anything useful
// unelevated, so a launcher left sitting there afterwards is a window with no
// working action in it.
//
// Nothing is shown on the way out. The only thing there is to report is the
// answer the user just gave to a dialog of their own, and a modal asking them to
// acknowledge it would have to be dismissed before the shutdown could continue —
// a prompt nobody clicks is a launcher that never closes. The rejected promise
// still reaches the frontend, so the toast fires if it wins the race.
func (a *App) declineShutdown() {
	// Once only: the injector and the companion are launched back to back and a
	// single "No" can come back declined from both.
	if !a.quitting.CompareAndSwap(false, true) {
		return
	}

	// Straight to the state machine rather than through showWindow, whose guard
	// has just been armed. Shutting down from behind a tray icon would leave the
	// window hidden and the icon lingering until something made Windows notice it
	// had no owner.
	a.presence.restore()

	if a.quitFn != nil {
		a.quitFn()
	}
}

// reportOrQuit closes the launcher when err is a refused elevation prompt, and
// hands every other failure straight back.
//
// A genuine fault — a missing Injector.exe, a blocked executable — is something
// the user may well be able to fix from the window that is still open, so only
// the deliberate refusal is fatal.
func (a *App) reportOrQuit(err error) error {
	if errors.Is(err, limbonia.ErrElevationCancelled) {
		a.declineShutdown()
	}
	return err
}

// mephiRunning asks whichever process lookup this App was built with.
func (a *App) mephiRunning() bool {
	if a.mephiRunningFn == nil {
		return false
	}
	return a.mephiRunningFn()
}

// limbusRunning reports whether the game is open.
//
// Missing wiring answers "not running" so an unwired launcher fails on the thing
// it actually cannot do, rather than blocking Open with a reason that isn't true.
func (a *App) limbusRunning() bool {
	if a.limbusRunningFn == nil {
		return false
	}
	return a.limbusRunningFn()
}

// IsGameRunning lets the frontend grey the Open button out while the game is up.
//
// Advisory only — the refusal in InjectLimbonia is the real one. This is polled,
// so it is always a little stale, and a UI check on its own would let a game
// started in the last second through.
func (a *App) IsGameRunning() bool {
	return a.limbusRunning()
}

func (a *App) watchMephi(p *limbonia.LaunchedProcess) {
	a.watchUntilGone(p.CanWait(), p.Wait)
}

// watchUntilGone restores the launcher once the companion is gone.
//
// The process handle from the elevated launch is the reliable signal, but it is
// not always there — the shell can decline to hand one back — and it says nothing
// about a Mephi that was already running when the launcher started. So the handle
// wait, when there is one, is followed by polling on the image name until nothing
// answers to it. Being fooled by a second copy started elsewhere is the safe
// direction to be wrong in: the launcher stays hidden a little longer, rather
// than popping up over a companion that is still in use.
//
// Split from watchMephi on its wait so a test can supply one, there being no way
// to arrange a real process that exits on cue.
func (a *App) watchUntilGone(canWait bool, wait func()) {
	if !a.mephiWatch.CompareAndSwap(false, true) {
		return
	}
	defer a.mephiWatch.Store(false)
	defer a.showWindow()

	if canWait {
		wait()
	} else {
		// With no handle, "not running" in the first few seconds means the process
		// has not appeared yet rather than that it has exited — and acting on it
		// would flash the launcher straight back up.
		deadline := time.Now().Add(a.mephiAppear)
		for !a.mephiRunning() && time.Now().Before(deadline) {
			time.Sleep(a.mephiPoll)
		}
	}

	for a.mephiRunning() {
		time.Sleep(a.mephiPoll)
	}
}

// InjectLimbonia patches the game and then brings up the companion.
//
// Mephi ships in the same bundle as the DLL and the injector and is how a user
// actually drives Limbonia, so Play means both. It was previously never started
// at all — OpenMephi existed and nothing called it, which presented as "the
// launcher does not open Mephi".
//
// Its failure is deliberately NOT this function's failure. The injection is the
// part that had to work; the game is already patched by the time we get here, and
// reporting an error would tell the user the thing that succeeded had failed.
// OpenMephi surfaces its own problems, and the absent case is silent by design —
// see OpenMephiIfInstalled.
func (a *App) InjectLimbonia() error {
	// A companion that is already up means Play is only the injection. Two of
	// them would fight over the same control pipe and the same settings file, and
	// the running one picks up the freshly patched game by itself — so the second
	// window would be pure confusion. Checked BEFORE injecting, because injecting
	// is what makes the game interesting to Mephi and could otherwise race.
	// Refused before anything else happens. The injector is what STARTS the game,
	// so with a copy already open there is nothing here that can reach it — it
	// would either fail or launch a second, and the user would carry on playing
	// unpatched with no idea why.
	if a.limbusRunning() {
		return limbonia.ErrLimbusRunning
	}

	alreadyRunning := a.mephiRunning()

	if a.injectFn == nil {
		return fmt.Errorf("the launcher is not set up to patch the game")
	}
	if err := a.injectFn(); err != nil {
		return a.reportOrQuit(err)
	}

	if alreadyRunning || a.openMephiFn == nil {
		return nil
	}

	// The one Mephi failure that is not swallowed: a refused elevation prompt
	// closes the launcher, because nothing it can still offer would work.
	// Everything else about the companion stays non-fatal — the injection is the
	// part that had to succeed, and it did.
	if err := a.openMephiFn(); errors.Is(err, limbonia.ErrElevationCancelled) {
		return a.reportOrQuit(err)
	}
	return nil
}

func (a *App) IsLinux() bool {
	return runtime.GOOS == "linux"
}

func (a *App) DeleteLimboniaDLL() error {
	cfg := config.Get()
	if cfg.LimbusFolder == "" {
		return fmt.Errorf("Limbus folder not set")
	}
	dlls := []string{"Limbonia.dll", "winhttp.dll"}
	var errs []string
	for _, dll := range dlls {
		dllPath := filepath.Join(cfg.LimbusFolder, dll)
		if _, err := os.Stat(dllPath); os.IsNotExist(err) {
			continue
		}
		if err := os.Remove(dllPath); err != nil {
			errs = append(errs, dll+": "+err.Error())
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("failed to delete: %s", strings.Join(errs, ", "))
	}
	return nil
}

// OpenFolder reveals a directory in the OS file manager.
//
// Previously this shelled out to `explorer` unconditionally, which cannot work on
// the Linux build. It also passed the path through unchecked, so a missing folder
// produced a silent no-op rather than something the caller could report.
func OpenFolder(path string) error {
	abs, err := resolveFolder(path)
	if err != nil {
		return err
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", abs)
	case "darwin":
		cmd = exec.Command("open", abs)
	default:
		cmd = exec.Command("xdg-open", abs)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("couldn't open the file manager")
	}

	// Reap the child so its handle isn't leaked. The exit status is deliberately
	// ignored: explorer.exe returns 1 even when it succeeded, so it tells us
	// nothing useful.
	go func() { _ = cmd.Wait() }()
	return nil
}

// resolveFolder validates a path and returns the absolute directory to reveal.
//
// Split out from OpenFolder so the validation can be tested without actually
// spawning a file-manager window.
func resolveFolder(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", fmt.Errorf("there's no folder to open yet")
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("that folder doesn't exist yet")
	}
	// Tolerate being handed a file: reveal the folder containing it.
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	return abs, nil
}

func (a *App) OpenSettingsFolder() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	return OpenFolder(filepath.Dir(exePath))
}

func (a *App) OpenLimbusFolder() error {
	cfg := config.Get()
	if cfg == nil || cfg.LimbusFolder == "" {
		return fmt.Errorf("set your Limbus Company folder first, in Settings")
	}
	return OpenFolder(cfg.LimbusFolder)
}

// OpenLimboniaFolder reveals Limbonia's settings directory.
//
// The DLL writes settings next to the GAME executable — SettingsManager resolves
// its base directory from GetModuleFileNameA(nullptr), which is the host process
// (LimbusCompany.exe), not the DLL. So this is <LimbusFolder>/Settings, and it
// only exists after the game has run once with Limbonia loaded.
func (a *App) OpenLimboniaFolder() error {
	cfg := config.Get()
	if cfg == nil || cfg.LimbusFolder == "" {
		return fmt.Errorf("set your Limbus Company folder first, in Settings")
	}

	settingsPath := filepath.Join(cfg.LimbusFolder, "Settings")
	if _, err := os.Stat(settingsPath); err != nil {
		return fmt.Errorf("Mephi hasn't saved any settings yet — start the game once, then try again")
	}

	return OpenFolder(settingsPath)
}

// DownloadLauncher replaces this executable with the newest published build.
//
// The binary is hash-verified before it is swapped in: this process is about to
// re-exec whatever lands at exePath, so installing an unverified download would
// be handing arbitrary code execution to anyone who can intercept the transfer.
func (a *App) DownloadLauncher() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	update, err := updater.GetLatestLauncherRelease()
	if err != nil {
		a.emitLauncherUpdate(LauncherUpdate{State: "error", Message: "Couldn't reach the update server."})
		return err
	}

	a.emitLauncherUpdate(LauncherUpdate{State: "downloading", Version: update.Version})

	res, err := http.Get(update.URL)
	if err != nil {
		a.emitLauncherUpdate(LauncherUpdate{State: "error", Version: update.Version, Message: "The download couldn't be started."})
		return err
	}
	defer res.Body.Close()

	// Without this, a 404/rate-limit HTML body would be written out and then
	// installed as the launcher.
	if res.StatusCode != http.StatusOK {
		a.emitLauncherUpdate(LauncherUpdate{State: "error", Version: update.Version, Message: "The download couldn't be completed."})
		return fmt.Errorf("update download failed with status %d", res.StatusCode)
	}

	newPath := exePath + ".new"
	f, err := os.Create(newPath)
	if err != nil {
		a.emitLauncherUpdate(LauncherUpdate{State: "error", Version: update.Version, Message: "Couldn't write the update to disk."})
		return err
	}
	_, err = io.Copy(&progressWriter{
		w:     f,
		total: res.ContentLength,
		onProgress: func(pct int) {
			a.emitLauncherUpdate(LauncherUpdate{State: "downloading", Version: update.Version, Percent: pct})
		},
	}, res.Body)
	f.Close()
	if err != nil {
		os.Remove(newPath)
		a.emitLauncherUpdate(LauncherUpdate{State: "error", Version: update.Version, Message: "The download was interrupted."})
		return err
	}

	// GitHub supplies a digest for releases created after it added artifact
	// digests. When present it is authoritative and a mismatch aborts the update;
	// when absent (older releases) there is nothing to check against, so the
	// download proceeds as before rather than blocking updates entirely.
	if update.Sha256 != "" {
		if err := client.VerifyFileSha256(newPath, update.Sha256); err != nil {
			os.Remove(newPath)
			a.emitLauncherUpdate(LauncherUpdate{
				State:   "error",
				Version: update.Version,
				Message: "The downloaded update failed its integrity check and was discarded.",
			})
			return err
		}
	}

	if err := os.Chmod(newPath, 0755); err != nil {
		os.Remove(newPath)
		a.emitLauncherUpdate(LauncherUpdate{State: "error", Version: update.Version, Message: "Couldn't prepare the update."})
		return err
	}

	// Rename errors were previously ignored, which could leave the old binary
	// moved aside with nothing put back in its place.
	oldPath := exePath + ".old"
	if err := os.Rename(exePath, oldPath); err != nil {
		os.Remove(newPath)
		a.emitLauncherUpdate(LauncherUpdate{State: "error", Version: update.Version, Message: "Couldn't replace the launcher. Close it and try again."})
		return err
	}
	if err := os.Rename(newPath, exePath); err != nil {
		os.Rename(oldPath, exePath) // put the working launcher back
		a.emitLauncherUpdate(LauncherUpdate{State: "error", Version: update.Version, Message: "Couldn't replace the launcher. Close it and try again."})
		return err
	}

	// Installed, but NOT applied: the running process is still the old build.
	// Previously this point called os.Exit, which closed the app out from under
	// the user. Now the status bar offers a restart and they choose when.
	a.updateMu.Lock()
	a.restartPath = exePath
	a.updateMu.Unlock()

	a.emitLauncherUpdate(LauncherUpdate{State: "ready", Version: update.Version, Percent: 100})
	return nil
}

// progressWriter reports download progress as a percentage while copying.
//
// Throttled to whole-percent changes: the raw copy loop fires far more often
// than the UI can meaningfully repaint, and emitting every chunk would flood the
// event bridge.
type progressWriter struct {
	w          io.Writer
	total      int64
	written    int64
	lastPct    int
	onProgress func(pct int)
}

func (p *progressWriter) Write(b []byte) (int, error) {
	n, err := p.w.Write(b)
	p.written += int64(n)

	// ContentLength is -1 for chunked responses; without a total there is no
	// percentage to report, so stay silent rather than emit nonsense.
	if p.total > 0 && p.onProgress != nil {
		pct := int(p.written * 100 / p.total)
		if pct > 100 {
			pct = 100
		}
		if pct != p.lastPct {
			p.lastPct = pct
			p.onProgress(pct)
		}
	}
	return n, err
}
