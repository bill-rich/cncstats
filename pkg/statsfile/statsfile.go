package statsfile

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	// CurrentStatsVersion is the stats JSON schema version this server is
	// written against. The Generals stats exporter stamps "version" at the
	// root of every upload (see StatsExporter.cpp in GeneralsGameCode). Bump
	// this in lockstep whenever the exporter's schema changes and the server
	// is updated to match.
	CurrentStatsVersion = 2

	// MinStatsVersion is the oldest schema version this server can still parse
	// without dropping data. Files at or above this version are read normally;
	// older ones are handled best-effort and flagged by CheckVersion.
	MinStatsVersion = 1
)

// VersionStatus describes how a loaded stats file's schema version relates to
// the range this server understands.
type VersionStatus int

const (
	// VersionOK means the version is within [MinStatsVersion, CurrentStatsVersion].
	VersionOK VersionStatus = iota
	// VersionMissing means no version field was present (0): a pre-versioning
	// export or a corrupt/truncated file.
	VersionMissing
	// VersionTooOld means the version is below MinStatsVersion.
	VersionTooOld
	// VersionNewer means the version is above CurrentStatsVersion: the client
	// is ahead of this server, which may not understand newer fields.
	VersionNewer
)

func (s VersionStatus) String() string {
	switch s {
	case VersionOK:
		return "ok"
	case VersionMissing:
		return "missing"
	case VersionTooOld:
		return "too-old"
	case VersionNewer:
		return "newer-than-server"
	default:
		return fmt.Sprintf("unknown(%d)", int(s))
	}
}

// CheckVersion reports whether the stats file's schema version is one this
// server fully understands. It never mutates the stats; callers decide how to
// react (the data is always parsed best-effort regardless).
func (g *GameStats) CheckVersion() VersionStatus {
	switch {
	case g.Version == 0:
		return VersionMissing
	case g.Version < MinStatsVersion:
		return VersionTooOld
	case g.Version > CurrentStatsVersion:
		return VersionNewer
	default:
		return VersionOK
	}
}

// GameStats represents the JSON structure from the Generals stats exporter
type GameStats struct {
	Version int      `json:"version"`
	Game    GameInfo `json:"game"`
	Players []Player `json:"players"`

	BuildEvents         []BuildEvent         `json:"buildEvents"`
	KillEvents          []KillEvent          `json:"killEvents"`
	CaptureEvents       []CaptureEvent       `json:"captureEvents"`
	EnergyEvents        []EnergyEvent        `json:"energyEvents"`
	RankEvents          []RankEvent          `json:"rankEvents"`
	SkillPointsEvents   []SkillPointsEvent   `json:"skillPointsEvents"`
	SciencePointsEvents []SciencePointsEvent `json:"sciencePointsEvents"`
	RadarEvents         []RadarEvent         `json:"radarEvents"`
	DeathEvents         []DeathEvent         `json:"deathEvents"`
	BattlePlanEvents    []BattlePlanEvent    `json:"battlePlanEvents"`
	TimeSeries          TimeSeries           `json:"timeSeries"`
}

type GameInfo struct {
	Map              string `json:"map"`
	Mode             string `json:"mode"`
	FrameCount       uint   `json:"frameCount"`
	Seed             uint   `json:"seed"`
	ReplayFile       string `json:"replayFile"`
	PlayerCount      int    `json:"playerCount"`
	SnapshotInterval int    `json:"snapshotInterval"`
}

type Player struct {
	Index       int      `json:"index"`
	DisplayName string   `json:"displayName"`
	Faction     string   `json:"faction,omitempty"`
	Side        string   `json:"side"`
	BaseSide    string   `json:"baseSide,omitempty"`
	Type        string   `json:"type"`
	Color       string   `json:"color"`
	Money       uint     `json:"money"`
	MoneyEarned int      `json:"moneyEarned"`
	MoneySpent  int      `json:"moneySpent"`
	Score       int      `json:"score"`
	// IncomeBySource breaks moneyEarned down by source (supply, hacker,
	// blackMarket, supplyDrop, oilDerrick, bounty, salvage, crate, theft,
	// other). Present for stats JSON version >= 2; nil for older uploads.
	IncomeBySource map[string]int `json:"incomeBySource,omitempty"`
	Academy        *Academy       `json:"academy,omitempty"`
}

