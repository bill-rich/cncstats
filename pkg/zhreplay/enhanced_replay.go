package zhreplay

import (
	"github.com/bill-rich/cncstats/pkg/database"
	"github.com/bill-rich/cncstats/pkg/zhreplay/body"
	"github.com/bill-rich/cncstats/pkg/zhreplay/header"
	"github.com/bill-rich/cncstats/pkg/zhreplay/object"
)

// EnhancedBodyChunk represents a body chunk with player money and stats data
type EnhancedBodyChunk struct {
	*body.BodyChunk
	PlayerMoney *PlayerMoneyData `json:"PlayerMoney,omitempty"`
	PlayerStats *PlayerStatsData `json:"PlayerStats,omitempty"`
}

// PlayerMoneyData represents the money data for players at a specific seed
type PlayerMoneyData struct {
	PlayerMoney [8]int `json:"PlayerMoney"`
}

// PlayerStatsData represents all the stats data for players at a specific seed
type PlayerStatsData struct {
	MoneyEarned              [8]int    `json:"money_earned"`
	UnitsBuilt               [8]int    `json:"units_built"`
	UnitsLost                [8]int    `json:"units_lost"`
	BuildingsBuilt           [8]int    `json:"buildings_built"`
	BuildingsLost            [8]int    `json:"buildings_lost"`
	BuildingsKilled          [8][8]int `json:"buildings_killed"`
	UnitsKilled              [8][8]int `json:"units_killed"`
	GeneralsPointsTotal      [8]int    `json:"generals_points_total"`
	GeneralsPointsUsed       [8]int    `json:"generals_points_used"`
	RadarsBuilt              [8]int    `json:"radars_built"`
	SearchAndDestroy         [8]int    `json:"search_and_destroy"`
	HoldTheLine              [8]int    `json:"hold_the_line"`
	Bombardment              [8]int    `json:"bombardment"`
	XP                       [8]int    `json:"xp"`
	XPLevel                  [8]int    `json:"xp_level"`
	TechBuildingsCaptured    [8]int    `json:"tech_buildings_captured"`
	FactionBuildingsCaptured [8]int    `json:"faction_buildings_captured"`
	PowerTotal               [8]int    `json:"power_total"`
	PowerUsed                [8]int    `json:"power_used"`
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
	// So we need to subtract 2 from playerID to get the correct index (0-based)
	playerIndex := playerID - 2

	if playerIndex >= 0 && playerIndex < 8 {
		return pmd.PlayerMoney[playerIndex]
	}
	return 0
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
				PlayerMoney: [8]int{
					moneyData.Player1Money,
					moneyData.Player2Money,
					moneyData.Player3Money,
					moneyData.Player4Money,
					moneyData.Player5Money,
					moneyData.Player6Money,
					moneyData.Player7Money,
					moneyData.Player8Money,
				},
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
				PlayerMoney: [8]int{
					moneyData.Player1Money,
					moneyData.Player2Money,
					moneyData.Player3Money,
					moneyData.Player4Money,
					moneyData.Player5Money,
					moneyData.Player6Money,
					moneyData.Player7Money,
					moneyData.Player8Money,
				},
			},
		}
		moneyChangeEvents = append(moneyChangeEvents, moneyChangeEvent)
	}

	// Merge the money change events with existing body chunks, sorted by timecode
	er.MergeChangeEvents(moneyChangeEvents)
}

