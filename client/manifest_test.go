package client

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
)

// These tests guard a cross-repo contract: Limbonia-Page signs the manifest and
// this package verifies it. If the two canonical messages drift apart, every
// launcher silently stops updating — a failure that would only show up in the
// field. The layout literal below is duplicated verbatim in the backend's
// routes/sign_test.go and in Mephi's update_test.go; all three must agree.
//
// SECURITY: this repository is PUBLIC. The server's private signing key must
// never appear here. The verification logic is exercised with a throwaway
// keypair generated per-test, and the claim "our pinned key is the real server
// key" is proven with a signature FIXTURE produced by the server — a signature
// is public data and reveals nothing about the private scalar.

// fixtureSig was produced by Limbonia-Page's signManifest() over fixtureManifest
// using the production key. Regenerate it if the server key is ever rotated.
const fixtureSig = "n9StqyFPVC7Fc0a8sZd8c3JJig9KPfa29J4HzFoMlxwp/Z6D0mhTiVjLC0gHGPzBidUavOnNY2N3GNVNMQzS2w=="

func fixtureManifest() *Manifest {
	return &Manifest{
		Client: ManifestFile{
			Version: "1.99.0",
			URL:     "https://files.ibello.cc/Limbonia-1.99.0.zip",
			Sha256:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			Size:    20 * 1024 * 1024,
		},
		MinClientVersion: "1.98.0",
		Kind:             "feature",
	}
}

// withEphemeralKey swaps the pinned key for a freshly generated one for the
// duration of a test, and returns the matching private key for signing.
func withEphemeralKey(t *testing.T) *ecdsa.PrivateKey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	original := pinnedPubKey
	t.Cleanup(func() { pinnedPubKey = original })

	var swapped [64]byte
	priv.PublicKey.X.FillBytes(swapped[:32])
	priv.PublicKey.Y.FillBytes(swapped[32:])
	pinnedPubKey = swapped

	return priv
}

// signWith reproduces routes/sign.go::signManifest with an arbitrary key.
func signWith(t *testing.T, priv *ecdsa.PrivateKey, m *Manifest) string {
	t.Helper()
	digest := sha256.Sum256(canonicalMessage(m))
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest[:])
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	out := make([]byte, 64)
	r.FillBytes(out[:32])
	s.FillBytes(out[32:])
	return base64.StdEncoding.EncodeToString(out)
}

// Proves the key pinned in manifest.go is the one the server actually signs
// with. If the server key is rotated without updating the pin (or vice versa),
// this fails and every launcher would otherwise have silently stopped updating.
func TestPinnedKeyVerifiesRealServerSignature(t *testing.T) {
	m := fixtureManifest()
	m.Sig = fixtureSig
	if err := m.Verify(); err != nil {
		t.Fatalf("the pinned key rejected a genuine server signature: %v", err)
	}
}

func TestVerifyAcceptsGenuineSignature(t *testing.T) {
	priv := withEphemeralKey(t)
	m := fixtureManifest()
	m.Sig = signWith(t, priv, m)
	if err := m.Verify(); err != nil {
		t.Fatalf("a genuine signature was rejected: %v", err)
	}
}

// Each mutation is an attack the signature exists to stop: redirect the
// download, swap the payload, lie about the size, forge a version, lower the
// update floor, or downplay how significant the update is.
func TestVerifyRejectsTampering(t *testing.T) {
	cases := map[string]func(*Manifest){
		"redirected url":  func(m *Manifest) { m.Client.URL = "https://evil.example/payload.zip" },
		"swapped hash":    func(m *Manifest) { m.Client.Sha256 = "00000000000000000000000000000000000000000000000000000000deadbeef" },
		"inflated size":   func(m *Manifest) { m.Client.Size += 1 },
		"forged version":  func(m *Manifest) { m.Client.Version = "9.9.9" },
		"lowered floor":   func(m *Manifest) { m.MinClientVersion = "0.0.1" },
		"downgraded kind": func(m *Manifest) { m.Kind = "hotfix" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			priv := withEphemeralKey(t)
			m := fixtureManifest()
			m.Sig = signWith(t, priv, m)
			mutate(m)
			if err := m.Verify(); err == nil {
				t.Error("tampered manifest verified, it must not")
			}
		})
	}
}

// A signature from ANY other key must be rejected, which is what stops someone
// pointing the launcher at a host they control and serving their own manifest.
func TestVerifyRejectsForeignKey(t *testing.T) {
	foreign, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	m := fixtureManifest()
	m.Sig = signWith(t, foreign, m) // signed by an attacker, verified against the real pin
	if err := m.Verify(); err == nil {
		t.Fatal("a manifest signed by a foreign key verified — the pin is not being enforced")
	}
}

