// Package crashfile receives Sentry envelopes posted by the Zulu GO client's
// sentry-native SDK and stores them next to the per-match logs, so a crash
// is read the same way everything else from a game night is read: pulled
// off the logs volume and grepped.
//
// The client is pointed at this server with a DSN of the form
//
//	https://<key>@cncstats.computersrfun.org/crash/<project>
//
// and sentry-native then POSTs to /crash/api/<project>/envelope/. One
// envelope is one crash (or one message event): a JSON header line, then
// items, each a JSON header line followed by a payload of the announced
// length. The event item is JSON; a minidump rides along as an attachment.
//
// Layout under logfile.LogsDir, reusing its <seed>/<player>/<file> scheme
// so /get_logs?seed=crash-YYYYMMDD returns a whole day's crashes as a zip:
//
//	crash-YYYYMMDD/
//	  <displayname>-HHMMSS-<event id prefix>/
//	    envelope.raw      # the envelope exactly as received (after gunzip)
//	    event.json        # the event item, pretty-printed
//	    <attachment>      # each attachment under its own filename (the minidump)
//	    summary.txt       # release, user, exception and frames as module+offset
package crashfile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Item is one envelope item: its header fields and the raw payload.
type Item struct {
	Header  map[string]any
	Payload []byte
}

// Type returns the item's "type" header, or "" when absent.
func (it Item) Type() string {
	s, _ := it.Header["type"].(string)
	return s
}

// Envelope is a parsed Sentry envelope.
type Envelope struct {
	Header map[string]any
	Items  []Item
}

// EventID returns the envelope header's event_id, or "" when absent.
func (e *Envelope) EventID() string {
	s, _ := e.Header["event_id"].(string)
	return s
}

// Event returns the first item of type "event" decoded as JSON, or nil when
// the envelope carries none (a session or a bare attachment envelope).
func (e *Envelope) Event() map[string]any {
	for _, it := range e.Items {
		if it.Type() != "event" {
			continue
		}
		var ev map[string]any
		if json.Unmarshal(it.Payload, &ev) == nil {
			return ev
		}
	}
	return nil
}

// HasAttachment reports whether any item is an attachment (a minidump).
func (e *Envelope) HasAttachment() bool {
	for _, it := range e.Items {
		if it.Type() == "attachment" {
			return true
		}
	}
	return false
}

// ErrMalformed is returned for a body that is not an envelope.
var ErrMalformed = errors.New("crashfile: malformed envelope")

// Parse decodes an envelope body. Items with a "length" header are read
// exactly; items without one run to the next newline (or the end of the
// body), per the envelope spec. Trailing newlines after a payload are
// consumed.
func Parse(body []byte) (*Envelope, error) {
	headerLine, rest, ok := bytes.Cut(body, []byte{'\n'})
	if !ok && len(body) == 0 {
		return nil, ErrMalformed
	}
	if !ok {
		headerLine, rest = body, nil
	}
	env := &Envelope{}
	if err := json.Unmarshal(headerLine, &env.Header); err != nil {
		return nil, fmt.Errorf("%w: header: %v", ErrMalformed, err)
	}
	for len(rest) > 0 {
		// Skip blank separators between items.
		if rest[0] == '\n' {
			rest = rest[1:]
			continue
		}
		itemHeaderLine, after, ok := bytes.Cut(rest, []byte{'\n'})
		if !ok {
			// A header with nothing after it: tolerate an empty final item.
			after = nil
		}
		var it Item
		if err := json.Unmarshal(itemHeaderLine, &it.Header); err != nil {
			return nil, fmt.Errorf("%w: item header: %v", ErrMalformed, err)
		}
		if n, has := itemLength(it.Header); has {
			if n < 0 || n > len(after) {
				return nil, fmt.Errorf("%w: item length %d exceeds body", ErrMalformed, n)
			}
			it.Payload = after[:n]
			rest = after[n:]
			if len(rest) > 0 && rest[0] == '\n' {
				rest = rest[1:]
			}
		} else {
			payload, tail, _ := bytes.Cut(after, []byte{'\n'})
			it.Payload = payload
			rest = tail
		}
		env.Items = append(env.Items, it)
	}
	return env, nil
}

