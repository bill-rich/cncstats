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

// CreatePlayerMoneyDataRequest represents the request payload for creating player money data
type CreatePlayerMoneyDataRequest struct {
	Seed         string `json:"seed" binding:"required"`
	Timecode     int    `json:"timecode" binding:"required"`
	Player1Money int    `json:"player_1_money"`
	Player2Money int    `json:"player_2_money"`
	Player3Money int    `json:"player_3_money"`
	Player4Money int    `json:"player_4_money"`
	Player5Money int    `json:"player_5_money"`
	Player6Money int    `json:"player_6_money"`
	Player7Money int    `json:"player_7_money"`
	Player8Money int    `json:"player_8_money"`
}

// CreatePlayerMoneyData creates a new player money data record or returns existing one
func (s *PlayerMoneyService) CreatePlayerMoneyData(req *CreatePlayerMoneyDataRequest) (*PlayerMoneyData, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	// First, try to find existing record with the same seed and timecode
	var existingData PlayerMoneyData
	err := s.db.Where("seed = ? AND timecode = ?", req.Seed, req.Timecode).First(&existingData).Error

	if err == nil {
		// Record already exists, return it without error
		return &existingData, nil
	}

	if err != gorm.ErrRecordNotFound {
		// Some other database error occurred
		return nil, fmt.Errorf("failed to check for existing player money data: %w", err)
	}

	// Record doesn't exist, create new one
	playerMoneyData := &PlayerMoneyData{
		Seed:         req.Seed,
		Timecode:     req.Timecode,
		Player1Money: req.Player1Money,
		Player2Money: req.Player2Money,
		Player3Money: req.Player3Money,
		Player4Money: req.Player4Money,
		Player5Money: req.Player5Money,
		Player6Money: req.Player6Money,
		Player7Money: req.Player7Money,
		Player8Money: req.Player8Money,
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
