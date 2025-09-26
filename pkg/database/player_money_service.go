package database

import (
	"fmt"
	"time"

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
	TimestampBegin time.Time `json:"timestamp_begin" binding:"required"`
	Timecode       int64     `json:"timecode" binding:"required"`
	Player1Money   int64     `json:"player_1_money"`
	Player2Money   int64     `json:"player_2_money"`
	Player3Money   int64     `json:"player_3_money"`
	Player4Money   int64     `json:"player_4_money"`
	Player5Money   int64     `json:"player_5_money"`
	Player6Money   int64     `json:"player_6_money"`
	Player7Money   int64     `json:"player_7_money"`
	Player8Money   int64     `json:"player_8_money"`
}

// CreatePlayerMoneyData creates a new player money data record or returns existing one
func (s *PlayerMoneyService) CreatePlayerMoneyData(req *CreatePlayerMoneyDataRequest) (*PlayerMoneyData, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	// First, try to find existing record with the same timestamp_begin and timecode
	var existingData PlayerMoneyData
	err := s.db.Where("timestamp_begin = ? AND timecode = ?", req.TimestampBegin, req.Timecode).First(&existingData).Error

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
		TimestampBegin: req.TimestampBegin,
		Timecode:       req.Timecode,
		Player1Money:   req.Player1Money,
		Player2Money:   req.Player2Money,
		Player3Money:   req.Player3Money,
		Player4Money:   req.Player4Money,
		Player5Money:   req.Player5Money,
		Player6Money:   req.Player6Money,
		Player7Money:   req.Player7Money,
		Player8Money:   req.Player8Money,
	}

	if err := s.db.Create(playerMoneyData).Error; err != nil {
		return nil, fmt.Errorf("failed to create player money data: %w", err)
	}

	return playerMoneyData, nil
}

// GetPlayerMoneyDataByTimestamp retrieves player money data by timestamp
func (s *PlayerMoneyService) GetPlayerMoneyDataByTimestamp(timestamp time.Time) ([]*PlayerMoneyData, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	var results []*PlayerMoneyData
	if err := s.db.Where("timestamp_begin = ?", timestamp).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get player money data by timestamp: %w", err)
	}

	return results, nil
}

// GetPlayerMoneyDataByTimecode retrieves player money data by timecode
func (s *PlayerMoneyService) GetPlayerMoneyDataByTimecode(timecode int64) ([]*PlayerMoneyData, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	var results []*PlayerMoneyData
	if err := s.db.Where("timecode = ?", timecode).Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get player money data by timecode: %w", err)
	}

	return results, nil
}

// GetPlayerMoneyDataByTimecodeAndTimestamp retrieves player money data by timecode and timestamp
func (s *PlayerMoneyService) GetPlayerMoneyDataByTimecodeAndTimestamp(timecode int64, timestamp time.Time) (*PlayerMoneyData, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	var result PlayerMoneyData
	if err := s.db.Where("timecode = ? AND timestamp_begin = ?", timecode, timestamp).First(&result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // No matching record found
		}
		return nil, fmt.Errorf("failed to get player money data by timecode and timestamp: %w", err)
	}

	return &result, nil
}

// GetAllPlayerMoneyData retrieves all player money data with pagination
func (s *PlayerMoneyService) GetAllPlayerMoneyData(limit, offset int) ([]*PlayerMoneyData, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	var results []*PlayerMoneyData
	query := s.db.Order("timestamp_begin DESC")

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
