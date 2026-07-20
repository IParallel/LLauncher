package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"WailsTest/config"
)

// The folder-shortcut buttons in Settings silently did nothing when they failed:
// the frontend dropped the returned promise, and OpenFolder shelled out to
// `explorer` on every platform without checking the path existed. These tests
// cover the validation half so a regression shows up here instead of as a dead
// button.

func TestResolveFolderRejectsEmpty(t *testing.T) {
	for _, in := range []string{"", "   ", "\t"} {
		if _, err := resolveFolder(in); err == nil {
			t.Errorf("resolveFolder(%q) should fail", in)
		}
	}
}

func TestResolveFolderRejectsMissing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "definitely-not-here")
	if _, err := resolveFolder(missing); err == nil {
		t.Error("a missing folder should report an error, not be silently opened")
	}
}

func TestResolveFolderAcceptsDirectory(t *testing.T) {
	dir := t.TempDir()
	got, err := resolveFolder(dir)
	if err != nil {
		t.Fatalf("an existing directory should resolve: %v", err)
	}
	if !filepath.IsAbs(got) {
		t.Errorf("resolved path should be absolute, got %q", got)
	}
}

// Being handed a file rather than a folder is a realistic mistake (the config
// stores a folder, but a caller could pass the exe). Reveal its parent instead
// of failing or trying to "open" the file — which on Windows would RUN it.
func TestResolveFolderOnFileReturnsParent(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "LimbusCompany.exe")
	if err := os.WriteFile(file, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveFolder(file)
	if err != nil {
		t.Fatalf("a file path should resolve to its parent: %v", err)
	}

	wantDir, _ := filepath.Abs(dir)
	gotResolved, _ := filepath.EvalSymlinks(got)
	wantResolved, _ := filepath.EvalSymlinks(wantDir)
	if gotResolved != wantResolved {
		t.Errorf("got %q, want the containing directory %q", gotResolved, wantResolved)
	}
}

// Both shortcuts must refuse with a readable reason when no Limbus folder has
// been picked, rather than opening something arbitrary.
func TestFolderShortcutsRequireLimbusFolder(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := config.Init(); err != nil {
		t.Fatalf("config init: %v", err)
	}
	config.Get().LimbusFolder = ""

	app := &App{}

	if err := app.OpenLimbusFolder(); err == nil {
		t.Error("OpenLimbusFolder should fail when no Limbus folder is set")
	}
	if err := app.OpenLimboniaFolder(); err == nil {
		t.Error("OpenLimboniaFolder should fail when no Limbus folder is set")
	}
}

// With a Limbus folder set but the game never run under Limbonia, the Settings
// directory doesn't exist yet. That must report the actual reason instead of
// appearing to do nothing.
func TestOpenLimboniaFolderReportsMissingSettings(t *testing.T) {
	work := t.TempDir()
	t.Chdir(work)
	if err := config.Init(); err != nil {
		t.Fatalf("config init: %v", err)
	}

	limbus := filepath.Join(work, "Limbus Company")
	if err := os.MkdirAll(limbus, 0o755); err != nil {
		t.Fatal(err)
	}
	config.Get().LimbusFolder = limbus

	app := &App{}
	err := app.OpenLimboniaFolder()
	if err == nil {
		t.Fatal("expected an error when Settings/ has not been created yet")
	}
	// The message is what the user actually sees in the toast, so it must point
	// at the fix rather than restate the failure.
	if !strings.Contains(err.Error(), "start the game") {
		t.Errorf("error should tell the user how to fix it, got: %q", err)
	}
}
