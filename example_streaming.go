package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/bill-rich/cncstats/pkg/iniparse"
	"github.com/bill-rich/cncstats/pkg/zhreplay"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run example_streaming.go <replay-file> [ini-data-path]")
		fmt.Println("Example: go run example_streaming.go ./example/simple-generals-replay.rep ./inizh/Data/INI")
		os.Exit(1)
	}

	replayFile := os.Args[1]
	iniDataPath := "./inizh/Data/INI"
	if len(os.Args) > 2 {
		iniDataPath = os.Args[2]
	}

	// Initialize stores
	objectStore, err := iniparse.NewObjectStore(iniDataPath)
	if err != nil {
		log.Fatalf("Could not load object store: %v", err)
	}

	powerStore, err := iniparse.NewPowerStore(iniDataPath)
	if err != nil {
		log.Fatalf("Could not load power store: %v", err)
	}

	upgradeStore, err := iniparse.NewUpgradeStore(iniDataPath)
	if err != nil {
		log.Fatalf("Could not load upgrade store: %v", err)
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Configure streaming options
	options := &zhreplay.StreamReplayOptions{
		PollInterval: 100 * time.Millisecond,
		MaxWaitTime:  30 * time.Second,
		BufferSize:   100,
	}

	fmt.Printf("Starting to stream replay: %s\n", replayFile)
	fmt.Println("Waiting for body events...")

	// Start streaming
	bodyChan, streamingReplay, err := zhreplay.StreamReplay(ctx, replayFile, objectStore, powerStore, upgradeStore, options)
	if err != nil {
		log.Fatalf("Failed to start streaming: %v", err)
	}

	// Print header information
	fmt.Printf("Replay Header:\n")
	fmt.Printf("  Map: %s\n", streamingReplay.Header.Metadata.MapFile)
	fmt.Printf("  Players: %d\n", len(streamingReplay.Header.Metadata.Players))
	for i, player := range streamingReplay.Header.Metadata.Players {
		fmt.Printf("    Player %d: %s (Team %s)\n", i+1, player.Name, player.Team)
	}
	fmt.Println()

	// Process body events
	eventCount := 0
	for {
		select {
		case chunk, ok := <-bodyChan:
			if !ok {
				fmt.Printf("\nStreaming completed. Processed %d events.\n", eventCount)
				return
			}

			eventCount++

			// Print event information
			fmt.Printf("Event %d: Time=%d, Order=%s, PlayerID=%d",
				eventCount, chunk.TimeCode, chunk.OrderName, chunk.PlayerID)

			// Add player name if available
			if chunk.PlayerName != "" {
				fmt.Printf(", Player=%s", chunk.PlayerName)
			}

			// Add details for specific order types
			if chunk.Details != nil {
				switch chunk.OrderCode {
				case 1047: // CreateUnit
					fmt.Printf(", Unit=%s (Cost=%d)", chunk.Details.GetName(), chunk.Details.GetCost())
				case 1049: // BuildObject
					fmt.Printf(", Building=%s (Cost=%d)", chunk.Details.GetName(), chunk.Details.GetCost())
				case 1045: // BuildUpgrade
					fmt.Printf(", Upgrade=%s (Cost=%d)", chunk.Details.GetName(), chunk.Details.GetCost())
				case 1040, 1041, 1042: // SpecialPower variants
					fmt.Printf(", Power=%s", chunk.Details.GetName())
				}
			}

			fmt.Println()

			// Check for EndReplay command
			if chunk.OrderCode == 27 {
				fmt.Println("EndReplay command detected - streaming will stop.")
			}

		case <-ctx.Done():
			fmt.Printf("\nContext cancelled. Processed %d events before timeout.\n", eventCount)
			return
		}
	}
}
