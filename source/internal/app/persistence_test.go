package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestSnapshotKeepsPendingSettingsIndependent(t *testing.T) {
	c := defaultConfig()
	c.Shortcuts["action"] = "Ctrl+A"
	c.Tools["tool"] = json.RawMessage(`{"enabled":true}`)
	snapshot := c.snapshot()
	c.Shortcuts["action"] = "Ctrl+B"
	c.Tools["tool"][0] = '['
	if snapshot.Shortcuts["action"] != "Ctrl+A" || string(snapshot.Tools["tool"]) != `{"enabled":true}` {
		t.Fatalf("queued snapshot changed with UI: %+v", snapshot)
	}
}

func TestWriterCoalescesEditsAndContinuesAfterFailure(t *testing.T) {
	started := make(chan string, 3)
	release := make(chan struct{})
	completed := make(chan uint64, 3)
	w := newConfigWriter(func(c Config) error {
		started <- c.Appearance
		if c.Appearance == "first" {
			<-release
			return errors.New("first write failed")
		}
		return nil
	}, func(revision uint64, _ error) { completed <- revision })
	defer w.close()
	w.submit(configWrite{revision: 1, config: Config{Appearance: "first"}})
	if got := receive(t, started); got != "first" {
		t.Fatalf("first write = %q", got)
	}
	w.submit(configWrite{revision: 2, config: Config{Appearance: "superseded"}})
	w.submit(configWrite{revision: 3, config: Config{Appearance: "latest"}})
	close(release)
	if receive(t, completed) != 1 || receive(t, started) != "latest" || receive(t, completed) != 3 {
		t.Fatal("writer did not continue with the latest complete snapshot")
	}
}

type saveResult struct {
	revision uint64
	err      error
}

func TestFailedSaveRetainsEditsAndRetryPersistsThem(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	// A directory at the destination makes atomic replacement fail reliably.
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	a := &App{path: path, cfg: defaultConfig()}
	completed := make(chan saveResult, 2)
	a.writer = newConfigWriter(func(c Config) error { return saveConfig(path, c) }, func(revision uint64, err error) {
		completed <- saveResult{revision, err}
	})
	defer a.writer.close()
	a.cfg.Appearance = "dark"
	a.save()
	result := receive(t, completed)
	a.saved(result.revision, result.err)
	if a.saveErr == "" || a.cfg.Appearance != "dark" || a.saving {
		t.Fatalf("failure lost edits or status: %+v", a)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	a.save()
	if a.saveErr == "" {
		t.Fatal("retry hid the previous error before success")
	}
	result = receive(t, completed)
	a.saved(result.revision, result.err)
	if a.saveErr != "" || a.saving {
		t.Fatalf("retry did not clear failure: %q", a.saveErr)
	}
	loaded, err := loadConfig(path)
	if err != nil || loaded.Appearance != "dark" {
		t.Fatalf("retry did not persist retained edits: %+v, %v", loaded, err)
	}
}

func TestStaleSaveResultsDoNotChangeLatestStatus(t *testing.T) {
	a := &App{saveRevision: 3, saving: true, saveErr: "previous failure"}
	a.saved(1, nil)
	a.saved(2, errors.New("stale error"))
	if !a.saving || a.saveErr != "previous failure" {
		t.Fatal("stale result changed current save status")
	}
	a.saved(3, nil)
	if a.saving || a.saveErr != "" {
		t.Fatal("latest successful write did not clear the failure")
	}
}

func TestQuitFlushesLatestSnapshotWithoutCancellingNativeRelaunch(t *testing.T) {
	a := &App{cfg: defaultConfig()}
	started, release := make(chan struct{}), make(chan struct{})
	completed := make(chan Config, 2)
	a.writer = newConfigWriter(func(c Config) error {
		if c.Appearance == "system" {
			close(started)
			<-release
		}
		completed <- c
		return nil
	}, func(uint64, error) {})
	defer a.writer.close()
	a.save()
	receive(t, started)
	a.cfg.Appearance = "dark"
	a.save()
	close(release)
	event := &mygo.QuitEvent{}
	a.beforeQuit(event)
	if event.DefaultPrevented() || !a.quitting || a.saving {
		t.Fatal("successful flush cancelled the native quit/relaunch request")
	}
	receive(t, completed)
	if receive(t, completed).Appearance != "dark" {
		t.Fatal("latest edit was not persisted before quitting")
	}
}

func TestApplicationVersionUsesPackagedValueOrDevelopmentLabel(t *testing.T) {
	for input, want := range map[string]string{"": "开发构建", " ": "开发构建", "0.1.0": "0.1.0", "1.0.0-rc.1": "1.0.0-rc.1"} {
		if got := applicationVersion(input); got != want {
			t.Errorf("version(%q) = %q, want %q", input, got, want)
		}
	}
}

func receive[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for the settings writer")
		var zero T
		return zero
	}
}
