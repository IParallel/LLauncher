// Package client fetches and verifies the signed release manifest that tells the
// launcher which client bundle to install.
//
// This replaces the old unauthenticated version.json + baked-in zip password.
// The password was only ever confidentiality, never integrity — it authenticated
// nothing, and the same password ships inside every released binary. Integrity
// now comes from two independent checks:
//
//  1. The manifest is ECDSA-P256 signed by the server. The public key is pinned
//     below, so redirecting the API to a hostile host produces a manifest that
//     fails to verify.
//  2. The downloaded archive is SHA-256 checked against the hash the signed
//     manifest commits to, so a tampered CDN payload is rejected even though the
//     manifest itself was genuine.
package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// ManifestURL is the signed client manifest endpoint.
const ManifestURL = "https://limbonia.ibello.cc/api/manifest"

// ChangelogURL is the public Companion changelog. Unsigned on purpose: it is
// display text, not a security boundary, and a tampered changelog cannot cause
// anything to be installed.
const ChangelogURL = "https://limbonia.ibello.cc/api/changelog"

// pinnedPubKey is the server's ECDSA-P256 public key as X||Y (64 bytes).
//
// This is the SAME key the C++ client embeds (AuthManager.cpp kObfPubX/kObfPubY)
// and that the backend derives from routes/sign.go. Verified to match at the time
// this was written. If the server's signing key is ever rotated, this must be
// updated in lockstep or every launcher stops updating.
var pinnedPubKey = [64]byte{
	0x4b, 0x0d, 0x30, 0x47, 0x17, 0x28, 0x1e, 0xbf, 0x7d, 0x36, 0x2a, 0xa5,
	0x59, 0x03, 0x64, 0x03, 0xd1, 0x81, 0x20, 0xda, 0x15, 0x7d, 0x9a, 0x93,
	0xac, 0xef, 0xde, 0xd7, 0x62, 0x6a, 0xa1, 0x3b, 0x4b, 0xb2, 0x74, 0x33,
	0x8c, 0xfd, 0xf1, 0xfc, 0xbd, 0xc6, 0x68, 0xd0, 0xd3, 0x1a, 0x6e, 0x4b,
	0x26, 0x0c, 0x66, 0xa0, 0x42, 0xce, 0xb7, 0xbb, 0x19, 0x7c, 0x3a, 0x6a,
	0x89, 0xf4, 0x47, 0xbf,
}

// httpClient has an explicit timeout. The stdlib default has none, which let a
// hung connection wedge the launcher's startup update check indefinitely.
var httpClient = &http.Client{Timeout: 30 * time.Second}

// ManifestFile is the artifact half of the manifest.
type ManifestFile struct {
	Version string `json:"version"`
	URL     string `json:"url"`
	Sha256  string `json:"sha256"`
	Size    int64  `json:"size_bytes"`
}

// Manifest is the signed response from /api/manifest.
type Manifest struct {
	Client           ManifestFile `json:"client"`
	MinClientVersion string       `json:"min_client_version"`
	// Kind is the update type chosen by the admin: "hotfix", "feature" or
	// "major". Signature-covered so it can't be altered in transit.
	Kind              string `json:"kind"`
	ChangelogLatestID string `json:"changelog_latest_id"`
	Sig               string `json:"sig"`
}

// ChangelogEntry is one published update, for the What's New panel.
type ChangelogEntry struct {
	ReleaseID   string `json:"release_id"`
	Version     string `json:"version"`
	PublishedAt string `json:"published_at"`
	Title       string `json:"title"`
	BodyMd      string `json:"body_md"`
}

// canonicalMessage rebuilds the exact bytes the server signed. Must stay
// byte-for-byte identical to routes/sign.go::canonicalManifestMessage.
//
// The "manifest-v1" prefix is domain separation: it stops a captured verify-key
// signature from being replayed as a manifest, since both use the same key.
func canonicalMessage(m *Manifest) []byte {
	return []byte("manifest-v1\n" +
		m.Client.Version + "\n" +
		m.Client.URL + "\n" +
		m.Client.Sha256 + "\n" +
		strconv.FormatInt(m.Client.Size, 10) + "\n" +
		m.MinClientVersion + "\n" +
		m.Kind)
}

