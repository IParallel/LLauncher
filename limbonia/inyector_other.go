//go:build !windows

package limbonia

import (
	"WailsTest/config"
	"WailsTest/updater"
	"errors"
	"fmt"
	"io"
	"net/http"
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

func ensureDLLs(gameFolder string) error {
	dlls := []string{"winhttp.dll", "Limbonia.dll"}
	var missing []string
	for _, dll := range dlls {
		if _, err := os.Stat(filepath.Join(gameFolder, dll)); err != nil {
			missing = append(missing, dll)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	// Try copying from local cache (./limbonia/) populated by the Update button
	allCopied := true
	for _, dll := range missing {
		if err := copyFile(filepath.Join(".", "limbonia", dll), filepath.Join(gameFolder, dll)); err != nil {
			allCopied = false
		}
	}
	if allCopied {
		return nil
	}

	// Fall back to downloading directly into the game folder
	return downloadLimboniaDLLs(gameFolder)
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
	_, err = io.Copy(out, in)
	return err
}

func downloadLimboniaDLLs(destDir string) error {
	// Download Limbonia.zip and extract
	res, err := http.Get(updater.LIMBONIA_DOWNLOAD_URL)
	if err != nil {
		return fmt.Errorf("failed to download Limbonia: %w", err)
	}
	defer res.Body.Close()

	tmp, err := os.CreateTemp("", "limbonia-*.zip")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err = io.Copy(tmp, res.Body); err != nil {
		tmp.Close()
		return err
	}
	tmp.Close()

	if err := updater.ExtractZipWithPassword(tmpPath, destDir, updater.ZIP_PASSWORD); err != nil {
		return err
	}

	// Download winhttp.dll directly
	return downloadFile(updater.WINHTTP_DOWNLOAD_URL, filepath.Join(destDir, "winhttp.dll"))
}

func downloadFile(url, dest string) error {
	res, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer res.Body.Close()
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, res.Body)
	return err
}

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
