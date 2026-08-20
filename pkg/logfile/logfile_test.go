package logfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func withTempLogsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev := LogsDir
	LogsDir = dir
	t.Cleanup(func() { LogsDir = prev })
	return dir
}

func TestStoreAndLoadNumericSeed(t *testing.T) {
	dir := withTempLogsDir(t)
	if err := Store("7774140", "131", "ReleaseLog.txt.gz", []byte("payload")); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if !Exists("7774140") {
		t.Error("Exists = false after Store")
	}
	got, err := Load("7774140", "131", "ReleaseLog.txt.gz")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if string(got) != "payload" {
		t.Errorf("Load = %q", string(got))
	}
	if _, err := os.Stat(filepath.Join(dir, "7774140", "131", "ReleaseLog.txt.gz")); err != nil {
		t.Errorf("unexpected on-disk layout: %v", err)
	}
	files, err := List("7774140")
	if err != nil || len(files) != 1 || files[0].Player != "131" {
		t.Errorf("List = %+v, err %v", files, err)
	}
}

// Connection-failure uploads have no match to key off, so they use a
// non-numeric daily bucket. That has to round-trip like any other seed.
func TestStoreConnectionFailureBucket(t *testing.T) {
	withTempLogsDir(t)
	const seed = "connfail-20260820"
	if err := Store(seed, "CD-201455-host", "ReleaseLog.txt.gz", []byte("x")); err != nil {
		t.Fatalf("Store: %v", err)
	}
	if err := Store(seed, "CD-201502-join", "ReleaseLog.txt.gz", []byte("y")); err != nil {
		t.Fatalf("Store: %v", err)
	}
	files, err := List(seed)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("expected both attempts under one bucket, got %+v", files)
	}
}

func TestSeedCannotEscapeLogsDir(t *testing.T) {
	dir := withTempLogsDir(t)
	for _, seed := range []string{"../evil", "..\\evil", "a/../../evil", "/etc/evil"} {
		if err := Store(seed, "p", "f.gz", []byte("x")); err != nil {
			continue // rejected outright is fine too
		}
		// Stored: the file must still be inside LogsDir.
		walked := false
		filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				walked = true
				if !strings.HasPrefix(path, dir) {
					t.Errorf("seed %q escaped LogsDir: %s", seed, path)
				}
			}
			return nil
		})
		if !walked {
			t.Errorf("seed %q stored nothing under LogsDir", seed)
		}
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "evil")); !os.IsNotExist(err) {
		t.Error("a sibling of LogsDir was created")
	}
}

func TestEmptyAndUnusableSeedsRejected(t *testing.T) {
	withTempLogsDir(t)
	for _, seed := range []string{"", "..", ".", "   "} {
		if err := Store(seed, "p", "f.gz", []byte("x")); err == nil {
			t.Errorf("Store(%q) = nil, want error", seed)
		}
		if Exists(seed) {
			t.Errorf("Exists(%q) = true", seed)
		}
		if _, err := List(seed); err == nil {
			t.Errorf("List(%q) = nil error", seed)
		}
	}
}