// AddStatsChangeEvents creates separate stats change events from database records and inserts them at appropriate timecode positions
func (er *EnhancedReplay) AddStatsChangeEvents() {
	// Get the player money service
	playerMoneyService := database.NewPlayerMoneyService()

	// Get seed from metadata
	seed := er.Header.Metadata.Seed
	if seed == "" {
		// If seed is empty, skip processing
		return
	}

	// Get all stats change records for this seed
	statsChanges, err := playerMoneyService.GetAllPlayerMoneyDataBySeed(seed)
	if err != nil {
		// Log error but continue processing
		return
	}

	// Create stats change events for each database record
	var statsChangeEvents []*EnhancedBodyChunk
	for _, statsData := range statsChanges {
		// Convert database arrays to Go arrays
		moneyEarned := convertInt32Array8ToIntArray8(statsData.MoneyEarned)
		unitsBuilt := convertInt32Array8ToIntArray8(statsData.UnitsBuilt)
		unitsLost := convertInt32Array8ToIntArray8(statsData.UnitsLost)
		buildingsBuilt := convertInt32Array8ToIntArray8(statsData.BuildingsBuilt)
		buildingsLost := convertInt32Array8ToIntArray8(statsData.BuildingsLost)
		buildingsKilled := convertInt32Array8x8ToIntArray8x8(statsData.BuildingsKilled)
		unitsKilled := convertInt32Array8x8ToIntArray8x8(statsData.UnitsKilled)
		generalsPointsTotal := convertInt32Array8ToIntArray8(statsData.GeneralsPointsTotal)
		generalsPointsUsed := convertInt32Array8ToIntArray8(statsData.GeneralsPointsUsed)
		radarsBuilt := convertInt32Array8ToIntArray8(statsData.RadarsBuilt)
		searchAndDestroy := convertInt32Array8ToIntArray8(statsData.SearchAndDestroy)
		holdTheLine := convertInt32Array8ToIntArray8(statsData.HoldTheLine)
		bombardment := convertInt32Array8ToIntArray8(statsData.Bombardment)
		xp := convertInt32Array8ToIntArray8(statsData.XP)
		xpLevel := convertInt32Array8ToIntArray8(statsData.XPLevel)
		techBuildingsCaptured := convertInt32Array8ToIntArray8(statsData.TechBuildingsCaptured)
		factionBuildingsCaptured := convertInt32Array8ToIntArray8(statsData.FactionBuildingsCaptured)
		powerTotal := convertInt32Array8ToIntArray8(statsData.PowerTotal)
		powerUsed := convertInt32Array8ToIntArray8(statsData.PowerUsed)

		// Helper function to create a stat change event
		createStatEvent := func(orderCode int, orderName string, stats *PlayerStatsData) *EnhancedBodyChunk {
			return &EnhancedBodyChunk{
				BodyChunk: &body.BodyChunk{
					TimeCode:          statsData.Timecode,
					OrderCode:         orderCode,
					OrderName:         orderName,
					PlayerID:          0, // Stats changes affect all players
					PlayerName:        "",
					NumberOfArguments: 0,
					Details:           nil,
					ArgMetadata:       []*body.ArgMetadata{},
					Arguments:         []interface{}{},
				},
				PlayerStats: stats,
			}
		}

		// Create separate events for each stat type
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2001, "MoneyEarnedChange", &PlayerStatsData{MoneyEarned: moneyEarned}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2002, "UnitsBuiltChange", &PlayerStatsData{UnitsBuilt: unitsBuilt}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2003, "UnitsLostChange", &PlayerStatsData{UnitsLost: unitsLost}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2004, "BuildingsBuiltChange", &PlayerStatsData{BuildingsBuilt: buildingsBuilt}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2005, "BuildingsLostChange", &PlayerStatsData{BuildingsLost: buildingsLost}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2006, "BuildingsKilledChange", &PlayerStatsData{BuildingsKilled: buildingsKilled}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2007, "UnitsKilledChange", &PlayerStatsData{UnitsKilled: unitsKilled}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2008, "GeneralsPointsTotalChange", &PlayerStatsData{GeneralsPointsTotal: generalsPointsTotal}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2009, "GeneralsPointsUsedChange", &PlayerStatsData{GeneralsPointsUsed: generalsPointsUsed}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2010, "RadarsBuiltChange", &PlayerStatsData{RadarsBuilt: radarsBuilt}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2011, "SearchAndDestroyChange", &PlayerStatsData{SearchAndDestroy: searchAndDestroy}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2012, "HoldTheLineChange", &PlayerStatsData{HoldTheLine: holdTheLine}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2013, "BombardmentChange", &PlayerStatsData{Bombardment: bombardment}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2014, "XPChange", &PlayerStatsData{XP: xp}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2015, "XPLevelChange", &PlayerStatsData{XPLevel: xpLevel}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2016, "TechBuildingsCapturedChange", &PlayerStatsData{TechBuildingsCaptured: techBuildingsCaptured}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2017, "FactionBuildingsCapturedChange", &PlayerStatsData{FactionBuildingsCaptured: factionBuildingsCaptured}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2018, "PowerTotalChange", &PlayerStatsData{PowerTotal: powerTotal}))
		statsChangeEvents = append(statsChangeEvents, createStatEvent(2019, "PowerUsedChange", &PlayerStatsData{PowerUsed: powerUsed}))
	}

	// Merge the stats change events with existing body chunks, sorted by timecode
	er.MergeChangeEvents(statsChangeEvents)
}

