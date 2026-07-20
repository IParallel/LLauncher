package updater

import (
	"WailsTest/client"
	"WailsTest/config"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/yeka/zip"
)

const (
	UPDATE_URL            = "https://files.ibello.cc/version.json"
	LIMBONIA_DOWNLOAD_URL = "https://files.ibello.cc/Limbonia.zip"
	WINHTTP_DOWNLOAD_URL  = "https://files.ibello.cc/winhttp.dll"
	GITHUB_LATEST_URL     = "https://api.github.com/repos/IParallel/LLauncher/releases/latest"
)

var CURRENT_LAUNCHER_VERSION = "dev"

var ZIP_PASSWORD = ""

// UpdateResponse is the version summary handed to the frontend.
//
// The bot no longer has its own version: it ships inside the one client bundle.
type UpdateResponse struct {
	ClientVersion   string `json:"client_version"`
	LauncherVersion string `json:"launcher_version"`
}

type githubAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	// Digest is GitHub's "sha256:<hex>" for the asset. Present on releases
	// created after GitHub added artifact digests; empty on older ones, which is
	// why callers must treat it as optional rather than required.
	Digest string `json:"digest"`
}

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

// LauncherAssetName is the release asset for the current OS.
func LauncherAssetName() string {
	if runtime.GOOS == "windows" {
		return "LLauncher.exe"
	}
	return "LLauncher-linux"
}

func LauncherDownloadURL() string {
	return "https://github.com/IParallel/LLauncher/releases/latest/download/" + LauncherAssetName()
}

// LauncherUpdate describes an available launcher self-update.
type LauncherUpdate struct {
	Version string
	URL     string
	// Sha256 is the expected hex digest, or "" when GitHub didn't supply one.
	Sha256 string
}

// httpClient bounds every request. The stdlib default has no timeout, so a hung
// connection could wedge the startup update check indefinitely.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// BotDownloadURL returns the BotQuixote archive URL for the current OS.
// Linux users get a separate build packaged as BotQuixote-linux.zip.
func BotDownloadURL() string {
	if runtime.GOOS == "windows" {
		return "https://files.ibello.cc/BotQuixote.zip"
	}
	return "https://files.ibello.cc/BotQuixote-linux.zip"
}

