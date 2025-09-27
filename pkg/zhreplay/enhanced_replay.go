package zhreplay

import (
	"github.com/bill-rich/cncstats/pkg/database"
	"github.com/bill-rich/cncstats/pkg/zhreplay/body"
	"github.com/bill-rich/cncstats/pkg/zhreplay/header"
	"github.com/bill-rich/cncstats/pkg/zhreplay/object"
)

// EnhancedBodyChunk represents a body chunk with player money data
type EnhancedBodyChunk struct {
	*body.BodyChunk
	PlayerMoney *PlayerMoneyData `json:"PlayerMoney,omitempty"`
}

// PlayerMoneyData represents the money data for players at a specific seed
type PlayerMoneyData struct {
	Player1Money int `json:"Player1Money"`
	Player2Money int `json:"Player2Money"`
	Player3Money int `json:"Player3Money"`
	Player4Money int `json:"Player4Money"`
	Player5Money int `json:"Player5Money"`
	Player6Money int `json:"Player6Money"`
	Player7Money int `json:"Player7Money"`
	Player8Money int `json:"Player8Money"`
}

// EnhancedReplay represents a replay with enhanced data including player money
type EnhancedReplay struct {
	Header  *header.GeneralsHeader  `json:"Header"`
	Body    []*EnhancedBodyChunk    `json:"Body"`
	Summary []*object.PlayerSummary `json:"Summary"`
	Offset  int                     `json:"Offset"`
}

// ConvertToEnhancedReplay converts a regular replay to an enhanced replay
func ConvertToEnhancedReplay(replay *Replay) *EnhancedReplay {
	enhanced := &EnhancedReplay{
		Header:  replay.Header,
		Summary: replay.Summary,
		Offset:  replay.Offset,
		Body:    make([]*EnhancedBodyChunk, len(replay.Body)),
	}

	for i, chunk := range replay.Body {
		enhanced.Body[i] = &EnhancedBodyChunk{
			BodyChunk: chunk,
		}
	}

	return enhanced
}

// AddPlayerMoneyToChunk adds player money data to a specific chunk
func (er *EnhancedReplay) AddPlayerMoneyToChunk(chunkIndex int, moneyData *PlayerMoneyData) {
	if chunkIndex >= 0 && chunkIndex < len(er.Body) {
		er.Body[chunkIndex].PlayerMoney = moneyData
	}
}

// GetPlayerMoneyForPlayerID returns the money amount for a specific player ID
// Note: PlayerID 2 = Player1, PlayerID 3 = Player2, etc.
func (pmd *PlayerMoneyData) GetPlayerMoneyForPlayerID(playerID int) int {
	// PlayerID 2 = Player1, PlayerID 3 = Player2, etc.
	// So we need to subtract 1 from playerID to get the correct index
	playerIndex := playerID - 1

	switch playerIndex {
	case 1:
		return pmd.Player1Money
	case 2:
		return pmd.Player2Money
	case 3:
		return pmd.Player3Money
	case 4:
		return pmd.Player4Money
	case 5:
		return pmd.Player5Money
	case 6:
		return pmd.Player6Money
	case 7:
		return pmd.Player7Money
	case 8:
		return pmd.Player8Money
	default:
		return 0
	}
}

// AddPlayerMoneyData matches replay chunks with database records and adds player money data
func (er *EnhancedReplay) AddPlayerMoneyData() {
	// Get the player money service
	playerMoneyService := database.NewPlayerMoneyService()

	// Get seed from metadata
	seed := er.Header.Metadata.Seed
	if seed == "" {
		// If seed is empty, skip processing
		return
	}

	// Process each body chunk
	for i, chunk := range er.Body {
		// Only process ordertypes that are NOT 1095, 1092, or 1003
		if chunk.OrderCode == 1095 || chunk.OrderCode == 1092 || chunk.OrderCode == 1003 {
			continue
		}

		// Try to find matching player money data for this timecode and seed
		moneyData, err := playerMoneyService.GetPlayerMoneyDataByTimecodeAndSeed(
			chunk.TimeCode,
			seed,
		)

		if err != nil {
			// Log error but continue processing
			continue
		}

		if moneyData != nil {
			// Convert database money data to our PlayerMoneyData format
			playerMoney := &PlayerMoneyData{
				Player1Money: moneyData.Player1Money,
				Player2Money: moneyData.Player2Money,
				Player3Money: moneyData.Player3Money,
				Player4Money: moneyData.Player4Money,
				Player5Money: moneyData.Player5Money,
				Player6Money: moneyData.Player6Money,
				Player7Money: moneyData.Player7Money,
				Player8Money: moneyData.Player8Money,
			}

			// Add the money data to this chunk
			er.AddPlayerMoneyToChunk(i, playerMoney)
		}
	}
}

// AddMoneyChangeEvents creates separate money change events from database records and inserts them at appropriate timecode positions
func (er *EnhancedReplay) AddMoneyChangeEvents() {
	// Get the player money service
	playerMoneyService := database.NewPlayerMoneyService()

	// Get seed from metadata
	seed := er.Header.Metadata.Seed
	if seed == "" {
		// If seed is empty, skip processing
		return
	}

	// Get all money change records for this seed
	moneyChanges, err := playerMoneyService.GetAllPlayerMoneyDataBySeed(seed)
	if err != nil {
		// Log error but continue processing
		return
	}

	// Create money change events for each database record
	var moneyChangeEvents []*EnhancedBodyChunk
	for _, moneyData := range moneyChanges {
		// Create a money change event for each timecode
		moneyChangeEvent := &EnhancedBodyChunk{
			BodyChunk: &body.BodyChunk{
				TimeCode:          moneyData.Timecode,
				OrderCode:         2000, // MoneyValueChange code
				OrderName:         "MoneyValueChange",
				PlayerID:          0, // Money changes affect all players
				PlayerName:        "",
				NumberOfArguments: 0,
				Details:           nil,
				ArgMetadata:       []*body.ArgMetadata{},
				Arguments:         []interface{}{},
			},
			PlayerMoney: &PlayerMoneyData{
				Player1Money: moneyData.Player1Money,
				Player2Money: moneyData.Player2Money,
				Player3Money: moneyData.Player3Money,
				Player4Money: moneyData.Player4Money,
				Player5Money: moneyData.Player5Money,
				Player6Money: moneyData.Player6Money,
				Player7Money: moneyData.Player7Money,
				Player8Money: moneyData.Player8Money,
			},
		}
		moneyChangeEvents = append(moneyChangeEvents, moneyChangeEvent)
	}

	// Merge the money change events with existing body chunks, sorted by timecode
	er.MergeMoneyChangeEvents(moneyChangeEvents)
}

// MergeMoneyChangeEvents merges money change events with existing body chunks, maintaining chronological order
func (er *EnhancedReplay) MergeMoneyChangeEvents(moneyChangeEvents []*EnhancedBodyChunk) {
	// Create a new slice to hold all events (original + money changes)
	var allEvents []*EnhancedBodyChunk

	// Add all existing body chunks
	allEvents = append(allEvents, er.Body...)

	// Add all money change events
	allEvents = append(allEvents, moneyChangeEvents...)

	// Sort all events by timecode
	// Simple bubble sort for now - could be optimized with a more efficient sort
	for i := 0; i < len(allEvents)-1; i++ {
		for j := 0; j < len(allEvents)-i-1; j++ {
			if allEvents[j].TimeCode > allEvents[j+1].TimeCode {
				allEvents[j], allEvents[j+1] = allEvents[j+1], allEvents[j]
			}
		}
	}

	// Update the body with the merged and sorted events
	er.Body = allEvents
}