func TestVerifyRejectsMalformedSignatures(t *testing.T) {
	for name, sig := range map[string]string{
		"empty":        "",
		"not base64":   "!!!!not-base64!!!!",
		"wrong length": base64.StdEncoding.EncodeToString(make([]byte, 32)),
		"all zeroes":   base64.StdEncoding.EncodeToString(make([]byte, 64)),
	} {
		m := fixtureManifest()
		m.Sig = sig
		if err := m.Verify(); err == nil {
			t.Errorf("%s: malformed signature accepted", name)
		}
	}
}

// Byte-for-byte lock on the wire format shared with the backend and Mephi.
func TestCanonicalMessageLayout(t *testing.T) {
	m := &Manifest{
		Client:           ManifestFile{Version: "1.2.3", URL: "https://x/y.zip", Sha256: "deadbeef", Size: 42},
		MinClientVersion: "1.0.0",
		Kind:             "hotfix",
	}
	got := string(canonicalMessage(m))
	want := "manifest-v1\n1.2.3\nhttps://x/y.zip\ndeadbeef\n42\n1.0.0\nhotfix"
	if got != want {
		t.Errorf("canonical message drifted from the server's:\n got: %q\nwant: %q", got, want)
	}
}

func TestVerifyFileSha256(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	const helloSum = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"

	if err := VerifyFileSha256(p, helloSum); err != nil {
		t.Errorf("correct hash rejected: %v", err)
	}
	// Case-insensitive: an uppercase digest from the server must still match.
	if err := VerifyFileSha256(p, "2CF24DBA5FB0A30E26E83B2AC5B9E29E1B161E5C1FA7425E73043362938B9824"); err != nil {
		t.Errorf("uppercase hash rejected: %v", err)
	}
	if err := VerifyFileSha256(p, "0000000000000000000000000000000000000000000000000000000000000000"); err == nil {
		t.Error("wrong hash accepted")
	}
	if err := VerifyFileSha256(filepath.Join(dir, "missing.bin"), helloSum); err == nil {
		t.Error("missing file accepted")
	}
}

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.1", "1.0.0", 1},
		{"1.0.0", "1.0.1", -1},
		{"1.10.0", "1.9.0", 1}, // numeric, not lexicographic — the old bug
		{"2.0.0", "1.99.99", 1},
		{"v1.99.0", "1.99.0", 0},     // tag prefix tolerated
		{"1.98.1:F3", "1.98.1", 0},   // legacy client suffix ignored
		{"1.0.0-beta.1", "1.0.0", 0}, // pre-release ignored for ordering
	}
	for _, tc := range cases {
		if got := CompareSemver(tc.a, tc.b); got != tc.want {
			t.Errorf("CompareSemver(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestNeedsUpdateAndMinimum(t *testing.T) {
	m := fixtureManifest() // client 1.99.0, floor 1.98.0

	if !NeedsUpdate("", m) {
		t.Error("a fresh install (no version) should need an update")
	}
	if !NeedsUpdate("1.98.5", m) {
		t.Error("an older install should need an update")
	}
	if NeedsUpdate("1.99.0", m) {
		t.Error("a current install should not need an update")
	}
	// A dev build ahead of the manifest must not trigger a downgrade loop —
	// this is what string equality got wrong before.
	if NeedsUpdate("2.0.0", m) {
		t.Error("a newer install must not be told to update")
	}

	if !BelowMinimum("1.97.0", m) {
		t.Error("below the floor should be mandatory")
	}
	if BelowMinimum("1.98.0", m) {
		t.Error("exactly at the floor is acceptable")
	}
	if BelowMinimum("1.99.0", m) {
		t.Error("above the floor is acceptable")
	}
}

// Guard against the private signing key ever being reintroduced into this
// PUBLIC repository. It signs both release manifests and verify-key auth
// responses, so a leak would let anyone forge license validations.
func TestNoPrivateKeyMaterialInThisPackage(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	// Split so this detector's own source doesn't match itself. The public key's
	// coordinates are expected in this package; the private scalar never is.
	privateScalarPrefix := "ff4822a4" + "0f5c3e78"

	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".go" {
			continue
		}
		body, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		if bytesContains(body, privateScalarPrefix) {
			t.Errorf("%s contains the server's PRIVATE signing key — it must never be committed to this public repo", e.Name())
		}
	}
}

func bytesContains(haystack []byte, needle string) bool {
	n := []byte(needle)
	for i := 0; i+len(n) <= len(haystack); i++ {
		if string(haystack[i:i+len(n)]) == needle {
			return true
		}
	}
	return false
}
