package updater

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yeka/zip"
)

// These tests exist because of a live failure that reached users as
// "flate: corrupt input before offset 5" when they clicked Update.
//
// The cause is that yeka/zip's legacy ZipCrypto path does NO password
// verification at all (zipcrypto.go: ZipCryptoDecryptor just decrypts and drops
// the 12-byte header). A wrong — or empty — password therefore decrypts to
// noise which is handed straight to the DEFLATE reader, and the only symptom is
// a decompression error that tells the user nothing. The AES path does return
// zip: invalid password, so only the legacy path is silently wrong.
//
// The passwords below are throwaway literals. The real archive password is a
// build secret and must never appear in this repository.

const (
	testPassword  = "test-archive-password"
	wrongPassword = "not-the-archive-password"
)

// payload is deliberately compressible and longer than one flate block, so a
// bad decryption fails inside the decompressor rather than by luck.
var payload = bytes.Repeat([]byte("Limbonia.dll payload bytes 0123456789 "), 200)

// buildZip writes an archive containing entries, encrypting them with password
// when enc is non-zero.
func buildZip(t *testing.T, path, password string, enc zip.EncryptionMethod, entries ...string) {
	t.Helper()
	if len(entries) == 0 {
		entries = []string{"Limbonia.dll"}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	for _, name := range entries {
		var (
			out io.Writer
			err error
		)
		if enc == 0 {
			out, err = w.Create(name)
		} else {
			out, err = w.Encrypt(name, password, enc)
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := out.Write(payload); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func assertExtracted(t *testing.T, destDir string, names ...string) {
	t.Helper()
	for _, name := range names {
		got, err := os.ReadFile(filepath.Join(destDir, name))
		if err != nil {
			t.Fatalf("%s was not extracted: %v", name, err)
		}
		if !bytes.Equal(got, payload) {
			t.Errorf("%s extracted with the wrong contents", name)
		}
	}
}

// assertActionable is the whole point of the fix: whatever goes wrong, the user
// must not be shown a raw compression-library error.
func assertActionable(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "flate:") {
		t.Errorf("raw decompression error surfaced to the user: %q", err)
	}
}

func TestExtractPlainZip(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "plain.zip")
	dest := filepath.Join(dir, "install")
	buildZip(t, archive, "", 0)

	if err := ExtractZipWithPassword(archive, dest, testPassword); err != nil {
		t.Fatalf("an unencrypted archive should extract even with a password set: %v", err)
	}
	assertExtracted(t, dest, "Limbonia.dll")
}

// A plain archive must also extract when no password is configured — the
// empty-password guard must only fire on encrypted entries.
func TestExtractPlainZipWithoutPassword(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "plain.zip")
	dest := filepath.Join(dir, "install")
	buildZip(t, archive, "", 0)

	if err := ExtractZipWithPassword(archive, dest, ""); err != nil {
		t.Fatalf("an unencrypted archive needs no password: %v", err)
	}
	assertExtracted(t, dest, "Limbonia.dll")
}

func TestExtractEncryptedZipWithCorrectPassword(t *testing.T) {
	for name, enc := range map[string]zip.EncryptionMethod{
		"zipcrypto": zip.StandardEncryption,
		"aes256":    zip.AES256Encryption,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			archive := filepath.Join(dir, "bundle.zip")
			dest := filepath.Join(dir, "install")
			buildZip(t, archive, testPassword, enc, "Limbonia.dll", "Injector.exe")

			if err := ExtractZipWithPassword(archive, dest, testPassword); err != nil {
				t.Fatalf("correct password failed to extract: %v", err)
			}
			assertExtracted(t, dest, "Limbonia.dll", "Injector.exe")
		})
	}
}

// The regression itself: a wrong ZipCrypto password produced "flate: corrupt
// input before offset N" and nothing else.
func TestExtractZipCryptoWrongPasswordIsActionable(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bundle.zip")
	dest := filepath.Join(dir, "install")
	buildZip(t, archive, testPassword, zip.StandardEncryption)

	err := ExtractZipWithPassword(archive, dest, wrongPassword)
	assertActionable(t, err)
	if !strings.Contains(err.Error(), "password") {
		t.Errorf("a wrong password should be named as the cause, got: %q", err)
	}
}

// AES already reports ErrPassword; check it is translated rather than passed
// through as "zip: invalid password".
func TestExtractAesWrongPasswordIsActionable(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bundle.zip")
	dest := filepath.Join(dir, "install")
	buildZip(t, archive, testPassword, zip.AES256Encryption)

	err := ExtractZipWithPassword(archive, dest, wrongPassword)
	assertActionable(t, err)
	if !strings.Contains(err.Error(), "password") {
		t.Errorf("a wrong password should be named as the cause, got: %q", err)
	}
}

