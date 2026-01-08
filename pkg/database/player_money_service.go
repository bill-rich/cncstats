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
	Seed                     string       `json:"seed"`
	Timecode                 int64        `json:"timecode"`
	Money                    *[8]int32    `json:"money,omitempty"`
	MoneyEarned              *[8]int32    `json:"money_earned,omitempty"`
	UnitsBuilt               *[8]int32    `json:"units_built,omitempty"`
	UnitsLost                *[8]int32    `json:"units_lost,omitempty"`
	BuildingsBuilt           *[8]int32    `json:"buildings_built,omitempty"`
	BuildingsLost            *[8]int32    `json:"buildings_lost,omitempty"`
	BuildingsKilled          *[8][8]int32 `json:"buildings_killed,omitempty"`
	UnitsKilled              *[8][8]int32 `json:"units_killed,omitempty"`
	GeneralsPointsTotal      *[8]int32    `json:"generals_points_total,omitempty"`
	GeneralsPointsUsed       *[8]int32    `json:"generals_points_used,omitempty"`
	RadarsBuilt              *[8]int32    `json:"radars_built,omitempty"`
	SearchAndDestroy         *[8]int32    `json:"search_and_destroy,omitempty"`
	HoldTheLine              *[8]int32    `json:"hold_the_line,omitempty"`
	Bombardment              *[8]int32    `json:"bombardment,omitempty"`
	XP                       *[8]int32    `json:"xp,omitempty"`
	XPLevel                  *[8]int32    `json:"xp_level,omitempty"`
	TechBuildingsCaptured    *[8]int32    `json:"tech_buildings_captured,omitempty"`
	FactionBuildingsCaptured *[8]int32    `json:"faction_buildings_captured,omitempty"`
	PowerTotal               *[8]int32    `json:"power_total,omitempty"`
	PowerUsed                *[8]int32    `json:"power_used,omitempty"`
}