// convertInt32Array8ToIntArray8 converts database Int32Array8 to Go [8]int
func convertInt32Array8ToIntArray8(arr database.Int32Array8) [8]int {
	result := [8]int{}
	for i := 0; i < 8; i++ {
		result[i] = int(arr[i])
	}
	return result
}

// convertInt32Array8x8ToIntArray8x8 converts database Int32Array8x8 to Go [8][8]int
func convertInt32Array8x8ToIntArray8x8(arr database.Int32Array8x8) [8][8]int {
	result := [8][8]int{}
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			result[i][j] = int(arr[i][j])
		}
	}
	return result
}

// MergeChangeEvents merges change events (money, stats, etc.) with existing body chunks, maintaining chronological order
func (er *EnhancedReplay) MergeChangeEvents(changeEvents []*EnhancedBodyChunk) {
	// Create a new slice to hold all events (original + change events)
	var allEvents []*EnhancedBodyChunk

	// Add all existing body chunks
	allEvents = append(allEvents, er.Body...)

	// Add all change events
	allEvents = append(allEvents, changeEvents...)

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

// MergeMoneyChangeEvents is a convenience wrapper for backward compatibility
func (er *EnhancedReplay) MergeMoneyChangeEvents(moneyChangeEvents []*EnhancedBodyChunk) {
	er.MergeChangeEvents(moneyChangeEvents)
}

// DetermineWinnersByMoney determines winners based on money information from the last money event
func (er *EnhancedReplay) DetermineWinnersByMoney() {
	// Find the last money event (order code 2000)
	var lastMoneyEvent *EnhancedBodyChunk
	for i := len(er.Body) - 1; i >= 0; i-- {
		if er.Body[i].OrderCode == 2000 { // MoneyValueChange
			lastMoneyEvent = er.Body[i]
			break
		}
	}

	// If no money event found, fall back to original logic
	if lastMoneyEvent == nil || lastMoneyEvent.PlayerMoney == nil {
		er.fallbackWinnerDetection()
		return
	}

	// Reset all players to not winning initially
	for _, player := range er.Summary {
		player.Win = false
	}

	// Create a map to track which teams have players with money
	teamWins := make(map[int]bool)

	// Check each player's money at the last money event
	for _, player := range er.Summary {
		// Skip observers (faction -2, identified by Side == "Observer")
		if player.Side == "Observer" {
			continue
		}

		// Get player ID (PlayerID starts at 2, so we need to map to the correct player index)
		playerID := er.getPlayerIDFromName(player.Name)
		if playerID == 0 {
			continue // Skip if we can't find the player ID
		}

		// Get money data for this player from the last money event
		playerMoney := lastMoneyEvent.PlayerMoney.GetPlayerMoneyForPlayerID(playerID)

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
		er.fallbackWinnerDetection()
		return
	}

	// Apply team win logic - any player on a winning team also wins (excluding observers)
	for _, player := range er.Summary {
		// Skip observers (faction -2, identified by Side == "Observer")
		if player.Side == "Observer" {
			continue
		}
		if teamWins[player.Team] {
			player.Win = true
		}
	}
}

// getPlayerIDFromName returns the player ID for a given player name
func (er *EnhancedReplay) getPlayerIDFromName(playerName string) int {
	for _, chunk := range er.Body {
		if chunk.PlayerName == playerName {
			return chunk.PlayerID
		}
	}
	return 0
}

// fallbackWinnerDetection provides the original winner detection logic as a fallback
func (er *EnhancedReplay) fallbackWinnerDetection() {
	// Original hacky way to check results. Both players losing by selling or getting fully destroyed will break detection.
	teamWins := map[int]bool{}
	for _, player := range er.Summary {
		teamWins[player.Team] = true
	}
	for player := range er.Summary {
		if !er.Summary[player].Win {
			teamWins[er.Summary[player].Team] = false
		}
	}
	for player := range er.Summary {
		if !teamWins[er.Summary[player].Team] {
			er.Summary[player].Win = false
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

		for player := range er.Summary {
			er.Summary[player].Win = false
		}

		for i := len(er.Body) - 1; i >= 0; i-- {
			chunk := er.Body[i]
			if chunk.OrderCode != 1095 && chunk.OrderCode != 1003 && chunk.OrderCode != 1092 && chunk.OrderCode != 27 && chunk.OrderCode != 1052 {
				teamID := 0
				for _, player := range er.Summary {
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

		for player := range er.Summary {
			if teamWins[er.Summary[player].Team] {
				er.Summary[player].Win = true
			}
		}
	}
}
