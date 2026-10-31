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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knothhe/odds/source/internal/buildinfo"
)

type archiveEntry struct {
	name string
	data []byte
	mode int64
	kind byte
	size int64
}

func releaseIdentity() buildinfo.Identity {
	return buildinfo.Identity{Schema: 1, Name: "odds", BundleID: "com.knothhe.odds", Version: "1.1.0", Channel: "release", Executor: "ci", Commit: strings.Repeat("b", 40), Target: "darwin-arm64", Source: "source-build"}
}

func archiveEntries(t *testing.T, identity buildinfo.Identity) []archiveEntry {
	t.Helper()
	metadata, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	return []archiveEntry{
		{name: "odds.app/Contents/Resources/build-info.json", data: metadata, mode: 0644, kind: tar.TypeReg},
		{name: "odds.app/Contents/MacOS/odds", data: []byte("fixture executable"), mode: 0755, kind: tar.TypeReg},
	}
}

func signedArchive(t *testing.T, version string, entries []archiveEntry) ([]byte, Manifest, string) {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	truncated := false
	for _, entry := range entries {
		size := int64(len(entry.data))
		if entry.size != 0 {
			size = entry.size
		}
		header := &tar.Header{Name: entry.name, Mode: entry.mode, Typeflag: entry.kind, Size: size}
		if entry.kind == tar.TypeSymlink || entry.kind == tar.TypeLink {
			header.Linkname = "/tmp/outside"
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write(entry.data); err != nil {
			t.Fatal(err)
		}
		truncated = size != int64(len(entry.data))
	}
	if err := writer.Close(); err != nil && !truncated {
		t.Fatal(err)
	}
	if err := compressed.Close(); err != nil {
		t.Fatal(err)
	}
	archive := buffer.Bytes()
	hash := sha256.Sum256(archive)
	manifest := Manifest{
		Version:   version,
		URL:       "https://github.com/knothhe/odds/releases/download/v" + version + "/odds-" + version + "-darwin-arm64.tar.gz",
		Size:      int64(len(archive)),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, hash[:])),
	}
	return archive, manifest, base64.StdEncoding.EncodeToString(pub)
}

