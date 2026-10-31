// Package buildinfo holds the channel and provenance injected by the build entry.
package buildinfo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
)

var (
	Channel    = "dev"
	Executor   = "local"
	Commit     = "unknown"
	Repository = ""
	PublicKey  = ""
)

type Identity struct {
	Schema   int    `json:"schema"`
	Name     string `json:"name"`
	BundleID string `json:"bundleID"`
	Version  string `json:"version"`
	Channel  string `json:"channel"`
	Executor string `json:"executor"`
	Commit   string `json:"commit"`
	Target   string `json:"target"`
	Source   string `json:"source"`
}

func Name() string {
	if Channel == "release" {
		return "odds"
	}
	return "odds Dev"
}

func BundleID() string {
	if Channel == "release" {
		return "com.knothhe.odds"
	}
	return "com.knothhe.odds.dev"
}

// ReleaseEligible binds the installed metadata to the compiled release identity.
// Dev, raw toolkit builds and temporary previews never join the release feed.
func ReleaseEligible(record Identity, version string, preview bool) bool {
	key := regexp.MustCompile(`^[A-Za-z0-9+/]{43}=$`)
	commit := regexp.MustCompile(`^[a-f0-9]{40}$`)
	return !preview && Channel == "release" && Executor == "ci" && commit.MatchString(Commit) &&
		Repository == "knothhe/odds" && key.MatchString(PublicKey) &&
		record == (Identity{1, Name(), BundleID(), version, Channel, Executor, Commit, "darwin-arm64", "source-build"})
}

func Read(bundle string) (Identity, error) {
	var value Identity
	data, err := os.ReadFile(filepath.Join(bundle, "Contents", "Resources", "build-info.json"))
	if err != nil {
		return value, err
	}
	err = json.Unmarshal(data, &value)
	return value, err
}
