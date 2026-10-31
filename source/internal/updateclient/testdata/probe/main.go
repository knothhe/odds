// The integration test builds this probe as a production MyGo binary inside a
// temporary .app. It never starts the UI or relaunches a downloaded executable.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/egoist/mygo"
	"github.com/knothhe/odds/source/internal/updateartifact"
	"github.com/knothhe/odds/source/internal/updateclient"
)

type probeTransport struct {
	feed, archive []byte
	manifest      updateartifact.Manifest
	requests      []string
}

func (transport *probeTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	transport.requests = append(transport.requests, request.URL.String())
	var data []byte
	switch request.URL.String() {
	case "https://github.com/knothhe/odds/releases/latest/download/update-darwin-arm64.json":
		data = transport.feed
	case transport.manifest.URL:
		data = transport.archive
	default:
		return nil, fmt.Errorf("unexpected offline request: %s", request.URL)
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header), Request: request}, nil
}

func run() error {
	if len(os.Args) != 4 {
		return errors.New("usage: probe manifest archive public-key")
	}
	feed, err := os.ReadFile(os.Args[1])
	if err != nil {
		return err
	}
	archive, err := os.ReadFile(os.Args[2])
	if err != nil {
		return err
	}
	transport := &probeTransport{feed: feed, archive: archive}
	if err := json.Unmarshal(feed, &transport.manifest); err != nil {
		return err
	}
	mygo.App.SetVersion("2.0.0")
	mygo.Updater.SetSource(updateclient.New("knothhe/odds", os.Args[3], "2.0.0", &http.Client{Transport: transport}))
	if !mygo.Updater.Enabled() {
		return errors.New("production probe updater is disabled")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	candidate, err := mygo.Updater.Check(ctx)
	if err != nil {
		return err
	}
	if candidate == nil || candidate.Version != transport.manifest.Version || candidate.Notes != transport.manifest.Notes || candidate.Date.IsZero() {
		return fmt.Errorf("native Check lost candidate or release notes: %+v", candidate)
	}
	var downloaded, total int64
	err = candidate.Install(ctx, func(done, size int64) { downloaded, total = done, size })
	message := ""
	if err != nil {
		message = err.Error()
	}
	return json.NewEncoder(os.Stdout).Encode(struct {
		InstallError string   `json:"installError"`
		Downloaded   int64    `json:"downloaded"`
		Total        int64    `json:"total"`
		Requests     []string `json:"requests"`
	}{message, downloaded, total, transport.requests})
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
