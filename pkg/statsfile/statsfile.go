package statsfile

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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
	HuntedEvents        []HuntedEvent        `json:"huntedEvents"`
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

// HuntedEvent marks a player transitioning into or out of the hunted state:
// hunted means they can no longer rebuild (no dozer or worker alive and no
// structure that can produce one, i.e. command center or GLA supply stash).
// Becoming un-hunted is rare and means they regained a builder, e.g. by
// capturing an enemy command center or hijacking a dozer. Present for stats
// JSON version >= 3; nil for older uploads.
type HuntedEvent struct {
	Frame  uint `json:"frame"`
	Player int  `json:"player"`
	Hunted bool `json:"hunted"`
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

// Size returns the size in bytes of the stored stats file for the given seed.
// It returns an error wrapping os.ErrNotExist if no file is stored.
func Size(seed string) (int64, error) {
	info, err := os.Stat(StatsPath(seed))
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// Identity is the part of a stats payload that says which match it is,
// as opposed to how that match went. Two uploads of the same match from
// two different players agree on all of it; two genuinely different
// matches essentially never do.
//
// This exists because the storage key is the game seed, and on a LAN the
// seed is GetTickCount() on the host, i.e. milliseconds since that machine
// booted. That is not a unique identifier: 373 of the first 3879 matches
// stored here have a seed under one hour of uptime, where GetTickCount's
// ~15.6ms granularity leaves only ~230k distinct values. Colliding matches
// used to be arbitrated purely on file size, so the bigger one silently
// replaced the other.
type Identity struct {
	Map     string   // MapLeaf'd, so a per-machine path cannot matter
	Players []string // display names, sorted, so player order cannot matter
}

// MapLeaf reduces a map path to its final component, lowercased and without
// its extension.
//
// It has to exist because Game.Map is whatever path the reporting client had
// open, and that is per-machine: the same match uploaded by six players
// arrives as six different strings that differ only in the Windows account
// name.
//
//	c:\users\bill\documents\...\maps\amazonassault\amazonassault.map
//	c:\users\ktkel\documents\...\maps\amazonassault\amazonassault.map
//
// Comparing those raw made every upload after the first look like a seed
// collision, so five of every six players in a six-player game were turned
// away with a 409. Only stock maps, whose path carries no account name,
// survived the check.
//
// Backslashes are replaced by hand: filepath.Base uses the host separator,
// and the server runs on Linux while every one of these paths is a Windows
// one.
func MapLeaf(path string) string {
	p := strings.ReplaceAll(path, "\\", "/")
	p = strings.TrimSuffix(p, "/")
	leaf := filepath.Base(p)
	leaf = strings.TrimSuffix(leaf, filepath.Ext(leaf))
	return strings.ToLower(leaf)
}

// IdentityOf extracts the match identity from parsed stats.
func IdentityOf(stats *GameStats) Identity {
	id := Identity{Map: MapLeaf(stats.Game.Map)}
	for _, p := range stats.Players {
		id.Players = append(id.Players, p.DisplayName)
	}
	sort.Strings(id.Players)
	return id
}

// PlayerTypeComputer is the Player.Type value the stats exporter writes for
// an AI slot.
const PlayerTypeComputer = "Computer"

// IdentityOfHumans is IdentityOf with AI slots left out.
//
// Use it only when comparing against a roster from somewhere else, never for
// the upload guard: two uploads of one match both come from the live stats
// exporter and agree about the AI, so the guard should hold them to the
// stricter whole-roster comparison.
//
// It exists because a replay-derived roster may not list the AI at all.
// GenTool rewrites the headers of replays uploaded through it and drops zulu's
// tactical-AI slots, so radarvan (which parses replays) sees seven humans on a
// match where cncstats (which records the live game) sees seven humans and a
// Tactical AI. That is a difference in where the two rosters came from, not
// two different matches, and reporting it as a collision would be noise.
func IdentityOfHumans(stats *GameStats) Identity {
	id := Identity{Map: MapLeaf(stats.Game.Map)}
	for _, p := range stats.Players {
		if p.Type == PlayerTypeComputer {
			continue
		}
		id.Players = append(id.Players, p.DisplayName)
	}
	sort.Strings(id.Players)
	return id
}

// SameMatch reports whether two identities describe the same match.
//
// An empty map name or an empty roster means the payload did not carry
// enough to tell (an old or partial export), and we say "same" rather than
// block a legitimate upload on missing data: the size guard still applies,
// and refusing here would be a regression for anything that predates these
// fields.
func (a Identity) SameMatch(b Identity) bool {
	if a.Map == "" || b.Map == "" || len(a.Players) == 0 || len(b.Players) == 0 {
		return true
	}
	if a.Map != b.Map || len(a.Players) != len(b.Players) {
		return false
	}
	for i := range a.Players {
		if a.Players[i] != b.Players[i] {
			return false
		}
	}
	return true
}

// String renders an identity for a log line.
func (a Identity) String() string {
	return fmt.Sprintf("map=%q players=%v", a.Map, a.Players)
}

// ParseBytes decodes a gzip-compressed stats payload that has not been
// stored yet, so an upload can be inspected before it overwrites anything.
func ParseBytes(data []byte) (*GameStats, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	raw, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("read stats: %w", err)
	}

	var stats GameStats
	if err := json.Unmarshal(raw, &stats); err != nil {
		return nil, fmt.Errorf("parse stats JSON: %w", err)
	}
	return &stats, nil
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
