package main

import (
	"WailsTest/config"
	"WailsTest/updater"
	"context"
	"embed"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

var ZIP_PASSWORD string

func main() {

	// The ldflags-stamped value is the one released builds actually use; .env is a
	// development convenience. It's looked up next to the executable as well as in
	// the CWD because the launcher is normally started from a shortcut, where the
	// CWD is somewhere else entirely and a CWD-relative .env silently never loads.
	if err := godotenv.Load(".env"); err != nil {
		if err := godotenv.Load(filepath.Join(config.Dir(), ".env")); err != nil {
			println("No .env file loaded:", err.Error())
		}
	}

	updater.ZIP_PASSWORD = os.Getenv("ZIP_PASSWORD")

	if updater.ZIP_PASSWORD == "" {
		updater.ZIP_PASSWORD = ZIP_PASSWORD
	}

	// An empty password can never unlock an encrypted archive, and the zip library
	// decrypts with it anyway — producing garbage that only fails later as
	// "flate: corrupt input". Say so once, here, where it's diagnosable.
	if updater.ZIP_PASSWORD == "" {
		println("Warning: no ZIP_PASSWORD is set — installing updates will fail if the archive is password-protected")
	}

	// Clean up the previous binary left behind by a self-update. Resolved against
	// the executable's own directory, not the CWD: the launcher is usually started
	// from a shortcut, where the CWD is somewhere else and this silently no-ops.
	if exe, err := os.Executable(); err == nil {
		stale := exe + ".old"
		if _, err := os.Stat(stale); err == nil {
			os.Remove(stale)
		}
	}

	if err := config.Init(); err != nil {
		println("Error loading config:", err.Error())
	}

	conf := config.Get()

	if conf.LimbusFolder != "" {
		if _, err := os.Stat(conf.LimbusFolder + "\\discord_game_sdk.dll"); err == nil {
			os.Remove(conf.LimbusFolder + "\\discord_game_sdk.dll")
		}
	}

	app := NewApp()

	err := wails.Run(&options.App{
		// Deliberately NOT AlwaysOnTop. The launcher used to pin itself over
		// everything, which made it a nuisance the moment anything else was open —
		// including the game and Mephi, which is exactly when it has nothing left
		// to say. It hides to the tray while Mephi runs instead.
		Title: "LLauncher " + updater.CURRENT_LAUNCHER_VERSION,
		// Compact on purpose: the launcher is a settings pane and a Play button,
		// not a workspace. Sized to the widest content (the Settings card at
		// 420px) plus the 150px rail, with the window left non-resizable so the
		// layout only ever has to work at this one size.
		Width:         600,
		Height:        440,
		DisableResize: true,
		// Frameless: the frontend draws its own title bar (drag region +
		// minimise/close), matching Mephi. Without a custom bar the window would
		// have no way to be moved or closed.
		Frameless: true,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 10, G: 10, B: 10, A: 255},
		OnStartup: func(ctx context.Context) {
			app.startup(ctx)
			app.LimboniaApp.Startup(ctx)

			// Self-update runs here, not before wails.Run: the update path shows a
			// Wails dialog and can os.Exit, and firing it earlier raced startup()
			// with a nil ctx. Backgrounded so a slow GitHub call doesn't hold up
			// the window appearing.
			go func() {
				if needs, err := updater.CheckForUpdate(); err == nil && needs {
					app.DownloadLauncher()
				}
			}()
		},
		OnShutdown: func(ctx context.Context) {
			app.shutdown(ctx)
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
