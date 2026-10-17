package crashfile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/bill-rich/cncstats/pkg/logfile"
)

// Stored describes where one envelope landed.
type Stored struct {
	Seed        string   `json:"seed"`   // crash-YYYYMMDD bucket
	Player      string   `json:"player"` // <name>-HHMMSS-<event id prefix>
	Files       []string `json:"files"`
	Attachments []string `json:"attachments"`
	EventID     string   `json:"event_id"`
	DisplayName string   `json:"display_name"`
	Release     string   `json:"release"`
}

// Store writes the envelope under the day's crash bucket. raw is the body as
// received (already gunzipped); received stamps the bucket and the summary.
func Store(raw []byte, env *Envelope, received time.Time) (*Stored, error) {
	ev := env.Event()
	name := safeName(DisplayName(ev))
	if name == "" {
		name = "anonymous"
	}
	eventID := env.EventID()
	short := strings.ReplaceAll(eventID, "-", "")
	if len(short) > 8 {
		short = short[:8]
	}
	if short == "" {
		short = "noid"
	}
	received = received.UTC()
	seed := received.Format("crash-20060102")
	player := fmt.Sprintf("%s-%s-%s", name, received.Format("150405"), short)

	out := &Stored{Seed: seed, Player: player, EventID: eventID, DisplayName: name}
	if ev != nil {
		out.Release = str(ev, "release")
	}

	write := func(filename string, data []byte) error {
		if err := logfile.Store(seed, player, filename, data); err != nil {
			return err
		}
		out.Files = append(out.Files, filename)
		return nil
	}

	if err := write("envelope.raw", raw); err != nil {
		return nil, err
	}
	for i, it := range env.Items {
		switch it.Type() {
		case "event":
			var pretty bytes.Buffer
			if json.Indent(&pretty, it.Payload, "", "  ") == nil {
				if err := write("event.json", pretty.Bytes()); err != nil {
					return nil, err
				}
			} else if err := write("event.json", it.Payload); err != nil {
				return nil, err
			}
		case "attachment":
			fn := safeFilename(strAny(it.Header["filename"]))
			if fn == "" {
				ext := ".bin"
				if strAny(it.Header["attachment_type"]) == "event.minidump" {
					ext = ".dmp"
				}
				fn = fmt.Sprintf("attachment-%d%s", i, ext)
			}
			// Never let an attachment shadow the files this package writes.
			switch fn {
			case "envelope.raw", "event.json", "summary.txt":
				fn = "attachment-" + fn
			}
			if err := write(fn, it.Payload); err != nil {
				return nil, err
			}
			out.Attachments = append(out.Attachments, fn)
		}
	}
	if err := write("summary.txt", []byte(Summary(env, ev, received, out.Attachments))); err != nil {
		return nil, err
	}
	return out, nil
}

func strAny(v any) string {
	s, _ := v.(string)
	return s
}

// safeName reduces a player name to something that can be a directory name
// on the volume and in a zip entry: letters, digits, '.', '_' and '-', at
// most 32 runes. Anything else becomes '_'.
func safeName(s string) string {
	var b strings.Builder
	n := 0
	for _, r := range s {
		if n >= 32 {
			break
		}
		switch {
		case r > unicode.MaxASCII:
			b.WriteRune('_')
		case unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '_' || r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
		n++
	}
	return strings.Trim(b.String(), "._-")
}

// safeFilename keeps an attachment's basename, with the extension intact.
func safeFilename(s string) string {
	s = strings.ReplaceAll(s, "\\", "/")
	s = filepath.Base(s)
	if s == "." || s == "/" {
		return ""
	}
	ext := filepath.Ext(s)
	stem := safeName(strings.TrimSuffix(s, ext))
	if stem == "" {
		return ""
	}
	if len(ext) > 9 || safeName(ext) != strings.TrimPrefix(ext, ".") {
		ext = ""
	}
	return stem + ext
}
