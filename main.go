package main

import (
	"WailsTest/config"
	"WailsTest/updater"
	"context"
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {

	if _, err := os.Stat("./LLauncher.old"); err == nil {
		os.Remove("./LLauncher.old")
	}

	config.Init()

	conf := config.Get()

	if conf.LimbusFolder != "" {
		if _, err := os.Stat(conf.LimbusFolder + "\\discord_game_sdk.dll"); err == nil {
			os.Remove(conf.LimbusFolder + "\\discord_game_sdk.dll")
		}
	}

	app := NewApp()

	if needs, err := updater.CheckForUpdate(); err == nil && needs {
		go app.DownloadLauncher()
	}

	err := wails.Run(&options.App{
		AlwaysOnTop:   true,
		Title:         "LLauncher v" + updater.CURRENT_LAUNCHER_VERSION,
		Width:         900,
		Height:        500,
		DisableResize: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 10, G: 10, B: 10, A: 255},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			app.LimboniaApp.Startup(ctx)
		},
		Bind: []any{
			app,
			app.LimboniaApp,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