// CreatePlayerMoneyData creates a new player money data record or updates existing one with partial data
func (s *PlayerMoneyService) CreatePlayerMoneyData(req *MoneyDataRequest) (*PlayerMoneyData, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	// First, try to find existing record with the same seed and timecode
	var existingData PlayerMoneyData
	err := s.db.Where("seed = ? AND timecode = ?", req.Seed, int(req.Timecode)).First(&existingData).Error

	updateMap := make(map[string]interface{})

	// Handle Money array - update individual player money fields
	if req.Money != nil {
		updateMap["Player1Money"] = int(req.Money[0])
		updateMap["Player2Money"] = int(req.Money[1])
		updateMap["Player3Money"] = int(req.Money[2])
		updateMap["Player4Money"] = int(req.Money[3])
		updateMap["Player5Money"] = int(req.Money[4])
		updateMap["Player6Money"] = int(req.Money[5])
		updateMap["Player7Money"] = int(req.Money[6])
		updateMap["Player8Money"] = int(req.Money[7])
	}

	// Handle all other optional fields - only update if provided
	if req.MoneyEarned != nil {
		updateMap["MoneyEarned"] = Int32Array8(*req.MoneyEarned)
	}
	if req.UnitsBuilt != nil {
		updateMap["UnitsBuilt"] = Int32Array8(*req.UnitsBuilt)
	}
	if req.UnitsLost != nil {
		updateMap["UnitsLost"] = Int32Array8(*req.UnitsLost)
	}
	if req.BuildingsBuilt != nil {
		updateMap["BuildingsBuilt"] = Int32Array8(*req.BuildingsBuilt)
	}
	if req.BuildingsLost != nil {
		updateMap["BuildingsLost"] = Int32Array8(*req.BuildingsLost)
	}
	if req.BuildingsKilled != nil {
		updateMap["BuildingsKilled"] = Int32Array8x8(*req.BuildingsKilled)
	}
	if req.UnitsKilled != nil {
		updateMap["UnitsKilled"] = Int32Array8x8(*req.UnitsKilled)
	}
	if req.GeneralsPointsTotal != nil {
		updateMap["GeneralsPointsTotal"] = Int32Array8(*req.GeneralsPointsTotal)
	}
	if req.GeneralsPointsUsed != nil {
		updateMap["GeneralsPointsUsed"] = Int32Array8(*req.GeneralsPointsUsed)
	}
	if req.RadarsBuilt != nil {
		updateMap["RadarsBuilt"] = Int32Array8(*req.RadarsBuilt)
	}
	if req.SearchAndDestroy != nil {
		updateMap["SearchAndDestroy"] = Int32Array8(*req.SearchAndDestroy)
	}
	if req.HoldTheLine != nil {
		updateMap["HoldTheLine"] = Int32Array8(*req.HoldTheLine)
	}
	if req.Bombardment != nil {
		updateMap["Bombardment"] = Int32Array8(*req.Bombardment)
	}
	if req.XP != nil {
		updateMap["XP"] = Int32Array8(*req.XP)
	}
	if req.XPLevel != nil {
		updateMap["XPLevel"] = Int32Array8(*req.XPLevel)
	}
	if req.TechBuildingsCaptured != nil {
		updateMap["TechBuildingsCaptured"] = Int32Array8(*req.TechBuildingsCaptured)
	}
	if req.FactionBuildingsCaptured != nil {
		updateMap["FactionBuildingsCaptured"] = Int32Array8(*req.FactionBuildingsCaptured)
	}
	if req.PowerTotal != nil {
		updateMap["PowerTotal"] = Int32Array8(*req.PowerTotal)
	}
	if req.PowerUsed != nil {
		updateMap["PowerUsed"] = Int32Array8(*req.PowerUsed)
	}

	if err == nil {
		// Record already exists, update only the provided fields
		if len(updateMap) > 0 {
			if err := s.db.Model(&existingData).Updates(updateMap).Error; err != nil {
				return nil, fmt.Errorf("failed to update player money data: %w", err)
			}
			// Reload the record to get updated values
			if err := s.db.Where("seed = ? AND timecode = ?", req.Seed, int(req.Timecode)).First(&existingData).Error; err != nil {
				return nil, fmt.Errorf("failed to reload updated player money data: %w", err)
			}
		}
		return &existingData, nil
	}

	if err != gorm.ErrRecordNotFound {
		// Some other database error occurred
		return nil, fmt.Errorf("failed to check for existing player money data: %w", err)
	}

	// Record doesn't exist, create new one with only provided fields
	playerMoneyData := &PlayerMoneyData{
		Seed:     req.Seed,
		Timecode: int(req.Timecode),
	}

	// Set money fields if provided
	if req.Money != nil {
		playerMoneyData.Player1Money = int(req.Money[0])
		playerMoneyData.Player2Money = int(req.Money[1])
		playerMoneyData.Player3Money = int(req.Money[2])
		playerMoneyData.Player4Money = int(req.Money[3])
		playerMoneyData.Player5Money = int(req.Money[4])
		playerMoneyData.Player6Money = int(req.Money[5])
		playerMoneyData.Player7Money = int(req.Money[6])
		playerMoneyData.Player8Money = int(req.Money[7])
	}

	// Set other fields if provided
	if req.MoneyEarned != nil {
		playerMoneyData.MoneyEarned = Int32Array8(*req.MoneyEarned)
	}
	if req.UnitsBuilt != nil {
		playerMoneyData.UnitsBuilt = Int32Array8(*req.UnitsBuilt)
	}
	if req.UnitsLost != nil {
		playerMoneyData.UnitsLost = Int32Array8(*req.UnitsLost)
	}
	if req.BuildingsBuilt != nil {
		playerMoneyData.BuildingsBuilt = Int32Array8(*req.BuildingsBuilt)
	}
	if req.BuildingsLost != nil {
		playerMoneyData.BuildingsLost = Int32Array8(*req.BuildingsLost)
	}
	if req.BuildingsKilled != nil {
		playerMoneyData.BuildingsKilled = Int32Array8x8(*req.BuildingsKilled)
	}
	if req.UnitsKilled != nil {
		playerMoneyData.UnitsKilled = Int32Array8x8(*req.UnitsKilled)
	}
	if req.GeneralsPointsTotal != nil {
		playerMoneyData.GeneralsPointsTotal = Int32Array8(*req.GeneralsPointsTotal)
	}
	if req.GeneralsPointsUsed != nil {
		playerMoneyData.GeneralsPointsUsed = Int32Array8(*req.GeneralsPointsUsed)
	}
	if req.RadarsBuilt != nil {
		playerMoneyData.RadarsBuilt = Int32Array8(*req.RadarsBuilt)
	}
	if req.SearchAndDestroy != nil {
		playerMoneyData.SearchAndDestroy = Int32Array8(*req.SearchAndDestroy)
	}
	if req.HoldTheLine != nil {
		playerMoneyData.HoldTheLine = Int32Array8(*req.HoldTheLine)
	}
	if req.Bombardment != nil {
		playerMoneyData.Bombardment = Int32Array8(*req.Bombardment)
	}
	if req.XP != nil {
		playerMoneyData.XP = Int32Array8(*req.XP)
	}
	if req.XPLevel != nil {
		playerMoneyData.XPLevel = Int32Array8(*req.XPLevel)
	}
	if req.TechBuildingsCaptured != nil {
		playerMoneyData.TechBuildingsCaptured = Int32Array8(*req.TechBuildingsCaptured)
	}
	if req.FactionBuildingsCaptured != nil {
		playerMoneyData.FactionBuildingsCaptured = Int32Array8(*req.FactionBuildingsCaptured)
	}
	if req.PowerTotal != nil {
		playerMoneyData.PowerTotal = Int32Array8(*req.PowerTotal)
	}
	if req.PowerUsed != nil {
		playerMoneyData.PowerUsed = Int32Array8(*req.PowerUsed)
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
