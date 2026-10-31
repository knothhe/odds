//go:build darwin

package updateclient

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/knothhe/odds/source/internal/buildinfo"
	"github.com/knothhe/odds/source/internal/updateartifact"
)

// Exercise the public MyGo Check/Install API in its real macOS bundle layout.
// The built probe and shell swap helper operate entirely under t.TempDir; the
// network is an offline RoundTripper and the replacement executable never runs.
func TestNativeUpdaterAuthenticatesReleaseBeforeStaging(t *testing.T) {
	if runtime.GOARCH != "arm64" {
		t.Skip("the supported release target is darwin-arm64")
	}
	root := t.TempDir()
	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize)) // Public test fixture key.
	public := base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
	probe := filepath.Join(root, "probe")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	flags := "-X github.com/egoist/mygo.production=1 -X github.com/egoist/mygo.packageUpdateFeed=https://github.com/knothhe/odds/releases/latest/download/update-darwin-arm64.json -X github.com/egoist/mygo.packageUpdateKey=" + public
	build := exec.CommandContext(ctx, "python3", "../../../scripts/go-patched.py", "build", "-ldflags", flags, "-o", probe, "./testdata/probe")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build native updater probe: %v\n%s", err, output)
	}
	probeBytes, err := os.ReadFile(probe)
	if err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name, advertised, bundled, installed string
		rejected, failedSwap                 bool
	}{
		{"signed-old-release-under-forged-manifest", "999.0.0", "1.0.0", "2.0.0", true, false},
		{"authentic-new-release", "3.0.0", "3.0.0", "3.0.0", false, false},
		{"update-and-rollback-moves-fail", "3.0.0", "3.0.0", "2.0.0", false, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			directory := filepath.Join(root, scenario.name)
			bundle := filepath.Join(directory, "odds.app")
			integrationWriteBundle(t, bundle, "2.0.0", probeBytes)
			archive := integrationArchive(t, filepath.Join(directory, "candidate"), scenario.bundled)
			digest := sha256.Sum256(archive)
			manifest := updateartifact.Manifest{
				Version: scenario.advertised,
				URL:     "https://github.com/knothhe/odds/releases/download/v" + scenario.advertised + "/odds-" + scenario.advertised + "-arm64.dmg",
				Size:    int64(len(archive)), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(private, digest[:])),
				Notes: "Native integration release notes", Date: "2026-10-31T00:00:00Z",
			}
			feed, err := json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			feedFile, archiveFile := filepath.Join(directory, "manifest.json"), filepath.Join(directory, "update.dmg")
			integrationWriteFile(t, feedFile, feed, 0600)
			integrationWriteFile(t, archiveFile, archive, 0600)
			childContext, childCancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer childCancel()
			command := exec.CommandContext(childContext, filepath.Join(bundle, "Contents", "MacOS", "odds"), feedFile, archiveFile, public)
			command.Env = append(os.Environ(), "TMPDIR="+directory)
			if scenario.failedSwap {
				bin := filepath.Join(directory, "bin")
				integrationWriteFile(t, filepath.Join(bin, "mv"), []byte(integrationFailingMove), 0755)
				command.Env = append(command.Env, "PATH="+bin+":"+os.Getenv("PATH"), "ODDS_TEST_MOVE_DIR="+directory)
			}
			output, err := command.CombinedOutput()
			if err != nil {
				t.Fatalf("run native updater probe: %v\n%s", err, output)
			}
			// A successful Install starts MyGo's helper, which waits for the
			// probe to exit before swapping. Wait even on a regression so the
			// helper finishes inside its private fixture before test cleanup.
			if scenario.failedSwap {
				integrationWaitForFailedHelper(t, directory)
			} else {
				integrationWaitForHelper(t, directory)
			}
			verificationFiles, err := filepath.Glob(filepath.Join(directory, "odds-update-verify-*"))
			if err != nil || len(verificationFiles) != 0 {
				t.Fatalf("native download left verification files: %v (%v)", verificationFiles, err)
			}
			var result struct {
				InstallError string   `json:"installError"`
				Downloaded   int64    `json:"downloaded"`
				Total        int64    `json:"total"`
				Requests     []string `json:"requests"`
			}
			if err := json.Unmarshal(output, &result); err != nil {
				t.Fatalf("decode native updater result: %v\n%s", err, output)
			}
			if scenario.rejected {
				if !strings.Contains(result.InstallError, "archive release identity or executable mismatch") {
					t.Fatalf("native Install accepted replay or failed for another reason: %+v", result)
				}
			} else if result.InstallError != "" {
				t.Fatalf("native Install rejected authentic release: %s", result.InstallError)
			}
			if result.Downloaded != manifest.Size || result.Total != manifest.Size || len(result.Requests) != 2 {
				t.Fatalf("native download progress or offline requests changed: %+v", result)
			}
			installedBundle := bundle
			if scenario.failedSwap {
				work, err := filepath.Glob(filepath.Join(directory, ".odds.app.update-*"))
				if err != nil || len(work) != 1 {
					t.Fatalf("double move failure discarded recovery directory: %v (%v)", work, err)
				}
				installedBundle = filepath.Join(work[0], "old")
				note, err := os.ReadFile(filepath.Join(work[0], "RECOVERY.txt"))
				if err != nil || !bytes.Contains(note, []byte(bundle)) || !bytes.Contains(note, []byte(installedBundle)) {
					t.Fatalf("double move failure lost persistent recovery instructions: %q (%v)", note, err)
				}
				candidate, err := buildinfo.Read(filepath.Join(work[0], "app", "odds.app"))
				if err != nil || candidate.Version != "3.0.0" {
					t.Fatalf("double move failure discarded staged candidate: %+v (%v)", candidate, err)
				}
				if _, err := os.Lstat(bundle); !os.IsNotExist(err) {
					t.Fatalf("unexpected target after injected failed moves: %v", err)
				}
			}
			identity, err := buildinfo.Read(installedBundle)
			if err != nil || identity.Version != scenario.installed {
				t.Fatalf("installed version = %q, want %q (read error %v)", identity.Version, scenario.installed, err)
			}
			installedBytes, err := os.ReadFile(filepath.Join(installedBundle, "Contents", "MacOS", "odds"))
			if err != nil {
				t.Fatal(err)
			}
			if (scenario.rejected || scenario.failedSwap) && !bytes.Equal(installedBytes, probeBytes) {
				t.Fatal("rejected archive changed the installed executable")
			}
			if !scenario.rejected && !scenario.failedSwap && !bytes.Equal(installedBytes, []byte(integrationReplacementExecutable)) {
				t.Fatal("native helper did not install the authentic replacement executable")
			}
			if _, err := os.Lstat(filepath.Join(bundle, "Contents", "MacOS", "odds.ran")); !os.IsNotExist(err) {
				t.Fatalf("replacement executable was launched: %v", err)
			}
		})
	}
}

