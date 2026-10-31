// Validates a MyGo Ed25519 private key from stdin; only the public key is output.
package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

func main() {
	data, err := io.ReadAll(io.LimitReader(os.Stdin, 1024))
	if err != nil {
		panic("cannot read signing key")
	}
	key, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(key) != ed25519.PrivateKeySize || !bytes.Equal(ed25519.NewKeyFromSeed(key[:32]), key) {
		fmt.Fprintln(os.Stderr, "invalid MyGo update signing key")
		os.Exit(1)
	}
	fmt.Println(base64.StdEncoding.EncodeToString(key[32:]))
}
