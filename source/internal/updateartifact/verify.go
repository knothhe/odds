// Package updateartifact verifies the exact native MyGo release artifact contract.
package updateartifact

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/knothhe/odds/source/internal/buildinfo"

	"github.com/knothhe/odds/source/internal/uicopy"
)

type Manifest struct {
	Version   string            `json:"version"`
	Notes     string            `json:"notes,omitempty"`
	Date      string            `json:"date,omitempty"`
	URL       string            `json:"url"`
	Size      int64             `json:"size"`
	Signature string            `json:"signature"`
	Deltas    []json.RawMessage `json:"deltas"`
}

const maxArchiveSize int64 = 1 << 30

var commitPattern = regexp.MustCompile(`^[a-f0-9]{40}$`)

// ValidateManifest checks the full-update contract before downloading anything.
// A current or older release is valid feed data; VerifyArchive alone authorizes
// an upgrade after authenticating and inspecting the downloaded DMG.
func ValidateManifest(manifest Manifest, current, repo string) error {
	if _, err := parseVersion(current); err != nil {
		return err
	}
	if err := validateManifest(manifest); err != nil {
		return err
	}
	name := "odds-" + manifest.Version + "-arm64.dmg"
	expectedURL := "https://github.com/" + repo + "/releases/download/v" + manifest.Version + "/" + name
	if manifest.URL != expectedURL {
		return fmt.Errorf("%s", uicopy.UpdateSourceMismatch())
	}
	return nil
}

func validateManifest(manifest Manifest) error {
	if _, err := parseVersion(manifest.Version); err != nil {
		return err
	}
	if manifest.Size <= 0 || manifest.Size > maxArchiveSize {
		return fmt.Errorf("%s", uicopy.UpdateSizeInvalid())
	}
	if len(manifest.Deltas) != 0 {
		return fmt.Errorf("%s", uicopy.UpdateDeltaUnsupported())
	}
	signature, err := base64.StdEncoding.DecodeString(manifest.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("%s", uicopy.UpdateSignatureInvalid())
	}
	return nil
}

// VerifyArchive authenticates a downloaded full update and checks that its
// release identity matches a strictly newer manifest version. The caller must
// separately bind the manifest URL to its repository with ValidateManifest.
func VerifyArchive(file io.ReadSeeker, manifest Manifest, current, public string) error {
	if err := validateManifest(manifest); err != nil {
		return err
	}
	candidate, _ := parseVersion(manifest.Version)
	installed, err := parseVersion(current)
	if err != nil {
		return err
	}
	if candidate.compare(installed) <= 0 {
		return fmt.Errorf("%s", uicopy.UpdateNotNewer())
	}
	_, err = verifyArchive(file, manifest, public)
	return err
}

func Verify(directory, version, commit, repo, public string) error {
	return VerifyRelease(directory, version, commit, repo, public, true)
}

func VerifyRelease(directory, version, commit, repo, public string, inspect bool) error {
	name := "odds-" + version + "-arm64.dmg"
	manifestPath := filepath.Join(directory, "update-darwin-arm64.json")
	manifestInfo, err := os.Lstat(manifestPath)
	if err != nil {
		return err
	}
	if !manifestInfo.Mode().IsRegular() || manifestInfo.Size() > 1<<20 {
		return fmt.Errorf("%s", uicopy.UpdateManifestInvalid())
	}
	manifestBytes, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	var manifest Manifest
	if err = json.Unmarshal(manifestBytes, &manifest); err != nil {
		return err
	}
	if manifest.Version != version {
		return fmt.Errorf("%s", uicopy.UpdateVersionMismatch())
	}
	if err := ValidateManifest(manifest, version, repo); err != nil {
		return err
	}
	archive := filepath.Join(directory, name)
	info, err := os.Lstat(archive)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() != manifest.Size || info.Size() > maxArchiveSize {
		return fmt.Errorf("%s", uicopy.UpdateFileMismatch())
	}
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	if !inspect {
		return VerifySignature(file, manifest, public)
	}
	identity, err := verifyArchive(file, manifest, public)
	if err != nil {
		return err
	}
	if identity.Commit != commit {
		return fmt.Errorf("%s", uicopy.UpdateCommitMismatch())
	}
	return nil
}

func verifySignature(file io.ReadSeeker, manifest Manifest, public string) (buildinfo.Identity, error) {
	var identity buildinfo.Identity
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(public))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return identity, fmt.Errorf("%s", uicopy.UpdatePublicKeyInvalid())
	}
	signature, err := base64.StdEncoding.DecodeString(manifest.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return identity, fmt.Errorf("%s", uicopy.UpdateSignatureInvalid())
	}
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return identity, err
	}
	if size != manifest.Size || size <= 0 || size > maxArchiveSize {
		return identity, fmt.Errorf("%s", uicopy.UpdateLengthMismatch())
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return identity, err
	}
	hash := sha256.New()
	read, err := io.Copy(hash, io.LimitReader(file, manifest.Size+1))
	if err != nil {
		return identity, err
	}
	if read != manifest.Size {
		return identity, fmt.Errorf("%s", uicopy.UpdateLengthChanged())
	}
	if !ed25519.Verify(key, hash.Sum(nil), signature) {
		return identity, fmt.Errorf("%s", uicopy.UpdateUnsigned())
	}
	return identity, nil
}

// VerifySignature authenticates the exact DMG bytes before any OS parser sees them.
func VerifySignature(file io.ReadSeeker, manifest Manifest, public string) error {
	if err := validateManifest(manifest); err != nil {
		return err
	}
	_, err := verifySignature(file, manifest, public)
	return err
}

func verifyArchive(file io.ReadSeeker, manifest Manifest, public string) (buildinfo.Identity, error) {
	if err := VerifySignature(file, manifest, public); err != nil {
		return buildinfo.Identity{}, err
	}
	return inspectDMG(file, manifest, public, "")
}
