package zhreplay

import (
	"github.com/bill-rich/cncstats/pkg/iniparse"
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
	Header    *header.GeneralsHeader `json:"header"`
	Version   int                    `json:"version"`
	WinMethod string                 `json:"winMethod"`
	GameInfo  *GameInfoV2            `json:"gameInfo,omitempty"`
	Stats     *EnrichedStats         `json:"stats"`
	Body      []*body.BodyChunk      `json:"body"`
	Summary   []*PlayerSummaryV2     `json:"summary"`
	Offset    int                    `json:"offset"`
}

// GameInfoV2 holds non-duplicate game metadata from the stats file.
type GameInfoV2 struct {
	Mode             string `json:"mode"`
	FrameCount       uint   `json:"frameCount"`
	PlayerCount      int    `json:"playerCount"`
	SnapshotInterval int    `json:"snapshotInterval"`
}

// PlayerSummaryV2 keeps per-type breakdowns from replay parsing, enriched with
// stats player data (index, economy, faction, etc.)
type PlayerSummaryV2 struct {
	Name           string                           `json:"name"`
	Side           string                           `json:"side"`
	Team           int                              `json:"team"`
	Win            bool                             `json:"win"`
	Index          int                              `json:"index"`
	PlayerType     string                           `json:"playerType"`
	Color          string                           `json:"color"`
	Faction        string                           `json:"faction"`
	BaseSide       string                           `json:"baseSide"`
	Money          uint                             `json:"money"`
	MoneyEarned    int                              `json:"moneyEarned"`
	MoneySpent     int                              `json:"moneySpent"`
	Score          int                              `json:"score"`
	Academy        *statsfile.Academy               `json:"academy,omitempty"`
	UnitsCreated   map[string]*object.ObjectSummary `json:"unitsCreated"`
	BuildingsBuilt map[string]*object.ObjectSummary `json:"buildingsBuilt"`
	UpgradesBuilt  map[string]*object.ObjectSummary `json:"upgradesBuilt"`
	PowersUsed     map[string]int                   `json:"powersUsed"`
}

// EnrichedBuildEvent embeds a BuildEvent and adds object type classification.
type EnrichedBuildEvent struct {
	statsfile.BuildEvent
	ObjectType   string `json:"objectType,omitempty"`
	ProducerType string `json:"producerType,omitempty"`
}

// EnrichedKillEvent embeds a KillEvent and adds object type classification.
type EnrichedKillEvent struct {
	statsfile.KillEvent
	KillerType string `json:"killerType,omitempty"`
	VictimType string `json:"victimType,omitempty"`
}

// EnrichedCaptureEvent embeds a CaptureEvent and adds object type classification.
type EnrichedCaptureEvent struct {
	statsfile.CaptureEvent
	ObjectType string `json:"objectType,omitempty"`
}

// EnrichedStats holds enriched events and time series from the stats file.
type EnrichedStats struct {
	BuildEvents         []EnrichedBuildEvent         `json:"buildEvents"`
	KillEvents          []EnrichedKillEvent          `json:"killEvents"`
	CaptureEvents       []EnrichedCaptureEvent       `json:"captureEvents"`
	EnergyEvents        []statsfile.EnergyEvent      `json:"energyEvents"`
	RankEvents          []statsfile.RankEvent         `json:"rankEvents"`
	SkillPointsEvents   []statsfile.SkillPointsEvent `json:"skillPointsEvents"`
	SciencePointsEvents []statsfile.SciencePointsEvent `json:"sciencePointsEvents"`
	RadarEvents         []statsfile.RadarEvent       `json:"radarEvents"`
	DeathEvents         []statsfile.DeathEvent       `json:"deathEvents"`
	BattlePlanEvents    []statsfile.BattlePlanEvent  `json:"battlePlanEvents"`
	TimeSeries          statsfile.TimeSeries         `json:"timeSeries"`
}

// lookupObjectType returns the ObjectType string for a given object name, or empty string.
func lookupObjectType(objectStore *iniparse.ObjectStore, name string) string {
	if objectStore == nil {
		return ""
	}
	obj := objectStore.GetObjectByName(name)
	if obj == nil {
		return ""
	}
	return string(obj.Type)
}

// enrichStats builds an EnrichedStats from a GameStats, looking up object types.
func enrichStats(stats *statsfile.GameStats, objectStore *iniparse.ObjectStore) *EnrichedStats {
	es := &EnrichedStats{
		EnergyEvents:        stats.EnergyEvents,
		RankEvents:          stats.RankEvents,
		SkillPointsEvents:   stats.SkillPointsEvents,
		SciencePointsEvents: stats.SciencePointsEvents,
		RadarEvents:         stats.RadarEvents,
		DeathEvents:         stats.DeathEvents,
		BattlePlanEvents:    stats.BattlePlanEvents,
		TimeSeries:          stats.TimeSeries,
	}

	es.BuildEvents = make([]EnrichedBuildEvent, len(stats.BuildEvents))
	for i, ev := range stats.BuildEvents {
		es.BuildEvents[i] = EnrichedBuildEvent{
			BuildEvent:   ev,
			ObjectType:   lookupObjectType(objectStore, ev.Object),
			ProducerType: lookupObjectType(objectStore, ev.Producer),
		}
	}

	es.KillEvents = make([]EnrichedKillEvent, len(stats.KillEvents))
	for i, ev := range stats.KillEvents {
		es.KillEvents[i] = EnrichedKillEvent{
			KillEvent:  ev,
			KillerType: lookupObjectType(objectStore, ev.Killer),
			VictimType: lookupObjectType(objectStore, ev.Victim),
		}
	}

	es.CaptureEvents = make([]EnrichedCaptureEvent, len(stats.CaptureEvents))
	for i, ev := range stats.CaptureEvents {
		es.CaptureEvents[i] = EnrichedCaptureEvent{
			CaptureEvent: ev,
			ObjectType:   lookupObjectType(objectStore, ev.Object),
		}
	}

	return es
}

// ConvertToEnhancedReplayV2 creates a v2 enhanced replay using the stats JSON file.
// If objectStore is non-nil, events are enriched with object type classification.
func ConvertToEnhancedReplayV2(replay *Replay, stats *statsfile.GameStats, objectStore *iniparse.ObjectStore) *EnhancedReplayV2 {
	v2 := &EnhancedReplayV2{
		Header:    replay.Header,
		Version:   EnhancedReplayVersionV2,
		WinMethod: replay.WinMethod,
		GameInfo: &GameInfoV2{
			Mode:             stats.Game.Mode,
			FrameCount:       stats.Game.FrameCount,
			PlayerCount:      stats.Game.PlayerCount,
			SnapshotInterval: stats.Game.SnapshotInterval,
		},
		Stats:   enrichStats(stats, objectStore),
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

	// Merge stats player data into summary entries
	for _, sp := range stats.Players {
		for _, p := range v2.Summary {
			if sp.DisplayName == p.Name || sp.Side == p.Side {
				p.Index = sp.Index
				p.PlayerType = sp.Type
				p.Color = sp.Color
				p.Faction = sp.Faction
				p.BaseSide = sp.BaseSide
				p.Money = sp.Money
				p.MoneyEarned = sp.MoneyEarned
				p.MoneySpent = sp.MoneySpent
				p.Score = sp.Score
				p.Academy = sp.Academy
				break
			}
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

	// Teams with at least one alive player win
	teamAlive := make(map[int]bool)
	for _, p := range v2.Summary {
		if p.Side == "Observer" {
			continue
		}
		if !deadPlayers[p.Index] {
			teamAlive[p.Team] = true
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
	v2.WinMethod = "deathEvents"
}
