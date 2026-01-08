package database

import (
	"fmt"

	"gorm.io/gorm"
)

// PlayerMoneyService handles operations related to player money data
type PlayerMoneyService struct {
	db *gorm.DB
}

// NewPlayerMoneyService creates a new PlayerMoneyService
func NewPlayerMoneyService() *PlayerMoneyService {
	return &PlayerMoneyService{
		db: DB,
	}
}

// MoneyDataRequest represents the request payload for creating player money data
type MoneyDataRequest struct {
	Seed                     string      `json:"seed"`
	Timecode                 int64       `json:"timecode"`
	Money                    [8]int32    `json:"money"`
	MoneyEarned              [8]int32    `json:"money_earned"`
	UnitsBuilt               [8]int32    `json:"units_built"`
	UnitsLost                [8]int32    `json:"units_lost"`
	BuildingsBuilt           [8]int32    `json:"buildings_built"`
	BuildingsLost            [8]int32    `json:"buildings_lost"`
	BuildingsKilled          [8][8]int32 `json:"buildings_killed"`
	UnitsKilled              [8][8]int32 `json:"units_killed"`
	GeneralsPointsTotal      [8]int32    `json:"generals_points_total"`
	GeneralsPointsUsed       [8]int32    `json:"generals_points_used"`
	RadarsBuilt              [8]int32    `json:"radars_built"`
	SearchAndDestroy         [8]int32    `json:"search_and_destroy"`
	HoldTheLine              [8]int32    `json:"hold_the_line"`
	Bombardment              [8]int32    `json:"bombardment"`
	XP                       [8]int32    `json:"xp"`
	XPLevel                  [8]int32    `json:"xp_level"`
	TechBuildingsCaptured    [8]int32    `json:"tech_buildings_captured"`
	FactionBuildingsCaptured [8]int32    `json:"faction_buildings_captured"`
	PowerTotal               [8]int32    `json:"power_total"`
	PowerUsed                [8]int32    `json:"power_used"`
}

// CreatePlayerMoneyData creates a new player money data record or returns existing one
func (s *PlayerMoneyService) CreatePlayerMoneyData(req *MoneyDataRequest) (*PlayerMoneyData, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	// First, try to find existing record with the same seed and timecode
	var existingData PlayerMoneyData
	err := s.db.Where("seed = ? AND timecode = ?", req.Seed, int(req.Timecode)).First(&existingData).Error

	if err == nil {
		// Record already exists, return it without error
		return &existingData, nil
	}

	if err != gorm.ErrRecordNotFound {
		// Some other database error occurred
		return nil, fmt.Errorf("failed to check for existing player money data: %w", err)
	}

	// Convert 2D arrays to Int32Array8x8
	buildingsKilled := Int32Array8x8(req.BuildingsKilled)
	unitsKilled := Int32Array8x8(req.UnitsKilled)

	// Record doesn't exist, create new one
	playerMoneyData := &PlayerMoneyData{
		Seed:                     req.Seed,
		Timecode:                 int(req.Timecode),
		Player1Money:             int(req.Money[0]),
		Player2Money:             int(req.Money[1]),
		Player3Money:             int(req.Money[2]),
		Player4Money:             int(req.Money[3]),
		Player5Money:             int(req.Money[4]),
		Player6Money:             int(req.Money[5]),
		Player7Money:             int(req.Money[6]),
		Player8Money:             int(req.Money[7]),
		MoneyEarned:              Int32Array8(req.MoneyEarned),
		UnitsBuilt:               Int32Array8(req.UnitsBuilt),
		UnitsLost:                Int32Array8(req.UnitsLost),
		BuildingsBuilt:           Int32Array8(req.BuildingsBuilt),
		BuildingsLost:            Int32Array8(req.BuildingsLost),
		BuildingsKilled:          buildingsKilled,
		UnitsKilled:              unitsKilled,
		GeneralsPointsTotal:      Int32Array8(req.GeneralsPointsTotal),
		GeneralsPointsUsed:       Int32Array8(req.GeneralsPointsUsed),
		RadarsBuilt:              Int32Array8(req.RadarsBuilt),
		SearchAndDestroy:         Int32Array8(req.SearchAndDestroy),
		HoldTheLine:              Int32Array8(req.HoldTheLine),
		Bombardment:              Int32Array8(req.Bombardment),
		XP:                       Int32Array8(req.XP),
		XPLevel:                  Int32Array8(req.XPLevel),
		TechBuildingsCaptured:    Int32Array8(req.TechBuildingsCaptured),
		FactionBuildingsCaptured: Int32Array8(req.FactionBuildingsCaptured),
		PowerTotal:               Int32Array8(req.PowerTotal),
		PowerUsed:                Int32Array8(req.PowerUsed),
	}

	if err := s.db.Create(playerMoneyData).Error; err != nil {
		return nil, fmt.Errorf("failed to create player money data: %w", err)
	}

	return playerMoneyData, nil
}

// GetPlayerMoneyDataBySeed retrieves player money data by seed
func (s *PlayerMoneyService) GetPlayerMoneyDataBySeed(seed string) ([]*PlayerMoneyData, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	var results []*PlayerMoneyData
	if err := s.db.Where("seed = ?", seed).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get player money data by seed: %w", err)
	}

	return results, nil
}

// GetAllPlayerMoneyDataBySeed retrieves all player money data for a specific seed, ordered by timecode
func (s *PlayerMoneyService) GetAllPlayerMoneyDataBySeed(seed string) ([]*PlayerMoneyData, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	var results []*PlayerMoneyData
	if err := s.db.Where("seed = ?", seed).Order("timecode ASC").Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get all player money data by seed: %w", err)
	}

	return results, nil
}

// GetPlayerMoneyDataByTimecode retrieves player money data by timecode
func (s *PlayerMoneyService) GetPlayerMoneyDataByTimecode(timecode int) ([]*PlayerMoneyData, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	var results []*PlayerMoneyData
	if err := s.db.Where("timecode = ?", timecode).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get player money data by timecode: %w", err)
	}

	return results, nil
}

// GetPlayerMoneyDataByTimecodeAndSeed retrieves player money data by timecode and seed
func (s *PlayerMoneyService) GetPlayerMoneyDataByTimecodeAndSeed(timecode int, seed string) (*PlayerMoneyData, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	var result PlayerMoneyData
	if err := s.db.Where("timecode = ? AND seed = ?", timecode, seed).First(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No matching record found
		}
		return nil, fmt.Errorf("failed to get player money data by timecode and seed: %w", err)
	}

	return &result, nil
}

// GetAllPlayerMoneyData retrieves all player money data with pagination
func (s *PlayerMoneyService) GetAllPlayerMoneyData(limit, offset int) ([]*PlayerMoneyData, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	var results []*PlayerMoneyData
	query := s.db.Order("seed DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get all player money data: %w", err)
	}

	return results, nil
}

// DeletePlayerMoneyData deletes player money data by ID
func (s *PlayerMoneyService) DeletePlayerMoneyData(id uint) error {
	if s.db == nil {
		return fmt.Errorf("database not connected")
	}

	if err := s.db.Delete(&PlayerMoneyData{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete player money data: %w", err)
	}

	return nil
}