func TestVerifyArchiveAcceptsAuthenticUpgrade(t *testing.T) {
	identity := releaseIdentity()
	archive, manifest, key := signedArchive(t, identity.Version, archiveEntries(t, identity))
	if err := ValidateManifest(manifest, "1.0.0", "knothhe/odds"); err != nil {
		t.Fatal(err)
	}
	reader := bytes.NewReader(archive)
	// Verification must start at the beginning regardless of the reader cursor.
	if _, err := reader.Seek(7, 0); err != nil {
		t.Fatal(err)
	}
	if err := VerifyArchive(reader, manifest, "1.0.0", key); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyArchiveRejectsReplayedReleaseWithInflatedManifest(t *testing.T) {
	identity := releaseIdentity()
	identity.Version = "1.0.0"
	archive, manifest, key := signedArchive(t, "999.0.0", archiveEntries(t, identity))
	// The attacker edits only the unsigned manifest and retains a valid signature
	// over the old release archive. Matching the actual bundle version is required.
	if err := ValidateManifest(manifest, "2.0.0", "knothhe/odds"); err != nil {
		t.Fatal(err)
	}
	if err := VerifyArchive(bytes.NewReader(archive), manifest, "2.0.0", key); err == nil {
		t.Fatal("signed old archive accepted under an inflated manifest version")
	}
}

func TestVerifyArchiveRequiresStrictlyNewerVersion(t *testing.T) {
	identity := releaseIdentity()
	archive, manifest, key := signedArchive(t, identity.Version, archiveEntries(t, identity))
	for _, current := range []string{"1.1.0", "1.1.0+different-build", "1.2.0", "2.0.0", "01.0.0", "v1.0.0", ""} {
		t.Run(current, func(t *testing.T) {
			if err := VerifyArchive(bytes.NewReader(archive), manifest, current, key); err == nil {
				t.Fatalf("accepted upgrade from %q to %q", current, manifest.Version)
			}
		})
	}
	if err := VerifyArchive(bytes.NewReader(archive), manifest, "1.1.0-rc.1", key); err != nil {
		t.Fatalf("release must be newer than its prerelease: %v", err)
	}
}

func TestVerifyArchiveRejectsWrongIdentityAndExecutable(t *testing.T) {
	cases := map[string]func(*buildinfo.Identity){
		"schema":           func(value *buildinfo.Identity) { value.Schema = 2 },
		"name":             func(value *buildinfo.Identity) { value.Name = "another-app" },
		"bundle":           func(value *buildinfo.Identity) { value.BundleID = "com.knothhe.odds.dev" },
		"version":          func(value *buildinfo.Identity) { value.Version = "1.0.1" },
		"channel":          func(value *buildinfo.Identity) { value.Channel = "dev" },
		"executor":         func(value *buildinfo.Identity) { value.Executor = "local" },
		"commit":           func(value *buildinfo.Identity) { value.Commit = "unknown" },
		"short-commit":     func(value *buildinfo.Identity) { value.Commit = strings.Repeat("b", 39) },
		"nonhex-commit":    func(value *buildinfo.Identity) { value.Commit = strings.Repeat("z", 40) },
		"uppercase-commit": func(value *buildinfo.Identity) { value.Commit = strings.Repeat("B", 40) },
		"target":           func(value *buildinfo.Identity) { value.Target = "darwin-amd64" },
		"source":           func(value *buildinfo.Identity) { value.Source = "prebuilt" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			identity := releaseIdentity()
			mutate(&identity)
			archive, manifest, key := signedArchive(t, "1.1.0", archiveEntries(t, identity))
			if err := VerifyArchive(bytes.NewReader(archive), manifest, "1.0.0", key); err == nil {
				t.Fatal("accepted wrong archive identity")
			}
		})
	}
	for _, executable := range []string{"missing", "unexecutable", "directory"} {
		t.Run(executable, func(t *testing.T) {
			entries := archiveEntries(t, releaseIdentity())
			switch executable {
			case "missing":
				entries = entries[:1]
			case "unexecutable":
				entries[1].mode = 0644
			case "directory":
				entries[1].kind = tar.TypeDir
				entries[1].data = nil
			}
			archive, manifest, key := signedArchive(t, "1.1.0", entries)
			if err := VerifyArchive(bytes.NewReader(archive), manifest, "1.0.0", key); err == nil {
				t.Fatal("accepted missing executable")
			}
		})
	}
}

func TestVerifyArchiveChecksSignatureAndExactSize(t *testing.T) {
	archive, manifest, key := signedArchive(t, "1.1.0", archiveEntries(t, releaseIdentity()))
	wrong, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyArchive(bytes.NewReader(archive), manifest, "1.0.0", base64.StdEncoding.EncodeToString(wrong)); err == nil {
		t.Fatal("wrong key accepted")
	}
	if err := VerifyArchive(bytes.NewReader(archive), manifest, "1.0.0", "invalid-key"); err == nil {
		t.Fatal("malformed key accepted")
	}
	tampered := append([]byte(nil), archive...)
	tampered[10] ^= 1
	if err := VerifyArchive(bytes.NewReader(tampered), manifest, "1.0.0", key); err == nil {
		t.Fatal("tampered archive accepted")
	}
	for _, size := range []int64{0, -1, manifest.Size - 1, manifest.Size + 1, maxArchiveSize + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			value := manifest
			value.Size = size
			if err := VerifyArchive(bytes.NewReader(archive), value, "1.0.0", key); err == nil {
				t.Fatal("wrong archive size accepted")
			}
		})
	}
}

