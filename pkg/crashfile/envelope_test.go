package crashfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/bill-rich/cncstats/pkg/logfile"
)

func withTempLogsDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	prev := logfile.LogsDir
	logfile.LogsDir = dir
	t.Cleanup(func() { logfile.LogsDir = prev })
	return dir
}

// sampleEvent is the shape sentry-native's inproc backend produces for an
// access violation on a non-main thread: the exception with a stacktrace of
// bare instruction addresses, and the module list to resolve them against.
func sampleEvent() map[string]any {
	return map[string]any{
		"event_id":    "9b5e3a7c1d2f4e6a8b0c1d2e3f4a5b6c",
		"release":     "zulugo@2.0.5",
		"environment": "zulugo",
		"level":       "fatal",
		"platform":    "native",
		"timestamp":   "2026-10-15T01:38:39Z",
		"tags":        map[string]any{"user_displayname": "Skip", "user_id": "2"},
		"user":        map[string]any{"id": "2", "username": "Skip"},
		"exception": map[string]any{
			"values": []any{map[string]any{
				"type":      "EXCEPTION_ACCESS_VIOLATION",
				"value":     "Fatal Error: EXCEPTION_ACCESS_VIOLATION",
				"thread_id": "4812",
				"mechanism": map[string]any{"type": "sentry.native", "handled": false},
				"stacktrace": map[string]any{"frames": []any{
					map[string]any{"instruction_addr": "0x77e51234"},
					map[string]any{"instruction_addr": "0x00512345"},
					map[string]any{"instruction_addr": "0x00401000"},
				}},
			}},
		},
		"debug_meta": map[string]any{"images": []any{
			map[string]any{"type": "pe", "code_file": `C:\Games\ZuluGO\ZuluGO.exe`, "image_addr": "0x00400000", "image_size": "0x01000000", "debug_id": "1234-5678"},
			map[string]any{"type": "pe", "code_file": `C:\Windows\System32\ntdll.dll`, "image_addr": "0x77e00000", "image_size": "0x00100000"},
		}},
	}
}

func buildEnvelope(t *testing.T, ev map[string]any, attachments map[string][]byte) []byte {
	t.Helper()
	var b bytes.Buffer
	hdr, _ := json.Marshal(map[string]any{"event_id": ev["event_id"], "sent_at": "2026-10-15T01:38:40Z"})
	b.Write(hdr)
	b.WriteByte('\n')
	evBytes, _ := json.Marshal(ev)
	fmt.Fprintf(&b, `{"type":"event","length":%d}`+"\n", len(evBytes))
	b.Write(evBytes)
	b.WriteByte('\n')
	for name, data := range attachments {
		fmt.Fprintf(&b, `{"type":"attachment","length":%d,"filename":%q,"attachment_type":"event.minidump"}`+"\n", len(data), name)
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.Bytes()
}

func TestParseRoundTrip(t *testing.T) {
	dump := []byte("MDMP\x00\x01\x02\x03\nbinary with newline\n\n")
	body := buildEnvelope(t, sampleEvent(), map[string][]byte{"crash.dmp": dump})
	env, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if env.EventID() != "9b5e3a7c1d2f4e6a8b0c1d2e3f4a5b6c" {
		t.Errorf("EventID = %q", env.EventID())
	}
	if len(env.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(env.Items))
	}
	if env.Items[0].Type() != "event" || env.Items[1].Type() != "attachment" {
		t.Errorf("item types = %q %q", env.Items[0].Type(), env.Items[1].Type())
	}
	if !bytes.Equal(env.Items[1].Payload, dump) {
		t.Errorf("attachment payload altered: %q", env.Items[1].Payload)
	}
	ev := env.Event()
	if ev == nil || ev["release"] != "zulugo@2.0.5" {
		t.Errorf("Event = %v", ev)
	}
	if DisplayName(ev) != "Skip" {
		t.Errorf("DisplayName = %q", DisplayName(ev))
	}
}

