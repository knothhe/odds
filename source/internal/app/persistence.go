package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/egoist/mygo"

	"github.com/knothhe/odds/source/internal/uicopy"
)

type configWrite struct {
	revision uint64
	config   Config
}

// configWriter is the application's only runtime file writer. While a write
// runs, the single pending slot keeps the latest complete snapshot. Callbacks
// run in write order; the application delivers them to the UI thread.
type configWriter struct {
	jobs    chan configWrite
	mu      sync.Mutex
	latest  saveCompletion
	changed chan struct{}
}

type saveCompletion struct {
	revision uint64
	err      error
}

func newConfigWriter(write func(Config) error, done func(uint64, error)) *configWriter {
	w := &configWriter{jobs: make(chan configWrite, 1), changed: make(chan struct{})}
	go func() {
		for job := range w.jobs {
			err := write(job.config)
			w.mu.Lock()
			w.latest = saveCompletion{job.revision, err}
			close(w.changed)
			w.changed = make(chan struct{})
			w.mu.Unlock()
			done(job.revision, err)
		}
	}()
	return w
}

// submit and close have one owner: the main thread. Rendering records a save
// intent here; disk access stays on the worker created at startup.
func (w *configWriter) submit(job configWrite) {
	select {
	case w.jobs <- job:
		return
	default:
	}
	select {
	case <-w.jobs:
	default:
	}
	w.jobs <- job
}

func (w *configWriter) close() { close(w.jobs) }

// The quit barrier waits on worker completion, independently of queued UI
// callbacks. Keeping a successful native quit uncancelled also preserves MyGo's
// Relaunch request after an update. Ordinary edits remain fully asynchronous.
func (w *configWriter) wait(revision uint64, timeout time.Duration) (error, bool) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		w.mu.Lock()
		latest, changed := w.latest, w.changed
		w.mu.Unlock()
		if latest.revision >= revision {
			return latest.err, true
		}
		select {
		case <-changed:
		case <-timer.C:
			return nil, false
		}
	}
}

func (a *App) saved(revision uint64, err error) {
	// A superseded completion cannot mark newer edits as saved or failed.
	if revision != a.saveRevision {
		return
	}
	a.saving = false
	a.saveCause = err
	if err != nil {
		a.saveErr = saveErrorMessage(err)
		log.Printf("save settings: %v", err)
	} else {
		a.saveErr = ""
		a.quitBlocked = false
	}
	if a.settings != nil {
		a.settings.Invalidate()
	}
}

func saveErrorMessage(err error) string {
	switch {
	case errors.Is(err, os.ErrPermission):
		return uicopy.SavePermission()
	case errors.Is(err, syscall.ENOSPC):
		return uicopy.SaveDiskFull()
	default:
		var pathErr *os.PathError
		if errors.As(err, &pathErr) {
			return fmt.Sprintf(uicopy.SaveWriteFailure(), pathErr.Err.Error())
		}
		return err.Error()
	}
}

func (a *App) beforeQuit(e *mygo.QuitEvent) {
	if a.quitting {
		return
	}
	if a.saving {
		err, finished := a.writer.wait(a.saveRevision, 3*time.Second)
		if !finished {
			e.PreventDefault()
			a.saveCause = nil
			a.saveErr = uicopy.SaveStillPending()
			a.quitBlocked = true
			a.openSettings()
			return
		}
		a.saving = false
		a.saveCause = err
		if err != nil {
			a.saveErr = saveErrorMessage(err)
		} else {
			a.saveErr = ""
		}
	}
	if a.saveErr != "" && !a.readOnly {
		e.PreventDefault()
		a.quitBlocked = true
		a.openSettings()
		return
	}
	a.quitting = true
}

func applicationVersion(version string) string {
	if strings.TrimSpace(version) == "" {
		return uicopy.DevelopmentBuild()
	}
	return version
}