func itemLength(h map[string]any) (int, bool) {
	v, ok := h["length"]
	if !ok {
		return 0, false
	}
	switch n := v.(type) {
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	}
	return 0, false
}

// image is one entry of the event's debug_meta.images list.
type image struct {
	codeFile  string
	debugFile string
	debugID   string
	addr      uint64
	size      uint64
}

func (im image) name() string {
	n := im.codeFile
	if n == "" {
		n = im.debugFile
	}
	if i := strings.LastIndexAny(n, `\/`); i >= 0 {
		n = n[i+1:]
	}
	if n == "" {
		n = "?"
	}
	return n
}

func parseAddr(v any) (uint64, bool) {
	switch a := v.(type) {
	case string:
		s := strings.TrimPrefix(strings.ToLower(a), "0x")
		n, err := strconv.ParseUint(s, 16, 64)
		return n, err == nil
	case float64:
		return uint64(a), true
	}
	return 0, false
}

func images(ev map[string]any) []image {
	dm, _ := ev["debug_meta"].(map[string]any)
	list, _ := dm["images"].([]any)
	out := make([]image, 0, len(list))
	for _, raw := range list {
		m, _ := raw.(map[string]any)
		if m == nil {
			continue
		}
		var im image
		im.codeFile, _ = m["code_file"].(string)
		im.debugFile, _ = m["debug_file"].(string)
		im.debugID, _ = m["debug_id"].(string)
		im.addr, _ = parseAddr(m["image_addr"])
		im.size, _ = parseAddr(m["image_size"])
		out = append(out, im)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].addr < out[j].addr })
	return out
}

func findImage(imgs []image, addr uint64) *image {
	for i := range imgs {
		im := &imgs[i]
		if addr >= im.addr && (im.size == 0 || addr < im.addr+im.size) {
			return im
		}
	}
	return nil
}

// DisplayName picks the player's name the way the client tags it:
// tags.user_displayname, then user.username, then user.id. "" when none.
func DisplayName(ev map[string]any) string {
	if tags, ok := ev["tags"].(map[string]any); ok {
		if s, _ := tags["user_displayname"].(string); s != "" && s != "Unknown" {
			return s
		}
	}
	if user, ok := ev["user"].(map[string]any); ok {
		if s, _ := user["username"].(string); s != "" {
			return s
		}
		if s, _ := user["id"].(string); s != "" {
			return s
		}
	}
	return ""
}

func str(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	switch v := m[key].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	}
	return ""
}