func TestValidateManifestContract(t *testing.T) {
	_, manifest, _ := signedArchive(t, "1.1.0", archiveEntries(t, releaseIdentity()))
	for _, current := range []string{"1.0.0", "1.1.0", "1.2.0"} {
		if err := ValidateManifest(manifest, current, "knothhe/odds"); err != nil {
			t.Fatalf("valid feed rejected for current %s: %v", current, err)
		}
	}
	cases := map[string]func(*Manifest){
		"version":         func(value *Manifest) { value.Version = "v1.1.0" },
		"repository":      func(value *Manifest) { value.URL = strings.Replace(value.URL, "knothhe/odds", "other/odds", 1) },
		"http":            func(value *Manifest) { value.URL = strings.Replace(value.URL, "https:", "http:", 1) },
		"query":           func(value *Manifest) { value.URL += "?download=1" },
		"tag":             func(value *Manifest) { value.URL = strings.Replace(value.URL, "/v1.1.0/", "/v1.0.0/", 1) },
		"architecture":    func(value *Manifest) { value.URL = strings.Replace(value.URL, "arm64", "amd64", 1) },
		"zero-size":       func(value *Manifest) { value.Size = 0 },
		"negative-size":   func(value *Manifest) { value.Size = -1 },
		"oversize":        func(value *Manifest) { value.Size = maxArchiveSize + 1 },
		"signature":       func(value *Manifest) { value.Signature = "invalid" },
		"short-signature": func(value *Manifest) { value.Signature = base64.StdEncoding.EncodeToString(make([]byte, 63)) },
		"long-signature":  func(value *Manifest) { value.Signature = base64.StdEncoding.EncodeToString(make([]byte, 65)) },
		"deltas":          func(value *Manifest) { value.Deltas = []json.RawMessage{json.RawMessage(`{}`)} },
		"null-delta":      func(value *Manifest) { value.Deltas = []json.RawMessage{json.RawMessage(`null`)} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			value := manifest
			mutate(&value)
			if err := ValidateManifest(value, "1.0.0", "knothhe/odds"); err == nil {
				t.Fatal("invalid update manifest accepted")
			}
		})
	}
	if err := ValidateManifest(manifest, "invalid-current", "knothhe/odds"); err == nil {
		t.Fatal("invalid installed version accepted")
	}
}

func TestVerifyArchiveRejectsUnsafeEntries(t *testing.T) {
	cases := []archiveEntry{
		{name: "../odds.app/escape", kind: tar.TypeReg},
		{name: "/odds.app/escape", kind: tar.TypeReg},
		{name: "another.app/escape", kind: tar.TypeReg},
		{name: "odds.app/../escape", kind: tar.TypeReg},
		{name: "odds.app/Contents/./escape", kind: tar.TypeReg},
		{name: "odds.app//escape", kind: tar.TypeReg},
		{name: "odds.app/Contents\\escape", kind: tar.TypeReg},
		{name: "odds.app/Contents/MacOS/odds", kind: tar.TypeReg},
		{name: "odds.app/Contents/escape", kind: tar.TypeSymlink},
		{name: "odds.app/Contents/escape", kind: tar.TypeLink},
		{name: "odds.app/Contents", kind: tar.TypeReg},
		{name: "odds.app", kind: tar.TypeReg},
	}
	for index, entry := range cases {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			entries := append(archiveEntries(t, releaseIdentity()), entry)
			archive, manifest, key := signedArchive(t, "1.1.0", entries)
			if err := VerifyArchive(bytes.NewReader(archive), manifest, "1.0.0", key); err == nil {
				t.Fatalf("unsafe entry accepted: %+v", entry)
			}
		})
	}
	entries := append([]archiveEntry{{name: "odds.app/Contents", kind: tar.TypeReg}}, archiveEntries(t, releaseIdentity())...)
	archive, manifest, key := signedArchive(t, "1.1.0", entries)
	if err := VerifyArchive(bytes.NewReader(archive), manifest, "1.0.0", key); err == nil {
		t.Fatal("file used as parent directory accepted")
	}
}

func TestVerifyArchiveRejectsCaseAliasedEntries(t *testing.T) {
	for _, entry := range []archiveEntry{
		{name: "odds.app/Contents/MacOS/ODDS", kind: tar.TypeReg, data: []byte("replacement executable"), mode: 0755},
		{name: "odds.app/contents/Resources/build-info.json", kind: tar.TypeReg, data: []byte(`{"version":"0.9.0"}`)},
		{name: "odds.app/contents", kind: tar.TypeReg},
	} {
		t.Run(entry.name, func(t *testing.T) {
			entries := append(archiveEntries(t, releaseIdentity()), entry)
			archive, manifest, key := signedArchive(t, "1.1.0", entries)
			if err := VerifyArchive(bytes.NewReader(archive), manifest, "1.0.0", key); err == nil {
				t.Fatal("case alias can change an already verified file or directory during extraction")
			}
		})
	}
	entries := append([]archiveEntry{{name: "odds.app/contents", kind: tar.TypeReg}}, archiveEntries(t, releaseIdentity())...)
	archive, manifest, key := signedArchive(t, "1.1.0", entries)
	if err := VerifyArchive(bytes.NewReader(archive), manifest, "1.0.0", key); err == nil {
		t.Fatal("case-aliased regular file accepted as a parent directory")
	}
}

