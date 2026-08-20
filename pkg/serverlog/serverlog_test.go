package serverlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteAppendsToTodaysFile(t *testing.T) {
	dir := t.TempDir()
	w, err := Open(dir, "cncstats", 30)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer w.Close()

	if _, err := w.Write([]byte("hello\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := w.Write([]byte("world\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	want := filepath.Join(dir, "cncstats-"+time.Now().UTC().Format("2006-01-02")+".log")
	if w.Path() != want {
		t.Fatalf("Path() = %q, want %q", w.Path(), want)
	}
	data, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "hello\nworld\n" {
		t.Fatalf("file contents = %q", string(data))
	}
}

func TestReopenAppendsRatherThanTruncates(t *testing.T) {
	dir := t.TempDir()
	w, err := Open(dir, "cncstats", 30)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	w.Write([]byte("first\n"))
	w.Close()

	w2, err := Open(dir, "cncstats", 30)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer w2.Close()
	w2.Write([]byte("second\n"))

	data, err := os.ReadFile(w2.Path())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "first\nsecond\n" {
		t.Fatalf("restart truncated the log: %q", string(data))
	}
}

func TestWriteRollsOverOnDayChange(t *testing.T) {
	dir := t.TempDir()
	w, err := Open(dir, "cncstats", 30)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer w.Close()
	w.Write([]byte("yesterday\n"))

	// Pretend the open file belongs to an earlier day; the next write must
	// roll over to today's file instead of appending to it.
	stale := w.Path()
	w.mu.Lock()
	w.day = "2000-01-01"
	w.mu.Unlock()

	w.Write([]byte("today\n"))
	if w.Path() != stale {
		t.Fatalf("expected roll back to today's file %q, got %q", stale, w.Path())
	}
}

func TestPruneRemovesOldFilesOnly(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "cncstats-2000-01-02.log")
	keep := filepath.Join(dir, "cncstats-"+time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02")+".log")
	other := filepath.Join(dir, "unrelated.txt")
	for _, p := range []string{old, keep, other} {
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatalf("seed %s: %v", p, err)
		}
	}

	w, err := Open(dir, "cncstats", 7) // Open prunes
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer w.Close()

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("expected %s to be pruned", old)
	}
	if _, err := os.Stat(keep); err != nil {
		t.Errorf("expected %s to be kept: %v", keep, err)
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("expected %s (not ours) to be untouched: %v", other, err)
	}
}

func TestRetentionDisabled(t *testing.T) {
	dir := t.TempDir()
	old := filepath.Join(dir, "cncstats-2000-01-02.log")
	if err := os.WriteFile(old, []byte("x"), 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}
	w, err := Open(dir, "cncstats", 0)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer w.Close()
	if _, err := os.Stat(old); err != nil {
		t.Errorf("retainDays<=0 must disable pruning: %v", err)
	}
}