// Summary renders the parts of an event a person wants first: who, which
// build, what blew up, and the crashing stack as module+offset so it can be
// symbolicated against that build's PDB without a Sentry server. received is
// the server time the envelope arrived; attachments lists stored files.
func Summary(env *Envelope, ev map[string]any, received time.Time, attachments []string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "received:    %s\n", received.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "event_id:    %s\n", env.EventID())
	if ev == nil {
		b.WriteString("event:       none in envelope\n")
		writeAttachments(&b, attachments)
		return b.String()
	}
	fmt.Fprintf(&b, "player:      %s\n", orDash(DisplayName(ev)))
	fmt.Fprintf(&b, "release:     %s\n", orDash(str(ev, "release")))
	fmt.Fprintf(&b, "environment: %s\n", orDash(str(ev, "environment")))
	fmt.Fprintf(&b, "level:       %s\n", orDash(str(ev, "level")))
	fmt.Fprintf(&b, "timestamp:   %s\n", orDash(str(ev, "timestamp")))
	if msg, ok := ev["message"].(map[string]any); ok {
		fmt.Fprintf(&b, "message:     %s\n", orDash(str(msg, "formatted")))
	} else if s := str(ev, "message"); s != "" {
		fmt.Fprintf(&b, "message:     %s\n", s)
	}
	if ctx, ok := ev["contexts"].(map[string]any); ok {
		if osCtx, ok := ctx["os"].(map[string]any); ok {
			fmt.Fprintf(&b, "os:          %s %s %s\n", str(osCtx, "name"), str(osCtx, "version"), str(osCtx, "build"))
		}
	}
	if tags, ok := ev["tags"].(map[string]any); ok && len(tags) > 0 {
		keys := make([]string, 0, len(tags))
		for k := range tags {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("tags:")
		for _, k := range keys {
			fmt.Fprintf(&b, " %s=%s", k, str(tags, k))
		}
		b.WriteString("\n")
	}

	imgs := images(ev)
	writeExceptions(&b, ev, imgs)
	writeThreads(&b, ev, imgs)
	if len(imgs) > 0 {
		b.WriteString("\nmodules (name image_addr size debug_id):\n")
		for _, im := range imgs {
			fmt.Fprintf(&b, "  %-32s 0x%08x 0x%08x %s\n", im.name(), im.addr, im.size, orDash(im.debugID))
		}
	}
	writeAttachments(&b, attachments)
	return b.String()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func writeAttachments(b *strings.Builder, attachments []string) {
	if len(attachments) == 0 {
		return
	}
	b.WriteString("\nattachments:\n")
	for _, a := range attachments {
		fmt.Fprintf(b, "  %s\n", a)
	}
}

func writeExceptions(b *strings.Builder, ev map[string]any, imgs []image) {
	exc, _ := ev["exception"].(map[string]any)
	values, _ := exc["values"].([]any)
	for i, raw := range values {
		e, _ := raw.(map[string]any)
		if e == nil {
			continue
		}
		fmt.Fprintf(b, "\nexception[%d]: %s: %s\n", i, orDash(str(e, "type")), orDash(str(e, "value")))
		if mech, ok := e["mechanism"].(map[string]any); ok {
			fmt.Fprintf(b, "  mechanism: %s handled=%s\n", orDash(str(mech, "type")), orDash(str(mech, "handled")))
			if meta, ok := mech["meta"].(map[string]any); ok {
				if sig, ok := meta["signal"].(map[string]any); ok {
					fmt.Fprintf(b, "  signal: %s %s\n", str(sig, "name"), str(sig, "number"))
				}
				if mx, ok := meta["mach_exception"].(map[string]any); ok {
					fmt.Fprintf(b, "  exception: %s\n", str(mx, "name"))
				}
			}
		}
		if tid := str(e, "thread_id"); tid != "" {
			fmt.Fprintf(b, "  thread_id: %s\n", tid)
		}
		if st, ok := e["stacktrace"].(map[string]any); ok {
			writeFrames(b, st, imgs)
		}
	}
}

func writeThreads(b *strings.Builder, ev map[string]any, imgs []image) {
	th, _ := ev["threads"].(map[string]any)
	values, _ := th["values"].([]any)
	for _, raw := range values {
		t, _ := raw.(map[string]any)
		if t == nil {
			continue
		}
		crashed, _ := t["crashed"].(bool)
		current, _ := t["current"].(bool)
		if !crashed && !current {
			continue
		}
		fmt.Fprintf(b, "\nthread %s (%s) crashed=%t current=%t\n", orDash(str(t, "id")), orDash(str(t, "name")), crashed, current)
		if st, ok := t["stacktrace"].(map[string]any); ok {
			writeFrames(b, st, imgs)
		}
	}
}

// writeFrames prints a stacktrace innermost first (Sentry stores frames
// outermost first), each as module+rva so llvm-symbolizer --relative-address
// resolves it against the PDB.
func writeFrames(b *strings.Builder, st map[string]any, imgs []image) {
	frames, _ := st["frames"].([]any)
	if len(frames) == 0 {
		return
	}
	b.WriteString("  frames (innermost first):\n")
	for i := len(frames) - 1; i >= 0; i-- {
		f, _ := frames[i].(map[string]any)
		if f == nil {
			continue
		}
		addr, ok := parseAddr(f["instruction_addr"])
		where := "?"
		if ok {
			where = fmt.Sprintf("0x%08x", addr)
			if im := findImage(imgs, addr); im != nil {
				where = fmt.Sprintf("%s+0x%x (0x%08x)", im.name(), addr-im.addr, addr)
			}
		}
		fn := str(f, "function")
		if fn == "" {
			fn = str(f, "symbol")
		}
		if fn != "" {
			where += "  " + fn
		}
		fmt.Fprintf(b, "    #%-2d %s\n", len(frames)-1-i, where)
	}
}
