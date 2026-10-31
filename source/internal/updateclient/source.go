// Package updateclient binds MyGo's native updater to authenticated release
// identities before it can extract or stage an archive.
package updateclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/netproxy"
	"github.com/knothhe/odds/source/internal/updateartifact"
)

const maxManifestSize = 1 << 20

type Source struct {
	repo, public, current string
	client                *http.Client
	tempDir               string
	mu                    sync.Mutex
	candidates            map[string]updateartifact.Manifest
	order                 []string
}

// New serves the repository's stable feed. A nil client uses the application's
// proxy transport with HTTPS-only redirects. Non-nil clients support isolated QA.
func New(repo, public, current string, client *http.Client) *Source {
	if client == nil {
		client = &http.Client{
			Transport: netproxy.Transport(), Timeout: 5 * time.Minute,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if req.URL.Scheme != "https" {
					return errors.New("update redirect must use HTTPS")
				}
				if len(via) >= 10 {
					return errors.New("too many update redirects")
				}
				return nil
			},
		}
	}
	return &Source{repo: repo, public: public, current: current, client: client,
		candidates: make(map[string]updateartifact.Manifest)}
}

func (s *Source) get(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	if req.URL.Scheme != "https" {
		return nil, errors.New("update download must use HTTPS")
	}
	req.Header.Set("User-Agent", "mygo-updater")
	response, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		response.Body.Close()
		return nil, fmt.Errorf("update server returned HTTP %d", response.StatusCode)
	}
	return response.Body, nil
}

func (s *Source) Manifest(ctx context.Context, prereleases bool) (io.ReadCloser, error) {
	if prereleases {
		return nil, errors.New("this update source supports stable releases only")
	}
	body, err := s.get(ctx, "https://github.com/"+s.repo+"/releases/latest/download/update-darwin-arm64.json")
	if err != nil {
		return nil, err
	}
	defer body.Close()
	data, err := io.ReadAll(io.LimitReader(body, maxManifestSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxManifestSize {
		return nil, errors.New("update manifest exceeds size limit")
	}
	var manifest updateartifact.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, err
	}
	if err := updateartifact.ValidateManifest(manifest, s.current, s.repo); err != nil {
		return nil, err
	}
	version, _, _ := strings.Cut(manifest.Version, "+")
	if strings.Contains(version, "-") {
		return nil, errors.New("the stable update feed cannot offer a prerelease")
	}
	canonical, err := json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(canonical)
	token := "odds-update:" + hex.EncodeToString(digest[:])
	// An Update keeps its source across later checks. Bind each opaque URL to
	// that immutable candidate, rather than whichever feed was fetched last.
	s.mu.Lock()
	if _, exists := s.candidates[token]; !exists {
		if len(s.order) >= 32 {
			delete(s.candidates, s.order[0])
			s.order = s.order[1:]
		}
		s.candidates[token] = manifest
		s.order = append(s.order, token)
	}
	s.mu.Unlock()
	manifest.URL = token
	canonical, err = json.Marshal(manifest)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(bytes.NewReader(canonical)), nil
}

func (s *Source) Open(ctx context.Context, token string) (io.ReadCloser, error) {
	s.mu.Lock()
	manifest, ok := s.candidates[token]
	s.mu.Unlock()
	if !ok {
		return nil, errors.New("update candidate expired; check for updates again")
	}
	body, err := s.get(ctx, manifest.URL)
	if err != nil {
		return nil, err
	}
	file, err := os.CreateTemp(s.tempDir, "odds-update-verify-*")
	if err != nil {
		body.Close()
		return nil, err
	}
	return &verifiedReader{ctx: ctx, body: body, file: file, manifest: manifest,
		current: s.current, public: s.public}, nil
}

// MyGo streams through Read to report progress. Withhold successful EOF until
// the complete signed archive's identity is checked, so extraction and its
// install helper cannot start on an unverified candidate.
type verifiedReader struct {
	ctx             context.Context
	body            io.ReadCloser
	file            *os.File
	manifest        updateartifact.Manifest
	current, public string
	size            int64
	terminal        error
	closed          bool
}

func (r *verifiedReader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, os.ErrClosed
	}
	if r.terminal != nil {
		return 0, r.terminal
	}
	if err := r.ctx.Err(); err != nil {
		r.terminal = err
		return 0, err
	}
	n, err := r.body.Read(p)
	if n > 0 {
		r.size += int64(n)
		if r.size > r.manifest.Size {
			err = errors.New("update archive exceeds declared size")
		} else if written, writeErr := r.file.Write(p[:n]); writeErr != nil {
			err = writeErr
		} else if written != n {
			err = io.ErrShortWrite
		}
	}
	if err == io.EOF {
		if verifyErr := updateartifact.VerifyArchive(r.file, r.manifest, r.current, r.public); verifyErr != nil {
			err = fmt.Errorf("update archive validation: %w", verifyErr)
		}
	}
	if cancelErr := r.ctx.Err(); cancelErr != nil {
		err = cancelErr
	}
	if err != nil {
		r.terminal = err
	}
	return n, err
}

func (r *verifiedReader) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	return errors.Join(r.body.Close(), r.file.Close(), os.Remove(r.file.Name()))
}
