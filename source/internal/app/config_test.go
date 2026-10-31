package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLegacyAndAdditiveSettingsSurviveSaving(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	original := `{"appearance":"dark","welcomed":true,"shortcuts":{"action":""},"futureOption":{"keep":true},"tools":{"mouse-center":{"highlight":false,"color":"red","newOption":42}}}`
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := loadConfig(path)
	if err != nil || c.Schema != configSchema || c.Appearance != "dark" || c.Shortcuts["action"] != "" {
		t.Fatalf("legacy load: %+v %v", c, err)
	}
	// Migration must not write before a real successful save.
	if data, _ := os.ReadFile(path); string(data) != original {
		t.Fatal("load rewrote legacy settings")
	}
	snapshot := c.snapshot()
	c.extra["futureOption"][0] = '['
	snapshot.Tools["mouse-center"] = mergeToolSettings(snapshot.Tools["mouse-center"], json.RawMessage(`{"highlight":true,"color":"green"}`))
	if err := saveConfig(path, snapshot); err != nil {
		t.Fatal(err)
	}
	again, err := loadConfig(path)
	var retained map[string]bool
	json.Unmarshal(again.extra["futureOption"], &retained)
	if err != nil || !retained["keep"] {
		t.Fatalf("lost unknown fields: %+v %v", again, err)
	}
	var fields map[string]any
	json.Unmarshal(again.Tools["mouse-center"], &fields)
	if fields["newOption"] != float64(42) || fields["highlight"] != true {
		t.Fatalf("lost tool settings: %v", fields)
	}
	if err := saveConfig(path, again); err != nil {
		t.Fatal(err)
	}
}

func TestFutureSettingsRemainUntouchedAndReadOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	// The future format may also change the types of fields we know.
	original := `{"schema":2,"appearance":{"mode":"dark"},"futureSecret":"keep"}`
	os.WriteFile(path, []byte(original), 0600)
	c, err := loadConfig(path)
	var future *futureConfigError
	if !errors.As(err, &future) || future.schema != 2 {
		t.Fatalf("future schema was treated as corruption: %v", err)
	}
	a := &App{path: path, cfg: c, readOnly: true}
	a.cfg.Appearance = "light"
	a.save()
	if data, _ := os.ReadFile(path); string(data) != original {
		t.Fatal("future settings overwritten")
	}
	backups, _ := filepath.Glob(path + ".broken-*")
	if len(backups) != 0 {
		t.Fatal("future settings backed up as corrupt")
	}
	c.Schema = 2
	if err := saveConfig(path, c); err == nil {
		t.Fatal("writer accepted a future schema")
	}
}

func TestInvalidConfigurationShapeIsCorrupt(t *testing.T) {
	for _, data := range []string{`null`, `[]`, `{"schema":-1}`, `{"schema":"2"}`, `{"schema":null}`} {
		t.Run(data, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			os.WriteFile(path, []byte(data), 0600)
			_, err := loadConfig(path)
			var corrupt *corruptError
			if !errors.As(err, &corrupt) {
				t.Fatalf("invalid shape accepted: %v", err)
			}
		})
	}
}

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
