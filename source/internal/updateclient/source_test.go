package updateclient

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/knothhe/odds/source/internal/buildinfo"
	"github.com/knothhe/odds/source/internal/updateartifact"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return fn(r) }

func clientFixture(t *testing.T, version string) (updateartifact.Manifest, []byte, string) {
	t.Helper()
	identity := buildinfo.Identity{Schema: 1, Name: "odds", BundleID: "com.knothhe.odds", Version: version,
		Channel: "release", Executor: "ci", Commit: strings.Repeat("a", 40), Target: "darwin-arm64", Source: "source-build"}
	metadata, err := json.Marshal(identity)
	if err != nil {
		t.Fatal(err)
	}
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tw := tar.NewWriter(gz)
	for _, entry := range []struct {
		name string
		data []byte
		mode int64
	}{{"odds.app/Contents/Resources/build-info.json", metadata, 0644}, {"odds.app/Contents/MacOS/odds", []byte("fixture"), 0755}} {
		if err := tw.WriteHeader(&tar.Header{Name: entry.name, Mode: entry.mode, Size: int64(len(entry.data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	private := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	hash := sha256.Sum256(archive.Bytes())
	manifest := updateartifact.Manifest{Version: version, URL: "https://github.com/knothhe/odds/releases/download/v" + version + "/odds-" + version + "-darwin-arm64.tar.gz",
		Size: int64(archive.Len()), Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(private, hash[:])), Notes: "Release notes", Date: "2026-10-31T00:00:00Z"}
	return manifest, archive.Bytes(), base64.StdEncoding.EncodeToString(private.Public().(ed25519.PublicKey))
}

func serveFixture(t *testing.T, manifest *updateartifact.Manifest, archives map[string][]byte) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("User-Agent") != "mygo-updater" {
			t.Error("missing updater user agent")
		}
		var data []byte
		if strings.HasSuffix(r.URL.Path, "/update-darwin-arm64.json") {
			data, _ = json.Marshal(manifest)
		} else {
			var ok bool
			data, ok = archives[r.URL.String()]
			if !ok {
				return nil, errors.New("unexpected archive request: " + r.URL.String())
			}
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header), Request: r}, nil
	})}
}

