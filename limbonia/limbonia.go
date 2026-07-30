package limbonia

import (
	"WailsTest/client"
	"WailsTest/config"
	"WailsTest/updater"
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	goruntime "runtime"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type LimboniaApp struct {
	ctx         context.Context
	downloading bool

	// OnMephiStarted, when set, is called with the companion process the moment
	// it starts. The launcher hides itself to the system tray while Mephi is up
	// and comes back when it exits; this is how it learns about both.
	//
	// A field rather than a setter method because Wails binds every exported
	// method on this struct — a SetMephiWatcher(func(...)) would end up in the
	// generated JavaScript API, taking a callback the frontend cannot supply.
	OnMephiStarted func(*LaunchedProcess) `json:"-"`
}

func NewApp() *LimboniaApp {
	return &LimboniaApp{}
}

func (a *LimboniaApp) Startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *LimboniaApp) OpenFileDialog() (string, error) {
	result, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:           "Find Limbus Company Executable",
		DefaultFilename: "LimbusCompany.exe",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "LimbusCompany.exe",
				Pattern:     "*.exe;*.jpg",
			},
		},
	})
	base := filepath.Base(result)
	runtime.LogInfo(a.ctx, base)
	if base != "LimbusCompany.exe" {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Invalid File",
			Message: "Please select LimbusCompany.exe",
			Type:    runtime.ErrorDialog,
		})
		return "", errors.New("invalid file")
	}
	path := filepath.Dir(result)
	config.Get().LimbusFolder = path
	// clientDir(), not "./limbonia": the launcher is usually started from a
	// shortcut, where the working directory is not the install directory, and the
	// injector.cfg written below has to land next to the Injector.exe that reads
	// it.
	//
	// That reader only exists on Windows. This function is not build-tagged, so
	// the file is written on Linux as well, where nothing ever opens it: no
	// Injector.exe is installed there (it lives under windows/ in the bundle,
	// which that platform's extractor skips — see updater.expectedFiles), and the
	// Linux path loads Limbonia through a Wine DLL override instead of injecting.
	// Harmless, but it is dead output on Linux rather than something Linux needs.
	limboniaDir := clientDir()
	if err := os.Mkdir(limboniaDir, 0750); err != nil && !errors.Is(err, os.ErrExist) {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to create the Mephi folder",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return "", err
	}
	injectorCfgPath := filepath.Join(limboniaDir, "injector.cfg")
	content := "exe=" + result + "\n"
	if err := os.WriteFile(injectorCfgPath, []byte(content), 0644); err != nil {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to write injector.cfg",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return "", err
	}
	discord_game_sdk_path := path + "\\discord_game_sdk.dll"
	if _, err := os.Stat(discord_game_sdk_path); err == nil {
		os.Remove(discord_game_sdk_path)
	}

	config.Save()
	if err != nil {
		return "", err
	}
	return path, nil
}

func (a *LimboniaApp) CheckLimboniaVersion() bool {
	result, err := updater.CheckForLimboniaVersion()
	if err != nil {
		runtime.LogError(a.ctx, err.Error())
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to check For Update",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return false
	}
	return result
}

// clientDir is where the client bundle installs, resolved against the launcher's
// own directory rather than the CWD (which differs when started from a shortcut).
func clientDir() string {
	return filepath.Join(config.Dir(), "limbonia")
}

// DownloadLimbonia installs the client bundle described by the signed manifest.
//
// The manifest is verified before anything is fetched and the archive is
// SHA-256 checked before it is extracted, so neither a hostile manifest host nor
// a tampered CDN payload can put files on disk. Everything in this bundle is
// later executed (Injector.exe, Mephi.exe), which is why the check is here and
// not merely advisory.
func (a *LimboniaApp) DownloadLimbonia() error {
	dir := clientDir()
	if err := os.MkdirAll(dir, 0750); err != nil {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to create directory",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return err
	}

	manifest, err := client.FetchManifest()
	if err != nil {
		runtime.LogError(a.ctx, err.Error())
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Couldn't check for updates",
			Message: "The update information couldn't be verified, so nothing was installed. Please try again later.",
			Type:    runtime.ErrorDialog,
		})
		return err
	}

	if err := a.downloadVerifiedArchive(manifest, dir); err != nil {
		runtime.LogError(a.ctx, err.Error())
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to install the update",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return err
	}

	// Recorded only after a verified install, so a failed download can't leave
	// the config claiming a version that isn't on disk.
	config.Get().CurrentClientVersion = manifest.Client.Version
	return config.Save()
}

