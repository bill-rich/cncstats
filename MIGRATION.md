# Database Migration: TimestampBegin Field

This document describes the migration of the `timestamp_begin` field from `time.Time` to `int64` in the `player_money_data` table.

## Overview

The `timestamp_begin` field in the `PlayerMoneyData` struct has been migrated from `time.Time` to `int64` to store Unix nanoseconds for better performance and consistency.

## Changes Made

### 1. Database Schema Changes
- **Before**: `timestamp_begin TIMESTAMP`
- **After**: `timestamp_begin BIGINT`

### 2. Code Changes
- Updated `PlayerMoneyData` struct to use `int64` for `TimestampBegin`
- Updated `CreatePlayerMoneyDataRequest` struct to use `int64` for `TimestampBegin`
- Updated service methods to handle `int64` timestamps
- Updated `enhanced_replay.go` to convert timestamps to int64 nanoseconds

### 3. Migration Process
The migration script (`pkg/database/migration.go`) performs the following steps:
1. Adds a temporary `timestamp_begin_int64` column
2. Converts existing timestamp data to Unix nanoseconds
3. Drops the unique index on the old column
4. Drops the old `timestamp_begin` column
5. Renames the new column to `timestamp_begin`
6. Sets NOT NULL constraint on the new column
7. Recreates the unique index

## Data Conversion

Existing timestamps are converted using PostgreSQL's `EXTRACT(EPOCH FROM timestamp_begin) * 1000000000` to get Unix nanoseconds.

## Testing

### Local Testing
Run the test script to verify the migration:

```bash
go run test_migration_local.go
```

This will:
1. Check the current column type
2. Run the migration (with fallback to manual migration if needed)
3. Verify the column type changed to bigint
4. Create a test record with int64 timestamp
5. Verify the record can be retrieved correctly

### Heroku Deployment
The migration includes automatic fallback mechanisms for Heroku's PostgreSQL:
- First tries the standard SQL migration
- Falls back to manual row-by-row migration if the standard approach fails
- Handles different PostgreSQL timestamp types (timestamp vs timestamptz)

## Rollback

If rollback is needed, you would need to:
1. Convert int64 values back to timestamps
2. Recreate the timestamp column
3. Update the application code to use time.Time again

## API Changes

The API now expects and returns `timestamp_begin` as an integer (Unix nanoseconds) instead of an ISO timestamp string.

### Before:
```json
{
  "timestamp_begin": "2023-12-01T10:30:00Z",
  "timecode": 12345
}
```

### After:
```json
{
  "timestamp_begin": 1701429000000000000,
  "timecode": 12345
}
```

## Utility Functions

The migration includes utility functions in `pkg/database/migration.go`:
- `ConvertTimeToInt64(t time.Time) int64` - Convert time.Time to int64 nanoseconds
- `ConvertInt64ToTime(ns int64) time.Time` - Convert int64 nanoseconds to time.Time
