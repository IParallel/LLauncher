package updater

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/yeka/zip"
)

// installedNames lists what actually landed in destDir.
func installedNames(t *testing.T, destDir string) []string {
	t.Helper()
	entries, err := os.ReadDir(destDir)
	if err != nil {
		t.Fatalf("reading install dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

func assertInstalled(t *testing.T, destDir string, want ...string) {
	t.Helper()
	sort.Strings(want)
	got := installedNames(t, destDir)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("installed %v, want %v", got, want)
	}
}

// The whole point of the change: one archive carries every platform, and only
// the running platform's files are installed.
func TestExtractKeepsOnlyCurrentPlatform(t *testing.T) {
	entries := []string{
		"common/Limbonia.dll",
		"windows/Injector.exe",
		"windows/Mephi.exe",
		"linux/Mephi",
	}

	t.Run("windows", func(t *testing.T) {
		dir := t.TempDir()
		archive := filepath.Join(dir, "bundle.zip")
		dest := filepath.Join(dir, "install")
		buildZip(t, archive, "", 0, entries...)

		if err := extractZipForOS(archive, dest, "", "windows"); err != nil {
			t.Fatalf("extract: %v", err)
		}
		assertInstalled(t, dest, "Limbonia.dll", "Injector.exe", "Mephi.exe")
		assertExtracted(t, dest, "Limbonia.dll", "Injector.exe", "Mephi.exe")
	})

	t.Run("linux", func(t *testing.T) {
		dir := t.TempDir()
		archive := filepath.Join(dir, "bundle.zip")
		dest := filepath.Join(dir, "install")
		buildZip(t, archive, "", 0, entries...)

		if err := extractZipForOS(archive, dest, "", "linux"); err != nil {
			t.Fatalf("extract: %v", err)
		}
		assertInstalled(t, dest, "Limbonia.dll", "Mephi")
		assertExtracted(t, dest, "Limbonia.dll", "Mephi")
	})
}

// The compatibility case, and the one that breaks every existing user if it
// regresses: the bundle published today is FLAT — Limbonia.dll and Injector.exe
// at the archive root, no platform directories anywhere. Every entry must be
// treated as common and installed, on both platforms.
func TestFlatLegacyBundleStillInstalls(t *testing.T) {
	for _, goos := range []string{"windows", "linux"} {
		t.Run(goos, func(t *testing.T) {
			dir := t.TempDir()
			archive := filepath.Join(dir, "bundle.zip")
			dest := filepath.Join(dir, "install")
			buildZip(t, archive, testPassword, zip.StandardEncryption,
				"Limbonia.dll", "Injector.exe", "Mephi.exe")

			if err := extractZipForOS(archive, dest, testPassword, goos); err != nil {
				t.Fatalf("a flat legacy bundle failed to install: %v", err)
			}
			assertInstalled(t, dest, "Limbonia.dll", "Injector.exe", "Mephi.exe")
			assertExtracted(t, dest, "Limbonia.dll", "Injector.exe", "Mephi.exe")
		})
	}
}

// An unrecognised directory is common too, not "some other platform's". Anything
// stricter would drop files from a bundle laid out in a way this launcher hasn't
// been taught about yet.
func TestUnknownDirectoryIsCommon(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bundle.zip")
	dest := filepath.Join(dir, "install")
	buildZip(t, archive, "", 0, "bundle/Limbonia.dll", "assets/data.bin")

	if err := extractZipForOS(archive, dest, "", "windows"); err != nil {
		t.Fatalf("extract: %v", err)
	}
	assertInstalled(t, dest, "Limbonia.dll", "data.bin")
}

func TestEntryScope(t *testing.T) {
	cases := map[string]string{
		"Limbonia.dll":         "",
		"common/Limbonia.dll":  "",
		"windows/Injector.exe": "windows",
		"linux/Mephi":          "linux",
		"darwin/Mephi":         "darwin",
		// Case is the packager's whim, not a signal.
		"Windows/Mephi.exe": "windows",
		"LINUX/Mephi":       "linux",
		// Backslashes: legal in a hostile (or Windows-authored) archive, and
		// path.Clean alone would miss the directory on Linux.
		`windows\Mephi.exe`: "windows",
		// A platform directory only counts at the top level.
		"extras/windows/thing.exe": "",
		// Traversal is not a platform, so it falls through to common and is then
		// flattened to a base name like anything else.
		"../../escaped.dll": "",
		"./Limbonia.dll":    "",
	}
	for name, want := range cases {
		if got := entryScope(name); got != want {
			t.Errorf("entryScope(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestEntryBaseName(t *testing.T) {
	cases := map[string]string{
		"common/Limbonia.dll": "Limbonia.dll",
		"linux/Mephi":         "Mephi",
		"../../escaped.dll":   "escaped.dll",
		`windows\Mephi.exe`:   "Mephi.exe",
		// Clean collapses this to ".", which the extraction loop skips outright.
		"a/..": ".",
		// This one survives Clean as ".." and is what stagedPath has to catch.
		"../..": "..",
	}
	for name, want := range cases {
		if got := entryBaseName(name); got != want {
			t.Errorf("entryBaseName(%q) = %q, want %q", name, got, want)
		}
	}
}

// The traversal guard is asserted rather than assumed, because the extraction
// loop now reads the entry path (to find the platform directory) instead of only
// flattening it. "../.." flattens to "..", which would join to staging's PARENT
// — the one case base-name flattening does not cover on its own.
func TestStagedPathRejectsEscape(t *testing.T) {
	staging := t.TempDir()

	if _, err := stagedPath(staging, "Limbonia.dll"); err != nil {
		t.Errorf("a normal name was rejected: %v", err)
	}
	for _, name := range []string{"..", filepath.Join("..", "evil.dll")} {
		if _, err := stagedPath(staging, name); err == nil {
			t.Errorf("stagedPath(%q) was accepted", name)
		}
	}
}

// A foreign platform's entries must never be written at all — not written and
// then deleted. The staging directory is removed on the way out, so this checks
// the observable consequence: nothing from the other OS reaches the install.
func TestForeignPlatformEntriesAreNotWritten(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bundle.zip")
	dest := filepath.Join(dir, "install")
	buildZip(t, archive, "", 0, "common/Limbonia.dll", "windows/Injector.exe")

	if err := extractZipForOS(archive, dest, "", "linux"); err != nil {
		t.Fatalf("extract: %v", err)
	}
	assertInstalled(t, dest, "Limbonia.dll")

	// And no staging leftovers holding the skipped file either.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".llauncher-staging-") {
			t.Errorf("staging directory %s was left behind", e.Name())
		}
	}
}

func TestWantsExecutableBit(t *testing.T) {
	if !wantsExecutableBit("linux", 0o644) {
		t.Error("a linux/ entry must be marked executable regardless of its recorded mode")
	}
	if !wantsExecutableBit("darwin", 0o644) {
		t.Error("a darwin/ entry must be marked executable")
	}
	if !wantsExecutableBit("", 0o755) {
		t.Error("a flat entry that records an executable mode must keep it")
	}
	if wantsExecutableBit("", 0o644) {
		t.Error("a plain common file must not be marked executable")
	}
	if wantsExecutableBit("windows", 0o644) {
		t.Error("a windows/ entry has no mode bit to set")
	}
}

// The end-to-end check of the mode: a binary out of linux/ has to be runnable,
// and archive/zip's mode does not survive os.Create.
func TestLinuxBinaryIsExecutable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode bits are not meaningful on Windows")
	}

	dir := t.TempDir()
	archive := filepath.Join(dir, "bundle.zip")
	dest := filepath.Join(dir, "install")
	buildZip(t, archive, "", 0, "common/Limbonia.dll", "linux/Mephi")

	if err := extractZipForOS(archive, dest, "", "linux"); err != nil {
		t.Fatalf("extract: %v", err)
	}

	info, err := os.Stat(filepath.Join(dest, "Mephi"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("Mephi is not executable (mode %v) and could not be started", info.Mode().Perm())
	}

	// The DLL is data; it must not be made executable as a side effect.
	dll, err := os.Stat(filepath.Join(dest, "Limbonia.dll"))
	if err != nil {
		t.Fatal(err)
	}
	if dll.Mode().Perm()&0o111 != 0 {
		t.Errorf("Limbonia.dll was marked executable (mode %v)", dll.Mode().Perm())
	}
}

// A legacy flat bundle has no linux/ directory, so the only signal left is the
// mode the archive itself recorded.
func TestFlatEntryKeepsRecordedExecutableMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("mode bits are not meaningful on Windows")
	}

	dir := t.TempDir()
	archive := filepath.Join(dir, "bundle.zip")
	dest := filepath.Join(dir, "install")

	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	hdr := &zip.FileHeader{Name: "Mephi", Method: zip.Deflate}
	hdr.SetMode(0o755)
	out, err := w.CreateHeader(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := out.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := extractZipForOS(archive, dest, "", "linux"); err != nil {
		t.Fatalf("extract: %v", err)
	}
	info, err := os.Stat(filepath.Join(dest, "Mephi"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Errorf("the archive's executable mode was lost (got %v)", info.Mode().Perm())
	}
}

// A bundle that carries no files for this platform extracts "successfully" and
// installs nothing. That has to be reported as a broken install rather than
// discovered later as a missing DLL at injection time.
func TestVerifyInstall(t *testing.T) {
	t.Run("missing Limbonia.dll fails", func(t *testing.T) {
		dir := t.TempDir()
		err := verifyInstall(dir, "windows")
		if err == nil {
			t.Fatal("an empty install directory should not verify")
		}
		if !strings.Contains(err.Error(), "Limbonia.dll") {
			t.Errorf("the missing file should be named, got: %q", err)
		}
	})

	t.Run("Limbonia.dll alone verifies", func(t *testing.T) {
		// An older bundle predating Mephi is incomplete but still usable, so the
		// companion binaries must not fail the install on their own.
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "Limbonia.dll"), payload, 0o644); err != nil {
			t.Fatal(err)
		}
		for _, goos := range []string{"windows", "linux"} {
			if err := verifyInstall(dir, goos); err != nil {
				t.Errorf("%s: %v", goos, err)
			}
		}
	})
}
