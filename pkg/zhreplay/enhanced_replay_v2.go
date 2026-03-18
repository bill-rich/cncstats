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
	Confidence   float64              `json:"confidence"`
	AgreeCount   int                  `json:"agreeCount"`   // how many factors pick the winner
	TotalFactors int                  `json:"totalFactors"` // total active factors (3 or 4)
	Teams        map[int]*TeamFactors `json:"teams"`
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
	RecentIncome   int     `json:"recentIncome"`
	PlayerScore    int     `json:"playerScore"`
	MoneyEarned    int     `json:"moneyEarned"`
	PlayersAlive   int     `json:"playersAlive"`
	PlayersDead    int     `json:"playersDead"`
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
	PlayerIDOffset int                   `json:"offset"`
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
	BuildEvents         []EnrichedBuildEvent           `json:"buildEvents"`
	KillEvents          []EnrichedKillEvent            `json:"killEvents"`
	CaptureEvents       []EnrichedCaptureEvent         `json:"captureEvents"`
	EnergyEvents        []statsfile.EnergyEvent        `json:"energyEvents"`
	RankEvents          []statsfile.RankEvent          `json:"rankEvents"`
	SkillPointsEvents   []statsfile.SkillPointsEvent   `json:"skillPointsEvents"`
	SciencePointsEvents []statsfile.SciencePointsEvent `json:"sciencePointsEvents"`
	RadarEvents         []statsfile.RadarEvent         `json:"radarEvents"`
	DeathEvents         []statsfile.DeathEvent         `json:"deathEvents"`
	BattlePlanEvents    []statsfile.BattlePlanEvent    `json:"battlePlanEvents"`
	TimeSeries          statsfile.TimeSeries           `json:"timeSeries"`
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
		PlayerIDOffset: replay.PlayerIDOffset,
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

const framesPerMinute = 1800 // 30 fps × 60 sec

