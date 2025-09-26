package database

import (
	"fmt"
)

// MigrateTimestampBeginToInt64 handles the migration of timestamp_begin from time.Time to int64
// This migration handles timestamp with time zone conversion issues on Heroku PostgreSQL
func MigrateTimestampBeginToInt64() error {
	if DB == nil {
		return fmt.Errorf("database not connected")
	}

	// Check if table exists
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

	if !tableExists {
		// Table doesn't exist yet, AutoMigrate will create it with the correct schema
		return nil
	}

	// Check if migration is already done
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

	if columnType == "bigint" {
		// Migration already completed or table was created with correct schema
		return nil
	}

	// Start a transaction
	tx := DB.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to start transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Step 1: Add temporary column
	if err := tx.Exec("ALTER TABLE player_money_data ADD COLUMN timestamp_begin_int64 BIGINT").Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to add temporary column: %w", err)
	}

	// Step 2: Convert data using the most compatible approach for timestamptz
	// This approach handles the specific casting issue on Heroku PostgreSQL
	updateErr := tx.Exec(`
		UPDATE player_money_data 
		SET timestamp_begin_int64 = (EXTRACT(EPOCH FROM timestamp_begin AT TIME ZONE 'UTC') * 1000000000)::BIGINT
	`).Error

	if updateErr != nil {
		// If that fails, try without timezone conversion
		updateErr = tx.Exec(`
			UPDATE player_money_data 
			SET timestamp_begin_int64 = (EXTRACT(EPOCH FROM timestamp_begin) * 1000000000)::BIGINT
		`).Error
	}

	if updateErr != nil {
		// Last resort: convert to text first, then to timestamp, then extract epoch
		updateErr = tx.Exec(`
			UPDATE player_money_data 
			SET timestamp_begin_int64 = (EXTRACT(EPOCH FROM timestamp_begin::text::timestamp) * 1000000000)::BIGINT
		`).Error
	}

	if updateErr != nil {
		tx.Rollback()
		return fmt.Errorf("failed to convert timestamp data: %w", updateErr)
	}

	// Step 3: Drop the unique index
	if err := tx.Exec("DROP INDEX IF EXISTS idx_timestamp_timecode").Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to drop old index: %w", err)
	}

	// Step 4: Drop the old column
	if err := tx.Exec("ALTER TABLE player_money_data DROP COLUMN timestamp_begin").Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to drop old column: %w", err)
	}

	// Step 5: Rename the new column
	if err := tx.Exec("ALTER TABLE player_money_data RENAME COLUMN timestamp_begin_int64 TO timestamp_begin").Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to rename column: %w", err)
	}

	// Step 6: Set NOT NULL constraint
	if err := tx.Exec("ALTER TABLE player_money_data ALTER COLUMN timestamp_begin SET NOT NULL").Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to set NOT NULL constraint: %w", err)
	}

	// Step 7: Recreate the unique index
	if err := tx.Exec("CREATE UNIQUE INDEX idx_timestamp_timecode ON player_money_data (timestamp_begin, timecode)").Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to recreate index: %w", err)
	}

	// Commit the transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
