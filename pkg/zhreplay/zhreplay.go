package zhreplay

import (
	"context"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/bill-rich/cncstats/pkg/bitparse"
	"github.com/bill-rich/cncstats/pkg/iniparse"
	"github.com/bill-rich/cncstats/pkg/zhreplay/body"
	"github.com/bill-rich/cncstats/pkg/zhreplay/header"
	"github.com/bill-rich/cncstats/pkg/zhreplay/object"
)

type Replay struct {
	Header  *header.GeneralsHeader
	Body    []*body.BodyChunk
	Summary []*object.PlayerSummary
	Offset  int
}

type ReplayEasyUnmarshall struct {
	Header  *header.GeneralsHeader
	Body    []*body.BodyChunkEasyUnmarshall
	Summary []*object.PlayerSummary
	Offset  int
}

func NewReplay(bp *bitparse.BitParser) *Replay {
	replay := &Replay{
		Offset: 2,
	}
	replay.Header = header.NewHeader(bp)
	replay.CreatePlayerList()
	replay.Body = body.ParseBody(bp, replay.Summary, bp.ObjectStore, bp.PowerStore, bp.UpgradeStore)
	replay.AdjustOffset()
	replay.AddUserNames()
	replay.GenerateData()
	return replay
}

func (r *Replay) AddUserNames() {
	for _, chunk := range r.Body {
		if chunk.PlayerID >= r.Offset && chunk.PlayerID-r.Offset < len(r.Summary) {
			chunk.PlayerName = r.Summary[chunk.PlayerID-r.Offset].Name
		}
	}
}

func (r *Replay) AdjustOffset() {
	lowest := 1000
	for _, chunk := range r.Body {
		if chunk.PlayerID < lowest {
			lowest = chunk.PlayerID
		}
	}
	r.Offset = lowest
}

func (r *Replay) CreatePlayerList() {
	for _, playerMd := range r.Header.Metadata.Players {
		team, _ := strconv.Atoi(playerMd.Team)
		player := &object.PlayerSummary{
			Name:           playerMd.Name,
			Team:           team + 1,
			Win:            true,
			BuildingsBuilt: map[string]*object.ObjectSummary{},
			UnitsCreated:   map[string]*object.ObjectSummary{},
			UpgradesBuilt:  map[string]*object.ObjectSummary{},
			PowersUsed:     map[string]int{},
		}
		r.Summary = append(r.Summary, player)
	}
}

var ConstructorMap = map[string]string{
	"GLAInfantryWorker":        "GLA",
	"Slth_GLAInfantryWorker":   "GLA Stealth",
	"Chem_GLAInfantryWorker":   "GLA Toxin",
	"Demo_GLAInfantryWorker":   "GLA Demo",
	"AmericaVehicleDozer":      "USA",
	"AirF_AmericaVehicleDozer": "USA Airforce",
	"Lazr_AmericaVehicleDozer": "USA Lazr",
	"SupW_AmericaVehicleDozer": "USA Superweapon",
	"ChinaVehicleDozer":        "China",
	"Infa_ChinaVehicleDozer":   "China Infantry",
	"Nuke_ChinaVehicleDozer":   "China Nuke",
	"Tank_ChinaVehicleDozer":   "China Tank",
}