const integrationReplacementExecutable = "#!/bin/sh\nprintf unexpected > \"$0.ran\"\nexit 1\n"

// Fail the new-bundle move and the old-bundle restore after the initial backup
// move succeeds. The probe's PATH isolates this shim from all other processes.
const integrationFailingMove = `#!/bin/sh
count=0
if [ -f "$ODDS_TEST_MOVE_DIR/moves" ]; then read -r count < "$ODDS_TEST_MOVE_DIR/moves"; fi
count=$((count + 1))
printf '%s\n' "$count" > "$ODDS_TEST_MOVE_DIR/moves"
printf '%s\n' "$PPID" > "$ODDS_TEST_MOVE_DIR/helper-pid"
if [ "$count" -ge 2 ]; then exit 1; fi
exec /bin/mv "$@"
`

func integrationWaitForFailedHelper(t *testing.T, directory string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		count, _ := os.ReadFile(filepath.Join(directory, "moves"))
		pidData, _ := os.ReadFile(filepath.Join(directory, "helper-pid"))
		pid, _ := strconv.Atoi(strings.TrimSpace(string(pidData)))
		if strings.TrimSpace(string(count)) == "3" && pid > 0 && errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("fault-injected helper did not finish: move count %q, pid %d", count, pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func integrationWriteFile(t *testing.T, filename string, data []byte, mode fs.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filename, data, mode); err != nil {
		t.Fatal(err)
	}
}

func integrationWriteBundle(t *testing.T, bundle, version string, executable []byte) {
	t.Helper()
	identity := buildinfo.Identity{Schema: 1, Name: "odds", BundleID: "com.knothhe.odds", Version: version,
		Channel: "release", Executor: "ci", Commit: strings.Repeat("a", 40), Target: "darwin-arm64", Source: "source-build"}
	metadata, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	integrationWriteFile(t, filepath.Join(bundle, "Contents", "Resources", "build-info.json"), metadata, 0644)
	integrationWriteFile(t, filepath.Join(bundle, "Contents", "MacOS", "odds"), executable, 0755)
}

func integrationArchive(t *testing.T, directory, version string) []byte {
	t.Helper()
	bundle := filepath.Join(directory, "odds.app")
	integrationWriteBundle(t, bundle, version, []byte(integrationReplacementExecutable))
	image := filepath.Join(directory, "candidate.dmg")
	command := exec.Command("/usr/bin/hdiutil", "create", "-quiet", "-format", "UDZO", "-fs", "HFS+", "-srcfolder", directory, image)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("create fixture DMG: %v: %s", err, output)
	}
	data, err := os.ReadFile(image)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func integrationWaitForHelper(t *testing.T, directory string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		work, err := filepath.Glob(filepath.Join(directory, ".odds.app.update-*"))
		if err != nil {
			t.Fatal(err)
		}
		if len(work) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("native update helper did not finish its temporary bundle swap: %v", work)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
