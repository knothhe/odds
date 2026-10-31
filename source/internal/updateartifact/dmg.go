package updateartifact

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/knothhe/odds/source/internal/buildinfo"

	"github.com/knothhe/odds/source/internal/uicopy"
)

// VerifyAndExtract authenticates again at the extraction boundary. Only odds.app
// is copied: the DMG's Applications link and other presentation files stay behind.
func VerifyAndExtract(ctx context.Context, filename, destination string, manifest Manifest, current, public string) error {
	candidate, err := parseVersion(manifest.Version)
	if err != nil {
		return err
	}
	installed, err := parseVersion(current)
	if err != nil {
		return err
	}
	if candidate.compare(installed) <= 0 {
		return fmt.Errorf("%s", uicopy.UpdateNotNewer())
	}
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	if err = VerifySignature(file, manifest, public); err != nil {
		return err
	}
	_, err = inspectDMGContext(ctx, file, manifest, public, destination)
	return err
}

func inspectDMG(file io.ReadSeeker, manifest Manifest, public, destination string) (buildinfo.Identity, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	return inspectDMGContext(ctx, file, manifest, public, destination)
}

func inspectDMGContext(ctx context.Context, file io.ReadSeeker, manifest Manifest, public, destination string) (identity buildinfo.Identity, result error) {
	if err := ctx.Err(); err != nil {
		return identity, err
	}
	if runtime.GOOS != "darwin" {
		return identity, fmt.Errorf("%s", uicopy.UpdateMacOnly())
	}
	work, err := os.MkdirTemp("", "odds-dmg-verify-*")
	if err != nil {
		return identity, err
	}
	mounted := false
	defer func() {
		if !mounted {
			os.RemoveAll(work)
		}
	}()
	// Snapshot authenticated bytes, rather than mounting a caller-controlled path.
	image := filepath.Join(work, "update.dmg")
	copy, err := os.OpenFile(image, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return identity, err
	}
	_, err = file.Seek(0, io.SeekStart)
	if err == nil {
		_, err = io.Copy(copy, io.LimitReader(file, maxArchiveSize+1))
	}
	err = errors.Join(err, copy.Close())
	if err != nil {
		return identity, err
	}
	snapshot, err := os.Open(image)
	if err != nil {
		return identity, err
	}
	err = VerifySignature(snapshot, manifest, public)
	err = errors.Join(err, snapshot.Close())
	if err != nil {
		return identity, err
	}
	mount := filepath.Join(work, "mount")
	if err = os.Mkdir(mount, 0700); err != nil {
		return identity, err
	}
	beforeMount, err := os.Stat(mount)
	if err != nil {
		return identity, err
	}
	defer func() {
		if !mounted {
			return
		}
		// Cancellation must not skip unmounting; never remove a mounted directory.
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if output, err := exec.CommandContext(cleanup, "/usr/bin/hdiutil", "detach", mount).CombinedOutput(); err != nil {
			result = errors.Join(result, fmt.Errorf(uicopy.UpdateUnmountFailure(), mount, err, output))
		} else {
			mounted = false
		}
	}()
	output, attachErr := exec.CommandContext(ctx, "/usr/bin/hdiutil", "attach", "-readonly", "-nobrowse", "-noautoopen", "-owners", "off", "-mountpoint", mount, image).CombinedOutput()
	// An interrupted attach may still have mounted the volume. Detect the root's
	// changed identity so the same cleanup applies on this failure path.
	afterMount, statErr := os.Stat(mount)
	mounted = statErr == nil && !os.SameFile(beforeMount, afterMount)
	if attachErr != nil {
		return identity, fmt.Errorf(uicopy.UpdateMountFailure(), attachErr, output)
	}
	mounted = true
	bundle := filepath.Join(mount, "odds.app")
	identity, err = verifyBundle(bundle, manifest.Version)
	if err != nil {
		return identity, err
	}
	if destination != "" {
		if err = ctx.Err(); err != nil {
			return identity, err
		}
		if err = os.Mkdir(destination, 0700); err != nil {
			return identity, err
		}
		staged := filepath.Join(destination, "odds.app")
		if output, err := exec.CommandContext(ctx, "/usr/bin/ditto", bundle, staged).CombinedOutput(); err != nil {
			return identity, fmt.Errorf(uicopy.UpdateStageFailure(), err, output)
		}
		// The copied bundle must satisfy the same identity and path contract.
		if _, err = verifyBundle(staged, manifest.Version); err != nil {
			return identity, err
		}
	}
	return identity, nil
}

func pathKey(name string) string {
	return strings.Map(func(char rune) rune {
		key := char
		for next := unicode.SimpleFold(char); next != char; next = unicode.SimpleFold(next) {
			if next < key {
				key = next
			}
		}
		return key
	}, name)
}

func verifyBundle(bundle, version string) (buildinfo.Identity, error) {
	var identity buildinfo.Identity
	seen := map[string]bool{}
	var total int64
	err := filepath.WalkDir(bundle, func(filename string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		name, err := filepath.Rel(bundle, filename)
		if err != nil {
			return err
		}
		key := pathKey(name)
		if seen[key] || strings.Contains(name, "\\") || !info.IsDir() && !info.Mode().IsRegular() {
			return fmt.Errorf(uicopy.UpdateUnsafeEntry(), name)
		}
		seen[key] = true
		if info.Size() > maxArchiveSize-total {
			return fmt.Errorf("%s", uicopy.UpdateExpandedLimit())
		}
		total += info.Size()
		return nil
	})
	if err != nil {
		return identity, err
	}
	metadata := filepath.Join(bundle, "Contents", "Resources", "build-info.json")
	info, err := os.Lstat(metadata)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 65536 {
		return identity, fmt.Errorf("%s", uicopy.UpdateMetadataInvalid())
	}
	data, err := os.ReadFile(metadata)
	if err != nil {
		return identity, err
	}
	if err = json.Unmarshal(data, &identity); err != nil {
		return identity, err
	}
	executable, err := os.Lstat(filepath.Join(bundle, "Contents", "MacOS", "odds"))
	expected := buildinfo.Identity{Schema: 1, Name: "odds", BundleID: "com.knothhe.odds", Version: version, Channel: "release", Executor: "ci", Commit: identity.Commit, Target: "darwin-arm64", Source: "source-build"}
	if err != nil || !executable.Mode().IsRegular() || executable.Mode().Perm()&0111 == 0 || !commitPattern.MatchString(identity.Commit) || identity != expected {
		return identity, fmt.Errorf("%s", uicopy.UpdateIdentityMismatch())
	}
	return identity, nil
}
