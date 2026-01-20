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

	// Handle Money array - update player_money field only if not all zeros
	if req.Money != nil && !isAllZerosInt32Array8(*req.Money) {
		updateMap["PlayerMoney"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.Money), Valid: true}
	}

	// Handle all other optional fields - only update if provided
	if req.MoneyEarned != nil {
		updateMap["MoneyEarned"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.MoneyEarned), Valid: true}
	}
	if req.UnitsBuilt != nil {
		updateMap["UnitsBuilt"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.UnitsBuilt), Valid: true}
	}
	if req.UnitsLost != nil {
		updateMap["UnitsLost"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.UnitsLost), Valid: true}
	}
	if req.BuildingsBuilt != nil {
		updateMap["BuildingsBuilt"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.BuildingsBuilt), Valid: true}
	}
	if req.BuildingsLost != nil {
		updateMap["BuildingsLost"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.BuildingsLost), Valid: true}
	}
	if req.BuildingsKilled != nil {
		updateMap["BuildingsKilled"] = NullableInt32Array8x8{Int32Array8x8: Int32Array8x8(*req.BuildingsKilled), Valid: true}
	}
	if req.UnitsKilled != nil {
		updateMap["UnitsKilled"] = NullableInt32Array8x8{Int32Array8x8: Int32Array8x8(*req.UnitsKilled), Valid: true}
	}
	if req.GeneralsPointsTotal != nil {
		updateMap["GeneralsPointsTotal"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.GeneralsPointsTotal), Valid: true}
	}
	if req.GeneralsPointsUsed != nil {
		updateMap["GeneralsPointsUsed"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.GeneralsPointsUsed), Valid: true}
	}
	if req.RadarsBuilt != nil {
		updateMap["RadarsBuilt"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.RadarsBuilt), Valid: true}
	}
	if req.SearchAndDestroy != nil {
		updateMap["SearchAndDestroy"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.SearchAndDestroy), Valid: true}
	}
	if req.HoldTheLine != nil {
		updateMap["HoldTheLine"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.HoldTheLine), Valid: true}
	}
	if req.Bombardment != nil {
		updateMap["Bombardment"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.Bombardment), Valid: true}
	}
	if req.XP != nil {
		updateMap["XP"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.XP), Valid: true}
	}
	if req.XPLevel != nil {
		updateMap["XPLevel"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.XPLevel), Valid: true}
	}
	if req.TechBuildingsCaptured != nil {
		updateMap["TechBuildingsCaptured"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.TechBuildingsCaptured), Valid: true}
	}
	if req.FactionBuildingsCaptured != nil {
		updateMap["FactionBuildingsCaptured"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.FactionBuildingsCaptured), Valid: true}
	}
	if req.PowerTotal != nil {
		updateMap["PowerTotal"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.PowerTotal), Valid: true}
	}
	if req.PowerUsed != nil {
		updateMap["PowerUsed"] = NullableInt32Array8{Int32Array8: Int32Array8(*req.PowerUsed), Valid: true}
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

	// Set money field if provided and not all zeros
	if req.Money != nil && !isAllZerosInt32Array8(*req.Money) {
		playerMoneyData.PlayerMoney = NullableInt32Array8{Int32Array8: Int32Array8(*req.Money), Valid: true}
	}

	// Set other fields if provided
	if req.MoneyEarned != nil {
		playerMoneyData.MoneyEarned = NullableInt32Array8{Int32Array8: Int32Array8(*req.MoneyEarned), Valid: true}
	}
	if req.UnitsBuilt != nil {
		playerMoneyData.UnitsBuilt = NullableInt32Array8{Int32Array8: Int32Array8(*req.UnitsBuilt), Valid: true}
	}
	if req.UnitsLost != nil {
		playerMoneyData.UnitsLost = NullableInt32Array8{Int32Array8: Int32Array8(*req.UnitsLost), Valid: true}
	}
	if req.BuildingsBuilt != nil {
		playerMoneyData.BuildingsBuilt = NullableInt32Array8{Int32Array8: Int32Array8(*req.BuildingsBuilt), Valid: true}
	}
	if req.BuildingsLost != nil {
		playerMoneyData.BuildingsLost = NullableInt32Array8{Int32Array8: Int32Array8(*req.BuildingsLost), Valid: true}
	}
	if req.BuildingsKilled != nil {
		playerMoneyData.BuildingsKilled = NullableInt32Array8x8{Int32Array8x8: Int32Array8x8(*req.BuildingsKilled), Valid: true}
	}
	if req.UnitsKilled != nil {
		playerMoneyData.UnitsKilled = NullableInt32Array8x8{Int32Array8x8: Int32Array8x8(*req.UnitsKilled), Valid: true}
	}
	if req.GeneralsPointsTotal != nil {
		playerMoneyData.GeneralsPointsTotal = NullableInt32Array8{Int32Array8: Int32Array8(*req.GeneralsPointsTotal), Valid: true}
	}
	if req.GeneralsPointsUsed != nil {
		playerMoneyData.GeneralsPointsUsed = NullableInt32Array8{Int32Array8: Int32Array8(*req.GeneralsPointsUsed), Valid: true}
	}
	if req.RadarsBuilt != nil {
		playerMoneyData.RadarsBuilt = NullableInt32Array8{Int32Array8: Int32Array8(*req.RadarsBuilt), Valid: true}
	}
	if req.SearchAndDestroy != nil {
		playerMoneyData.SearchAndDestroy = NullableInt32Array8{Int32Array8: Int32Array8(*req.SearchAndDestroy), Valid: true}
	}
	if req.HoldTheLine != nil {
		playerMoneyData.HoldTheLine = NullableInt32Array8{Int32Array8: Int32Array8(*req.HoldTheLine), Valid: true}
	}
	if req.Bombardment != nil {
		playerMoneyData.Bombardment = NullableInt32Array8{Int32Array8: Int32Array8(*req.Bombardment), Valid: true}
	}
	if req.XP != nil {
		playerMoneyData.XP = NullableInt32Array8{Int32Array8: Int32Array8(*req.XP), Valid: true}
	}
	if req.XPLevel != nil {
		playerMoneyData.XPLevel = NullableInt32Array8{Int32Array8: Int32Array8(*req.XPLevel), Valid: true}
	}
	if req.TechBuildingsCaptured != nil {
		playerMoneyData.TechBuildingsCaptured = NullableInt32Array8{Int32Array8: Int32Array8(*req.TechBuildingsCaptured), Valid: true}
	}
	if req.FactionBuildingsCaptured != nil {
		playerMoneyData.FactionBuildingsCaptured = NullableInt32Array8{Int32Array8: Int32Array8(*req.FactionBuildingsCaptured), Valid: true}
	}
	if req.PowerTotal != nil {
		playerMoneyData.PowerTotal = NullableInt32Array8{Int32Array8: Int32Array8(*req.PowerTotal), Valid: true}
	}
	if req.PowerUsed != nil {
		playerMoneyData.PowerUsed = NullableInt32Array8{Int32Array8: Int32Array8(*req.PowerUsed), Valid: true}
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

// DeletePlayerMoneyDataBySeed deletes all player money data for a specific seed
func (s *PlayerMoneyService) DeletePlayerMoneyDataBySeed(seed string) error {
	if s.db == nil {
		return fmt.Errorf("database not connected")
	}

	if err := s.db.Where("seed = ?", seed).Delete(&PlayerMoneyData{}).Error; err != nil {
		return fmt.Errorf("failed to delete player money data by seed: %w", err)
	}

	return nil
}

// HasDataForSeed checks if there is any data in the database for the given seed
func (s *PlayerMoneyService) HasDataForSeed(seed string) (bool, error) {
	if s.db == nil {
		return false, fmt.Errorf("database not connected")
	}

	var count int64
	if err := s.db.Model(&PlayerMoneyData{}).Where("seed = ?", seed).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check for existing data: %w", err)
	}

	return count > 0, nil
}

// isAllZerosInt32Array8 checks if all values in an [8]int32 array are zero
func isAllZerosInt32Array8(arr [8]int32) bool {
	for _, v := range arr {
		if v != 0 {
			return false
		}
	}
	return true
}
