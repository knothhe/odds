package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigMissingGivesDefaults(t *testing.T) {
	c, err := loadConfig(filepath.Join(t.TempDir(), "config.json"))
	if err != nil || c.Appearance != "system" || c.Shortcuts == nil {
		t.Fatalf("config %+v, err %v", c, err)
	}
}

// An invalid file is moved aside before anything is saved over it.
func TestInvalidConfigIsBackedUpNotOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	broken := `{"appearance": "dark", "shortcuts": {` // truncated
	if err := os.WriteFile(path, []byte(broken), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &App{path: path}
	var err error
	a.cfg, err = loadConfig(path)
	var corrupt *corruptError
	if !errors.As(err, &corrupt) {
		t.Fatalf("err = %v, want corruptError", err)
	}
	a.recoverConfig(err)
	if a.readOnly || !a.backedUp || !strings.Contains(a.errMsg, ".broken-") {
		t.Fatalf("readOnly %v, backedUp %v, msg %q", a.readOnly, a.backedUp, a.errMsg)
	}
	a.save()
	matches, _ := filepath.Glob(path + ".broken-*")
	if len(matches) != 1 {
		t.Fatalf("backups %v", matches)
	}
	if data, _ := os.ReadFile(matches[0]); string(data) != broken {
		t.Fatalf("backup holds %q", data)
	}
	if _, err := loadConfig(path); err != nil {
		t.Fatalf("saved defaults do not load: %v", err)
	}
}

// A file that cannot be read is never saved over.
func TestUnreadableConfigIsNotSaved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.Mkdir(path, 0o700); err != nil { // reading a directory fails
		t.Fatal(err)
	}
	a := &App{path: path, cfg: defaultConfig(), readOnly: true}
	a.save()
	if fi, err := os.Stat(path); err != nil || !fi.IsDir() {
		t.Fatalf("config path was replaced: %v", err)
	}
}

func TestBackupConfigName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	os.WriteFile(path, []byte("x"), 0o600)
	got, err := backupConfig(path, time.Date(2026, 10, 31, 5, 4, 3, 0, time.UTC))
	if err != nil || got != path+".broken-20261031-050403" {
		t.Fatalf("backup %q, err %v", got, err)
	}
}
