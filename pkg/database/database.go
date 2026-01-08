package database

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the global database instance
var DB *gorm.DB

// Int32Array8 represents an array of 8 int32 values for JSON storage
type Int32Array8 [8]int32

// Value implements the driver.Valuer interface
func (a Int32Array8) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Scan implements the sql.Scanner interface
func (a *Int32Array8) Scan(value interface{}) error {
	if value == nil {
		*a = Int32Array8{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal Int32Array8 value: %v", value)
	}
	return json.Unmarshal(bytes, a)
}

// Int32Array8x8 represents a 2D array of 8x8 int32 values for JSON storage
type Int32Array8x8 [8][8]int32

// Value implements the driver.Valuer interface
func (a Int32Array8x8) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Scan implements the sql.Scanner interface
func (a *Int32Array8x8) Scan(value interface{}) error {
	if value == nil {
		*a = Int32Array8x8{}
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal Int32Array8x8 value: %v", value)
	}
	return json.Unmarshal(bytes, a)
}

// NullableInt32Array8 represents a nullable Int32Array8 for database storage
type NullableInt32Array8 struct {
	Int32Array8
	Valid bool
}

// Value implements the driver.Valuer interface
func (n NullableInt32Array8) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return json.Marshal(n.Int32Array8)
}

// Scan implements the sql.Scanner interface
func (n *NullableInt32Array8) Scan(value interface{}) error {
	if value == nil {
		n.Int32Array8 = Int32Array8{}
		n.Valid = false
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal NullableInt32Array8 value: %v", value)
	}
	n.Valid = true
	return json.Unmarshal(bytes, &n.Int32Array8)
}

// NullableInt32Array8x8 represents a nullable Int32Array8x8 for database storage
type NullableInt32Array8x8 struct {
	Int32Array8x8
	Valid bool
}

// Value implements the driver.Valuer interface
func (n NullableInt32Array8x8) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return json.Marshal(n.Int32Array8x8)
}

// Scan implements the sql.Scanner interface
func (n *NullableInt32Array8x8) Scan(value interface{}) error {
	if value == nil {
		n.Int32Array8x8 = Int32Array8x8{}
		n.Valid = false
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return fmt.Errorf("failed to unmarshal NullableInt32Array8x8 value: %v", value)
	}
	n.Valid = true
	return json.Unmarshal(bytes, &n.Int32Array8x8)
}

// PlayerMoneyData represents the money data for players at a specific seed
type PlayerMoneyData struct {
	ID                       uint                  `gorm:"primaryKey" json:"id"`
	Seed                     string                `gorm:"not null;uniqueIndex:idx_seed_timecode" json:"seed"`
	Timecode                 int                   `gorm:"not null;uniqueIndex:idx_seed_timecode" json:"timecode"`
	Player1Money             int                   `gorm:"default:0" json:"player_1_money"`
	Player2Money             int                   `gorm:"default:0" json:"player_2_money"`
	Player3Money             int                   `gorm:"default:0" json:"player_3_money"`
	Player4Money             int                   `gorm:"default:0" json:"player_4_money"`
	Player5Money             int                   `gorm:"default:0" json:"player_5_money"`
	Player6Money             int                   `gorm:"default:0" json:"player_6_money"`
	Player7Money             int                   `gorm:"default:0" json:"player_7_money"`
	Player8Money             int                   `gorm:"default:0" json:"player_8_money"`
	MoneyEarned              NullableInt32Array8   `gorm:"type:jsonb" json:"money_earned"`
	UnitsBuilt               NullableInt32Array8   `gorm:"type:jsonb" json:"units_built"`
	UnitsLost                NullableInt32Array8   `gorm:"type:jsonb" json:"units_lost"`
	BuildingsBuilt           NullableInt32Array8   `gorm:"type:jsonb" json:"buildings_built"`
	BuildingsLost            NullableInt32Array8   `gorm:"type:jsonb" json:"buildings_lost"`
	BuildingsKilled          NullableInt32Array8x8 `gorm:"type:jsonb" json:"buildings_killed"`
	UnitsKilled              NullableInt32Array8x8 `gorm:"type:jsonb" json:"units_killed"`
	GeneralsPointsTotal      NullableInt32Array8   `gorm:"type:jsonb" json:"generals_points_total"`
	GeneralsPointsUsed       NullableInt32Array8   `gorm:"type:jsonb" json:"generals_points_used"`
	RadarsBuilt              NullableInt32Array8   `gorm:"type:jsonb" json:"radars_built"`
	SearchAndDestroy         NullableInt32Array8   `gorm:"type:jsonb" json:"search_and_destroy"`
	HoldTheLine              NullableInt32Array8   `gorm:"type:jsonb" json:"hold_the_line"`
	Bombardment              NullableInt32Array8   `gorm:"type:jsonb" json:"bombardment"`
	XP                       NullableInt32Array8   `gorm:"type:jsonb" json:"xp"`
	XPLevel                  NullableInt32Array8   `gorm:"type:jsonb" json:"xp_level"`
	TechBuildingsCaptured    NullableInt32Array8   `gorm:"type:jsonb" json:"tech_buildings_captured"`
	FactionBuildingsCaptured NullableInt32Array8   `gorm:"type:jsonb" json:"faction_buildings_captured"`
	PowerTotal               NullableInt32Array8   `gorm:"type:jsonb" json:"power_total"`
	PowerUsed                NullableInt32Array8   `gorm:"type:jsonb" json:"power_used"`
	CreatedAt                time.Time             `json:"created_at"`
	UpdatedAt                time.Time             `json:"updated_at"`
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
