package updateartifact

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseSignatureOnlyVerification(t *testing.T) {
	data, manifest, key := signedImage(t)
	directory := t.TempDir()
	encoded, _ := json.Marshal(manifest)
	os.WriteFile(filepath.Join(directory, "update-darwin-arm64.json"), encoded, 0600)
	os.WriteFile(filepath.Join(directory, "odds-1.1.0-arm64.dmg"), data, 0600)
	if err := VerifyRelease(directory, "1.1.0", "", "knothhe/odds", key, false); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRelease(directory, "1.1.0", "", "other/repo", key, false); err == nil {
		t.Fatal("wrong repository accepted")
	}
	// Authentic bytes alone cannot authorize an equal or older installed version.
	for _, current := range []string{"1.1.0", "1.2.0", "1.1.0+build", "invalid"} {
		if err := VerifyArchive(bytes.NewReader(data), manifest, current, key); err == nil {
			t.Fatal("non-upgrade accepted")
		}
	}
}
