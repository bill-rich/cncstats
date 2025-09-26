package database

import (
	"fmt"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global database instance
var DB *gorm.DB

// PlayerMoneyData represents the money data for players at a specific seed
type PlayerMoneyData struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Seed         string    `gorm:"not null;uniqueIndex:idx_seed_timecode" json:"seed"`
	Timecode     int       `gorm:"not null;uniqueIndex:idx_seed_timecode" json:"timecode"`
	Player1Money int       `gorm:"default:0" json:"player_1_money"`
	Player2Money int       `gorm:"default:0" json:"player_2_money"`
	Player3Money int       `gorm:"default:0" json:"player_3_money"`
	Player4Money int       `gorm:"default:0" json:"player_4_money"`
	Player5Money int       `gorm:"default:0" json:"player_5_money"`
	Player6Money int       `gorm:"default:0" json:"player_6_money"`
	Player7Money int       `gorm:"default:0" json:"player_7_money"`
	Player8Money int       `gorm:"default:0" json:"player_8_money"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Connect initializes the database connection
func Connect() error {
	// Get database URL from environment variable
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		// For local development, construct from individual components
		host := getEnvOrDefault("DB_HOST", "localhost")
		port := getEnvOrDefault("DB_PORT", "5432")
		user := getEnvOrDefault("DB_USER", "postgres")
		password := getEnvOrDefault("DB_PASSWORD", "postgres")
		dbname := getEnvOrDefault("DB_NAME", "cncstats")

		databaseURL = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
			host, port, user, password, dbname)
	}

	// Configure GORM logger
	var gormLogger logger.Interface
	if os.Getenv("GORM_LOG_LEVEL") == "debug" {
		gormLogger = logger.Default.LogMode(logger.Info)
	} else {
		gormLogger = logger.Default.LogMode(logger.Silent)
	}

	// Connect to database
	db, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	DB = db
	return nil
}

// Migrate runs database migrations
func Migrate() error {
	if DB == nil {
		return fmt.Errorf("database not connected")
	}

	// Auto-migrate the schema
	err := DB.AutoMigrate(&PlayerMoneyData{})
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}

	return nil
}

// Close closes the database connection
func Close() error {
	if DB == nil {
		return nil
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return err
	}

	return sqlDB.Close()
}

// getEnvOrDefault returns the environment variable value or a default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
