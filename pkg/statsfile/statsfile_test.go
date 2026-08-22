package statsfile

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"testing"
)

func gzipJSON(t *testing.T, v any) []byte {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func statsFor(mapName string, players ...string) *GameStats {
	s := &GameStats{Game: GameInfo{Map: mapName}}
	for i, p := range players {
		s.Players = append(s.Players, Player{Index: i, DisplayName: p})
	}
	return s
}

func TestIdentityOfSortsPlayers(t *testing.T) {
	id := IdentityOf(statsFor("maps/tournament", "Wld", "Gorn", "Mod"))
	want := []string{"Gorn", "Mod", "Wld"}
	if len(id.Players) != len(want) {
		t.Fatalf("got %v, want %v", id.Players, want)
	}
	for i := range want {
		if id.Players[i] != want[i] {
			t.Fatalf("got %v, want %v", id.Players, want)
		}
	}
	if id.Map != "maps/tournament" {
		t.Fatalf("map = %q", id.Map)
	}
}

func TestSameMatchIgnoresPlayerOrder(t *testing.T) {
	// The two clients that upload one match list the roster from their own
	// point of view; slot order must not read as a different match.
	a := IdentityOf(statsFor("maps/alpine", "Wld", "Gorn"))
	b := IdentityOf(statsFor("maps/alpine", "Gorn", "Wld"))
	if !a.SameMatch(b) {
		t.Fatal("same match with reordered players reported as different")
	}
}

func TestSameMatchDetectsDifferentMatches(t *testing.T) {
	cases := []struct {
		name string
		a, b *GameStats
	}{
		{
			name: "different map",
			a:    statsFor("maps/alpine", "Wld", "Gorn"),
			b:    statsFor("maps/tournament", "Wld", "Gorn"),
		},
		{
			name: "different roster",
			a:    statsFor("maps/alpine", "Wld", "Gorn"),
			b:    statsFor("maps/alpine", "Neo", "CDwg"),
		},
		{
			name: "different player count",
			a:    statsFor("maps/alpine", "Wld", "Gorn"),
			b:    statsFor("maps/alpine", "Wld", "Gorn", "Mod"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if IdentityOf(tc.a).SameMatch(IdentityOf(tc.b)) {
				t.Fatal("different matches reported as the same")
			}
		})
	}
}

func TestSameMatchAllowsIncompletePayloads(t *testing.T) {
	// Older exports may carry no map or no roster. Those must not be
	// treated as collisions, or the guard becomes a regression for every
	// upload that predates the fields it reads.
	full := IdentityOf(statsFor("maps/alpine", "Wld", "Gorn"))
	cases := map[string]*GameStats{
		"no map":     statsFor("", "Wld", "Gorn"),
		"no players": statsFor("maps/alpine"),
	}
	for name, partial := range cases {
		t.Run(name, func(t *testing.T) {
			id := IdentityOf(partial)
			if !full.SameMatch(id) || !id.SameMatch(full) {
				t.Fatal("incomplete payload treated as a collision")
			}
		})
	}
}

func TestParseBytesRoundTrip(t *testing.T) {
	want := statsFor("maps/alpine", "Wld", "Gorn")
	got, err := ParseBytes(gzipJSON(t, want))
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if !IdentityOf(got).SameMatch(IdentityOf(want)) {
		t.Fatalf("round trip changed identity: %v vs %v", IdentityOf(got), IdentityOf(want))
	}
}

func TestParseBytesRejectsGarbage(t *testing.T) {
	if _, err := ParseBytes([]byte("not gzip")); err == nil {
		t.Fatal("expected an error for non-gzip input")
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte("{not json")); err != nil {
		t.Fatalf("gzip write: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	if _, err := ParseBytes(buf.Bytes()); err == nil {
		t.Fatal("expected an error for gzipped non-JSON")
	}
}
