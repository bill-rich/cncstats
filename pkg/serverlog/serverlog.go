// Package serverlog persists the process's log stream to disk.
//
// The service normally logs to stderr, which docker captures per container.
// That stream dies with the container: recreating it on a redeploy discards
// every line the old one wrote, and those lines are the only record of what
// the online coordinator told a player whose host/join attempt failed. This
// package tees the same stream into a file under a mounted volume, one file
// per UTC day, with a retention window so it can't grow without bound.
//
// Layout under Dir:
//
//	<prefix>-YYYY-MM-DD.log
//
// A Writer is safe for concurrent use: logrus writes from every request
// goroutine and from the coordinator's session goroutines.
package serverlog

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Writer is an io.Writer that appends to a per-day log file, rolling over
// at UTC midnight and pruning files older than the retention window.
type Writer struct {
	mu         sync.Mutex
	dir        string
	prefix     string
	retainDays int
	day        string // "2006-01-02" of the currently open file
	f          *os.File
}

// Open creates dir if needed and opens today's log file for appending.
// retainDays <= 0 disables pruning.
func Open(dir, prefix string, retainDays int) (*Writer, error) {
	if dir == "" {
		return nil, fmt.Errorf("serverlog: empty dir")
	}
	if prefix == "" {
		prefix = "server"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("serverlog: create dir: %w", err)
	}
	w := &Writer{dir: dir, prefix: prefix, retainDays: retainDays}
	if err := w.rollTo(time.Now().UTC().Format("2006-01-02")); err != nil {
		return nil, err
	}
	return w, nil
}

// Path returns the file currently being written.
func (w *Writer) Path() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return ""
	}
	return w.f.Name()
}

// Write appends p to today's file, rolling over first if the UTC day
// changed since the last write. A write that fails to roll over falls back
// to the already-open file rather than dropping the line.
func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if today := time.Now().UTC().Format("2006-01-02"); today != w.day {
		// Roll failures are non-fatal: keep writing to the open file so a
		// full disk or a permissions change can't silence the log.
		_ = w.rollTo(today)
	}
	if w.f == nil {
		return len(p), nil
	}
	return w.f.Write(p)
}

// Close closes the current file.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f == nil {
		return nil
	}
	err := w.f.Close()
	w.f = nil
	return err
}

// rollTo opens the file for the given day. Caller holds w.mu.
func (w *Writer) rollTo(day string) error {
	name := filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.prefix, day))
	f, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("serverlog: open %s: %w", name, err)
	}
	if w.f != nil {
		w.f.Close()
	}
	w.f = f
	w.day = day
	w.prune()
	return nil
}

// prune removes log files older than the retention window. Files are
// selected by the date in their name, so a stopped service that never
// wrote for a week still cleans up correctly on its next start. Caller
// holds w.mu.
func (w *Writer) prune() {
	if w.retainDays <= 0 {
		return
	}
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -w.retainDays)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, w.prefix+"-") || !strings.HasSuffix(name, ".log") {
			continue
		}
		datePart := strings.TrimSuffix(strings.TrimPrefix(name, w.prefix+"-"), ".log")
		day, err := time.Parse("2006-01-02", datePart)
		if err != nil {
			continue
		}
		if day.Before(cutoff) {
			os.Remove(filepath.Join(w.dir, name))
		}
	}
}

// Files lists the stored log files, newest first. Useful for tooling that
// wants to fetch yesterday's log without knowing the naming scheme.
func Files(dir, prefix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), prefix+"-") && strings.HasSuffix(e.Name(), ".log") {
			out = append(out, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(out)))
	return out, nil
}