func (r *Replay) GenerateData() {
	for _, player := range r.Summary {
		for _, order := range r.Body {
			if order.PlayerName != player.Name {
				continue
			}
			if order.OrderCode == 1047 {
				if side, ok := ConstructorMap[order.Details.GetName()]; ok {
					if player.Side == "" {
						player.Side = side
					}
				}
				unit := object.Unit{
					Name: order.Details.GetName(),
					Cost: order.Details.GetCost(),
				}
				summary, ok := player.UnitsCreated[order.Details.GetName()]
				if !ok {
					summary = &object.ObjectSummary{}
					player.UnitsCreated[order.Details.GetName()] = summary
				}
				summary.Count++
				summary.TotalSpent += unit.Cost
				player.MoneySpent += unit.Cost
			}
			if order.OrderCode == 1049 {
				building := object.Building{
					Name: order.Details.GetName(),
					Cost: order.Details.GetCost(),
				}
				summary, ok := player.BuildingsBuilt[order.Details.GetName()]
				if !ok {
					summary = &object.ObjectSummary{}
					player.BuildingsBuilt[order.Details.GetName()] = summary
				}
				summary.Count++
				summary.TotalSpent += building.Cost
				player.MoneySpent += building.Cost
			}
			if order.OrderCode == 1045 {
				upgrade := object.Upgrade{
					Name: order.Details.GetName(),
					Cost: order.Details.GetCost(),
				}
				summary, ok := player.UpgradesBuilt[order.Details.GetName()]
				if !ok {
					summary = &object.ObjectSummary{}
					player.UpgradesBuilt[order.Details.GetName()] = summary
				}
				summary.Count++
				summary.TotalSpent += upgrade.Cost
				player.MoneySpent += upgrade.Cost

			}
			if order.OrderCode == 1041 || order.OrderCode == 1042 {
				player.PowersUsed[order.Details.GetName()]++

			}
			if order.OrderCode == 1093 {
				player.Win = false
			}
		}
	}

	// Hacky way to check results. Both players losing by selling or getting fully destroyed will break detection.
	teamWins := map[int]bool{}
	for _, player := range r.Summary {
		teamWins[player.Team] = true
	}
	for player := range r.Summary {
		if !r.Summary[player].Win {
			teamWins[r.Summary[player].Team] = false
		}
	}
	for player := range r.Summary {
		if !teamWins[r.Summary[player].Team] {
			r.Summary[player].Win = false
		}
	}
	winners := 0
	for _, teamWon := range teamWins {
		if teamWon {
			winners++
		}
	}

	if winners > 1 {
		// Uh oh. Hack it up real bad
		for teamID := range teamWins {
			teamWins[teamID] = false
		}

		for player := range r.Summary {
			r.Summary[player].Win = false
		}

		for i := len(r.Body) - 1; i >= 0; i-- {
			chunk := r.Body[i]
			if chunk.OrderCode != 1095 && chunk.OrderCode != 1003 && chunk.OrderCode != 1092 && chunk.OrderCode != 27 && chunk.OrderCode != 1052 {
				teamID := 0
				for _, player := range r.Summary {
					if player.Name == chunk.PlayerName {
						teamID = player.Team
					}
				}
				if teamID != 0 {
					teamWins[teamID] = true
					break
				}
			}
		}

		for player := range r.Summary {
			if teamWins[r.Summary[player].Team] {
				r.Summary[player].Win = true
			}
		}
	}
}

// StreamingReplay represents a streaming replay parser
type StreamingReplay struct {
	Header *header.GeneralsHeader
	Offset int
}

// StreamReplayOptions contains options for streaming replay parsing
type StreamReplayOptions struct {
	// PollInterval is the time to wait between file checks when no new data is available
	PollInterval time.Duration
	// MaxWaitTime is the maximum time to wait for new data before timing out
	MaxWaitTime time.Duration
	// BufferSize is the size of the channel buffer for body events
	BufferSize int
	// InactivityTimeout is the time to wait with no new data before stopping (default 2 minutes)
	InactivityTimeout time.Duration
}

// DefaultStreamReplayOptions returns sensible defaults for streaming
func DefaultStreamReplayOptions() *StreamReplayOptions {
	return &StreamReplayOptions{
		PollInterval:      100 * time.Millisecond,
		MaxWaitTime:       30 * time.Second,
		BufferSize:        100,
		InactivityTimeout: 2 * time.Minute,
	}
}

