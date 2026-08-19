package zhreplay

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/bill-rich/cncstats/pkg/statsfile"
	"github.com/bill-rich/cncstats/pkg/zhreplay/header"
	"github.com/bill-rich/cncstats/pkg/zhreplay/object"
)

// Mirrors the shape StatsExporter.cpp emits for version 3 stats JSON.
const huntedStatsJSON = `{
"version":3,
"game":{"map":"maps/test","mode":"LAN","frameCount":30000,"seed":42,"replayFile":"test.rep","playerCount":2,"snapshotInterval":30},
"players":[
{"index":1,"displayName":"Alice","side":"America","type":"Human","color":"#FF0000","money":100,"moneyEarned":5000,"moneySpent":4000,"score":10},
{"index":2,"displayName":"Bob","side":"GLA","type":"Human","color":"#00FF00","money":200,"moneyEarned":6000,"moneySpent":5000,"score":20}
],
"buildEvents":[],
"killEvents":[],
"captureEvents":[],
"energyEvents":[],
"rankEvents":[],
"skillPointsEvents":[],
"sciencePointsEvents":[],
"radarEvents":[],
"deathEvents":[],
"huntedEvents":[
{"frame":9000,"player":2,"hunted":true},
{"frame":9600,"player":2,"hunted":false},
{"frame":21000,"player":2,"hunted":true}
],
"battlePlanEvents":[],
"timeSeries":{"players":[]}
}`

func huntedTestStats(t *testing.T) *statsfile.GameStats {
	t.Helper()
	var stats statsfile.GameStats
	if err := json.Unmarshal([]byte(huntedStatsJSON), &stats); err != nil {
		t.Fatalf("unmarshal stats JSON: %v", err)
	}
	return &stats
}

func TestHuntedEventsParse(t *testing.T) {
	stats := huntedTestStats(t)

	if len(stats.HuntedEvents) != 3 {
		t.Fatalf("expected 3 hunted events, got %d", len(stats.HuntedEvents))
	}
	first := stats.HuntedEvents[0]
	if first.Frame != 9000 || first.Player != 2 || !first.Hunted {
		t.Errorf("unexpected first hunted event: %+v", first)
	}
	second := stats.HuntedEvents[1]
	if second.Frame != 9600 || second.Player != 2 || second.Hunted {
		t.Errorf("unexpected second hunted event: %+v", second)
	}
}

func TestHuntedSummaryFold(t *testing.T) {
	stats := huntedTestStats(t)

	replay := &Replay{
		Header: &header.GeneralsHeader{},
		Summary: []*object.PlayerSummary{
			{Name: "Alice", Side: "USA", Team: 1},
			{Name: "Bob", Side: "GLA", Team: 2},
		},
	}

	v2 := ConvertToEnhancedReplayV2(replay, stats, nil)

	if v2.Stats == nil || len(v2.Stats.HuntedEvents) != 3 {
		t.Fatalf("hunted events not passed through to enriched stats: %+v", v2.Stats)
	}

	alice := v2.Summary[0]
	if alice.HuntedFrame != 0 || alice.UnhuntedFrame != 0 || alice.Hunted {
		t.Errorf("Alice should have no hunted state, got frame=%d unhunted=%d hunted=%v",
			alice.HuntedFrame, alice.UnhuntedFrame, alice.Hunted)
	}

	bob := v2.Summary[1]
	if bob.HuntedFrame != 9000 {
		t.Errorf("Bob HuntedFrame = %d, want 9000 (first hunted event)", bob.HuntedFrame)
	}
	if bob.UnhuntedFrame != 9600 {
		t.Errorf("Bob UnhuntedFrame = %d, want 9600 (first recovery)", bob.UnhuntedFrame)
	}
	if !bob.Hunted {
		t.Errorf("Bob final Hunted = false, want true (last event was hunted)")
	}
}

func TestHuntedSummaryJSONOmitsWhenNever(t *testing.T) {
	stats := huntedTestStats(t)
	stats.HuntedEvents = nil

	replay := &Replay{
		Header: &header.GeneralsHeader{},
		Summary: []*object.PlayerSummary{
			{Name: "Alice", Side: "USA", Team: 1},
			{Name: "Bob", Side: "GLA", Team: 2},
		},
	}

	v2 := ConvertToEnhancedReplayV2(replay, stats, nil)
	out, err := json.Marshal(v2.Summary[0])
	if err != nil {
		t.Fatalf("marshal summary: %v", err)
	}
	for _, key := range []string{"huntedFrame", "unhuntedFrame", `"hunted"`} {
		if strings.Contains(string(out), key) {
			t.Errorf("summary JSON should omit %s when never hunted: %s", key, out)
		}
	}
}
