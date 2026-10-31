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
	flag.Parse()
	var updates struct {
		PublicKey string `json:"publicKey"`
	}
	data, err := os.ReadFile("config/updates.json")
	if err == nil {
		err = json.Unmarshal(data, &updates)
	}
	if err == nil {
		err = updateartifact.Verify(*dir, *version, *commit, *repo, updates.PublicKey)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Verified update signature, archive paths and exact release identity.")
}
