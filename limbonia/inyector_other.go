//go:build !windows

package limbonia

import (
	"WailsTest/config"
	"WailsTest/updater"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	limbusAppID  = "1973530"
	launchOption = `WINEDLLOVERRIDES="winhttp=n,b" %command%`
)

func InjectLimbo() error {
	cfg := config.Get()
	if cfg == nil || cfg.LimbusFolder == "" {
		return errors.New("Limbus Company folder not set — configure it in Settings")
	}

	if err := ensureDLLs(cfg.LimbusFolder); err != nil {
		return err
	}

	if err := setSteamLaunchOption(limbusAppID, launchOption); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not update Steam launch options: %v\n", err)
	}

	return exec.Command("xdg-open", "steam://rungameid/"+limbusAppID).Start()
}

// ensureDLLs puts the DLLs the Wine-side game loads into the game folder.
//
// The client bundle is NOT extracted into the game folder any more. It installs
// to clientDir() on every platform, and the two DLLs the game needs are copied
// across from there. That split matters beyond tidiness: Mephi locates the
// launcher's config.json by looking at its own parent directory and reads
// current_client_version from it, so a Mephi binary dropped into
// steamapps/common/Limbus Company/ has no config.json above it and can never
// tell that an update exists.
//
// Limbonia.dll is always re-copied rather than skipped when present, so an
// updated bundle actually reaches the game folder instead of leaving a stale
// build in place.
func ensureDLLs(gameFolder string) error {
	cache := clientDir()

	// winhttp.dll is not part of the client bundle — it ships on its own — so it
	// is only fetched when the cache does not already have it.
	if _, err := os.Stat(filepath.Join(cache, "winhttp.dll")); err != nil {
		if err := os.MkdirAll(cache, 0750); err != nil {
			return err
		}
		if err := downloadToFile(updater.WINHTTP_DOWNLOAD_URL, filepath.Join(cache, "winhttp.dll")); err != nil {
			return fmt.Errorf("couldn't download winhttp.dll: %w", err)
		}
	}

	// A missing Limbonia.dll means the bundle was never installed (or was
	// installed by an older launcher straight into the game folder). Install it
	// through the signed manifest — never by fetching a bare URL.
	if _, err := os.Stat(filepath.Join(cache, "Limbonia.dll")); err != nil {
		if err := installClientBundle(); err != nil {
			return err
		}
	}

	for _, dll := range []string{"winhttp.dll", "Limbonia.dll"} {
		src := filepath.Join(cache, dll)
		if err := copyFile(src, filepath.Join(gameFolder, dll)); err != nil {
			return fmt.Errorf("couldn't copy %s into the game folder: %w", dll, err)
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	// Closed explicitly: a deferred Close drops its error, and a short write
	// flushed at close would leave a truncated Limbonia.dll in the game folder
	// that the game then tries to load.
	return out.Close()
}

// downloadLimboniaDLLs was removed: the bundle install now goes through
// installClientBundle() in limbonia.go, which is the same signed-manifest and
// hash-checked path the Update button uses, extracting to clientDir() rather
// than into the game folder.

// setSteamLaunchOption reads the Steam localconfig.vdf, patches the LaunchOptions
// for appID, and writes it back.
func setSteamLaunchOption(appID, option string) error {
	vdfPath, err := findLocalConfig()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(vdfPath)
	if err != nil {
		return err
	}
	updated := patchLaunchOptions(string(data), appID, option)
	return os.WriteFile(vdfPath, []byte(updated), 0644)
}

func findLocalConfig() (string, error) {
	home, _ := os.UserHomeDir()
	bases := []string{
		filepath.Join(home, ".local/share/Steam/userdata"),
		filepath.Join(home, ".steam/steam/userdata"),
		filepath.Join(home, ".steam/root/userdata"),
	}
	for _, base := range bases {
		entries, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			p := filepath.Join(base, e.Name(), "config", "localconfig.vdf")
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}
	return "", errors.New("Steam localconfig.vdf not found")
}

// patchLaunchOptions sets the LaunchOptions key for appID inside the VDF content.
// It handles three cases: app section exists with the key, exists without it, or
// the section is missing entirely.
func patchLaunchOptions(content, appID, option string) string {
	// Escape quotes for VDF string value
	escaped := strings.ReplaceAll(option, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)

	lines := strings.Split(content, "\n")

	// Find the line that is just `"appID"`
	appLine := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == `"`+appID+`"` {
			appLine = i
			break
		}
	}
	if appLine == -1 {
		return insertAppSection(lines, appID, escaped)
	}

	// Find the opening brace of this section
	braceOpen := -1
	for i := appLine + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "{" {
			braceOpen = i
			break
		}
	}
	if braceOpen == -1 {
		return content
	}

	// Find the matching closing brace
	depth := 1
	braceClose := -1
	for i := braceOpen + 1; i < len(lines); i++ {
		switch strings.TrimSpace(lines[i]) {
		case "{":
			depth++
		case "}":
			depth--
			if depth == 0 {
				braceClose = i
			}
		}
		if braceClose != -1 {
			break
		}
	}
	if braceClose == -1 {
		return content
	}

	indent := vdfIndent(lines[braceClose]) + "\t"
	newLine := indent + `"LaunchOptions"` + "\t\t" + `"` + escaped + `"`

	// Replace existing LaunchOptions if present
	for i := braceOpen + 1; i < braceClose; i++ {
		if strings.Contains(strings.TrimSpace(lines[i]), `"LaunchOptions"`) {
			lines[i] = newLine
			return strings.Join(lines, "\n")
		}
	}

	// Insert before closing brace
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:braceClose]...)
	out = append(out, newLine)
	out = append(out, lines[braceClose:]...)
	return strings.Join(out, "\n")
}

// insertAppSection adds a new app entry to the "apps" section of the VDF.
func insertAppSection(lines []string, appID, escapedOption string) string {
	appsLine := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == `"apps"` {
			appsLine = i
			break
		}
	}
	if appsLine == -1 {
		return strings.Join(lines, "\n")
	}

	braceOpen := -1
	for i := appsLine + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "{" {
			braceOpen = i
			break
		}
	}
	if braceOpen == -1 {
		return strings.Join(lines, "\n")
	}

	depth := 1
	braceClose := -1
	for i := braceOpen + 1; i < len(lines); i++ {
		switch strings.TrimSpace(lines[i]) {
		case "{":
			depth++
		case "}":
			depth--
			if depth == 0 {
				braceClose = i
			}
		}
		if braceClose != -1 {
			break
		}
	}
	if braceClose == -1 {
		return strings.Join(lines, "\n")
	}

	indent := vdfIndent(lines[braceClose])
	section := []string{
		indent + "\t" + `"` + appID + `"`,
		indent + "\t{",
		indent + "\t\t" + `"LaunchOptions"` + "\t\t" + `"` + escapedOption + `"`,
		indent + "\t}",
	}

	out := make([]string, 0, len(lines)+4)
	out = append(out, lines[:braceClose]...)
	out = append(out, section...)
	out = append(out, lines[braceClose:]...)
	return strings.Join(out, "\n")
}

// vdfIndent returns the leading tab characters of a line.
func vdfIndent(s string) string {
	return s[:len(s)-len(strings.TrimLeft(s, "\t"))]
}