type Academy struct {
	SupplyCentersBuilt               uint `json:"supplyCentersBuilt"`
	PeonsBuilt                       uint `json:"peonsBuilt"`
	StructuresCaptured               uint `json:"structuresCaptured"`
	GeneralsPointsSpent              uint `json:"generalsPointsSpent"`
	SpecialPowersUsed                uint `json:"specialPowersUsed"`
	StructuresGarrisoned             uint `json:"structuresGarrisoned"`
	UpgradesPurchased                uint `json:"upgradesPurchased"`
	GatherersBuilt                   uint `json:"gatherersBuilt"`
	HeroesBuilt                      uint `json:"heroesBuilt"`
	ControlGroupsUsed                uint `json:"controlGroupsUsed"`
	SecondaryIncomeUnitsBuilt        uint `json:"secondaryIncomeUnitsBuilt"`
	ClearedGarrisonedBuildings       uint `json:"clearedGarrisonedBuildings"`
	SalvageCollected                 uint `json:"salvageCollected"`
	GuardAbilityUsedCount            uint `json:"guardAbilityUsedCount"`
	DoubleClickAttackMoveOrdersGiven uint `json:"doubleClickAttackMoveOrdersGiven"`
	MinesCleared                     uint `json:"minesCleared"`
	VehiclesDisguised                uint `json:"vehiclesDisguised"`
	FirestormsCreated                uint `json:"firestormsCreated"`
}

type BuildEvent struct {
	Frame     uint    `json:"frame"`
	Player    int     `json:"player"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Cost      int     `json:"cost"`
	BuildTime int     `json:"buildTime"`
	Object    string  `json:"object"`
	Producer  string  `json:"producer"`
}

type KillEvent struct {
	Frame        uint    `json:"frame"`
	KillerPlayer int     `json:"killerPlayer"`
	VictimPlayer int     `json:"victimPlayer"`
	X            float64 `json:"x"`
	Y            float64 `json:"y"`
	Killer       string  `json:"killer"`
	Victim       string  `json:"victim"`
	DamageType   string  `json:"damageType"`
}

type CaptureEvent struct {
	Frame    uint    `json:"frame"`
	NewOwner int     `json:"newOwner"`
	OldOwner int     `json:"oldOwner"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Object   string  `json:"object"`
}

type EnergyEvent struct {
	Frame       uint `json:"frame"`
	Player      int  `json:"player"`
	Production  int  `json:"production"`
	Consumption int  `json:"consumption"`
}

type RankEvent struct {
	Frame     uint `json:"frame"`
	Player    int  `json:"player"`
	RankLevel int  `json:"rankLevel"`
}

type SkillPointsEvent struct {
	Frame       uint `json:"frame"`
	Player      int  `json:"player"`
	SkillPoints int  `json:"skillPoints"`
}

type SciencePointsEvent struct {
	Frame                 uint `json:"frame"`
	Player                int  `json:"player"`
	SciencePurchasePoints int  `json:"sciencePurchasePoints"`
}

type RadarEvent struct {
	Frame    uint `json:"frame"`
	Player   int  `json:"player"`
	HasRadar bool `json:"hasRadar"`
}

type DeathEvent struct {
	Frame  uint `json:"frame"`
	Player int  `json:"player"`
}

type BattlePlanEvent struct {
	Frame            uint `json:"frame"`
	Player           int  `json:"player"`
	Bombardment      int  `json:"bombardment"`
	HoldTheLine      int  `json:"holdTheLine"`
	SearchAndDestroy int  `json:"searchAndDestroy"`
}

type TimeSeries struct {
	Players []TimeSeriesPlayer `json:"players"`
}

type TimeSeriesPlayer struct {
	Index       int    `json:"index"`
	Money       []uint `json:"money"`
	MoneyEarned []int  `json:"moneyEarned"`
	MoneySpent  []int  `json:"moneySpent"`
	// IncomeBySource holds one cumulative-income series per source, keyed by
	// the same source names as Player.IncomeBySource. Present for stats JSON
	// version >= 2; nil for older uploads.
	IncomeBySource map[string][]int `json:"incomeBySource,omitempty"`
}

// StatsDir is the directory where stats files are stored.
// Set via STATS_DIR env var or defaults to ./stats/
var StatsDir = "./stats"

func init() {
	if dir := os.Getenv("STATS_DIR"); dir != "" {
		StatsDir = dir
	}
}

// StatsPath returns the file path for a given seed
func StatsPath(seed string) string {
	return filepath.Join(StatsDir, seed+".json.gz")
}

// Store saves gzip-compressed stats data for the given seed
func Store(seed string, data []byte) error {
	if err := os.MkdirAll(StatsDir, 0755); err != nil {
		return fmt.Errorf("create stats dir: %w", err)
	}
	return os.WriteFile(StatsPath(seed), data, 0644)
}

// Exists checks if stats data exists for the given seed
func Exists(seed string) bool {
	_, err := os.Stat(StatsPath(seed))
	return err == nil
}

// Load reads and decompresses stats data for the given seed
func Load(seed string) (*GameStats, error) {
	f, err := os.Open(StatsPath(seed))
	if err != nil {
		return nil, fmt.Errorf("open stats file: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	data, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("read stats: %w", err)
	}

	var stats GameStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return nil, fmt.Errorf("parse stats JSON: %w", err)
	}

	return &stats, nil
}
