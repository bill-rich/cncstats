#!/bin/bash

# Development setup script for CNC Stats
# This script helps set up the local development environment

echo "Setting up CNC Stats development environment..."

# Check if Docker is installed
if ! command -v docker &> /dev/null; then
    echo "Error: Docker is not installed. Please install Docker first."
    exit 1
fi

# Check if Docker Compose is installed
if ! command -v docker-compose &> /dev/null; then
    echo "Error: Docker Compose is not installed. Please install Docker Compose first."
    exit 1
fi

# Start PostgreSQL database
echo "Starting PostgreSQL database..."
docker-compose up -d postgres

# Wait for database to be ready
echo "Waiting for database to be ready..."
sleep 10

# Set environment variables for local development
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/cncstats?sslmode=disable"
export CNC_INI="./inizh/Data/INI"
export PORT="8080"

echo "Environment variables set:"
echo "DATABASE_URL=$DATABASE_URL"
echo "CNC_INI=$CNC_INI"
echo "PORT=$PORT"

echo ""
echo "Development environment is ready!"
echo ""
echo "To run the application locally:"
echo "  go run main.go"
echo ""
echo "To test the API endpoints:"
echo "  # Create player money data:"
echo "  curl -X POST http://localhost:8080/player-money \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -d '{\"timestamp_begin\":\"2024-01-01T12:00:00Z\",\"timecode\":12345,\"player_1_money\":1000,\"player_2_money\":2000}'"
echo ""
echo "  # Get all player money data:"
echo "  curl http://localhost:8080/player-money"
echo ""
echo "To stop the database:"
echo "  docker-compose down"
