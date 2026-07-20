package main

import (
	"WailsTest/client"
	"WailsTest/config"
	"WailsTest/limbonia"
	"WailsTest/updater"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
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
	return &App{
		LimboniaApp: limbonia.NewApp(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	if exe, err := os.Executable(); err == nil {
		os.Remove(exe + ".old")
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
	if err := limbonia.InjectLimbo(); err != nil {
		return err
	}

	_ = a.LimboniaApp.OpenMephiIfInstalled()
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
