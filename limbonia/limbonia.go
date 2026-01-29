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
	err := os.Mkdir("./limbonia", 0750)
	if err != nil && !errors.Is(err, os.ErrExist) {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to create directory",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return err
	}
	err = a.DownloadUpdate(updater.INJECTOR_DOWNLOAD_URL, "./limbonia/injector.exe")
	if err != nil {
		runtime.LogError(a.ctx, err.Error())
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to download the injector",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return err
	}
	err = a.DownloadUpdate(updater.LIMBONIA_DOWNLOAD_URL, "./limbonia/Limbonia.dll")
	if err != nil {
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
	err := os.Mkdir("./bot", 0750)
	if err != nil && !errors.Is(err, os.ErrExist) {
		runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
			Title:   "Failed to create directory",
			Message: err.Error(),
			Type:    runtime.ErrorDialog,
		})
		return err
	}
	err = a.DownloadUpdate(updater.BOT_DOWNLOAD_URL, "./bot/BotQuixote.exe")
	if err != nil {
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
		err := a.DownloadUpdate(updater.LAUNCHER_DOWNLOAD_URL, "./LLauncher.exe")
		if err != nil {
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

func (a *LimboniaApp) DownloadUpdate(url, dest string) error {

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
				runtime.EventsEmit(a.ctx, "download:progress", int(percent))
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

	runtime.EventsEmit(a.ctx, "download:complete", dest)
	return nil
}