// StreamReplay streams body events from a replay file as it's being written.
// It reads the header first, then continuously reads body chunks and sends them
// through the returned channel. The channel is closed when the "EndReplay" command
// (order code 27) is encountered, when no new data has been written for the specified
// inactivity timeout, or when the context is cancelled.
func StreamReplay(ctx context.Context, filePath string, objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore, options *StreamReplayOptions) (<-chan *body.BodyChunk, *StreamingReplay, error) {
	if options == nil {
		options = DefaultStreamReplayOptions()
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, err
	}

	// Create BitParser for header reading
	bp := &bitparse.BitParser{
		Source:       file,
		ObjectStore:  objectStore,
		PowerStore:   powerStore,
		UpgradeStore: upgradeStore,
	}

	// Read header first
	header := header.NewHeader(bp)
	streamingReplay := &StreamingReplay{
		Header: header,
		Offset: 2, // Default offset, will be adjusted as we read body chunks
	}

	// Create channel for body events
	bodyChan := make(chan *body.BodyChunk, options.BufferSize)

	// Start streaming goroutine
	go func() {
		defer close(bodyChan)
		defer file.Close()

		// Create a new BitParser for body reading that can handle streaming
		streamBp := &bitparse.BitParser{
			Source:       file,
			ObjectStore:  objectStore,
			PowerStore:   powerStore,
			UpgradeStore: upgradeStore,
		}

		// Track the lowest player ID for offset calculation
		lowestPlayerID := 1000

		// Track last activity time for inactivity timeout
		lastActivity := time.Now()
		consecutiveEOFCount := 0

		for {
			select {
			case <-ctx.Done():
				return
			default:
				// Try to read a body chunk
				chunk, err := readStreamingBodyChunk(streamBp, objectStore, powerStore, upgradeStore)
				if err != nil {
					if err == io.EOF {
						// No more data available at current position
						consecutiveEOFCount++

						// Check if we've been inactive for too long
						if time.Since(lastActivity) > options.InactivityTimeout {
							// No activity for the specified timeout period, stop streaming
							return
						}

						// Wait a bit and try again
						time.Sleep(options.PollInterval)
						continue
					}
					// Other error, stop streaming
					return
				}

				if chunk == nil {
					// No chunk available yet, wait and try again
					time.Sleep(options.PollInterval)
					continue
				}

				// Reset consecutive EOF count and update activity time when we get a chunk
				consecutiveEOFCount = 0
				lastActivity = time.Now()

				// Check if this is the EndReplay command
				if chunk.OrderCode == 27 {
					// EndReplay command found, close the channel and return
					return
				}

				// Update offset if we found a lower player ID
				if chunk.PlayerID < lowestPlayerID {
					lowestPlayerID = chunk.PlayerID
					streamingReplay.Offset = lowestPlayerID
				}

				// Send chunk through channel
				select {
				case bodyChan <- chunk:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return bodyChan, streamingReplay, nil
}

// readStreamingBodyChunk attempts to read a single body chunk from the stream
func readStreamingBodyChunk(bp *bitparse.BitParser, objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore) (*body.BodyChunk, error) {
	// Read basic chunk data with error handling
	timeCode, err := bp.ReadUInt32()
	if err != nil {
		return nil, err
	}

	orderCode, err := bp.ReadUInt32()
	if err != nil {
		return nil, err
	}

	playerID, err := bp.ReadUInt32()
	if err != nil {
		return nil, err
	}

	numberOfArguments, err := bp.ReadUInt8()
	if err != nil {
		return nil, err
	}

	// Validate reasonable bounds for numberOfArguments
	if !body.ValidateArgCount(int(numberOfArguments)) {
		return nil, io.ErrUnexpectedEOF
	}

	chunk := &body.BodyChunk{
		TimeCode:          timeCode,
		OrderCode:         orderCode,
		PlayerID:          playerID,
		NumberOfArguments: numberOfArguments,
		ArgMetadata:       []*body.ArgMetadata{},
		Arguments:         []interface{}{},
	}
	chunk.OrderName = body.CommandType[chunk.OrderCode]

	// Read argument metadata
	for i := 0; i < chunk.NumberOfArguments; i++ {
		argType, err1 := bp.ReadUInt8()
		argCount, err2 := bp.ReadUInt8()
		if err1 != nil || err2 != nil {
			return nil, err1
		}
		// Validate argument type and count
		if !body.ValidateArgType(int(argType)) || !body.ValidateArgCount(int(argCount)) {
			return nil, io.ErrUnexpectedEOF
		}
		argCountData := &body.ArgMetadata{
			Type:  argType,
			Count: argCount,
		}
		chunk.ArgMetadata = append(chunk.ArgMetadata, argCountData)
	}

	// Read arguments
	for _, argData := range chunk.ArgMetadata {
		for i := 0; i < argData.Count; i++ {
			chunk.Arguments = append(chunk.Arguments, body.ConvertArg(bp, argData.Type))
		}
	}

	chunk.AddExtraData(objectStore, powerStore, upgradeStore)

	// Check for end of data markers
	if chunk.TimeCode == 0 && chunk.OrderCode == 0 && chunk.PlayerID == 0 {
		return nil, io.EOF
	}

	return chunk, nil
}
