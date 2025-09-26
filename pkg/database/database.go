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

// PlayerMoneyData represents the money data for players at a specific timestamp
type PlayerMoneyData struct {
	ID             uint      `gorm:"primaryKey" json:"id"`
	TimestampBegin int64     `gorm:"not null;uniqueIndex:idx_timestamp_timecode" json:"timestamp_begin"`
	Timecode       int64     `gorm:"not null;uniqueIndex:idx_timestamp_timecode" json:"timecode"`
	Player1Money   int64     `gorm:"default:0" json:"player_1_money"`
	Player2Money   int64     `gorm:"default:0" json:"player_2_money"`
	Player3Money   int64     `gorm:"default:0" json:"player_3_money"`
	Player4Money   int64     `gorm:"default:0" json:"player_4_money"`
	Player5Money   int64     `gorm:"default:0" json:"player_5_money"`
	Player6Money   int64     `gorm:"default:0" json:"player_6_money"`
	Player7Money   int64     `gorm:"default:0" json:"player_7_money"`
	Player8Money   int64     `gorm:"default:0" json:"player_8_money"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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

	// Check if table exists and what the current schema looks like
	var tableExists bool
	err := DB.Raw(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'player_money_data'
		)
	`).Scan(&tableExists).Error

	if err != nil {
		return fmt.Errorf("failed to check if table exists: %w", err)
	}

	if tableExists {
		// Table exists, check if we need to migrate the timestamp_begin column
		var columnType string
		err = DB.Raw(`
			SELECT data_type 
			FROM information_schema.columns 
			WHERE table_name = 'player_money_data' 
			AND column_name = 'timestamp_begin'
		`).Scan(&columnType).Error

		if err != nil {
			return fmt.Errorf("failed to check column type: %w", err)
		}

		if columnType != "bigint" {
			// Table exists with old schema, we need to handle this carefully
			// Don't use AutoMigrate as it will fail with incompatible types
			// The timestamp migration will handle this
			return nil
		}
	}

	// Either table doesn't exist or has correct schema, safe to use AutoMigrate
	err = DB.AutoMigrate(&PlayerMoneyData{})
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
