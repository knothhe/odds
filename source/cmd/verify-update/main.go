package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/knothhe/odds/source/internal/updateartifact"
	"os"
)

func main() {
	dir := flag.String("dir", "dist/release/darwin-arm64", "release artifacts")
	version := flag.String("version", "", "version")
	commit := flag.String("commit", "", "source commit")
	repo := flag.String("repo", "knothhe/odds", "repository")
	signatureOnly := flag.Bool("signature-only", false, "verify bytes only after the macOS build job inspected the bundle")
	flag.Parse()
	var updates struct {
		PublicKey string `json:"publicKey"`
	}
	data, err := os.ReadFile("config/updates.json")
	if err == nil {
		err = json.Unmarshal(data, &updates)
	}
	if err == nil {
		err = updateartifact.VerifyRelease(*dir, *version, *commit, *repo, updates.PublicKey, !*signatureOnly)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Verified DMG update contract.")
}