func candidateToken(t *testing.T, source *Source) updateartifact.Manifest {
	t.Helper()
	body, err := source.Manifest(context.Background(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	var offered updateartifact.Manifest
	if err := json.NewDecoder(body).Decode(&offered); err != nil {
		t.Fatal(err)
	}
	return offered
}

func TestStreamRejectsSignedOldArchiveWithForgedManifestVersion(t *testing.T) {
	manifest, archive, public := clientFixture(t, "1.0.0")
	manifest.Version = "999.0.0"
	manifest.URL = "https://github.com/knothhe/odds/releases/download/v999.0.0/odds-999.0.0-darwin-arm64.tar.gz"
	source := New("knothhe/odds", public, "2.0.0", serveFixture(t, &manifest, map[string][]byte{manifest.URL: archive}))
	source.tempDir = t.TempDir()
	offered := candidateToken(t, source)
	body, err := source.Open(context.Background(), offered.URL)
	if err != nil {
		t.Fatal(err)
	}
	// Match MyGo's download boundary: a successful copy would allow staging.
	if _, err := io.Copy(io.Discard, io.LimitReader(body, manifest.Size+1)); err == nil {
		t.Fatal("signed old archive reached successful EOF under forged version")
	}
	if err := body.Close(); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(source.tempDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("verification files retained: %v, %v", entries, err)
	}
}

func TestCandidatesRemainBoundAcrossChecks(t *testing.T) {
	first, firstArchive, public := clientFixture(t, "3.0.0")
	second, secondArchive, _ := clientFixture(t, "4.0.0")
	feed := first
	source := New("knothhe/odds", public, "2.0.0", serveFixture(t, &feed,
		map[string][]byte{first.URL: firstArchive, second.URL: secondArchive}))
	source.tempDir = t.TempDir()
	firstOffer := candidateToken(t, source)
	feed = second
	secondOffer := candidateToken(t, source)
	if firstOffer.URL == secondOffer.URL || firstOffer.Notes != first.Notes || firstOffer.Date != first.Date {
		t.Fatal("candidate identity or release details lost")
	}
	for _, offer := range []updateartifact.Manifest{firstOffer, secondOffer} {
		body, err := source.Open(context.Background(), offer.URL)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(io.Discard, body); err != nil {
			t.Fatal(err)
		}
		if err := body.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := source.Open(context.Background(), first.URL); err == nil {
		t.Fatal("accepted direct URL without a bound candidate")
	}
}

func TestStreamErrorsAndCancellationCleanUp(t *testing.T) {
	for _, scenario := range []string{"cancel", "truncated", "oversized", "tampered", "partial-close"} {
		t.Run(scenario, func(t *testing.T) {
			manifest, archive, public := clientFixture(t, "3.0.0")
			switch scenario {
			case "truncated":
				archive = archive[:len(archive)-1]
			case "oversized":
				archive = append(archive, 1)
			case "tampered":
				archive[len(archive)/2] ^= 1
			}
			source := New("knothhe/odds", public, "2.0.0", serveFixture(t, &manifest, map[string][]byte{manifest.URL: archive}))
			source.tempDir = t.TempDir()
			offer := candidateToken(t, source)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			body, err := source.Open(ctx, offer.URL)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "cancel" {
				cancel()
			}
			if scenario == "partial-close" {
				_, err = body.Read(make([]byte, 1))
			} else {
				_, err = io.Copy(io.Discard, body)
				if err == nil {
					t.Fatal("invalid download was accepted")
				}
			}
			if err := body.Close(); err != nil {
				t.Fatal(err)
			}
			if err := body.Close(); err != nil {
				t.Fatal(err)
			}
			entries, err := os.ReadDir(source.tempDir)
			if err != nil || len(entries) != 0 {
				t.Fatalf("temporary files leaked: %v, %v", entries, err)
			}
		})
	}
}

func TestManifestFailureAndCurrentRelease(t *testing.T) {
	manifest, _, public := clientFixture(t, "2.0.0")
	source := New("knothhe/odds", public, "2.0.0", serveFixture(t, &manifest, nil))
	if candidateToken(t, source).Version != "2.0.0" {
		t.Fatal("healthy current release did not remain valid feed data")
	}
	manifest.URL = "https://example.org/untrusted.tar.gz"
	if _, err := source.Manifest(context.Background(), false); err == nil {
		t.Fatal("accepted untrusted archive URL")
	}
	if _, err := source.Manifest(context.Background(), true); err == nil {
		t.Fatal("silently accepted unsupported prerelease channel")
	}
	for _, scenario := range []string{"oversized", "status"} {
		t.Run(scenario, func(t *testing.T) {
			closed := false
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				status, data := http.StatusOK, strings.Repeat(" ", maxManifestSize+1)
				if scenario == "status" {
					status, data = http.StatusNotFound, "missing"
				}
				return &http.Response{StatusCode: status, Body: &closeObserver{Reader: strings.NewReader(data), closed: &closed}, Header: make(http.Header), Request: r}, nil
			})}
			if _, err := New("knothhe/odds", public, "2.0.0", client).Manifest(context.Background(), false); err == nil || !closed {
				t.Fatalf("manifest error/cleanup failed: %v, closed=%v", err, closed)
			}
		})
	}
}

func TestStableSourceRejectsPrereleaseCandidates(t *testing.T) {
	for _, version := range []string{"3.0.0-beta.1", "3.0.0-beta.1+build.2"} {
		t.Run(version, func(t *testing.T) {
			manifest, _, public := clientFixture(t, version)
			source := New("knothhe/odds", public, "2.0.0", serveFixture(t, &manifest, nil))
			body, err := source.Manifest(context.Background(), false)
			if body != nil {
				body.Close()
			}
			if err == nil {
				t.Fatal("stable feed offered a prerelease candidate")
			}
		})
	}
	manifest, _, public := clientFixture(t, "3.0.0+build-with-hyphen")
	source := New("knothhe/odds", public, "2.0.0", serveFixture(t, &manifest, nil))
	if offer := candidateToken(t, source); offer.Version != manifest.Version {
		t.Fatal("build metadata was mistaken for a prerelease")
	}
}

type closeObserver struct {
	io.Reader
	closed *bool
}

func (r *closeObserver) Close() error { *r.closed = true; return nil }

func TestDefaultClientRejectsHTTPSDowngrade(t *testing.T) {
	source := New("knothhe/odds", "", "2.0.0", nil)
	req, _ := http.NewRequest(http.MethodGet, "http://example.org/archive", nil)
	if err := source.client.CheckRedirect(req, nil); err == nil {
		t.Fatal("accepted redirect to HTTP")
	}
}
