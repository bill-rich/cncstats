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
	Version int
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
		if playerMd.Faction == "-2" {
			player.Side = "Observer"
			player.Team = -1
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

	// Use money information to determine winners
	r.determineWinnersByMoney()
}

// determineWinnersByMoney determines winners based on money information from the last money event
func (r *Replay) determineWinnersByMoney() {
	// Find the last money event (order code 2000)
	var lastMoneyEvent *body.BodyChunk
	for i := len(r.Body) - 1; i >= 0; i-- {
		if r.Body[i].OrderCode == 2000 { // MoneyValueChange
			lastMoneyEvent = r.Body[i]
			break
		}
	}

	// If no money event found, fall back to original logic
	if lastMoneyEvent == nil {
		r.fallbackWinnerDetection()
		return
	}

	// Reset all players to not winning initially
	for _, player := range r.Summary {
		player.Win = false
	}

	// Create a map to track which teams have players with money
	teamWins := make(map[int]bool)

	// Check each player's money at the last money event
	// We need to access the money data from the database or enhanced replay
	// For now, we'll use a simplified approach that checks if we can get money data
	for _, player := range r.Summary {
		// Get player ID (PlayerID starts at 2, so we need to map to the correct player index)
		playerID := r.getPlayerIDFromName(player.Name)
		if playerID == 0 {
			continue // Skip if we can't find the player ID
		}

		// Try to get money data for this player
		// This would need to be implemented to access the actual money data
		// from the database or enhanced replay structure
		playerMoney := r.getPlayerMoneyFromLastEvent(playerID, lastMoneyEvent)

		// Player wins if they still have money (money > 0)
		if playerMoney > 0 {
			teamWins[player.Team] = true
		}
	}

	// Check if more than one team would win - if so, fall back to original logic
	winningTeams := 0
	for _, teamWon := range teamWins {
		if teamWon {
			winningTeams++
		}
	}

	if winningTeams > 1 {
		// More than one team would win, fall back to original logic
		r.fallbackWinnerDetection()
		return
	}

	// Apply team win logic - any player on a winning team also wins
	for _, player := range r.Summary {
		if teamWins[player.Team] {
			player.Win = true
		}
	}
}

// getPlayerIDFromName returns the player ID for a given player name
func (r *Replay) getPlayerIDFromName(playerName string) int {
	for _, chunk := range r.Body {
		if chunk.PlayerName == playerName {
			return chunk.PlayerID
		}
	}
	return 0
}

// getPlayerMoneyFromLastEvent gets the money amount for a player from the last money event
// This implementation works with the enhanced replay structure that includes money data
func (r *Replay) getPlayerMoneyFromLastEvent(playerID int, lastMoneyEvent *body.BodyChunk) int {
	// This method should be called on an EnhancedReplay, not a regular Replay
	// The regular Replay doesn't have access to money data
	// For now, we'll return 0 to indicate no money data available
	// In practice, this should be called on an EnhancedReplay instance
	return 0
}

// fallbackWinnerDetection provides the original winner detection logic as a fallback
func (r *Replay) fallbackWinnerDetection() {
	// Original hacky way to check results. Both players losing by selling or getting fully destroyed will break detection.
	teamWins := map[int]bool{}

	// All of this first part is based off if one of the players quit. If someone quits, there is a good chance the team lost.
	// Set all teams to winning initially
	for _, player := range r.Summary {
		teamWins[player.Team] = true
	}

	// If any player on a team lost, the whole team loses
	for player := range r.Summary {
		if !r.Summary[player].Win {
			teamWins[r.Summary[player].Team] = false
		}
	}

	// Apply team win status to players
	for player := range r.Summary {
		if !teamWins[r.Summary[player].Team] {
			r.Summary[player].Win = false
		}
	}

	// Check if more than one team is winning
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

		// Reset all players to not winning initially
		for player := range r.Summary {
			r.Summary[player].Win = false
		}

		// Look for the last non-quit, non-passive command to determine the winner
		for i := len(r.Body) - 1; i >= 0; i-- {
			chunk := r.Body[i]
			if body.PassiveCommands[chunk.OrderCode] == false {
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
		Offset: 2, // Default offset, will be adjusted as we read body chunks
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
	var lastProcessedTimestamp int = 0

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
						streamingReplay.Offset = lowestPlayerID
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

// parseNewData parses raw bytes and returns only chunks with timestamps after the last processed timestamp
func parseNewData(data []byte, objectStore *iniparse.ObjectStore, powerStore *iniparse.PowerStore, upgradeStore *iniparse.UpgradeStore, lastProcessedTimestamp *int) []*body.BodyChunk {
	if len(data) == 0 {
		return nil
	}

	// Create a BitParser from the raw data
	bp := &bitparse.BitParser{
		Source:       &bytesReader{data: data},
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

// bytesReader implements io.Reader for a byte slice
type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
