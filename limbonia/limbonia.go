package limbonia

import (
	"WailsTest/config"
	"WailsTest/updater"
	"context"
	_ "embed"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type LimboniaApp struct {
	ctx         context.Context
	downloading bool
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
	limboniaDir := filepath.Join(".", "limbonia")
	if err := os.Mkdir(limboniaDir, 0750); err != nil && !errors.Is(err, os.ErrExist) {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to create Limbonia folder",
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

func (a *LimboniaApp) DownloadLimbonia() error {
	if err := os.MkdirAll("./limbonia", 0750); err != nil {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to create directory",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return err
	}
	if err := a.DownloadAndExtract(updater.LIMBONIA_DOWNLOAD_URL, "./limbonia"); err != nil {
		runtime.LogError(a.ctx, err.Error())
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to download Limbonia.dll",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return err
	}
	config.Get().CurrentLimboniaVersion = updater.GetVersions().LimboniaVersion
	config.Save()
	return nil
}

func (a *LimboniaApp) OpenBotQuixote() error {
	cmd := exec.Command("./bot/BotQuixote.exe")
	if err := cmd.Start(); err != nil {
		runtime.LogError(a.ctx, err.Error())
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to open BotQuixote",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return err
	}
	return nil
}

func (a *LimboniaApp) DownloadBotQuixote() error {
	if err := os.MkdirAll("./bot", 0750); err != nil {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to create directory",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return err
	}
	if err := a.DownloadAndExtract(updater.BOT_DOWNLOAD_URL, "./bot"); err != nil {
		runtime.LogError(a.ctx, err.Error())
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to download the update",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return err
	}
	config.Get().CurrentBotVersion = updater.GetVersions().BotVersion
	config.Save()
	return nil
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

	if need {
		if err := a.DownloadAndExtract(updater.LAUNCHER_DOWNLOAD_URL, "."); err != nil {
			runtime.LogError(a.ctx, err.Error())
			runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
				Title:   "Failed to download LLauncher.exe",
				Message: err.Error(),
				Type:    runtime.ErrorDialog,
			})
			return false, err
		}
	}
	return true, nil
}

// DownloadAndExtract downloads a password-protected zip from url and extracts
// its contents into destDir, then removes the temporary zip file.
// Progress/complete events are emitted using the URL's base filename as label.
func (a *LimboniaApp) DownloadAndExtract(url, destDir string) error {
	tmpFile, err := os.CreateTemp("", "llauncher-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	label := filepath.Base(url) // e.g. "Limbonia.zip"
	if err := a.downloadFile(url, tmpPath, label, false); err != nil {
		return err
	}
	if err := updater.ExtractZipWithPassword(tmpPath, destDir, updater.ZIP_PASSWORD); err != nil {
		return err
	}
	// Emit complete after successful extraction
	runtime.EventsEmit(a.ctx, "download:complete", label)
	return nil
}

// DownloadUpdate downloads a file from url to dest, emitting progress events
// using dest as the label. Exposed to the Wails frontend.
func (a *LimboniaApp) DownloadUpdate(url, dest string) error {
	return a.downloadFile(url, dest, filepath.Base(dest), true)
}

// downloadFile is the internal implementation that accepts a custom label for events.
// emitComplete controls whether download:complete is emitted at the end.
func (a *LimboniaApp) downloadFile(url, dest, label string, emitComplete bool) error {
	if a.downloading {
		runtime.LogInfo(a.ctx, "Already downloading")
		return nil
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
			out.Write(buf[:n])
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

	runtime.EventsEmit(a.ctx, "download:progress", map[string]interface{}{"file": label, "percent": 100})
	// Note: download:complete for DownloadAndExtract callers is emitted after extraction.
	// For direct DownloadUpdate calls, emit it here.
	if emitComplete {
		runtime.EventsEmit(a.ctx, "download:complete", label)
	}
	return nil
}
