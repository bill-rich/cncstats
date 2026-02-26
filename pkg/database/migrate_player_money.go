package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
)

// MigratePlayerMoneyToArray migrates playerX_money columns to a single player_money array column
func MigratePlayerMoneyToArray() error {
	if DB == nil {
		return fmt.Errorf("database not connected")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	log.Println("Starting migration: Combining playerX_money columns into player_money array...")

	// Step 1: Check if player_money column already exists
	var columnExists bool
	err = sqlDB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.columns 
			WHERE table_name = 'player_money_data' 
			AND column_name = 'player_money'
		)
	`).Scan(&columnExists)
	if err != nil {
		return fmt.Errorf("failed to check if player_money column exists: %w", err)
	}

	if columnExists {
		log.Println("player_money column already exists, checking if migration is needed...")
		// Check if old columns still exist
		var oldColumnsExist bool
		err = sqlDB.QueryRow(`
			SELECT EXISTS (
				SELECT 1 
				FROM information_schema.columns 
				WHERE table_name = 'player_money_data' 
				AND column_name = 'player1_money'
			)
		`).Scan(&oldColumnsExist)
		if err != nil {
			return fmt.Errorf("failed to check if old columns exist: %w", err)
		}

		if !oldColumnsExist {
			log.Println("Migration already completed - old columns removed")
			return nil
		}

		log.Println("player_money column exists but old columns still present, migrating data...")
	} else {
		// Step 2: Add the new player_money column
		log.Println("Adding player_money column...")
		_, err = sqlDB.Exec(`
			ALTER TABLE player_money_data 
			ADD COLUMN player_money JSONB
		`)
		if err != nil {
			return fmt.Errorf("failed to add player_money column: %w", err)
		}
		log.Println("player_money column added successfully")
	}

	// Step 3: Migrate existing data from individual columns to array
	log.Println("Migrating data from individual columns to array...")
	rows, err := sqlDB.Query(`
		SELECT id, player1_money, player2_money, player3_money, player4_money,
		       player5_money, player6_money, player7_money, player8_money
		FROM player_money_data
		WHERE player_money IS NULL
	`)
	if err != nil {
		return fmt.Errorf("failed to query existing data: %w", err)
	}
	defer rows.Close()

	var migratedCount int
	for rows.Next() {
		var id uint
		var p1, p2, p3, p4, p5, p6, p7, p8 sql.NullInt64

		err := rows.Scan(&id, &p1, &p2, &p3, &p4, &p5, &p6, &p7, &p8)
		if err != nil {
			log.Printf("Warning: failed to scan row %d: %v", id, err)
			continue
		}

		// Convert to int32 array
		moneyArray := [8]int32{
			int32(getIntValue(p1)),
			int32(getIntValue(p2)),
			int32(getIntValue(p3)),
			int32(getIntValue(p4)),
			int32(getIntValue(p5)),
			int32(getIntValue(p6)),
			int32(getIntValue(p7)),
			int32(getIntValue(p8)),
		}

		// Marshal to JSON
		jsonData, err := json.Marshal(moneyArray)
		if err != nil {
			log.Printf("Warning: failed to marshal money array for row %d: %v", id, err)
			continue
		}

		// Update the row
		_, err = sqlDB.Exec(`
			UPDATE player_money_data 
			SET player_money = $1::jsonb
			WHERE id = $2
		`, jsonData, id)
		if err != nil {
			log.Printf("Warning: failed to update row %d: %v", id, err)
			continue
		}

		migratedCount++
		if migratedCount%100 == 0 {
			log.Printf("Migrated %d rows...", migratedCount)
		}
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("error iterating rows: %w", err)
	}

	log.Printf("Successfully migrated %d rows", migratedCount)

	// Step 4: Drop the old columns
	log.Println("Dropping old playerX_money columns...")
	dropColumns := []string{
		"player1_money",
		"player2_money",
		"player3_money",
		"player4_money",
		"player5_money",
		"player6_money",
		"player7_money",
		"player8_money",
	}

	for _, col := range dropColumns {
		_, err = sqlDB.Exec(fmt.Sprintf(`
			ALTER TABLE player_money_data 
			DROP COLUMN IF EXISTS %s
		`, col))
		if err != nil {
			return fmt.Errorf("failed to drop column %s: %w", col, err)
		}
		log.Printf("Dropped column %s", col)
	}

	log.Println("Migration completed successfully!")
	return nil
}

// MigrateSeedIndex adds an index on the seed column for faster lookups
func MigrateSeedIndex() error {
	if DB == nil {
		return fmt.Errorf("database not connected")
	}

	sqlDB, err := DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	log.Println("Checking if seed index exists...")

	// Check if index already exists
	var indexExists bool
	err = sqlDB.QueryRow(`
		SELECT EXISTS (
			SELECT 1 
			FROM pg_indexes 
			WHERE tablename = 'player_money_data' 
			AND indexname = 'idx_seed'
		)
	`).Scan(&indexExists)
	if err != nil {
		return fmt.Errorf("failed to check if seed index exists: %w", err)
	}

	if indexExists {
		log.Println("Seed index already exists")
		return nil
	}

	// Create the index
	log.Println("Creating index on seed column...")
	_, err = sqlDB.Exec(`
		CREATE INDEX idx_seed ON player_money_data(seed)
	`)
	if err != nil {
		return fmt.Errorf("failed to create seed index: %w", err)
	}

	log.Println("Seed index created successfully!")
	return nil
}

// getIntValue safely extracts int value from sql.NullInt64
func getIntValue(n sql.NullInt64) int {
	if n.Valid {
		return int(n.Int64)
	}
	return 0
}
