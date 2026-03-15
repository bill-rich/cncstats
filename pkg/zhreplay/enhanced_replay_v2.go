package zhreplay

import (
	"github.com/bill-rich/cncstats/pkg/statsfile"
	"github.com/bill-rich/cncstats/pkg/zhreplay/body"
	"github.com/bill-rich/cncstats/pkg/zhreplay/header"
	"github.com/bill-rich/cncstats/pkg/zhreplay/object"
)

const (
	EnhancedReplayVersionV2 = 2
)

// EnhancedReplayV2 represents a replay with stats from the Generals JSON exporter
type EnhancedReplayV2 struct {
	Header  *header.GeneralsHeader     `json:"Header"`
	Version int                        `json:"Version"`
	Stats   *statsfile.GameStats       `json:"Stats"`
	Body    []*body.BodyChunk          `json:"Body"`
	Summary []*PlayerSummaryV2         `json:"Summary"`
	Offset  int                        `json:"Offset"`
}

// PlayerSummaryV2 keeps per-type breakdowns from replay parsing but removes
// fields that are now in the Stats section
type PlayerSummaryV2 struct {
	Name           string                              `json:"Name"`
	Side           string                              `json:"Side"`
	Team           int                                 `json:"Team"`
	Win            bool                                `json:"Win"`
	UnitsCreated   map[string]*object.ObjectSummary    `json:"UnitsCreated"`
	BuildingsBuilt map[string]*object.ObjectSummary    `json:"BuildingsBuilt"`
	UpgradesBuilt  map[string]*object.ObjectSummary    `json:"UpgradesBuilt"`
	PowersUsed     map[string]int                      `json:"PowersUsed"`
}

// ConvertToEnhancedReplayV2 creates a v2 enhanced replay using the stats JSON file
func ConvertToEnhancedReplayV2(replay *Replay, stats *statsfile.GameStats) *EnhancedReplayV2 {
	v2 := &EnhancedReplayV2{
		Header:  replay.Header,
		Version: EnhancedReplayVersionV2,
		Stats:   stats,
		Body:    replay.Body,
		Offset:  replay.Offset,
		Summary: make([]*PlayerSummaryV2, len(replay.Summary)),
	}

	for i, ps := range replay.Summary {
		v2.Summary[i] = &PlayerSummaryV2{
			Name:           ps.Name,
			Side:           ps.Side,
			Team:           ps.Team,
			Win:            ps.Win,
			UnitsCreated:   ps.UnitsCreated,
			BuildingsBuilt: ps.BuildingsBuilt,
			UpgradesBuilt:  ps.UpgradesBuilt,
			PowersUsed:     ps.PowersUsed,
		}
	}

	// Determine winners using death events from stats
	v2.DetermineWinnersByDeathEvents()

	return v2
}

// DetermineWinnersByDeathEvents uses the stats death events to determine winners
func (v2 *EnhancedReplayV2) DetermineWinnersByDeathEvents() {
	if v2.Stats == nil {
		return
	}

	// Build a set of player indices that died
	deadPlayers := make(map[int]bool)
	for _, de := range v2.Stats.DeathEvents {
		deadPlayers[de.Player] = true
	}

	// If no death events, can't determine — leave default
	if len(deadPlayers) == 0 {
		return
	}

	// Reset wins
	for _, p := range v2.Summary {
		p.Win = false
	}

	// Build player index → team mapping from stats players
	playerTeam := make(map[int]int)
	for _, sp := range v2.Stats.Players {
		// Find matching summary player by display name
		for _, p := range v2.Summary {
			if p.Team > 0 {
				playerTeam[sp.Index] = p.Team
			}
		}
	}

	// Teams with all players dead lose; teams with at least one alive win
	teamAlive := make(map[int]bool)
	for _, p := range v2.Summary {
		if p.Side == "Observer" {
			continue
		}
		// Find the stats player index for this summary player
		for _, sp := range v2.Stats.Players {
			if sp.DisplayName == p.Name || matchPlayerBySide(sp, p) {
				if !deadPlayers[sp.Index] {
					teamAlive[p.Team] = true
				}
				break
			}
		}
	}

	// Mark winners
	for _, p := range v2.Summary {
		if p.Side == "Observer" {
			continue
		}
		if teamAlive[p.Team] {
			p.Win = true
		}
	}
}

// matchPlayerBySide is a fallback matcher when display names don't match
func matchPlayerBySide(sp statsfile.Player, ps *PlayerSummaryV2) bool {
	return sp.Side == ps.Side && sp.Index > 0
}