// An empty password can never be correct, so it is refused before anything is
// decrypted. This is the exact state a locally-built launcher is in when
// ZIP_PASSWORD was neither stamped in by ldflags nor present in .env.
func TestExtractEncryptedZipWithoutPasswordIsRefused(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bundle.zip")
	dest := filepath.Join(dir, "install")
	buildZip(t, archive, testPassword, zip.StandardEncryption)

	err := ExtractZipWithPassword(archive, dest, "")
	assertActionable(t, err)
	if !strings.Contains(err.Error(), "password") {
		t.Errorf("the missing password should be named, got: %q", err)
	}
}

// What a 404 body or an interrupted transfer looks like on disk. This is caught
// by the hash check first on the real path, but extraction must still refuse it
// with something readable rather than an archive/zip internal error.
func TestExtractRejectsNonArchive(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "notazip.zip")
	dest := filepath.Join(dir, "install")
	if err := os.WriteFile(archive, []byte("<!doctype html><title>404 Not Found</title>"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ExtractZipWithPassword(archive, dest, testPassword)
	assertActionable(t, err)
	if !strings.Contains(err.Error(), "zip archive") {
		t.Errorf("error should say the file isn't a usable archive, got: %q", err)
	}
}

// A truncated archive loses its end-of-central-directory record.
func TestExtractRejectsTruncatedArchive(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bundle.zip")
	dest := filepath.Join(dir, "install")
	buildZip(t, archive, testPassword, zip.StandardEncryption)

	full, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(archive, full[:len(full)/2], 0o644); err != nil {
		t.Fatal(err)
	}

	assertActionable(t, ExtractZipWithPassword(archive, dest, testPassword))
}

// An archive whose central directory is intact but whose compressed stream is
// damaged opens fine and only fails at decompression. That is a real corruption
// rather than a password problem, and must be reported as such — with the
// explanation first and the library detail kept only as a trailing diagnostic.
func TestExtractReportsDamagedArchive(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bundle.zip")
	dest := filepath.Join(dir, "install")
	buildZip(t, archive, "", 0)

	r, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	offset, err := r.File[0].DataOffset()
	r.Close()
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	for i := offset; i < offset+64 && int(i) < len(raw); i++ {
		raw[i] ^= 0xff
	}
	if err := os.WriteFile(archive, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	err = ExtractZipWithPassword(archive, dest, testPassword)
	if err == nil {
		t.Fatal("a damaged archive should not extract")
	}
	if !strings.Contains(err.Error(), "damaged") {
		t.Errorf("a damaged archive should be named as such, got: %q", err)
	}
	if strings.HasPrefix(err.Error(), "flate:") {
		t.Errorf("the user-facing explanation must come first, got: %q", err)
	}
}

// A failed extraction used to overwrite the installed files as it went, so a
// bad password left a truncated Limbonia.dll behind and broke a working install.
func TestFailedExtractionLeavesInstallUntouched(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bundle.zip")
	dest := filepath.Join(dir, "install")
	buildZip(t, archive, testPassword, zip.StandardEncryption)

	if err := os.MkdirAll(dest, 0o750); err != nil {
		t.Fatal(err)
	}
	existing := filepath.Join(dest, "Limbonia.dll")
	previous := []byte("the previously installed build")
	if err := os.WriteFile(existing, previous, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ExtractZipWithPassword(archive, dest, wrongPassword); err == nil {
		t.Fatal("expected the extraction to fail")
	}

	got, err := os.ReadFile(existing)
	if err != nil {
		t.Fatalf("the existing install was removed by a failed update: %v", err)
	}
	if !bytes.Equal(got, previous) {
		t.Error("a failed update overwrote the working install")
	}

	// The staging directory must not survive a failure either.
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

// Entries are flattened to their base name, so a nested path can't write
// outside destDir and a directory entry isn't turned into a junk file.
func TestExtractFlattensPathsAndSkipsDirectories(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "nested.zip")
	dest := filepath.Join(dir, "install")
	buildZip(t, archive, "", 0, "bundle/", "bundle/Limbonia.dll", "../../escaped.dll")

	if err := ExtractZipWithPassword(archive, dest, ""); err != nil {
		t.Fatalf("extract: %v", err)
	}
	assertExtracted(t, dest, "Limbonia.dll", "escaped.dll")

	if _, err := os.Stat(filepath.Join(dir, "escaped.dll")); err == nil {
		t.Error("an entry escaped the destination directory")
	}
	if _, err := os.Stat(filepath.Join(dest, "bundle")); err == nil {
		t.Error("a directory entry was written out as a file")
	}
}

// The header check is what lets a wrong password be named rather than guessed
// at. It must not misfire on the correct one, which would turn a working update
// into a spurious "wrong password".
func TestWrongZipCryptoPasswordDetection(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "bundle.zip")
	buildZip(t, archive, testPassword, zip.StandardEncryption)

	r, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	f := r.File[0]
	if wrongZipCryptoPassword(archive, f, testPassword) {
		t.Error("the correct password was reported as wrong")
	}
	if !wrongZipCryptoPassword(archive, f, wrongPassword) {
		t.Error("a wrong password was not detected")
	}
	if !wrongZipCryptoPassword(archive, f, "") {
		t.Error("an empty password was not detected")
	}
}
