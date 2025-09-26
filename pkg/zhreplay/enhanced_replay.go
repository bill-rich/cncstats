package zhreplay

import (
	"time"

	"github.com/bill-rich/cncstats/pkg/database"
	"github.com/bill-rich/cncstats/pkg/zhreplay/body"
	"github.com/bill-rich/cncstats/pkg/zhreplay/header"
	"github.com/bill-rich/cncstats/pkg/zhreplay/object"
)

// EnhancedBodyChunk represents a body chunk with player money data
type EnhancedBodyChunk struct {
	*body.BodyChunk
	PlayerMoney *PlayerMoneyData `json:"player_money,omitempty"`
}

// PlayerMoneyData represents the money data for players at a specific timestamp
type PlayerMoneyData struct {
	Player1Money int64 `json:"player_1_money"`
	Player2Money int64 `json:"player_2_money"`
	Player3Money int64 `json:"player_3_money"`
	Player4Money int64 `json:"player_4_money"`
	Player5Money int64 `json:"player_5_money"`
	Player6Money int64 `json:"player_6_money"`
	Player7Money int64 `json:"player_7_money"`
	Player8Money int64 `json:"player_8_money"`
}

// EnhancedReplay represents a replay with enhanced data including player money
type EnhancedReplay struct {
	Header  *header.GeneralsHeader  `json:"header"`
	Body    []*EnhancedBodyChunk    `json:"body"`
	Summary []*object.PlayerSummary `json:"summary"`
	Offset  int                     `json:"offset"`
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
func (pmd *PlayerMoneyData) GetPlayerMoneyForPlayerID(playerID int) int64 {
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

	// Get the timestamp begin from the header
	timestampBegin := time.Unix(int64(er.Header.TimeStampBegin), 0)

	// Process each body chunk
	for i, chunk := range er.Body {
		// Only process ordertypes that are NOT 1095, 1092, or 1003
		if chunk.OrderCode == 1095 || chunk.OrderCode == 1092 || chunk.OrderCode == 1003 {
			continue
		}

		// Try to find matching player money data for this timecode and timestamp
		moneyData, err := playerMoneyService.GetPlayerMoneyDataByTimecodeAndTimestamp(
			int64(chunk.TimeCode),
			timestampBegin,
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
