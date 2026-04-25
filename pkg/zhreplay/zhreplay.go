package zhreplay

import (
	"bytes"
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
	Header         *header.GeneralsHeader
	Body           []*body.BodyChunk
	Summary        []*object.PlayerSummary
	PlayerIDOffset int
	WinMethod      string
}

func NewReplay(bp *bitparse.BitParser) *Replay {
	replay := &Replay{
		PlayerIDOffset: 2,
	}
	replay.Header = header.NewHeader(bp)
	replay.CreatePlayerList()
	replay.Body = body.ParseBody(bp, bp.ObjectStore, bp.PowerStore, bp.UpgradeStore)
	replay.AdjustPlayerIDOffset()
	replay.AddUserNames()
	replay.GenerateData()
	return replay
}

func (r *Replay) AddUserNames() {
	for _, chunk := range r.Body {
		if chunk.PlayerID >= r.PlayerIDOffset && chunk.PlayerID-r.PlayerIDOffset < len(r.Summary) {
			chunk.PlayerName = r.Summary[chunk.PlayerID-r.PlayerIDOffset].Name
		}
	}
}

func (r *Replay) AdjustPlayerIDOffset() {
	lowest := 1000
	for _, chunk := range r.Body {
		if chunk.PlayerID < lowest {
			lowest = chunk.PlayerID
		}
	}
	r.PlayerIDOffset = lowest
}

func (r *Replay) CreatePlayerList() {
	nextSoloTeam := 100 // FFA players get unique team IDs starting at 100
	for _, playerMd := range r.Header.Metadata.Players {
		team, _ := strconv.Atoi(playerMd.Team)
		team++ // header is 0-indexed, we use 1-indexed

		// In FFA games, team is 0 (header value "-1" + 1). Assign each
		// teamless player their own unique team so winner detection
		// treats them individually.
		if team == 0 {
			team = nextSoloTeam
			nextSoloTeam++
		}

		player := &object.PlayerSummary{
			Name:           playerMd.Name,
			Team:           team,
			Win:            true,
			BuildingsBuilt: map[string]*object.ObjectSummary{},
			UnitsCreated:   map[string]*object.ObjectSummary{},
			UpgradesBuilt:  map[string]*object.ObjectSummary{},
			PowersUsed:     map[string]int{},
		}
		if playerMd.PlayerTemplate == "-2" {
			player.Side = "Observer"
			player.Team = -1
		}
		r.Summary = append(r.Summary, player)
	}
}

var constructorMap = map[string]string{
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

// trackObject increments the count and cost in the given summary map and updates player spend.
func trackObject(summaryMap map[string]*object.ObjectSummary, player *object.PlayerSummary, name string, cost int) {
	summary, ok := summaryMap[name]
	if !ok {
		summary = &object.ObjectSummary{}
		summaryMap[name] = summary
	}
	summary.Count++
	summary.TotalSpent += cost
	player.MoneySpent += cost
}

func (r *Replay) GenerateData() {
	for _, player := range r.Summary {
		for _, order := range r.Body {
			if order.PlayerName != player.Name {
				continue
			}

			switch order.OrderCode {
			case 1047: // CreateUnit
				if order.Details == nil {
					continue
				}
				name := order.Details.GetName()
				cost := order.Details.GetCost()
				if side, ok := constructorMap[name]; ok && player.Side == "" {
					player.Side = side
				}
				trackObject(player.UnitsCreated, player, name, cost)
			case 1049: // BuildObject
				if order.Details == nil {
					continue
				}
				trackObject(player.BuildingsBuilt, player, order.Details.GetName(), order.Details.GetCost())
			case 1045: // BuildUpgrade
				if order.Details == nil {
					continue
				}
				trackObject(player.UpgradesBuilt, player, order.Details.GetName(), order.Details.GetCost())
			case 1041, 1042: // SpecialPower at location/object
				if order.Details == nil {
					continue
				}
				player.PowersUsed[order.Details.GetName()]++
			case 1093: // Surrender
				player.Win = false
			}
		}
	}

	r.fallbackWinnerDetection()
}

// fallbackWinnerDetection determines the winner from replay commands.
// Strategy 1 (quitCommand): A player who surrendered (order 1093) has Win=false.
//   If exactly one team has all members still winning, that team wins.
// Strategy 2 (lastCommand): If no team quit, the team that issued the last
//   non-passive command (attack, build, etc.) is assumed to have won.
func (r *Replay) fallbackWinnerDetection() {
	teamWins := map[int]bool{}

	// Start by assuming every team wins; mark a team as lost if any member surrendered.
	for _, p := range r.Summary {
		teamWins[p.Team] = true
	}
	for _, p := range r.Summary {
		if !p.Win {
			teamWins[p.Team] = false
		}
	}

	// Propagate team result to individual players.
	for _, p := range r.Summary {
		if !teamWins[p.Team] {
			p.Win = false
		}
	}

	// Count winning teams.
	winners := 0
	for _, won := range teamWins {
		if won {
			winners++
		}
	}

	// If exactly one team won via quit detection, we're done.
	if winners <= 1 {
		r.WinMethod = "quitCommand"
		return
	}

	// Multiple teams still winning — fall back to last-command heuristic.
	// Reset everyone to losing, then find the team of the last active command.
	for _, p := range r.Summary {
		p.Win = false
	}
	for id := range teamWins {
		teamWins[id] = false
	}

	for i := len(r.Body) - 1; i >= 0; i-- {
		chunk := r.Body[i]
		if body.PassiveCommands[chunk.OrderCode] {
			continue
		}
		for _, p := range r.Summary {
			if p.Name == chunk.PlayerName {
				teamWins[p.Team] = true
				break
			}
		}
		break
	}

	for _, p := range r.Summary {
		p.Win = teamWins[p.Team]
	}
	r.WinMethod = "lastCommand"
}

// StreamingReplay represents a streaming replay parser
type StreamingReplay struct {
	Header         *header.GeneralsHeader
	PlayerIDOffset int
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
// It reads the header first, then continuously monitors the file for new data using
// file system notifications. The channel is closed when the "EndReplay" command
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
		PlayerIDOffset: 2,
	}

	// Create channel for body events
	bodyChan := make(chan *body.BodyChunk, options.BufferSize)

	// Start streaming goroutine
	go func() {
		defer close(bodyChan)
		defer file.Close()

		// For now, use the polling approach which is more reliable
		// TODO: Implement proper file watching once we confirm polling works
		streamWithPolling(ctx, file, bodyChan, streamingReplay, objectStore, powerStore, upgradeStore, options)
	}()

	return bodyChan, streamingReplay, nil
}