// GetLatestLauncherRelease fetches the newest GitHub release and picks out the
// asset for this OS, along with its digest when GitHub provides one.
func GetLatestLauncherRelease() (*LauncherUpdate, error) {
	req, err := http.NewRequest("GET", GITHUB_LATEST_URL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// Unauthenticated GitHub API calls are rate-limited to 60/hr per IP. Without
	// this check a 403 body would unmarshal to an empty release and read as
	// "version ''", which the old string compare treated as an update.
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub release check failed with status %d", res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var release githubRelease
	if err := json.Unmarshal(body, &release); err != nil {
		return nil, err
	}
	if release.TagName == "" {
		return nil, fmt.Errorf("GitHub release has no tag")
	}

	update := &LauncherUpdate{
		Version: release.TagName,
		URL:     LauncherDownloadURL(),
	}
	want := LauncherAssetName()
	for _, a := range release.Assets {
		if a.Name != want {
			continue
		}
		if a.URL != "" {
			update.URL = a.URL
		}
		update.Sha256 = strings.TrimPrefix(a.Digest, "sha256:")
		break
	}
	return update, nil
}

// GetLatestLauncherVersion returns just the newest release tag.
func GetLatestLauncherVersion() (string, error) {
	update, err := GetLatestLauncherRelease()
	if err != nil {
		return "", err
	}
	return update.Version, nil
}

// CheckForUpdate reports whether a NEWER launcher exists.
//
// Compares semver rather than string equality. Equality reported an update
// whenever the versions merely differed, which meant a dev build (or any build
// ahead of the published release) re-downloaded and relaunched itself forever.
func CheckForUpdate() (needsUpdate bool, err error) {
	update, err := GetLatestLauncherRelease()
	if err != nil {
		return false, err
	}
	return CompareSemver(CURRENT_LAUNCHER_VERSION, update.Version) < 0, nil
}

// CompareSemver returns -1, 0 or 1 comparing two version strings numerically.
// A leading "v" and any pre-release/build suffix are ignored.
//
// "dev" (the default ldflags value) parses to 0.0.0, so an unstamped local build
// always considers the published release newer. That is deliberate: it is the
// safe direction, and a real build always carries a real tag.
func CompareSemver(a, b string) int {
	pa := parseSemver(a)
	pb := parseSemver(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			if pa[i] < pb[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func parseSemver(v string) [3]int {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+:"); i >= 0 {
		v = v[:i]
	}
	for i, part := range strings.SplitN(v, ".", 3) {
		if i > 2 {
			break
		}
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return out
		}
		out[i] = n
	}
	return out
}

// GetVersions reports the versions the frontend displays.
//
// Backed by the signed manifest now rather than the old unauthenticated
// version.json. Errors surface as an empty response for the frontend's benefit —
// but note that nothing here decides whether to INSTALL anything; that path goes
// through the verified manifest in package client.
func GetVersions() UpdateResponse {
	m, err := client.FetchManifest()
	if err != nil {
		return UpdateResponse{}
	}
	return UpdateResponse{
		ClientVersion:   m.Client.Version,
		LauncherVersion: CURRENT_LAUNCHER_VERSION,
	}
}

// CheckForLimboniaVersion reports whether a newer client bundle is available.
//
// Semver-compared against the signed manifest. The previous implementation used
// `!=` against a version.json fetch that swallowed every error into a zero value,
// so any network blip reported "update available" against an empty string.
func CheckForLimboniaVersion() (bool, error) {
	conf := config.Get()
	if conf == nil {
		return false, nil
	}

	m, err := client.FetchManifest()
	if err != nil {
		// Report "no update" on failure rather than "update available": a failed
		// or unverifiable manifest must never trigger an install.
		return false, err
	}
	return client.NeedsUpdate(conf.CurrentClientVersion, m), nil
}

// ExtractZipWithPassword extracts a password-protected zip file into destDir,
// keeping only the entries that apply to the OS the launcher is running on.
//
// Each file inside the archive is written to destDir preserving only its base
// name; the bundle's platform directories (see layout.go) decide what is kept,
// not where it lands.
//
// Everything is unpacked to a staging directory first and only moved into
// destDir once every entry has been read successfully. A failure part-way
// through used to leave a half-written install behind — a truncated
// Limbonia.dll next to a stale Injector.exe — which is worse than not updating
// at all.
func ExtractZipWithPassword(zipPath, destDir, password string) error {
	return extractZipForOS(zipPath, destDir, password, runtime.GOOS)
}

// stagedEntry is one extracted file awaiting the move into destDir.
type stagedEntry struct {
	name string
	exec bool
}

// extractZipForOS is ExtractZipWithPassword with the target OS injected, so the
// filtering can be tested for a platform other than the one running the tests.
func extractZipForOS(zipPath, destDir, password, goos string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		// A structurally invalid file here is almost never a "zip bug": it is an
		// error page, a truncated transfer, or a file that isn't an archive.
		return fmt.Errorf("the downloaded update isn't a readable zip archive — the download may have been interrupted or blocked: %w", err)
	}
	defer r.Close()

	// A legacy-encrypted (ZipCrypto) entry is decrypted by yeka/zip WITHOUT any
	// password verification, so a missing password produces plausible-looking
	// garbage that only fails later, inside the DEFLATE reader, as
	// "flate: corrupt input". Refusing up front turns the single most likely
	// misconfiguration — a build with no ZIP_PASSWORD compiled in — into a
	// message that says so.
	if password == "" {
		for _, f := range r.File {
			if f.IsEncrypted() {
				return fmt.Errorf("this update is password-protected but this launcher build has no archive password — reinstall the launcher from the official download")
			}
		}
	}

	if err := os.MkdirAll(destDir, 0750); err != nil {
		return err
	}

	// Staged alongside destDir so the final move is a same-volume rename.
	staging, err := os.MkdirTemp(filepath.Dir(destDir), ".llauncher-staging-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)

	var staged []stagedEntry
	for _, f := range r.File {
		// Foreign-platform entries are never extracted at all. Extracting and then
		// deleting would put another OS's executables on disk, however briefly.
		scope := entryScope(f.Name)
		if scope != "" && scope != goos {
			continue
		}

		// Only base names are kept, so nested entries flatten and a "../" name
		// can't escape destDir.
		name := entryBaseName(f.Name)
		if f.FileInfo().IsDir() || name == "." || name == ".." || name == "/" || name == "" {
			continue
		}

		// The flattening above is the guard, but it is now no longer the only
		// thing that reads the entry path — entryScope reads it too — so the
		// containment is asserted explicitly rather than left implied. It also
		// catches the case flattening alone does not: path.Base("../..") is "..",
		// which would join to staging's PARENT.
		target, err := stagedPath(staging, name)
		if err != nil {
			return err
		}

		if f.IsEncrypted() {
			f.SetPassword(password)
		}

		if err := extractEntry(f, target); err != nil {
			return describeExtractFailure(zipPath, f, password, err)
		}

		exec := wantsExecutableBit(scope, uint32(f.Mode().Perm()))
		if exec {
			if err := markExecutable(target); err != nil {
				return fmt.Errorf("couldn't make %s runnable: %w", name, err)
			}
		}
		staged = append(staged, stagedEntry{name: name, exec: exec})
	}

	for _, e := range staged {
		src := filepath.Join(staging, e.name)
		dst := filepath.Join(destDir, e.name)
		// Windows won't rename onto an existing file, and it won't replace one
		// that's still running either — which is the actionable case here.
		os.Remove(dst)
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("couldn't replace %s — close the game and Mephi, then try again: %w", e.name, err)
		}
		// Rename carries the mode across, but a fallback copy would not, and the
		// cost of being wrong is a binary that cannot start.
		if e.exec {
			if err := markExecutable(dst); err != nil {
				return fmt.Errorf("couldn't make %s runnable: %w", e.name, err)
			}
		}
	}
	return nil
}

// stagedPath joins name onto staging and verifies the result stays inside it.
//
// name is already a base name, so this should never fail — that is the point.
// It is a cheap assertion on the one invariant that stops an archive entry from
// writing outside the install directory, and it fails closed if a future change
// stops flattening.
func stagedPath(staging, name string) (string, error) {
	target := filepath.Join(staging, name)
	rel, err := filepath.Rel(staging, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("the update archive contains an unsafe file path (%q) and was not installed", name)
	}
	return target, nil
}

// extractEntry writes a single archive entry to destPath.
//
// Split out so the file handles are closed on every path; the previous inline
// version leaked the reader whenever os.Create failed.
func extractEntry(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// describeExtractFailure turns a raw decompression error into something a user
// can act on.
//
// "flate: corrupt input before offset 5" is what a wrong ZipCrypto password
// looks like, because that path never checks the password — the bytes simply
// decrypt to noise. Reporting it verbatim tells the user nothing, so the
// encryption header is re-checked here to say which of the two it actually is.
func describeExtractFailure(zipPath string, f *zip.File, password string, err error) error {
	if !f.IsEncrypted() {
		return fmt.Errorf("the update archive is damaged (%s could not be unpacked) — please try the update again: %w", filepath.Base(f.Name), err)
	}
	if errors.Is(err, zip.ErrPassword) || wrongZipCryptoPassword(zipPath, f, password) {
		return fmt.Errorf("the update archive couldn't be unlocked — this launcher's archive password doesn't match the published update. Reinstall the launcher from the official download")
	}
	return fmt.Errorf("the update archive is damaged (%s could not be unpacked) — please try the update again: %w", filepath.Base(f.Name), err)
}

// wrongZipCryptoPassword reports whether password positively fails the PKWARE
// encryption-header check on a legacy-encrypted entry.
//
// Every ZipCrypto entry is prefixed with a 12-byte header whose last byte is a
// check value; yeka/zip discards those bytes without looking at them. Depending
// on the writer the byte is the high byte of the CRC-32 or of the mod time, so
// both are accepted — this only has to separate "wrong password" from "damaged
// download", and a false negative just falls back to the generic message.
func wrongZipCryptoPassword(zipPath string, f *zip.File, password string) bool {
	offset, err := f.DataOffset()
	if err != nil {
		return false
	}

	fh, err := os.Open(zipPath)
	if err != nil {
		return false
	}
	defer fh.Close()

	var header [12]byte
	if _, err := io.ReadFull(io.NewSectionReader(fh, offset, int64(len(header))), header[:]); err != nil {
		return false
	}

	plain := zip.NewZipCrypto([]byte(password)).Decrypt(header[:])
	check := plain[len(plain)-1]
	return check != byte(f.CRC32>>24) && check != byte(f.ModifiedTime>>8)
}