// downloadVerifiedArchive fetches the manifest's archive to a temp file, checks
// its hash, and only then extracts it.
func (a *LimboniaApp) downloadVerifiedArchive(manifest *client.Manifest, destDir string) error {
	tmp, err := os.CreateTemp("", "llauncher-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := a.downloadFile(manifest.Client.URL, tmpPath, "Limbonia.zip", false); err != nil {
		return err
	}

	if err := client.VerifyFileSha256(tmpPath, manifest.Client.Sha256); err != nil {
		return err
	}

	if err := updater.ExtractZipWithPassword(tmpPath, destDir, updater.ZIP_PASSWORD); err != nil {
		return err
	}

	// The bundle ships every platform's files and extraction keeps only this
	// one's, so an archive built with the wrong layout extracts "successfully"
	// while leaving nothing behind. Caught here rather than at injection time,
	// where it surfaces as a missing-DLL error that names no cause.
	if err := updater.VerifyInstall(destDir); err != nil {
		return err
	}

	runtime.EventsEmit(a.ctx, "download:complete", "Limbonia.zip")
	return nil
}

// installClientBundle downloads and installs the client bundle into clientDir()
// without any UI, for callers outside the Wails-bound update button.
//
// It goes through the signed manifest and hash-checks the archive first, exactly
// as the update button does; nothing may install a payload the manifest did not
// commit to.
func installClientBundle() error {
	dir := clientDir()
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	manifest, err := client.FetchManifest()
	if err != nil {
		return fmt.Errorf("couldn't check for the client bundle: %w", err)
	}

	tmp, err := os.CreateTemp("", "limbonia-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpPath)

	if err := downloadToFile(manifest.Client.URL, tmpPath); err != nil {
		return err
	}
	if err := client.VerifyFileSha256(tmpPath, manifest.Client.Sha256); err != nil {
		return err
	}
	if err := updater.ExtractZipWithPassword(tmpPath, dir, updater.ZIP_PASSWORD); err != nil {
		return err
	}
	return updater.VerifyInstall(dir)
}

// mephiPath is where the companion lives once the bundle is installed.
//
// The name differs by platform because the bundle ships both (windows/Mephi.exe
// and linux/Mephi) and extraction keeps only the one this OS needs — see
// updater/layout.go.
func mephiPath() string {
	name := "Mephi.exe"
	if goruntime.GOOS != "windows" {
		name = "Mephi"
	}
	return filepath.Join(clientDir(), name)
}

// mephiProcessName is the image name to match a running companion by, which is
// just the file name of whatever this platform's binary is called.
func mephiProcessName() string {
	return filepath.Base(mephiPath())
}

// OpenMephiIfInstalled launches Mephi when the installed bundle has it, and does
// nothing at all when it does not.
//
// This is the PLAY path: Mephi is started after a successful injection, and a
// user whose installed bundle predates Mephi must not be shown a dialog every
// single time they launch the game. OpenMephi's "isn't installed yet" prompt is
// right when somebody asks for Mephi directly and wrong as a side effect of
// pressing Play, so the absent case is silent here.
//
// Anything else — present but unlaunchable — still reports, because that is a
// real fault rather than an old install.
func (a *LimboniaApp) OpenMephiIfInstalled() error {
	if _, err := os.Stat(mephiPath()); err != nil {
		return nil
	}
	return a.OpenMephi()
}

// OpenMephi launches the companion app that ships inside the client bundle.
func (a *LimboniaApp) OpenMephi() error {
	binPath := mephiPath()

	if goruntime.GOOS != "windows" {
		// Extraction already sets this for anything under linux/ (and for any entry
		// whose archive mode says so), but an install left by an older launcher
		// won't have it. Same 0755 the extractor uses, so re-launching can't
		// downgrade what the installer set.
		if err := os.Chmod(binPath, 0755); err != nil {
			runtime.LogError(a.ctx, err.Error())
		}
	}

	// Check before spawning so a missing bundle reports something actionable
	// rather than a bare exec error.
	if _, err := os.Stat(binPath); err != nil {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Mephi isn't installed yet",
			Message: "Install the update first, then try again.",
			Type:    runtime.ErrorDialog,
		})
		return err
	}

	// Elevated: Mephi drives the injected DLL, and a medium-integrity process
	// cannot open a handle to an elevated game. The prompt is per launch — the
	// launcher itself is deliberately not marked requireAdministrator.
	proc, err := launchElevated(binPath, clientDir())
	if err != nil {
		runtime.LogError(a.ctx, err.Error())
		if errors.Is(err, ErrElevationCancelled) {
			// Not a fault: the user was asked and said no. The caller turns this
			// into a toast, and a modal on top of that would be saying it twice.
			return elevationRefused("Mephi", err)
		}
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to open Mephi",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return err
	}

	if a.OnMephiStarted != nil {
		a.OnMephiStarted(proc)
	}
	return nil
}