// estimateWinner uses five independent factors to estimate the winner via
// majority vote, with confidence derived from factor agreement and the net
// asset ratio. Calibrated against 394 complete games at 75-80% completion.
//
// Factors (ranked by individual accuracy at 75-80%):
//  1. netAssets       — 76-78% accurate; best late-game single predictor
//  2. moneyEarned     — 74% accurate; cumulative economic strength
//  3. playerScore     — 74% accurate; in-game score from stats
//  4. recentIncome    — 71% accurate; last-minute money earned
//  5. playersAlive    — 71-73% accurate; strongest amplifier when paired
//
// When all 5 agree at 80%, accuracy is 99% (96/97 games).
// The pair netAssets+playersAlive alone reaches 92%.
//
// Confidence formula:
//   base = sigmoid(2.75 * ln(clampedAssetRatio))
//   confidence = base * (0.5 + 0.5 * agreeCount/totalFactors)
// Asset ratio is clamped to [1, 4] because ratios above 4-5x actually
// indicate the leading team has destroyed everything (low net assets)
// and accuracy drops to ~45%.
func (v2 *EnhancedReplayV2) estimateWinner(objectStore *iniparse.ObjectStore) {
	if v2.Stats == nil {
		return
	}

	// Build cost map from BuildEvents
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
	if len(teams) < 2 {
		return
	}

	// Find the last frame in the data
	var lastFrame uint
	for _, ev := range v2.Stats.BuildEvents {
		if ev.Frame > lastFrame {
			lastFrame = ev.Frame
		}
	}
	for _, ev := range v2.Stats.KillEvents {
		if ev.Frame > lastFrame {
			lastFrame = ev.Frame
		}
	}
	const minGameLength = framesPerMinute * 10 // 10 minutes minimum
	if lastFrame < minGameLength {
		return
	}
	minuteAgoFrame := lastFrame - framesPerMinute

	// Initialize team factors
	factors := make(map[int]*TeamFactors)
	for team := range teams {
		factors[team] = &TeamFactors{}
	}

	// --- Cumulative economic factors ---
	for _, ev := range v2.Stats.BuildEvents {
		if team, ok := playerTeam[ev.Player]; ok {
			factors[team].BuiltValue += ev.Cost
		}
	}
	for _, ev := range v2.Stats.KillEvents {
		cost := lookupCost(ev.Victim)
		if team, ok := playerTeam[ev.VictimPlayer]; ok {
			factors[team].LostValue += cost
		}
		if team, ok := playerTeam[ev.KillerPlayer]; ok {
			factors[team].DestroyedValue += cost
		}
	}
	for _, ev := range v2.Stats.CaptureEvents {
		cost := lookupCost(ev.Object)
		if team, ok := playerTeam[ev.NewOwner]; ok {
			factors[team].CaptureGain += cost
		}
		if team, ok := playerTeam[ev.OldOwner]; ok {
			factors[team].CaptureLoss += cost
		}
	}
	for _, tf := range factors {
		tf.NetAssets = tf.BuiltValue - tf.LostValue + tf.CaptureGain - tf.CaptureLoss
		tf.Efficiency = float64(tf.DestroyedValue+1) / float64(tf.LostValue+1)
		tf.Score = float64(tf.NetAssets) * tf.Efficiency
	}

	// --- Player score and money earned from summary ---
	for _, p := range v2.Summary {
		if p.Side == "Observer" {
			continue
		}
		factors[p.Team].PlayerScore += p.Score
		factors[p.Team].MoneyEarned += p.MoneyEarned
	}

	// --- Recent income (last minute from time series) ---
	if v2.GameInfo != nil {
		snapInterval := v2.GameInfo.SnapshotInterval
		if snapInterval <= 0 {
			snapInterval = 30
		}
		cutoffIdx := int(lastFrame) / snapInterval
		minuteAgoIdx := int(minuteAgoFrame) / snapInterval

		for _, tsp := range v2.Stats.TimeSeries.Players {
			team, ok := playerTeam[tsp.Index]
			if !ok {
				continue
			}
			earned := tsp.MoneyEarned
			ci := cutoffIdx
			if ci >= len(earned) {
				ci = len(earned) - 1
			}
			mi := minuteAgoIdx
			if mi >= len(earned) {
				mi = len(earned) - 1
			}
			if mi < 0 {
				mi = 0
			}
			if ci >= 0 && ci < len(earned) {
				factors[team].RecentIncome += earned[ci] - earned[mi]
			}
		}
	}

	// --- Players alive/dead per team ---
	deadPlayers := make(map[int]bool)
	for _, de := range v2.Stats.DeathEvents {
		deadPlayers[de.Player] = true
	}
	teamSize := make(map[int]int)
	for _, p := range v2.Summary {
		if p.Side == "Observer" {
			continue
		}
		teamSize[p.Team]++
		if deadPlayers[p.Index] {
			factors[p.Team].PlayersDead++
		}
	}
	for team, tf := range factors {
		tf.PlayersAlive = teamSize[team] - tf.PlayersDead
	}

	// --- Determine leader for each of 5 factors ---
	netAssetsLeader := teamWithHighest(factors, func(tf *TeamFactors) float64 { return float64(tf.NetAssets) })
	moneyEarnedLeader := teamWithHighest(factors, func(tf *TeamFactors) float64 { return float64(tf.MoneyEarned) })
	playerScoreLeader := teamWithHighest(factors, func(tf *TeamFactors) float64 { return float64(tf.PlayerScore) })
	incomeLeader := teamWithHighest(factors, func(tf *TeamFactors) float64 { return float64(tf.RecentIncome) })
	survivalLeader := teamWithHighest(factors, func(tf *TeamFactors) float64 { return float64(tf.PlayersAlive) })

	// --- Majority vote across all active factors ---
	totalFactors := 0
	votes := map[int]int{}

	addVote := func(leader int) {
		if leader >= 0 {
			votes[leader]++
			totalFactors++
		}
	}

	addVote(netAssetsLeader)
	addVote(moneyEarnedLeader)
	addVote(playerScoreLeader)
	addVote(incomeLeader)

	// Only count survival if there's a meaningful difference between teams.
	if survivalLeader >= 0 && len(deadPlayers) > 0 {
		leaderAlive := factors[survivalLeader].PlayersAlive
		hasDifference := false
		for team, tf := range factors {
			if team != survivalLeader && tf.PlayersAlive != leaderAlive {
				hasDifference = true
				break
			}
		}
		if hasDifference {
			addVote(survivalLeader)
		}
	}

	bestTeam := -1
	bestVotes := 0
	for team, v := range votes {
		if v > bestVotes {
			bestVotes = v
			bestTeam = team
		}
	}
	if bestTeam < 0 || totalFactors == 0 {
		return
	}

	// --- Confidence ---
	//
	// Calibrated against 293 human-only games and 72 AI games at 80% completion.
	//
	// Three components multiply together:
	//
	// 1. Agreement drives the base. For human games, even moderate agreement
	//    with any asset lead is ~95% correct. Agreement fraction maps directly:
	//      unanimous (5/5) → 0.98, majority (3/5) → 0.80, bare split (2/4) → 0.55
	//    Formula: agreementBase = 0.5 + 0.48 * (votes/total)
	//    Range: 0.50 (worst split) to 0.98 (unanimous)
	//
	// 2. Asset ratio provides a secondary boost via sigmoid.
	//    Clamped to [1, 4] because ratios above 5x actually reduce accuracy
	//    (the winner has destroyed everything and has LOW net assets).
	//      1.0x → 0.60, 1.25x → 0.78, 1.5x → 0.87, 2.0x → 0.95, 4.0x → 0.99
	//    Formula: ratioBoost = sigmoid(3.5 * ln(clampedRatio)) * 0.4 + 0.6
	//    Range: 0.60 (no ratio signal) to ~0.99 (dominant lead)
	//
	// 3. AI penalty. AI players have fundamentally different economics.
	//      0 AI → 1.0, 1 AI → 0.70, 2+ AI → 0.05
	//    Observed accuracy: human-only 95%, 1 AI 67%, 2+ AI 2%.

	winnerNet := factors[bestTeam].NetAssets
	if winnerNet <= 0 {
		return
	}

	var bestOpponentNet int
	for team, tf := range factors {
		if team != bestTeam && tf.NetAssets > bestOpponentNet {
			bestOpponentNet = tf.NetAssets
		}
	}

	var assetRatio float64
	if bestOpponentNet > 0 {
		assetRatio = float64(winnerNet) / float64(bestOpponentNet)
	} else {
		assetRatio = 4.0
	}
	if assetRatio > 4.0 {
		assetRatio = 4.0
	}
	if assetRatio < 1.0 {
		assetRatio = 1.0
	}

	// Component 1: agreement base
	agreementFraction := float64(bestVotes) / float64(totalFactors)
	agreementBase := 0.50 + 0.48*agreementFraction

	// Component 2: asset ratio boost
	const k = 3.5
	logRatio := math.Log(assetRatio)
	rawSigmoid := 2.0/(1.0+math.Exp(-k*logRatio)) - 1.0
	ratioBoost := 0.60 + 0.40*rawSigmoid

	// Component 3: AI penalty
	aiCount := 0
	for _, p := range v2.Summary {
		if p.PlayerType == "Computer" || p.PlayerType == "C" {
			aiCount++
		}
	}
	// Also check the header metadata — AI players from replays have Type "C"
	// and may not get matched during the stats merge (empty DisplayName).
	if aiCount == 0 && v2.Header != nil {
		for _, hp := range v2.Header.Metadata.Players {
			if hp.Type == "C" {
				aiCount++
			}
		}
	}
	aiMultiplier := 1.0
	switch {
	case aiCount >= 2:
		aiMultiplier = 0.05
	case aiCount == 1:
		aiMultiplier = 0.70
	}

	confidence := agreementBase * ratioBoost * aiMultiplier

	if confidence <= 0 {
		return
	}
	if confidence > 0.99 {
		confidence = 0.99
	}

	for _, p := range v2.Summary {
		if p.Side == "Observer" {
			continue
		}
		p.Win = p.Team == bestTeam
	}
	v2.WinMethod = "estimatedWinner"
	v2.WinEstimation = &WinEstimation{
		Confidence:   confidence,
		AgreeCount:   bestVotes,
		TotalFactors: totalFactors,
		Teams:        factors,
	}
}

// teamWithHighest returns the team ID with the highest value for the given
// metric, or -1 if no team has a positive value.
func teamWithHighest(factors map[int]*TeamFactors, metric func(*TeamFactors) float64) int {
	best := -1
	bestVal := 0.0
	for team, tf := range factors {
		v := metric(tf)
		if v > bestVal {
			bestVal = v
			best = team
		}
	}
	return best
}