func TestVerifyArchiveRejectsUnicodeCaseAliases(t *testing.T) {
	for _, entry := range []archiveEntry{
		{name: "odds.app/Contents/MacOS/oddſ", kind: tar.TypeReg, data: []byte("replacement executable"), mode: 0755},
		{name: "odds.app/Contents/MacOſ", kind: tar.TypeReg},
	} {
		t.Run(entry.name, func(t *testing.T) {
			entries := append(archiveEntries(t, releaseIdentity()), entry)
			archive, manifest, key := signedArchive(t, "1.1.0", entries)
			if err := VerifyArchive(bytes.NewReader(archive), manifest, "1.0.0", key); err == nil {
				t.Fatal("Unicode case alias can overwrite a verified executable or directory on macOS")
			}
		})
	}
}

func TestVerifyArchiveRejectsOversizedOrInvalidMetadata(t *testing.T) {
	for _, mode := range []string{"oversized", "trailing-json", "directory", "missing", "expanded-limit"} {
		t.Run(mode, func(t *testing.T) {
			entries := archiveEntries(t, releaseIdentity())
			switch mode {
			case "oversized":
				entries[0].data = bytes.Repeat([]byte(" "), 65537)
			case "trailing-json":
				entries[0].data = append(entries[0].data, []byte(` {}`)...)
			case "directory":
				entries[0].kind = tar.TypeDir
				entries[0].data = nil
			case "missing":
				entries = entries[1:]
			case "expanded-limit":
				entries = append(entries, archiveEntry{name: "odds.app/Contents/Resources/large", kind: tar.TypeReg, size: maxArchiveSize + 1})
			}
			archive, manifest, key := signedArchive(t, "1.1.0", entries)
			if err := VerifyArchive(bytes.NewReader(archive), manifest, "1.0.0", key); err == nil {
				t.Fatal("invalid archive metadata or size accepted")
			}
		})
	}
}

func TestCLIStillRequiresExactCommit(t *testing.T) {
	for _, version := range []string{"1.1.0", "1.1.0-rc.1+build.2"} {
		t.Run(version, func(t *testing.T) {
			identity := releaseIdentity()
			identity.Version = version
			archive, manifest, key := signedArchive(t, identity.Version, archiveEntries(t, identity))
			directory := t.TempDir()
			encoded, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "update-darwin-arm64.json"), encoded, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(directory, "odds-"+version+"-darwin-arm64.tar.gz"), archive, 0600); err != nil {
				t.Fatal(err)
			}
			if err := Verify(directory, version, identity.Commit, "knothhe/odds", key); err != nil {
				t.Fatal(err)
			}
			if err := Verify(directory, version, strings.Repeat("c", 40), "knothhe/odds", key); err == nil {
				t.Fatal("CLI accepted an unexpected release commit")
			}
		})
	}
}

func TestVerifyArchiveRejectsSignedBrokenGzip(t *testing.T) {
	original, manifest, _ := signedArchive(t, "1.1.0", archiveEntries(t, releaseIdentity()))
	for _, damage := range []string{"missing-trailer", "partial-trailer", "checksum"} {
		t.Run(damage, func(t *testing.T) {
			archive := append([]byte(nil), original...)
			switch damage {
			case "missing-trailer":
				archive = archive[:len(archive)-8]
			case "partial-trailer":
				archive = archive[:len(archive)-1]
			case "checksum":
				archive[len(archive)-8] ^= 1
			}
			public, private, err := ed25519.GenerateKey(rand.Reader)
			if err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(archive)
			value := manifest
			value.Size = int64(len(archive))
			value.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(private, digest[:]))
			if err := VerifyArchive(bytes.NewReader(archive), value, "1.0.0", base64.StdEncoding.EncodeToString(public)); err == nil {
				t.Fatal("signed archive with invalid gzip trailer accepted")
			}
		})
	}
}