// GetChangelog returns recent published updates for the What's New panel.
func (a *LimboniaApp) GetChangelog() []client.ChangelogEntry {
	entries, err := client.FetchChangelog(5)
	if err != nil {
		// Non-fatal: the panel shows nothing rather than blocking the launcher.
		runtime.LogError(a.ctx, err.Error())
		return []client.ChangelogEntry{}
	}
	return entries
}

func (a *LimboniaApp) GetConfig() config.Config {
	return *config.Get()
}

func (a *LimboniaApp) GetServerVersion() updater.UpdateResponse {
	return updater.GetVersions()
}

func (a *LimboniaApp) CheckForUpdate() (bool, error) {
	need, err := updater.CheckForUpdate()
	if err != nil {
		runtime.LogError(a.ctx, err.Error())
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to check For Update",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return false, err
	}

	return need, nil
}

// DownloadAndExtract and DownloadUpdate were both removed deliberately.
//
// DownloadAndExtract took an arbitrary URL from the frontend, downloaded it and
// extracted it over the install directory with no hash check at all.
// DownloadUpdate was worse: an arbitrary URL to an arbitrary path, i.e. write
// any remote file anywhere on disk. Both were bound methods, so anything running
// in the webview could call them, and both bypassed the signed manifest.
// Nothing called either. Installs go through DownloadLimbonia, which only ever
// writes a payload the signed manifest committed to.

// downloadToFile fetches url to dest with no progress reporting.
//
// Used by the install paths that run outside the Wails UI, where there is no
// frontend to emit progress events to.
func downloadToFile(url, dest string) error {
	res, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer res.Body.Close()
	// An unchecked status wrote the body of a 404 to disk under the name of the
	// file it was meant to be.
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download %s: status %d", url, res.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, res.Body); err != nil {
		return err
	}
	return f.Close()
}

// downloadFile is the internal implementation that accepts a custom label for events.
// emitComplete controls whether download:complete is emitted at the end.
func (a *LimboniaApp) downloadFile(url, dest, label string, emitComplete bool) error {
	// Reporting success here would be a lie: the caller goes on to hash and
	// extract a file that was never written.
	if a.downloading {
		runtime.LogInfo(a.ctx, "Already downloading")
		return errors.New("a download is already in progress — wait for it to finish")
	}
	a.downloading = true
	defer func() {
		a.downloading = false
	}()
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Without this the body of a 404 or an error page is written out as if it
	// were the file. Callers that hash-check catch it a step later with a
	// confusing "integrity check" message; callers that don't, don't catch it.
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s failed with status %d", label, resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	total := resp.ContentLength
	var downloaded int64 = 0
	buf := make([]byte, 32*1024)
	lastEmit := time.Now()

	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			// A dropped write error here meant a disk-full download completed
			// "successfully" and only failed later as a hash mismatch.
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			downloaded += int64(n)

			if time.Since(lastEmit) > 200*time.Millisecond {
				percent := float64(downloaded) / float64(total) * 100
				runtime.EventsEmit(a.ctx, "download:progress", map[string]interface{}{"file": label, "percent": int(percent)})
				lastEmit = time.Now()
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	// Closed explicitly so a deferred error can't be swallowed: callers hash this
	// file immediately, and a failed close would show up as a bogus mismatch.
	// The deferred Close above then no-ops with "file already closed".
	if err := out.Close(); err != nil {
		return err
	}

	runtime.EventsEmit(a.ctx, "download:progress", map[string]interface{}{"file": label, "percent": 100})
	// Note: download:complete for DownloadAndExtract callers is emitted after extraction.
	// For direct DownloadUpdate calls, emit it here.
	if emitComplete {
		runtime.EventsEmit(a.ctx, "download:complete", label)
	}
	return nil
}
