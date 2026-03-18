package zhreplay

import (
	"math"

	"github.com/bill-rich/cncstats/pkg/iniparse"
	"github.com/bill-rich/cncstats/pkg/statsfile"
	"github.com/bill-rich/cncstats/pkg/zhreplay/body"
	"github.com/bill-rich/cncstats/pkg/zhreplay/header"
	"github.com/bill-rich/cncstats/pkg/zhreplay/object"
)

const (
	EnhancedReplayVersionV2 = 2
)

// WinEstimation holds the estimated winner result and per-team breakdown.
type WinEstimation struct {
	Confidence float64              `json:"confidence"`
	Teams      map[int]*TeamFactors `json:"teams"`
}

// TeamFactors holds the scoring factors used to estimate a team's strength.
type TeamFactors struct {
	BuiltValue     int     `json:"builtValue"`
	LostValue      int     `json:"lostValue"`
	DestroyedValue int     `json:"destroyedValue"`
	CaptureGain    int     `json:"captureGain"`
	CaptureLoss    int     `json:"captureLoss"`
	NetAssets      int     `json:"netAssets"`
	Efficiency     float64 `json:"efficiency"`
	Score          float64 `json:"score"`
}

// EnhancedReplayV2 represents a replay with stats from the Generals JSON exporter
type EnhancedReplayV2 struct {
	Header        *header.GeneralsHeader `json:"header"`
	Version       int                    `json:"version"`
	WinMethod     string                 `json:"winMethod"`
	WinEstimation *WinEstimation         `json:"winEstimation,omitempty"`
	GameInfo      *GameInfoV2            `json:"gameInfo,omitempty"`
	Stats         *EnrichedStats         `json:"stats"`
	Body          []*body.BodyChunk      `json:"body"`
	Summary       []*PlayerSummaryV2     `json:"summary"`
	Offset        int                    `json:"offset"`
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
	v2.DetermineWinnersByDeathEvents(objectStore)

	return v2
}