// streamWithPolling is a fallback implementation that uses polling instead of file watching
func streamWithPolling(ctx context.Context, file *os.File, bodyChan chan<- *body.BodyChunk, streamingReplay *StreamingReplay, objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore, options *StreamReplayOptions) {
	// Track the lowest player ID for offset calculation
	lowestPlayerID := 1000

	// Track last activity time for inactivity timeout
	lastActivity := time.Now()

	// Track the last processed timestamp to filter out old chunks
	var lastProcessedTimestamp int

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Get current file size
			fileInfo, err := file.Stat()
			if err != nil {
				// File error, stop streaming
				return
			}

			// Get current file position
			currentPos, err := file.Seek(0, io.SeekCurrent)
			if err != nil {
				// Seek error, stop streaming
				return
			}

			// If file has grown, read the new data
			if fileInfo.Size() > currentPos {
				// Read new data from current position to end of file
				newData := make([]byte, fileInfo.Size()-currentPos)
				_, err := file.Read(newData)
				if err != nil && err != io.EOF {
					// Read error, stop streaming
					return
				}

				// Parse the new data
				chunks := parseNewData(newData, objectStore, powerStore, upgradeStore, &lastProcessedTimestamp)

				// Process each new chunk
				for _, chunk := range chunks {
					// Update activity time when we get a chunk
					lastActivity = time.Now()

					// Check if this is the EndReplay command
					if chunk.OrderCode == 27 {
						// EndReplay command found, close the channel and return
						return
					}

					// Update offset if we found a lower player ID
					if chunk.PlayerID < lowestPlayerID {
						lowestPlayerID = chunk.PlayerID
						streamingReplay.PlayerIDOffset = lowestPlayerID
					}

					// Send chunk through channel
					select {
					case bodyChan <- chunk:
					case <-ctx.Done():
						return
					}
				}
			} else {
				// No new data, check for inactivity timeout
				if time.Since(lastActivity) > options.InactivityTimeout {
					// No activity for the specified timeout period, stop streaming
					return
				}
			}

			// Wait before next poll
			time.Sleep(options.PollInterval)
		}
	}
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
		if err1 != nil {
			return nil, err1
		}
		if err2 != nil {
			return nil, err2
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

// parseNewData parses raw bytes and returns only chunks with timestamps after the last processed timestamp
func parseNewData(data []byte, objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore, lastProcessedTimestamp *int) []*body.BodyChunk {
	if len(data) == 0 {
		return nil
	}

	// Create a BitParser from the raw data
	bp := &bitparse.BitParser{
		Source:       bytes.NewReader(data),
		ObjectStore:  objectStore,
		PowerStore:   powerStore,
		UpgradeStore: upgradeStore,
	}

	var chunks []*body.BodyChunk

	// Parse chunks from the data
	for {
		chunk, err := readStreamingBodyChunk(bp, objectStore, powerStore, upgradeStore)
		if err != nil {
			// EOF or other error, we're done with this batch
			break
		}

		if chunk == nil {
			// No more chunks available
			break
		}

		// Only include chunks with timestamps after the last processed timestamp
		if chunk.TimeCode > *lastProcessedTimestamp {
			chunks = append(chunks, chunk)
			// Update the last processed timestamp
			*lastProcessedTimestamp = chunk.TimeCode
		}
	}

	return chunks
}