func TestParseItemsWithoutLength(t *testing.T) {
	body := []byte(`{"event_id":"abc"}` + "\n" +
		`{"type":"session"}` + "\n" +
		`{"status":"exited"}` + "\n" +
		`{"type":"event"}` + "\n" +
		`{"release":"zulugo@2.0.5"}`)
	env, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(env.Items) != 2 {
		t.Fatalf("items = %d, want 2", len(env.Items))
	}
	if string(env.Items[0].Payload) != `{"status":"exited"}` {
		t.Errorf("session payload = %q", env.Items[0].Payload)
	}
	if ev := env.Event(); ev == nil || ev["release"] != "zulugo@2.0.5" {
		t.Errorf("Event = %v", ev)
	}
}

func TestParseRejectsGarbage(t *testing.T) {
	for _, body := range []string{"", "not json\n", `{"event_id":"x"}` + "\n" + `{"type":"event","length":999}` + "\nshort"} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("Parse(%q) accepted", body)
		}
	}
}

func TestSummaryResolvesModules(t *testing.T) {
	env := &Envelope{Header: map[string]any{"event_id": "abc"}}
	s := Summary(env, sampleEvent(), time.Date(2026, 10, 15, 1, 38, 40, 0, time.UTC), []string{"crash.dmp"})
	for _, want := range []string{
		"player:      Skip",
		"release:     zulugo@2.0.5",
		"exception[0]: EXCEPTION_ACCESS_VIOLATION",
		"thread_id: 4812",
		"#0  ZuluGO.exe+0x1000 (0x00401000)",
		"#1  ZuluGO.exe+0x112345 (0x00512345)",
		"#2  ntdll.dll+0x51234 (0x77e51234)",
		"ZuluGO.exe                       0x00400000 0x01000000 1234-5678",
		"attachments:\n  crash.dmp",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("summary missing %q:\n%s", want, s)
		}
	}
}

func TestStoreLayout(t *testing.T) {
	dir := withTempLogsDir(t)
	body := buildEnvelope(t, sampleEvent(), map[string][]byte{`C:\x\crash.dmp`: []byte("MDMP"), "summary.txt": []byte("spoof")})
	env, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	received := time.Date(2026, 10, 15, 1, 38, 40, 0, time.UTC)
	st, err := Store(body, env, received)
	if err != nil {
		t.Fatalf("Store: %v", err)
	}
	if st.Seed != "crash-20261015" {
		t.Errorf("Seed = %q", st.Seed)
	}
	if st.Player != "Skip-013840-9b5e3a7c" {
		t.Errorf("Player = %q", st.Player)
	}
	base := filepath.Join(dir, st.Seed, st.Player)
	for _, f := range []string{"envelope.raw", "event.json", "crash.dmp", "attachment-summary.txt", "summary.txt"} {
		if _, err := os.Stat(filepath.Join(base, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	sum, _ := os.ReadFile(filepath.Join(base, "summary.txt"))
	if strings.Contains(string(sum), "spoof") {
		t.Error("attachment overwrote summary.txt")
	}
	raw, _ := os.ReadFile(filepath.Join(base, "envelope.raw"))
	if !bytes.Equal(raw, body) {
		t.Error("envelope.raw is not the received body")
	}
}

func TestStoreAnonymousAndOddNames(t *testing.T) {
	withTempLogsDir(t)
	ev := sampleEvent()
	delete(ev, "tags")
	delete(ev, "user")
	env, err := Parse(buildEnvelope(t, ev, nil))
	if err != nil {
		t.Fatal(err)
	}
	st, err := Store(nil, env, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(st.Player, "anonymous-") {
		t.Errorf("Player = %q", st.Player)
	}
	if got := safeName("../Évil Name/..."); got != "vil_Name" {
		t.Errorf("safeName = %q", got)
	}
	if got := safeFilename(`..\..\Crash F-20261015.dmp`); got != "Crash_F-20261015.dmp" {
		t.Errorf("safeFilename = %q", got)
	}
}
