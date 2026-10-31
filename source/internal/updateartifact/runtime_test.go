package updateartifact

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knothhe/odds/source/internal/buildinfo"
)

func releaseIdentity() buildinfo.Identity {
	return buildinfo.Identity{Schema: 1, Name: "odds", BundleID: "com.knothhe.odds", Version: "1.1.0", Channel: "release", Executor: "ci", Commit: strings.Repeat("b", 40), Target: "darwin-arm64", Source: "source-build"}
}

func signedImage(t *testing.T) ([]byte, Manifest, string) {
	t.Helper()
	data := []byte("fixture disk image bytes")
	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	hash := sha256.Sum256(data)
	return data, Manifest{Version: "1.1.0", URL: "https://github.com/knothhe/odds/releases/download/v1.1.0/odds-1.1.0-arm64.dmg", Size: int64(len(data)), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(private, hash[:]))}, base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
}

func TestDMGSignatureAndSize(t *testing.T) {
	data, manifest, key := signedImage(t)
	reader := bytes.NewReader(data)
	reader.Seek(7, 0)
	if err := VerifySignature(reader, manifest, key); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []string{"tampered", "truncated", "size", "key", "signature"} {
		t.Run(scenario, func(t *testing.T) {
			payload, value, public := append([]byte(nil), data...), manifest, key
			switch scenario {
			case "tampered":
				payload[0] ^= 1
			case "truncated":
				payload = payload[:len(payload)-1]
			case "size":
				value.Size++
			case "key":
				public = base64.StdEncoding.EncodeToString(make([]byte, 32))
			case "signature":
				value.Signature = "invalid"
			}
			if err := VerifySignature(bytes.NewReader(payload), value, public); err == nil {
				t.Fatal("invalid DMG authenticated")
			}
		})
	}
}

func writeBundle(t *testing.T, identity buildinfo.Identity) string {
	t.Helper()
	bundle := filepath.Join(t.TempDir(), "odds.app")
	for _, dir := range []string{"Contents/MacOS", "Contents/Resources"} {
		if err := os.MkdirAll(filepath.Join(bundle, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	data, _ := json.Marshal(identity)
	if err := os.WriteFile(filepath.Join(bundle, "Contents/Resources/build-info.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bundle, "Contents/MacOS/odds"), []byte("fixture executable"), 0755); err != nil {
		t.Fatal(err)
	}
	return bundle
}

func TestMountedBundleIdentity(t *testing.T) {
	identity := releaseIdentity()
	bundle := writeBundle(t, identity)
	if _, err := verifyBundle(bundle, identity.Version); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"channel", "version", "commit", "executor", "target", "bundle", "name", "schema", "source"} {
		t.Run(field, func(t *testing.T) {
			value := identity
			switch field {
			case "channel":
				value.Channel = "dev"
			case "version":
				value.Version = "1.0.0"
			case "commit":
				value.Commit = "unknown"
			case "executor":
				value.Executor = "local"
			case "target":
				value.Target = "darwin-amd64"
			case "bundle":
				value.BundleID = "com.knothhe.odds.dev"
			case "name":
				value.Name = "other"
			case "schema":
				value.Schema = 2
			case "source":
				value.Source = "unknown"
			}
			if _, err := verifyBundle(writeBundle(t, value), identity.Version); err == nil {
				t.Fatal("wrong identity accepted")
			}
		})
	}
}

func TestMountedBundleRejectsUnsafeFiles(t *testing.T) {
	for _, scenario := range []string{"link", "executable", "missing", "oversized-metadata", "invalid-json", "directory-metadata", "backslash", "expanded-limit"} {
		t.Run(scenario, func(t *testing.T) {
			bundle := writeBundle(t, releaseIdentity())
			exe := filepath.Join(bundle, "Contents/MacOS/odds")
			metadata := filepath.Join(bundle, "Contents/Resources/build-info.json")
			var err error
			switch scenario {
			case "link":
				err = os.Symlink("/tmp/outside", filepath.Join(bundle, "Contents/escape"))
			case "executable":
				err = os.Chmod(exe, 0644)
			case "missing":
				err = os.Remove(exe)
			case "oversized-metadata":
				err = os.WriteFile(metadata, bytes.Repeat([]byte(" "), 65537), 0644)
			case "invalid-json":
				err = os.WriteFile(metadata, []byte(`{} {}`), 0644)
			case "directory-metadata":
				os.Remove(metadata)
				err = os.Mkdir(metadata, 0755)
			case "backslash":
				err = os.WriteFile(filepath.Join(bundle, `Contents\escape`), nil, 0644)
			case "expanded-limit":
				file, e := os.Create(filepath.Join(bundle, "large"))
				if e != nil {
					t.Fatal(e)
				}
				err = file.Truncate(maxArchiveSize + 1)
				file.Close()
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := verifyBundle(bundle, "1.1.0"); err == nil {
				t.Fatal("unsafe bundle accepted")
			}
		})
	}
}

func TestPathCaseAliases(t *testing.T) {
	for _, pair := range [][2]string{{"Contents/MacOS/odds", "contents/macos/ODDS"}, {"MacOS/odds", "MacOſ/oddſ"}} {
		if pathKey(pair[0]) != pathKey(pair[1]) {
			t.Fatal("case alias not folded")
		}
	}
}

func TestValidateManifestContract(t *testing.T) {
	_, manifest, _ := signedImage(t)
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
