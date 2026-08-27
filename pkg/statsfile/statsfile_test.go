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
	if id.Map != "tournament" {
		t.Fatalf("map = %q", id.Map)
	}
}

func TestMapLeaf(t *testing.T) {
	cases := map[string]string{
		`c:\users\bill\documents\command and conquer generals zero hour data\maps\amazonassault\amazonassault.map`: "amazonassault",
		`C:\Users\ktkel\Documents\...\Maps\AmazonAssault\AmazonAssault.map`:                                        "amazonassault",
		"userdata/maps/Alpine Assault/Alpine Assault.map":                                                          "alpine assault",
		"maps/alpine assault":      "alpine assault",
		`Maps\Defcon6\Defcon6.map`: "defcon6",
		"maps/tournament/":         "tournament",
		"":                         "",
	}
	for in, want := range cases {
		if got := MapLeaf(in); got != want {
			t.Errorf("MapLeaf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSameMatchIgnoresPerMachineMapPath(t *testing.T) {
	// The regression this guard shipped with: Game.Map is whatever path the
	// reporting client had open, so the same match uploaded by six players
	// arrived as six strings differing only in the Windows account name.
	// Every upload after the first was refused with a 409.
	base := `\documents\command and conquer generals zero hour data\maps\amazonassault\amazonassault.map`
	first := IdentityOf(statsFor(`c:\users\bill`+base, "131", "Mod", "Neo", "Skip", "Syn", "Wld"))
	for _, user := range []string{"ktkel", "bmfah", "danie", "jcoll", "bdubo"} {
		other := IdentityOf(statsFor(`c:\users\`+user+base, "Wld", "Syn", "Skip", "Neo", "Mod", "131"))
		if !first.SameMatch(other) {
			t.Fatalf("upload from %q reported as a seed collision", user)
		}
	}
}

func TestSameMatchStillCatchesDifferentMapsAcrossMachines(t *testing.T) {
	// Normalizing the path must not normalize away a genuine collision:
	// two different matches that really do share a seed still differ.
	a := IdentityOf(statsFor(`c:\users\bill\...\maps\amazonassault\amazonassault.map`, "131", "Mod"))
	b := IdentityOf(statsFor(`c:\users\ktkel\...\maps\icy frontier\icy frontier.map`, "131", "Mod"))
	if a.SameMatch(b) {
		t.Fatal("different maps on different machines reported as the same match")
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

func TestIdentityOfHumansSkipsAI(t *testing.T) {
	// A match with an AI slot. IdentityOf keeps it (the upload guard wants
	// the strictest comparison); IdentityOfHumans drops it, so a
	// replay-derived roster that never saw the AI still matches.
	s := statsFor("maps/alpine", "Wld", "Gorn")
	s.Players = append(s.Players, Player{
		Index: 2, DisplayName: "Tactical AI", Type: PlayerTypeComputer,
	})

	full := IdentityOf(s)
	if len(full.Players) != 3 {
		t.Fatalf("IdentityOf dropped the AI: %v", full.Players)
	}

	humans := IdentityOfHumans(s)
	if len(humans.Players) != 2 {
		t.Fatalf("IdentityOfHumans kept the AI: %v", humans.Players)
	}

	// This is the real-world case: cncstats records the AI, radarvan's
	// gentool-rewritten replay does not. Same match.
	replaySide := IdentityOf(statsFor("maps/alpine", "Gorn", "Wld"))
	if !humans.SameMatch(replaySide) {
		t.Fatal("human-only comparison still reported a collision")
	}
	if full.SameMatch(replaySide) {
		t.Fatal("whole-roster comparison should still see a difference")
	}
}
