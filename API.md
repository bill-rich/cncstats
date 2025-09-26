# CNC Stats API Documentation

## Player Money Data Endpoints

The API provides endpoints for storing and retrieving player money data at specific timestamps during gameplay.

### POST /player-money

Creates a new player money data record or returns existing record if the combination of `timestamp_begin` and `timecode` already exists.

**Request Body:**
```json
{
  "timestamp_begin": "2024-01-01T12:00:00Z",
  "timecode": 12345,
  "player_1_money": 1000,
  "player_2_money": 2000,
  "player_3_money": 0,
  "player_4_money": 0,
  "player_5_money": 0,
  "player_6_money": 0,
  "player_7_money": 0,
  "player_8_money": 0
}
```

**Response (201 Created for new record, 200 OK for existing record):**
```json
{
  "id": 1,
  "timestamp_begin": "2024-01-01T12:00:00Z",
  "timecode": 12345,
  "player_1_money": 1000,
  "player_2_money": 2000,
  "player_3_money": 0,
  "player_4_money": 0,
  "player_5_money": 0,
  "player_6_money": 0,
  "player_7_money": 0,
  "player_8_money": 0,
  "created_at": "2024-01-01T12:00:00Z",
  "updated_at": "2024-01-01T12:00:00Z"
}
```

**Note:** If a record with the same `timestamp_begin` and `timecode` combination already exists, the API will return the existing record without creating a duplicate. No error is returned in this case.

### GET /player-money

Retrieves player money data with optional pagination.

**Query Parameters:**
- `limit` (optional): Maximum number of records to return
- `offset` (optional): Number of records to skip

**Example:**
```
GET /player-money?limit=10&offset=0
```

**Response (200 OK):**
```json
{
  "data": [
    {
      "id": 1,
      "timestamp_begin": "2024-01-01T12:00:00Z",
      "timecode": 12345,
      "player_1_money": 1000,
      "player_2_money": 2000,
      "player_3_money": 0,
      "player_4_money": 0,
      "player_5_money": 0,
      "player_6_money": 0,
      "player_7_money": 0,
      "player_8_money": 0,
      "created_at": "2024-01-01T12:00:00Z",
      "updated_at": "2024-01-01T12:00:00Z"
    }
  ],
  "count": 1
}
```

### GET /player-money/:id

Retrieves a specific player money data record by ID.

**Response (200 OK):**
```json
{
  "id": 1,
  "timestamp_begin": "2024-01-01T12:00:00Z",
  "timecode": 12345,
  "player_1_money": 1000,
  "player_2_money": 2000,
  "player_3_money": 0,
  "player_4_money": 0,
  "player_5_money": 0,
  "player_6_money": 0,
  "player_7_money": 0,
  "player_8_money": 0,
  "created_at": "2024-01-01T12:00:00Z",
  "updated_at": "2024-01-01T12:00:00Z"
}
```

### DELETE /player-money/:id

Deletes a specific player money data record by ID.

**Response (200 OK):**
```json
{
  "message": "Player money data deleted successfully"
}
```

## Error Responses

All endpoints return appropriate HTTP status codes and error messages:

- `400 Bad Request`: Invalid request format or parameters
- `404 Not Found`: Resource not found
- `500 Internal Server Error`: Server error

**Error Response Format:**
```json
{
  "error": "Error message",
  "details": "Detailed error information"
}
```

## Database Schema

The `player_money_data` table has the following structure:

- `id`: Primary key (auto-increment)
- `timestamp_begin`: Timestamp when the data was recorded (part of unique constraint with timecode)
- `timecode`: Game timecode at the moment of recording (part of unique constraint with timestamp_begin)
- `player_1_money` through `player_8_money`: Money amounts for players 1-8
- `created_at`: Record creation timestamp
- `updated_at`: Record last update timestamp

**Unique Constraint:** The combination of `timestamp_begin` and `timecode` must be unique. This prevents duplicate entries for the same game moment and ensures data integrity.

## Environment Variables

### Local Development
- `DATABASE_URL`: PostgreSQL connection string
- `CNC_INI`: Path to CNC INI data directory
- `PORT`: Application port (default: 8080)
- `GORM_LOG_LEVEL`: Database logging level (debug/info/silent)

### Heroku Production
- `DATABASE_URL`: Automatically set by Heroku Postgres addon
- `PORT`: Automatically set by Heroku
