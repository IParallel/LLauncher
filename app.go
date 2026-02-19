package main

import (
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

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx         context.Context
	LimboniaApp *limbonia.LimboniaApp
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		LimboniaApp: limbonia.NewApp(),
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) InjectLimbonia() error {
	err := limbonia.InjectLimbo()

	if err != nil {
		return err
	}

	return nil
}

func OpenFolder(path string) error {
	cmd := exec.Command("explorer", path)
	if err := cmd.Start(); err != nil {
		return err
	}
	return nil
}

func (a *App) OpenSettingsFolder() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}
	return OpenFolder(filepath.Dir(exePath))
}

func (a *App) OpenLimbusFolder() error {
	config := config.Get()
	if config.LimbusFolder == "" {
		return fmt.Errorf("Limbus folder not set")
	}
	return OpenFolder(config.LimbusFolder)
}

func (a *App) OpenLimboniaFolder() error {
	cfg := config.Get()
	if cfg.LimbusFolder == "" {
		return fmt.Errorf("Limbus folder not set")
	}

	limboniaSettingsPath := filepath.Join(cfg.LimbusFolder, "Settings")
	if _, err := os.Stat(limboniaSettingsPath); os.IsNotExist(err) {
		return fmt.Errorf("Limbonia Settings folder not found")
	}

	return OpenFolder(limboniaSettingsPath)
}

func (a *App) DownloadLauncher() error {
	tmpFile, err := os.CreateTemp("", "llauncher-*.zip")
	if err != nil {
		return err
	}
	tmpZipPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpZipPath)

	res, err := http.Get(updater.LAUNCHER_DOWNLOAD_URL)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	zipFile, err := os.Create(tmpZipPath)
	if err != nil {
		return err
	}
	_, err = io.Copy(zipFile, res.Body)
	zipFile.Close()
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "llauncher-extract-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	if err := updater.ExtractZipWithPassword(tmpZipPath, tmpDir, updater.ZIP_PASSWORD); err != nil {
		return err
	}

	if err := os.Rename(filepath.Join(tmpDir, "LLauncher.exe"), "./LLauncher.new"); err != nil {
		return err
	}

	runtime.MessageDialog(a.ctx, runtime.MessageDialogOptions{
		Type:    runtime.InfoDialog,
		Title:   "New Update!",
		Message: "Please restart to apply the update",
		Buttons: []string{"OK"},
	})

	os.Rename("./LLauncher.exe", "./LLauncher.old")
	os.Rename("./LLauncher.new", "./LLauncher.exe")

	os.Exit(0)

	return nil
}
