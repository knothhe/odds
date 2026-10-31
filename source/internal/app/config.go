package app

import (
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"time"
)

// Config is everything odds stores, in config.json in the user config
// directory.
type Config struct {
	Appearance string `json:"appearance"` // system, light or dark
	Welcomed   bool   `json:"welcomed"`
	// Shortcuts maps an action ID to its accelerator. An empty value turns
	// the shortcut off; an absent one uses the action's default.
	Shortcuts map[string]string `json:"shortcuts"`
	// Tools holds each tool's settings by tool ID.
	Tools map[string]json.RawMessage `json:"tools"`
}

func defaultConfig() Config {
	return Config{Appearance: "system", Shortcuts: map[string]string{}, Tools: map[string]json.RawMessage{}}
}

// snapshot detaches maps and raw JSON from the UI-owned configuration.
func (c Config) snapshot() Config {
	c.Shortcuts = maps.Clone(c.Shortcuts)
	tools := make(map[string]json.RawMessage, len(c.Tools))
	for id, raw := range c.Tools {
		tools[id] = append(json.RawMessage(nil), raw...)
	}
	c.Tools = tools
	return c
}

func (c *Config) normalize() {
	if c.Appearance != "light" && c.Appearance != "dark" {
		c.Appearance = "system"
	}
	if c.Shortcuts == nil {
		c.Shortcuts = map[string]string{}
	}
	if c.Tools == nil {
		c.Tools = map[string]json.RawMessage{}
	}
}

// loadConfig reads the configuration; a missing file gives the defaults.
// A file that cannot be read, or cannot be parsed, also gives the defaults
// with an error: the caller must then not save over it unless backupConfig
// has moved it aside.
func loadConfig(path string) (Config, error) {
	c := defaultConfig()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return defaultConfig(), &corruptError{err}
	}
	c.normalize()
	return c, nil
}

// corruptError reports a configuration file that is not valid.
type corruptError struct{ err error }

func (e *corruptError) Error() string { return e.err.Error() }
func (e *corruptError) Unwrap() error { return e.err }

// backupConfig moves the file at path aside, next to it, and returns where
// it went, so that saving the defaults does not destroy what it held.
func backupConfig(path string, now time.Time) (string, error) {
	backup := path + ".broken-" + now.Format("20060102-150405")
	if err := os.Rename(path, backup); err != nil {
		return "", err
	}
	return backup, nil
}

// saveConfig replaces the file atomically, so an interrupted save keeps the
// previous settings.
func saveConfig(path string, c Config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
