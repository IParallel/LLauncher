package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// Config is the launcher's persisted state.
//
// It lives next to the executable, NOT in the working directory: the launcher is
// commonly started from a shortcut or from Steam, where the CWD is somewhere else
// entirely, and a CWD-relative config silently creates a fresh one each time.
type Config struct {
	LimbusFolder string `json:"limbus_folder"`

	// CurrentClientVersion is the installed client bundle (Limbonia.dll +
	// Injector.exe + Mephi.exe). Plain semver as of the manifest cutover —
	// the old "1.98.1:F3" format is normalised away by migrate().
	CurrentClientVersion string `json:"current_client_version"`

	// ── Legacy fields, read once so an existing config.json can be migrated,
	// then cleared. Kept in the struct (not deleted) so unmarshalling an old
	// file doesn't drop the values before migrate() can move them.
	LegacyLimboniaVersion string `json:"current_limbonia_version,omitempty"`
	LegacyBotVersion      string `json:"current_bot_version,omitempty"`
	LegacyVersion         string `json:"current_version,omitempty"`
}

var (
	mu     sync.Mutex
	config *Config
)

func Get() *Config {
	return config
}

// path returns the config file location next to the executable, falling back to
// the working directory only if the executable path can't be resolved.
func path() string {
	exe, err := os.Executable()
	if err != nil {
		return "config.json"
	}
	return filepath.Join(filepath.Dir(exe), "config.json")
}

// Dir returns the directory the launcher installs into (the executable's own
// directory). All install paths — ./limbonia, config.json — hang off this rather
// than the CWD.
func Dir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

// Init loads the config, creating a default one when absent. It never panics: a
// corrupt config.json is replaced with defaults rather than crashing the launcher
// before the UI exists to report anything.
func Init() error {
	mu.Lock()
	defer mu.Unlock()

	data, err := os.ReadFile(path())
	if err != nil {
		config = &Config{}
		return save()
	}

	cfg := &Config{}
	if err := json.Unmarshal(data, cfg); err != nil {
		// Unreadable config: start clean instead of dying. The only real loss is
		// the Limbus folder, which the user can re-pick.
		config = &Config{}
		return save()
	}
	config = cfg

	if migrate() {
		return save()
	}
	return nil
}

// migrate folds the pre-Mephi config layout into the current one and removes the
// standalone bot install. Returns true when something changed and needs saving.
//
// The old launcher tracked two products with two version keys; there is now one
// bundle. Anything left in ./bot is dead weight that the new launcher will never
// look at again.
func migrate() bool {
	changed := false

	if config.CurrentClientVersion == "" && config.LegacyLimboniaVersion != "" {
		// Old client versions looked like "1.98.1:F3". The suffix was never
		// compared, only equality-checked, so drop it and keep plain semver.
		v := config.LegacyLimboniaVersion
		for i := 0; i < len(v); i++ {
			if v[i] == ':' {
				v = v[:i]
				break
			}
		}
		config.CurrentClientVersion = v
		changed = true
	}

	if config.LegacyLimboniaVersion != "" || config.LegacyBotVersion != "" || config.LegacyVersion != "" {
		config.LegacyLimboniaVersion = ""
		config.LegacyBotVersion = ""
		config.LegacyVersion = ""
		changed = true
	}

	// The bot ships inside the client bundle now; the separate install is stale.
	botDir := filepath.Join(Dir(), "bot")
	if _, err := os.Stat(botDir); err == nil {
		_ = os.RemoveAll(botDir) // best-effort; a locked file just leaves it behind
	}

	return changed
}

// Save persists the config. Safe to call from any goroutine.
func Save() error {
	mu.Lock()
	defer mu.Unlock()
	return save()
}

// save writes atomically (temp file + rename) so a crash mid-write can't leave a
// truncated config.json behind. Callers must hold mu.
func save() error {
	if config == nil {
		return nil
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	target := path()
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}
