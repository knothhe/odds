// Package updateartifact verifies the exact native MyGo release artifact contract.
package updateartifact

import (
	"archive/tar"
	"compress/gzip"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/knothhe/odds/source/internal/buildinfo"
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

func archivePathKey(name string) string {
	return strings.Map(func(char rune) rune {
		key := char
		// SimpleFold includes non-ASCII aliases of ASCII letters, such as
		// long s (ſ), which lowercasing alone leaves distinct from s.
		for next := unicode.SimpleFold(char); next != char; next = unicode.SimpleFold(next) {
			if next < key {
				key = next
			}
		}
		return key
	}, name)
}

// ValidateManifest checks the full-update contract before downloading anything.
// A current or older release is valid feed data; VerifyArchive alone authorizes
// an upgrade after authenticating and inspecting the downloaded archive.
func ValidateManifest(manifest Manifest, current, repo string) error {
	if _, err := parseVersion(current); err != nil {
		return err
	}
	if err := validateManifest(manifest); err != nil {
		return err
	}
	name := "odds-" + manifest.Version + "-darwin-arm64.tar.gz"
	expectedURL := "https://github.com/" + repo + "/releases/download/v" + manifest.Version + "/" + name
	if manifest.URL != expectedURL {
		return fmt.Errorf("manifest release source mismatch")
	}
	return nil
}

func validateManifest(manifest Manifest) error {
	if _, err := parseVersion(manifest.Version); err != nil {
		return err
	}
	if manifest.Size <= 0 || manifest.Size > maxArchiveSize {
		return fmt.Errorf("invalid or oversized update archive")
	}
	if len(manifest.Deltas) != 0 {
		return fmt.Errorf("delta updates are not supported")
	}
	signature, err := base64.StdEncoding.DecodeString(manifest.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return fmt.Errorf("invalid update signature encoding or size")
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
		return fmt.Errorf("update version must be newer than the installed version")
	}
	_, err = verifyArchive(file, manifest, public)
	return err
}

func Verify(directory, version, commit, repo, public string) error {
	name := "odds-" + version + "-darwin-arm64.tar.gz"
	manifestPath := filepath.Join(directory, "update-darwin-arm64.json")
	manifestInfo, err := os.Lstat(manifestPath)
	if err != nil {
		return err
	}
	if !manifestInfo.Mode().IsRegular() || manifestInfo.Size() > 1<<20 {
		return fmt.Errorf("invalid or oversized manifest")
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
		return fmt.Errorf("manifest version mismatch")
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
		return fmt.Errorf("archive size or file type mismatch")
	}
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	identity, err := verifyArchive(file, manifest, public)
	if err != nil {
		return err
	}
	if identity.Commit != commit {
		return fmt.Errorf("archive release commit mismatch")
	}
	return nil
}

func verifyArchive(file io.ReadSeeker, manifest Manifest, public string) (buildinfo.Identity, error) {
	var identity buildinfo.Identity
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(public))
	if err != nil || len(key) != ed25519.PublicKeySize {
		return identity, fmt.Errorf("invalid pinned update public key")
	}
	signature, err := base64.StdEncoding.DecodeString(manifest.Signature)
	if err != nil || len(signature) != ed25519.SignatureSize {
		return identity, fmt.Errorf("invalid update signature encoding or size")
	}
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return identity, err
	}
	if size != manifest.Size || size <= 0 || size > maxArchiveSize {
		return identity, fmt.Errorf("archive size mismatch")
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
		return identity, fmt.Errorf("archive size changed during verification")
	}
	if !ed25519.Verify(key, hash.Sum(nil), signature) {
		return identity, fmt.Errorf("archive is not signed by the pinned update key")
	}
	if _, err = file.Seek(0, io.SeekStart); err != nil {
		return identity, err
	}
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return identity, err
	}
	defer compressed.Close()
	// Bound the entire expanded stream, including tar metadata and padding.
	// tar.Reader stops at the end marker before gzip necessarily checks its
	// trailer, so the remaining stream must also be consumed below.
	expandedStream := &io.LimitedReader{R: compressed, N: maxArchiveSize + 1}
	reader := tar.NewReader(expandedStream)
	metadataFound, executableFound := false, false
	seen := map[string]byte{}
	parentDirectories := map[string]bool{}
	var expanded int64
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return identity, err
		}
		name := strings.TrimSuffix(header.Name, "/")
		// The supported macOS installation volume may be case-insensitive.
		// Compare extraction destinations after folding case, while retaining
		// the original spelling for canonical paths and release identity checks.
		key := archivePathKey(name)
		if name != path.Clean(name) || !strings.HasPrefix(name, "odds.app/") && name != "odds.app" ||
			path.IsAbs(name) || strings.Contains(name, "\\") || seen[key] != 0 || header.Size < 0 ||
			header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeDir {
			return identity, fmt.Errorf("unsafe or duplicate update archive entry: %q", header.Name)
		}
		if header.Typeflag == tar.TypeReg && (name == "odds.app" || parentDirectories[key]) {
			return identity, fmt.Errorf("archive file conflicts with a directory: %q", name)
		}
		for parent := path.Dir(key); parent != "."; parent = path.Dir(parent) {
			if seen[parent] == tar.TypeReg {
				return identity, fmt.Errorf("archive parent is not a directory: %q", parent)
			}
			parentDirectories[parent] = true
		}
		seen[key] = header.Typeflag
		if header.Size > maxArchiveSize-expanded {
			return identity, fmt.Errorf("expanded archive exceeds limit")
		}
		expanded += header.Size
		if name == "odds.app/Contents/Resources/build-info.json" {
			if header.Typeflag != tar.TypeReg || header.Size > 65536 {
				return identity, fmt.Errorf("invalid or oversized build metadata")
			}
			data, err := io.ReadAll(io.LimitReader(reader, 65536))
			if err != nil {
				return identity, err
			}
			if err := json.Unmarshal(data, &identity); err != nil {
				return identity, err
			}
			metadataFound = true
		}
		if name == "odds.app/Contents/MacOS/odds" && header.Typeflag == tar.TypeReg && header.Mode&0111 != 0 {
			executableFound = true
		}
	}
	if _, err := io.Copy(io.Discard, expandedStream); err != nil {
		return identity, fmt.Errorf("invalid compressed archive trailer: %w", err)
	}
	if expandedStream.N == 0 {
		return identity, fmt.Errorf("expanded archive exceeds limit")
	}
	expected := buildinfo.Identity{Schema: 1, Name: "odds", BundleID: "com.knothhe.odds", Version: manifest.Version, Channel: "release", Executor: "ci", Commit: identity.Commit, Target: "darwin-arm64", Source: "source-build"}
	if !metadataFound || !executableFound || !commitPattern.MatchString(identity.Commit) || identity != expected {
		return identity, fmt.Errorf("archive release identity or executable mismatch")
	}
	return identity, nil
}