// Verify checks the manifest signature against the pinned public key.
func (m *Manifest) Verify() error {
	sig, err := base64.StdEncoding.DecodeString(m.Sig)
	if err != nil {
		return fmt.Errorf("manifest signature is not valid base64: %w", err)
	}
	if len(sig) != 64 {
		return fmt.Errorf("manifest signature is %d bytes, expected 64", len(sig))
	}

	pub := &ecdsa.PublicKey{
		Curve: elliptic.P256(),
		X:     new(big.Int).SetBytes(pinnedPubKey[:32]),
		Y:     new(big.Int).SetBytes(pinnedPubKey[32:]),
	}

	digest := sha256.Sum256(canonicalMessage(m))
	r := new(big.Int).SetBytes(sig[:32])
	s := new(big.Int).SetBytes(sig[32:])

	if !ecdsa.Verify(pub, digest[:], r, s) {
		return fmt.Errorf("manifest signature does not verify — refusing to install")
	}
	return nil
}

// FetchManifest retrieves and verifies the manifest. It returns an error rather
// than an unverified manifest: an unsigned or badly-signed manifest is
// indistinguishable from an attack, so there is no safe way to use one.
func FetchManifest() (*Manifest, error) {
	res, err := httpClient.Get(ManifestURL)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("manifest request failed with status %d", res.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return nil, err
	}

	var m Manifest
	if err := json.Unmarshal(body, &m); err != nil {
		return nil, fmt.Errorf("manifest is not valid JSON: %w", err)
	}
	if err := m.Verify(); err != nil {
		return nil, err
	}
	return &m, nil
}

// FetchChangelog retrieves the public changelog for the What's New panel.
// A failure here is non-fatal — the caller shows nothing rather than blocking.
func FetchChangelog(limit int) ([]ChangelogEntry, error) {
	if limit < 1 || limit > 50 {
		limit = 5
	}
	res, err := httpClient.Get(fmt.Sprintf("%s?limit=%d", ChangelogURL, limit))
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("changelog request failed with status %d", res.StatusCode)
	}

	var payload struct {
		Items []ChangelogEntry `json:"items"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	return payload.Items, nil
}

// VerifyFileSha256 checks a file on disk against an expected hex digest.
//
// Compares lowercase hex strings rather than raw bytes so a server that emits
// uppercase can't cause a spurious mismatch.
func VerifyFileSha256(path, expectedHex string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}

	got := hex.EncodeToString(hasher.Sum(nil))
	want := strings.ToLower(strings.TrimSpace(expectedHex))
	if got != want {
		return fmt.Errorf("downloaded file failed its integrity check (expected %s, got %s)", want, got)
	}
	return nil
}

// NeedsUpdate reports whether the installed client version is older than what the
// manifest advertises.
func NeedsUpdate(installed string, m *Manifest) bool {
	if installed == "" {
		return true
	}
	return CompareSemver(installed, m.Client.Version) < 0
}

// BelowMinimum reports whether the installed version is below the manifest's
// hard floor, meaning the update is mandatory rather than optional.
func BelowMinimum(installed string, m *Manifest) bool {
	if m.MinClientVersion == "" {
		return false
	}
	if installed == "" {
		return true
	}
	return CompareSemver(installed, m.MinClientVersion) < 0
}

// CompareSemver returns -1, 0 or 1 comparing two "1.2.3" strings numerically.
//
// String equality was the old behaviour and was wrong in two ways: it reported an
// update whenever a dev build's version differed at all (an update loop), and it
// treated "1.10.0" and "1.9.0" as merely different rather than ordered. A
// pre-release suffix is ignored for ordering.
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
	v = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(v), "v"))
	// Drop any build/pre-release suffix: "1.2.3-beta.1", "1.2.3+meta", "1.98.1:F3".
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
