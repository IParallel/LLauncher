 # LLauncher

[![Build Status](https://github.com/IParallel/LLauncher/actions/workflows/release.yml/badge.svg)](https://github.com/IParallel/LLauncher/actions/workflows/release.yml)


 This repository contains LLauncher — a Wails (Go + web frontend) desktop application.

 This README documents how to install prerequisites, run the app in development, and produce production builds on Windows (PowerShell examples). It also includes troubleshooting tips and where to find built artifacts.

 ## Quick contract
 - Inputs: local machine (Windows) with administrator access to install runtimes and tools.
 - Outputs: runnable LLauncher app executable (Windows .exe) or a dev environment that reloads on frontend changes.
 - Success: `wails dev` runs the app in dev mode; `wails build` produces a Windows executable in `build/bin/`.

 ## Prerequisites

 - Go 1.20+.
 - Node.js (16+ recommended) and npm or pnpm.
 - Wails v2 CLI.

 ### Install on Windows (PowerShell)

 1. Clone the repo
 
 2. Install Go: https://go.dev/dl/ (Make sure `go` is on your PATH)

 3. Install Node.js: https://nodejs.org/

 4. Install the Wails CLI (from a powershell prompt). Ensure your Go env has a `GOBIN` or `%GOPATH%\\bin` on PATH. Example:

 ```powershell
 # Install Wails CLI
 go install github.com/wailsapp/wails/v2/cmd/wails@latest
 # Verify
 wails --version
 ```

 ## Project layout (key files)

 - `main.go`, `app.go` — Go application entry and app bindings.
 - `wails.json` — Wails project configuration (frontend build/install commands are defined here).
 - `frontend/` — Vue + Vite frontend. Build scripts live in `frontend/package.json`.
 - `build/` — output for built assets and platform binaries.

 ## Development

 There are two common modes during development:

 1. Dev mode (recommended) — uses a frontend dev server (hot reload) and Wails dev integration.
 2. Manual mode — run frontend dev server and `wails dev` or `go run` separately.

 ### Recommended: single-command dev (from project root)

 Wails is configured to use the frontend dev server automatically. From the project root run:

 ```powershell
 # From repository root (d:\\Weas\\Repos\\Go\\LLauncher)
 wails dev
 ```

 This will start the frontend dev watcher (the `wails.json` in this repo sets `frontend:dev:watcher` to `npm run dev`) and the Go backend together. Open the app window created by Wails.

 ### Notes about frontend dev scripts

 Frontend scripts are defined in `frontend/package.json`. Key scripts:
 - `npm run dev` — runs Vite + tailwind watcher concurrently (used by Wails dev watcher).

 Before running dev, ensure frontend dependencies are installed:

 ```powershell
 cd frontend
 npm install
 ```

 ## Production build

 1. Install frontend deps and build the frontend assets.

 ```powershell
 # From repo root
 cd frontend
 npm install
 npm run build
 ```

 2. Build the desktop app using Wails.

 ```powershell
 # Back to repo root
 cd ..
 wails build
 ```

 The produced binaries and packaging will be in the `build/bin/` folder. For example, Windows builds typically appear under `build/bin/windows/` or directly under `build/bin/` depending on config. The `wails.json` `outputfilename` is `LLauncher` (see `wails.json`).

 ## Running a built executable

 After a successful build, navigate to the produced binary directory and run the `.exe` file:

 ```powershell
 # Example (adjust path if different)
 cd build\\bin\\windows
 .\\LLauncher.exe
 ```

 ## Troubleshooting

 - "wails: command not found" — ensure `$GOBIN` or `%GOPATH%\\bin` is on PATH and `go install` for Wails CLI completed successfully.
 - WebView2 errors / blank window — install the WebView2 Evergreen runtime from Microsoft.
 - Frontend build errors (vite, tailwind, typescript) — run `cd frontend; npm run build` and inspect the stack trace. Ensure Node version is compatible with the devDependencies.
 - Go build errors — ensure Go version matches `go.mod` (`go 1.24.0`), run `go env` and `go version` to debug.

 ## Author / Contact

 From `wails.json`:
 - Author: Bello <bello.dev99@gmail.com>

 ## Notes / Caveats

 - `wails.json` in this repo sets `frontend:dev:serverUrl` to `auto` and `frontend:dev:watcher` to `npm run ttas`. The watcher uses the `ttas` script from `frontend/package.json` which runs `vite` and the tailwind watcher concurrently.
 - On Windows, Wails uses WebView2 (Edge) — ensure it is available on target machines.
 - Cross-compilation requires setting `GOOS` and `GOARCH` and that the required platform toolchain/runtimes are present.

 ## Where to look next

 - Frontend: `frontend/` (Vue + Vite)
 - Backend: `main.go`, `app.go`, `config/`, `limbonia/`, `updater/`
 - Wails config: `wails.json`
