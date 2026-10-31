package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"time"

	"github.com/knothhe/odds/source/internal/uicopy"
)

const configSchema = 1

// Config is everything odds stores, in config.json in the user config
// directory.
type Config struct {
	Schema     int    `json:"schema"`
	Appearance string `json:"appearance"` // system, light or dark
	Welcomed   bool   `json:"welcomed"`
	// Shortcuts maps an action ID to its accelerator. An empty value turns
	// the shortcut off; an absent one uses the action's default.
	Shortcuts map[string]string `json:"shortcuts"`
	// Tools holds each tool's settings by tool ID.
	Tools map[string]json.RawMessage `json:"tools"`
	extra map[string]json.RawMessage
}

func defaultConfig() Config {
	return Config{Schema: configSchema, Appearance: "system", Shortcuts: map[string]string{}, Tools: map[string]json.RawMessage{}}
}

// Keep additive fields when an older client edits a compatible configuration.
// A newer schema is rejected before interpreting any of its fields.
func (c *Config) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if fields == nil {
		return fmt.Errorf(uicopy.ConfigObjectError)
	}
	var schema int
	if raw, ok := fields["schema"]; ok {
		if string(raw) == "null" {
			return fmt.Errorf(uicopy.ConfigObjectError)
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			return err
		}
	}
	if schema > configSchema {
		return &futureConfigError{schema}
	}
	if schema < 0 {
		return fmt.Errorf(uicopy.ConfigSchemaError, schema)
	}
	type plain Config
	value := plain(defaultConfig())
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	for _, key := range []string{"schema", "appearance", "welcomed", "shortcuts", "tools"} {
		delete(fields, key)
	}
	*c = Config(value)
	c.Schema, c.extra = configSchema, fields // unversioned files migrate in memory
	return nil
}

func (c Config) MarshalJSON() ([]byte, error) {
	if c.Schema > configSchema || c.Schema < 0 {
		return nil, fmt.Errorf(uicopy.ConfigSchemaError, c.Schema)
	}
	type plain Config
	c.Schema = configSchema
	data, err := json.Marshal(plain(c))
	if err != nil {
		return nil, err
	}
	fields := cloneRaw(c.extra)
	var known map[string]json.RawMessage
	if err := json.Unmarshal(data, &known); err != nil {
		return nil, err
	}
	maps.Copy(fields, known)
	return json.Marshal(fields)
}

type futureConfigError struct{ schema int }

func (e *futureConfigError) Error() string { return fmt.Sprintf(uicopy.ConfigSchemaError, e.schema) }

func cloneRaw(source map[string]json.RawMessage) map[string]json.RawMessage {
	result := make(map[string]json.RawMessage, len(source))
	for key, raw := range source {
		result[key] = append(json.RawMessage(nil), raw...)
	}
	return result
}

// Tool settings are JSON objects. Updating known fields must not erase additive
// fields written by another compatible version of the same tool.
func mergeToolSettings(previous, current json.RawMessage) json.RawMessage {
	var before, after map[string]json.RawMessage
	if json.Unmarshal(previous, &before) != nil || before == nil || json.Unmarshal(current, &after) != nil || after == nil {
		return current
	}
	maps.Copy(before, after)
	merged, err := json.Marshal(before)
	if err != nil {
		return current
	}
	return merged
}

// snapshot detaches maps and raw JSON from the UI-owned configuration.
func (c Config) snapshot() Config {
	c.Shortcuts = maps.Clone(c.Shortcuts)
	c.Tools = cloneRaw(c.Tools)
	c.extra = cloneRaw(c.extra)
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
		var future *futureConfigError
		if errors.As(err, &future) {
			return defaultConfig(), future
		}
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
