package updateartifact

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knothhe/odds/source/internal/buildinfo"
)

func fixture(t *testing.T, channel string, unsafe bool) (string, string, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	tw := tar.NewWriter(gz)
	commit := strings.Repeat("a", 40)
	record := buildinfo.Identity{Schema: 1, Name: "odds", BundleID: "com.knothhe.odds", Version: "0.1.1", Channel: channel, Executor: "ci", Commit: commit, Target: "darwin-arm64", Source: "source-build"}
	data, _ := json.Marshal(record)
	entries := []struct {
		name string
		data []byte
		mode int64
	}{{"odds.app/Contents/Resources/build-info.json", data, 0644}, {"odds.app/Contents/MacOS/odds", []byte("fixture executable"), 0755}}
	for _, entry := range entries {
		if err = tw.WriteHeader(&tar.Header{Name: entry.name, Mode: entry.mode, Size: int64(len(entry.data)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err = tw.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if unsafe {
		if err = tw.WriteHeader(&tar.Header{Name: "odds.app/Contents/escape", Linkname: "/tmp/outside", Typeflag: tar.TypeSymlink}); err != nil {
			t.Fatal(err)
		}
	}
	if err = tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = gz.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	archive := compressed.Bytes()
	hash := sha256.Sum256(archive)
	manifest := Manifest{Version: "0.1.1", URL: "https://github.com/knothhe/odds/releases/download/v0.1.1/odds-0.1.1-darwin-arm64.tar.gz", Size: int64(len(archive)), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, hash[:]))}
	encoded, _ := json.Marshal(manifest)
	if err = os.WriteFile(filepath.Join(dir, "odds-0.1.1-darwin-arm64.tar.gz"), archive, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, "update-darwin-arm64.json"), encoded, 0600); err != nil {
		t.Fatal(err)
	}
	return dir, commit, base64.StdEncoding.EncodeToString(pub)
}

func TestValidSignedArchive(t *testing.T) {
	dir, commit, key := fixture(t, "release", false)
	if err := Verify(dir, "0.1.1", commit, "knothhe/odds", key); err != nil {
		t.Fatal(err)
	}
}
func TestTamperedAndWrongKeyArchivesAreRejected(t *testing.T) {
	dir, commit, key := fixture(t, "release", false)
	if err := Verify(dir, "0.1.1", commit, "other/repo", key); err == nil {
		t.Fatal("wrong repository accepted")
	}
	wrong, _, _ := ed25519.GenerateKey(rand.Reader)
	if err := Verify(dir, "0.1.1", commit, "knothhe/odds", base64.StdEncoding.EncodeToString(wrong)); err == nil {
		t.Fatal("wrong key accepted")
	}
	file := filepath.Join(dir, "odds-0.1.1-darwin-arm64.tar.gz")
	data, _ := os.ReadFile(file)
	data[10] ^= 1
	if err := os.WriteFile(file, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Verify(dir, "0.1.1", commit, "knothhe/odds", key); err == nil {
		t.Fatal("tampered archive accepted")
	}
}
func TestSignedWrongIdentityAndUnsafeEntriesAreRejected(t *testing.T) {
	for _, tc := range []struct {
		channel string
		unsafe  bool
	}{{"dev", false}, {"release", true}} {
		dir, commit, key := fixture(t, tc.channel, tc.unsafe)
		if err := Verify(dir, "0.1.1", commit, "knothhe/odds", key); err == nil {
			t.Fatalf("accepted unsafe/wrong-channel archive: %+v", tc)
		}
	}
}