// DetermineWinnersByDeathEvents uses the stats death events to determine winners.
// If the result is ambiguous (more than one winning team) or no death events exist,
// it falls back to estimateWinner before returning.
func (v2 *EnhancedReplayV2) DetermineWinnersByDeathEvents(objectStore *iniparse.ObjectStore) {
	if v2.Stats == nil {
		return
	}

	// Build a set of player indices that died
	deadPlayers := make(map[int]bool)
	for _, de := range v2.Stats.DeathEvents {
		deadPlayers[de.Player] = true
	}

	// If no death events, try estimated winner
	if len(deadPlayers) == 0 {
		v2.estimateWinner(objectStore)
		return
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

	// Count winning teams — if more than one, ambiguous; try estimated winner
	winningTeams := 0
	for _, alive := range teamAlive {
		if alive {
			winningTeams++
		}
	}
	if winningTeams != 1 {
		v2.estimateWinner(objectStore)
		return
	}

	// Exactly one winning team — apply death-based results
	for _, p := range v2.Summary {
		if p.Side == "Observer" {
			continue
		}
		p.Win = teamAlive[p.Team]
	}
	v2.WinMethod = "deathEvents"
}

// estimateWinner uses build/kill/capture economic data to estimate the winner.
// It computes a composite score per team from net assets and kill efficiency,
// then picks the highest-scoring team if confidence is above zero.
func (v2 *EnhancedReplayV2) estimateWinner(objectStore *iniparse.ObjectStore) {
	if v2.Stats == nil {
		return
	}

	// Build cost map from BuildEvents (object name → cost)
	objectCost := make(map[string]int)
	for _, ev := range v2.Stats.BuildEvents {
		objectCost[ev.Object] = ev.Cost
	}

	lookupCost := func(name string) int {
		if c, ok := objectCost[name]; ok {
			return c
		}
		if objectStore != nil {
			if obj := objectStore.GetObjectByName(name); obj != nil {
				return obj.Cost
			}
		}
		return 0
	}

	// Build player index → team map
	playerTeam := make(map[int]int)
	teams := make(map[int]bool)
	for _, p := range v2.Summary {
		if p.Side == "Observer" {
			continue
		}
		playerTeam[p.Index] = p.Team
		teams[p.Team] = true
	}

	// Need at least 2 teams to compare
	if len(teams) < 2 {
		return
	}

	// Initialize team factors
	factors := make(map[int]*TeamFactors)
	for team := range teams {
		factors[team] = &TeamFactors{}
	}

	// Accumulate built value
	for _, ev := range v2.Stats.BuildEvents {
		team, ok := playerTeam[ev.Player]
		if !ok {
			continue
		}
		factors[team].BuiltValue += ev.Cost
	}

	// Accumulate kill values
	for _, ev := range v2.Stats.KillEvents {
		cost := lookupCost(ev.Victim)

		if team, ok := playerTeam[ev.VictimPlayer]; ok {
			factors[team].LostValue += cost
		}
		if team, ok := playerTeam[ev.KillerPlayer]; ok {
			factors[team].DestroyedValue += cost
		}
	}

	// Accumulate capture values
	for _, ev := range v2.Stats.CaptureEvents {
		cost := lookupCost(ev.Object)

		if team, ok := playerTeam[ev.NewOwner]; ok {
			factors[team].CaptureGain += cost
		}
		if team, ok := playerTeam[ev.OldOwner]; ok {
			factors[team].CaptureLoss += cost
		}
	}

	// Compute per-team derived values
	for _, tf := range factors {
		tf.NetAssets = tf.BuiltValue - tf.LostValue + tf.CaptureGain - tf.CaptureLoss
		tf.Efficiency = float64(tf.DestroyedValue+1) / float64(tf.LostValue+1)
	}

	// Find min/max for normalization
	minNet, maxNet := math.MaxFloat64, -math.MaxFloat64
	minEff, maxEff := math.MaxFloat64, -math.MaxFloat64
	for _, tf := range factors {
		net := float64(tf.NetAssets)
		if net < minNet {
			minNet = net
		}
		if net > maxNet {
			maxNet = net
		}
		if tf.Efficiency < minEff {
			minEff = tf.Efficiency
		}
		if tf.Efficiency > maxEff {
			maxEff = tf.Efficiency
		}
	}

	netRange := maxNet - minNet
	effRange := maxEff - minEff

	// Compute composite score
	for _, tf := range factors {
		var normNet, normEff float64
		if netRange > 0 {
			normNet = (float64(tf.NetAssets) - minNet) / netRange
		} else {
			normNet = 0.5
		}
		if effRange > 0 {
			normEff = (tf.Efficiency - minEff) / effRange
		} else {
			normEff = 0.5
		}
		tf.Score = 0.6*normNet + 0.4*normEff
	}

	// Find highest-scoring team
	bestTeam := -1
	bestScore := -1.0
	secondScore := -1.0
	for team, tf := range factors {
		if tf.Score > bestScore {
			secondScore = bestScore
			bestScore = tf.Score
			bestTeam = team
		} else if tf.Score > secondScore {
			secondScore = tf.Score
		}
	}

	if bestTeam < 0 || bestScore <= 0 {
		return
	}

	// Compute confidence from margin
	var confidence float64
	if bestScore > 0 {
		margin := math.Abs(bestScore-secondScore) / bestScore
		confidence = math.Min(margin*2, 1.0)
	}

	if confidence <= 0 {
		return
	}

	// Apply wins
	for _, p := range v2.Summary {
		if p.Side == "Observer" {
			continue
		}
		p.Win = p.Team == bestTeam
	}
	v2.WinMethod = "estimatedWinner"
	v2.WinEstimation = &WinEstimation{
		Confidence: confidence,
		Teams:      factors,
	}
}
