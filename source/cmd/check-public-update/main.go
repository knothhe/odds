// Command check-public-update verifies the default public feed using MyGo's
// native Check API and odds' authenticated download source. No Install is called.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/egoist/mygo"
	"github.com/knothhe/odds/source/internal/buildinfo"
	"github.com/knothhe/odds/source/internal/updateartifact"
	"github.com/knothhe/odds/source/internal/updateclient"
)

// Retain the exact manifest parsed by the native Check, including its immutable
// source token. Fetching the feed a second time could select a different release.
type source struct {
	*updateclient.Source
	manifest updateartifact.Manifest
}

func (s *source) Manifest(ctx context.Context, pre bool) (io.ReadCloser, error) {
	body, err := s.Source.Manifest(ctx, pre)
	if err != nil {
		return nil, err
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, &s.manifest); err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(data)), nil
}

func run() error {
	current := flag.String("current", "", "Installed client version to check from")
	expected := flag.String("expected", "", "Exact stable candidate version expected from the public feed")
	public := flag.String("public-key", "", "Pinned project update public key supplied by the verification wrapper")
	flag.Parse()
	if *current == "" || *expected == "" || *public == "" {
		return errors.New("current, expected and public-key are required")
	}
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		return errors.New("requires the supported macOS arm64 target")
	}
	mygo.App.SetVersion(*current)
	s := &source{Source: updateclient.New("knothhe/odds", *public, *current, nil)}
	mygo.Updater.SetSource(s)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	candidate, err := mygo.Updater.Check(ctx)
	if err != nil {
		return err
	}
	if candidate == nil || candidate.Version != *expected {
		return fmt.Errorf("public feed did not offer the expected upgrade to %s", *expected)
	}
	file, err := os.CreateTemp("", "odds-public-download-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	body, err := s.Open(ctx, s.manifest.URL)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, body)
	if err := errors.Join(copyErr, body.Close()); err != nil {
		return err
	}
	// Exercise the same authenticated DMG extraction boundary as the updater,
	// entirely inside a private directory. No install helper or relaunch runs.
	staging, err := os.MkdirTemp("", "odds-public-stage-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	destination := filepath.Join(staging, "app")
	if err = s.Extract(ctx, s.manifest.URL, file.Name(), destination); err != nil {
		return err
	}
	identity, err := buildinfo.Read(filepath.Join(destination, "odds.app"))
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"current": *current, "candidate": candidate.Version, "identity": identity,
		"feed":     "https://github.com/knothhe/odds/releases/latest/download/update-darwin-arm64.json",
		"verified": "native-check-authenticated-dmg-and-staging", "installed": false,
	})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
