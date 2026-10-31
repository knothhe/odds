package buildinfo

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestOnlyMatchingCIReleaseCanUseReleaseFeed(t *testing.T) {
	oldChannel, oldExecutor, oldCommit, oldRepo, oldKey := Channel, Executor, Commit, Repository, PublicKey
	defer func() {
		Channel, Executor, Commit, Repository, PublicKey = oldChannel, oldExecutor, oldCommit, oldRepo, oldKey
	}()
	Channel, Executor, Commit, Repository, PublicKey = "release", "ci", strings.Repeat("a", 40), "knothhe/odds", base64.StdEncoding.EncodeToString(make([]byte, 32))
	record := Identity{1, "odds", "com.knothhe.odds", "0.1.0", "release", "ci", Commit, "darwin-arm64", "source-build"}
	if !ReleaseEligible(record, "0.1.0", false) {
		t.Fatal("valid CI release was rejected")
	}
	if ReleaseEligible(record, "0.1.0", true) {
		t.Fatal("preview joined the release feed")
	}
	for _, change := range []func(){func() { Channel = "dev" }, func() { Executor = "local" }, func() { Repository = "other/repo" }, func() { Commit = "unknown" }, func() { PublicKey = "" }} {
		Channel, Executor, Commit, Repository, PublicKey = "release", "ci", strings.Repeat("a", 40), "knothhe/odds", base64.StdEncoding.EncodeToString(make([]byte, 32))
		change()
		if ReleaseEligible(record, "0.1.0", false) {
			t.Fatal("ineligible build joined the release feed")
		}
	}
	Channel, Executor, Commit, Repository, PublicKey = "release", "ci", record.Commit, "knothhe/odds", base64.StdEncoding.EncodeToString(make([]byte, 32))
	record.BundleID = "com.knothhe.odds.dev"
	if ReleaseEligible(record, "0.1.0", false) {
		t.Fatal("mismatched installed identity joined the release feed")
	}
}
