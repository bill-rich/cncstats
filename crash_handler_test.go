package main

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/bill-rich/cncstats/pkg/logfile"
	"github.com/gin-gonic/gin"
)

func TestCrashEnvelopeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	prev := logfile.LogsDir
	logfile.LogsDir = dir
	t.Cleanup(func() { logfile.LogsDir = prev })

	keys := loadAPIKeys("zulu:secret123")
	r := gin.New()
	h := crashEnvelopeHandler(keys, true)
	r.POST("/crash/api/:project/envelope/", h)

	env := []byte(`{"event_id":"0123456789abcdef0123456789abcdef"}` + "\n" +
		`{"type":"event","length":62}` + "\n" +
		`{"release":"zulugo@2.0.5","tags":{"user_displayname":"Skip"}}` + "\n" +
		`{"type":"attachment","length":4,"filename":"x.dmp","attachment_type":"event.minidump"}` + "\n" +
		"MDMP\n")

	// wrong key
	req := httptest.NewRequest(http.MethodPost, "/crash/api/1/envelope/", bytes.NewReader(env))
	req.Header.Set("X-Sentry-Auth", "Sentry sentry_version=7, sentry_client=sentry.native/0.7.0, sentry_key=nope")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("bad key: status %d body %s", w.Code, w.Body.String())
	}

	// right key, gzip body
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	zw.Write(env)
	zw.Close()
	req = httptest.NewRequest(http.MethodPost, "/crash/api/1/envelope/", bytes.NewReader(gz.Bytes()))
	req.Header.Set("X-Sentry-Auth", "Sentry sentry_version=7, sentry_client=sentry.native/0.7.0, sentry_key=secret123")
	req.Header.Set("Content-Encoding", "gzip")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d body %s", w.Code, w.Body.String())
	}
	matches, _ := filepath.Glob(filepath.Join(dir, "crash-*", "Skip-*-01234567", "x.dmp"))
	if len(matches) != 1 {
		t.Fatalf("stored files: %v", matches)
	}
	if b, _ := os.ReadFile(matches[0]); string(b) != "MDMP" {
		t.Errorf("dmp = %q", b)
	}
	sum, _ := os.ReadFile(filepath.Join(filepath.Dir(matches[0]), "summary.txt"))
	if !bytes.Contains(sum, []byte("release:     zulugo@2.0.5")) {
		t.Errorf("summary:\n%s", sum)
	}

	// garbage
	req = httptest.NewRequest(http.MethodPost, "/crash/api/1/envelope/", bytes.NewReader([]byte("nope")))
	req.Header.Set("X-Sentry-Auth", "Sentry sentry_key=secret123")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("garbage: status %d", w.Code)
	}
}

func TestCrashEnvelopeHandlerIgnoresSessions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	prev := logfile.LogsDir
	logfile.LogsDir = dir
	t.Cleanup(func() { logfile.LogsDir = prev })

	r := gin.New()
	r.POST("/crash/api/:project/envelope/", crashEnvelopeHandler(loadAPIKeys("zulu:k"), true))
	env := []byte(`{"event_id":"abc"}` + "\n" + `{"type":"session"}` + "\n" + `{"status":"exited"}` + "\n")
	req := httptest.NewRequest(http.MethodPost, "/crash/api/1/envelope/", bytes.NewReader(env))
	req.Header.Set("X-Sentry-Auth", "Sentry sentry_key=k")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "crash-*")); len(matches) != 0 {
		t.Fatalf("session envelope was stored: %v", matches)
	}
}
