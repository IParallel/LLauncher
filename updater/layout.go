package updater

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

// Bundle layout
//
// Limbonia.zip carries the files for every platform in one archive, sorted into
// top-level directories. The launcher extracts only what the running OS needs:
//
//	common/    Limbonia.dll          — every platform
//	windows/   Injector.exe  Mephi.exe
//	linux/     Mephi
//
// Anything NOT under a recognised platform directory is treated as common and
// installed everywhere. That is what keeps the flat bundles published before
// this change working: an archive with Limbonia.dll and Injector.exe at the root
// has no platform directories at all, so every entry is common and every entry
// is kept. Do not "fix" that into a strict whitelist — it would silently install
// nothing from an older bundle.
//
// The directory is authoritative; file extensions are never inspected. A Linux
// binary is free to be called anything, and a shared data file that happens to
// end in .dll belongs wherever the packager put it, not wherever a suffix rule
// guesses.
const (
	dirCommon  = "common"
	dirWindows = "windows"
	dirLinux   = "linux"
	dirDarwin  = "darwin"
)

// platformDirs are the directory names that scope an entry to one OS. The values
// are GOOS strings so a scope compares directly against runtime.GOOS.
var platformDirs = map[string]string{
	dirWindows: "windows",
	dirLinux:   "linux",
	dirDarwin:  "darwin",
}

// entryScope returns the GOOS an archive entry is restricted to, or "" when the
// entry is common to every platform.
//
// The leading path segment is read here, which means this function inspects an
// attacker-controlled path. It deliberately returns only a scope — never a
// filesystem path — so nothing it derives can be used to build a destination.
// The destination is always built from the base name, and checked again by
// stagedPath.
func entryScope(name string) string {
	// Zip entries use forward slashes by spec, but nothing enforces it: a hostile
	// (or Windows-authored) archive can contain backslashes, and on Linux
	// path.Clean would treat `windows\Mephi.exe` as one long file name and miss
	// the platform directory entirely.
	normalized := path.Clean(strings.ReplaceAll(name, `\`, "/"))
	normalized = strings.TrimPrefix(normalized, "/")

	slash := strings.Index(normalized, "/")
	if slash < 0 {
		// A bare file name — no directory, therefore common.
		return ""
	}

	// Matched case-insensitively: zip tooling on Windows preserves whatever the
	// packager typed, and "Windows/Mephi.exe" is plainly meant to be the Windows
	// directory.
	dir := strings.ToLower(normalized[:slash])
	if dir == dirCommon {
		return ""
	}
	if goos, ok := platformDirs[dir]; ok {
		return goos
	}

	// Unrecognised directory (including a leading "..", which Clean leaves in
	// place): common. See the note on backwards compatibility above.
	return ""
}

// entryBaseName returns the flattened name an entry is written under.
//
// Everything is flattened to a base name, which is both the install layout (the
// bundle's directories are a packaging detail, not something that ends up on
// disk) and the primary defence against a "../" entry escaping the destination.
func entryBaseName(name string) string {
	return path.Base(path.Clean(strings.ReplaceAll(name, `\`, "/")))
}

// wantsExecutableBit reports whether an extracted entry has to be marked
// executable.
//
// Two signals, because neither alone is enough:
//
//   - The entry is scoped to a Unix platform directory. That is the reliable
//     one: something in linux/ is there because it runs on Linux, and the only
//     file we ship there is the Mephi binary. Deciding by file name instead
//     would break the moment the binary is renamed.
//   - The zip entry itself records an executable mode. Archives written on Linux
//     carry the mode in the external attributes, so this covers a Mephi binary
//     sitting at the root of a legacy flat bundle, where there is no directory
//     to go on.
//
// Marking a non-executable file executable is harmless; failing to mark the
// binary means it cannot start at all, so the bias is deliberate.
func wantsExecutableBit(scope string, mode uint32) bool {
	if scope == "linux" || scope == "darwin" {
		return true
	}
	const anyExecBit = 0o111
	return mode&anyExecBit != 0
}

// VerifyInstall checks that an extracted bundle actually contains what this OS
// needs to run.
//
// Only Limbonia.dll is fatal: without it there is nothing to inject, and the
// failure would otherwise surface much later as an opaque error from the
// injector or from Wine. The companion binaries are reported by name in the
// error when they are also missing, because "the bundle is missing everything"
// and "the bundle is missing one file" want different fixes — but they do not
// fail the install on their own, since an older bundle predating Mephi is still
// a usable Limbonia install.
func VerifyInstall(dir string) error {
	return verifyInstall(dir, runtime.GOOS)
}

func verifyInstall(dir, goos string) error {
	var missing []string
	for _, name := range expectedFiles(goos) {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			missing = append(missing, name)
		}
	}

	for _, name := range missing {
		if name != "Limbonia.dll" {
			continue
		}
		return fmt.Errorf(
			"the update installed but Limbonia.dll is missing from it (missing: %s) — the published bundle may not include this platform's files",
			strings.Join(missing, ", "),
		)
	}
	return nil
}

// expectedFiles lists what a complete bundle leaves in the install directory for
// a platform, in priority order.
func expectedFiles(goos string) []string {
	if goos == "windows" {
		return []string{"Limbonia.dll", "Injector.exe", "Mephi.exe"}
	}
	return []string{"Limbonia.dll", "Mephi"}
}
